package agentapi

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// Proxy configuration
const (
	// MicroVM ports
	terminalPort = 3001
	filesPort    = 3002

	// WebSocket configuration
	wsReadBufferSize  = 1024
	wsWriteBufferSize = 1024
	wsPongWait        = 60 * time.Second
	wsPingPeriod      = (wsPongWait * 9) / 10
)

// WebSocket upgrader with permissive origin check (agent is internal)
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  wsReadBufferSize,
	WriteBufferSize: wsWriteBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins (agent is behind firewall)
	},
}

// getMicroVMIP returns the IP address for a MicroVM based on seat ID
func getMicroVMIP(seatID int) string {
	return fmt.Sprintf("192.168.100.%d", 10+seatID)
}

// handleTerminalProxy proxies WebSocket connections to the MicroVM's terminal server
func (s *Server) handleTerminalProxy(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "workshopID")
	seatIDStr := chi.URLParam(r, "seatID")

	seatID, err := strconv.Atoi(seatIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_seat_id", "seat_id must be an integer")
		return
	}

	// Verify VM exists
	_, err = s.provider.GetIP(r.Context(), workshopID, seatID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "vm_not_found", "VM not found")
		return
	}

	vmIP := getMicroVMIP(seatID)
	// MicroVM server expects /terminal route
	targetURL := fmt.Sprintf("ws://%s:%d/terminal", vmIP, terminalPort)

	s.logger.Infof("Proxying terminal WebSocket: %s seat %d -> %s", workshopID, seatID, targetURL)

	// Upgrade client connection
	clientConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Errorf("Failed to upgrade WebSocket: %v", err)
		return
	}
	defer clientConn.Close()

	// Connect to MicroVM
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// Forward headers that might be needed
	headers := http.Header{}
	if auth := r.Header.Get("Authorization"); auth != "" {
		headers.Set("Authorization", auth)
	}

	backendConn, resp, err := dialer.Dial(targetURL, headers)
	if err != nil {
		s.logger.Errorf("Failed to connect to MicroVM terminal: %v", err)
		if resp != nil {
			s.logger.Errorf("Backend response: %d", resp.StatusCode)
		}
		clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Failed to connect to terminal"))
		return
	}
	defer backendConn.Close()

	// Bidirectional proxy
	errChan := make(chan error, 2)

	// Client -> Backend
	go func() {
		for {
			messageType, message, err := clientConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if err := backendConn.WriteMessage(messageType, message); err != nil {
				errChan <- err
				return
			}
		}
	}()

	// Backend -> Client
	go func() {
		for {
			messageType, message, err := backendConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if err := clientConn.WriteMessage(messageType, message); err != nil {
				errChan <- err
				return
			}
		}
	}()

	// Wait for either direction to close
	<-errChan
	s.logger.Infof("Terminal proxy closed: %s seat %d", workshopID, seatID)
}

// handleFilesProxy proxies HTTP requests to the MicroVM's file server
func (s *Server) handleFilesProxy(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "workshopID")
	seatIDStr := chi.URLParam(r, "seatID")

	seatID, err := strconv.Atoi(seatIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_seat_id", "seat_id must be an integer")
		return
	}

	// Verify VM exists
	_, err = s.provider.GetIP(r.Context(), workshopID, seatID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "vm_not_found", "VM not found")
		return
	}

	vmIP := getMicroVMIP(seatID)
	targetURL, _ := url.Parse(fmt.Sprintf("http://%s:%d", vmIP, filesPort))

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Custom transport with reasonable timeouts
	proxy.Transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	// Custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		s.logger.Errorf("Proxy error for %s seat %d: %v", workshopID, seatID, err)
		http.Error(w, "Failed to connect to file server", http.StatusBadGateway)
	}

	// Modify the request
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host

		// Rewrite path: /proxy/{workshopID}/{seatID}/files/path -> /files/path
		// MicroVM server expects /files prefix (MICROVM_MODE routes)
		prefix := fmt.Sprintf("/proxy/%s/%s", workshopID, seatIDStr)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
		if req.URL.Path == "" {
			req.URL.Path = "/files"
		}

		// Forward original request info
		if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			req.Header.Set("X-Forwarded-For", clientIP)
		}
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	s.logger.Debugf("Proxying files request: %s seat %d %s %s", workshopID, seatID, r.Method, r.URL.Path)
	proxy.ServeHTTP(w, r)
}

// handleHealthProxy proxies health check to MicroVM (for debugging)
func (s *Server) handleHealthProxy(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "workshopID")
	seatIDStr := chi.URLParam(r, "seatID")

	seatID, err := strconv.Atoi(seatIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_seat_id", "seat_id must be an integer")
		return
	}

	vmIP := getMicroVMIP(seatID)

	// Check terminal server health
	terminalURL := fmt.Sprintf("http://%s:%d/health", vmIP, terminalPort)
	terminalOK := checkHealth(terminalURL)

	// Check file server health
	filesURL := fmt.Sprintf("http://%s:%d/health", vmIP, filesPort)
	filesOK := checkHealth(filesURL)

	status := "healthy"
	if !terminalOK || !filesOK {
		status = "unhealthy"
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"workshop_id": workshopID,
		"seat_id":     seatID,
		"vm_ip":       vmIP,
		"status":      status,
		"terminal":    terminalOK,
		"files":       filesOK,
	})
}

// checkHealth performs a simple HTTP health check
func checkHealth(url string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}
