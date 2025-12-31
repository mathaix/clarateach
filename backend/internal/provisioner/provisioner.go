package provisioner

import (
	"context"
	"time"
)

// VMConfig holds the configuration for creating a VM
type VMConfig struct {
	WorkshopID     string
	Seats          int
	Image          string // Artifact Registry image URL
	MachineType    string // e.g., "e2-standard-4"
	DiskSizeGB     int
	Spot           bool   // Use spot/preemptible VMs
	SSHPublicKey   string // Optional SSH public key for debugging
	EnableOpsAgent bool   // Install Google Cloud Ops Agent
	AuthDisabled   bool   // Disable JWT auth in workspace containers
}

// VMInstance represents a provisioned VM
type VMInstance struct {
	ID         string `json:"id"`          // GCE instance ID (numeric)
	Name       string `json:"name"`        // Instance name (clarateach-ws-xxxxx)
	ExternalIP string `json:"external_ip"` // Public IP address
	InternalIP string `json:"internal_ip"` // Private IP address
	Status     string `json:"status"`      // RUNNING, TERMINATED, etc.
	Zone       string `json:"zone"`
	SelfLink   string `json:"self_link"`   // Full resource URL
}

// Provisioner defines the interface for VM lifecycle management
type Provisioner interface {
	// CreateVM provisions a new GCE VM for a workshop
	CreateVM(ctx context.Context, cfg VMConfig) (*VMInstance, error)

	// DeleteVM terminates and removes a workshop VM
	DeleteVM(ctx context.Context, workshopID string) error

	// GetVM returns the current state of a workshop VM
	GetVM(ctx context.Context, workshopID string) (*VMInstance, error)

	// WaitForReady blocks until the VM is ready to accept connections
	WaitForReady(ctx context.Context, workshopID string, timeout time.Duration) error

	// ListVMs returns all ClaraTeach VMs (optionally filtered by workshop)
	ListVMs(ctx context.Context, workshopID string) ([]*VMInstance, error)
}

// DefaultConfig returns sensible defaults for VM configuration
func DefaultConfig(workshopID string, seats int) VMConfig {
	return VMConfig{
		WorkshopID:     workshopID,
		Seats:          seats,
		MachineType:    "e2-standard-4",
		DiskSizeGB:     50,
		Spot:           false,
		EnableOpsAgent: false, // COS doesn't support Ops Agent installation
	}
}
