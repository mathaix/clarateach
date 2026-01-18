package provisioner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"
)

// GCPFirecrackerProvider implements Provisioner for GCP VMs running Firecracker MicroVMs.
// It creates a VM from a pre-baked snapshot that has the agent installed, then
// calls the agent API to create MicroVMs for each seat.
type GCPFirecrackerProvider struct {
	project              string
	zone                 string
	snapshotName         string // Snapshot with agent pre-installed (e.g., "clara2-snapshot")
	network              string
	machineType          string // Must support nested virt (n2-standard-8)
	agentPort            int
	agentToken           string
	backendURL           string // Backend URL for tunnel registration
	workspaceTokenSecret string // Secret for workspace JWT validation
	httpClient           *http.Client
}

// GCPFirecrackerConfig holds configuration for the GCP Firecracker provider
type GCPFirecrackerConfig struct {
	Project              string
	Zone                 string
	SnapshotName         string // Required: snapshot with agent pre-installed
	Network              string // default: "default"
	MachineType          string // default: "n2-standard-8"
	AgentPort            int    // default: 9090
	AgentToken           string // Token for agent authentication
	BackendURL           string // Backend URL for tunnel registration (e.g., https://learn.claramap.com)
	WorkspaceTokenSecret string // Secret for workspace JWT validation
}

