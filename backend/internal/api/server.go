package api

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/clarateach/backend/internal/provisioner"
	"github.com/clarateach/backend/internal/sshutil"
	"github.com/clarateach/backend/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Server struct {
	router       *chi.Mux
	store        store.Store
	provisioner  provisioner.Provisioner
	useSpotVMs   bool
	authDisabled bool
}

func NewServer(store store.Store, prov provisioner.Provisioner, useSpotVMs bool, authDisabled bool) *Server {
	s := &Server{
		store:        store,
		provisioner:  prov,
		router:       chi.NewRouter(),
		useSpotVMs:   useSpotVMs,
		authDisabled: authDisabled,
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"}, // In prod, be more restrictive
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	s.router.Route("/api", func(r chi.Router) {
		r.Route("/workshops", func(r chi.Router) {
			r.Get("/", s.listWorkshops)
			r.Post("/", s.createWorkshop)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.getWorkshop)
				r.Delete("/", s.deleteWorkshop)
				r.Post("/start", s.startWorkshop)
			})
		})
		r.Post("/join", s.joinWorkshop)

		// Admin API for VM management
		r.Route("/admin", func(r chi.Router) {
			r.Get("/overview", s.adminOverview)
			r.Get("/vms", s.listVMs)
			r.Get("/vms/{workshop_id}", s.getVMDetails)
			r.Get("/vms/{workshop_id}/ssh-key", s.getSSHKey)
		})
	})
}

// Handlers

func (s *Server) listWorkshops(w http.ResponseWriter, r *http.Request) {
	workshops, err := s.store.ListWorkshops()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"workshops": workshops})
}

func (s *Server) createWorkshop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Seats  int    `json:"seats"`
		ApiKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	workshop := &store.Workshop{
		ID:        "ws-" + generateID(6),
		Name:      req.Name,
		Code:      generateCode(),
		Seats:     req.Seats,
		ApiKey:    req.ApiKey,
		Status:    "created",
		CreatedAt: time.Now(),
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

	// Provision GCP VM
	log.Printf("Provisioning GCP VM for workshop %s (%s) with %d seats", workshop.Name, workshop.ID, workshop.Seats)
	s.store.UpdateWorkshopStatus(workshop.ID, "provisioning")

	// Generate SSH key pair for debugging access
	keyPair, err := sshutil.GenerateKeyPair(fmt.Sprintf("clarateach-%s", workshop.ID))
	if err != nil {
		log.Printf("Failed to generate SSH key: %v", err)
		s.store.UpdateWorkshopStatus(workshop.ID, "error")
		http.Error(w, fmt.Sprintf("Failed to generate SSH key: %v", err), http.StatusInternalServerError)
		return
	}

	// Create VM config
	vmConfig := provisioner.DefaultConfig(workshop.ID, workshop.Seats)
	vmConfig.Spot = s.useSpotVMs
	vmConfig.SSHPublicKey = keyPair.PublicKey
	vmConfig.AuthDisabled = s.authDisabled

	// Track provisioning time
	provisioningStartedAt := time.Now()

	// Provision VM
	ctx := r.Context()
	vmInstance, err := s.provisioner.CreateVM(ctx, vmConfig)
	if err != nil {
		log.Printf("Failed to provision VM: %v", err)
		s.store.UpdateWorkshopStatus(workshop.ID, "error")
		http.Error(w, fmt.Sprintf("Failed to provision VM: %v", err), http.StatusInternalServerError)
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
	workshop.Status = "running"

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"workshop": workshop})
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
	if vm, err := s.provisioner.GetVM(ctx, id); err == nil && vm != nil {
		resp["vm_ip"] = vm.ExternalIP
		resp["vm_status"] = vm.Status
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"workshop": resp})
}

func (s *Server) deleteWorkshop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Delete the GCP VM
	log.Printf("Deleting GCP VM for workshop %s", id)
	ctx := r.Context()
	if err := s.provisioner.DeleteVM(ctx, id); err != nil {
		log.Printf("Failed to delete VM (continuing with workshop deletion): %v", err)
	} else {
		log.Printf("VM deleted successfully for workshop %s", id)
	}

	// Mark VM as removed in database (soft delete)
	if err := s.store.MarkVMRemoved(id); err != nil {
		log.Printf("Failed to mark VM as removed: %v", err)
	}

	// Delete workshop and sessions from database
	if err := s.store.DeleteWorkshop(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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

	// Track provisioning time
	provisioningStartedAt := time.Now()

	// Provision VM
	ctx := r.Context()
	vmInstance, err := s.provisioner.CreateVM(ctx, vmConfig)
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
