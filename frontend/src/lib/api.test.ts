import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from './api';

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock localStorage
const localStorageMock = {
  store: {} as Record<string, string>,
  getItem: vi.fn((key: string) => localStorageMock.store[key] || null),
  setItem: vi.fn((key: string, value: string) => {
    localStorageMock.store[key] = value;
  }),
  removeItem: vi.fn((key: string) => {
    delete localStorageMock.store[key];
  }),
  clear: vi.fn(() => {
    localStorageMock.store = {};
  }),
};

Object.defineProperty(window, 'localStorage', {
  value: localStorageMock,
});

describe('ApiClient', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorageMock.clear();
    localStorageMock.store = {};
  });

  describe('health', () => {
    it('should check server health', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify({ status: 'ok' }),
      });

      const result = await api.health();

      expect(result).toEqual({ status: 'ok' });
      expect(mockFetch).toHaveBeenCalledWith('/api/health', expect.any(Object));
    });
  });

  describe('auth', () => {
    it('should register a new user', async () => {
      const mockResponse = {
        token: 'new-token',
        user: {
          id: 'user-123',
          email: 'test@example.com',
          name: 'Test User',
          role: 'instructor',
          created_at: '2024-01-01T00:00:00Z',
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify(mockResponse),
      });

      const result = await api.authRegister('test@example.com', 'password123', 'Test User');

      expect(result).toEqual(mockResponse);
      expect(mockFetch).toHaveBeenCalledWith('/api/auth/register', {
        method: 'POST',
        body: JSON.stringify({ email: 'test@example.com', password: 'password123', name: 'Test User' }),
        headers: {
          'Content-Type': 'application/json',
        },
        auth: false,
      });
    });

    it('should login a user', async () => {
      const mockResponse = {
        token: 'login-token',
        user: {
          id: 'user-123',
          email: 'test@example.com',
          name: 'Test User',
          role: 'instructor',
          created_at: '2024-01-01T00:00:00Z',
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify(mockResponse),
      });

      const result = await api.authLogin('test@example.com', 'password123');

      expect(result).toEqual(mockResponse);
    });

    it('should handle login error', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => JSON.stringify({ error: { message: 'Invalid credentials' } }),
      });

      await expect(api.authLogin('test@example.com', 'wrongpassword')).rejects.toThrow('Invalid credentials');
    });

    it('should get current user', async () => {
      const mockUser = {
        id: 'user-123',
        email: 'test@example.com',
        name: 'Test User',
        role: 'instructor',
        created_at: '2024-01-01T00:00:00Z',
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify({ user: mockUser }),
      });

      const result = await api.authMe('test-token');

      expect(result.user).toEqual(mockUser);
      expect(mockFetch).toHaveBeenCalledWith('/api/auth/me', expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer test-token',
        }),
      }));
    });
  });

  describe('workshops', () => {
    it('should list workshops with auth header', async () => {
      localStorageMock.store['clarateach_token'] = 'saved-token';

      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify({ workshops: [] }),
      });

      const result = await api.listWorkshops();

      expect(result).toEqual({ workshops: [] });
      expect(mockFetch).toHaveBeenCalledWith('/api/workshops', expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer saved-token',
        }),
      }));
    });

    it('should get a workshop', async () => {
      const mockWorkshop = {
        id: 'workshop-123',
        name: 'Test Workshop',
        code: 'ABC123',
        seats: 10,
        status: 'created',
        created_at: '2024-01-01T00:00:00Z',
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify({ workshop: mockWorkshop }),
      });

      const result = await api.getWorkshop('workshop-123');

      expect(result.workshop).toEqual(mockWorkshop);
    });

    it('should create a workshop', async () => {
      const mockWorkshop = {
        id: 'new-workshop',
        name: 'New Workshop',
        code: 'XYZ789',
        seats: 5,
        status: 'created',
        created_at: '2024-01-01T00:00:00Z',
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify({ workshop: mockWorkshop }),
      });

      const result = await api.createWorkshop({
        name: 'New Workshop',
        seats: 5,
        api_key: 'sk-test-key',
      });

      expect(result.workshop).toEqual(mockWorkshop);
      expect(mockFetch).toHaveBeenCalledWith('/api/workshops', expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          name: 'New Workshop',
          seats: 5,
          api_key: 'sk-test-key',
        }),
      }));
    });

    it('should delete a workshop', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify({ success: true }),
      });

      const result = await api.deleteWorkshop('workshop-123');

      expect(result.success).toBe(true);
      expect(mockFetch).toHaveBeenCalledWith('/api/workshops/workshop-123', expect.objectContaining({
        method: 'DELETE',
      }));
    });

    it('should start a workshop', async () => {
      const mockWorkshop = {
        id: 'workshop-123',
        name: 'Test Workshop',
        code: 'ABC123',
        seats: 10,
        status: 'running',
        created_at: '2024-01-01T00:00:00Z',
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify({ workshop: mockWorkshop }),
      });

      const result = await api.startWorkshop('workshop-123');

      expect(result.workshop.status).toBe('running');
      expect(mockFetch).toHaveBeenCalledWith('/api/workshops/workshop-123/start', expect.objectContaining({
        method: 'POST',
      }));
    });
  });

  describe('registration', () => {
    it('should register for a workshop', async () => {
      const mockResponse = {
        access_code: 'FZL-7X9K',
        already_registered: false,
        message: 'Successfully registered',
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify(mockResponse),
      });

      const result = await api.register({
        workshop_code: 'ABC123',
        email: 'learner@example.com',
        name: 'Test Learner',
      });

      expect(result).toEqual(mockResponse);
    });

    it('should get session by access code', async () => {
      const mockResponse = {
        status: 'ready',
        endpoint: 'http://localhost:8080',
        seat: 1,
        name: 'Test Learner',
        workshop_id: 'workshop-123',
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => JSON.stringify(mockResponse),
      });

      const result = await api.getSession('FZL-7X9K');

      expect(result).toEqual(mockResponse);
    });
  });

  describe('error handling', () => {
    it('should handle empty response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => '',
      });

      await expect(api.health()).rejects.toThrow('Empty response from server');
    });

    it('should handle invalid JSON response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: async () => 'not json',
      });

      await expect(api.health()).rejects.toThrow('Invalid JSON response from server');
    });

    it('should handle network errors', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      await expect(api.health()).rejects.toThrow('Network error');
    });

    it('should handle HTTP error with message', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        text: async () => JSON.stringify({ error: { message: 'Bad request' } }),
      });

      await expect(api.health()).rejects.toThrow('Bad request');
    });

    it('should handle HTTP error without message', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: async () => 'Internal Server Error',
      });

      await expect(api.health()).rejects.toThrow('Internal Server Error');
    });
  });
});