// NewGCPFirecrackerProvider creates a new GCP Firecracker provisioner
func NewGCPFirecrackerProvider(cfg GCPFirecrackerConfig) *GCPFirecrackerProvider {
	if cfg.Network == "" {
		cfg.Network = "default"
	}
	if cfg.MachineType == "" {
		cfg.MachineType = "n2-standard-8"
	}
	if cfg.AgentPort == 0 {
		cfg.AgentPort = 9090
	}

	return &GCPFirecrackerProvider{
		project:              cfg.Project,
		zone:                 cfg.Zone,
		snapshotName:         cfg.SnapshotName,
		network:              cfg.Network,
		machineType:          cfg.MachineType,
		agentPort:            cfg.AgentPort,
		agentToken:           cfg.AgentToken,
		backendURL:           cfg.BackendURL,
		workspaceTokenSecret: cfg.WorkspaceTokenSecret,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// vmName generates a consistent VM name for a workshop
func (p *GCPFirecrackerProvider) vmName(workshopID string) string {
	return fmt.Sprintf("clarateach-fc-%s", workshopID)
}

// CreateVM provisions a new GCE VM from snapshot and creates MicroVMs via the agent
func (p *GCPFirecrackerProvider) CreateVM(ctx context.Context, cfg VMConfig) (*VMInstance, error) {
	// Step 1: Create GCP VM from snapshot
	vm, err := p.createGCPVM(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP VM: %w", err)
	}

	vmName := p.vmName(cfg.WorkshopID)

	// Step 2: Wait for agent to be healthy
	agentURL := fmt.Sprintf("http://%s:%d", vm.ExternalIP, p.agentPort)
	if err := p.waitForAgentHealth(ctx, agentURL, 2*time.Minute); err != nil {
		// Don't delete VM on failure - keep it for debugging
		// Check serial console: gcloud compute instances get-serial-port-output %s --zone=%s
		return nil, fmt.Errorf("agent health check failed (VM %s kept for debugging): %w", vmName, err)
	}

	// Step 3: Create MicroVMs for each seat
	if err := p.createMicroVMs(ctx, agentURL, cfg.WorkshopID, cfg.Seats); err != nil {
		// Don't delete VM on failure - keep it for debugging
		return nil, fmt.Errorf("failed to create MicroVMs (VM %s kept for debugging): %w", vmName, err)
	}

	return vm, nil
}

// createGCPVM creates the GCE instance from snapshot with nested virtualization
func (p *GCPFirecrackerProvider) createGCPVM(ctx context.Context, cfg VMConfig) (*VMInstance, error) {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}
	defer client.Close()

	vmName := p.vmName(cfg.WorkshopID)

	// Build metadata
	metadata := []*computepb.Items{
		{Key: proto.String("workshop-id"), Value: proto.String(cfg.WorkshopID)},
		{Key: proto.String("seats"), Value: proto.String(strconv.Itoa(cfg.Seats))},
		{Key: proto.String("agent-token"), Value: proto.String(p.agentToken)},
	}

	// Add tunnel-related metadata if configured
	if p.backendURL != "" {
		metadata = append(metadata, &computepb.Items{
			Key:   proto.String("backend-url"),
			Value: proto.String(p.backendURL),
		})
	}
	if p.workspaceTokenSecret != "" {
		metadata = append(metadata, &computepb.Items{
			Key:   proto.String("workspace-token-secret"),
			Value: proto.String(p.workspaceTokenSecret),
		})
	}

	// Add SSH key if provided
	if cfg.SSHPublicKey != "" {
		sshKeyEntry := fmt.Sprintf("clarateach:%s", cfg.SSHPublicKey)
		metadata = append(metadata, &computepb.Items{
			Key:   proto.String("ssh-keys"),
			Value: proto.String(sshKeyEntry),
		})
	}

	// Scheduling config (spot vs on-demand)
	// cfg.Spot is set by the server based on configuration
	scheduling := &computepb.Scheduling{
		AutomaticRestart:  proto.Bool(!cfg.Spot),
		OnHostMaintenance: proto.String("TERMINATE"), // Required for nested virtualization
	}
	if cfg.Spot {
		scheduling.Preemptible = proto.Bool(true)
		scheduling.ProvisioningModel = proto.String("SPOT")
		scheduling.InstanceTerminationAction = proto.String("STOP")
	}

	// Build instance with nested virtualization enabled
	instance := &computepb.Instance{
		Name:        proto.String(vmName),
		MachineType: proto.String(fmt.Sprintf("zones/%s/machineTypes/%s", p.zone, p.machineType)),
		// Enable nested virtualization
		AdvancedMachineFeatures: &computepb.AdvancedMachineFeatures{
			EnableNestedVirtualization: proto.Bool(true),
		},
		Disks: []*computepb.AttachedDisk{
			{
				Boot:       proto.Bool(true),
				AutoDelete: proto.Bool(true),
				InitializeParams: &computepb.AttachedDiskInitializeParams{
					SourceSnapshot: proto.String(fmt.Sprintf("projects/%s/global/snapshots/%s", p.project, p.snapshotName)),
					DiskSizeGb:     proto.Int64(int64(cfg.DiskSizeGB)),
					DiskType:       proto.String(fmt.Sprintf("zones/%s/diskTypes/pd-balanced", p.zone)),
				},
			},
		},
		NetworkInterfaces: []*computepb.NetworkInterface{
			{
				Network: proto.String(fmt.Sprintf("global/networks/%s", p.network)),
				AccessConfigs: []*computepb.AccessConfig{
					{
						Name:        proto.String("External NAT"),
						Type:        proto.String("ONE_TO_ONE_NAT"),
						NetworkTier: proto.String("PREMIUM"),
					},
				},
			},
		},
		Metadata: &computepb.Metadata{
			Items: metadata,
		},
		Scheduling: scheduling,
		Labels: map[string]string{
			"clarateach":          "true",
			"clarateach-workshop": cfg.WorkshopID,
			"clarateach-runtime":  "firecracker",
			"managed-by":          "clarateach-backend",
		},
		Tags: &computepb.Tags{
			Items: []string{"clarateach", "clarateach-agent"},
		},
		ServiceAccounts: []*computepb.ServiceAccount{
			{
				Email: proto.String("default"),
				Scopes: []string{
					"https://www.googleapis.com/auth/compute.readonly",
					"https://www.googleapis.com/auth/logging.write",
					"https://www.googleapis.com/auth/monitoring.write",
				},
			},
		},
	}

	// Create the VM
	op, err := client.Insert(ctx, &computepb.InsertInstanceRequest{
		Project:          p.project,
		Zone:             p.zone,
		InstanceResource: instance,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	// Wait for operation to complete
	if err := op.Wait(ctx); err != nil {
		return nil, fmt.Errorf("failed waiting for VM creation: %w", err)
	}

	// Get the created instance details (includes external IP)
	return p.GetVM(ctx, cfg.WorkshopID)
}

// waitForAgentHealth polls the agent health endpoint until it responds
func (p *GCPFirecrackerProvider) waitForAgentHealth(ctx context.Context, agentURL string, timeout time.Duration) error {
	healthURL := fmt.Sprintf("%s/health", agentURL)
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Initial delay to let VM boot
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for agent to become healthy")
			}

			resp, err := p.httpClient.Get(healthURL)
			if err != nil {
				continue // Agent not ready yet
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil // Agent is healthy
			}
		}
	}
}

// createMicroVMs calls the agent API to create a MicroVM for each seat
func (p *GCPFirecrackerProvider) createMicroVMs(ctx context.Context, agentURL string, workshopID string, seats int) error {
	createURL := fmt.Sprintf("%s/vms", agentURL)

	for seatID := 1; seatID <= seats; seatID++ {
		reqBody := map[string]interface{}{
			"workshop_id": workshopID,
			"seat_id":     seatID,
		}
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request for seat %d: %w", seatID, err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to create request for seat %d: %w", seatID, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.agentToken))

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to create MicroVM for seat %d: %w", seatID, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("failed to create MicroVM for seat %d: status %d, body: %s", seatID, resp.StatusCode, string(body))
		}
	}

	return nil
}

