// Package tunnel manages Cloudflare Quick Tunnels for exposing the agent.
package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Manager handles the cloudflared tunnel lifecycle.
type Manager struct {
	workshopID   string
	backendURL   string
	localPort    int
	tunnelURL    string
	cmd          *exec.Cmd
	mu           sync.RWMutex
	registered   bool
	registerErr  error
	registerDone chan struct{}
	closeOnce    sync.Once
	ctx          context.Context
	cancel       context.CancelFunc
}

// Config holds tunnel manager configuration.
type Config struct {
	WorkshopID string
	BackendURL string
	LocalPort  int // Port to tunnel (default 9090)
}

// NewManager creates a new tunnel manager.
func NewManager(cfg Config) *Manager {
	if cfg.LocalPort == 0 {
		cfg.LocalPort = 9090
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		workshopID:   cfg.WorkshopID,
		backendURL:   strings.TrimSuffix(cfg.BackendURL, "/"),
		localPort:    cfg.LocalPort,
		registerDone: make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// signalDone safely closes the registerDone channel exactly once.
func (m *Manager) signalDone() {
	m.closeOnce.Do(func() {
		close(m.registerDone)
	})
}

// Start launches cloudflared and captures the tunnel URL.
func (m *Manager) Start() error {
	if m.workshopID == "" {
		return fmt.Errorf("workshop ID is required")
	}
	if m.backendURL == "" {
		return fmt.Errorf("backend URL is required")
	}

	log.Printf("[tunnel] Starting Cloudflare Quick Tunnel for workshop %s", m.workshopID)
	log.Printf("[tunnel] Backend URL: %s", m.backendURL)
	log.Printf("[tunnel] Local port: %d", m.localPort)

	// Start cloudflared
	localURL := fmt.Sprintf("http://localhost:%d", m.localPort)
	m.cmd = exec.CommandContext(m.ctx, "cloudflared", "tunnel", "--url", localURL)

	// Capture both stdout and stderr (cloudflared outputs URL to stderr)
	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start cloudflared: %w", err)
	}

	log.Printf("[tunnel] cloudflared started with PID %d", m.cmd.Process.Pid)

	// Process output in goroutines
	go m.processOutput(stdout, "stdout")
	go m.processOutput(stderr, "stderr")

	// Timeout for URL capture - if no URL found in 60s, fail
	go func() {
		select {
		case <-time.After(60 * time.Second):
			m.mu.Lock()
			if m.tunnelURL == "" {
				m.registerErr = fmt.Errorf("timeout: no tunnel URL captured after 60 seconds")
				m.mu.Unlock()
				m.signalDone()
				return
			}
			m.mu.Unlock()
		case <-m.registerDone:
			// Already done (URL found and registration attempted)
		case <-m.ctx.Done():
			// Cancelled
		}
	}()

	// Wait for cloudflared to exit in background
	go func() {
		err := m.cmd.Wait()
		if err != nil && m.ctx.Err() == nil {
			log.Printf("[tunnel] cloudflared exited with error: %v", err)
			// If cloudflared exits before we capture URL, signal failure
			m.mu.Lock()
			if m.tunnelURL == "" {
				m.registerErr = fmt.Errorf("cloudflared exited before URL captured: %v", err)
				m.mu.Unlock()
				m.signalDone()
				return
			}
			m.mu.Unlock()
		}
	}()

	return nil
}

// processOutput reads output line by line looking for the tunnel URL.
func (m *Manager) processOutput(r io.Reader, source string) {
	scanner := bufio.NewScanner(r)
	urlPattern := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("[tunnel:%s] %s", source, line)

		// Check if line contains tunnel URL
		if match := urlPattern.FindString(line); match != "" {
			m.mu.Lock()
			if m.tunnelURL == "" {
				m.tunnelURL = match
				log.Printf("[tunnel] Captured tunnel URL: %s", m.tunnelURL)
				// Register in background
				go m.registerTunnelURL()
			}
			m.mu.Unlock()
		}
	}

	if err := scanner.Err(); err != nil && m.ctx.Err() == nil {
		log.Printf("[tunnel:%s] scanner error: %v", source, err)
	}
}

// registerTunnelURL reports the tunnel URL to the backend.
func (m *Manager) registerTunnelURL() {
	defer m.signalDone()

	m.mu.RLock()
	url := m.tunnelURL
	m.mu.RUnlock()

	if url == "" {
		m.mu.Lock()
		m.registerErr = fmt.Errorf("no tunnel URL captured")
		m.mu.Unlock()
		log.Printf("[tunnel] ERROR: No tunnel URL to register")
		return
	}

	endpoint := fmt.Sprintf("%s/api/internal/workshops/%s/tunnel", m.backendURL, m.workshopID)
	payload := map[string]string{"tunnel_url": url}
	body, _ := json.Marshal(payload)

	log.Printf("[tunnel] Registering tunnel URL with backend: %s", endpoint)

	// Retry up to 5 times
	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error

	for attempt := 1; attempt <= 5; attempt++ {
		req, err := http.NewRequestWithContext(m.ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("[tunnel] Registration attempt %d failed: %v", attempt, err)
			time.Sleep(2 * time.Second)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			m.mu.Lock()
			m.registered = true
			m.mu.Unlock()
			log.Printf("[tunnel] Successfully registered tunnel URL")
			return
		}

		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		log.Printf("[tunnel] Registration attempt %d failed: %v", attempt, lastErr)
		time.Sleep(2 * time.Second)
	}

	m.mu.Lock()
	m.registerErr = fmt.Errorf("failed after 5 attempts: %w", lastErr)
	m.mu.Unlock()
	log.Printf("[tunnel] ERROR: Failed to register tunnel URL after 5 attempts: %v", lastErr)
}

// WaitForRegistration blocks until tunnel registration completes or timeout.
// Returns error if registration failed or timed out.
func (m *Manager) WaitForRegistration(timeout time.Duration) error {
	select {
	case <-m.registerDone:
		m.mu.RLock()
		err := m.registerErr
		m.mu.RUnlock()
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for tunnel registration after %v", timeout)
	case <-m.ctx.Done():
		return fmt.Errorf("tunnel manager cancelled")
	}
}

// TunnelURL returns the current tunnel URL (empty if not yet available).
func (m *Manager) TunnelURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tunnelURL
}

// IsRegistered returns whether the tunnel URL has been registered with the backend.
func (m *Manager) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}

// Stop terminates the cloudflared process.
func (m *Manager) Stop() {
	log.Printf("[tunnel] Stopping tunnel manager")
	m.cancel()
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
	}
}
