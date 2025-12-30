/**
 * End-to-End Flow Integration Tests
 *
 * Tests the complete user flow:
 * - Instructor creates workshop
 * - Instructor starts workshop
 * - Learner joins with code
 * - Learner uses workspace (files, terminal)
 * - Learner reconnects using odehash
 * - Multiple learners join
 * - Instructor stops workshop
 *
 * Prerequisites: Services must be running (`./scripts/stack.sh start`)
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { WebSocket } from 'ws';
import {
  portal,
  workspace,
  generateTestId,
  cleanupWorkshop,
  waitForWorkshopStatus,
  waitForWorkspaceReady,
  sleep,
  WORKSPACE_URL,
} from './helpers.js';

describe('End-to-End Flow', () => {
  describe('Complete Learner Journey', () => {
    let workshopId: string;
    let workshopCode: string;

    afterAll(async () => {
      if (workshopId) {
        await cleanupWorkshop(workshopId);
      }
    });

    it('should complete full learner flow from join to work to disconnect', async () => {
      // === INSTRUCTOR: Create Workshop ===
      const createResult = await portal.createWorkshop(
        `E2E Test ${generateTestId()}`,
        3,
        'sk-ant-e2e-test-key-12345'
      );
      expect(createResult.status).toBe(201);
      workshopId = createResult.body.workshop.id;
      workshopCode = createResult.body.workshop.code;

      // === INSTRUCTOR: Start Workshop ===
      const startResult = await portal.startWorkshop(workshopId);
      expect(startResult.status).toBe(202);

      // Wait for running
      await waitForWorkshopStatus(workshopId, 'running', 60000);

      // Wait for workspace to be ready
      await waitForWorkspaceReady(1, 30000);

      // === LEARNER 1: Join Workshop ===
      const join1 = await portal.join(workshopCode, 'Alice');
      expect(join1.status).toBe(200);
      expect(join1.body.seat).toBe(1);
      expect(join1.body.token).toBeDefined();
      expect(join1.body.odehash).toMatch(/^[a-z0-9]{5}$/);
      expect(join1.body.endpoint).toBeDefined();

      const aliceToken = join1.body.token;
      const aliceOdehash = join1.body.odehash;

      // === LEARNER 1: Create a file ===
      const fileName = 'hello.py';
      const fileContent = 'print("Hello from Alice!")';

      const writeResult = await workspace.writeFile(1, fileName, fileContent, aliceToken);
      expect(writeResult.status).toBe(200);

      // === LEARNER 1: Verify file exists ===
      const listResult = await workspace.listFiles(1, undefined, aliceToken);
      expect(listResult.status).toBe(200);
      expect(listResult.body.files.some(f => f.name === fileName)).toBe(true);

      // === LEARNER 1: Read file back ===
      const readResult = await workspace.readFile(1, fileName, aliceToken);
      expect(readResult.status).toBe(200);
      expect(readResult.body.content).toBe(fileContent);

      // === LEARNER 1: Simulate disconnect and reconnect ===
      const reconnect1 = await portal.join(workshopCode, undefined, aliceOdehash);
      expect(reconnect1.status).toBe(200);
      expect(reconnect1.body.seat).toBe(1); // Same seat
      expect(reconnect1.body.odehash).toBe(aliceOdehash); // Same odehash

      // File should still exist after reconnect
      const afterReconnect = await workspace.readFile(1, fileName, reconnect1.body.token);
      expect(afterReconnect.status).toBe(200);
      expect(afterReconnect.body.content).toBe(fileContent);

      // === INSTRUCTOR: Check connected learners ===
      const learnersResult = await portal.getLearners(workshopId);
      expect(learnersResult.status).toBe(200);
      expect(learnersResult.body.learners.length).toBe(1);
      expect(learnersResult.body.learners[0].name).toBe('Alice');
      expect(learnersResult.body.learners[0].seat).toBe(1);

      // === LEARNER 2: Join Workshop ===
      await waitForWorkspaceReady(2, 30000);
      const join2 = await portal.join(workshopCode, 'Bob');
      expect(join2.status).toBe(200);
      expect(join2.body.seat).toBe(2);

      // === LEARNER 2: Create file in their workspace ===
      const bobToken = join2.body.token;
      const bobFileName = 'bob.txt';
      await workspace.writeFile(2, bobFileName, 'Hello from Bob', bobToken);

      // === INSTRUCTOR: Check both learners ===
      const allLearners = await portal.getLearners(workshopId);
      expect(allLearners.body.learners.length).toBe(2);

      // === CLEANUP: Learner deletes file ===
      const deleteResult = await workspace.deleteFile(1, fileName, aliceToken);
      expect(deleteResult.status).toBe(200);

      // === INSTRUCTOR: Stop Workshop ===
      const stopResult = await portal.stopWorkshop(workshopId);
      expect(stopResult.status).toBe(200);

      await waitForWorkshopStatus(workshopId, 'stopped', 30000);

      // === Verify workshop is stopped ===
      const finalStatus = await portal.getWorkshop(workshopId);
      expect(finalStatus.body.workshop.status).toBe('stopped');
    }, 180000); // 3 minute timeout for full E2E test
  });

  describe('Multi-Learner Scenarios', () => {
    let workshopId: string;
    let workshopCode: string;

    afterAll(async () => {
      if (workshopId) {
        await cleanupWorkshop(workshopId);
      }
    });

    it('should handle multiple learners joining simultaneously', async () => {
      // Create and start workshop
      const createResult = await portal.createWorkshop(
        `Multi-Learner ${generateTestId()}`,
        5,
        'sk-ant-multi-test-key-12345'
      );
      workshopId = createResult.body.workshop.id;
      workshopCode = createResult.body.workshop.code;

      await portal.startWorkshop(workshopId);
      await waitForWorkshopStatus(workshopId, 'running', 60000);
      await waitForWorkspaceReady(1, 30000);

      // Join multiple learners
      const joinPromises = [
        portal.join(workshopCode, 'Learner1'),
        portal.join(workshopCode, 'Learner2'),
        portal.join(workshopCode, 'Learner3'),
      ];

      const results = await Promise.all(joinPromises);

      // All should succeed
      results.forEach((result, i) => {
        expect(result.status).toBe(200);
        expect(result.body.token).toBeDefined();
      });

      // Each should have a different seat
      const seats = results.map(r => r.body.seat);
      const uniqueSeats = new Set(seats);
      expect(uniqueSeats.size).toBe(3);

      // Verify learner count
      const learners = await portal.getLearners(workshopId);
      expect(learners.body.learners.length).toBe(3);
    }, 120000);
  });

  describe('Workshop Full Capacity', () => {
    it('should reject learners when workshop is full', async () => {
      // Create workshop with 1 seat
      const createResult = await portal.createWorkshop(
        `Full Workshop ${generateTestId()}`,
        1,
        'sk-ant-full-test-key-12345'
      );
      const workshopId = createResult.body.workshop.id;
      const workshopCode = createResult.body.workshop.code;

      try {
        await portal.startWorkshop(workshopId);
        await waitForWorkshopStatus(workshopId, 'running', 60000);
        await waitForWorkspaceReady(1, 30000);

        // First learner joins successfully
        const join1 = await portal.join(workshopCode, 'First');
        expect(join1.status).toBe(200);
        expect(join1.body.seat).toBe(1);

        // Second learner should be rejected
        const join2 = await portal.join(workshopCode, 'Second');
        expect(join2.status).toBe(400);
        expect((join2.body as { error: { code: string } }).error.code).toBe('NO_SEATS');
      } finally {
        await cleanupWorkshop(workshopId);
      }
    }, 120000);
  });

  describe('Invalid Join Attempts', () => {
    it('should reject join with invalid workshop code', async () => {
      const result = await portal.join('XXXX-YYYY');
      expect(result.status).toBe(404);
      expect((result.body as { error: { code: string } }).error.code).toBe('NOT_FOUND');
    });

    it('should reject join to non-running workshop', async () => {
      const createResult = await portal.createWorkshop(
        `Not Started ${generateTestId()}`,
        2,
        'sk-ant-nostart-key-12345'
      );
      const workshopId = createResult.body.workshop.id;

      try {
        const joinResult = await portal.join(createResult.body.workshop.code, 'Eager');
        expect(joinResult.status).toBe(400);
        expect((joinResult.body as { error: { code: string } }).error.code).toBe('NOT_RUNNING');
      } finally {
        await cleanupWorkshop(workshopId);
      }
    });

    it('should reject reconnect with invalid odehash', async () => {
      const createResult = await portal.createWorkshop(
        `Bad Odehash ${generateTestId()}`,
        2,
        'sk-ant-badode-key-12345'
      );
      const workshopId = createResult.body.workshop.id;

      try {
        await portal.startWorkshop(workshopId);
        await waitForWorkshopStatus(workshopId, 'running', 60000);

        const result = await portal.join(
          createResult.body.workshop.code,
          undefined,
          'xxxxx' // Invalid odehash
        );
        expect(result.status).toBe(404);
        expect((result.body as { error: { code: string } }).error.code).toBe('SESSION_NOT_FOUND');
      } finally {
        await cleanupWorkshop(workshopId);
      }
    }, 120000);
  });

  describe('Terminal WebSocket', () => {
    let workshopId: string;

    afterAll(async () => {
      if (workshopId) {
        await cleanupWorkshop(workshopId);
      }
    });

    it('should connect to terminal WebSocket', async () => {
      // Create and start workshop
      const createResult = await portal.createWorkshop(
        `Terminal WS ${generateTestId()}`,
        1,
        'sk-ant-terminal-test-key-12345'
      );
      workshopId = createResult.body.workshop.id;

      await portal.startWorkshop(workshopId);
      await waitForWorkshopStatus(workshopId, 'running', 60000);
      await waitForWorkspaceReady(1, 30000);

      // Join to get token
      const joinResult = await portal.join(createResult.body.workshop.code, 'Terminal Tester');
      expect(joinResult.status).toBe(200);

      const token = joinResult.body.token;
      const seat = joinResult.body.seat;

      // Connect to WebSocket
      const wsUrl = `${WORKSPACE_URL.replace('http', 'ws')}/vm/${seat}/terminal?token=${encodeURIComponent(token)}`;

      const connected = await new Promise<boolean>((resolve) => {
        const ws = new WebSocket(wsUrl);
        const timeout = setTimeout(() => {
          ws.close();
          resolve(false);
        }, 10000);

        ws.on('open', () => {
          clearTimeout(timeout);
          ws.close();
          resolve(true);
        });

        ws.on('error', () => {
          clearTimeout(timeout);
          resolve(false);
        });
      });

      expect(connected).toBe(true);
    }, 120000);
  });
});
