# Testing Guide - Firecracker MicroVMs

This guide explains how to test the Firecracker MicroVM flow on ClaraTeach.

## Quick Start

There are two test scripts available:

| Script | What it tests | Where to run |
|--------|--------------|--------------|
| `./scripts/test-e2e-local.sh` | Agent + MicroVMs only | On clara2 (the worker VM) |
| `./scripts/test-e2e-gcp.sh` | Full flow: Backend → GCP → Agent → MicroVMs | Anywhere with backend access |

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

### GCP VM Scopes (for E2E tests)

If running the full GCP E2E test (`test-e2e-gcp.sh`) from a GCE VM, the VM must have sufficient OAuth scopes to create/manage other VMs. If you see this error:

```
googleapi: Error 403: Request had insufficient authentication scopes.
```

**Fix: Update VM scopes**

From a machine with proper GCP access (or use Cloud Console):

```bash
# Get VM name
VM_NAME=<your-vm-name>

# Stop the VM
gcloud compute instances stop $VM_NAME --zone=us-central1-b

# Set scopes (includes compute read-write)
gcloud compute instances set-service-account $VM_NAME \
  --zone=us-central1-b \
  --scopes=compute-rw,storage-ro,logging-write,monitoring

# Start the VM
gcloud compute instances start $VM_NAME --zone=us-central1-b
```

**Alternative: Via Cloud Console**
1. Go to Compute Engine → VM instances
2. Stop the VM
3. Edit → Service account → Set access scopes to "Allow full access to all Cloud APIs"
4. Start the VM

**Alternative: Use a service account key**
```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json
```

## Step-by-Step Testing

### 1. SSH into clara2

```bash
# From your local machine
gcloud compute ssh clara2 --zone=us-central1-b --project=clarateach
```

### 2. Go to the project directory

```bash
cd ~/clarateach
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

## Accessing the User Interface

Once MicroVMs are running, users can access two interfaces:

### Terminal (WebSocket on port 3001)

The terminal provides a web-based shell into the MicroVM workspace.

**Via Proxy (Recommended):**
```
ws://<agent-ip>:9090/proxy/<workshop-id>/<seat-id>/terminal
```

**Direct (internal network only):**
```
ws://192.168.100.<10+seat>/3001
```

Example with websocat:
```bash
# Install websocat
cargo install websocat

# Connect to terminal
websocat ws://34.68.136.93:9090/proxy/my-workshop/1/terminal
```

### Files API (HTTP on port 3002)

The file server provides a REST API for file operations.

**Via Proxy (Recommended):**
```
http://<agent-ip>:9090/proxy/<workshop-id>/<seat-id>/files/
```

**Direct (internal network only):**
```
http://192.168.100.<10+seat>:3002/
```

**API Endpoints:**

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/files/` | List files in workspace |
| GET | `/files/<path>` | Read file content |
| POST | `/files/<path>` | Create/update file |
| DELETE | `/files/<path>` | Delete file |

Example:
```bash
# List files
curl http://34.68.136.93:9090/proxy/my-workshop/1/files/

# Create a file
curl -X POST http://34.68.136.93:9090/proxy/my-workshop/1/files/hello.txt \
  -H "Content-Type: text/plain" \
  -d "Hello, World!"

# Read the file
curl http://34.68.136.93:9090/proxy/my-workshop/1/files/hello.txt
```

## Full GCP Test Script

For testing the complete flow from backend API to working MicroVMs:

```bash
# Start the backend first (from the backend directory)
cd ~/clarateach/backend
GCP_PROJECT=clarateach \
GCP_ZONE=us-central1-b \
GCP_REGISTRY=us-central1-docker.pkg.dev/clarateach/clarateach \
FC_SNAPSHOT_NAME=clara2-snapshot \
go run ./cmd/server/

# In another terminal, run the full E2E test (from the project root)
cd ~/clarateach
./scripts/test-e2e-gcp.sh
```

The script will:
1. Create a workshop via the backend API
2. Wait for GCP VM to be provisioned
3. Verify agent is healthy
4. Check MicroVM services
5. Display access URLs for each seat
6. Wait for you to press Enter before cleanup

