package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/clarateach/backend/internal/api"
	"github.com/clarateach/backend/internal/provisioner"
	"github.com/clarateach/backend/internal/store"
	"github.com/go-chi/cors"
)

func main() {
	// Configuration
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./clarateach.db"
	}

	// GCP Configuration (required)
	gcpProject := os.Getenv("GCP_PROJECT")
	if gcpProject == "" {
		log.Fatalf("GCP_PROJECT environment variable is required")
	}
	gcpZone := os.Getenv("GCP_ZONE")
	if gcpZone == "" {
		gcpZone = "us-central1-a"
	}
	gcpRegistry := os.Getenv("GCP_REGISTRY") // e.g., "us-central1-docker.pkg.dev/PROJECT/clarateach"
	if gcpRegistry == "" {
		log.Fatalf("GCP_REGISTRY environment variable is required")
	}
	useSpotVMs := os.Getenv("GCP_USE_SPOT") == "true"
	authDisabled := os.Getenv("AUTH_DISABLED") == "true"

	// Firecracker configuration (optional)
	fcSnapshotName := os.Getenv("FC_SNAPSHOT_NAME")       // e.g., "clara2-snapshot"
	fcAgentToken := os.Getenv("FC_AGENT_TOKEN")           // Token for agent authentication
	backendURL := os.Getenv("BACKEND_URL")                // e.g., "https://learn.claramap.com"
	workspaceTokenSecret := os.Getenv("WORKSPACE_TOKEN_SECRET")

	// 1. Initialize Store
	db, err := store.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	st := store.NewSQLiteStore(db)

	// 2. Initialize GCP Provisioner
	log.Printf("GCP provisioning: project=%s, zone=%s, registry=%s", gcpProject, gcpZone, gcpRegistry)
	vmProvisioner := provisioner.NewGCPProvider(provisioner.GCPConfig{
		Project:     gcpProject,
		Zone:        gcpZone,
		RegistryURL: gcpRegistry,
	})

	// 3. Initialize API Server
	apiServer := api.NewServer(st, vmProvisioner, useSpotVMs, authDisabled)

	// 4. Initialize GCP Firecracker Provisioner (optional)
	if fcSnapshotName != "" {
		log.Printf("Initializing GCP Firecracker provisioner with snapshot: %s", fcSnapshotName)
		fcProvisioner := provisioner.NewGCPFirecrackerProvider(provisioner.GCPFirecrackerConfig{
			Project:              gcpProject,
			Zone:                 gcpZone,
			SnapshotName:         fcSnapshotName,
			AgentToken:           fcAgentToken,
			BackendURL:           backendURL,
			WorkspaceTokenSecret: workspaceTokenSecret,
		})
		apiServer.SetGCPFirecrackerProvisioner(fcProvisioner, fcSnapshotName)
	}

	// 5. CORS Middleware
	// CORS_ORIGINS: comma-separated list (e.g., "https://learn.claramap.com,http://localhost:5173")
	// Defaults to "*" for development
	corsOrigins := os.Getenv("CORS_ORIGINS")
	var allowedOrigins []string
	if corsOrigins == "" || corsOrigins == "*" {
		allowedOrigins = []string{"*"}
	} else {
		allowedOrigins = strings.Split(corsOrigins, ",")
		for i, origin := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(origin)
		}
	}
	log.Printf("CORS allowed origins: %v", allowedOrigins)

	corsHandler := cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	// 6. Root Handler
	rootHandler := corsHandler(apiServer)

	log.Printf("ClaraTeach Backend running on port %s", port)
	log.Printf("Database: %s", dbPath)
	log.Printf("Auth Disabled: %v", authDisabled)

	if err := http.ListenAndServe(":"+port, rootHandler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
