import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { LogIn, ArrowLeft, Loader2, UserPlus, KeyRound, GraduationCap } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

type Mode = 'select' | 'join';

export function Join() {
  const navigate = useNavigate();
  const savedAccessCode = localStorage.getItem('clarateach_access_code') || '';

  const [mode, setMode] = useState<Mode>(savedAccessCode ? 'join' : 'select');
  const [accessCode, setAccessCode] = useState(savedAccessCode);
  const [error, setError] = useState('');
  const [joining, setJoining] = useState(false);

  // If there's a saved code, pre-select join mode
  useEffect(() => {
    if (savedAccessCode) {
      setMode('join');
    }
  }, [savedAccessCode]);

  const handleJoin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!accessCode.trim()) {
      setError('Please enter your access code');
      return;
    }

    setJoining(true);

    try {
      // Navigate to workspace with access code
      navigate(`/s/${accessCode.toUpperCase()}`);
    } catch (err) {
      console.error('Failed to join:', err);
      setError(err instanceof Error ? err.message : 'Failed to join workshop');
      setJoining(false);
    }
  };

  const formatAccessCode = (value: string) => {
    const cleaned = value.replace(/[^A-Za-z0-9]/g, '').toUpperCase();
    if (cleaned.length > 3) {
      return `${cleaned.slice(0, 3)}-${cleaned.slice(3, 7)}`;
    }
    return cleaned;
  };

  if (mode === 'select') {
    return (
      <div className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100 flex flex-col">
        {/* Header */}
        <header className="bg-white border-b">
          <div className="max-w-7xl mx-auto px-4 h-16 flex items-center">
            <Link to="/" className="flex items-center gap-2 hover:opacity-80 transition-opacity">
              <GraduationCap className="w-7 h-7 text-indigo-600" />
              <span className="text-xl font-semibold text-gray-900">ClaraTeach</span>
            </Link>
          </div>
        </header>

        {/* Main */}
        <div className="flex-1 flex items-center justify-center p-4">
          <div className="w-full max-w-md">
            <Button variant="ghost" className="mb-4" onClick={() => navigate('/')}>
              <ArrowLeft className="w-4 h-4 mr-2" />
              Back to Home
            </Button>

            <Card className="min-h-[520px]">
            <CardHeader className="text-center">
              <div className="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4">
                <LogIn className="w-8 h-8 text-green-600" />
              </div>
              <CardTitle>Join a Workshop</CardTitle>
              <CardDescription>Choose how you want to join</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {/* Register Option */}
              <button
                onClick={() => navigate('/register')}
                className="w-full p-4 border-2 border-gray-200 rounded-xl hover:border-green-300 hover:bg-green-50 transition-all text-left group"
              >
                <div className="flex items-start gap-4">
                  <div className="w-12 h-12 bg-green-100 rounded-full flex items-center justify-center flex-shrink-0 group-hover:bg-green-200 transition-colors">
                    <UserPlus className="w-6 h-6 text-green-600" />
                  </div>
                  <div>
                    <p className="font-semibold text-gray-900">Register for a Workshop</p>
                    <p className="text-sm text-gray-500 mt-1">
                      New here? Register with the workshop code from your instructor
                    </p>
                  </div>
                </div>
              </button>

              {/* Divider */}
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-gray-200" />
                </div>
                <div className="relative flex justify-center text-sm">
                  <span className="px-2 bg-white text-gray-500">or</span>
                </div>
              </div>

              {/* Join with Code Option */}
              <button
                onClick={() => setMode('join')}
                className="w-full p-4 border-2 border-gray-200 rounded-xl hover:border-indigo-300 hover:bg-indigo-50 transition-all text-left group"
              >
                <div className="flex items-start gap-4">
                  <div className="w-12 h-12 bg-indigo-100 rounded-full flex items-center justify-center flex-shrink-0 group-hover:bg-indigo-200 transition-colors">
                    <KeyRound className="w-6 h-6 text-indigo-600" />
                  </div>
                  <div>
                    <p className="font-semibold text-gray-900">I Have an Access Code</p>
                    <p className="text-sm text-gray-500 mt-1">
                      Already registered? Enter your personal access code
                    </p>
                  </div>
                </div>
              </button>
            </CardContent>
          </Card>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100 flex flex-col">
      {/* Header */}
      <header className="bg-white border-b">
        <div className="max-w-7xl mx-auto px-4 h-16 flex items-center">
          <Link to="/" className="flex items-center gap-2 hover:opacity-80 transition-opacity">
            <GraduationCap className="w-7 h-7 text-indigo-600" />
            <span className="text-xl font-semibold text-gray-900">ClaraTeach</span>
          </Link>
        </div>
      </header>

      {/* Main */}
      <div className="flex-1 flex items-center justify-center p-4">
        <div className="w-full max-w-md">
          <Button variant="ghost" className="mb-4" onClick={() => setMode('select')}>
            <ArrowLeft className="w-4 h-4 mr-2" />
            Back
          </Button>

          <Card className="min-h-[520px]">
          <CardHeader className="text-center">
            <div className="w-16 h-16 bg-indigo-100 rounded-full flex items-center justify-center mx-auto mb-4">
              <KeyRound className="w-8 h-8 text-indigo-600" />
            </div>
            <CardTitle>Enter Access Code</CardTitle>
            <CardDescription>Use your personal access code to join</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleJoin} className="space-y-4">
              <div>
                <Label htmlFor="access-code">Access Code</Label>
                <Input
                  id="access-code"
                  placeholder="XXX-XXXX"
                  value={accessCode}
                  onChange={(e) => setAccessCode(formatAccessCode(e.target.value))}
                  maxLength={8}
                  className="text-2xl tracking-wider text-center uppercase font-mono"
                  required
                />
                <p className="text-sm text-gray-500 mt-1">Your personal access code from registration</p>
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

            <div className="mt-6 pt-4 border-t">
              <p className="text-sm text-gray-500 text-center">
                Don't have an access code?{' '}
                <button
                  onClick={() => navigate('/register')}
                  className="text-indigo-600 hover:underline font-medium"
                >
                  Register here
                </button>
              </p>
            </div>
          </CardContent>
        </Card>
        </div>
      </div>
    </div>
  );
}