---

## Testing Frontend with Firecracker MicroVMs

This section explains how to test the frontend (Editor + Terminal) connecting to Firecracker MicroVMs instead of Docker containers.

### Overview

The frontend detects `runtime_type` from the session and routes requests accordingly:

| Runtime Type | API Path | WebSocket Path |
|--------------|----------|----------------|
| Docker | `/vm/{seat}/files` | `/vm/{seat}/terminal` |
| Firecracker | `/proxy/{workshopID}/{seatID}/files` | `/proxy/{workshopID}/{seatID}/terminal` |

### Prerequisites

1. **clara2 VM running** with the agent service
2. **MicroVM rootfs** built with the workspace server (MICROVM_MODE=true)
3. **Backend** running locally or accessible
4. **Frontend** running locally

### Step 1: Verify Agent is Running on clara2

```bash
# SSH into clara2
gcloud compute ssh clara2 --zone=us-central1-b --project=clarateach

# Check agent status
sudo systemctl status clarateach-agent
curl localhost:9090/health
```

### Step 2: Create MicroVMs for Testing

```bash
# On clara2, create test VMs
curl -X POST localhost:9090/vms \
  -H "Content-Type: application/json" \
  -d '{"workshop_id": "frontend-test", "seat_id": 1}'

curl -X POST localhost:9090/vms \
  -H "Content-Type: application/json" \
  -d '{"workshop_id": "frontend-test", "seat_id": 2}'

# Verify they're running
curl localhost:9090/vms

# Check if workspace services are responding
curl localhost:9090/proxy/frontend-test/1/health
```

Expected health response when services are running:
```json
{"workshop_id":"frontend-test","seat_id":1,"vm_ip":"192.168.100.11","status":"healthy","terminal":true,"files":true}
```

### Step 3: Start Backend with Firecracker Support

```bash
cd ~/clarateach/backend

# Start backend (it will auto-detect Firecracker provisioner on Linux with KVM)
AUTH_DISABLED=true \
GCP_PROJECT=clarateach \
GCP_ZONE=us-central1-b \
go run ./cmd/server/
```

### Step 4: Create a Firecracker Workshop via API

```bash
# Create a workshop with runtime_type=firecracker
curl -X POST http://localhost:8080/api/workshops \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Frontend Test Workshop",
    "seats": 3,
    "runtime_type": "firecracker"
  }'
```

Note the workshop `code` in the response (e.g., `ABC12-XYZ9`).

### Step 5: Register and Get Session

```bash
# Register for the workshop
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "workshop_code": "ABC12-XYZ9",
    "email": "test@example.com",
    "name": "Test User"
  }'

# Note the access_code in response (e.g., "XYZ-1234")

# Get session details
curl http://localhost:8080/api/session/XYZ-1234
```

Expected response:
```json
{
  "status": "ready",
  "endpoint": "http://<vm-ip>:9090",
  "seat": 1,
  "name": "Test User",
  "workshop_id": "ws-abc123",
  "runtime_type": "firecracker"
}
```

Key fields to verify:
- `runtime_type` is `"firecracker"`
- `endpoint` points to port `9090` (agent port, not 8080)

### Step 6: Start Frontend and Test

```bash
cd ~/clarateach/frontend
npm run dev
```

Open browser to `http://localhost:5173/session/XYZ-1234`

**What to verify:**

1. **File Explorer loads** - Should show files from MicroVM's `/workspace` directory
2. **Terminal connects** - Should get a bash shell inside the MicroVM
3. **File editing works** - Create/edit/save files
4. **Console shows correct paths** - Open browser DevTools Network tab:
   - File requests go to `/proxy/{workshopID}/{seatID}/files/...`
   - WebSocket connects to `/proxy/{workshopID}/{seatID}/terminal`

### Step 7: Manual API Testing

Test the proxy endpoints directly to verify routing:

