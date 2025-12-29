import { useState, useEffect } from 'react';
import MonacoEditor from '@monaco-editor/react';
import { Code, Folder, FileText, ChevronRight, ChevronDown, RefreshCw } from 'lucide-react';

interface FileInfo {
  name: string;
  path: string;
  isDirectory: boolean;
  size: number;
  modifiedAt: string;
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
    if (file.isDirectory) {
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
        {file.isDirectory && (
          isOpen ? (
            <ChevronDown className="w-3 h-3 text-vscode-text flex-shrink-0" />
          ) : (
            <ChevronRight className="w-3 h-3 text-vscode-text flex-shrink-0" />
          )
        )}
        {file.isDirectory ? (
          <Folder className="w-4 h-4 text-yellow-500 flex-shrink-0" />
        ) : (
          <FileText className="w-4 h-4 text-vscode-text flex-shrink-0" />
        )}
        <span className="text-sm text-vscode-text truncate">{file.name}</span>
        {loading && <RefreshCw className="w-3 h-3 text-vscode-text animate-spin" />}
      </div>
      {file.isDirectory && isOpen && children.map((child) => (
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

  const API_BASE = `http://${window.location.hostname}:3000`;

  const loadDirectory = async (path: string = ''): Promise<FileInfo[]> => {
    const url = path ? `${API_BASE}/files?path=${encodeURIComponent(path)}` : `${API_BASE}/files`;
    const response = await fetch(url);
    const data = await response.json();
    return data.files || [];
  };

  useEffect(() => {
    loadDirectory()
      .then(setFiles)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  const handleFileSelect = async (file: FileInfo) => {
    if (file.isDirectory) return;

    setSelectedFile(file);
    try {
      const response = await fetch(`${API_BASE}/files/${file.path}`);
      const data = await response.json();
      setContent(data.content || '');
    } catch (err) {
      console.error('Failed to load file:', err);
    }
  };

  const handleSave = async () => {
    if (!selectedFile) return;

    setSaving(true);
    try {
      await fetch(`${API_BASE}/files/${selectedFile.path}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content }),
      });
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
          {loading ? (
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
