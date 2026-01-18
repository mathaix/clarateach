// Package main implements the Worker Agent binary.
// The Worker Agent runs on KVM-enabled VMs and manages local Firecracker MicroVMs.
// It exposes an HTTP API on port 9090 for the Control Plane to manage VM lifecycle.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/clarateach/backend/internal/agentapi"
	"github.com/clarateach/backend/internal/network"
	"github.com/clarateach/backend/internal/orchestrator"
	"github.com/clarateach/backend/internal/tunnel"
)

const (
	defaultPort     = "9090"
	defaultCapacity = 50
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Load configuration from environment
	port := getEnv("PORT", defaultPort)
	agentToken := getAgentToken()
	workerID := getWorkerID()
	capacity := getCapacity()

	// Log startup info
	log.Printf("Starting ClaraTeach Worker Agent")
	log.Printf("  Worker ID: %s", workerID)
	log.Printf("  Port: %s", port)
	log.Printf("  Capacity: %d VMs", capacity)
	if agentToken != "" {
		log.Printf("  Auth: enabled")
	} else {
		log.Printf("  Auth: disabled (no AGENT_TOKEN set)")
	}

	// Setup MicroVM bridge network (replaces setup-microvm-network.sh)
	bridgeCfg := network.DefaultBridgeConfig()
	if bridgeName := os.Getenv("BRIDGE_NAME"); bridgeName != "" {
		bridgeCfg.BridgeName = bridgeName
	}
	if bridgeIP := os.Getenv("BRIDGE_IP"); bridgeIP != "" {
		bridgeCfg.BridgeIP = bridgeIP
	}
	if err := network.SetupBridge(bridgeCfg); err != nil {
		log.Fatalf("FATAL: Failed to setup bridge network: %v", err)
	}

	// Initialize Firecracker provider
	fcConfig := orchestrator.DefaultConfig()

	// Override config from environment if set
	if imagesDir := os.Getenv("IMAGES_DIR"); imagesDir != "" {
		fcConfig.ImagesDir = imagesDir
		fcConfig.KernelPath = imagesDir + "/vmlinux"
		fcConfig.RootfsPath = imagesDir + "/rootfs.ext4"
	}
	if socketDir := os.Getenv("SOCKET_DIR"); socketDir != "" {
		fcConfig.SocketDir = socketDir
	}
	if bridgeCfg.BridgeName != "" {
		fcConfig.BridgeName = bridgeCfg.BridgeName
	}
	if bridgeCfg.BridgeIP != "" {
		fcConfig.BridgeIP = bridgeCfg.BridgeIP
	}

	provider, err := orchestrator.NewFirecrackerProviderWithConfig(fcConfig)
	if err != nil {
		log.Fatalf("Failed to create Firecracker provider: %v", err)
	}

	// Create API server
	server := agentapi.NewServer(provider, agentapi.Config{
		AgentToken: agentToken,
		WorkerID:   workerID,
		Capacity:   capacity,
	})

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      server,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Listening on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Start tunnel manager (required unless DEV_MODE=true)
	var tunnelMgr *tunnel.Manager
	devMode := os.Getenv("DEV_MODE") == "true"

	workshopID, workshopErr := getGCPMetadata("workshop-id")
	backendURL, backendErr := getGCPMetadata("backend-url")

	if devMode {
		log.Printf("DEV_MODE=true, skipping tunnel setup")
	} else {
		// Production mode - tunnel is required
		if workshopID == "" {
			log.Fatalf("FATAL: workshop-id metadata not found (error: %v). Set DEV_MODE=true for local development.", workshopErr)
		}
		if backendURL == "" {
			log.Fatalf("FATAL: backend-url metadata not found (error: %v). Set DEV_MODE=true for local development.", backendErr)
		}

		log.Printf("Starting tunnel manager for workshop %s", workshopID)
		tunnelMgr = tunnel.NewManager(tunnel.Config{
			WorkshopID: workshopID,
			BackendURL: backendURL,
			LocalPort:  9090,
		})
		if err := tunnelMgr.Start(); err != nil {
			log.Fatalf("FATAL: Failed to start tunnel: %v", err)
		}

		// Wait for tunnel registration (2 minute timeout)
		log.Printf("Waiting for tunnel registration...")
		if err := tunnelMgr.WaitForRegistration(2 * time.Minute); err != nil {
			log.Fatalf("FATAL: Tunnel registration failed: %v", err)
		}
		log.Printf("Tunnel registered successfully: %s", tunnelMgr.TunnelURL())
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop tunnel manager if running
	if tunnelMgr != nil {
		tunnelMgr.Stop()
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getAgentToken returns the agent token from environment or GCP metadata.
func getAgentToken() string {
	// First check environment variable (for local dev)
	if token := os.Getenv("AGENT_TOKEN"); token != "" {
		return token
	}

	// Try GCP metadata service
	token, err := getGCPMetadata("agent-token")
	if err != nil {
		log.Printf("Could not get agent-token from GCP metadata: %v", err)
		return ""
	}
	return token
}

// getWorkerID returns the worker ID from environment, GCP metadata, or hostname.
func getWorkerID() string {
	// Check environment variable first
	if id := os.Getenv("WORKER_ID"); id != "" {
		return id
	}

	// Try GCP instance name
	if name, err := getGCPMetadata("name"); err == nil && name != "" {
		return name
	}

	// Fall back to hostname
	hostname, err := os.Hostname()
	if err != nil {
		return "worker-unknown"
	}
	return hostname
}

// getCapacity returns the worker capacity from environment.
func getCapacity() int {
	if capStr := os.Getenv("CAPACITY"); capStr != "" {
		cap, err := strconv.Atoi(capStr)
		if err == nil && cap > 0 {
			return cap
		}
	}
	return defaultCapacity
}

// getGCPMetadata fetches a value from the GCP metadata service.
func getGCPMetadata(key string) (string, error) {
	url := fmt.Sprintf("http://metadata.google.internal/computeMetadata/v1/instance/attributes/%s", key)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metadata request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
