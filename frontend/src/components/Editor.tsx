import { useState, useEffect } from 'react';
import MonacoEditor from '@monaco-editor/react';
import { Code, Folder, FileText, ChevronRight, ChevronDown, RefreshCw } from 'lucide-react';
import { getWorkspaceSession } from '../lib/workspaceSession';

interface FileInfo {
  name: string;
  path: string;
  is_directory: boolean;
  size: number;
  modified_at: string;
}

interface FileTreeItemProps {
  file: FileInfo;
  depth: number;
  onSelect: (file: FileInfo) => void;
  selectedPath: string | null;
  onLoadChildren: (path: string) => Promise<FileInfo[]>;
}

function FileTreeItem({ file, depth, onSelect, selectedPath, onLoadChildren }: FileTreeItemProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [children, setChildren] = useState<FileInfo[]>([]);
  const [loading, setLoading] = useState(false);

  const handleClick = async () => {
    if (file.is_directory) {
      if (!isOpen && children.length === 0) {
        setLoading(true);
        try {
          const files = await onLoadChildren(file.path);
          setChildren(files);
        } catch (err) {
          console.error('Failed to load directory:', err);
        }
        setLoading(false);
      }
      setIsOpen(!isOpen);
    } else {
      onSelect(file);
    }
  };

  const isSelected = selectedPath === file.path;

  return (
    <div>
      <div
        className={`flex items-center gap-1 px-2 py-1 cursor-pointer hover:bg-vscode-header ${
          isSelected ? 'bg-vscode-header' : ''
        }`}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
        onClick={handleClick}
      >
        {file.is_directory && (
          isOpen ? (
            <ChevronDown className="w-3 h-3 text-vscode-text flex-shrink-0" />
          ) : (
            <ChevronRight className="w-3 h-3 text-vscode-text flex-shrink-0" />
          )
        )}
        {file.is_directory ? (
          <Folder className="w-4 h-4 text-yellow-500 flex-shrink-0" />
        ) : (
          <FileText className="w-4 h-4 text-vscode-text flex-shrink-0" />
        )}
        <span className="text-sm text-vscode-text truncate">{file.name}</span>
        {loading && <RefreshCw className="w-3 h-3 text-vscode-text animate-spin" />}
      </div>
      {file.is_directory && isOpen && children.map((child) => (
        <FileTreeItem
          key={child.path}
          file={child}
          depth={depth + 1}
          onSelect={onSelect}
          selectedPath={selectedPath}
          onLoadChildren={onLoadChildren}
        />
      ))}
    </div>
  );
}

