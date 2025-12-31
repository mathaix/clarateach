export interface WorkspaceSession {
  token?: string;
  endpoint: string;
  odehash?: string;
  seat: number;
  code?: string;
  name?: string;
}

// In-memory session storage for new registration flow
let currentSession: WorkspaceSession | null = null;

export function setWorkspaceSession(session: WorkspaceSession): void {
  currentSession = session;
  // Also store in localStorage for legacy support
  localStorage.setItem('clarateach_session', JSON.stringify(session));
}

export function getWorkspaceSession(): WorkspaceSession | null {
  // Prefer in-memory session (set by SessionWorkspace)
  if (currentSession) {
    return currentSession;
  }
  // Fall back to localStorage (legacy flow)
  const data = localStorage.getItem('clarateach_session');
  if (!data) return null;
  try {
    return JSON.parse(data) as WorkspaceSession;
  } catch {
    return null;
  }
}

export function getAuthHeaders(): Record<string, string> {
  const session = getWorkspaceSession();
  if (session?.token) {
    return { Authorization: `Bearer ${session.token}` };
  }
  return {};
}

export function getWorkspaceEndpoint(): string {
  const session = getWorkspaceSession();
  // In production, this would be the VM endpoint
  // For local dev, proxy through the current origin
  return session?.endpoint || '';
}
