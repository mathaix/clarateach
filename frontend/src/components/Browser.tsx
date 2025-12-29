import { useState } from 'react';
import { Globe, RefreshCw, ExternalLink } from 'lucide-react';

export function Browser() {
  // neko URL with auto-login parameters
  const [nekoUrl] = useState(
    `http://${window.location.hostname}:8080/?usr=learner&pwd=neko`
  );

  const handleRefresh = () => {
    const iframe = document.getElementById('neko-frame') as HTMLIFrameElement;
    if (iframe) {
      iframe.src = iframe.src;
    }
  };

  const handleOpenExternal = () => {
    window.open(nekoUrl, '_blank');
  };

  return (
    <div className="h-full bg-vscode-bg flex flex-col">
      {/* Header */}
      <div className="bg-vscode-sidebar border-b border-vscode-border px-4 py-2 flex items-center justify-between flex-shrink-0">
        <div className="flex items-center gap-2">
          <Globe className="w-4 h-4 text-vscode-text" />
          <span className="text-vscode-text text-sm">Browser Preview</span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={handleRefresh}
            className="p-1 hover:bg-vscode-header rounded text-vscode-text"
            title="Refresh"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
          <button
            onClick={handleOpenExternal}
            className="p-1 hover:bg-vscode-header rounded text-vscode-text"
            title="Open in new tab"
          >
            <ExternalLink className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* neko iframe */}
      <div className="flex-1 bg-black">
        <iframe
          id="neko-frame"
          src={nekoUrl}
          className="w-full h-full border-0"
          allow="autoplay; clipboard-read; clipboard-write"
          title="Browser Preview"
        />
      </div>

      {/* Info bar */}
      <div className="bg-vscode-sidebar border-t border-vscode-border px-4 py-1 flex items-center justify-between flex-shrink-0">
        <span className="text-xs text-gray-500">
          neko browser • auto-login as "learner"
        </span>
        <span className="text-xs text-gray-500">
          localhost:8080
        </span>
      </div>
    </div>
  );
}
