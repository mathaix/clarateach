package config

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// Config holds all application configuration
type Config struct {
	// Server
	Port string

	// Database
	DatabaseURL string

	// GCP
	GCPProject  string
	GCPZone     string
	GCPRegistry string
	GCPUseSpot  bool

	// Firecracker
	FCSnapshotName       string
	FCAgentToken         string
	BackendURL           string
	WorkspaceTokenSecret string

	// CORS
	CORSOrigins []string
}

// Load loads configuration from GCP Secret Manager with fallback to environment variables
func Load() (*Config, error) {
	// Try to load .env file first (for local development)
	loadEnvFile(".env")

	gcpProject := getEnv("GCP_PROJECT", "")

	cfg := &Config{
		Port:                 getEnv("PORT", "8080"),
		GCPProject:           gcpProject,
		GCPZone:              getEnv("GCP_ZONE", "us-central1-a"),
		GCPRegistry:          getEnv("GCP_REGISTRY", ""),
		GCPUseSpot:           getEnv("GCP_USE_SPOT", "") == "true",
		FCSnapshotName:       getEnv("FC_SNAPSHOT_NAME", ""),
		FCAgentToken:         getEnv("FC_AGENT_TOKEN", ""),
		BackendURL:           getEnv("BACKEND_URL", ""),
		WorkspaceTokenSecret: getEnv("WORKSPACE_TOKEN_SECRET", ""),
	}

	// Load DATABASE_URL - try Secret Manager first, then env
	databaseURL, err := getSecret(gcpProject, "DATABASE_URL")
	if err != nil || databaseURL == "" {
		databaseURL = getEnv("DATABASE_URL", "")
	}
	cfg.DatabaseURL = databaseURL

	// Load sensitive configs from Secret Manager with env fallback
	if token, err := getSecret(gcpProject, "FC_AGENT_TOKEN"); err == nil && token != "" {
		cfg.FCAgentToken = token
	}
	if secret, err := getSecret(gcpProject, "WORKSPACE_TOKEN_SECRET"); err == nil && secret != "" {
		cfg.WorkspaceTokenSecret = secret
	}

	// Parse CORS origins
	corsOrigins := getEnv("CORS_ORIGINS", "*")
	if corsOrigins == "" || corsOrigins == "*" {
		cfg.CORSOrigins = []string{"*"}
	} else {
		origins := strings.Split(corsOrigins, ",")
		for i, origin := range origins {
			origins[i] = strings.TrimSpace(origin)
		}
		cfg.CORSOrigins = origins
	}

	return cfg, nil
}

// Validate checks that required configuration is present
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required (set via GCP Secret Manager or environment)")
	}
	if c.GCPProject == "" {
		return fmt.Errorf("GCP_PROJECT is required")
	}
	if c.GCPRegistry == "" {
		return fmt.Errorf("GCP_REGISTRY is required")
	}
	return nil
}

// getSecret retrieves a secret from GCP Secret Manager
// Returns empty string and nil error if Secret Manager is not available
func getSecret(project, secretName string) (string, error) {
	if project == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		log.Printf("Secret Manager client creation failed (falling back to env): %v", err)
		return "", nil
	}
	defer client.Close()

	// Access the latest version of the secret
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", project, secretName)
	result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		// Don't log as error - secret may not exist, which is fine (fallback to env)
		return "", nil
	}

	return string(result.Payload.Data), nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// loadEnvFile loads environment variables from a .env file
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		// .env file is optional
		return
	}
	defer file.Close()

	log.Printf("Loading environment from %s", filename)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		value = strings.Trim(value, `"'`)

		// Only set if not already set in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
