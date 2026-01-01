package orchestrator

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// FirecrackerConfig holds configuration for the Firecracker provider.
type FirecrackerConfig struct {
	ImagesDir       string // Directory containing kernel and rootfs (default: /var/lib/clarateach/images)
	KernelPath      string // Path to vmlinux kernel (default: ImagesDir/vmlinux)
	RootfsPath      string // Path to base rootfs.ext4 (default: ImagesDir/rootfs.ext4)
	FirecrackerPath string // Path to firecracker binary (default: /usr/local/bin/firecracker)
	SocketDir       string // Directory for Firecracker sockets (default: /tmp/clarateach)
	VCPUs           int64  // Number of vCPUs per VM (default: 2)
	MemoryMB        int64  // Memory in MB per VM (default: 512)
	BridgeName      string // Bridge name (default: clarateach0)
	BridgeIP        string // Bridge IP (default: 192.168.100.1/24)
}

// DefaultConfig returns the default Firecracker configuration.
func DefaultConfig() FirecrackerConfig {
	imagesDir := "/var/lib/clarateach/images"
	return FirecrackerConfig{
		ImagesDir:       imagesDir,
		KernelPath:      imagesDir + "/vmlinux",
		RootfsPath:      imagesDir + "/rootfs.ext4",
		FirecrackerPath: "/usr/local/bin/firecracker",
		SocketDir:       "/tmp/clarateach",
		VCPUs:           2,
		MemoryMB:        512,
		BridgeName:      "clarateach0",
		BridgeIP:        "192.168.100.1/24",
	}
}

// vmState tracks a running Firecracker VM
type vmState struct {
	machine    *firecracker.Machine
	rootfsPath string
	tapName    string
	ip         string
}

// FirecrackerProvider implements the Provider interface for Firecracker MicroVMs.
type FirecrackerProvider struct {
	config FirecrackerConfig
	vms    map[string]*vmState // key: "workshopID-seatID"
	mu     sync.RWMutex
	logger *logrus.Logger
}

// NewFirecrackerProvider creates a new FirecrackerProvider with default configuration.
func NewFirecrackerProvider() (*FirecrackerProvider, error) {
	return NewFirecrackerProviderWithConfig(DefaultConfig())
}

// NewFirecrackerProviderWithConfig creates a new FirecrackerProvider with custom configuration.
func NewFirecrackerProviderWithConfig(cfg FirecrackerConfig) (*FirecrackerProvider, error) {
	// Ensure socket directory exists
	if err := os.MkdirAll(cfg.SocketDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &FirecrackerProvider{
		config: cfg,
		vms:    make(map[string]*vmState),
		logger: logger,
	}, nil
}

// vmKey generates a unique key for a VM
func vmKey(workshopID string, seatID int) string {
	return fmt.Sprintf("%s-%d", workshopID, seatID)
}

// Create provisions a new Firecracker MicroVM instance.
func (f *FirecrackerProvider) Create(ctx context.Context, cfg InstanceConfig) (*Instance, error) {
	key := vmKey(cfg.WorkshopID, cfg.SeatID)

	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if VM already exists
	if _, exists := f.vms[key]; exists {
		return nil, fmt.Errorf("VM already exists for workshop %s seat %d", cfg.WorkshopID, cfg.SeatID)
	}

	// 1. Ensure bridge exists and is configured
	if err := f.ensureBridge(); err != nil {
		return nil, fmt.Errorf("failed to setup bridge: %w", err)
	}

	// 2. Create TAP device
	// Use up to 8 chars of workshop ID, handling short IDs
	workshopPrefix := cfg.WorkshopID
	if len(workshopPrefix) > 8 {
		workshopPrefix = workshopPrefix[:8]
	}
	tapName := fmt.Sprintf("tap%s%d", workshopPrefix, cfg.SeatID)
	if len(tapName) > 15 {
		tapName = tapName[:15] // Linux interface name limit
	}
	if err := f.createTAP(tapName); err != nil {
		return nil, fmt.Errorf("failed to create TAP device: %w", err)
	}

	// 3. Calculate IP for this VM
	vmIP := fmt.Sprintf("192.168.100.%d", 10+cfg.SeatID)
	gatewayIP := "192.168.100.1"

	// 4. Copy rootfs for this VM
	vmRootfs := filepath.Join(f.config.SocketDir, fmt.Sprintf("rootfs-%s.ext4", key))
	if err := copyFile(f.config.RootfsPath, vmRootfs); err != nil {
		f.deleteTAP(tapName)
		return nil, fmt.Errorf("failed to copy rootfs: %w", err)
	}

	// 5. Create Firecracker VM
	socketPath := filepath.Join(f.config.SocketDir, fmt.Sprintf("%s.sock", key))

	// Remove stale socket if exists
	os.Remove(socketPath)

	// Build kernel boot args with network config
	// Format: ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>
	bootArgs := fmt.Sprintf("console=ttyS0 reboot=k panic=1 pci=off ip=%s::%s:255.255.255.0::eth0:off", vmIP, gatewayIP)

	fcCfg := firecracker.Config{
		SocketPath:      socketPath,
		KernelImagePath: f.config.KernelPath,
		KernelArgs:      bootArgs,
		Drives: []models.Drive{
			{
				DriveID:      firecracker.String("rootfs"),
				PathOnHost:   firecracker.String(vmRootfs),
				IsRootDevice: firecracker.Bool(true),
				IsReadOnly:   firecracker.Bool(false),
			},
		},
		NetworkInterfaces: []firecracker.NetworkInterface{
			{
				StaticConfiguration: &firecracker.StaticNetworkConfiguration{
					MacAddress:  fmt.Sprintf("AA:FC:00:00:%02X:%02X", cfg.SeatID/256, cfg.SeatID%256),
					HostDevName: tapName,
				},
			},
		},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(f.config.VCPUs),
			MemSizeMib: firecracker.Int64(f.config.MemoryMB),
		},
	}

	// Create the machine
	// Use background context for the VM process so it survives beyond the HTTP request
	cmd := firecracker.VMCommandBuilder{}.
		WithBin(f.config.FirecrackerPath).
		WithSocketPath(socketPath).
		Build(context.Background())

	// Use background context for the machine so it survives beyond the HTTP request
	machineCtx := context.Background()
	machine, err := firecracker.NewMachine(machineCtx, fcCfg, firecracker.WithProcessRunner(cmd), firecracker.WithLogger(logrus.NewEntry(f.logger)))
	if err != nil {
		os.Remove(vmRootfs)
		f.deleteTAP(tapName)
		return nil, fmt.Errorf("failed to create Firecracker machine: %w", err)
	}

	// Start the machine with background context
	if err := machine.Start(machineCtx); err != nil {
		os.Remove(vmRootfs)
		f.deleteTAP(tapName)
		return nil, fmt.Errorf("failed to start Firecracker machine: %w", err)
	}

	// Track the VM
	f.vms[key] = &vmState{
		machine:    machine,
		rootfsPath: vmRootfs,
		tapName:    tapName,
		ip:         vmIP,
	}

	f.logger.Infof("Started VM %s with IP %s", key, vmIP)

	return &Instance{
		WorkshopID: cfg.WorkshopID,
		SeatID:     cfg.SeatID,
		IP:         vmIP,
	}, nil
}

