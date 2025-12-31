import { useEffect, useState, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels';
import { Terminal } from '@/components/Terminal';
import { Editor } from '@/components/Editor';
import { Browser } from '@/components/Browser';
import { User, LogOut, Loader2, AlertCircle, RefreshCw } from 'lucide-react';
import { api, SessionResponse } from '@/lib/api';
import { setWorkspaceSession } from '@/lib/workspaceSession';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';

type Status = 'loading' | 'pending' | 'ready' | 'error' | 'ended';

export function SessionWorkspace() {
  const navigate = useNavigate();
  const { code } = useParams<{ code: string }>();

  const [status, setStatus] = useState<Status>('loading');
  const [error, setError] = useState<string>('');
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [isNarrow, setIsNarrow] = useState(false);

  const fetchSession = useCallback(async () => {
    if (!code) {
      setError('No access code provided');
      setStatus('error');
      return;
    }

    try {
      const response = await api.getSession(code);

      if (response.status === 'pending') {
        setStatus('pending');
        setSession(response);
      } else if (response.status === 'ready') {
        // Store in workspace session for components to use
        setWorkspaceSession({
          endpoint: response.endpoint!,
          seat: response.seat!,
          token: '', // No token needed with registration flow
          name: response.name,
        });
        setSession(response);
        setStatus('ready');
      }
    } catch (err) {
      console.error('Failed to fetch session:', err);
      const message = err instanceof Error ? err.message : 'Failed to load session';

      if (message.includes('ended') || message.includes('410')) {
        setStatus('ended');
        setError('This workshop has ended.');
      } else if (message.includes('Invalid')) {
        setStatus('error');
        setError('Invalid access code. Please check and try again.');
      } else {
        setStatus('error');
        setError(message);
      }
    }
  }, [code]);

  // Initial load
  useEffect(() => {
    fetchSession();
  }, [fetchSession]);

  // Poll while pending
  useEffect(() => {
    if (status !== 'pending') return;

    const interval = setInterval(() => {
      fetchSession();
    }, 5000); // Check every 5 seconds

    return () => clearInterval(interval);
  }, [status, fetchSession]);

  // Responsive layout
  useEffect(() => {
    if (typeof window === 'undefined') return;
    const media = window.matchMedia('(max-width: 1024px)');
    const update = () => setIsNarrow(media.matches);
    update();
    if (media.addEventListener) {
      media.addEventListener('change', update);
      return () => media.removeEventListener('change', update);
    }
    media.addListener(update);
    return () => media.removeListener(update);
  }, []);

  const handleLeave = () => {
    if (confirm('Are you sure you want to leave the workspace?')) {
      navigate('/');
    }
  };

  // Loading state
  if (status === 'loading') {
    return (
      <div className="min-h-screen bg-[#1e1e1e] flex items-center justify-center">
        <div className="text-center">
          <Loader2 className="w-12 h-12 text-indigo-500 animate-spin mx-auto mb-4" />
          <p className="text-gray-400">Loading workspace...</p>
        </div>
      </div>
    );
  }

  // Pending state (workshop starting)
  if (status === 'pending') {
    return (
      <div className="min-h-screen bg-gradient-to-br from-indigo-50 to-blue-100 flex items-center justify-center p-4">
        <Card className="w-full max-w-md">
          <CardHeader className="text-center">
            <div className="w-16 h-16 bg-indigo-100 rounded-full flex items-center justify-center mx-auto mb-4">
              <RefreshCw className="w-8 h-8 text-indigo-600 animate-spin" />
            </div>
            <CardTitle>Workshop Starting</CardTitle>
            <CardDescription>
              {session?.message || 'Please wait while the workshop is being prepared...'}
            </CardDescription>
          </CardHeader>
          <CardContent className="text-center">
            <p className="text-sm text-gray-500 mb-4">
              This page will refresh automatically when ready.
            </p>
            <Button variant="outline" onClick={() => navigate('/')}>
              Back to Home
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Error state
  if (status === 'error' || status === 'ended') {
    return (
      <div className="min-h-screen bg-gradient-to-br from-red-50 to-orange-100 flex items-center justify-center p-4">
        <Card className="w-full max-w-md">
          <CardHeader className="text-center">
            <div className="w-16 h-16 bg-red-100 rounded-full flex items-center justify-center mx-auto mb-4">
              <AlertCircle className="w-8 h-8 text-red-600" />
            </div>
            <CardTitle>{status === 'ended' ? 'Workshop Ended' : 'Error'}</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Button className="w-full" onClick={() => navigate('/join')}>
              Join Another Workshop
            </Button>
            <Button variant="outline" className="w-full" onClick={() => navigate('/')}>
              Back to Home
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Ready state - show workspace
  return (
    <div className="h-screen w-screen bg-[#1e1e1e] overflow-hidden flex flex-col">
      {/* Header */}
      <header className="bg-[#323233] border-b border-[#3c3c3c] px-4 py-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between flex-shrink-0">
        <div className="flex items-center gap-3">
          <h1 className="text-white font-semibold">ClaraTeach Workspace</h1>
          {session?.seat && (
            <span className="text-gray-400 text-sm">Seat {session.seat}</span>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-4">
          <div className="flex items-center gap-2 text-gray-300">
            <User className="w-4 h-4" />
            <span className="text-sm">{session?.name || 'Learner'}</span>
          </div>
          <button
            onClick={handleLeave}
            className="flex items-center gap-2 text-gray-300 hover:text-white transition-colors"
          >
            <LogOut className="w-4 h-4" />
            <span className="text-sm">Leave</span>
          </button>
        </div>
      </header>

      {/* Main Content */}
      <div className="flex-1 overflow-hidden">
        <PanelGroup direction={isNarrow ? 'vertical' : 'horizontal'}>
          {/* Left Section - Editor and Terminal */}
          <Panel defaultSize={isNarrow ? 55 : 60} minSize={isNarrow ? 35 : 30}>
            <PanelGroup direction="vertical">
              {/* Editor */}
              <Panel defaultSize={60} minSize={20}>
                <Editor />
              </Panel>

              <PanelResizeHandle className="h-1 bg-[#3c3c3c] hover:bg-[#007acc] transition-colors" />

              {/* Terminal */}
              <Panel defaultSize={40} minSize={15}>
                <Terminal />
              </Panel>
            </PanelGroup>
          </Panel>

          <PanelResizeHandle
            className={`${isNarrow ? 'h-1' : 'w-1'} bg-[#3c3c3c] hover:bg-[#007acc] transition-colors`}
          />

          {/* Right Section - Browser */}
          <Panel defaultSize={isNarrow ? 45 : 40} minSize={25}>
            <Browser />
          </Panel>
        </PanelGroup>
      </div>
    </div>
  );
}
