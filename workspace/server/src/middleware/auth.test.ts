import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { authMiddleware, wsAuthMiddleware, TokenClaims } from './auth.js';
import type { FastifyRequest, FastifyReply } from 'fastify';

// Mock jose module
vi.mock('jose', () => ({
  createRemoteJWKSet: vi.fn(() => vi.fn()),
  jwtVerify: vi.fn(),
  errors: {
    JWTExpired: class JWTExpired extends Error {
      constructor() {
        super('JWT expired');
        this.name = 'JWTExpired';
      }
    },
    JWTClaimValidationFailed: class JWTClaimValidationFailed extends Error {
      constructor() {
        super('JWT claim validation failed');
        this.name = 'JWTClaimValidationFailed';
      }
    },
    JWSSignatureVerificationFailed: class JWSSignatureVerificationFailed extends Error {
      constructor() {
        super('JWS signature verification failed');
        this.name = 'JWSSignatureVerificationFailed';
      }
    },
  },
}));

import { jwtVerify, errors } from 'jose';

const mockJwtVerify = vi.mocked(jwtVerify);

function createMockRequest(overrides: Partial<FastifyRequest> = {}): FastifyRequest {
  return {
    headers: {},
    query: {},
    log: { warn: vi.fn() },
    ...overrides,
  } as unknown as FastifyRequest;
}

function createMockReply(): FastifyReply & { sentData: unknown; sentCode: number } {
  const reply = {
    sentCode: 200,
    sentData: null as unknown,
    code(statusCode: number) {
      this.sentCode = statusCode;
      return this;
    },
    send(data: unknown) {
      this.sentData = data;
      return this;
    },
  };
  return reply as FastifyReply & { sentData: unknown; sentCode: number };
}

describe('authMiddleware', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    vi.resetModules();
    process.env = { ...originalEnv };
    vi.clearAllMocks();
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('when AUTH_DISABLED=true', () => {
    it('should set default token claims and not validate', async () => {
      process.env.AUTH_DISABLED = 'true';

      const request = createMockRequest();
      const reply = createMockReply();

      await authMiddleware(request, reply);

      const authRequest = request as FastifyRequest & { token: TokenClaims };
      expect(authRequest.token).toEqual({
        seat: 1,
        workshop_id: 'local',
        container_id: 'local-dev',
        name: 'Developer',
      });
      expect(reply.sentData).toBeNull();
    });
  });

  describe('when AUTH_DISABLED is not set', () => {
    beforeEach(() => {
      process.env.AUTH_DISABLED = 'false';
      process.env.JWKS_URL = 'https://example.com/.well-known/jwks.json';
    });

    it('should return 401 when Authorization header is missing', async () => {
      const request = createMockRequest();
      const reply = createMockReply();

      await authMiddleware(request, reply);

      expect(reply.sentCode).toBe(401);
      expect(reply.sentData).toEqual({
        error: { code: 'UNAUTHORIZED', message: 'Missing authorization token' },
      });
    });

    it('should return 401 when Authorization header format is invalid', async () => {
      const request = createMockRequest({
        headers: { authorization: 'InvalidFormat token123' },
      });
      const reply = createMockReply();

      await authMiddleware(request, reply);

      expect(reply.sentCode).toBe(401);
      expect(reply.sentData).toEqual({
        error: { code: 'UNAUTHORIZED', message: 'Missing authorization token' },
      });
    });

    it('should return 401 when Bearer token is missing', async () => {
      const request = createMockRequest({
        headers: { authorization: 'Bearer' },
      });
      const reply = createMockReply();

      await authMiddleware(request, reply);

      expect(reply.sentCode).toBe(401);
    });

    it('should validate token and attach claims on success', async () => {
      const validClaims: TokenClaims = {
        seat: 5,
        workshop_id: 'ws-123',
        container_id: 'container-abc',
        name: 'Test User',
      };

      mockJwtVerify.mockResolvedValue({
        payload: validClaims,
        protectedHeader: { alg: 'RS256' },
      } as Awaited<ReturnType<typeof jwtVerify>>);

      const request = createMockRequest({
        headers: { authorization: 'Bearer valid.jwt.token' },
      });
      const reply = createMockReply();

      await authMiddleware(request, reply);

      const authRequest = request as FastifyRequest & { token: TokenClaims };
      expect(authRequest.token).toEqual(validClaims);
      expect(reply.sentData).toBeNull();
    });

    it('should return 401 with expired message for JWTExpired error', async () => {
      mockJwtVerify.mockRejectedValue(new errors.JWTExpired());

      const request = createMockRequest({
        headers: { authorization: 'Bearer expired.jwt.token' },
      });
      const reply = createMockReply();

      await authMiddleware(request, reply);

      expect(reply.sentCode).toBe(401);
      expect(reply.sentData).toEqual({
        error: { code: 'UNAUTHORIZED', message: 'Token has expired' },
      });
    });

    it('should return 401 with claim validation message for JWTClaimValidationFailed', async () => {
      mockJwtVerify.mockRejectedValue(new errors.JWTClaimValidationFailed());

      const request = createMockRequest({
        headers: { authorization: 'Bearer invalid.claims.token' },
      });
      const reply = createMockReply();

      await authMiddleware(request, reply);

      expect(reply.sentCode).toBe(401);
      expect(reply.sentData).toEqual({
        error: { code: 'UNAUTHORIZED', message: 'Token claims validation failed' },
      });
    });

    it('should return 401 with signature message for JWSSignatureVerificationFailed', async () => {
      mockJwtVerify.mockRejectedValue(new errors.JWSSignatureVerificationFailed());

      const request = createMockRequest({
        headers: { authorization: 'Bearer tampered.jwt.token' },
      });
      const reply = createMockReply();

      await authMiddleware(request, reply);

      expect(reply.sentCode).toBe(401);
      expect(reply.sentData).toEqual({
        error: { code: 'UNAUTHORIZED', message: 'Token signature verification failed' },
      });
    });

    it('should return 401 with generic message for other errors', async () => {
      mockJwtVerify.mockRejectedValue(new Error('Some other error'));

      const request = createMockRequest({
        headers: { authorization: 'Bearer bad.jwt.token' },
      });
      const reply = createMockReply();

      await authMiddleware(request, reply);

      expect(reply.sentCode).toBe(401);
      expect(reply.sentData).toEqual({
        error: { code: 'UNAUTHORIZED', message: 'Invalid token' },
      });
    });
  });
});

