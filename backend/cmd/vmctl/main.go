package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/clarateach/backend/internal/provisioner"
)

func main() {
	// Subcommands
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
	getCmd := flag.NewFlagSet("get", flag.ExitOnError)
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)

	// Global flags
	project := os.Getenv("GCP_PROJECT")
	zone := os.Getenv("GCP_ZONE")
	if zone == "" {
		zone = "us-central1-a"
	}
	registry := os.Getenv("GCP_REGISTRY") // e.g., us-central1-docker.pkg.dev/project/repo

	// Create flags
	createWorkshopID := createCmd.String("workshop", "", "Workshop ID (required)")
	createSeats := createCmd.Int("seats", 1, "Number of seats")
	createSpot := createCmd.Bool("spot", true, "Use spot/preemptible VMs")
	createSSHKey := createCmd.String("ssh-key", "", "SSH public key for debugging")
	createImage := createCmd.String("image", "", "Container image URL")
	createMachineType := createCmd.String("machine-type", "e2-standard-4", "GCE machine type")
	createDiskSize := createCmd.Int("disk-size", 50, "Boot disk size in GB")

	// Delete flags
	deleteWorkshopID := deleteCmd.String("workshop", "", "Workshop ID (required)")

	// Get flags
	getWorkshopID := getCmd.String("workshop", "", "Workshop ID (required)")

	// List flags
	listWorkshopID := listCmd.String("workshop", "", "Filter by workshop ID (optional)")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create":
		createCmd.Parse(os.Args[2:])
		if *createWorkshopID == "" {
			fmt.Println("Error: -workshop is required")
			createCmd.PrintDefaults()
			os.Exit(1)
		}
		if project == "" {
			fmt.Println("Error: GCP_PROJECT environment variable is required")
			os.Exit(1)
		}

		p := provisioner.NewGCPProvider(provisioner.GCPConfig{
			Project:     project,
			Zone:        zone,
			RegistryURL: registry,
		})

		cfg := provisioner.VMConfig{
			WorkshopID:     *createWorkshopID,
			Seats:          *createSeats,
			Image:          *createImage,
			MachineType:    *createMachineType,
			DiskSizeGB:     *createDiskSize,
			Spot:           *createSpot,
			SSHPublicKey:   *createSSHKey,
			EnableOpsAgent: true,
		}

		fmt.Printf("Creating VM for workshop %s...\n", *createWorkshopID)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		vm, err := p.CreateVM(ctx, cfg)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		printVM(vm)
		fmt.Printf("\nSSH: gcloud compute ssh %s --zone=%s --project=%s\n", vm.Name, zone, project)

	case "delete":
		deleteCmd.Parse(os.Args[2:])
		if *deleteWorkshopID == "" {
			fmt.Println("Error: -workshop is required")
			deleteCmd.PrintDefaults()
			os.Exit(1)
		}
		if project == "" {
			fmt.Println("Error: GCP_PROJECT environment variable is required")
			os.Exit(1)
		}

		p := provisioner.NewGCPProvider(provisioner.GCPConfig{
			Project: project,
			Zone:    zone,
		})

		fmt.Printf("Deleting VM for workshop %s...\n", *deleteWorkshopID)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := p.DeleteVM(ctx, *deleteWorkshopID); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("VM deleted successfully")

	case "get":
		getCmd.Parse(os.Args[2:])
		if *getWorkshopID == "" {
			fmt.Println("Error: -workshop is required")
			getCmd.PrintDefaults()
			os.Exit(1)
		}
		if project == "" {
			fmt.Println("Error: GCP_PROJECT environment variable is required")
			os.Exit(1)
		}

		p := provisioner.NewGCPProvider(provisioner.GCPConfig{
			Project: project,
			Zone:    zone,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		vm, err := p.GetVM(ctx, *getWorkshopID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		printVM(vm)

	case "list":
		listCmd.Parse(os.Args[2:])
		if project == "" {
			fmt.Println("Error: GCP_PROJECT environment variable is required")
			os.Exit(1)
		}

		p := provisioner.NewGCPProvider(provisioner.GCPConfig{
			Project: project,
			Zone:    zone,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		vms, err := p.ListVMs(ctx, *listWorkshopID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		if len(vms) == 0 {
			fmt.Println("No VMs found")
			return
		}

		for _, vm := range vms {
			printVM(vm)
			fmt.Println("---")
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`vmctl - ClaraTeach VM provisioner CLI

Usage:
  vmctl <command> [flags]

Commands:
  create    Create a new workshop VM
  delete    Delete a workshop VM
  get       Get details of a workshop VM
  list      List all workshop VMs

Environment Variables:
  GCP_PROJECT   GCP project ID (required)
  GCP_ZONE      GCP zone (default: us-central1-a)
  GCP_REGISTRY  Artifact Registry URL (optional)

Examples:
  # Create a VM with 2 seats
  vmctl create -workshop=test123 -seats=2 -spot=true

  # Create with SSH key for debugging
  vmctl create -workshop=test123 -ssh-key="$(cat ~/.ssh/id_rsa.pub)"

  # Get VM details
  vmctl get -workshop=test123

  # Delete VM
  vmctl delete -workshop=test123

  # List all VMs
  vmctl list`)
}

func printVM(vm *provisioner.VMInstance) {
	data, _ := json.MarshalIndent(vm, "", "  ")
	fmt.Println(string(data))
}
