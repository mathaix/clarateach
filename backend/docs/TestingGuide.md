# Testing Guide - Firecracker MicroVMs

This guide explains how to test the Firecracker MicroVM flow on ClaraTeach.

## Architecture Overview

Think of it like Russian nesting dolls:

```
┌─────────────────────────────────────────────┐
│  GCP VM (clara2)                            │
│  - A virtual machine in Google Cloud        │
│  - Has nested virtualization enabled        │
│                                             │
│  ┌───────────────────────────────────────┐  │
│  │  Agent (port 9090)                    │  │
│  │  - A Go program that manages VMs      │  │
│  │  - Runs as a systemd service          │  │
│  └───────────────────────────────────────┘  │
│                                             │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐       │
│  │MicroVM 1│ │MicroVM 2│ │MicroVM 3│       │
│  │  .11    │ │  .12    │ │  .13    │       │
│  └─────────┘ └─────────┘ └─────────┘       │
│                                             │
│  These are tiny VMs inside the big VM!      │
└─────────────────────────────────────────────┘
```

## Prerequisites

- Access to the `clarateach` GCP project
- SSH access to clara2 VM

## Step-by-Step Testing

### 1. SSH into clara2

```bash
# From your local machine
gcloud compute ssh clara2 --zone=us-central1-b --project=clarateach
```

### 2. Go to the backend directory

```bash
cd ~/clarateach/backend
```

### 3. Check if the agent is running

```bash
# This should show "active (running)"
sudo systemctl status clarateach-agent

# Or test the health endpoint directly
curl localhost:9090/health
```

You should see something like:
```json
{"status":"healthy","worker_id":"clara2","vm_count":0,"capacity":50,"uptime_seconds":123}
```

### 4. Run the E2E test script

```bash
./scripts/test-e2e-local.sh
```

This script does everything automatically:
1. Checks if the agent is healthy
2. Creates 3 MicroVMs
3. Verifies they're running
4. Pings each MicroVM
5. Tests the proxy endpoints
6. Cleans up (deletes the VMs)

Expected output:
```
==============================================
ClaraTeach E2E Test - Local Firecracker
==============================================
Agent URL: http://localhost:9090
Workshop ID: e2e-test-1234567890
Seats: 3

=== Test 1: Agent Health Check ===
[✓] Agent is healthy (worker: clara2)

=== Test 2: Create MicroVMs ===
[✓] Created VM seat 1 with IP 192.168.100.11
[✓] Created VM seat 2 with IP 192.168.100.12
[✓] Created VM seat 3 with IP 192.168.100.13

...

==============================================
Test Summary
==============================================
Passed: 14
Failed: 0

All tests passed!
```

### 5. Manual Testing

If you want to understand each step individually:

```bash
# Create a single MicroVM
curl -X POST localhost:9090/vms \
  -H "Content-Type: application/json" \
  -d '{"workshop_id": "my-test", "seat_id": 1}'

# List all VMs
curl localhost:9090/vms

# List VMs for a specific workshop
curl "localhost:9090/vms?workshop_id=my-test"

# Ping the MicroVM (seat 1 = IP .11)
ping -c 3 192.168.100.11

# Check proxy health (shows if services inside MicroVM are running)
curl localhost:9090/proxy/my-test/1/health

# Delete the VM when done
curl -X DELETE localhost:9090/vms/my-test/1
```

## Understanding IP Addresses

Each MicroVM gets an IP based on its seat ID:

| Seat ID | IP Address |
|---------|------------|
| 1 | 192.168.100.11 |
| 2 | 192.168.100.12 |
| 3 | 192.168.100.13 |
| N | 192.168.100.(10+N) |

The network bridge (`clarateach0`) has IP `192.168.100.1`.

## Understanding Proxy Health

When you check proxy health:
```bash
curl localhost:9090/proxy/my-test/1/health
```

You might see:
```json
{"workshop_id":"my-test","seat_id":1,"vm_ip":"192.168.100.11","status":"unhealthy","terminal":false,"files":false}
```

This is **expected** if the MicroVM services aren't running yet. It means:
- `terminal: false` - Terminal server (port 3001) not responding
- `files: false` - File server (port 3002) not responding

The MicroVM itself is running, but the services inside it haven't started.

## Agent API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Check agent health (no auth required) |
| `/info` | GET | Get worker info |
| `/vms` | GET | List all VMs |
| `/vms?workshop_id=X` | GET | List VMs for a workshop |
| `/vms` | POST | Create a new VM |
| `/vms/{workshopID}/{seatID}` | GET | Get VM details |
| `/vms/{workshopID}/{seatID}` | DELETE | Destroy a VM |
| `/proxy/{workshopID}/{seatID}/health` | GET | Check MicroVM services health |
| `/proxy/{workshopID}/{seatID}/terminal` | WebSocket | Terminal proxy |
| `/proxy/{workshopID}/{seatID}/files/*` | HTTP | File server proxy |

## Troubleshooting

### Agent not running?

```bash
# Start it
sudo systemctl start clarateach-agent

# Check logs if it fails
sudo journalctl -u clarateach-agent -n 50

# Watch logs in real-time
sudo journalctl -u clarateach-agent -f
```

### Can't create VMs?

```bash
# Check if kernel and rootfs exist
ls -la /var/lib/clarateach/images/

# Should show:
# vmlinux      (the Linux kernel)
# rootfs.ext4  (the filesystem for MicroVMs)
```

### Can't ping MicroVMs?

```bash
# Check if the network bridge exists
ip link show clarateach0

# Check bridge IP
ip addr show clarateach0

# Should show 192.168.100.1/24
```

### Port 9090 already in use?

```bash
# Find what's using the port
sudo ss -tlnp | grep 9090

# Kill the old process if needed
sudo kill <PID>

# Restart the service
sudo systemctl restart clarateach-agent
```

## Quick Reference

| Command | What it does |
|---------|--------------|
| `./scripts/test-e2e-local.sh` | Run all tests automatically |
| `curl localhost:9090/health` | Check if agent is healthy |
| `curl localhost:9090/vms` | List all MicroVMs |
| `sudo systemctl status clarateach-agent` | Check agent service status |
| `sudo systemctl restart clarateach-agent` | Restart the agent |
| `sudo journalctl -u clarateach-agent -f` | Watch agent logs in real-time |

## Test Script Options

The E2E test script accepts environment variables:

```bash
# Custom workshop ID
WORKSHOP_ID=my-workshop ./scripts/test-e2e-local.sh

# Different number of seats
SEATS=5 ./scripts/test-e2e-local.sh

# Custom agent URL (if not localhost)
AGENT_URL=http://192.168.1.100:9090 ./scripts/test-e2e-local.sh

# With authentication token
AGENT_TOKEN=secret123 ./scripts/test-e2e-local.sh
```
