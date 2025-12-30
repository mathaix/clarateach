/**
 * Workspace API Integration Tests
 *
 * Tests the workspace file API through Caddy proxy:
 * - File listing
 * - File reading
 * - File writing
 * - File deletion
 *
 * Prerequisites:
 * - Services must be running (`./scripts/stack.sh start`)
 * - A workshop must be created and started with at least 1 seat
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import {
  portal,
  workspace,
  generateTestId,
  cleanupWorkshop,
  waitForWorkshopStatus,
  waitForWorkspaceReady,
} from './helpers.js';

describe('Workspace API', () => {
  let workshopId: string;
  let workshopCode: string;
  let token: string;
  const seat = 1;

  beforeAll(async () => {
    // Create and start a workshop for testing
    const createResult = await portal.createWorkshop(
      `Workspace API Test ${generateTestId()}`,
      2,
      'sk-ant-workspace-test-key-12345'
    );

    if (createResult.status !== 201) {
      throw new Error(`Failed to create workshop: ${JSON.stringify(createResult.body)}`);
    }

    workshopId = createResult.body.workshop.id;
    workshopCode = createResult.body.workshop.code;

    // Start the workshop
    const startResult = await portal.startWorkshop(workshopId);
    if (startResult.status !== 202) {
      await cleanupWorkshop(workshopId);
      throw new Error(`Failed to start workshop: ${JSON.stringify(startResult.body)}`);
    }

    // Wait for running
    await waitForWorkshopStatus(workshopId, 'running', 60000);

    // Wait for workspace to be ready
    await waitForWorkspaceReady(seat, 30000);

    // Join to get a token
    const joinResult = await portal.join(workshopCode, 'Test User');
    if (joinResult.status !== 200) {
      await cleanupWorkshop(workshopId);
      throw new Error(`Failed to join workshop: ${JSON.stringify(joinResult.body)}`);
    }
    token = joinResult.body.token;
  }, 120000);

  afterAll(async () => {
    if (workshopId) {
      await cleanupWorkshop(workshopId);
    }
  });

  describe('GET /vm/:seat/files (List Directory)', () => {
    it('should list workspace root directory', async () => {
      const result = await workspace.listFiles(seat, undefined, token);

      expect(result.status).toBe(200);
      expect(result.body.files).toBeDefined();
      expect(Array.isArray(result.body.files)).toBe(true);
    });

    it('should return 403 for path traversal attempt', async () => {
      const result = await workspace.listFiles(seat, '../../../etc', token);

      expect(result.status).toBe(403);
    });
  });

  describe('PUT /vm/:seat/files/* (Write File)', () => {
    it('should create a new file', async () => {
      const fileName = `test-${generateTestId()}.txt`;
      const content = 'Hello, Integration Test!';

      const writeResult = await workspace.writeFile(seat, fileName, content, token);
      expect(writeResult.status).toBe(200);
      expect(writeResult.body.success).toBe(true);

      // Verify by reading
      const readResult = await workspace.readFile(seat, fileName, token);
      expect(readResult.status).toBe(200);
      expect(readResult.body.content).toBe(content);
      expect(readResult.body.encoding).toBe('utf-8');

      // Clean up
      await workspace.deleteFile(seat, fileName, token);
    });

    it('should create file in nested directory', async () => {
      const dirPath = `test-dir-${generateTestId()}`;
      const fileName = `${dirPath}/nested/file.txt`;
      const content = 'Nested content';

      const writeResult = await workspace.writeFile(seat, fileName, content, token);
      expect(writeResult.status).toBe(200);

      // Verify
      const readResult = await workspace.readFile(seat, fileName, token);
      expect(readResult.status).toBe(200);
      expect(readResult.body.content).toBe(content);

      // Clean up
      await workspace.deleteFile(seat, dirPath, token);
    });

    it('should overwrite existing file', async () => {
      const fileName = `overwrite-${generateTestId()}.txt`;

      // Create
      await workspace.writeFile(seat, fileName, 'original', token);

      // Overwrite
      const writeResult = await workspace.writeFile(seat, fileName, 'updated', token);
      expect(writeResult.status).toBe(200);

      // Verify
      const readResult = await workspace.readFile(seat, fileName, token);
      expect(readResult.body.content).toBe('updated');

      // Clean up
      await workspace.deleteFile(seat, fileName, token);
    });

    it('should write binary content with base64 encoding', async () => {
      const fileName = `binary-${generateTestId()}.bin`;
      const binaryContent = Buffer.from([0x00, 0x01, 0x02, 0xFF]);
      const base64Content = binaryContent.toString('base64');

      const writeResult = await workspace.writeFile(seat, fileName, base64Content, token, 'base64');
      expect(writeResult.status).toBe(200);

      // Verify
      const readResult = await workspace.readFile(seat, fileName, token);
      expect(readResult.status).toBe(200);
      expect(readResult.body.encoding).toBe('base64');
      expect(Buffer.from(readResult.body.content, 'base64').equals(binaryContent)).toBe(true);

      // Clean up
      await workspace.deleteFile(seat, fileName, token);
    });
  });

  describe('GET /vm/:seat/files/* (Read File)', () => {
    it('should read text file content', async () => {
      const fileName = `read-test-${generateTestId()}.txt`;
      const content = 'Content to read';

      await workspace.writeFile(seat, fileName, content, token);

      const result = await workspace.readFile(seat, fileName, token);
      expect(result.status).toBe(200);
      expect(result.body.content).toBe(content);
      expect(result.body.encoding).toBe('utf-8');

      await workspace.deleteFile(seat, fileName, token);
    });

    it('should return 404 for non-existent file', async () => {
      const result = await workspace.readFile(seat, 'nonexistent-file.txt', token);

      expect(result.status).toBe(404);
    });
  });

  describe('DELETE /vm/:seat/files/* (Delete File)', () => {
    it('should delete a file', async () => {
      const fileName = `delete-test-${generateTestId()}.txt`;

      // Create
      await workspace.writeFile(seat, fileName, 'to delete', token);

      // Delete
      const deleteResult = await workspace.deleteFile(seat, fileName, token);
      expect(deleteResult.status).toBe(200);
      expect(deleteResult.body.success).toBe(true);

      // Verify gone
      const readResult = await workspace.readFile(seat, fileName, token);
      expect(readResult.status).toBe(404);
    });

    it('should delete directory recursively', async () => {
      const dirName = `delete-dir-${generateTestId()}`;

      // Create directory with files
      await workspace.writeFile(seat, `${dirName}/file1.txt`, 'content1', token);
      await workspace.writeFile(seat, `${dirName}/sub/file2.txt`, 'content2', token);

      // Delete directory
      const deleteResult = await workspace.deleteFile(seat, dirName, token);
      expect(deleteResult.status).toBe(200);

      // Verify all gone
      const listResult = await workspace.listFiles(seat, dirName, token);
      expect(listResult.status).toBe(404);
    });

    it('should return 404 for non-existent file', async () => {
      const result = await workspace.deleteFile(seat, 'nonexistent.txt', token);

      expect(result.status).toBe(404);
    });
  });

  describe('Workspace Availability', () => {
    it('should respond to file listing requests', async () => {
      // Since Caddy doesn't strip /vm/:seat/terminal prefix, the /health endpoint
      // isn't accessible. We verify workspace availability via the files API instead.
      const result = await workspace.listFiles(seat, undefined, token);

      expect(result.status).toBe(200);
      expect(result.body.files).toBeDefined();
    });
  });
});
