import { Globe, RefreshCw, ExternalLink } from 'lucide-react';
import { getWorkspaceSession } from '../lib/workspaceSession';

export function Browser() {
  const session = getWorkspaceSession();
  let baseUrl = '';
  if (session) {
    // Check if endpoint has a path (debug proxy)
    const url = new URL(session.endpoint);
    const basePath = url.pathname === '/' ? '' : url.pathname;
    // Trailing slash is critical - ensures relative paths in Neko resolve correctly
    baseUrl = `${url.origin}${basePath}/vm/${session.seat}/browser/`;
  }
  const tokenParam = session?.token ? `?token=${encodeURIComponent(session.token)}` : '';
  const browserUrl = `${baseUrl}${tokenParam}`;

  const handleRefresh = () => {
    const iframe = document.getElementById('neko-frame') as HTMLIFrameElement;
    if (iframe) {
      iframe.src = iframe.src;
    }
  };

  const handleOpenExternal = () => {
    window.open(browserUrl, '_blank');
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

      {/* Browser iframe */}
      <div className="flex-1 bg-black">
        <iframe
          id="neko-frame"
          src={browserUrl}
          className="w-full h-full border-0"
          allow="autoplay; clipboard-read; clipboard-write"
          title="Browser Preview"
        />
      </div>

      {/* Info bar */}
      <div className="bg-vscode-sidebar border-t border-vscode-border px-4 py-1 flex items-center justify-between flex-shrink-0">
        <span className="text-xs text-gray-500">
          neko browser preview
        </span>
        <span className="text-xs text-gray-500">
          {session ? session.endpoint : 'missing workspace session'}
        </span>
      </div>
    </div>
  );
}
