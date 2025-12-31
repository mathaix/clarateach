import { useEffect, useRef } from 'react';
import { Terminal as XTerm } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { WebLinksAddon } from 'xterm-addon-web-links';
import { Terminal as TerminalIcon } from 'lucide-react';
import { getWorkspaceSession } from '../lib/workspaceSession';
import 'xterm/css/xterm.css';

export function Terminal() {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);

  const focusTerminal = () => {
    xtermRef.current?.focus();
  };

  useEffect(() => {
    if (!terminalRef.current || xtermRef.current) return;

    // Create xterm instance
    const xterm = new XTerm({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      theme: {
        background: '#1e1e1e',
        foreground: '#cccccc',
        cursor: '#cccccc',
        cursorAccent: '#1e1e1e',
        selectionBackground: '#264f78',
        black: '#000000',
        red: '#cd3131',
        green: '#0dbc79',
        yellow: '#e5e510',
        blue: '#2472c8',
        magenta: '#bc3fbc',
        cyan: '#11a8cd',
        white: '#e5e5e5',
        brightBlack: '#666666',
        brightRed: '#f14c4c',
        brightGreen: '#23d18b',
        brightYellow: '#f5f543',
        brightBlue: '#3b8eea',
        brightMagenta: '#d670d6',
        brightCyan: '#29b8db',
        brightWhite: '#ffffff',
      },
    });

    const fitAddon = new FitAddon();
    const webLinksAddon = new WebLinksAddon();

    xterm.loadAddon(fitAddon);
    xterm.loadAddon(webLinksAddon);
    xterm.open(terminalRef.current);
    fitAddon.fit();

    // Focus terminal to receive input
    focusTerminal();

    xtermRef.current = xterm;
    fitAddonRef.current = fitAddon;

    const session = getWorkspaceSession();
    let ws: WebSocket | null = null;

    if (!session) {
      xterm.write('\r\n\x1b[31mMissing workspace session. Provide token/seat/endpoint.\x1b[0m\r\n');
    } else {
      const endpointUrl = new URL(session.endpoint);
      const wsProtocol = endpointUrl.protocol === 'https:' ? 'wss:' : 'ws:';
      const tokenParam = session.token ? `?token=${encodeURIComponent(session.token)}` : '';
      // Ensure we keep the pathname (e.g. /debug/proxy/ws-id)
      const basePath = endpointUrl.pathname === '/' ? '' : endpointUrl.pathname;
      const wsUrl = `${wsProtocol}//${endpointUrl.host}${basePath}/vm/${session.seat}/terminal${tokenParam}`;
      ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        console.log('Terminal WebSocket connected');
        // Send initial resize
        ws?.send(JSON.stringify({
          type: 'resize',
          cols: xterm.cols,
          rows: xterm.rows,
        }));
      };

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          if (msg.type === 'output') {
            xterm.write(msg.data);
          }
        } catch {
          // Handle raw data
          xterm.write(event.data);
        }
      };

      ws.onerror = (error) => {
        console.error('Terminal WebSocket error:', error);
        xterm.write('\r\n\x1b[31mConnection error. Is the workspace server running?\x1b[0m\r\n');
      };

      ws.onclose = () => {
        console.log('Terminal WebSocket closed');
        xterm.write('\r\n\x1b[33mConnection closed.\x1b[0m\r\n');
      };
    }

    // Handle terminal input
    xterm.onData((data) => {
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }));
      }
    });

    // Handle resize
    const handleResize = () => {
      fitAddon.fit();
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
          type: 'resize',
          cols: xterm.cols,
          rows: xterm.rows,
        }));
      }
    };

    window.addEventListener('resize', handleResize);

    // ResizeObserver for panel resizing
    const resizeObserver = new ResizeObserver(() => {
      handleResize();
    });
    resizeObserver.observe(terminalRef.current);

    return () => {
      window.removeEventListener('resize', handleResize);
      resizeObserver.disconnect();
      ws?.close();
      xterm.dispose();
      // Reset refs so effect can re-run after Strict Mode remount
      xtermRef.current = null;
      wsRef.current = null;
      fitAddonRef.current = null;
    };
  }, []);

  return (
    <div className="h-full bg-vscode-bg flex flex-col">
      {/* Header */}
      <div className="bg-vscode-sidebar border-b border-vscode-border px-4 py-2 flex items-center gap-2 flex-shrink-0">
        <TerminalIcon className="w-4 h-4 text-vscode-text" />
        <span className="text-vscode-text text-sm">Terminal</span>
      </div>

      {/* Terminal Content */}
      <div
        ref={terminalRef}
        className="flex-1 p-2 cursor-text"
        tabIndex={0}
        onClick={focusTerminal}
        onMouseDown={(event) => {
          event.preventDefault();
          focusTerminal();
        }}
        onTouchStart={(event) => {
          event.preventDefault();
          focusTerminal();
        }}
      />
    </div>
  );
}
