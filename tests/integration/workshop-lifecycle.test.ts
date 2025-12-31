/**
 * Workshop Lifecycle Integration Tests
 *
 * Tests the complete lifecycle of workshops:
 * - Create workshop
 * - Start workshop (provision containers)
 * - Stop workshop (destroy containers)
 * - Delete workshop
 *
 * Prerequisites: Services must be running (`./scripts/stack.sh start`)
 */

import { describe, it, expect, beforeAll, afterAll, afterEach } from 'vitest';
import {
  portal,
  generateTestId,
  cleanupWorkshop,
  waitForWorkshopStatus,
  waitForWorkspaceReady,
  sleep,
} from './helpers.js';

describe('Workshop Lifecycle', () => {
  const createdWorkshops: string[] = [];

  beforeAll(async () => {
    // Verify portal is running
    const health = await portal.health();
    expect(health.status).toBe(200);
    expect(health.body.status).toBe('ok');
  });

  afterEach(async () => {
    // Clean up any workshops created during tests
    for (const id of createdWorkshops) {
      await cleanupWorkshop(id);
    }
    createdWorkshops.length = 0;
  });

  describe('POST /api/workshops (Create Workshop)', () => {
    it('should create a new workshop with valid input', async () => {
      const name = `Test Workshop ${generateTestId()}`;
      const result = await portal.createWorkshop(name, 5, 'sk-ant-test-key-12345');

      expect(result.status).toBe(201);
      expect(result.body.workshop).toBeDefined();
      expect(result.body.workshop.name).toBe(name);
      expect(result.body.workshop.seats).toBe(5);
      expect(result.body.workshop.status).toBe('created');
      expect(result.body.workshop.code).toMatch(/^[A-Z0-9]{4}-[A-Z0-9]{4}$/);

      createdWorkshops.push(result.body.workshop.id);
    });

    it('should reject workshop with empty name', async () => {
      const result = await portal.createWorkshop('', 5, 'sk-ant-test-key-12345');

      expect(result.status).toBe(400);
      expect((result.body as { error: { code: string } }).error.code).toBe('INVALID_INPUT');
    });

    it('should reject workshop with too many seats', async () => {
      const result = await portal.createWorkshop('Test', 100, 'sk-ant-test-key-12345');

      expect(result.status).toBe(400);
    });

    it('should reject workshop without API key', async () => {
      const result = await portal.createWorkshop('Test', 5, '');

      expect(result.status).toBe(400);
    });

    it('should create multiple workshops independently', async () => {
      const result1 = await portal.createWorkshop(`Workshop 1 ${generateTestId()}`, 3, 'key1-123456789');
      const result2 = await portal.createWorkshop(`Workshop 2 ${generateTestId()}`, 5, 'key2-123456789');

      expect(result1.status).toBe(201);
      expect(result2.status).toBe(201);
      expect(result1.body.workshop.id).not.toBe(result2.body.workshop.id);
      expect(result1.body.workshop.code).not.toBe(result2.body.workshop.code);

      createdWorkshops.push(result1.body.workshop.id, result2.body.workshop.id);
    });
  });

  describe('GET /api/workshops (List Workshops)', () => {
    it('should list all created workshops', async () => {
      // Create two workshops
      const w1 = await portal.createWorkshop(`List Test 1 ${generateTestId()}`, 2, 'key-1234567890');
      const w2 = await portal.createWorkshop(`List Test 2 ${generateTestId()}`, 4, 'key-0987654321');

      createdWorkshops.push(w1.body.workshop.id, w2.body.workshop.id);

      const list = await portal.listWorkshops();

      expect(list.status).toBe(200);
      expect(list.body.workshops.length).toBeGreaterThanOrEqual(2);

      const ids = list.body.workshops.map((w) => w.id);
      expect(ids).toContain(w1.body.workshop.id);
      expect(ids).toContain(w2.body.workshop.id);
    });
  });

  describe('GET /api/workshops/:id (Get Workshop)', () => {
    it('should get workshop by ID', async () => {
      const created = await portal.createWorkshop(`Get Test ${generateTestId()}`, 3, 'key-1234567890');
      createdWorkshops.push(created.body.workshop.id);

      const result = await portal.getWorkshop(created.body.workshop.id);

      expect(result.status).toBe(200);
      expect(result.body.workshop.id).toBe(created.body.workshop.id);
      expect(result.body.workshop.name).toBe(created.body.workshop.name);
    });

    it('should return 404 for non-existent workshop', async () => {
      const result = await portal.getWorkshop('ws-nonexistent');

      expect(result.status).toBe(404);
      expect((result.body as { error: { code: string } }).error.code).toBe('NOT_FOUND');
    });
  });

  describe('DELETE /api/workshops/:id (Delete Workshop)', () => {
    it('should delete a workshop', async () => {
      const created = await portal.createWorkshop(`Delete Test ${generateTestId()}`, 2, 'key-1234567890');
      const workshopId = created.body.workshop.id;

      const deleteResult = await portal.deleteWorkshop(workshopId);
      expect(deleteResult.status).toBe(200);
      expect(deleteResult.body.success).toBe(true);

      // Verify it's gone
      const getResult = await portal.getWorkshop(workshopId);
      expect(getResult.status).toBe(404);
    });

    it('should return 404 when deleting non-existent workshop', async () => {
      const result = await portal.deleteWorkshop('ws-nonexistent');

      expect(result.status).toBe(404);
    });
  });

  describe('POST /api/workshops/:id/start (Start Workshop)', () => {
    it('should start a workshop and provision containers', async () => {
      const created = await portal.createWorkshop(`Start Test ${generateTestId()}`, 2, 'key-1234567890');
      createdWorkshops.push(created.body.workshop.id);
      const workshopId = created.body.workshop.id;

      // Start the workshop
      const startResult = await portal.startWorkshop(workshopId);
      expect(startResult.status).toBe(202);
      expect(startResult.body.workshop.status).toBe('provisioning');

      // Wait for it to become running
      const workshop = await waitForWorkshopStatus(workshopId, 'running', 45000);
      expect(workshop.status).toBe('running');
      expect(workshop.vm_ip).toBeDefined();
    });

    it('should reject starting an already running workshop', async () => {
      const created = await portal.createWorkshop(`Already Running ${generateTestId()}`, 1, 'key-1234567890');
      createdWorkshops.push(created.body.workshop.id);
      const workshopId = created.body.workshop.id;

      // Start it
      await portal.startWorkshop(workshopId);
      await waitForWorkshopStatus(workshopId, 'running', 45000);

      // Try to start again
      const result = await portal.startWorkshop(workshopId);
      expect(result.status).toBe(400);
      expect((result.body as { error: { code: string } }).error.code).toBe('ALREADY_RUNNING');
    });
  });

  describe('POST /api/workshops/:id/stop (Stop Workshop)', () => {
    it('should stop a running workshop', async () => {
      const created = await portal.createWorkshop(`Stop Test ${generateTestId()}`, 1, 'key-1234567890');
      createdWorkshops.push(created.body.workshop.id);
      const workshopId = created.body.workshop.id;

      // Start and wait
      await portal.startWorkshop(workshopId);
      await waitForWorkshopStatus(workshopId, 'running', 45000);

      // Stop it
      const stopResult = await portal.stopWorkshop(workshopId);
      expect(stopResult.status).toBe(200);
      expect(stopResult.body.success).toBe(true);

      // Wait for stopped status
      const workshop = await waitForWorkshopStatus(workshopId, 'stopped', 30000);
      expect(workshop.status).toBe('stopped');
    });

    it('should reject stopping a non-running workshop', async () => {
      const created = await portal.createWorkshop(`Not Running ${generateTestId()}`, 1, 'key-1234567890');
      createdWorkshops.push(created.body.workshop.id);

      const result = await portal.stopWorkshop(created.body.workshop.id);
      expect(result.status).toBe(400);
      expect((result.body as { error: { code: string } }).error.code).toBe('NOT_RUNNING');
    });
  });

  describe('Complete Lifecycle', () => {
    it('should handle full create -> start -> stop -> delete cycle', async () => {
      // Create
      const createResult = await portal.createWorkshop(`Full Lifecycle ${generateTestId()}`, 2, 'key-1234567890');
      expect(createResult.status).toBe(201);
      const workshopId = createResult.body.workshop.id;

      try {
        // Start
        const startResult = await portal.startWorkshop(workshopId);
        expect(startResult.status).toBe(202);

        // Wait for running
        await waitForWorkshopStatus(workshopId, 'running', 45000);

        // Stop
        const stopResult = await portal.stopWorkshop(workshopId);
        expect(stopResult.status).toBe(200);

        // Wait for stopped
        await waitForWorkshopStatus(workshopId, 'stopped', 30000);

        // Delete
        const deleteResult = await portal.deleteWorkshop(workshopId);
        expect(deleteResult.status).toBe(200);

        // Verify gone
        const getResult = await portal.getWorkshop(workshopId);
        expect(getResult.status).toBe(404);
      } catch (err) {
        // Clean up on failure
        await cleanupWorkshop(workshopId);
        throw err;
      }
    });
  });
});
