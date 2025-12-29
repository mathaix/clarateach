import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels';
import { Terminal } from './components/Terminal';
import { Editor } from './components/Editor';
import { Browser } from './components/Browser';
import { User, LogOut } from 'lucide-react';

export default function App() {
  return (
    <div className="h-screen w-screen bg-vscode-bg overflow-hidden flex flex-col">
      {/* Header */}
      <header className="bg-vscode-header border-b border-vscode-border px-4 py-3 flex items-center justify-between flex-shrink-0">
        <div className="flex items-center gap-4">
          <h1 className="text-white font-semibold">ClaraTeach Workspace</h1>
          <span className="text-gray-400 text-sm">Local Development</span>
        </div>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2 text-gray-300">
            <User className="w-4 h-4" />
            <span className="text-sm">Developer</span>
          </div>
          <button className="flex items-center gap-2 text-gray-300 hover:text-white transition-colors">
            <LogOut className="w-4 h-4" />
            <span className="text-sm">Leave</span>
          </button>
        </div>
      </header>

      {/* Main Content */}
      <div className="flex-1 overflow-hidden">
        <PanelGroup direction="horizontal">
          {/* Left Section - Editor and Terminal */}
          <Panel defaultSize={60} minSize={30}>
            <PanelGroup direction="vertical">
              {/* Editor */}
              <Panel defaultSize={60} minSize={20}>
                <Editor />
              </Panel>

              <PanelResizeHandle className="h-1 bg-vscode-border hover:bg-vscode-accent transition-colors" />

              {/* Terminal */}
              <Panel defaultSize={40} minSize={15}>
                <Terminal />
              </Panel>
            </PanelGroup>
          </Panel>

          <PanelResizeHandle className="w-1 bg-vscode-border hover:bg-vscode-accent transition-colors" />

          {/* Right Section - Browser */}
          <Panel defaultSize={40} minSize={25}>
            <Browser />
          </Panel>
        </PanelGroup>
      </div>
    </div>
  );
}
