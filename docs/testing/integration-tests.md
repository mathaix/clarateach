# Integration Testing Guide

This document describes how to run integration tests for the ClaraTeach MicroVM system.

## Overview

Integration tests verify that all components work together before deploying changes. They should be run:
- After modifying the init script
- After rebuilding the rootfs
- After creating a new GCP snapshot
- Before doing a full browser test

## Test Suite Location

```
scripts/test-microvm-integration.sh
```

## Quick Start

### Running on the Agent VM (Recommended)

SSH into the agent VM and run:

```bash
./scripts/test-microvm-integration.sh
```

### Running Remotely

If you have the agent's external IP:

```bash
AGENT_HOST=35.202.241.41:9090 ./scripts/test-microvm-integration.sh
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT_HOST` | auto-detect | Agent server address (e.g., `35.202.241.41:9090`) |
| `WORKSHOP_ID` | `test-integration` | Workshop ID for test MicroVM |
| `SEAT_ID` | `99` | Seat ID for test MicroVM |
| `ROOTFS_PATH` | `/var/lib/clarateach/images/rootfs.ext4` | Path to rootfs image |

## Test Breakdown

### Test 1: Rootfs Validation

**What it tests**: The rootfs.ext4 image contains all required components.

**Checks**:
- `/sbin/init` exists and is executable
- Init script contains `MICROVM_MODE=true`
- Init script contains `AUTH_DISABLED=true`
- Workspace server exists at `/home/learner/server/dist/index.js`
- Workspace server has MICROVM_MODE support

**Note**: This test only runs when executing directly on the agent VM (local access to rootfs required).

### Test 2: MicroVM Boot Test

**What it tests**: A MicroVM can be created and the workspace server starts.

**Steps**:
1. Check agent health
2. Create a test MicroVM via agent API
3. Wait for workspace server to respond on port 3001

**Success criteria**: Workspace server responds within 30 seconds.

### Test 3: Route Validation

**What it tests**: MICROVM_MODE routes are correctly registered.

**Checks**:
- `GET /health` returns 200 with status ok
- `GET /files` returns 200 with files array (MICROVM_MODE route)
- `GET /terminal` does not return 404 (endpoint exists)
- `GET /vm/1/files` returns 404 (Docker routes disabled)

**This is the critical test** - if `/files` returns 404, MICROVM_MODE is not set correctly.

### Test 4: Agent Proxy Test

**What it tests**: Agent correctly proxies requests to MicroVM.

**Checks**:
- `GET /proxy/{workshop}/{seat}/files` works through agent
- Agent routes to correct MicroVM based on seat ID

### Test 5: Cleanup

**What it does**: Deletes the test MicroVM to avoid resource leaks.

## Example Output

```
==============================================
  MicroVM Integration Test Suite
==============================================

[INFO] Detected local agent at localhost:9090

Configuration:
  AGENT_HOST:   localhost:9090
  WORKSHOP_ID:  test-integration
  SEAT_ID:      99
  ROOTFS_PATH:  /var/lib/clarateach/images/rootfs.ext4

[TEST] 1. Rootfs Validation
  ✓ PASS: /sbin/init exists
  ✓ PASS: /sbin/init is executable
  ✓ PASS: Init script contains MICROVM_MODE=true
  ✓ PASS: Init script contains AUTH_DISABLED=true
  ✓ PASS: Workspace server dist/index.js exists
  ✓ PASS: Workspace server has MICROVM_MODE support

[TEST] 2. MicroVM Boot Test
  ✓ PASS: Agent is healthy
[INFO] Creating test MicroVM (workshop=test-integration, seat=99)...
  ✓ PASS: MicroVM created successfully
[INFO] MicroVM IP: 192.168.100.109
[INFO] Waiting for workspace server to start...
  ✓ PASS: Workspace server is responding on port 3001

[TEST] 3. Route Validation (MICROVM_MODE routes)
  ✓ PASS: GET /health returns 200 with status ok
  ✓ PASS: GET /files returns 200 with files array (MICROVM_MODE working)
  ✓ PASS: GET /terminal does not return 404 (endpoint exists)
  ✓ PASS: GET /vm/1/files returns 404 (Docker routes correctly disabled)

[TEST] 4. Agent Proxy Test
  ✓ PASS: Agent proxy /files works correctly
  ✓ PASS: Agent proxy /health works correctly

[TEST] 5. Cleanup
[INFO] Deleting test MicroVM...
  ✓ PASS: Test MicroVM deleted

==============================================
  Test Summary
==============================================
  Passed: 14
  Failed: 0

All tests passed! Ready for browser testing.
```

## Interpreting Failures

### "Init script missing MICROVM_MODE=true"

**Problem**: The init script in the rootfs doesn't set the required environment variable.

**Solution**:
1. Update `backend/internal/rootfs/initscript.go`
2. Rebuild rootfs: `go run ./cmd/rootfs-builder/`
3. Create new snapshot

### "GET /files failed"

**Problem**: MICROVM_MODE routes not registered in workspace server.

**Possible causes**:
- `MICROVM_MODE=true` not set in init script
- Workspace server doesn't support MICROVM_MODE
- Workspace server failed to start

**Debug**:
```bash
# Check if workspace server is running
curl http://192.168.100.109:3001/health

# Check what routes are available
curl http://192.168.100.109:3002/
```

### "Workspace server did not start"

**Problem**: MicroVM booted but workspace server isn't responding.

**Possible causes**:
- Init script has `set -e` causing early exit
- Node.js not installed in rootfs
- Workspace server dist files missing

**Debug**:
```bash
# Check Firecracker logs
cat /var/log/firecracker-test-integration-99.log

# Check if process is running
ps aux | grep firecracker
```

### "Agent not reachable"

**Problem**: Cannot connect to agent server.

**Possible causes**:
- Agent VM not running
- Agent service not started
- Firewall blocking port 9090

**Debug**:
```bash
# Check VM status
gcloud compute instances describe clara2 --zone=us-central1-b

# Check agent service
sudo systemctl status clarateach-agent
```

## Adding New Tests

To add a new test, follow this pattern:

```bash
test_new_feature() {
    log_test "N. New Feature Test"

    # Test logic here
    if some_condition; then
        pass "Description of what passed"
    else
        fail "Description of what failed"
    fi
}
```

Then add `test_new_feature` to the `main()` function.

## CI/CD Integration

To run tests in CI:

```bash
# Set non-interactive mode
export CI=true

# Run with specific agent
AGENT_HOST=$AGENT_IP:9090 ./scripts/test-microvm-integration.sh

# Check exit code
if [ $? -eq 0 ]; then
    echo "Tests passed, proceeding with deployment"
else
    echo "Tests failed, aborting deployment"
    exit 1
fi
```

## Manual Testing Checklist

After integration tests pass, do a manual browser test:

1. [ ] Create new workshop via UI
2. [ ] Wait for VM provisioning
3. [ ] Register for workshop
4. [ ] Launch workspace
5. [ ] Verify terminal shows prompt
6. [ ] Run a command in terminal
7. [ ] Create a file via terminal
8. [ ] Refresh file explorer
9. [ ] Click file to open in editor
10. [ ] Verify syntax highlighting works

## Related Documentation

- [Architecture Overview](../architecture/firecracker-microvm.md)
- [Init Script Requirements](../development/init-script.md)
- [Troubleshooting Guide](../troubleshooting/workspace-issues.md)
- [Update GCP Snapshot](../operations/update-gcp-snapshot.md)
