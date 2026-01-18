//go:build !linux

// Package network handles network setup for the agent.
package network

import "log"

// BridgeConfig holds configuration for the MicroVM bridge network.
type BridgeConfig struct {
	BridgeName string
	BridgeIP   string
}

// DefaultBridgeConfig returns the default bridge configuration.
func DefaultBridgeConfig() BridgeConfig {
	return BridgeConfig{
		BridgeName: "fcbr0",
		BridgeIP:   "192.168.100.1/24",
	}
}

// SetupBridge is a no-op on non-Linux platforms.
func SetupBridge(cfg BridgeConfig) error {
	log.Printf("[network] Bridge setup skipped (not Linux)")
	return nil
}
