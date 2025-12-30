package orchestrator

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type DockerProvider struct {
	cli *client.Client
}

func NewDockerProvider() (*DockerProvider, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerProvider{cli: cli}, nil
}

func (d *DockerProvider) Create(ctx context.Context, cfg InstanceConfig) (*Instance, error) {
	containerName := fmt.Sprintf("clarateach-%s-%d", cfg.WorkshopID, cfg.SeatID)
	networkName := "bridge"
	volumeName := fmt.Sprintf("%s-data", containerName)

	// Calculate host ports for local dev (seat 1 = 10001-10003, seat 2 = 10011-10013, etc.)
	basePort := 10000 + (cfg.SeatID * 10)
	hostTerminalPort := basePort + 1 // 10001, 10011, etc.
	hostFilesPort := basePort + 2    // 10002, 10012, etc.
	hostBrowserPort := basePort + 3  // 10003, 10013, etc.

	// 1. Ensure Volume Exists
	if err := d.ensureVolume(ctx, volumeName, cfg.WorkshopID, cfg.SeatID); err != nil {
		return nil, fmt.Errorf("failed to ensure volume: %w", err)
	}

	// 2. Check if container already exists
	existing, err := d.findContainer(ctx, containerName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.State.Running {
			// Get ports from existing container
			return d.buildInstanceFromContainer(existing, hostTerminalPort, hostFilesPort, hostBrowserPort), nil
		}
		// If stopped, remove it to recreate
		_ = d.cli.ContainerRemove(ctx, existing.ID, types.ContainerRemoveOptions{Force: true})
	}

	// 3. Create Container
	env := []string{
		"WORKSPACE_DIR=/workspace",
		"TERM=xterm-256color",
		"TERMINAL_PORT=3001",
		"FILES_PORT=3002",
		fmt.Sprintf("SEAT=%d", cfg.SeatID),
		fmt.Sprintf("CONTAINER_ID=c-%02d", cfg.SeatID),
		"AUTH_DISABLED=false",
	}
	if cfg.ApiKey != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", cfg.ApiKey))
	}

	labels := map[string]string{
		"clarateach.type":     "learner-workspace",
		"clarateach.workshop": cfg.WorkshopID,
		"clarateach.seat":     fmt.Sprintf("%d", cfg.SeatID),
	}

	// Port bindings for local development (maps container ports to host ports)
	portBindings := nat.PortMap{
		"3001/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", hostTerminalPort)}},
		"3002/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", hostFilesPort)}},
	}

	resp, err := d.cli.ContainerCreate(ctx,
		&container.Config{
			Image:    cfg.Image,
			Hostname: fmt.Sprintf("seat-%d", cfg.SeatID),
			Env:      env,
			Labels:   labels,
			Cmd:      []string{"node", "/home/learner/server/dist/index.js"},
			ExposedPorts: nat.PortSet{
				"3001/tcp": struct{}{},
				"3002/tcp": struct{}{},
			},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeVolume,
					Source: volumeName,
					Target: "/workspace",
				},
			},
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
			PortBindings:  portBindings,
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {},
			},
		},
		nil,
		containerName,
	)
	if err != nil {
		// If image not found, try to pull it
		if client.IsErrNotFound(err) {
			r, err := d.cli.ImagePull(ctx, cfg.Image, types.ImagePullOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to pull image: %w", err)
			}
			io.Copy(io.Discard, r)
			// Retry create
			resp, err = d.cli.ContainerCreate(ctx,
				&container.Config{
					Image:    cfg.Image,
					Hostname: fmt.Sprintf("seat-%d", cfg.SeatID),
					Env:      env,
					Labels:   labels,
					Cmd:      []string{"node", "/home/learner/server/dist/index.js"},
					ExposedPorts: nat.PortSet{
						"3001/tcp": struct{}{},
						"3002/tcp": struct{}{},
					},
				},
				&container.HostConfig{
					Mounts: []mount.Mount{
						{
							Type:   mount.TypeVolume,
							Source: volumeName,
							Target: "/workspace",
						},
					},
					RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
					PortBindings:  portBindings,
				},
				&network.NetworkingConfig{
					EndpointsConfig: map[string]*network.EndpointSettings{
						networkName: {},
					},
				},
				nil,
				containerName,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create container after pull: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to create container: %w", err)
		}
	}

	// 4. Start Container
	if err := d.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// --- NEKO SIDECAR START ---
	nekoName := fmt.Sprintf("%s-neko", containerName)
	// Check/Remove existing neko
	_ = d.cli.ContainerRemove(ctx, nekoName, types.ContainerRemoveOptions{Force: true})

	_, err = d.cli.ContainerCreate(ctx,
		&container.Config{
			Image: "m1k1o/neko:firefox",
			Env: []string{
				"NEKO_PASSWORD=clarateach",
				"NEKO_PASSWORD_ADMIN=clarateach",
				"NEKO_BIND=:3003",
				"NEKO_EPR=:59000-59100",
			},
			Labels: map[string]string{
				"clarateach.type":     "learner-browser",
				"clarateach.workshop": cfg.WorkshopID,
				"clarateach.seat":     fmt.Sprintf("%d", cfg.SeatID),
			},
			ExposedPorts: map[nat.Port]struct{}{
				"3003/tcp": {},
			},
		},
		&container.HostConfig{
			ShmSize:       1024 * 1024 * 1024 * 2, // 2GB
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {},
			},
		},
		nil,
		nekoName,
	)
	if err != nil {
		// Try pull if missing
		if client.IsErrNotFound(err) {
			r, _ := d.cli.ImagePull(ctx, "m1k1o/neko:firefox", types.ImagePullOptions{})
			io.Copy(io.Discard, r)
			// Retry create with full config
			_, err = d.cli.ContainerCreate(
				ctx,
				&container.Config{
					Image: "m1k1o/neko:firefox",
					Env: []string{
						"NEKO_PASSWORD=clarateach",
						"NEKO_PASSWORD_ADMIN=clarateach",
						"NEKO_BIND=:3003",
						"NEKO_EPR=:59000-59100",
					},
					Labels: map[string]string{
						"clarateach.type":     "learner-browser",
						"clarateach.workshop": cfg.WorkshopID,
						"clarateach.seat":     fmt.Sprintf("%d", cfg.SeatID),
					},
					ExposedPorts: map[nat.Port]struct{}{
						"3003/tcp": {},
					},
				},
				&container.HostConfig{
					ShmSize:       1024 * 1024 * 1024 * 2,
					RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
				},
				&network.NetworkingConfig{
					EndpointsConfig: map[string]*network.EndpointSettings{
						networkName: {},
					},
				},
				nil,
				nekoName,
			)
		}
		if err != nil {
			// Log warning but don't fail main container? Or fail?
			// For now, let's log and continue, as browser is secondary.
			fmt.Printf("Warning: Failed to create Neko container: %v\n", err)
		}
	} else {
		// Start Neko
		d.cli.ContainerStart(ctx, nekoName, types.ContainerStartOptions{})
	}
	// --- NEKO SIDECAR END ---

	// 5. Inspect to get IP
	inspect, err := d.cli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return nil, err
	}

	ip := d.getContainerIP(&inspect, networkName)
	return &Instance{
		ID:               resp.ID,
		IP:               ip,
		Status:           "running",
		HostTerminalPort: hostTerminalPort,
		HostFilesPort:    hostFilesPort,
		HostBrowserPort:  hostBrowserPort,
	}, nil
}

