package orchestrator

import (
	"context"
)

type InstanceConfig struct {
	WorkshopID string
	SeatID     int
	Image      string
	ApiKey     string
	Network    string
}

type Instance struct {
	ID        string `json:"id"`
	IP        string `json:"ip"`
	Status    string `json:"status"` // "running", "stopped", "unknown"

	// Host port mappings for local development (macOS can't route to container IPs)
	HostTerminalPort int `json:"host_terminal_port,omitempty"` // Maps to container:3001
	HostFilesPort    int `json:"host_files_port,omitempty"`    // Maps to container:3002
	HostBrowserPort  int `json:"host_browser_port,omitempty"`  // Maps to container:3003 (neko)
}

// Provider defines the interface for creating and managing learner environments
type Provider interface {
	// Create provisions a new instance (Container or MicroVM)
	Create(ctx context.Context, cfg InstanceConfig) (*Instance, error)

	// Destroy removes an instance
	Destroy(ctx context.Context, workshopID string, seatID int) error

	// List returns all instances for a workshop
	List(ctx context.Context, workshopID string) ([]*Instance, error)

	// GetIP returns the IP address of a specific seat's instance
	GetIP(ctx context.Context, workshopID string, seatID int) (string, error)

	// GetBrowserIP returns the IP address of the browser sidecar
	GetBrowserIP(ctx context.Context, workshopID string, seatID int) (string, error)

	// GetInstance returns full instance details including host ports (for local dev)
	GetInstance(ctx context.Context, workshopID string, seatID int) (*Instance, error)
}
