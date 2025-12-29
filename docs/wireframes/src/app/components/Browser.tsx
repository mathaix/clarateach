import { useState } from 'react';
import { Globe, RefreshCw, ArrowLeft, ArrowRight, Home } from 'lucide-react';

export function Browser() {
  const [url, setUrl] = useState('http://localhost:3000');
  const [inputUrl, setInputUrl] = useState('http://localhost:3000');

  const handleNavigate = (e: React.FormEvent) => {
    e.preventDefault();
    setUrl(inputUrl);
  };

  const handleRefresh = () => {
    // Simulate refresh
    const temp = url;
    setUrl('');
    setTimeout(() => setUrl(temp), 100);
  };

  const handleHome = () => {
    const homeUrl = 'http://localhost:3000';
    setUrl(homeUrl);
    setInputUrl(homeUrl);
  };

  return (
    <div className="h-full bg-[#1e1e1e] text-[#cccccc] flex flex-col">
      {/* Header */}
      <div className="bg-[#2d2d2d] border-b border-[#3e3e3e] px-4 py-2 flex items-center gap-2">
        <Globe className="w-4 h-4" />
        <span>Browser</span>
      </div>

      {/* Browser Controls */}
      <div className="bg-[#252526] border-b border-[#3e3e3e] p-2 flex items-center gap-2">
        <button
          className="p-2 hover:bg-[#2a2d2e] rounded"
          title="Back"
          onClick={() => {}}
        >
          <ArrowLeft className="w-4 h-4" />
        </button>
        <button
          className="p-2 hover:bg-[#2a2d2e] rounded"
          title="Forward"
          onClick={() => {}}
        >
          <ArrowRight className="w-4 h-4" />
        </button>
        <button
          className="p-2 hover:bg-[#2a2d2e] rounded"
          title="Refresh"
          onClick={handleRefresh}
        >
          <RefreshCw className="w-4 h-4" />
        </button>
        <button
          className="p-2 hover:bg-[#2a2d2e] rounded"
          title="Home"
          onClick={handleHome}
        >
          <Home className="w-4 h-4" />
        </button>

        {/* Address Bar */}
        <form onSubmit={handleNavigate} className="flex-1 flex items-center">
          <input
            type="text"
            value={inputUrl}
            onChange={(e) => setInputUrl(e.target.value)}
            className="flex-1 bg-[#3c3c3c] text-[#cccccc] px-4 py-2 rounded outline-none focus:ring-2 focus:ring-[#007acc]"
            placeholder="Enter URL..."
          />
        </form>
      </div>

      {/* Browser Content Area */}
      <div className="flex-1 bg-white overflow-auto">
        {url ? (
          <div className="h-full flex items-center justify-center">
            <div className="text-center p-8">
              <Globe className="w-16 h-16 text-gray-400 mx-auto mb-4" />
              <h2 className="text-2xl font-semibold text-gray-800 mb-2">
                Browser View
              </h2>
              <p className="text-gray-600 mb-4">
                Currently viewing: <span className="font-mono text-blue-600">{url}</span>
              </p>
              <div className="bg-gray-50 rounded-lg p-6 max-w-2xl mx-auto text-left">
                <h3 className="font-semibold text-gray-800 mb-3">Sample Web Page</h3>
                <p className="text-gray-700 mb-4">
                  This is a simulated browser view running in the VM environment.
                  In a real implementation, this would display actual web content from the VM.
                </p>
                <div className="space-y-2">
                  <div className="bg-white p-3 rounded border border-gray-200">
                    <h4 className="font-medium text-gray-800">Feature 1</h4>
                    <p className="text-sm text-gray-600">Sample content section</p>
                  </div>
                  <div className="bg-white p-3 rounded border border-gray-200">
                    <h4 className="font-medium text-gray-800">Feature 2</h4>
                    <p className="text-sm text-gray-600">Another content section</p>
                  </div>
                  <div className="bg-white p-3 rounded border border-gray-200">
                    <h4 className="font-medium text-gray-800">Feature 3</h4>
                    <p className="text-sm text-gray-600">More sample content</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        ) : (
          <div className="h-full flex items-center justify-center">
            <p className="text-gray-400">Loading...</p>
          </div>
        )}
      </div>
    </div>
  );
}
