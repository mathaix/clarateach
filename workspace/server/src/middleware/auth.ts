import { FastifyRequest, FastifyReply } from 'fastify';
import { createRemoteJWKSet, jwtVerify, JWTPayload, errors } from 'jose';

/**
 * Token claims expected in ClaraTeach JWTs
 */
export interface TokenClaims extends JWTPayload {
  seat: number;
  workshop_id: string;
  container_id: string;
  odehash?: string;
  vm_ip?: string;
  name?: string;
}

/**
 * Extended request with verified token claims
 */
export interface AuthenticatedRequest extends FastifyRequest {
  token: TokenClaims;
}

// Cache the JWKS client
let jwksClient: ReturnType<typeof createRemoteJWKSet> | null = null;

/**
 * Get or create the JWKS client for token verification
 */
function getJWKSClient(): ReturnType<typeof createRemoteJWKSet> {
  if (!jwksClient) {
    const jwksUrl = process.env.JWKS_URL;
    if (!jwksUrl) {
      throw new Error('JWKS_URL environment variable is required');
    }
    jwksClient = createRemoteJWKSet(new URL(jwksUrl));
  }
  return jwksClient;
}

/**
 * Validate RS256 JWT and extract claims
 */
async function validateToken(token: string): Promise<TokenClaims> {
  const jwks = getJWKSClient();

  const { payload } = await jwtVerify(token, jwks, {
    algorithms: ['RS256'],
    issuer: process.env.JWT_ISSUER ?? 'clarateach-portal',
    audience: process.env.JWT_AUDIENCE ?? 'clarateach-workspace',
  });

  // Validate required claims
  if (typeof payload.seat !== 'number' || !Number.isInteger(payload.seat) || payload.seat < 1 || payload.seat > 10) {
    throw new Error('Missing or invalid seat claim');
  }
  if (typeof payload.workshop_id !== 'string') {
    throw new Error('Missing or invalid workshop_id claim');
  }
  if (typeof payload.container_id !== 'string') {
    throw new Error('Missing or invalid container_id claim');
  }

  return payload as TokenClaims;
}

/**
 * Extract bearer token from Authorization header
 */
function extractBearerToken(authHeader: string | undefined): string | null {
  if (!authHeader) return null;

  const parts = authHeader.split(' ');
  if (parts.length !== 2 || parts[0].toLowerCase() !== 'bearer') {
    return null;
  }

  return parts[1];
}

/**
 * Auth middleware for Fastify routes
 *
 * Validates JWT from Authorization header and attaches claims to request.
 * Returns 401 for missing/invalid tokens.
 */
export async function authMiddleware(
  request: FastifyRequest,
  reply: FastifyReply
): Promise<void> {
  // Skip auth if disabled (for local development)
  if (process.env.AUTH_DISABLED === 'true') {
    const seat = Number(process.env.SEAT ?? '1');
    (request as AuthenticatedRequest).token = {
      seat: Number.isFinite(seat) ? seat : 1,
      workshop_id: 'local',
      container_id: process.env.CONTAINER_ID ?? 'local-dev',
      name: 'Developer',
    };
    return;
  }

  const token = extractBearerToken(request.headers.authorization);

  if (!token) {
    reply.code(401).send({
      error: { code: 'UNAUTHORIZED', message: 'Missing authorization token' },
    });
    return;
  }

  try {
    const claims = await validateToken(token);
    (request as AuthenticatedRequest).token = claims;
  } catch (err) {
    request.log.warn({ err }, 'Token validation failed');

    let message = 'Invalid token';
    if (err instanceof errors.JWTExpired) {
      message = 'Token has expired';
    } else if (err instanceof errors.JWTClaimValidationFailed) {
      message = 'Token claims validation failed';
    } else if (err instanceof errors.JWSSignatureVerificationFailed) {
      message = 'Token signature verification failed';
    }

    reply.code(401).send({
      error: { code: 'UNAUTHORIZED', message },
    });
  }
}

/**
 * Auth hook for WebSocket connections
 *
 * For WebSocket routes, token can be passed as query parameter since
 * browsers don't support custom headers on WebSocket connections.
 */
export async function wsAuthMiddleware(
  request: FastifyRequest,
  reply: FastifyReply
): Promise<void> {
  // Skip auth if disabled (for local development)
  if (process.env.AUTH_DISABLED === 'true') {
    const seat = Number(process.env.SEAT ?? '1');
    (request as AuthenticatedRequest).token = {
      seat: Number.isFinite(seat) ? seat : 1,
      workshop_id: 'local',
      container_id: process.env.CONTAINER_ID ?? 'local-dev',
      name: 'Developer',
    };
    return;
  }

  // Try Authorization header first, then query parameter
  let token = extractBearerToken(request.headers.authorization);

  if (!token) {
    const query = request.query as { token?: string };
    token = query.token || null;
  }

  if (!token) {
    reply.code(401).send({
      error: { code: 'UNAUTHORIZED', message: 'Missing authorization token' },
    });
    return;
  }

  try {
    const claims = await validateToken(token);
    (request as AuthenticatedRequest).token = claims;
  } catch (err) {
    request.log.warn({ err }, 'WebSocket token validation failed');

    reply.code(401).send({
      error: { code: 'UNAUTHORIZED', message: 'Invalid token' },
    });
  }
}
