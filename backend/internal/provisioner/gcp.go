package provisioner

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"
)

// GCPProvider implements the Provisioner interface for Google Cloud
type GCPProvider struct {
	project       string
	zone          string
	network       string
	subnetwork    string
	imageProject  string // cos-cloud for Container-Optimized OS
	imageFamily   string // cos-stable
	registryURL   string // Artifact Registry base URL
	sshUser       string // Default SSH user for key injection
}

// GCPConfig holds configuration for the GCP provider
type GCPConfig struct {
	Project      string
	Zone         string
	Network      string // default: "default"
	Subnetwork   string // default: "" (auto)
	RegistryURL  string // e.g., "us-central1-docker.pkg.dev/PROJECT/clarateach"
	SSHUser      string // default: "clarateach"
}

// NewGCPProvider creates a new GCP provisioner
func NewGCPProvider(cfg GCPConfig) *GCPProvider {
	if cfg.Network == "" {
		cfg.Network = "default"
	}
	if cfg.SSHUser == "" {
		cfg.SSHUser = "clarateach"
	}
	return &GCPProvider{
		project:      cfg.Project,
		zone:         cfg.Zone,
		network:      cfg.Network,
		subnetwork:   cfg.Subnetwork,
		imageProject: "cos-cloud",
		imageFamily:  "cos-stable",
		registryURL:  cfg.RegistryURL,
		sshUser:      cfg.SSHUser,
	}
}

// vmName generates a consistent VM name for a workshop
func (p *GCPProvider) vmName(workshopID string) string {
	return fmt.Sprintf("clarateach-ws-%s", workshopID)
}