// DeleteVM destroys all MicroVMs and deletes the GCP VM
func (p *GCPFirecrackerProvider) DeleteVM(ctx context.Context, workshopID string) error {
	// First, try to get the VM to get its IP for agent cleanup
	vm, err := p.GetVM(ctx, workshopID)
	if err == nil && vm.ExternalIP != "" {
		// Try to clean up MicroVMs via agent (best effort)
		agentURL := fmt.Sprintf("http://%s:%d", vm.ExternalIP, p.agentPort)
		_ = p.destroyMicroVMs(ctx, agentURL, workshopID)
	}

	// Delete the GCP VM
	return p.deleteGCPVM(ctx, workshopID)
}

// destroyMicroVMs calls the agent API to destroy all MicroVMs for a workshop
func (p *GCPFirecrackerProvider) destroyMicroVMs(ctx context.Context, agentURL string, workshopID string) error {
	// List VMs to get seat IDs
	listURL := fmt.Sprintf("%s/vms?workshop_id=%s", agentURL, workshopID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.agentToken))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to list VMs: status %d", resp.StatusCode)
	}

	var listResp struct {
		VMs []struct {
			SeatID int `json:"seat_id"`
		} `json:"vms"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return err
	}

	// Delete each VM
	for _, vm := range listResp.VMs {
		deleteURL := fmt.Sprintf("%s/vms/%s/%d", agentURL, workshopID, vm.SeatID)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.agentToken))

		resp, err := p.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
	}

	return nil
}

// deleteGCPVM deletes the GCE instance
func (p *GCPFirecrackerProvider) deleteGCPVM(ctx context.Context, workshopID string) error {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	defer client.Close()

	vmName := p.vmName(workshopID)

	op, err := client.Delete(ctx, &computepb.DeleteInstanceRequest{
		Project:  p.project,
		Zone:     p.zone,
		Instance: vmName,
	})
	if err != nil {
		// If not found, consider it already deleted
		if strings.Contains(err.Error(), "notFound") {
			return nil
		}
		return fmt.Errorf("failed to delete VM: %w", err)
	}

	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("failed waiting for VM deletion: %w", err)
	}

	return nil
}

// GetVM returns the current state of a workshop VM
func (p *GCPFirecrackerProvider) GetVM(ctx context.Context, workshopID string) (*VMInstance, error) {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}
	defer client.Close()

	vmName := p.vmName(workshopID)

	instance, err := client.Get(ctx, &computepb.GetInstanceRequest{
		Project:  p.project,
		Zone:     p.zone,
		Instance: vmName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get VM: %w", err)
	}

	return p.instanceToVMInstance(instance), nil
}

// WaitForReady blocks until the VM is ready (agent healthy + MicroVMs created)
func (p *GCPFirecrackerProvider) WaitForReady(ctx context.Context, workshopID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for VM to be ready")
			}

			vm, err := p.GetVM(ctx, workshopID)
			if err != nil {
				continue
			}

			if vm.Status != "RUNNING" || vm.ExternalIP == "" {
				continue
			}

			// Check agent health
			agentURL := fmt.Sprintf("http://%s:%d", vm.ExternalIP, p.agentPort)
			healthURL := fmt.Sprintf("%s/health", agentURL)

			resp, err := p.httpClient.Get(healthURL)
			if err != nil {
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

// ListVMs returns all ClaraTeach Firecracker VMs (optionally filtered by workshop)
func (p *GCPFirecrackerProvider) ListVMs(ctx context.Context, workshopID string) ([]*VMInstance, error) {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}
	defer client.Close()

	filter := "labels.clarateach-runtime=firecracker"
	if workshopID != "" {
		filter = fmt.Sprintf("labels.clarateach-workshop=%s AND labels.clarateach-runtime=firecracker", workshopID)
	}

	req := &computepb.ListInstancesRequest{
		Project: p.project,
		Zone:    p.zone,
		Filter:  proto.String(filter),
	}

	var instances []*VMInstance
	it := client.List(ctx, req)
	for {
		instance, err := it.Next()
		if err != nil {
			break
		}
		instances = append(instances, p.instanceToVMInstance(instance))
	}

	return instances, nil
}

// instanceToVMInstance converts a GCE instance to our VMInstance type
func (p *GCPFirecrackerProvider) instanceToVMInstance(instance *computepb.Instance) *VMInstance {
	vm := &VMInstance{
		ID:       strconv.FormatUint(instance.GetId(), 10),
		Name:     instance.GetName(),
		Status:   instance.GetStatus(),
		Zone:     p.zone,
		SelfLink: instance.GetSelfLink(),
	}

	// Get IPs from network interfaces
	for _, ni := range instance.GetNetworkInterfaces() {
		vm.InternalIP = ni.GetNetworkIP()
		for _, ac := range ni.GetAccessConfigs() {
			if ac.GetNatIP() != "" {
				vm.ExternalIP = ac.GetNatIP()
				break
			}
		}
	}

	return vm
}
