package agentapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/clarateach/backend/internal/orchestrator"
	"github.com/go-chi/chi/v5"
)

// handleHealth returns the health status of the worker agent.
// This endpoint does not require authentication.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	vmCount := s.getVMCount()
	uptime := int64(time.Since(s.startTime).Seconds())

	s.writeJSON(w, http.StatusOK, HealthResponse{
		Status:        "healthy",
		WorkerID:      s.workerID,
		VMCount:       vmCount,
		Capacity:      s.capacity,
		UptimeSeconds: uptime,
	})
}

// handleInfo returns detailed information about the worker agent.
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	vmCount := s.getVMCount()
	uptime := int64(time.Since(s.startTime).Seconds())

	s.writeJSON(w, http.StatusOK, InfoResponse{
		WorkerID:       s.workerID,
		Version:        "1.0.0",
		Capacity:       s.capacity,
		CurrentVMs:     vmCount,
		AvailableSlots: s.capacity - vmCount,
		BridgeIP:       "192.168.100.1/24",
		UptimeSeconds:  uptime,
	})
}

// handleListVMs returns a list of all VMs, optionally filtered by workshop_id.
func (s *Server) handleListVMs(w http.ResponseWriter, r *http.Request) {
	workshopID := r.URL.Query().Get("workshop_id")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	instances, err := s.provider.List(ctx, workshopID)
	if err != nil {
		s.logger.Errorf("Failed to list VMs: %v", err)
		s.writeError(w, http.StatusInternalServerError, "list_failed", "Failed to list VMs")
		return
	}

	vms := make([]VMResponse, 0, len(instances))
	for _, inst := range instances {
		vms = append(vms, VMResponse{
			WorkshopID: inst.WorkshopID,
			SeatID:     inst.SeatID,
			IP:         inst.IP,
			Status:     "running",
		})
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"vms": vms,
	})
}

// handleCreateVM creates a new Firecracker MicroVM.
func (s *Server) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	var req VMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON request body")
		return
	}

	// Validate request
	if req.WorkshopID == "" {
		s.writeError(w, http.StatusBadRequest, "missing_field", "workshop_id is required")
		return
	}
	if req.SeatID <= 0 {
		s.writeError(w, http.StatusBadRequest, "invalid_field", "seat_id must be positive")
		return
	}

	// Check capacity
	vmCount := s.getVMCount()
	if vmCount >= s.capacity {
		s.writeError(w, http.StatusServiceUnavailable, "at_capacity", "Worker is at capacity")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	cfg := orchestrator.InstanceConfig{
		WorkshopID: req.WorkshopID,
		SeatID:     req.SeatID,
	}

	instance, err := s.provider.Create(ctx, cfg)
	if err != nil {
		// Check for specific error types
		errStr := err.Error()
		if strings.Contains(errStr, "already exists") {
			s.writeError(w, http.StatusConflict, "vm_exists", "VM already exists for this workshop and seat")
			return
		}
		s.logger.Errorf("Failed to create VM: %v", err)
		s.writeError(w, http.StatusInternalServerError, "create_failed", "Failed to create VM: "+errStr)
		return
	}

	s.logger.Infof("Created VM for workshop=%s seat=%d ip=%s", req.WorkshopID, req.SeatID, instance.IP)

	s.writeJSON(w, http.StatusCreated, VMResponse{
		WorkshopID: instance.WorkshopID,
		SeatID:     instance.SeatID,
		IP:         instance.IP,
		Status:     "running",
	})
}

// handleGetVM returns information about a specific VM.
func (s *Server) handleGetVM(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "workshopID")
	seatIDStr := chi.URLParam(r, "seatID")

	seatID, err := strconv.Atoi(seatIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_seat_id", "seat_id must be an integer")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ip, err := s.provider.GetIP(ctx, workshopID, seatID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.writeError(w, http.StatusNotFound, "vm_not_found", "VM not found")
			return
		}
		s.logger.Errorf("Failed to get VM: %v", err)
		s.writeError(w, http.StatusInternalServerError, "get_failed", "Failed to get VM")
		return
	}

	s.writeJSON(w, http.StatusOK, VMResponse{
		WorkshopID: workshopID,
		SeatID:     seatID,
		IP:         ip,
		Status:     "running",
	})
}

// handleDestroyVM destroys a Firecracker MicroVM.
func (s *Server) handleDestroyVM(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "workshopID")
	seatIDStr := chi.URLParam(r, "seatID")

	seatID, err := strconv.Atoi(seatIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_seat_id", "seat_id must be an integer")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.provider.Destroy(ctx, workshopID, seatID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.writeError(w, http.StatusNotFound, "vm_not_found", "VM not found")
			return
		}
		s.logger.Errorf("Failed to destroy VM: %v", err)
		s.writeError(w, http.StatusInternalServerError, "destroy_failed", "Failed to destroy VM")
		return
	}

	s.logger.Infof("Destroyed VM for workshop=%s seat=%d", workshopID, seatID)

	w.WriteHeader(http.StatusNoContent)
}
