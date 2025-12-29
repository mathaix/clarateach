import { ArrowLeft, User } from 'lucide-react';
import { Terminal } from './Terminal';
import { CodeEditor } from './CodeEditor';
import { Browser } from './Browser';
import { ResizablePanelGroup, ResizablePanel, ResizableHandle } from './ui/resizable';
import { Button } from './ui/button';

interface WorkspaceProps {
  learnerName: string;
  classCode: string;
  onLeave: () => void;
}

export function Workspace({ learnerName, classCode, onLeave }: WorkspaceProps) {
  return (
    <div className="h-screen w-screen bg-[#1e1e1e] overflow-hidden">
      {/* Header */}
      <div className="bg-[#323233] border-b border-[#3e3e3e] px-4 py-3 flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h1 className="text-white">ClaraTeach Workspace</h1>
          <span className="text-gray-400 text-sm">Class: {classCode}</span>
        </div>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2 text-gray-300">
            <User className="w-4 h-4" />
            <span className="text-sm">{learnerName}</span>
          </div>
          <Button variant="ghost" size="sm" onClick={onLeave} className="text-gray-300 hover:text-white">
            <ArrowLeft className="w-4 h-4 mr-2" />
            Leave
          </Button>
        </div>
      </div>

      {/* Main Content */}
      <div className="h-[calc(100vh-57px)]">
        <ResizablePanelGroup direction="horizontal">
          {/* Left Section - Code Editor and Terminal */}
          <ResizablePanel defaultSize={60} minSize={30}>
            <ResizablePanelGroup direction="vertical">
              {/* Code Editor */}
              <ResizablePanel defaultSize={60} minSize={30}>
                <CodeEditor />
              </ResizablePanel>

              <ResizableHandle withHandle />

              {/* Terminal */}
              <ResizablePanel defaultSize={40} minSize={20}>
                <Terminal />
              </ResizablePanel>
            </ResizablePanelGroup>
          </ResizablePanel>

          <ResizableHandle withHandle />

          {/* Right Section - Browser (Full Height) */}
          <ResizablePanel defaultSize={40} minSize={25}>
            <Browser />
          </ResizablePanel>
        </ResizablePanelGroup>
      </div>
    </div>
  );
}