describe('wsAuthMiddleware', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    vi.resetModules();
    process.env = { ...originalEnv };
    vi.clearAllMocks();
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('when AUTH_DISABLED=true', () => {
    it('should set default token claims', async () => {
      process.env.AUTH_DISABLED = 'true';

      const request = createMockRequest();
      const reply = createMockReply();

      await wsAuthMiddleware(request, reply);

      const authRequest = request as FastifyRequest & { token: TokenClaims };
      expect(authRequest.token).toEqual({
        seat: 1,
        workshop_id: 'local',
        container_id: 'local-dev',
        name: 'Developer',
      });
    });
  });

  describe('when AUTH_DISABLED is not set', () => {
    beforeEach(() => {
      process.env.AUTH_DISABLED = 'false';
      process.env.JWKS_URL = 'https://example.com/.well-known/jwks.json';
    });

    it('should accept token from Authorization header', async () => {
      const validClaims: TokenClaims = {
        seat: 3,
        workshop_id: 'ws-456',
        container_id: 'container-def',
        name: 'WS User',
      };

      mockJwtVerify.mockResolvedValue({
        payload: validClaims,
        protectedHeader: { alg: 'RS256' },
      } as Awaited<ReturnType<typeof jwtVerify>>);

      const request = createMockRequest({
        headers: { authorization: 'Bearer header.jwt.token' },
      });
      const reply = createMockReply();

      await wsAuthMiddleware(request, reply);

      const authRequest = request as FastifyRequest & { token: TokenClaims };
      expect(authRequest.token).toEqual(validClaims);
    });

    it('should accept token from query parameter when header is missing', async () => {
      const validClaims: TokenClaims = {
        seat: 7,
        workshop_id: 'ws-789',
        container_id: 'container-ghi',
        name: 'Query User',
      };

      mockJwtVerify.mockResolvedValue({
        payload: validClaims,
        protectedHeader: { alg: 'RS256' },
      } as Awaited<ReturnType<typeof jwtVerify>>);

      const request = createMockRequest({
        query: { token: 'query.jwt.token' },
      });
      const reply = createMockReply();

      await wsAuthMiddleware(request, reply);

      const authRequest = request as FastifyRequest & { token: TokenClaims };
      expect(authRequest.token).toEqual(validClaims);
    });

    it('should prefer Authorization header over query parameter', async () => {
      const headerClaims: TokenClaims = {
        seat: 1,
        workshop_id: 'header-ws',
        container_id: 'header-container',
      };

      mockJwtVerify.mockResolvedValue({
        payload: headerClaims,
        protectedHeader: { alg: 'RS256' },
      } as Awaited<ReturnType<typeof jwtVerify>>);

      const request = createMockRequest({
        headers: { authorization: 'Bearer header.token' },
        query: { token: 'query.token' },
      });
      const reply = createMockReply();

      await wsAuthMiddleware(request, reply);

      // The header token should be used (verified by mocking that returns headerClaims)
      expect(mockJwtVerify).toHaveBeenCalledWith(
        'header.token',
        expect.anything(),
        expect.anything()
      );
    });

    it('should return 401 when no token provided anywhere', async () => {
      const request = createMockRequest();
      const reply = createMockReply();

      await wsAuthMiddleware(request, reply);

      expect(reply.sentCode).toBe(401);
      expect(reply.sentData).toEqual({
        error: { code: 'UNAUTHORIZED', message: 'Missing authorization token' },
      });
    });

    it('should return 401 for invalid token', async () => {
      mockJwtVerify.mockRejectedValue(new Error('Invalid'));

      const request = createMockRequest({
        query: { token: 'invalid.token' },
      });
      const reply = createMockReply();

      await wsAuthMiddleware(request, reply);

      expect(reply.sentCode).toBe(401);
      expect(reply.sentData).toEqual({
        error: { code: 'UNAUTHORIZED', message: 'Invalid token' },
      });
    });
  });
});
