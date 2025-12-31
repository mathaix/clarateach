import type { Workshop, Session, JoinResponse, ApiError } from './types';

const API_BASE = '/api';

class ApiClient {
  private async request<T>(path: string, options?: RequestInit): Promise<T> {
    const response = await fetch(`${API_BASE}${path}`, {
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
      ...options,
    });

    const text = await response.text();
    let data: any = null;
    let parseError = false;
    if (text) {
      try {
        data = JSON.parse(text);
      } catch {
        parseError = true;
      }
    }

    if (!response.ok) {
      const error = (data?.error as ApiError | undefined);
      throw new Error(error?.message || text || `Request failed (${response.status})`);
    }

    if (!text) {
      throw new Error('Empty response from server');
    }
    if (parseError) {
      throw new Error('Invalid JSON response from server');
    }

    return data as T;
  }

  // Health
  async health(): Promise<{ status: string }> {
    return this.request('/health');
  }

  // Workshops
  async listWorkshops(): Promise<{ workshops: Workshop[] }> {
    return this.request('/workshops');
  }

  async getWorkshop(id: string): Promise<{ workshop: Workshop }> {
    return this.request(`/workshops/${id}`);
  }

  async createWorkshop(data: { name: string; seats: number; api_key: string }): Promise<{ workshop: Workshop }> {
    return this.request('/workshops', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async deleteWorkshop(id: string): Promise<{ success: boolean }> {
    return this.request(`/workshops/${id}`, {
      method: 'DELETE',
    });
  }

  async startWorkshop(id: string): Promise<{ workshop: Workshop }> {
    return this.request(`/workshops/${id}/start`, {
      method: 'POST',
    });
  }

  async stopWorkshop(id: string): Promise<{ success: boolean }> {
    return this.request(`/workshops/${id}/stop`, {
      method: 'POST',
    });
  }

  async getWorkshopLearners(id: string): Promise<{ learners: Session[]; connected: number }> {
    return this.request(`/workshops/${id}/learners`);
  }

  // Join
  async joinWorkshop(data: { code: string; name?: string; odehash?: string }): Promise<JoinResponse> {
    return this.request('/join', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  // Admin
  async adminOverview(): Promise<{ workshops: AdminWorkshopView[]; total: number }> {
    return this.request('/admin/overview');
  }

  async listVMs(): Promise<{ vms: VMWithWorkshop[]; total: number }> {
    return this.request('/admin/vms');
  }

  async getVMDetails(workshopId: string): Promise<VMDetails> {
    return this.request(`/admin/vms/${workshopId}`);
  }

  getSSHKeyDownloadUrl(workshopId: string): string {
    return `${API_BASE}/admin/vms/${workshopId}/ssh-key`;
  }
}

// Admin types
export interface WorkshopVM {
  id: string;
  workshop_id: string;
  vm_name: string;
  vm_id: string;
  zone: string;
  machine_type: string;
  external_ip: string;
  internal_ip: string;
  status: string;
  ssh_public_key: string;
  ssh_user: string;
  created_at: string;
  updated_at: string;
}

export interface AdminWorkshopView {
  workshop: Workshop;
  vm?: WorkshopVM;
  sessions: Session[];
  active_students: number;
  total_seats: number;
  ssh_command?: string;
}

export interface VMWithWorkshop extends WorkshopVM {
  workshop_name: string;
  active_students: number;
  total_seats: number;
  ssh_command: string;
  gcloud_ssh: string;
}

export interface VMDetails {
  vm: WorkshopVM;
  workshop: Workshop;
  sessions: Session[];
  stats: {
    active_students: number;
    total_seats: number;
  };
  access: {
    ssh_command: string;
    gcloud_ssh: string;
  };
}

export const api = new ApiClient();