```bash
# Get clara2's external IP
AGENT_IP=$(gcloud compute instances describe clara2 --zone=us-central1-b --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

# List files (should return workspace contents)
curl "http://$AGENT_IP:9090/proxy/frontend-test/1/files"

# Read a file
curl "http://$AGENT_IP:9090/proxy/frontend-test/1/files/README.md"

# Create a file
curl -X PUT "http://$AGENT_IP:9090/proxy/frontend-test/1/files/test.txt" \
  -H "Content-Type: application/json" \
  -d '{"content": "Hello from test!"}'

# Test WebSocket (requires websocat)
websocat "ws://$AGENT_IP:9090/proxy/frontend-test/1/terminal"
```

### Step 8: Cleanup

```bash
# On clara2, delete test VMs
curl -X DELETE localhost:9090/vms/frontend-test/1
curl -X DELETE localhost:9090/vms/frontend-test/2

# Verify they're gone
curl localhost:9090/vms
```

### Troubleshooting Frontend Integration

#### Files not loading?

1. Check browser DevTools → Network tab for failed requests
2. Verify the request URL contains `/proxy/{workshopID}/{seatID}/files`
3. Check CORS headers - agent should allow cross-origin requests

```bash
# Test CORS preflight
curl -X OPTIONS "http://$AGENT_IP:9090/proxy/frontend-test/1/files" \
  -H "Origin: http://localhost:5173" \
  -H "Access-Control-Request-Method: GET" -v
```

#### Terminal not connecting?

1. Check browser DevTools → Network tab → WS for WebSocket connection
2. Verify WebSocket URL is `ws://<ip>:9090/proxy/{workshopID}/{seatID}/terminal`
3. Check agent logs for connection attempts:
   ```bash
   sudo journalctl -u clarateach-agent -f | grep -i terminal
   ```

#### "VM not found" errors?

1. Verify MicroVMs are running:
   ```bash
   curl localhost:9090/vms
   ```
2. Check the workshop_id and seat_id match what was created

#### Auth errors from workspace server?

The MicroVM's workspace server should have `AUTH_DISABLED=true` for testing:
```bash
# Check environment in MicroVM (from clara2)
# The rootfs should be built with AUTH_DISABLED=true
```

### MicroVM Rootfs Requirements

For the frontend to work with MicroVMs, the rootfs must include:

1. **Workspace server** with `MICROVM_MODE=true` - Routes are `/files` and `/terminal` (no `/vm/:seat` prefix)
2. **AUTH_DISABLED=true** - Or proper JWT validation configured
3. **Ports exposed**:
   - 3001: Terminal server (WebSocket)
   - 3002: File server (HTTP)

Build the rootfs with these settings:
```bash
cd ~/clarateach
sudo ./scripts/rootfs-builder/build-rootfs.sh
```

The build script should set in the rootfs's systemd service:
```ini
Environment=MICROVM_MODE=true
Environment=AUTH_DISABLED=true
Environment=WORKSPACE_DIR=/workspace
```

### Expected Request Flow

```
Browser                 Backend               Agent (clara2)           MicroVM
   │                       │                       │                      │
   │ GET /api/session/code │                       │                      │
   │──────────────────────>│                       │                      │
   │                       │                       │                      │
   │ {runtime_type:        │                       │                      │
   │  "firecracker",       │                       │                      │
   │  endpoint: ":9090"}   │                       │                      │
   │<──────────────────────│                       │                      │
   │                       │                       │                      │
   │ GET /proxy/ws/1/files ──────────────────────>│                      │
   │                       │                       │ GET /files           │
   │                       │                       │─────────────────────>│
   │                       │                       │                      │
   │                       │                       │ {files: [...]}       │
   │                       │                       │<─────────────────────│
   │ {files: [...]}        │                       │                      │
   │<─────────────────────────────────────────────│                      │
   │                       │                       │                      │
   │ WS /proxy/ws/1/terminal ────────────────────>│                      │
   │                       │                       │ WS /terminal         │
   │                       │                       │─────────────────────>│
   │ <──────────── bidirectional ────────────────────────────────────────>│
```
