// Package agentapi provides the HTTP API server for the Worker Agent.
// The Worker Agent runs on KVM-enabled VMs and manages local Firecracker MicroVMs.
package agentapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/clarateach/backend/internal/orchestrator"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

// Server is the Worker Agent HTTP API server.
type Server struct {
	router     *chi.Mux
	provider   *orchestrator.FirecrackerProvider
	agentToken string
	workerID   string
	capacity   int
	startTime  time.Time
	logger     *logrus.Logger
	mu         sync.RWMutex
}

// Config holds configuration for the Worker Agent server.
type Config struct {
	AgentToken string // Token for authenticating requests from Control Plane
	WorkerID   string // Unique identifier for this worker
	Capacity   int    // Maximum number of VMs this worker can host
}

// NewServer creates a new Worker Agent HTTP server.
func NewServer(provider *orchestrator.FirecrackerProvider, cfg Config) *Server {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	if cfg.Capacity == 0 {
		cfg.Capacity = 50 // Default capacity
	}

	s := &Server{
		router:     chi.NewRouter(),
		provider:   provider,
		agentToken: cfg.AgentToken,
		workerID:   cfg.WorkerID,
		capacity:   cfg.Capacity,
		startTime:  time.Now(),
		logger:     logger,
	}

	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// routes sets up the HTTP routes for the Worker Agent API.
func (s *Server) routes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))

	// Health check - no auth required
	s.router.Get("/health", s.handleHealth)

	// Control plane routes require agent authentication
	s.router.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)

		// Worker info
		r.Get("/info", s.handleInfo)

		// VM lifecycle
		r.Route("/vms", func(r chi.Router) {
			r.Get("/", s.handleListVMs)
			r.Post("/", s.handleCreateVM)
			r.Get("/{workshopID}/{seatID}", s.handleGetVM)
			r.Delete("/{workshopID}/{seatID}", s.handleDestroyVM)
		})
	})

	// Proxy routes are public - auth handled by MicroVM's workspace server
	s.router.Route("/proxy", func(r chi.Router) {
		r.Get("/{workshopID}/{seatID}/terminal", s.handleTerminalProxy)
		// Files proxy: both /files (directory listing) and /files/* (file operations)
		r.Get("/{workshopID}/{seatID}/files", s.handleFilesProxy)
		r.HandleFunc("/{workshopID}/{seatID}/files/*", s.handleFilesProxy)
		r.Get("/{workshopID}/{seatID}/health", s.handleHealthProxy)
	})
}

// HealthResponse is the response for the health check endpoint.
type HealthResponse struct {
	Status        string `json:"status"`
	WorkerID      string `json:"worker_id"`
	VMCount       int    `json:"vm_count"`
	Capacity      int    `json:"capacity"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// InfoResponse is the response for the worker info endpoint.
type InfoResponse struct {
	WorkerID       string `json:"worker_id"`
	Version        string `json:"version"`
	Capacity       int    `json:"capacity"`
	CurrentVMs     int    `json:"current_vms"`
	AvailableSlots int    `json:"available_slots"`
	BridgeIP       string `json:"bridge_ip"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
}

// VMRequest is the request body for creating a VM.
type VMRequest struct {
	WorkshopID string `json:"workshop_id"`
	SeatID     int    `json:"seat_id"`
	VCPUs      int64  `json:"vcpus,omitempty"`
	MemoryMB   int64  `json:"memory_mb,omitempty"`
}

// VMResponse is the response for VM operations.
type VMResponse struct {
	WorkshopID string `json:"workshop_id"`
	SeatID     int    `json:"seat_id"`
	IP         string `json:"ip"`
	Status     string `json:"status"`
}

// ErrorResponse is returned for error cases.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// Helper functions for JSON responses

func (s *Server) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.logger.Errorf("Failed to encode JSON response: %v", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	s.writeJSON(w, status, ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// getVMCount returns the current number of VMs managed by this worker.
func (s *Server) getVMCount() int {
	instances, err := s.provider.List(nil, "")
	if err != nil {
		return 0
	}
	// List with empty workshopID returns all VMs, but we need to count all
	// Since the provider tracks VMs in a map, we'll use a workaround
	// For now, return 0 and we'll fix this in the orchestrator
	return len(instances)
}