func (d *DockerProvider) buildInstanceFromContainer(c *types.ContainerJSON, termPort, filesPort, browserPort int) *Instance {
	ip := d.getContainerIP(c, "bridge")
	return &Instance{
		ID:               c.ID,
		IP:               ip,
		Status:           "running",
		HostTerminalPort: termPort,
		HostFilesPort:    filesPort,
		HostBrowserPort:  browserPort,
	}
}

func (d *DockerProvider) Destroy(ctx context.Context, workshopID string, seatID int) error {
	containerName := fmt.Sprintf("clarateach-%s-%d", workshopID, seatID)
	nekoName := fmt.Sprintf("%s-neko", containerName)

	err1 := d.cli.ContainerRemove(ctx, containerName, types.ContainerRemoveOptions{Force: true})
	err2 := d.cli.ContainerRemove(ctx, nekoName, types.ContainerRemoveOptions{Force: true})

	if err1 != nil {
		return err1
	}
	return err2
}

func (d *DockerProvider) List(ctx context.Context, workshopID string) ([]*Instance, error) {
	filters := filters.NewArgs()
	filters.Add("label", "clarateach.type=learner-workspace")
	if workshopID != "" {
		filters.Add("label", fmt.Sprintf("clarateach.workshop=%s", workshopID))
	}

	containers, err := d.cli.ContainerList(ctx, types.ContainerListOptions{Filters: filters, All: true})
	if err != nil {
		return nil, err
	}

	var instances []*Instance
	for _, c := range containers {
		ip := ""
		if c.NetworkSettings != nil && c.NetworkSettings.Networks != nil {
			// Prefer bridge network (for OrbStack local dev)
			if net, ok := c.NetworkSettings.Networks["bridge"]; ok {
				ip = net.IPAddress
			} else {
				// Fallback to any network
				for _, net := range c.NetworkSettings.Networks {
					ip = net.IPAddress
					break
				}
			}
		}

		instances = append(instances, &Instance{
			ID:     c.ID,
			IP:     ip,
			Status: c.State,
		})
	}
	return instances, nil
}

