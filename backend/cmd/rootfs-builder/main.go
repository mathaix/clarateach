// Package main implements the rootfs-builder CLI tool.
// This tool creates ext4 rootfs images for Firecracker MicroVMs from Docker images.
//
// Usage:
//
//	rootfs-builder --image <docker-image> --output <path>
//	rootfs-builder --dockerfile <path> --output <path>
//
// Examples:
//
//	# Build from existing Docker image
//	rootfs-builder --image clarateach-workspace --output /var/lib/clarateach/images/rootfs.ext4
//
//	# Build from Dockerfile
//	rootfs-builder --dockerfile ./workspace/Dockerfile --output ./rootfs.ext4 --size 4G
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/clarateach/backend/internal/rootfs"
	"github.com/sirupsen/logrus"
)

func main() {
	// Define flags
	dockerImage := flag.String("image", "", "Docker image name to export (e.g., clarateach-workspace)")
	dockerfile := flag.String("dockerfile", "", "Path to Dockerfile to build (optional)")
	dockerContext := flag.String("context", "", "Docker build context directory (defaults to Dockerfile's directory)")
	output := flag.String("output", "", "Output path for rootfs.ext4 (required)")
	size := flag.String("size", "2G", "Size of the rootfs image (e.g., 2G, 4G)")
	initScript := flag.String("init-script", "", "Path to custom init script (optional)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "rootfs-builder - Build Firecracker rootfs images from Docker\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  rootfs-builder --image <name> --output <path>\n")
		fmt.Fprintf(os.Stderr, "  rootfs-builder --dockerfile <path> --output <path>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  rootfs-builder --image clarateach-workspace --output ./rootfs.ext4\n")
		fmt.Fprintf(os.Stderr, "  rootfs-builder --dockerfile ./workspace/Dockerfile --image myimage --output ./rootfs.ext4 --size 4G\n")
	}

	flag.Parse()

	// Validate flags
	if *output == "" {
		fmt.Fprintln(os.Stderr, "Error: --output is required")
		flag.Usage()
		os.Exit(1)
	}

	if *dockerImage == "" && *dockerfile == "" {
		fmt.Fprintln(os.Stderr, "Error: either --image or --dockerfile is required")
		flag.Usage()
		os.Exit(1)
	}

	// If only dockerfile is provided, derive image name
	if *dockerImage == "" && *dockerfile != "" {
		*dockerImage = "rootfs-build-temp"
	}

	// Read custom init script if provided
	var customInitScript string
	if *initScript != "" {
		data, err := os.ReadFile(*initScript)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading init script: %v\n", err)
			os.Exit(1)
		}
		customInitScript = string(data)
	}

	// Create builder
	builder := rootfs.NewBuilder()
	if *verbose {
		builder.SetLogLevel(logrus.DebugLevel)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted, cleaning up...")
		cancel()
	}()

	// Build config
	cfg := rootfs.Config{
		DockerImage:    *dockerImage,
		DockerfilePath: *dockerfile,
		DockerContext:  *dockerContext,
		OutputPath:     *output,
		Size:           *size,
		InitScript:     customInitScript,
	}

	fmt.Printf("Building rootfs image...\n")
	fmt.Printf("  Docker Image: %s\n", cfg.DockerImage)
	if cfg.DockerfilePath != "" {
		fmt.Printf("  Dockerfile:   %s\n", cfg.DockerfilePath)
	}
	fmt.Printf("  Output:       %s\n", cfg.OutputPath)
	fmt.Printf("  Size:         %s\n", cfg.Size)
	fmt.Println()

	// Run build
	if err := builder.Build(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSuccess! Rootfs image created at: %s\n", cfg.OutputPath)
}
