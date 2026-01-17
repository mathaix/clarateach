package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/clarateach/backend/internal/auth"
	"github.com/clarateach/backend/internal/provisioner"
	"github.com/clarateach/backend/internal/sshutil"
	"github.com/clarateach/backend/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Server struct {
	router                    *chi.Mux
	store                     store.Store
	provisioner               provisioner.Provisioner
	firecrackerProvisioner    *provisioner.FirecrackerProvisioner    // Local Firecracker
	gcpFirecrackerProvisioner *provisioner.GCPFirecrackerProvider    // GCP + Firecracker
	useSpotVMs                bool
	authDisabled              bool
}

func NewServer(store store.Store, prov provisioner.Provisioner, useSpotVMs bool, authDisabled bool) *Server {
	s := &Server{
		store:        store,
		provisioner:  prov,
		router:       chi.NewRouter(),
		useSpotVMs:   useSpotVMs,
		authDisabled: authDisabled,
	}

	// Initialize local Firecracker provisioner (optional - may fail if not on Linux with KVM)
	fcProv, err := provisioner.NewFirecrackerProvisioner()
	if err != nil {
		log.Printf("Local Firecracker provisioner not available: %v", err)
	} else {
		s.firecrackerProvisioner = fcProv
		log.Printf("Local Firecracker provisioner initialized")
	}

	s.routes()
	return s
}

// SetGCPFirecrackerProvisioner sets the GCP Firecracker provisioner
func (s *Server) SetGCPFirecrackerProvisioner(prov *provisioner.GCPFirecrackerProvider) {
	s.gcpFirecrackerProvisioner = prov
	log.Printf("GCP Firecracker provisioner initialized")
}

// getProvisioner returns the appropriate provisioner based on runtime type
func (s *Server) getProvisioner(runtimeType string) provisioner.Provisioner {
	if runtimeType == "firecracker" {
		// Prefer GCP Firecracker provisioner (creates GCP VM + MicroVMs)
		if s.gcpFirecrackerProvisioner != nil {
			return s.gcpFirecrackerProvisioner
		}
		// Fall back to local Firecracker provisioner (for dev/testing)
		if s.firecrackerProvisioner != nil {
			return s.firecrackerProvisioner
		}
	}
	return s.provisioner
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // In prod, be more restrictive
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
	}))

	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	s.router.Route("/api", func(r chi.Router) {
		// Auth endpoints (public)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.authRegister)
			r.Post("/login", s.authLogin)
			r.Post("/logout", s.authLogout)
		})

		// Protected auth endpoint
		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware(s.store))
			r.Get("/auth/me", s.authMe)
		})

		// Learner registration (public)
		r.Post("/register", s.registerForWorkshop)
		r.Get("/session/{code}", s.getSessionByCode)
		r.Post("/join", s.joinWorkshop)

		// Instructor routes (protected)
		r.Route("/workshops", func(r chi.Router) {
			if !s.authDisabled {
				r.Use(auth.AuthMiddleware(s.store))
			}
			r.Get("/", s.listWorkshops)
			r.Post("/", s.createWorkshop)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.getWorkshop)
				r.Delete("/", s.deleteWorkshop)
				r.Post("/start", s.startWorkshop)
				r.Post("/stop", s.stopWorkshop)
			})
		})

		// Admin API for VM management (protected, admin only)
		r.Route("/admin", func(r chi.Router) {
			if !s.authDisabled {
				r.Use(auth.AuthMiddleware(s.store))
				r.Use(auth.AdminMiddleware)
			}
			r.Get("/overview", s.adminOverview)
			r.Get("/vms", s.listVMs)
			r.Get("/vms/{workshop_id}", s.getVMDetails)
			r.Get("/vms/{workshop_id}/ssh-key", s.getSSHKey)
			r.Get("/users", s.listUsers)
		})

		// Internal API for agent VMs (no auth - called from within GCP)
		r.Route("/internal", func(r chi.Router) {
			r.Post("/workshops/{id}/tunnel", s.registerTunnel)
		})
	})

	// Seed admin user on startup if configured
	s.seedAdminUser()
}

// Handlers

