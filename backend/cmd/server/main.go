package main

import (
	"log"
	"net/http"

	"github.com/clarateach/backend/internal/api"
	"github.com/clarateach/backend/internal/config"
	"github.com/clarateach/backend/internal/provisioner"
	"github.com/clarateach/backend/internal/store"
	"github.com/go-chi/cors"
)

func main() {
	// 1. Load Configuration (GCP Secret Manager -> env -> .env)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	// 2. Initialize Store (PostgreSQL)
	db, err := store.InitPostgresDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	st := store.NewPostgresStore(db)

	// 3. Initialize GCP Provisioner
	log.Printf("GCP provisioning: project=%s, zone=%s, registry=%s", cfg.GCPProject, cfg.GCPZone, cfg.GCPRegistry)
	vmProvisioner := provisioner.NewGCPProvider(provisioner.GCPConfig{
		Project:     cfg.GCPProject,
		Zone:        cfg.GCPZone,
		RegistryURL: cfg.GCPRegistry,
	})

	// 4. Initialize API Server
	apiServer := api.NewServer(st, vmProvisioner, cfg.GCPUseSpot)

	// 5. Initialize GCP Firecracker Provisioner (optional)
	if cfg.FCSnapshotName != "" {
		log.Printf("Initializing GCP Firecracker provisioner with snapshot: %s", cfg.FCSnapshotName)
		fcProvisioner := provisioner.NewGCPFirecrackerProvider(provisioner.GCPFirecrackerConfig{
			Project:              cfg.GCPProject,
			Zone:                 cfg.GCPZone,
			SnapshotName:         cfg.FCSnapshotName,
			AgentToken:           cfg.FCAgentToken,
			BackendURL:           cfg.BackendURL,
			WorkspaceTokenSecret: cfg.WorkspaceTokenSecret,
		})
		apiServer.SetGCPFirecrackerProvisioner(fcProvisioner, cfg.FCSnapshotName)
	}

	// 6. CORS Middleware
	log.Printf("CORS allowed origins: %v", cfg.CORSOrigins)
	corsHandler := cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	// 7. Root Handler
	rootHandler := corsHandler(apiServer)

	log.Printf("ClaraTeach Backend running on port %s", cfg.Port)
	log.Printf("Database: PostgreSQL (connected)")

	if err := http.ListenAndServe(":"+cfg.Port, rootHandler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