// CreateVM provisions a new GCE VM for a workshop
func (p *GCPProvider) CreateVM(ctx context.Context, cfg VMConfig) (*VMInstance, error) {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}
	defer client.Close()

	vmName := p.vmName(cfg.WorkshopID)

	// Build startup script
	startupScript := p.buildStartupScript(cfg)

	// Build metadata
	// Note: We don't enable OS Login because it ignores metadata SSH keys
	// Using metadata SSH keys for debugging access
	metadata := []*computepb.Items{
		{Key: proto.String("startup-script"), Value: proto.String(startupScript)},
		{Key: proto.String("workshop_id"), Value: proto.String(cfg.WorkshopID)},
		{Key: proto.String("seats"), Value: proto.String(strconv.Itoa(cfg.Seats))},
	}

	// Add SSH key if provided (for debugging)
	if cfg.SSHPublicKey != "" {
		sshKeyEntry := fmt.Sprintf("%s:%s", p.sshUser, cfg.SSHPublicKey)
		metadata = append(metadata, &computepb.Items{
			Key:   proto.String("ssh-keys"),
			Value: proto.String(sshKeyEntry),
		})
	}

	// Build scheduling config (spot vs on-demand)
	// e2 instances require MIGRATE for on-demand, TERMINATE only for spot/preemptible
	scheduling := &computepb.Scheduling{
		AutomaticRestart: proto.Bool(!cfg.Spot),
	}
	if cfg.Spot {
		scheduling.OnHostMaintenance = proto.String("TERMINATE")
		scheduling.Preemptible = proto.Bool(true)
		scheduling.ProvisioningModel = proto.String("SPOT")
	} else {
		scheduling.OnHostMaintenance = proto.String("MIGRATE")
	}

	// Build instance resource
	instance := &computepb.Instance{
		Name:        proto.String(vmName),
		MachineType: proto.String(fmt.Sprintf("zones/%s/machineTypes/%s", p.zone, cfg.MachineType)),
		Disks: []*computepb.AttachedDisk{
			{
				Boot:       proto.Bool(true),
				AutoDelete: proto.Bool(true),
				InitializeParams: &computepb.AttachedDiskInitializeParams{
					SourceImage: proto.String(fmt.Sprintf("projects/%s/global/images/family/%s", p.imageProject, p.imageFamily)),
					DiskSizeGb:  proto.Int64(int64(cfg.DiskSizeGB)),
					DiskType:    proto.String(fmt.Sprintf("zones/%s/diskTypes/pd-balanced", p.zone)),
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
			"managed-by":          "clarateach-backend",
		},
		Tags: &computepb.Tags{
			Items: []string{"clarateach", "http-server", "https-server"},
		},
		ServiceAccounts: []*computepb.ServiceAccount{
			{
				Email: proto.String("default"),
				Scopes: []string{
					"https://www.googleapis.com/auth/devstorage.read_only", // Pull from Artifact Registry
					"https://www.googleapis.com/auth/logging.write",        // Cloud Logging
					"https://www.googleapis.com/auth/monitoring.write",     // Cloud Monitoring
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

	// Get the created instance details
	return p.GetVM(ctx, cfg.WorkshopID)
}

// DeleteVM terminates and removes a workshop VM
func (p *GCPProvider) DeleteVM(ctx context.Context, workshopID string) error {
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
func (p *GCPProvider) GetVM(ctx context.Context, workshopID string) (*VMInstance, error) {
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

// WaitForReady blocks until the VM is ready to accept connections
func (p *GCPProvider) WaitForReady(ctx context.Context, workshopID string, timeout time.Duration) error {
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
				continue // VM might not exist yet
			}

			if vm.Status == "RUNNING" && vm.ExternalIP != "" {
				// TODO: Add actual health check (HTTP probe to Caddy)
				return nil
			}
		}
	}
}

// ListVMs returns all ClaraTeach VMs (optionally filtered by workshop)
func (p *GCPProvider) ListVMs(ctx context.Context, workshopID string) ([]*VMInstance, error) {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}
	defer client.Close()

	filter := "labels.clarateach=true"
	if workshopID != "" {
		filter = fmt.Sprintf("labels.clarateach-workshop=%s", workshopID)
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
			break // End of list or error
		}
		instances = append(instances, p.instanceToVMInstance(instance))
	}

	return instances, nil
}

// instanceToVMInstance converts a GCE instance to our VMInstance type
func (p *GCPProvider) instanceToVMInstance(instance *computepb.Instance) *VMInstance {
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

// buildStartupScript generates the startup script for the VM
func (p *GCPProvider) buildStartupScript(cfg VMConfig) string {
	// Get image URL - use provided or construct from registry
	imageURL := cfg.Image
	if imageURL == "" && p.registryURL != "" {
		imageURL = fmt.Sprintf("%s/workspace:latest", p.registryURL)
	}

	authDisabled := "false"
	if cfg.AuthDisabled {
		authDisabled = "true"
	}

	script := `#!/bin/bash
set -e

# Log to both file and Cloud Logging
log() {
    echo "$1"
    logger -t clarateach-startup "$1"
}

exec > >(tee /var/log/clarateach-startup.log) 2>&1
log "ClaraTeach VM startup script starting at $(date)"

# Get metadata
WORKSHOP_ID=$(curl -sf -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/attributes/workshop_id)
SEATS=$(curl -sf -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/attributes/seats)
IMAGE="` + imageURL + `"
AUTH_DISABLED="` + authDisabled + `"

log "Workshop ID: $WORKSHOP_ID, Seats: $SEATS, Auth Disabled: $AUTH_DISABLED"
log "Image: $IMAGE"

# Open firewall for port 8080 (COS default iptables blocks non-SSH ports)
log "Opening iptables for port 8080..."
iptables -A INPUT -p tcp --dport 8080 -j ACCEPT

# Wait for Docker (COS has Docker pre-installed)
log "Waiting for Docker..."
while ! docker info > /dev/null 2>&1; do
    sleep 1
done
log "Docker is ready"

# Configure Docker to authenticate with Artifact Registry using access token
# COS has read-only root filesystem, so set DOCKER_CONFIG to writable location
log "Configuring Docker authentication..."
export DOCKER_CONFIG=/home/chronos/.docker
mkdir -p $DOCKER_CONFIG
ACCESS_TOKEN=$(curl -sf -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token | cut -d'"' -f4)
echo "$ACCESS_TOKEN" | docker login -u oauth2accesstoken --password-stdin https://` + p.extractRegistry(imageURL) + `

log "Pulling workspace image: $IMAGE"
docker pull "$IMAGE"

log "Pulling Neko browser image..."
docker pull m1k1o/neko:firefox || log "Warning: Failed to pull Neko image"

log "Pulling Caddy image..."
docker pull caddy:latest

# Create Docker network for containers
docker network create clarateach-net 2>/dev/null || true

# Create Caddy config directory
mkdir -p /etc/caddy

log "Generating Caddy configuration for $SEATS seats..."
cat > /etc/caddy/Caddyfile <<EOF
{
    auto_https off
}

:8080 {
    # Health check endpoint
    handle /health {
        respond "OK" 200
    }

EOF

# Add route for each seat
for i in $(seq 1 $SEATS); do
    TERM_PORT=$((3000 + i*10 + 1))
    FILES_PORT=$((3000 + i*10 + 2))
    BROWSER_PORT=$((3000 + i*10 + 3))

    cat >> /etc/caddy/Caddyfile <<EOF

    # Seat $i routes
    # Terminal and files - pass through with full path (server expects /vm/:seat/...)
    @terminal$i path /vm/$i/terminal /vm/$i/terminal/*
    handle @terminal$i {
        reverse_proxy localhost:$TERM_PORT
    }

    @files$i path /vm/$i/files /vm/$i/files/*
    handle @files$i {
        reverse_proxy localhost:$FILES_PORT
    }

    # Browser (Neko) - strip prefix, Neko expects / or /ws
    # Use @matcher to match all paths under /vm/N/browser (including /browser, /browser/, /browser/js/*)
    @browser$i path /vm/$i/browser /vm/$i/browser/*
    handle @browser$i {
        uri strip_prefix /vm/$i/browser
        reverse_proxy localhost:$BROWSER_PORT
    }
EOF
done

# Close the Caddyfile
cat >> /etc/caddy/Caddyfile <<'EOF'

    # Default response
    respond "ClaraTeach Workshop VM" 200
}
EOF

log "Starting $SEATS seat containers..."
for i in $(seq 1 $SEATS); do
    TERM_PORT=$((3000 + i*10 + 1))
    FILES_PORT=$((3000 + i*10 + 2))
    BROWSER_PORT=$((3000 + i*10 + 3))

    log "Starting seat $i (terminal:$TERM_PORT, files:$FILES_PORT, browser:$BROWSER_PORT)"

    # Create data volume for this seat
    docker volume create "seat-${i}-data" 2>/dev/null || true

    # Start main workspace container
    docker run -d \
        --name "seat-$i" \
        --restart unless-stopped \
        --network clarateach-net \
        -p "${TERM_PORT}:3001" \
        -p "${FILES_PORT}:3002" \
        -v "seat-${i}-data:/workspace" \
        -e "SEAT=$i" \
        -e "CONTAINER_ID=c-$(printf '%02d' $i)" \
        -e "WORKSPACE_DIR=/workspace" \
        -e "TERM=xterm-256color" \
        -e "TERMINAL_PORT=3001" \
        -e "FILES_PORT=3002" \
        -e "AUTH_DISABLED=$AUTH_DISABLED" \
        "$IMAGE" && log "Seat $i workspace started" || log "ERROR: Failed to start seat $i workspace"

    # Start Neko browser sidecar
    docker run -d \
        --name "seat-${i}-neko" \
        --restart unless-stopped \
        --network clarateach-net \
        --shm-size=2g \
        -p "${BROWSER_PORT}:8080" \
        -e "NEKO_SCREEN=1280x720@30" \
        -e "NEKO_PASSWORD=neko" \
        -e "NEKO_PASSWORD_ADMIN=admin" \
        m1k1o/neko:firefox && log "Seat $i Neko started" || log "Warning: Neko container failed for seat $i"
done

log "Starting Caddy reverse proxy..."
docker run -d \
    --name caddy \
    --restart unless-stopped \
    --network host \
    -v /etc/caddy/Caddyfile:/etc/caddy/Caddyfile:ro \
    caddy:latest && log "Caddy started" || log "ERROR: Caddy failed to start"

# Wait a bit and check status
sleep 5
CONTAINER_COUNT=$(docker ps -q | wc -l)
log "Containers running: $CONTAINER_COUNT"
docker ps --format "table {{.Names}}\t{{.Status}}" | while read line; do log "$line"; done

# Health check
curl -sf http://localhost:8080/health && log "VM healthy" || log "VM health check failed"

log "ClaraTeach VM startup complete at $(date)"
`

	return script
}

// extractRegistry extracts the registry hostname from an image URL
func (p *GCPProvider) extractRegistry(imageURL string) string {
	// Handle format: us-central1-docker.pkg.dev/project/repo/image:tag
	parts := strings.Split(imageURL, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return "us-central1-docker.pkg.dev"
}
