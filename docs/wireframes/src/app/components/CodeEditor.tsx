import { useState } from 'react';
import { Code, Folder, FileText, ChevronRight } from 'lucide-react';

interface FileNode {
  name: string;
  type: 'file' | 'folder';
  content?: string;
  children?: FileNode[];
}

const initialFiles: FileNode[] = [
  {
    name: 'src',
    type: 'folder',
    children: [
      {
        name: 'index.html',
        type: 'file',
        content: `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>My App</title>
</head>
<body>
    <h1>Hello World!</h1>
    <script src="app.js"></script>
</body>
</html>`
      },
      {
        name: 'app.js',
        type: 'file',
        content: `// Main application file
console.log('Hello from app.js');

function init() {
    console.log('Application initialized');
}

init();`
      },
      {
        name: 'styles.css',
        type: 'file',
        content: `body {
    margin: 0;
    padding: 20px;
    font-family: Arial, sans-serif;
    background-color: #f0f0f0;
}

h1 {
    color: #333;
}`
      }
    ]
  },
  {
    name: 'README.md',
    type: 'file',
    content: `# My Project

This is a sample project running in the VM.

## Getting Started

1. Edit files in the src directory
2. View the output in the browser panel
3. Run commands in the terminal`
  }
];

interface FileTreeItemProps {
  node: FileNode;
  depth: number;
  onFileSelect: (node: FileNode) => void;
  selectedFile: FileNode | null;
}

function FileTreeItem({ node, depth, onFileSelect, selectedFile }: FileTreeItemProps) {
  const [isOpen, setIsOpen] = useState(true);

  const handleClick = () => {
    if (node.type === 'folder') {
      setIsOpen(!isOpen);
    } else {
      onFileSelect(node);
    }
  };

  return (
    <div>
      <div
        className={`flex items-center gap-1 px-2 py-1 cursor-pointer hover:bg-[#2a2d2e] ${
          selectedFile === node ? 'bg-[#37373d]' : ''
        }`}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
        onClick={handleClick}
      >
        {node.type === 'folder' && (
          <ChevronRight
            className={`w-3 h-3 transition-transform ${isOpen ? 'rotate-90' : ''}`}
          />
        )}
        {node.type === 'folder' ? (
          <Folder className="w-4 h-4 text-[#dcb67a]" />
        ) : (
          <FileText className="w-4 h-4 text-[#6d8086]" />
        )}
        <span className="text-sm">{node.name}</span>
      </div>
      {node.type === 'folder' && isOpen && node.children && (
        <div>
          {node.children.map((child, idx) => (
            <FileTreeItem
              key={idx}
              node={child}
              depth={depth + 1}
              onFileSelect={onFileSelect}
              selectedFile={selectedFile}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function CodeEditor() {
  const [selectedFile, setSelectedFile] = useState<FileNode | null>(initialFiles[0].children![0]);
  const [fileContent, setFileContent] = useState(initialFiles[0].children![0].content || '');

  const handleFileSelect = (node: FileNode) => {
    setSelectedFile(node);
    setFileContent(node.content || '');
  };

  const handleContentChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setFileContent(e.target.value);
    if (selectedFile && selectedFile.content !== undefined) {
      selectedFile.content = e.target.value;
    }
  };

  return (
    <div className="h-full bg-[#1e1e1e] text-[#cccccc] flex flex-col">
      {/* Header */}
      <div className="bg-[#2d2d2d] border-b border-[#3e3e3e] px-4 py-2 flex items-center gap-2">
        <Code className="w-4 h-4" />
        <span>Code Editor</span>
      </div>

      <div className="flex-1 flex overflow-hidden">
        {/* File Explorer */}
        <div className="w-64 bg-[#252526] border-r border-[#3e3e3e] overflow-auto">
          <div className="px-2 py-2 text-xs text-[#cccccc] uppercase tracking-wider">
            Explorer
          </div>
          {initialFiles.map((node, idx) => (
            <FileTreeItem
              key={idx}
              node={node}
              depth={0}
              onFileSelect={handleFileSelect}
              selectedFile={selectedFile}
            />
          ))}
        </div>

        {/* Editor */}
        <div className="flex-1 flex flex-col">
          {selectedFile && (
            <>
              <div className="bg-[#2d2d2d] border-b border-[#3e3e3e] px-4 py-2 flex items-center gap-2">
                <FileText className="w-4 h-4" />
                <span className="text-sm">{selectedFile.name}</span>
              </div>
              <div className="flex-1 overflow-auto">
                <textarea
                  value={fileContent}
                  onChange={handleContentChange}
                  className="w-full h-full bg-[#1e1e1e] text-[#d4d4d4] p-4 outline-none resize-none font-mono"
                  spellCheck={false}
                  style={{ tabSize: 4 }}
                />
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
