export type WorkshopStatus = 'created' | 'provisioning' | 'running' | 'stopping' | 'stopped' | 'error';

export interface Workshop {
  id: string;
  name: string;
  code: string;
  seats: number;
  status: WorkshopStatus;
  created_at: string;
  vm_name?: string;
  vm_ip?: string;
  endpoint?: string;
  connected_learners?: number;
}

export interface Session {
  odehash: string;
  seat: number;
  name?: string;
  joined_at: string;
}

export interface JoinResponse {
  token: string;
  endpoint: string;
  odehash: string;
  seat: number;
}

export interface ApiError {
  code: string;
  message: string;
}
