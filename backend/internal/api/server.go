package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/clarateach/backend/internal/orchestrator"
	"github.com/clarateach/backend/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Server struct {
	router       *chi.Mux
	store        store.Store
	orchestrator orchestrator.Provider
	baseDomain   string
}

func NewServer(store store.Store, orch orchestrator.Provider, baseDomain string) *Server {
	s := &Server{
		store:        store,
		orchestrator: orch,
		router:       chi.NewRouter(),
		baseDomain:   baseDomain,
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
				r.Post("/start", s.startWorkshop) // Provision VM/Network
			})
		})
		r.Post("/join", s.joinWorkshop)
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
		Status:    "running",
		CreatedAt: time.Now(),
	}

	if err := s.store.CreateWorkshop(workshop); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Pre-provision containers (Synchronous)
	ctx := context.Background()
	fmt.Printf(">>> Starting Provisioning for Workshop %s (%s) - %d seats\n", workshop.Name, workshop.ID, workshop.Seats)
	
	// 1. Create Placeholder Sessions
	for i := 1; i <= workshop.Seats; i++ {
		session := &store.Session{
			OdeHash:    generateID(5),
			WorkshopID: workshop.ID,
			SeatID:     i,
			Status:     "provisioning",
			JoinedAt:   time.Now(),
		}
		if err := s.store.CreateSession(session); err != nil {
			fmt.Printf("    [Seat %d] Failed to create session record: %v\n", i, err)
			http.Error(w, fmt.Sprintf("Failed to init session %d: %v", i, err), http.StatusInternalServerError)
			return
		}
	}

	// 2. Provision Containers
	for i := 1; i <= workshop.Seats; i++ {
		fmt.Printf("    [Seat %d] Creating container...\n", i)
		instance, err := s.orchestrator.Create(ctx, orchestrator.InstanceConfig{
			WorkshopID: workshop.ID,
			SeatID:     i,
			Image:      "clarateach-workspace",
			ApiKey:     workshop.ApiKey,
		})
		if err != nil {
			fmt.Printf("    [Seat %d] ERROR: %v\n", i, err)
			// TODO: Rollback/Cleanup previous containers?
			http.Error(w, fmt.Sprintf("Failed to provision seat %d: %v", i, err), http.StatusInternalServerError)
			return
		} else {
			fmt.Printf("    [Seat %d] READY at %s (ID: %s)\n", i, instance.IP, instance.ID[:12])
			// Update Session
			sess, _ := s.store.GetSessionBySeat(workshop.ID, i)
			if sess != nil {
				sess.Status = "ready"
				sess.IP = instance.IP
				sess.ContainerID = instance.ID
				s.store.UpdateSession(sess)
			}
		}
	}
	fmt.Printf(">>> Workshop %s fully provisioned.\n", workshop.ID)

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
	json.NewEncoder(w).Encode(map[string]interface{}{"workshop": workshop})
}

func (s *Server) deleteWorkshop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// TODO: Orchestrator destroy logic
	if err := s.store.DeleteWorkshop(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) startWorkshop(w http.ResponseWriter, r *http.Request) {
	// In Docker mode, "start" might just mean "ready to accept".
	// The containers are lazy-provisioned on join.
	// Or we can pre-provision here.
	id := chi.URLParam(r, "id")
	if err := s.store.UpdateWorkshopStatus(id, "running"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
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
		// Allocate Seat
		// Logic: Find first available session (provisioning or ready) that has no user assigned
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
		session.Name = req.Name // req.Name might be empty if optional, handle logic? 
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

	// 3. Ensure Container Running (Idempotent)
	// If session was "provisioning", we might need to wait or just trigger create again to be safe
	instance, err := s.orchestrator.Create(r.Context(), orchestrator.InstanceConfig{
		WorkshopID: workshop.ID,
		SeatID:     session.SeatID,
		Image:      "clarateach-workspace", // Env var?
		ApiKey:     workshop.ApiKey,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to provision workspace: %v", err), http.StatusInternalServerError)
		return
	}

	// Update IP if it changed (e.g. restart)
	if instance.IP != session.IP {
		session.IP = instance.IP
		session.ContainerID = instance.ID
		s.store.UpdateSession(session)
	}

	// 4. Return Access Token & Info
	// TODO: Generate JWT
	
	var endpoint string
	if s.baseDomain == "localhost" {
		endpoint = fmt.Sprintf("http://localhost:8080/debug/proxy/%s", workshop.ID)
	} else {
		endpoint = fmt.Sprintf("https://%s.%s", workshop.ID, s.baseDomain)
	}

	resp := map[string]interface{}{
		"workshop_id": workshop.ID,
		"seat":        session.SeatID,
		"odehash":     session.OdeHash,
		"endpoint":    endpoint,
		"ip":          instance.IP,
	}
	json.NewEncoder(w).Encode(resp)
}

// Utils (Mock)

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
