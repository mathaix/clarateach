import Fastify from 'fastify';
import websocket from '@fastify/websocket';
import cors from '@fastify/cors';
import { registerTerminalRoutes } from './terminal.js';
import { registerFileRoutes } from './files.js';

const TERMINAL_PORT = parseInt(process.env.TERMINAL_PORT || '3001', 10);
const FILES_PORT = parseInt(process.env.FILES_PORT || '3002', 10);
const HOST = process.env.HOST || '0.0.0.0';
const WORKSPACE_DIR = process.env.WORKSPACE_DIR || '/workspace';
// In MicroVM mode, routes don't have /vm/:seat prefix (single-tenant)
const MICROVM_MODE = process.env.MICROVM_MODE === 'true';

async function buildServer(enableWebsocket: boolean) {
  const fastify = Fastify({
    logger: {
      level: 'info',
    },
  });

  await fastify.register(cors, {
    origin: true,
    credentials: true,
  });

  if (enableWebsocket) {
    await fastify.register(websocket);
  }

  fastify.addContentTypeParser('application/json', { parseAs: 'string' }, (req, body, done) => {
    try {
      const json = JSON.parse(body as string);
      done(null, json);
    } catch (err: any) {
      done(err, undefined);
    }
  });

  fastify.get('/health', async () => {
    return { status: 'ok', workspace: WORKSPACE_DIR };
  });

  return fastify;
}

async function main() {
  const terminalServer = await buildServer(true);
  registerTerminalRoutes(terminalServer, WORKSPACE_DIR, MICROVM_MODE);

  const fileServer = await buildServer(false);
  registerFileRoutes(fileServer, WORKSPACE_DIR, MICROVM_MODE);

  try {
    await terminalServer.listen({ port: TERMINAL_PORT, host: HOST });
    console.log(`Workspace terminal server running at http://${HOST}:${TERMINAL_PORT}`);
    await fileServer.listen({ port: FILES_PORT, host: HOST });
    console.log(`Workspace file server running at http://${HOST}:${FILES_PORT}`);
    console.log(`Workspace directory: ${WORKSPACE_DIR}`);
  } catch (err) {
    terminalServer.log.error(err);
    process.exit(1);
  }
}

main();
