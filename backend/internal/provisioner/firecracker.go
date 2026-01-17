//go:build linux

package provisioner

import (
	"context"
	"fmt"
	"time"

	"github.com/clarateach/backend/internal/orchestrator"
)

// FirecrackerProvisioner implements Provisioner using Firecracker MicroVMs.
// Unlike GCPProvider which creates one VM per workshop, this creates one MicroVM per seat.
type FirecrackerProvisioner struct {
	provider *orchestrator.FirecrackerProvider
}

// NewFirecrackerProvisioner creates a new Firecracker-based provisioner.
func NewFirecrackerProvisioner() (*FirecrackerProvisioner, error) {
	provider, err := orchestrator.NewFirecrackerProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to create Firecracker provider: %w", err)
	}
	return &FirecrackerProvisioner{provider: provider}, nil
}

// CreateVM provisions Firecracker MicroVMs for all seats in a workshop.
// Returns info about the first seat's VM for compatibility with the API.
func (f *FirecrackerProvisioner) CreateVM(ctx context.Context, cfg VMConfig) (*VMInstance, error) {
	var firstInstance *orchestrator.Instance
	var lastErr error

	// Create a MicroVM for each seat
	for seatID := 1; seatID <= cfg.Seats; seatID++ {
		instance, err := f.provider.Create(ctx, orchestrator.InstanceConfig{
			WorkshopID: cfg.WorkshopID,
			SeatID:     seatID,
		})
		if err != nil {
			lastErr = err
			continue
		}
		if firstInstance == nil {
			firstInstance = instance
		}
	}

	if firstInstance == nil {
		if lastErr != nil {
			return nil, fmt.Errorf("failed to create any VMs: %w", lastErr)
		}
		return nil, fmt.Errorf("no VMs created")
	}

	return &VMInstance{
		ID:         fmt.Sprintf("fc-%s", cfg.WorkshopID),
		Name:       fmt.Sprintf("clarateach-fc-%s", cfg.WorkshopID),
		ExternalIP: firstInstance.IP, // First VM's IP
		InternalIP: firstInstance.IP,
		Status:     "RUNNING",
		Zone:       "local",
	}, nil
}

// DeleteVM destroys all MicroVMs for a workshop.
func (f *FirecrackerProvisioner) DeleteVM(ctx context.Context, workshopID string) error {
	instances, err := f.provider.List(ctx, workshopID)
	if err != nil {
		return fmt.Errorf("failed to list VMs: %w", err)
	}

	var lastErr error
	for _, inst := range instances {
		if err := f.provider.Destroy(ctx, workshopID, inst.SeatID); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// GetVM returns info about the workshop's MicroVMs.
func (f *FirecrackerProvisioner) GetVM(ctx context.Context, workshopID string) (*VMInstance, error) {
	instances, err := f.provider.List(ctx, workshopID)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, nil
	}

	// Return first instance for compatibility
	first := instances[0]
	return &VMInstance{
		ID:         fmt.Sprintf("fc-%s-%d", workshopID, first.SeatID),
		Name:       fmt.Sprintf("clarateach-fc-%s", workshopID),
		ExternalIP: first.IP,
		InternalIP: first.IP,
		Status:     "RUNNING",
		Zone:       "local",
	}, nil
}

// WaitForReady waits for MicroVMs to be ready.
// Firecracker VMs start very fast (~125ms), so this is mostly a no-op.
func (f *FirecrackerProvisioner) WaitForReady(ctx context.Context, workshopID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		instances, err := f.provider.List(ctx, workshopID)
		if err != nil {
			return err
		}
		if len(instances) > 0 {
			return nil // At least one VM is running
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for VMs to be ready")
}

// ListVMs returns all MicroVMs, optionally filtered by workshop.
func (f *FirecrackerProvisioner) ListVMs(ctx context.Context, workshopID string) ([]*VMInstance, error) {
	instances, err := f.provider.List(ctx, workshopID)
	if err != nil {
		return nil, err
	}

	var result []*VMInstance
	for _, inst := range instances {
		result = append(result, &VMInstance{
			ID:         fmt.Sprintf("fc-%s-%d", inst.WorkshopID, inst.SeatID),
			Name:       fmt.Sprintf("clarateach-fc-%s-seat%d", inst.WorkshopID, inst.SeatID),
			ExternalIP: inst.IP,
			InternalIP: inst.IP,
			Status:     "RUNNING",
			Zone:       "local",
		})
	}
	return result, nil
}

// GetSeatIP returns the IP for a specific seat.
func (f *FirecrackerProvisioner) GetSeatIP(ctx context.Context, workshopID string, seatID int) (string, error) {
	return f.provider.GetIP(ctx, workshopID, seatID)
}