export function Editor() {
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [selectedFile, setSelectedFile] = useState<FileInfo | null>(null);
  const [content, setContent] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const session = getWorkspaceSession();
  const API_BASE = session ? `${session.endpoint}/vm/${session.seat}` : '';

  const getHeaders = (extra?: Record<string, string>): Record<string, string> => {
    const headers: Record<string, string> = { ...extra };
    if (session?.token) {
      headers['Authorization'] = `Bearer ${session.token}`;
    }
    return headers;
  };

  const encodePath = (filePath: string): string =>
    filePath.split('/').map(segment => encodeURIComponent(segment)).join('/');

  const parseJson = async <T,>(response: Response): Promise<T> => {
    const text = await response.text();
    if (!text) {
      throw new Error('Empty response from workspace');
    }
    try {
      return JSON.parse(text) as T;
    } catch {
      throw new Error('Invalid JSON response from workspace');
    }
  };

  const loadDirectory = async (path: string = ''): Promise<FileInfo[]> => {
    const url = path ? `${API_BASE}/files?path=${encodeURIComponent(path)}` : `${API_BASE}/files`;
    const response = await fetch(url, {
      headers: getHeaders(),
    });
    const data = await parseJson<{ files?: FileInfo[]; error?: { message?: string } }>(response);
    if (!response.ok) {
      throw new Error(data.error?.message || `Request failed (${response.status})`);
    }
    return data.files || [];
  };

  useEffect(() => {
    if (!API_BASE) {
      setLoading(false);
      return;
    }
    loadDirectory()
      .then(setFiles)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [API_BASE]);

  const handleFileSelect = async (file: FileInfo) => {
    if (file.is_directory) return;

    setSelectedFile(file);
    try {
      const filePath = file.path.startsWith('/') ? file.path.slice(1) : file.path;
      const response = await fetch(`${API_BASE}/files/${encodePath(filePath)}`, {
        headers: getHeaders(),
      });
      const data = await parseJson<{ content?: string; error?: { message?: string } }>(response);
      if (!response.ok) {
        throw new Error(data.error?.message || `Request failed (${response.status})`);
      }
      setContent(data.content || '');
    } catch (err) {
      console.error('Failed to load file:', err);
    }
  };

  const handleSave = async () => {
    if (!selectedFile) return;

    setSaving(true);
    try {
      const filePath = selectedFile.path.startsWith('/') ? selectedFile.path.slice(1) : selectedFile.path;
      const response = await fetch(`${API_BASE}/files/${encodePath(filePath)}`, {
        method: 'PUT',
        headers: getHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({ content }),
      });
      if (!response.ok) {
        const data = await parseJson<{ error?: { message?: string } }>(response);
        throw new Error(data.error?.message || `Request failed (${response.status})`);
      }
    } catch (err) {
      console.error('Failed to save file:', err);
    }
    setSaving(false);
  };

  // Get file language for Monaco
  const getLanguage = (filename: string): string => {
    const ext = filename.split('.').pop()?.toLowerCase();
    const languageMap: Record<string, string> = {
      js: 'javascript',
      jsx: 'javascript',
      ts: 'typescript',
      tsx: 'typescript',
      py: 'python',
      json: 'json',
      md: 'markdown',
      html: 'html',
      css: 'css',
      sh: 'shell',
      bash: 'shell',
      yaml: 'yaml',
      yml: 'yaml',
    };
    return languageMap[ext || ''] || 'plaintext';
  };

  return (
    <div className="h-full bg-vscode-bg flex flex-col">
      {/* Header */}
      <div className="bg-vscode-sidebar border-b border-vscode-border px-4 py-2 flex items-center justify-between flex-shrink-0">
        <div className="flex items-center gap-2">
          <Code className="w-4 h-4 text-vscode-text" />
          <span className="text-vscode-text text-sm">
            {selectedFile ? selectedFile.name : 'Editor'}
          </span>
        </div>
        {selectedFile && (
          <button
            onClick={handleSave}
            disabled={saving}
            className="text-xs px-2 py-1 bg-vscode-header hover:bg-vscode-border rounded text-vscode-text disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Save'}
          </button>
        )}
      </div>

      <div className="flex-1 flex overflow-hidden">
        {/* File Explorer */}
        <div className="w-56 bg-vscode-sidebar border-r border-vscode-border overflow-auto flex-shrink-0">
          <div className="px-3 py-2 text-xs text-vscode-text uppercase tracking-wider">
            Explorer
          </div>
          {!session ? (
            <div className="px-3 py-2 text-sm text-vscode-text">
              Missing workspace session
            </div>
          ) : loading ? (
            <div className="px-3 py-2 text-sm text-vscode-text">Loading...</div>
          ) : files.length === 0 ? (
            <div className="px-3 py-2 text-sm text-vscode-text">No files</div>
          ) : (
            files.map((file) => (
              <FileTreeItem
                key={file.path}
                file={file}
                depth={0}
                onSelect={handleFileSelect}
                selectedPath={selectedFile?.path || null}
                onLoadChildren={loadDirectory}
              />
            ))
          )}
        </div>

        {/* Monaco Editor */}
        <div className="flex-1">
          {selectedFile ? (
            <MonacoEditor
              height="100%"
              language={getLanguage(selectedFile.name)}
              value={content}
              onChange={(value) => setContent(value || '')}
              theme="vs-dark"
              options={{
                fontSize: 14,
                fontFamily: 'Menlo, Monaco, "Courier New", monospace',
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                wordWrap: 'on',
                automaticLayout: true,
              }}
            />
          ) : (
            <div className="h-full flex items-center justify-center text-vscode-text">
              <div className="text-center">
                <FileText className="w-12 h-12 mx-auto mb-4 opacity-50" />
                <p>Select a file to edit</p>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
