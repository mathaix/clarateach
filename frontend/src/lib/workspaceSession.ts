interface WorkspaceSession {
  token: string;
  endpoint: string;
  odehash: string;
  seat: number;
  code: string;
  name?: string;
}

export function getWorkspaceSession(): WorkspaceSession | null {
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
