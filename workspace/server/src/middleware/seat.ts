import type { FastifyReply, FastifyRequest } from 'fastify';
import type { AuthenticatedRequest } from './auth.js';

const MIN_SEAT = 1;
const MAX_SEAT = 10;

function parseSeatParam(seatParam: string | undefined): number | null {
  if (!seatParam || !/^\d+$/.test(seatParam)) {
    return null;
  }
  const seat = Number(seatParam);
  if (!Number.isSafeInteger(seat) || seat < MIN_SEAT || seat > MAX_SEAT) {
    return null;
  }
  return seat;
}

export function enforceSeatAccess(request: FastifyRequest, reply: FastifyReply): number | null {
  if (reply.sent) {
    return null;
  }

  const seatParam = (request.params as { seat?: string }).seat;
  const seat = parseSeatParam(seatParam);
  if (!seat) {
    reply.status(400).send({
      error: { code: 'INVALID_INPUT', message: 'Invalid seat' },
    });
    return null;
  }

  const token = (request as AuthenticatedRequest).token;
  if (!token || token.seat !== seat) {
    reply.status(403).send({
      error: { code: 'FORBIDDEN', message: 'Seat access denied' },
    });
    return null;
  }

  const expectedSeat = process.env.SEAT ? Number(process.env.SEAT) : undefined;
  if (expectedSeat && expectedSeat !== seat) {
    reply.status(403).send({
      error: { code: 'FORBIDDEN', message: 'Seat mismatch for container' },
    });
    return null;
  }

  const expectedContainerId = process.env.CONTAINER_ID;
  if (expectedContainerId && token.container_id !== expectedContainerId) {
    reply.status(403).send({
      error: { code: 'FORBIDDEN', message: 'Container access denied' },
    });
    return null;
  }

  return seat;
}
