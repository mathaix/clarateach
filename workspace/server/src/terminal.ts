import type { FastifyInstance, FastifyRequest, FastifyReply } from 'fastify';
import type { WebSocket } from 'ws';
import * as pty from 'node-pty';
import { wsAuthMiddleware, AuthenticatedRequest } from './middleware/auth.js';
import { enforceSeatAccess } from './middleware/seat.js';

interface TerminalMessage {
  type: 'input' | 'resize';
  data?: string;
  cols?: number;
  rows?: number;
}

export function registerTerminalRoutes(fastify: FastifyInstance, workspaceDir: string) {
  const seatGuard = async (request: FastifyRequest, reply: FastifyReply) => {
    enforceSeatAccess(request, reply);
  };

  fastify.get('/vm/:seat/terminal', { websocket: true, preHandler: [wsAuthMiddleware, seatGuard] }, (socket: WebSocket, request) => {
    const authRequest = request as AuthenticatedRequest;
    const { seat, workshop_id, name } = authRequest.token;
    const ws = (socket as unknown as { socket?: WebSocket }).socket ?? socket;
    fastify.log.info({ seat, workshop_id, name }, 'Terminal WebSocket connected');

    // Spawn a PTY shell with interactive login
    const shell = process.env.SHELL || '/bin/bash';
    const ptyProcess = pty.spawn(shell, ['--login'], {
      name: 'xterm-256color',
      cols: 80,
      rows: 24,
      cwd: workspaceDir,
      env: {
        ...process.env,
        TERM: 'xterm-256color',
        // Ensure bash knows it's interactive
        PS1: '\\u@\\h:\\w\\$ ',
      },
    });

    fastify.log.info(`PTY spawned with PID ${ptyProcess.pid}`);

    const pingInterval = setInterval(() => {
      if (ws.readyState === ws.OPEN) {
        ws.ping();
      }
    }, 30000);

    // Send PTY output to WebSocket
    ptyProcess.onData((data: string) => {
      try {
        if (ws.readyState === ws.OPEN) {
          ws.send(JSON.stringify({ type: 'output', data }));
        }
      } catch (err) {
        fastify.log.error({ err }, 'Error sending PTY data');
      }
    });

    // Handle PTY exit
    ptyProcess.onExit(({ exitCode, signal }) => {
      fastify.log.info(`PTY exited with code ${exitCode}, signal ${signal}`);
      if (ws.readyState === ws.OPEN) {
        ws.send(JSON.stringify({ type: 'exit', code: exitCode, signal }));
        ws.close();
      }
    });

    // Handle WebSocket messages
    ws.on('message', (message: Buffer | string) => {
      try {
        const msg: TerminalMessage = JSON.parse(message.toString());

        switch (msg.type) {
          case 'input':
            if (msg.data) {
              ptyProcess.write(msg.data);
            }
            break;

          case 'resize':
            if (msg.cols && msg.rows) {
              ptyProcess.resize(msg.cols, msg.rows);
              fastify.log.info(`PTY resized to ${msg.cols}x${msg.rows}`);
            }
            break;

          default:
            fastify.log.warn(`Unknown message type: ${(msg as any).type}`);
        }
      } catch (err) {
        fastify.log.error({ err }, 'Error parsing WebSocket message');
      }
    });

    // Handle WebSocket close
    ws.on('close', () => {
      fastify.log.info('Terminal WebSocket closed');
      clearInterval(pingInterval);
      ptyProcess.kill();
    });

    // Handle WebSocket error
    ws.on('error', (err: Error) => {
      fastify.log.error({ err }, 'Terminal WebSocket error');
      clearInterval(pingInterval);
      ptyProcess.kill();
    });
  });
}
