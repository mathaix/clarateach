import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels';
import { Terminal } from '@/components/Terminal';
import { Editor } from '@/components/Editor';
import { Browser } from '@/components/Browser';
import { User, LogOut } from 'lucide-react';

export function Workspace() {
  const navigate = useNavigate();
  const [isNarrow, setIsNarrow] = useState(false);

  // Get session info from localStorage
  const sessionData = localStorage.getItem('clarateach_session');
  const session = sessionData ? JSON.parse(sessionData) : null;

  const handleLeave = () => {
    if (confirm('Are you sure you want to leave the workspace?')) {
      localStorage.removeItem('clarateach_session');
      navigate('/');
    }
  };

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

  return (
    <div className="h-screen w-screen bg-[#1e1e1e] overflow-hidden flex flex-col">
      {/* Header */}
      <header className="bg-[#323233] border-b border-[#3c3c3c] px-4 py-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between flex-shrink-0">
        <div className="flex items-center gap-3">
          <h1 className="text-white font-semibold">ClaraTeach Workspace</h1>
          {session && (
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
