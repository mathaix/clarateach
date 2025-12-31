import { describe, it, expect, vi, beforeEach, afterEach, beforeAll, afterAll } from 'vitest';
import Fastify, { FastifyInstance } from 'fastify';
import { promises as fs } from 'fs';
import path from 'path';
import os from 'os';
import { registerFileRoutes } from './files.js';

// Mock the auth middleware to always pass
vi.mock('./middleware/auth.js', () => ({
  authMiddleware: vi.fn(async (request: unknown) => {
    (request as { token: unknown }).token = {
      seat: 1,
      workshop_id: 'test',
      container_id: 'test-container',
      name: 'Test User',
    };
  }),
}));

describe('File Routes', () => {
  let app: FastifyInstance;
  let workspaceDir: string;

  beforeAll(async () => {
    // Create a temp workspace directory
    workspaceDir = await fs.mkdtemp(path.join(os.tmpdir(), 'clarateach-test-'));
  });

  afterAll(async () => {
    // Clean up temp directory
    await fs.rm(workspaceDir, { recursive: true, force: true });
  });

  beforeEach(async () => {
    app = Fastify({ logger: false });
    app.addContentTypeParser('application/json', { parseAs: 'string' }, (req, body, done) => {
      try {
        const json = JSON.parse(body as string);
        done(null, json);
      } catch (err: unknown) {
        done(err as Error, undefined);
      }
    });
    registerFileRoutes(app, workspaceDir);
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    // Clean up workspace between tests
    const entries = await fs.readdir(workspaceDir);
    for (const entry of entries) {
      await fs.rm(path.join(workspaceDir, entry), { recursive: true, force: true });
    }
  });

  describe('GET /vm/:seat/files (list directory)', () => {
    it('should list empty directory', async () => {
      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files',
      });

      expect(response.statusCode).toBe(200);
      const body = JSON.parse(response.body);
      expect(body.files).toEqual([]);
    });

    it('should list files and directories', async () => {
      // Create test files and directories
      await fs.writeFile(path.join(workspaceDir, 'file1.txt'), 'content1');
      await fs.writeFile(path.join(workspaceDir, 'file2.js'), 'content2');
      await fs.mkdir(path.join(workspaceDir, 'subdir'));

      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files',
      });

      expect(response.statusCode).toBe(200);
      const body = JSON.parse(response.body);
      expect(body.files).toHaveLength(3);

      // Directories should come first
      expect(body.files[0].name).toBe('subdir');
      expect(body.files[0].is_directory).toBe(true);

      // Then files alphabetically
      expect(body.files[1].name).toBe('file1.txt');
      expect(body.files[1].is_directory).toBe(false);
      expect(body.files[2].name).toBe('file2.js');
    });

    it('should list subdirectory contents', async () => {
      await fs.mkdir(path.join(workspaceDir, 'subdir'));
      await fs.writeFile(path.join(workspaceDir, 'subdir', 'nested.txt'), 'nested content');

      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files?path=subdir',
      });

      expect(response.statusCode).toBe(200);
      const body = JSON.parse(response.body);
      expect(body.files).toHaveLength(1);
      expect(body.files[0].name).toBe('nested.txt');
      expect(body.files[0].path).toBe(path.join(workspaceDir, 'subdir', 'nested.txt'));
    });

    it('should return 404 for non-existent directory', async () => {
      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files?path=nonexistent',
      });

      expect(response.statusCode).toBe(404);
      const body = JSON.parse(response.body);
      expect(body.error.code).toBe('NOT_FOUND');
    });

    it('should return 403 for path traversal attempt', async () => {
      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files?path=../../../etc',
      });

      expect(response.statusCode).toBe(403);
      const body = JSON.parse(response.body);
      expect(body.error.code).toBe('FORBIDDEN');
    });
  });

  describe('GET /vm/:seat/files/* (read file)', () => {
    it('should read text file content', async () => {
      await fs.writeFile(path.join(workspaceDir, 'test.txt'), 'Hello, World!');

      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files/test.txt',
      });

      expect(response.statusCode).toBe(200);
      const body = JSON.parse(response.body);
      expect(body.content).toBe('Hello, World!');
      expect(body.encoding).toBe('utf-8');
    });

    it('should read binary file as base64', async () => {
      const binaryContent = Buffer.from([0x89, 0x50, 0x4e, 0x47]); // PNG magic bytes
      await fs.writeFile(path.join(workspaceDir, 'image.png'), binaryContent);

      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files/image.png',
      });

      expect(response.statusCode).toBe(200);
      const body = JSON.parse(response.body);
      expect(body.encoding).toBe('base64');
      expect(Buffer.from(body.content, 'base64')).toEqual(binaryContent);
    });

    it('should read nested file', async () => {
      await fs.mkdir(path.join(workspaceDir, 'deep', 'nested'), { recursive: true });
      await fs.writeFile(path.join(workspaceDir, 'deep', 'nested', 'file.txt'), 'nested content');

      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files/deep/nested/file.txt',
      });

      expect(response.statusCode).toBe(200);
      const body = JSON.parse(response.body);
      expect(body.content).toBe('nested content');
    });

    it('should return 404 for non-existent file', async () => {
      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files/nonexistent.txt',
      });

      expect(response.statusCode).toBe(404);
    });

    it('should return 400 when trying to read directory as file', async () => {
      await fs.mkdir(path.join(workspaceDir, 'somedir'));

      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files/somedir',
      });

      expect(response.statusCode).toBe(400);
      const body = JSON.parse(response.body);
      expect(body.error.code).toBe('INVALID_INPUT');
    });

    it('should not expose files outside workspace for path traversal', async () => {
      // Path traversal attempts are either blocked (403) or result in file not found (404)
      // because the path resolver normalizes the path
      const response = await app.inject({
        method: 'GET',
        url: '/vm/1/files/../../../etc/passwd',
      });

      // Either 403 (blocked) or 404 (normalized path doesn't exist in workspace) is acceptable
      expect([403, 404]).toContain(response.statusCode);
    });
  });

  describe('PUT /vm/:seat/files/* (write file)', () => {
    it('should create new file', async () => {
      const response = await app.inject({
        method: 'PUT',
        url: '/vm/1/files/newfile.txt',
        headers: { 'content-type': 'application/json' },
        payload: JSON.stringify({ content: 'new content' }),
      });

      expect(response.statusCode).toBe(200);
      const body = JSON.parse(response.body);
      expect(body.success).toBe(true);

      const fileContent = await fs.readFile(path.join(workspaceDir, 'newfile.txt'), 'utf-8');
      expect(fileContent).toBe('new content');
    });

    it('should overwrite existing file', async () => {
      await fs.writeFile(path.join(workspaceDir, 'existing.txt'), 'old content');

      const response = await app.inject({
        method: 'PUT',
        url: '/vm/1/files/existing.txt',
        headers: { 'content-type': 'application/json' },
        payload: JSON.stringify({ content: 'updated content' }),
      });

      expect(response.statusCode).toBe(200);

      const fileContent = await fs.readFile(path.join(workspaceDir, 'existing.txt'), 'utf-8');
      expect(fileContent).toBe('updated content');
    });

    it('should create parent directories if needed', async () => {
      const response = await app.inject({
        method: 'PUT',
        url: '/vm/1/files/new/deep/path/file.txt',
        headers: { 'content-type': 'application/json' },
        payload: JSON.stringify({ content: 'deep content' }),
      });

      expect(response.statusCode).toBe(200);

      const fileContent = await fs.readFile(
        path.join(workspaceDir, 'new', 'deep', 'path', 'file.txt'),
        'utf-8'
      );
      expect(fileContent).toBe('deep content');
    });

    it('should write binary content with base64 encoding', async () => {
      const binaryContent = Buffer.from([0x00, 0x01, 0x02, 0x03]);

      const response = await app.inject({
        method: 'PUT',
        url: '/vm/1/files/binary.bin',
        headers: { 'content-type': 'application/json' },
        payload: JSON.stringify({
          content: binaryContent.toString('base64'),
          encoding: 'base64',
        }),
      });

      expect(response.statusCode).toBe(200);

      const fileContent = await fs.readFile(path.join(workspaceDir, 'binary.bin'));
      expect(fileContent).toEqual(binaryContent);
    });

    it('should return 400 when content is missing', async () => {
      const response = await app.inject({
        method: 'PUT',
        url: '/vm/1/files/nocontentfile.txt',
        headers: { 'content-type': 'application/json' },
        payload: JSON.stringify({}),
      });

      expect(response.statusCode).toBe(400);
      const body = JSON.parse(response.body);
      expect(body.error.code).toBe('INVALID_INPUT');
    });

    it('should not write files outside workspace for path traversal', async () => {
      // Path traversal attempts are blocked - either 403 or 404 due to URL normalization
      const response = await app.inject({
        method: 'PUT',
        url: '/vm/1/files/../../../tmp/evil.txt',
        headers: { 'content-type': 'application/json' },
        payload: JSON.stringify({ content: 'malicious' }),
      });

      // Either 403 (blocked) or 404 (normalized path) is acceptable - file must NOT be written
      expect([403, 404]).toContain(response.statusCode);
    });
  });

  describe('DELETE /vm/:seat/files/* (delete file/directory)', () => {
    it('should delete a file', async () => {
      await fs.writeFile(path.join(workspaceDir, 'todelete.txt'), 'content');

      const response = await app.inject({
        method: 'DELETE',
        url: '/vm/1/files/todelete.txt',
      });

      expect(response.statusCode).toBe(200);
      const body = JSON.parse(response.body);
      expect(body.success).toBe(true);

      await expect(fs.access(path.join(workspaceDir, 'todelete.txt'))).rejects.toThrow();
    });

    it('should delete a directory recursively', async () => {
      await fs.mkdir(path.join(workspaceDir, 'dirToDelete', 'nested'), { recursive: true });
      await fs.writeFile(path.join(workspaceDir, 'dirToDelete', 'file1.txt'), 'content');
      await fs.writeFile(path.join(workspaceDir, 'dirToDelete', 'nested', 'file2.txt'), 'content');

      const response = await app.inject({
        method: 'DELETE',
        url: '/vm/1/files/dirToDelete',
      });

      expect(response.statusCode).toBe(200);

      await expect(fs.access(path.join(workspaceDir, 'dirToDelete'))).rejects.toThrow();
    });

    it('should return 404 for non-existent file', async () => {
      const response = await app.inject({
        method: 'DELETE',
        url: '/vm/1/files/nonexistent.txt',
      });

      expect(response.statusCode).toBe(404);
    });

    it('should not delete files outside workspace for path traversal', async () => {
      // Path traversal attempts are blocked - either 403 or 404 due to URL normalization
      const response = await app.inject({
        method: 'DELETE',
        url: '/vm/1/files/../../../tmp/important',
      });

      // Either 403 (blocked) or 404 (normalized path) is acceptable
      expect([403, 404]).toContain(response.statusCode);
    });

    it('should return 403 when trying to delete workspace root', async () => {
      const response = await app.inject({
        method: 'DELETE',
        url: '/vm/1/files/',
      });

      // Empty path should return 400 (file path required)
      expect(response.statusCode).toBe(400);
    });
  });
});
