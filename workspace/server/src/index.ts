import Fastify from 'fastify';
import websocket from '@fastify/websocket';
import cors from '@fastify/cors';
import { registerTerminalRoutes } from './terminal.js';
import { registerFileRoutes } from './files.js';

const PORT = parseInt(process.env.PORT || '3000', 10);
const HOST = process.env.HOST || '0.0.0.0';
const WORKSPACE_DIR = process.env.WORKSPACE_DIR || '/workspace';

async function main() {
  const fastify = Fastify({
    logger: {
      level: 'info',
    },
  });

  // Register plugins
  await fastify.register(cors, {
    origin: true,
    credentials: true,
  });

  await fastify.register(websocket);

  // Add content type parser for JSON
  fastify.addContentTypeParser('application/json', { parseAs: 'string' }, (req, body, done) => {
    try {
      const json = JSON.parse(body as string);
      done(null, json);
    } catch (err: any) {
      done(err, undefined);
    }
  });

  // Health check endpoint
  fastify.get('/health', async () => {
    return { status: 'ok', workspace: WORKSPACE_DIR };
  });

  // Register routes
  registerTerminalRoutes(fastify, WORKSPACE_DIR);
  registerFileRoutes(fastify, WORKSPACE_DIR);

  // Start server
  try {
    await fastify.listen({ port: PORT, host: HOST });
    console.log(`Workspace server running at http://${HOST}:${PORT}`);
    console.log(`Workspace directory: ${WORKSPACE_DIR}`);
  } catch (err) {
    fastify.log.error(err);
    process.exit(1);
  }
}

main();
