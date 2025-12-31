package provisioner

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockProvider implements Provisioner for local development and testing
type MockProvider struct {
	mu      sync.RWMutex
	vms     map[string]*VMInstance
	counter int
}

// NewMockProvider creates a new mock provisioner
func NewMockProvider() *MockProvider {
	return &MockProvider{
		vms: make(map[string]*VMInstance),
	}
}

func (m *MockProvider) CreateVM(ctx context.Context, cfg VMConfig) (*VMInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counter++
	vmName := fmt.Sprintf("clarateach-ws-%s", cfg.WorkshopID)

	vm := &VMInstance{
		ID:         fmt.Sprintf("mock-%d", m.counter),
		Name:       vmName,
		ExternalIP: fmt.Sprintf("10.0.0.%d", m.counter),
		InternalIP: fmt.Sprintf("192.168.1.%d", m.counter),
		Status:     "RUNNING",
		Zone:       "mock-zone-1",
		SelfLink:   fmt.Sprintf("https://compute.googleapis.com/compute/v1/projects/mock/zones/mock-zone-1/instances/%s", vmName),
	}

	m.vms[cfg.WorkshopID] = vm
	return vm, nil
}

func (m *MockProvider) DeleteVM(ctx context.Context, workshopID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.vms, workshopID)
	return nil
}

func (m *MockProvider) GetVM(ctx context.Context, workshopID string) (*VMInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vm, ok := m.vms[workshopID]
	if !ok {
		return nil, fmt.Errorf("VM not found for workshop %s", workshopID)
	}
	return vm, nil
}

func (m *MockProvider) WaitForReady(ctx context.Context, workshopID string, timeout time.Duration) error {
	// Mock VMs are always ready immediately
	_, err := m.GetVM(ctx, workshopID)
	return err
}

func (m *MockProvider) ListVMs(ctx context.Context, workshopID string) ([]*VMInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var vms []*VMInstance
	for wsID, vm := range m.vms {
		if workshopID == "" || wsID == workshopID {
			vms = append(vms, vm)
		}
	}
	return vms, nil
}