func (d *DockerProvider) GetIP(ctx context.Context, workshopID string, seatID int) (string, error) {
	containerName := fmt.Sprintf("clarateach-%s-%d", workshopID, seatID)

	inspect, err := d.cli.ContainerInspect(ctx, containerName)
	if err != nil {
		return "", err
	}

	return d.getContainerIP(&inspect, "bridge"), nil
}

func (d *DockerProvider) GetBrowserIP(ctx context.Context, workshopID string, seatID int) (string, error) {
	nekoName := fmt.Sprintf("clarateach-%s-%d-neko", workshopID, seatID)

	inspect, err := d.cli.ContainerInspect(ctx, nekoName)
	if err != nil {
		return "", err
	}

	return d.getContainerIP(&inspect, "bridge"), nil
}

func (d *DockerProvider) GetInstance(ctx context.Context, workshopID string, seatID int) (*Instance, error) {
	containerName := fmt.Sprintf("clarateach-%s-%d", workshopID, seatID)

	inspect, err := d.cli.ContainerInspect(ctx, containerName)
	if err != nil {
		return nil, err
	}

	// Calculate expected host ports based on seat ID
	basePort := 10000 + (seatID * 10)

	return &Instance{
		ID:               inspect.ID,
		IP:               d.getContainerIP(&inspect, "bridge"),
		Status:           inspect.State.Status,
		HostTerminalPort: basePort + 1,
		HostFilesPort:    basePort + 2,
		HostBrowserPort:  basePort + 3,
	}, nil
}

// Helpers

func (d *DockerProvider) ensureNetwork(ctx context.Context, name string, workshopID string) error {
	_, err := d.cli.NetworkInspect(ctx, name, types.NetworkInspectOptions{})
	if err == nil {
		return nil
	}
	if !client.IsErrNotFound(err) {
		return err
	}

	_, err = d.cli.NetworkCreate(ctx, name, types.NetworkCreate{
		Driver: "bridge",
		Labels: map[string]string{
			"clarateach.type":     "workshop-network",
			"clarateach.workshop": workshopID,
		},
	})
	return err
}

func (d *DockerProvider) ensureVolume(ctx context.Context, name string, workshopID string, seatID int) error {
	_, err := d.cli.VolumeInspect(ctx, name)
	if err == nil {
		return nil
	}
	if !client.IsErrNotFound(err) {
		return err
	}

	_, err = d.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
		Labels: map[string]string{
			"clarateach.type":     "workspace-data",
			"clarateach.workshop": workshopID,
			"clarateach.seat":     fmt.Sprintf("%d", seatID),
		},
	})
	return err
}

func (d *DockerProvider) findContainer(ctx context.Context, name string) (*types.ContainerJSON, error) {
	c, err := d.cli.ContainerInspect(ctx, name)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (d *DockerProvider) getContainerIP(c *types.ContainerJSON, networkName string) string {
	if c.NetworkSettings == nil || c.NetworkSettings.Networks == nil {
		return ""
	}
	if net, ok := c.NetworkSettings.Networks[networkName]; ok {
		return net.IPAddress
	}
	for _, net := range c.NetworkSettings.Networks {
		return net.IPAddress
	}
	return ""
}
