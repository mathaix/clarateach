import type { FastifyInstance, FastifyRequest, FastifyReply } from 'fastify';
import { promises as fs } from 'fs';
import path from 'path';

interface FileInfo {
  name: string;
  path: string;
  isDirectory: boolean;
  size: number;
  modifiedAt: string;
}

interface FileContent {
  content: string;
  encoding: 'utf-8' | 'base64';
}

// Ensure path is within workspace directory (prevent path traversal)
function resolveSafePath(workspaceDir: string, requestedPath: string): string | null {
  const resolved = path.resolve(workspaceDir, requestedPath);
  if (!resolved.startsWith(workspaceDir)) {
    return null;
  }
  return resolved;
}

// Check if file is binary
function isBinaryFile(filename: string): boolean {
  const binaryExtensions = [
    '.png', '.jpg', '.jpeg', '.gif', '.webp', '.ico', '.bmp',
    '.pdf', '.zip', '.tar', '.gz', '.7z', '.rar',
    '.exe', '.dll', '.so', '.dylib',
    '.mp3', '.mp4', '.wav', '.avi', '.mov',
    '.woff', '.woff2', '.ttf', '.otf', '.eot',
  ];
  const ext = path.extname(filename).toLowerCase();
  return binaryExtensions.includes(ext);
}

export function registerFileRoutes(fastify: FastifyInstance, workspaceDir: string) {
  // List directory contents
  fastify.get('/files', async (request: FastifyRequest, reply: FastifyReply) => {
    const { path: dirPath = '' } = request.query as { path?: string };

    const safePath = resolveSafePath(workspaceDir, dirPath);
    if (!safePath) {
      return reply.status(403).send({ error: { code: 'FORBIDDEN', message: 'Path outside workspace' } });
    }

    try {
      const entries = await fs.readdir(safePath, { withFileTypes: true });
      const files: FileInfo[] = await Promise.all(
        entries.map(async (entry) => {
          const fullPath = path.join(safePath, entry.name);
          const relativePath = path.relative(workspaceDir, fullPath);
          const stats = await fs.stat(fullPath);

          return {
            name: entry.name,
            path: relativePath,
            isDirectory: entry.isDirectory(),
            size: stats.size,
            modifiedAt: stats.mtime.toISOString(),
          };
        })
      );

      // Sort: directories first, then alphabetically
      files.sort((a, b) => {
        if (a.isDirectory !== b.isDirectory) {
          return a.isDirectory ? -1 : 1;
        }
        return a.name.localeCompare(b.name);
      });

      return { files };
    } catch (err: any) {
      if (err.code === 'ENOENT') {
        return reply.status(404).send({ error: { code: 'NOT_FOUND', message: 'Directory not found' } });
      }
      throw err;
    }
  });

  // Read file contents
  fastify.get('/files/*', async (request: FastifyRequest, reply: FastifyReply) => {
    const filePath = (request.params as { '*': string })['*'];

    if (!filePath) {
      return reply.status(400).send({ error: { code: 'INVALID_INPUT', message: 'File path required' } });
    }

    const safePath = resolveSafePath(workspaceDir, filePath);
    if (!safePath) {
      return reply.status(403).send({ error: { code: 'FORBIDDEN', message: 'Path outside workspace' } });
    }

    try {
      const stats = await fs.stat(safePath);

      if (stats.isDirectory()) {
        return reply.status(400).send({ error: { code: 'INVALID_INPUT', message: 'Cannot read directory as file' } });
      }

      const binary = isBinaryFile(filePath);
      const content = await fs.readFile(safePath, binary ? 'base64' : 'utf-8');

      const response: FileContent = {
        content: content as string,
        encoding: binary ? 'base64' : 'utf-8',
      };

      return response;
    } catch (err: any) {
      if (err.code === 'ENOENT') {
        return reply.status(404).send({ error: { code: 'NOT_FOUND', message: 'File not found' } });
      }
      throw err;
    }
  });

  // Write file contents
  fastify.put('/files/*', async (request: FastifyRequest, reply: FastifyReply) => {
    const filePath = (request.params as { '*': string })['*'];

    if (!filePath) {
      return reply.status(400).send({ error: { code: 'INVALID_INPUT', message: 'File path required' } });
    }

    const safePath = resolveSafePath(workspaceDir, filePath);
    if (!safePath) {
      return reply.status(403).send({ error: { code: 'FORBIDDEN', message: 'Path outside workspace' } });
    }

    const body = request.body as { content?: string; encoding?: 'utf-8' | 'base64' };

    if (typeof body.content !== 'string') {
      return reply.status(400).send({ error: { code: 'INVALID_INPUT', message: 'Content required' } });
    }

    try {
      // Create parent directories if needed
      const dir = path.dirname(safePath);
      await fs.mkdir(dir, { recursive: true });

      // Write file
      const encoding = body.encoding || 'utf-8';
      if (encoding === 'base64') {
        await fs.writeFile(safePath, Buffer.from(body.content, 'base64'));
      } else {
        await fs.writeFile(safePath, body.content, 'utf-8');
      }

      return { success: true };
    } catch (err: any) {
      fastify.log.error('Error writing file:', err);
      throw err;
    }
  });

  // Delete file or directory
  fastify.delete('/files/*', async (request: FastifyRequest, reply: FastifyReply) => {
    const filePath = (request.params as { '*': string })['*'];

    if (!filePath) {
      return reply.status(400).send({ error: { code: 'INVALID_INPUT', message: 'File path required' } });
    }

    const safePath = resolveSafePath(workspaceDir, filePath);
    if (!safePath) {
      return reply.status(403).send({ error: { code: 'FORBIDDEN', message: 'Path outside workspace' } });
    }

    // Prevent deleting the workspace root
    if (safePath === workspaceDir) {
      return reply.status(403).send({ error: { code: 'FORBIDDEN', message: 'Cannot delete workspace root' } });
    }

    try {
      const stats = await fs.stat(safePath);

      if (stats.isDirectory()) {
        await fs.rm(safePath, { recursive: true });
      } else {
        await fs.unlink(safePath);
      }

      return { success: true };
    } catch (err: any) {
      if (err.code === 'ENOENT') {
        return reply.status(404).send({ error: { code: 'NOT_FOUND', message: 'File not found' } });
      }
      throw err;
    }
  });
}
