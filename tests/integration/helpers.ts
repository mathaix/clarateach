/**
 * Integration test helpers for ClaraTeach
 *
 * These helpers provide utilities for testing against running services.
 * Services must be running before tests are executed.
 */

const PORTAL_PORT = process.env.PORTAL_PORT ?? '3000';
const CADDY_PORT = process.env.CADDY_PORT ?? '8000';

export const PORTAL_URL = process.env.PORTAL_URL || `http://localhost:${PORTAL_PORT}`;
export const WORKSPACE_URL = process.env.WORKSPACE_URL || `http://localhost:${CADDY_PORT}`;

export interface Workshop {
  id: string;
  name: string;
  code: string;
  seats: number;
  status: string;
  created_at: string;
  vm_name?: string;
  vm_ip?: string;
  endpoint?: string;
  connected_learners?: number;
}

export interface JoinResponse {
  token: string;
  endpoint: string;
  odehash: string;
  seat: number;
}

export interface FileInfo {
  name: string;
  path: string;
  is_directory: boolean;
  size: number;
  modified_at: string;
}

/**
 * HTTP client wrapper for testing
 */
export async function apiRequest<T = unknown>(
  method: string,
  url: string,
  body?: unknown,
  headers?: Record<string, string>
): Promise<{ status: number; body: T }> {
  const response = await fetch(url, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...headers,
    },
    body: body ? JSON.stringify(body) : undefined,
  });

  let responseBody: T;
  const text = await response.text();
  try {
    responseBody = JSON.parse(text);
  } catch {
    responseBody = text as T;
  }

  return { status: response.status, body: responseBody };
}

/**
 * Portal API client
 */
export const portal = {
  async health() {
    return apiRequest<{ status: string }>('GET', `${PORTAL_URL}/api/health`);
  },

  async createWorkshop(name: string, seats: number, apiKey: string) {
    return apiRequest<{ workshop: Workshop }>(
      'POST',
      `${PORTAL_URL}/api/workshops`,
      { name, seats, api_key: apiKey }
    );
  },

  async getWorkshop(id: string) {
    return apiRequest<{ workshop: Workshop }>(
      'GET',
      `${PORTAL_URL}/api/workshops/${id}`
    );
  },

  async listWorkshops() {
    return apiRequest<{ workshops: Workshop[] }>(
      'GET',
      `${PORTAL_URL}/api/workshops`
    );
  },

  async deleteWorkshop(id: string) {
    return apiRequest<{ success: boolean }>(
      'DELETE',
      `${PORTAL_URL}/api/workshops/${id}`
    );
  },

  async startWorkshop(id: string) {
    return apiRequest<{ workshop: Workshop }>(
      'POST',
      `${PORTAL_URL}/api/workshops/${id}/start`
    );
  },

  async stopWorkshop(id: string) {
    return apiRequest<{ success: boolean }>(
      'POST',
      `${PORTAL_URL}/api/workshops/${id}/stop`
    );
  },

  async getLearners(id: string) {
    return apiRequest<{ learners: Array<{ seat: number; name: string; joined_at: string }> }>(
      'GET',
      `${PORTAL_URL}/api/workshops/${id}/learners`
    );
  },

  async join(code: string, name?: string, odehash?: string) {
    return apiRequest<JoinResponse>(
      'POST',
      `${PORTAL_URL}/api/join`,
      { code, name, odehash }
    );
  },
};

/**
 * Workspace API client (via Caddy proxy)
 */
export const workspace = {
  async listFiles(seat: number, path?: string, token?: string) {
    const url = path
      ? `${WORKSPACE_URL}/vm/${seat}/files?path=${encodeURIComponent(path)}`
      : `${WORKSPACE_URL}/vm/${seat}/files`;
    return apiRequest<{ files: FileInfo[] }>(
      'GET',
      url,
      undefined,
      token ? { Authorization: `Bearer ${token}` } : undefined
    );
  },

  async readFile(seat: number, filePath: string, token?: string) {
    return apiRequest<{ content: string; encoding: string }>(
      'GET',
      `${WORKSPACE_URL}/vm/${seat}/files/${filePath}`,
      undefined,
      token ? { Authorization: `Bearer ${token}` } : undefined
    );
  },

  async writeFile(seat: number, filePath: string, content: string, token?: string, encoding?: string) {
    return apiRequest<{ success: boolean }>(
      'PUT',
      `${WORKSPACE_URL}/vm/${seat}/files/${filePath}`,
      { content, encoding },
      token ? { Authorization: `Bearer ${token}` } : undefined
    );
  },

  async deleteFile(seat: number, filePath: string, token?: string) {
    return apiRequest<{ success: boolean }>(
      'DELETE',
      `${WORKSPACE_URL}/vm/${seat}/files/${filePath}`,
      undefined,
      token ? { Authorization: `Bearer ${token}` } : undefined
    );
  },

  async health(seat: number) {
    return apiRequest<{ status: string; workspace: string }>(
      'GET',
      `${WORKSPACE_URL}/vm/${seat}/terminal/health`
    );
  },
};

/**
 * Wait for a workshop to reach a specific status
 */
export async function waitForWorkshopStatus(
  workshopId: string,
  expectedStatus: string,
  timeoutMs = 30000,
  pollIntervalMs = 500
): Promise<Workshop> {
  const startTime = Date.now();

  while (Date.now() - startTime < timeoutMs) {
    const result = await portal.getWorkshop(workshopId);
    if (result.status === 200 && result.body.workshop.status === expectedStatus) {
      return result.body.workshop;
    }
    await sleep(pollIntervalMs);
  }

  throw new Error(`Timeout waiting for workshop ${workshopId} to reach status ${expectedStatus}`);
}

/**
 * Wait for workspace to be ready (files API responds)
 * Uses the files API instead of health check since Caddy doesn't strip prefixes
 * Considers 401 as "ready" since it means the container is up and running (just needs auth)
 */
export async function waitForWorkspaceReady(
  seat: number,
  timeoutMs = 30000,
  pollIntervalMs = 500
): Promise<void> {
  const startTime = Date.now();

  while (Date.now() - startTime < timeoutMs) {
    try {
      // Use files endpoint since it's available and works
      const result = await workspace.listFiles(seat);
      // 200 = success (auth disabled), 401 = container ready but needs auth
      if (result.status === 200 || result.status === 401) {
        return;
      }
    } catch {
      // Ignore errors and keep polling
    }
    await sleep(pollIntervalMs);
  }

  throw new Error(`Timeout waiting for workspace seat ${seat} to be ready`);
}

/**
 * Sleep utility
 */
export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Generate a unique test identifier
 */
export function generateTestId(): string {
  return `test${Date.now().toString(36)}`;
}

/**
 * Clean up a workshop after tests
 */
export async function cleanupWorkshop(workshopId: string): Promise<void> {
  try {
    await portal.deleteWorkshop(workshopId);
  } catch {
    // Ignore cleanup errors
  }
}
