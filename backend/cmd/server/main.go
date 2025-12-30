package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/clarateach/backend/internal/api"
	"github.com/clarateach/backend/internal/orchestrator"
	"github.com/clarateach/backend/internal/proxy"
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
	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "clarateach.io"
	}

	// 1. Initialize Store
	db, err := store.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	st := store.NewSQLiteStore(db)

	// 2. Initialize Orchestrator
	orch, err := orchestrator.NewDockerProvider()
	if err != nil {
		log.Fatalf("Failed to init Orchestrator: %v", err)
	}

	// 3. Initialize API Server
	apiServer := api.NewServer(st, orch, baseDomain)

	// 4. Initialize Proxy
	proxyServer := proxy.NewDynamicProxy(st, orch, baseDomain)

	// CORS Middleware
	corsHandler := cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	})

	// 5. Root Handler (Routing Logic)
	// We check if the Host is "api.clarateach.io" or "ws-*.clarateach.io"
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if idx := strings.Index(host, ":"); idx != -1 {
				host = host[:idx]
			}

			// Debug Proxy for Localhost Testing
			// Usage: http://localhost:8080/debug/proxy/<workshop_id>/vm/<seat>/...
			if strings.HasPrefix(r.URL.Path, "/debug/proxy/") {
				parts := strings.Split(r.URL.Path, "/")
				if len(parts) >= 4 {
					// parts[0]="" parts[1]="debug" parts[2]="proxy" parts[3]="ws-id"
					workshopID := parts[3]
					realPath := "/" + strings.Join(parts[4:], "/")
					
					// Mock the request for the proxy
					r.Host = workshopID + "." + baseDomain
					r.URL.Path = realPath
					proxyServer.ServeHTTP(w, r)
					return
				}
			}

			if host == "api."+baseDomain || host == "localhost" {
				apiServer.ServeHTTP(w, r)
			} else if strings.HasSuffix(host, "."+baseDomain) {
				proxyServer.ServeHTTP(w, r)
			} else {
				// Fallback (maybe landing page?)
				apiServer.ServeHTTP(w, r)
			}
		})).ServeHTTP(w, r)
	})

	log.Printf("ClaraTeach Backend running on port %s", port)
	log.Printf("Database: %s", dbPath)
	log.Printf("Base Domain: %s", baseDomain)
	
	if err := http.ListenAndServe(":"+port, rootHandler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