// ensureBridge ensures the clarateach0 bridge exists and is configured
func (f *FirecrackerProvider) ensureBridge() error {
	bridgeName := f.config.BridgeName

	link, err := netlink.LinkByName(bridgeName)
	if err != nil {
		// Bridge doesn't exist, create it
		bridge := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: bridgeName,
			},
		}
		if err := netlink.LinkAdd(bridge); err != nil {
			return fmt.Errorf("failed to create bridge %s: %w", bridgeName, err)
		}
		link, _ = netlink.LinkByName(bridgeName)
	}

	// Assign IP to bridge if not already assigned
	addr, _ := netlink.ParseAddr(f.config.BridgeIP)
	addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
	hasIP := false
	for _, a := range addrs {
		if a.IP.Equal(addr.IP) {
			hasIP = true
			break
		}
	}
	if !hasIP {
		if err := netlink.AddrAdd(link, addr); err != nil {
			return fmt.Errorf("failed to assign IP to bridge: %w", err)
		}
	}

	// Bring bridge up
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring up bridge: %w", err)
	}

	// Setup NAT
	return f.setupNAT()
}

// createTAP creates a TAP device and attaches it to the bridge
func (f *FirecrackerProvider) createTAP(tapName string) error {
	// Check if TAP already exists
	if _, err := netlink.LinkByName(tapName); err == nil {
		// Already exists, delete and recreate
		f.deleteTAP(tapName)
	}

	tap := &netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: tapName,
		},
		Mode: netlink.TUNTAP_MODE_TAP,
	}
	if err := netlink.LinkAdd(tap); err != nil {
		return fmt.Errorf("failed to create TAP device %s: %w", tapName, err)
	}

	// Get the TAP device we just created
	link, err := netlink.LinkByName(tapName)
	if err != nil {
		return fmt.Errorf("failed to find TAP device %s: %w", tapName, err)
	}

	// Attach to bridge
	bridge, err := netlink.LinkByName(f.config.BridgeName)
	if err != nil {
		return fmt.Errorf("failed to find bridge %s: %w", f.config.BridgeName, err)
	}
	if err := netlink.LinkSetMaster(link, bridge.(*netlink.Bridge)); err != nil {
		return fmt.Errorf("failed to attach TAP to bridge: %w", err)
	}

	// Bring TAP up
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring up TAP device: %w", err)
	}

	return nil
}

