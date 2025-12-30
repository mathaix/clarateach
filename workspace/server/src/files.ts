import type { FastifyInstance, FastifyRequest, FastifyReply } from 'fastify';
import { promises as fs } from 'fs';
import path from 'path';
import { authMiddleware } from './middleware/auth.js';
import { enforceSeatAccess } from './middleware/seat.js';

interface FileInfo {
  name: string;
  path: string;
  is_directory: boolean;
  size: number;
  modified_at: string;
}

interface FileContent {
  content: string;
  encoding: 'utf-8' | 'base64';
}

function isSubPath(parent: string, target: string): boolean {
  const relative = path.relative(parent, target);
  return relative === '' || (!relative.startsWith('..') && !path.isAbsolute(relative));
}

function normalizeRequestedPath(workspaceDir: string, requestedPath: string): string | null {
  const trimmed = requestedPath.trim();
  if (trimmed === '' || trimmed === '/') {
    return '';
  }

  if (path.isAbsolute(trimmed)) {
    const relative = path.relative(workspaceDir, trimmed);
    if (relative.startsWith('..') || path.isAbsolute(relative)) {
      return null;
    }
    return relative;
  }

  if (trimmed === 'workspace') {
    return '';
  }

  if (trimmed.startsWith('workspace/')) {
    return trimmed.slice('workspace/'.length);
  }

  return trimmed;
}

// Ensure path is within workspace directory (prevent path traversal + symlink escape)
async function resolveSafePath(workspaceDir: string, requestedPath: string): Promise<string | null> {
  const normalized = normalizeRequestedPath(workspaceDir, requestedPath);
  if (normalized === null) {
    return null;
  }

  const resolved = path.resolve(workspaceDir, normalized);
  const realWorkspace = await fs.realpath(workspaceDir);

  // If resolved path IS the workspace directory, allow it
  if (resolved === workspaceDir) {
    return resolved;
  }

  const resolvedParent = path.dirname(resolved);

  let realParent = resolvedParent;
  try {
    let probePath = resolvedParent;
    while (true) {
      try {
        realParent = await fs.realpath(probePath);
        break;
      } catch (err: any) {
        if (err?.code !== 'ENOENT') {
          throw err;
        }
        const parent = path.dirname(probePath);
        if (parent === probePath) {
          return null;
        }
        probePath = parent;
      }
    }
  } catch {
    return null;
  }

  if (!isSubPath(realWorkspace, realParent)) {
    return null;
  }

  try {
    const realTarget = await fs.realpath(resolved);
    if (!isSubPath(realWorkspace, realTarget)) {
      return null;
    }
  } catch (err: any) {
    if (err?.code !== 'ENOENT') {
      throw err;
    }
  }

  return resolved;
}

// Check if file is binary
function isBinaryFile(filename: string): boolean {
  const binaryExtensions = [
    '.png', '.jpg', '.jpeg', '.gif', '.webp', '.ico', '.bmp',
    '.pdf', '.zip', '.tar', '.gz', '.7z', '.rar',
    '.bin',
    '.exe', '.dll', '.so', '.dylib',
    '.mp3', '.mp4', '.wav', '.avi', '.mov',
    '.woff', '.woff2', '.ttf', '.otf', '.eot',
  ];
  const ext = path.extname(filename).toLowerCase();
  return binaryExtensions.includes(ext);
}

export function registerFileRoutes(fastify: FastifyInstance, workspaceDir: string) {
  // Apply auth middleware to all file routes
  const seatGuard = async (request: FastifyRequest, reply: FastifyReply) => {
    enforceSeatAccess(request, reply);
  };
  const authHook = { preHandler: [authMiddleware, seatGuard] };

  // List directory contents
  fastify.get('/vm/:seat/files', authHook, async (request: FastifyRequest, reply: FastifyReply) => {
    const { path: dirPath = workspaceDir } = request.query as { path?: string };

    const safePath = await resolveSafePath(workspaceDir, dirPath);
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
            path: path.join(workspaceDir, relativePath),
            is_directory: entry.isDirectory(),
            size: stats.size,
            modified_at: stats.mtime.toISOString(),
          };
        })
      );

      // Sort: directories first, then alphabetically
      files.sort((a, b) => {
        if (a.is_directory !== b.is_directory) {
          return a.is_directory ? -1 : 1;
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
  fastify.get('/vm/:seat/files/*', authHook, async (request: FastifyRequest, reply: FastifyReply) => {
    const filePath = (request.params as { '*': string })['*'];

    if (!filePath) {
      return reply.status(400).send({ error: { code: 'INVALID_INPUT', message: 'File path required' } });
    }

    const safePath = await resolveSafePath(workspaceDir, filePath);
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
  fastify.put('/vm/:seat/files/*', authHook, async (request: FastifyRequest, reply: FastifyReply) => {
    const filePath = (request.params as { '*': string })['*'];

    if (!filePath) {
      return reply.status(400).send({ error: { code: 'INVALID_INPUT', message: 'File path required' } });
    }

    const safePath = await resolveSafePath(workspaceDir, filePath);
    if (!safePath) {
      return reply.status(403).send({ error: { code: 'FORBIDDEN', message: 'Path outside workspace' } });
    }

    const body = request.body as { content?: string; encoding?: 'utf-8' | 'base64' };

    if (typeof body.content !== 'string') {
      return reply.status(400).send({ error: { code: 'INVALID_INPUT', message: 'Content required' } });
    }
    if (body.encoding && body.encoding !== 'utf-8' && body.encoding !== 'base64') {
      return reply.status(400).send({ error: { code: 'INVALID_INPUT', message: 'Unsupported encoding' } });
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
  fastify.delete('/vm/:seat/files/*', authHook, async (request: FastifyRequest, reply: FastifyReply) => {
    const filePath = (request.params as { '*': string })['*'];

    if (!filePath) {
      return reply.status(400).send({ error: { code: 'INVALID_INPUT', message: 'File path required' } });
    }

    const safePath = await resolveSafePath(workspaceDir, filePath);
    if (!safePath) {
      return reply.status(403).send({ error: { code: 'FORBIDDEN', message: 'Path outside workspace' } });
    }

    // Prevent deleting the workspace root
    if (path.resolve(safePath) === path.resolve(workspaceDir)) {
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
