import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { LogIn, ArrowLeft, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { api } from '@/lib/api';

export function Join() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const initialCode = searchParams.get('code') || '';

  const [code, setCode] = useState(initialCode);
  const [name, setName] = useState('');
  const [error, setError] = useState('');
  const [joining, setJoining] = useState(false);

  const handleJoin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!code.trim()) {
      setError('Please enter a workshop code');
      return;
    }

    setJoining(true);

    try {
      const response = await api.joinWorkshop({
        code: code.toUpperCase(),
        name: name.trim() || undefined,
      });

      // Store session info
      localStorage.setItem('clarateach_session', JSON.stringify({
        token: response.token,
        endpoint: response.endpoint,
        odehash: response.odehash,
        seat: response.seat,
        code: code.toUpperCase(),
        name: name.trim() || undefined,
      }));

      // Navigate to workspace
      navigate('/workspace');
    } catch (err) {
      console.error('Failed to join workshop:', err);
      setError(err instanceof Error ? err.message : 'Failed to join workshop');
    } finally {
      setJoining(false);
    }
  };

  const formatCode = (value: string) => {
    // Remove any non-alphanumeric characters
    const cleaned = value.replace(/[^A-Za-z0-9]/g, '').toUpperCase();
    // Add dash after 5 characters
    if (cleaned.length > 5) {
      return `${cleaned.slice(0, 5)}-${cleaned.slice(5, 9)}`;
    }
    return cleaned;
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-indigo-50 to-blue-100 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <Button variant="ghost" className="mb-4" onClick={() => navigate('/')}>
          <ArrowLeft className="w-4 h-4 mr-2" />
          Back
        </Button>

        <Card>
          <CardHeader className="text-center">
            <div className="w-16 h-16 bg-indigo-100 rounded-full flex items-center justify-center mx-auto mb-4">
              <LogIn className="w-8 h-8 text-indigo-600" />
            </div>
            <CardTitle>Join a Workshop</CardTitle>
            <CardDescription>Enter the workshop code from your instructor</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleJoin} className="space-y-4">
              <div>
                <Label htmlFor="name">Your Name (optional)</Label>
                <Input
                  id="name"
                  placeholder="e.g., Alex Smith"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>

              <div>
                <Label htmlFor="code">Workshop Code</Label>
                <Input
                  id="code"
                  placeholder="XXXXX-XXXX"
                  value={code}
                  onChange={(e) => setCode(formatCode(e.target.value))}
                  maxLength={10}
                  className="text-2xl tracking-wider text-center uppercase font-mono"
                  required
                />
                <p className="text-sm text-gray-500 mt-1">10-character code from your instructor</p>
              </div>

              {error && (
                <div className="p-3 bg-red-50 border border-red-200 rounded-lg">
                  <p className="text-sm text-red-600">{error}</p>
                </div>
              )}

              <Button type="submit" className="w-full" disabled={joining}>
                {joining ? (
                  <>
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    Joining...
                  </>
                ) : (
                  <>
                    <LogIn className="w-4 h-4 mr-2" />
                    Join Workshop
                  </>
                )}
              </Button>
            </form>

            <div className="mt-6 p-4 bg-blue-50 rounded-lg">
              <p className="text-sm text-gray-700 font-medium">What happens next:</p>
              <ul className="text-sm text-gray-600 mt-2 space-y-1">
                <li>You'll get your own private workspace</li>
                <li>Your work is saved during the workshop</li>
                <li>You can rejoin if disconnected</li>
              </ul>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