// deleteTAP removes a TAP device
func (f *FirecrackerProvider) deleteTAP(tapName string) error {
	link, err := netlink.LinkByName(tapName)
	if err != nil {
		return nil // Already gone
	}
	return netlink.LinkDel(link)
}

// setupNAT configures iptables for NAT/masquerading
func (f *FirecrackerProvider) setupNAT() error {
	// Enable IP forwarding
	if err := runCommand("sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	// Detect the primary interface (not the bridge)
	primaryIface, err := detectPrimaryInterface()
	if err != nil {
		return fmt.Errorf("failed to detect primary interface: %w", err)
	}

	// Add POSTROUTING rule for masquerading
	if err := runCommand("iptables", "-t", "nat", "-C", "POSTROUTING", "-o", primaryIface, "-j", "MASQUERADE"); err != nil {
		if err := runCommand("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", primaryIface, "-j", "MASQUERADE"); err != nil {
			return fmt.Errorf("failed to add masquerade rule: %w", err)
		}
	}

	// Allow forwarding from bridge
	if err := runCommand("iptables", "-C", "FORWARD", "-i", f.config.BridgeName, "-o", primaryIface, "-j", "ACCEPT"); err != nil {
		if err := runCommand("iptables", "-A", "FORWARD", "-i", f.config.BridgeName, "-o", primaryIface, "-j", "ACCEPT"); err != nil {
			return fmt.Errorf("failed to add forward rule: %w", err)
		}
	}

	// Allow established connections back
	if err := runCommand("iptables", "-C", "FORWARD", "-i", primaryIface, "-o", f.config.BridgeName, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		if err := runCommand("iptables", "-A", "FORWARD", "-i", primaryIface, "-o", f.config.BridgeName, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
			return fmt.Errorf("failed to add established forward rule: %w", err)
		}
	}

	return nil
}

// detectPrimaryInterface finds the interface with the default route
func detectPrimaryInterface() (string, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return "", err
	}
	for _, route := range routes {
		// Default route: Dst is nil OR 0.0.0.0/0
		isDefault := route.Dst == nil
		if !isDefault && route.Dst != nil {
			ones, _ := route.Dst.Mask.Size()
			isDefault = route.Dst.IP.IsUnspecified() && ones == 0
		}
		if isDefault {
			link, err := netlink.LinkByIndex(route.LinkIndex)
			if err != nil {
				continue
			}
			return link.Attrs().Name, nil
		}
	}
	return "", fmt.Errorf("no default route found")
}

// Destroy destroys a Firecracker MicroVM instance.
func (f *FirecrackerProvider) Destroy(ctx context.Context, workshopID string, seatID int) error {
	key := vmKey(workshopID, seatID)

	f.mu.Lock()
	defer f.mu.Unlock()

	vm, exists := f.vms[key]
	if !exists {
		return fmt.Errorf("VM not found: %s", key)
	}

	// Stop the VM
	if err := vm.machine.StopVMM(); err != nil {
		f.logger.Warnf("Failed to stop VMM for %s: %v", key, err)
	}

	// Cleanup resources
	f.deleteTAP(vm.tapName)
	os.Remove(vm.rootfsPath)

	// Remove socket
	socketPath := filepath.Join(f.config.SocketDir, fmt.Sprintf("%s.sock", key))
	os.Remove(socketPath)

	delete(f.vms, key)
	f.logger.Infof("Destroyed VM %s", key)

	return nil
}

// List lists all active Firecracker MicroVM instances for a workshop.
func (f *FirecrackerProvider) List(ctx context.Context, workshopID string) ([]*Instance, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var instances []*Instance
	prefix := workshopID + "-"
	for key, vm := range f.vms {
		if strings.HasPrefix(key, prefix) {
			parts := strings.Split(key, "-")
			seatID := 0
			fmt.Sscanf(parts[len(parts)-1], "%d", &seatID)
			instances = append(instances, &Instance{
				WorkshopID: workshopID,
				SeatID:     seatID,
				IP:         vm.ip,
			})
		}
	}
	return instances, nil
}

// GetIP returns the IP address of a Firecracker MicroVM instance.
func (f *FirecrackerProvider) GetIP(ctx context.Context, workshopID string, seatID int) (string, error) {
	key := vmKey(workshopID, seatID)

	f.mu.RLock()
	defer f.mu.RUnlock()

	vm, exists := f.vms[key]
	if !exists {
		return "", fmt.Errorf("VM not found: %s", key)
	}
	return vm.ip, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// runCommand executes a shell command and returns an error if it fails.
func runCommand(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command %s %s failed: %w\nOutput: %s", name, strings.Join(arg, " "), err, string(output))
	}
	return nil
}
