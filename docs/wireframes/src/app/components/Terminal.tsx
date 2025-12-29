import { useState, useRef, useEffect } from 'react';
import { Terminal as TerminalIcon } from 'lucide-react';

interface CommandHistory {
  command: string;
  output: string;
}

export function Terminal() {
  const [input, setInput] = useState('');
  const [history, setHistory] = useState<CommandHistory[]>([
    { command: 'welcome', output: 'Ubuntu 22.04.3 LTS\nWelcome to the VM Terminal\nType "help" for available commands' }
  ]);
  const terminalEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    terminalEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [history]);

  const executeCommand = (cmd: string) => {
    const trimmedCmd = cmd.trim();
    let output = '';

    switch (trimmedCmd.toLowerCase()) {
      case 'help':
        output = `Available commands:
  help     - Show this help message
  ls       - List directory contents
  pwd      - Print working directory
  date     - Show current date and time
  whoami   - Display current user
  clear    - Clear terminal
  echo     - Echo text back`;
        break;
      case 'ls':
        output = 'Documents  Downloads  Pictures  Videos  projects  index.html';
        break;
      case 'pwd':
        output = '/home/user';
        break;
      case 'date':
        output = new Date().toString();
        break;
      case 'whoami':
        output = 'user';
        break;
      case 'clear':
        setHistory([]);
        return;
      case '':
        return;
      default:
        if (trimmedCmd.startsWith('echo ')) {
          output = trimmedCmd.substring(5);
        } else {
          output = `bash: ${trimmedCmd}: command not found`;
        }
    }

    setHistory(prev => [...prev, { command: trimmedCmd, output }]);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    executeCommand(input);
    setInput('');
  };

  const handleTerminalClick = () => {
    inputRef.current?.focus();
  };

  return (
    <div className="h-full bg-[#1e1e1e] text-[#cccccc] flex flex-col font-mono">
      {/* Header */}
      <div className="bg-[#2d2d2d] border-b border-[#3e3e3e] px-4 py-2 flex items-center gap-2">
        <TerminalIcon className="w-4 h-4" />
        <span>Terminal</span>
      </div>

      {/* Terminal Content */}
      <div
        className="flex-1 overflow-auto p-4 cursor-text"
        onClick={handleTerminalClick}
      >
        {history.map((item, idx) => (
          <div key={idx} className="mb-2">
            <div className="flex items-center gap-2">
              <span className="text-[#4ec9b0]">user@vm</span>
              <span className="text-[#cccccc]">:</span>
              <span className="text-[#569cd6]">~</span>
              <span className="text-[#cccccc]">$</span>
              <span className="text-[#cccccc]">{item.command}</span>
            </div>
            {item.output && (
              <div className="mt-1 whitespace-pre-wrap text-[#d4d4d4]">
                {item.output}
              </div>
            )}
          </div>
        ))}

        {/* Input Line */}
        <form onSubmit={handleSubmit} className="flex items-center gap-2">
          <span className="text-[#4ec9b0]">user@vm</span>
          <span className="text-[#cccccc]">:</span>
          <span className="text-[#569cd6]">~</span>
          <span className="text-[#cccccc]">$</span>
          <input
            ref={inputRef}
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            className="flex-1 bg-transparent outline-none text-[#cccccc]"
            autoFocus
          />
        </form>

        <div ref={terminalEndRef} />
      </div>
    </div>
  );
}
