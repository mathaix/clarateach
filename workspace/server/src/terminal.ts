import type { FastifyInstance } from 'fastify';
import * as pty from 'node-pty';
import type { WebSocket } from 'ws';

interface TerminalMessage {
  type: 'input' | 'resize';
  data?: string;
  cols?: number;
  rows?: number;
}

export function registerTerminalRoutes(fastify: FastifyInstance, workspaceDir: string) {
  fastify.get('/terminal', { websocket: true }, (socket: WebSocket) => {
    fastify.log.info('Terminal WebSocket connected');

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

    // Send PTY output to WebSocket
    ptyProcess.onData((data: string) => {
      try {
        if (socket.readyState === socket.OPEN) {
          socket.send(JSON.stringify({ type: 'output', data }));
        }
      } catch (err) {
        fastify.log.error({ err }, 'Error sending PTY data');
      }
    });

    // Handle PTY exit
    ptyProcess.onExit(({ exitCode, signal }) => {
      fastify.log.info(`PTY exited with code ${exitCode}, signal ${signal}`);
      if (socket.readyState === socket.OPEN) {
        socket.send(JSON.stringify({ type: 'exit', code: exitCode, signal }));
        socket.close();
      }
    });

    // Handle WebSocket messages
    socket.on('message', (message: Buffer | string) => {
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
    socket.on('close', () => {
      fastify.log.info('Terminal WebSocket closed');
      ptyProcess.kill();
    });

    // Handle WebSocket error
    socket.on('error', (err) => {
      fastify.log.error({ err }, 'Terminal WebSocket error');
      ptyProcess.kill();
    });
  });
}