func (s *Server) listWorkshops(w http.ResponseWriter, r *http.Request) {
	var workshops []*store.Workshop
	var err error

	// If auth is enabled, filter by owner (unless admin)
	user := auth.GetUserFromContext(r.Context())
	if user != nil && user.Role != "admin" {
		workshops, err = s.store.ListWorkshopsByOwner(user.ID)
	} else {
		workshops, err = s.store.ListWorkshops()
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"workshops": workshops})
}

func (s *Server) createWorkshop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Seats       int    `json:"seats"`
		ApiKey      string `json:"api_key"`
		RuntimeType string `json:"runtime_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Default runtime to docker
	if req.RuntimeType == "" {
		req.RuntimeType = "docker"
	}

	// Get owner from auth context if available
	ownerID := ""
	user := auth.GetUserFromContext(r.Context())
	if user != nil {
		ownerID = user.ID
	}

	workshop := &store.Workshop{
		ID:          "ws-" + generateID(6),
		Name:        req.Name,
		Code:        generateCode(),
		Seats:       req.Seats,
		ApiKey:      req.ApiKey,
		RuntimeType: req.RuntimeType,
		Status:      "created",
		OwnerID:     ownerID,
		CreatedAt:   time.Now(),
	}

	if err := s.store.CreateWorkshop(workshop); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create placeholder sessions for seat tracking
	for i := 1; i <= workshop.Seats; i++ {
		session := &store.Session{
			OdeHash:    generateID(5),
			WorkshopID: workshop.ID,
			SeatID:     i,
			Status:     "pending",
			JoinedAt:   time.Now(),
		}
		if err := s.store.CreateSession(session); err != nil {
			log.Printf("Failed to create session record for seat %d: %v", i, err)
		}
	}

	// Set status to provisioning immediately
	log.Printf("Provisioning GCP VM for workshop %s (%s) with %d seats", workshop.Name, workshop.ID, workshop.Seats)
	s.store.UpdateWorkshopStatus(workshop.ID, "provisioning")
	workshop.Status = "provisioning"

	// Return response immediately - provisioning happens async
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"workshop": workshop})

	// Provision VM asynchronously
	go func() {
		// Generate SSH key pair for debugging access
		keyPair, err := sshutil.GenerateKeyPair(fmt.Sprintf("clarateach-%s", workshop.ID))
		if err != nil {
			log.Printf("Failed to generate SSH key: %v", err)
			s.store.UpdateWorkshopStatus(workshop.ID, "error")
			return
		}

		// Create VM config
		vmConfig := provisioner.DefaultConfig(workshop.ID, workshop.Seats)
		vmConfig.Spot = s.useSpotVMs
		vmConfig.SSHPublicKey = keyPair.PublicKey
		vmConfig.AuthDisabled = s.authDisabled
		vmConfig.RuntimeType = workshop.RuntimeType

		// Track provisioning time
		provisioningStartedAt := time.Now()

		// Provision VM with background context (not tied to HTTP request)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// Use runtime-specific provisioner
		prov := s.getProvisioner(workshop.RuntimeType)
		vmInstance, err := prov.CreateVM(ctx, vmConfig)
		if err != nil {
			log.Printf("Failed to provision VM: %v", err)
			s.store.UpdateWorkshopStatus(workshop.ID, "error")
			return
		}

		provisioningCompletedAt := time.Now()
		provisioningDurationMs := provisioningCompletedAt.Sub(provisioningStartedAt).Milliseconds()

		log.Printf("VM created: %s (IP: %s) in %dms", vmInstance.Name, vmInstance.ExternalIP, provisioningDurationMs)

		// Store VM info in database
		workshopVM := &store.WorkshopVM{
			ID:                      generateID(8),
			WorkshopID:              workshop.ID,
			VMName:                  vmInstance.Name,
			VMID:                    vmInstance.ID,
			Zone:                    vmInstance.Zone,
			MachineType:             vmConfig.MachineType,
			ExternalIP:              vmInstance.ExternalIP,
			InternalIP:              vmInstance.InternalIP,
			Status:                  vmInstance.Status,
			SSHPublicKey:            keyPair.PublicKey,
			SSHPrivateKey:           keyPair.PrivateKey,
			SSHUser:                 "clarateach",
			ProvisioningStartedAt:   &provisioningStartedAt,
			ProvisioningCompletedAt: &provisioningCompletedAt,
			ProvisioningDurationMs:  provisioningDurationMs,
			CreatedAt:               time.Now(),
			UpdatedAt:               time.Now(),
		}

		if err := s.store.CreateVM(workshopVM); err != nil {
			log.Printf("Failed to save VM info: %v", err)
		}

		// Update sessions to ready (containers run inside VM via startup script)
		for i := 1; i <= workshop.Seats; i++ {
			sess, _ := s.store.GetSessionBySeat(workshop.ID, i)
			if sess != nil {
				sess.Status = "ready"
				sess.IP = vmInstance.ExternalIP
				s.store.UpdateSession(sess)
			}
		}

		s.store.UpdateWorkshopStatus(workshop.ID, "running")
		log.Printf("Workshop %s is now running", workshop.ID)
	}()
}

func (s *Server) getWorkshop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workshop, err := s.store.GetWorkshop(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workshop == nil {
		http.Error(w, "Workshop not found", http.StatusNotFound)
		return
	}

	// Build response with VM info
	resp := map[string]interface{}{
		"id":         workshop.ID,
		"name":       workshop.Name,
		"code":       workshop.Code,
		"seats":      workshop.Seats,
		"status":     workshop.Status,
		"created_at": workshop.CreatedAt,
	}

	// Try to get VM info for the IP
	ctx := r.Context()
	prov := s.getProvisioner(workshop.RuntimeType)
	if vm, err := prov.GetVM(ctx, id); err == nil && vm != nil {
		resp["vm_ip"] = vm.ExternalIP
		resp["vm_status"] = vm.Status
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"workshop": resp})
}

func (s *Server) deleteWorkshop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get workshop to determine runtime type
	workshop, err := s.store.GetWorkshop(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workshop == nil {
		http.Error(w, "Workshop not found", http.StatusNotFound)
		return
	}
	runtimeType := workshop.RuntimeType

	// Set status to "deleting" immediately for UI feedback
	if err := s.store.UpdateWorkshopStatus(id, "deleting"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Delete the VM asynchronously, then set status to "deleted"
	go func() {
		log.Printf("Deleting VM for workshop %s (runtime: %s, async)", id, runtimeType)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		prov := s.getProvisioner(runtimeType)
		if err := prov.DeleteVM(ctx, id); err != nil {
			log.Printf("Failed to delete VM for workshop %s: %v", id, err)
		} else {
			log.Printf("VM deleted successfully for workshop %s", id)
		}

		// Mark VM as removed in database
		if err := s.store.MarkVMRemoved(id); err != nil {
			log.Printf("Failed to mark VM as removed: %v", err)
		}

		// Update status to "deleted" after VM cleanup
		if err := s.store.UpdateWorkshopStatus(id, "deleted"); err != nil {
			log.Printf("Failed to update workshop status to deleted: %v", err)
		}
	}()

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) stopWorkshop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get workshop to verify it exists
	workshop, err := s.store.GetWorkshop(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workshop == nil {
		http.Error(w, "Workshop not found", http.StatusNotFound)
		return
	}

	// Update status to "stopping" immediately for UI feedback
	if err := s.store.UpdateWorkshopStatus(id, "stopping"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	runtimeType := workshop.RuntimeType

	// Delete the VM asynchronously, then set status to "stopped"
	go func() {
		log.Printf("Stopping workshop %s - deleting VM (runtime: %s, async)", id, runtimeType)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		prov := s.getProvisioner(runtimeType)
		if err := prov.DeleteVM(ctx, id); err != nil {
			log.Printf("Failed to delete VM for workshop %s: %v", id, err)
		} else {
			log.Printf("VM deleted successfully for workshop %s", id)
		}

		// Mark VM as removed in database
		if err := s.store.MarkVMRemoved(id); err != nil {
			log.Printf("Failed to mark VM as removed: %v", err)
		}

		// Update status to "stopped" after VM cleanup
		if err := s.store.UpdateWorkshopStatus(id, "stopped"); err != nil {
			log.Printf("Failed to update workshop status to stopped: %v", err)
		}
	}()

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) startWorkshop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get workshop
	workshop, err := s.store.GetWorkshop(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workshop == nil {
		http.Error(w, "Workshop not found", http.StatusNotFound)
		return
	}

	// Update status to provisioning
	if err := s.store.UpdateWorkshopStatus(id, "provisioning"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Provisioning GCP VM for workshop %s (%s) with %d seats", workshop.Name, id, workshop.Seats)

	// Generate SSH key pair for debugging access
	keyPair, err := sshutil.GenerateKeyPair(fmt.Sprintf("clarateach-%s", id))
	if err != nil {
		log.Printf("Failed to generate SSH key: %v", err)
		s.store.UpdateWorkshopStatus(id, "error")
		http.Error(w, fmt.Sprintf("Failed to generate SSH key: %v", err), http.StatusInternalServerError)
		return
	}

	// Create VM config
	vmConfig := provisioner.DefaultConfig(id, workshop.Seats)
	vmConfig.Spot = s.useSpotVMs
	vmConfig.SSHPublicKey = keyPair.PublicKey
	vmConfig.AuthDisabled = s.authDisabled
	vmConfig.RuntimeType = workshop.RuntimeType

	// Track provisioning time
	provisioningStartedAt := time.Now()

	// Provision VM using runtime-specific provisioner
	ctx := r.Context()
	prov := s.getProvisioner(workshop.RuntimeType)
	vmInstance, err := prov.CreateVM(ctx, vmConfig)
	if err != nil {
		log.Printf("Failed to provision VM: %v", err)
		s.store.UpdateWorkshopStatus(id, "error")
		http.Error(w, fmt.Sprintf("Failed to provision VM: %v", err), http.StatusInternalServerError)
		return
	}

	provisioningCompletedAt := time.Now()
	provisioningDurationMs := provisioningCompletedAt.Sub(provisioningStartedAt).Milliseconds()

	log.Printf("VM created: %s (IP: %s) in %dms", vmInstance.Name, vmInstance.ExternalIP, provisioningDurationMs)

	// Store VM info in database
	workshopVM := &store.WorkshopVM{
		ID:                      generateID(8),
		WorkshopID:              id,
		VMName:                  vmInstance.Name,
		VMID:                    vmInstance.ID,
		Zone:                    vmInstance.Zone,
		MachineType:             vmConfig.MachineType,
		ExternalIP:              vmInstance.ExternalIP,
		InternalIP:              vmInstance.InternalIP,
		Status:                  vmInstance.Status,
		SSHPublicKey:            keyPair.PublicKey,
		SSHPrivateKey:           keyPair.PrivateKey,
		SSHUser:                 "clarateach",
		ProvisioningStartedAt:   &provisioningStartedAt,
		ProvisioningCompletedAt: &provisioningCompletedAt,
		ProvisioningDurationMs:  provisioningDurationMs,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}

	if err := s.store.CreateVM(workshopVM); err != nil {
		log.Printf("Failed to save VM info: %v", err)
	}

	// Update workshop status
	s.store.UpdateWorkshopStatus(id, "running")

	// Return success with VM info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"vm": map[string]string{
			"name":                     vmInstance.Name,
			"external_ip":              vmInstance.ExternalIP,
			"status":                   vmInstance.Status,
			"provisioning_duration_ms": fmt.Sprintf("%d", provisioningDurationMs),
		},
	})
}

func (s *Server) joinWorkshop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code    string `json:"code"`
		OdeHash string `json:"odehash"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Find Workshop
	workshop, err := s.store.GetWorkshopByCode(req.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workshop == nil {
		http.Error(w, "Workshop not found", http.StatusNotFound)
		return
	}

	// 2. Handle Reconnect vs New Join
	var session *store.Session
	if req.OdeHash != "" {
		session, err = s.store.GetSession(req.OdeHash)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if session == nil {
		// Allocate Seat - find first available session
		existing, err := s.store.ListSessions(workshop.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, sess := range existing {
			if sess.Name == "" && (sess.Status == "ready" || sess.Status == "provisioning") {
				session = sess
				break
			}
		}

		if session == nil {
			http.Error(w, "Workshop is full", http.StatusConflict)
			return
		}

		// Assign User to Session
		session.Name = req.Name
		if session.Name == "" {
			session.Name = "Learner " + fmt.Sprintf("%d", session.SeatID)
		}
		session.Status = "occupied"
		session.JoinedAt = time.Now()

		if err := s.store.UpdateSession(session); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// 3. Get VM's external IP
	vm, err := s.store.GetVM(workshop.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get VM info: %v", err), http.StatusInternalServerError)
		return
	}
	if vm == nil {
		http.Error(w, "Workshop VM not found - workshop may not be started", http.StatusServiceUnavailable)
		return
	}

	containerIP := vm.ExternalIP
	session.IP = containerIP
	session.ContainerID = fmt.Sprintf("seat-%d", session.SeatID)
	if err := s.store.UpdateSession(session); err != nil {
		log.Printf("Warning: failed to update session: %v", err)
	}

	// 4. Construct endpoint URL
	endpoint := fmt.Sprintf("http://%s:8080", containerIP)

	resp := map[string]interface{}{
		"workshop_id": workshop.ID,
		"seat":        session.SeatID,
		"odehash":     session.OdeHash,
		"endpoint":    endpoint,
		"ip":          containerIP,
	}
	json.NewEncoder(w).Encode(resp)
}

// Utils

func generateID(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func generateCodeID(n int) string {
	var letters = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func generateCode() string {
	return generateCodeID(5) + "-" + generateCodeID(4)
}

func generateAccessCode() string {
	// Format: XXX-XXXX (3 letters/numbers - 4 letters/numbers)
	return generateCodeID(3) + "-" + generateCodeID(4)
}

// ================== Registration Handlers ==================

func (s *Server) registerForWorkshop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkshopCode string `json:"workshop_code"`
		Email        string `json:"email"`
		Name         string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.WorkshopCode == "" || req.Email == "" || req.Name == "" {
		http.Error(w, "Workshop code, email, and name are required", http.StatusBadRequest)
		return
	}

	// Find workshop by code
	workshop, err := s.store.GetWorkshopByCode(req.WorkshopCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workshop == nil {
		http.Error(w, "Workshop not found", http.StatusNotFound)
		return
	}

	// Check workshop status
	if workshop.Status == "ended" || workshop.Status == "deleted" {
		http.Error(w, "Workshop has ended", http.StatusGone)
		return
	}

	// Check if email already registered for this workshop
	existing, err := s.store.GetRegistrationByEmail(workshop.ID, req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existing != nil {
		// Return existing registration
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_code":        existing.AccessCode,
			"already_registered": true,
			"message":            "You are already registered for this workshop",
		})
		return
	}

	// Check if workshop is full
	registrationCount, err := s.store.CountRegistrations(workshop.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if registrationCount >= workshop.Seats {
		http.Error(w, "Workshop is full", http.StatusConflict)
		return
	}

	// Create registration
	registration := &store.Registration{
		ID:         "reg-" + generateID(8),
		AccessCode: generateAccessCode(),
		Email:      req.Email,
		Name:       req.Name,
		WorkshopID: workshop.ID,
		Status:     "registered",
		CreatedAt:  time.Now(),
	}

	if err := s.store.CreateRegistration(registration); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_code":        registration.AccessCode,
		"already_registered": false,
		"message":            "Registration successful",
	})
}

func (s *Server) getSessionByCode(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.Error(w, "Access code is required", http.StatusBadRequest)
		return
	}

	// Find registration by access code
	registration, err := s.store.GetRegistration(code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if registration == nil {
		http.Error(w, "Invalid access code", http.StatusNotFound)
		return
	}

	// Get workshop
	workshop, err := s.store.GetWorkshop(registration.WorkshopID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workshop == nil {
		http.Error(w, "Workshop not found", http.StatusNotFound)
		return
	}

	// Check workshop status
	if workshop.Status == "ended" || workshop.Status == "deleted" {
		http.Error(w, "Workshop has ended", http.StatusGone)
		return
	}

	// Get VM info
	vm, err := s.store.GetVM(workshop.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if vm == nil || vm.ExternalIP == "" {
		// Workshop not started yet
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "pending",
			"message":     "Workshop is starting. Please wait...",
			"workshop_id": workshop.ID,
		})
		return
	}

	// If user doesn't have a seat yet, assign one
	if registration.SeatID == nil {
		// Find available seat
		sessions, err := s.store.ListSessions(workshop.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var availableSeat *store.Session
		for _, sess := range sessions {
			if sess.Name == "" && (sess.Status == "ready" || sess.Status == "provisioning") {
				availableSeat = sess
				break
			}
		}

		if availableSeat == nil {
			http.Error(w, "No seats available", http.StatusConflict)
			return
		}

		// Assign seat to registration
		seatID := availableSeat.SeatID
		registration.SeatID = &seatID
		registration.Status = "active"
		now := time.Now()
		registration.JoinedAt = &now

		if err := s.store.UpdateRegistration(registration); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Update session with user info
		availableSeat.Name = registration.Name
		availableSeat.Status = "occupied"
		availableSeat.IP = vm.ExternalIP
		availableSeat.ContainerID = fmt.Sprintf("seat-%d", seatID)
		if err := s.store.UpdateSession(availableSeat); err != nil {
			log.Printf("Warning: failed to update session: %v", err)
		}
	}

	// Build endpoint URL based on runtime type and tunnel availability
	var endpoint string
	if workshop.RuntimeType == "firecracker" {
		// Prefer tunnel URL if available (HTTPS via Cloudflare)
		if vm.TunnelURL != "" {
			endpoint = vm.TunnelURL
		} else {
			// Fallback to direct IP (HTTP) - only for development
			endpoint = fmt.Sprintf("http://%s:9090", vm.ExternalIP)
		}
	} else {
		// Docker workspace server runs on port 8080
		endpoint = fmt.Sprintf("http://%s:8080", vm.ExternalIP)
	}

	// Generate workspace token for WebSocket authentication
	token, err := auth.GenerateWorkspaceToken(workshop.ID, *registration.SeatID)
	if err != nil {
		log.Printf("Failed to generate workspace token: %v", err)
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ready",
		"endpoint":     endpoint,
		"token":        token,
		"seat":         *registration.SeatID,
		"name":         registration.Name,
		"workshop_id":  workshop.ID,
		"runtime_type": workshop.RuntimeType,
	})
}

// ================== Internal API Handlers ==================

func (s *Server) registerTunnel(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "id")

	var req struct {
		TunnelURL string `json:"tunnel_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TunnelURL == "" {
		http.Error(w, "tunnel_url is required", http.StatusBadRequest)
		return
	}

	// Verify workshop exists
	workshop, err := s.store.GetWorkshop(workshopID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workshop == nil {
		http.Error(w, "Workshop not found", http.StatusNotFound)
		return
	}

	// Update tunnel URL
	if err := s.store.UpdateVMTunnelURL(workshopID, req.TunnelURL); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Tunnel URL registered for workshop %s: %s", workshopID, req.TunnelURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"tunnel_url": req.TunnelURL,
	})
}

// ================== Admin Handlers ==================

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	workshops, err := s.store.ListWorkshops()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var views []store.AdminWorkshopView
	for _, ws := range workshops {
		view := store.AdminWorkshopView{
			Workshop:   ws,
			TotalSeats: ws.Seats,
		}

		// Get VM info
		vm, _ := s.store.GetVM(ws.ID)
		if vm != nil {
			view.VM = vm
			if vm.ExternalIP != "" && vm.SSHUser != "" {
				view.SSHCommand = fmt.Sprintf("ssh -i clarateach_%s.pem %s@%s", ws.ID, vm.SSHUser, vm.ExternalIP)
			}
		}

		// Get sessions and count active students
		sessions, _ := s.store.ListSessions(ws.ID)
		view.Sessions = sessions
		for _, sess := range sessions {
			if sess.Status == "occupied" && sess.Name != "" {
				view.ActiveStudents++
			}
		}

		views = append(views, view)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workshops": views,
		"total":     len(views),
	})
}

func (s *Server) listVMs(w http.ResponseWriter, r *http.Request) {
	vms, err := s.store.ListVMs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type VMWithWorkshop struct {
		*store.WorkshopVM
		WorkshopName   string `json:"workshop_name"`
		ActiveStudents int    `json:"active_students"`
		TotalSeats     int    `json:"total_seats"`
		SSHCommand     string `json:"ssh_command"`
		GCloudSSH      string `json:"gcloud_ssh"`
	}

	var enriched []VMWithWorkshop
	for _, vm := range vms {
		vmw := VMWithWorkshop{WorkshopVM: vm}

		ws, _ := s.store.GetWorkshop(vm.WorkshopID)
		if ws != nil {
			vmw.WorkshopName = ws.Name
			vmw.TotalSeats = ws.Seats
		}

		sessions, _ := s.store.ListSessions(vm.WorkshopID)
		for _, sess := range sessions {
			if sess.Status == "occupied" && sess.Name != "" {
				vmw.ActiveStudents++
			}
		}

		if vm.ExternalIP != "" && vm.SSHUser != "" {
			vmw.SSHCommand = fmt.Sprintf("ssh -i clarateach_%s.pem %s@%s", vm.WorkshopID, vm.SSHUser, vm.ExternalIP)
		}
		if vm.VMName != "" && vm.Zone != "" {
			vmw.GCloudSSH = fmt.Sprintf("gcloud compute ssh %s --zone=%s", vm.VMName, vm.Zone)
		}

		enriched = append(enriched, vmw)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"vms":   enriched,
		"total": len(enriched),
	})
}

func (s *Server) getVMDetails(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "workshop_id")

	vm, err := s.store.GetVM(workshopID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if vm == nil {
		http.Error(w, "VM not found", http.StatusNotFound)
		return
	}

	ws, _ := s.store.GetWorkshop(workshopID)
	sessions, _ := s.store.ListSessions(workshopID)

	activeStudents := 0
	for _, sess := range sessions {
		if sess.Status == "occupied" && sess.Name != "" {
			activeStudents++
		}
	}

	response := map[string]interface{}{
		"vm":       vm,
		"workshop": ws,
		"sessions": sessions,
		"stats": map[string]interface{}{
			"active_students": activeStudents,
			"total_seats":     ws.Seats,
		},
		"access": map[string]string{
			"ssh_command": fmt.Sprintf("ssh -i clarateach_%s.pem %s@%s", workshopID, vm.SSHUser, vm.ExternalIP),
			"gcloud_ssh":  fmt.Sprintf("gcloud compute ssh %s --zone=%s", vm.VMName, vm.Zone),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) getSSHKey(w http.ResponseWriter, r *http.Request) {
	workshopID := chi.URLParam(r, "workshop_id")

	privateKey, err := s.store.GetVMPrivateKey(workshopID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if privateKey == "" {
		http.Error(w, "SSH key not found", http.StatusNotFound)
		return
	}

	filename := fmt.Sprintf("clarateach_%s.pem", workshopID)
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write([]byte(privateKey))
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"users": users})
}

// ================== Auth Handlers ==================

func (s *Server) authRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" || req.Name == "" {
		http.Error(w, "Email, password, and name are required", http.StatusBadRequest)
		return
	}

	// Check if email already exists
	existing, err := s.store.GetUserByEmail(req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "Email already registered", http.StatusConflict)
		return
	}

	// Hash password
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Create user
	user := &store.User{
		ID:           "user-" + generateID(8),
		Email:        req.Email,
		PasswordHash: hash,
		Name:         req.Name,
		Role:         "instructor",
		CreatedAt:    time.Now(),
	}

	if err := s.store.CreateUser(user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate token
	token, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Find user
	user, err := s.store.GetUserByEmail(req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check password
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate token
	token, err := auth.GenerateToken(user)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	// JWT is stateless, so we just return success
	// Client should delete the token
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"user": user})
}

// seedAdminUser creates an admin user from env vars if configured
func (s *Server) seedAdminUser() {
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	if adminEmail == "" || adminPassword == "" {
		return
	}

	// Check if admin already exists
	existing, err := s.store.GetUserByEmail(adminEmail)
	if err != nil {
		log.Printf("Failed to check for existing admin: %v", err)
		return
	}
	if existing != nil {
		log.Printf("Admin user already exists: %s", adminEmail)
		return
	}

	// Create admin user
	hash, err := auth.HashPassword(adminPassword)
	if err != nil {
		log.Printf("Failed to hash admin password: %v", err)
		return
	}

	admin := &store.User{
		ID:           "user-admin",
		Email:        adminEmail,
		PasswordHash: hash,
		Name:         "Admin",
		Role:         "admin",
		CreatedAt:    time.Now(),
	}

	if err := s.store.CreateUser(admin); err != nil {
		log.Printf("Failed to create admin user: %v", err)
		return
	}

	log.Printf("Created admin user: %s", adminEmail)
}
