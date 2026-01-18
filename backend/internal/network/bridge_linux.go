//go:build linux

// Package network handles network setup for the agent.
package network

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// BridgeConfig holds configuration for the MicroVM bridge network.
type BridgeConfig struct {
	BridgeName string // e.g., "fcbr0"
	BridgeIP   string // e.g., "192.168.100.1/24"
}

// DefaultBridgeConfig returns the default bridge configuration.
func DefaultBridgeConfig() BridgeConfig {
	return BridgeConfig{
		BridgeName: "fcbr0",
		BridgeIP:   "192.168.100.1/24",
	}
}

// SetupBridge ensures the MicroVM bridge network exists.
// This replaces the setup-microvm-network.sh script.
func SetupBridge(cfg BridgeConfig) error {
	log.Printf("[network] Setting up bridge %s with IP %s", cfg.BridgeName, cfg.BridgeIP)

	// Check if bridge already exists
	if bridgeExists(cfg.BridgeName) {
		log.Printf("[network] Bridge %s already exists", cfg.BridgeName)
		return nil
	}

	// Create bridge
	if err := runCmd("ip", "link", "add", "name", cfg.BridgeName, "type", "bridge"); err != nil {
		return fmt.Errorf("failed to create bridge: %w", err)
	}
	log.Printf("[network] Created bridge %s", cfg.BridgeName)

	// Add IP address
	if err := runCmd("ip", "addr", "add", cfg.BridgeIP, "dev", cfg.BridgeName); err != nil {
		// Ignore if address already exists
		if !strings.Contains(err.Error(), "RTNETLINK answers: File exists") {
			return fmt.Errorf("failed to add IP to bridge: %w", err)
		}
	}
	log.Printf("[network] Added IP %s to bridge", cfg.BridgeIP)

	// Bring bridge up
	if err := runCmd("ip", "link", "set", cfg.BridgeName, "up"); err != nil {
		return fmt.Errorf("failed to bring bridge up: %w", err)
	}
	log.Printf("[network] Bridge %s is up", cfg.BridgeName)

	return nil
}

// bridgeExists checks if a network interface exists.
func bridgeExists(name string) bool {
	err := runCmd("ip", "link", "show", name)
	return err == nil
}

// runCmd executes a command and returns any error.
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %w (output: %s)", name, args, err, string(output))
	}
	return nil
}
