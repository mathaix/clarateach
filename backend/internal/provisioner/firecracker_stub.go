//go:build !linux

package provisioner

import (
	"context"
	"fmt"
	"time"
)

// FirecrackerProvisioner is a stub for non-Linux platforms.
// The real implementation requires Linux with KVM support.
type FirecrackerProvisioner struct{}

// NewFirecrackerProvisioner returns an error on non-Linux platforms.
func NewFirecrackerProvisioner() (*FirecrackerProvisioner, error) {
	return nil, fmt.Errorf("Firecracker provisioner requires Linux with KVM support")
}

func (f *FirecrackerProvisioner) CreateVM(ctx context.Context, cfg VMConfig) (*VMInstance, error) {
	return nil, fmt.Errorf("Firecracker not supported on this platform")
}

func (f *FirecrackerProvisioner) DeleteVM(ctx context.Context, workshopID string) error {
	return fmt.Errorf("Firecracker not supported on this platform")
}

func (f *FirecrackerProvisioner) GetVM(ctx context.Context, workshopID string) (*VMInstance, error) {
	return nil, fmt.Errorf("Firecracker not supported on this platform")
}

func (f *FirecrackerProvisioner) WaitForReady(ctx context.Context, workshopID string, timeout time.Duration) error {
	return fmt.Errorf("Firecracker not supported on this platform")
}

func (f *FirecrackerProvisioner) ListVMs(ctx context.Context, workshopID string) ([]*VMInstance, error) {
	return nil, fmt.Errorf("Firecracker not supported on this platform")
}

func (f *FirecrackerProvisioner) GetSeatIP(ctx context.Context, workshopID string, seatID int) (string, error) {
	return "", fmt.Errorf("Firecracker not supported on this platform")
}
