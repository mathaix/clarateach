package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/vishvananda/netlink"
)

// FirecrackerProvider implements the Provider interface for Firecracker MicroVMs.
type FirecrackerProvider struct {
	// TODO: Add fields for configuration like kernel path, rootfs path, etc.
}

// NewFirecrackerProvider creates a new FirecrackerProvider.
func NewFirecrackerProvider() (*FirecrackerProvider, error) {
	// TODO: Initialize with necessary configuration
	return &FirecrackerProvider{}, nil
}

// Create provisions a new Firecracker MicroVM instance.
func (f *FirecrackerProvider) Create(ctx context.Context, cfg InstanceConfig) (*Instance, error) {
	// Phase 2: Networking Plumbing
	// 1. Ensure clarateach0 bridge exists
	bridgeName := "clarateach0"
	link, err := netlink.LinkByName(bridgeName)
	if err != nil {
		// Bridge does not exist, create it
		bridge := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: bridgeName,
			},
		}
		if err := netlink.LinkAdd(bridge); err != nil {
			return nil, fmt.Errorf("failed to create bridge %s: %w", bridgeName, err)
		}
		link = bridge
	}

	// Ensure bridge is up
	if err := netlink.LinkSetUp(link); err != nil {
		return nil, fmt.Errorf("failed to bring up bridge %s: %w", bridgeName, err)
	}

	// Setup NAT/Masquerading for the bridge
	if err := setupNAT(bridgeName); err != nil {
		return nil, fmt.Errorf("failed to set up NAT for bridge %s: %w", bridgeName, err)
	}

	// 2. Create TAP device
	tapName := fmt.Sprintf("tap%d", cfg.SeatID)
	tap := &netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: tapName,
		},
		Mode: netlink.TUNTAP_MODE_TAP,
	}
	if err := netlink.LinkAdd(tap); err != nil {
		return nil, fmt.Errorf("failed to create TAP device %s: %w", tapName, err)
	}

	// 3. Attach TAP to bridge
	if err := netlink.LinkSetMaster(tap, link.(*netlink.Bridge)); err != nil {
		return nil, fmt.Errorf("failed to attach TAP device %s to bridge %s: %w", tapName, bridgeName, err)
	}

	// Ensure TAP device is up
	if err := netlink.LinkSetUp(tap); err != nil {
		return nil, fmt.Errorf("failed to bring up TAP device %s: %w", tapName, err)
	}

	// 4. Manual IPAM
	ipAddr := fmt.Sprintf("192.168.100.%d/24", 100+cfg.SeatID)
	addr, err := netlink.ParseAddr(ipAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IP address %s: %w", ipAddr, err)
	}
	// Assign IP to the TAP device (this IP will be used by the MicroVM)
	if err := netlink.AddrAdd(tap, addr); err != nil {
		return nil, fmt.Errorf("failed to assign IP %s to TAP device %s: %w", ipAddr, tapName, err)
	}

	return &Instance{
		WorkshopID: cfg.WorkshopID,
		SeatID:     cfg.SeatID,
		IP:         addr.IP.String(),
	}, nil
}

// setupNAT configures iptables for NAT/masquerading on the given bridge.
func setupNAT(bridgeName string) error {
	// Enable IP forwarding
	if err := runCommand("sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	// Add POSTROUTING rule for masquerading
	if err := runCommand("iptables", "-t", "nat", "-C", "POSTROUTING", "-o", "ens4", "-j", "MASQUERADE"); err != nil {
		// Rule doesn't exist, add it
		if err := runCommand("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "ens4", "-j", "MASQUERADE"); err != nil {
			return fmt.Errorf("failed to add POSTROUTING masquerade rule: %w", err)
		}
	}

	// Add FORWARD rules
	// Allow traffic from bridge to outside
	if err := runCommand("iptables", "-C", "FORWARD", "-i", bridgeName, "-o", "ens4", "-j", "ACCEPT"); err != nil {
		// Rule doesn't exist, add it
		if err := runCommand("iptables", "-A", "FORWARD", "-i", bridgeName, "-o", "ens4", "-j", "ACCEPT"); err != nil {
			return fmt.Errorf("failed to add FORWARD rule from bridge to outside: %w", err)
		}
	}
	// Allow established/related traffic back to bridge
	if err := runCommand("iptables", "-C", "FORWARD", "-o", bridgeName, "-i", "ens4", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		// Rule doesn't exist, add it
		if err := runCommand("iptables", "-A", "FORWARD", "-o", bridgeName, "-i", "ens4", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
			return fmt.Errorf("failed to add FORWARD rule for established/related traffic to bridge: %w", err)
		}
	}

	return nil
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

// Destroy destroys a Firecracker MicroVM instance.
func (f *FirecrackerProvider) Destroy(ctx context.Context, workshopID string, seatID int) error {
	return fmt.Errorf("FirecrackerProvider Destroy not implemented")
}

// List lists all active Firecracker MicroVM instances for a workshop.
func (f *FirecrackerProvider) List(ctx context.Context, workshopID string) ([]*Instance, error) {
	return nil, fmt.Errorf("FirecrackerProvider List not implemented")
}

// GetIP returns the IP address of a Firecracker MicroVM instance.
func (f *FirecrackerProvider) GetIP(ctx context.Context, workshopID string, seatID int) (string, error) {
	return "", fmt.Errorf("FirecrackerProvider GetIP not implemented")
}
