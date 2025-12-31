package orchestrator

import (
	"context"
)

// InstanceConfig holds the configuration for a new instance.
type InstanceConfig struct {
	WorkshopID string
	SeatID     int
	// Add other configuration parameters as needed, e.g., ImageID, ResourceLimits
}

// Instance represents a running instance (either Docker container or Firecracker MicroVM).
type Instance struct {
	WorkshopID string
	SeatID     int
	IP         string
	// Add other instance details as needed, e.g., ProcessID, NetworkInterface
}

// Provider defines the interface for provisioning and managing instances.
type Provider interface {
	Create(ctx context.Context, cfg InstanceConfig) (*Instance, error)
	Destroy(ctx context.Context, workshopID string, seatID int) error
	List(ctx context.Context, workshopID string) ([]*Instance, error)
	GetIP(ctx context.Context, workshopID string, seatID int) (string, error)
}
