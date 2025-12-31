import { useState } from 'react';
import { useNavigate, useSearchParams, Link } from 'react-router-dom';
import { UserPlus, ArrowLeft, Loader2, GraduationCap } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { api } from '@/lib/api';

export function Register() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const initialWorkshopCode = searchParams.get('workshop') || '';

  const [workshopCode, setWorkshopCode] = useState(initialWorkshopCode);
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [error, setError] = useState('');
  const [registering, setRegistering] = useState(false);

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!workshopCode.trim()) {
      setError('Please enter a workshop code');
      return;
    }
    if (!email.trim()) {
      setError('Please enter your email');
      return;
    }
    if (!name.trim()) {
      setError('Please enter your name');
      return;
    }

    setRegistering(true);

    try {
      const response = await api.register({
        workshop_code: workshopCode.toUpperCase(),
        email: email.trim(),
        name: name.trim(),
      });

      // Save access code to localStorage for convenience
      localStorage.setItem('clarateach_access_code', response.access_code);

      // Navigate to registered page with the code
      navigate(`/registered/${response.access_code}`, {
        state: { alreadyRegistered: response.already_registered }
      });
    } catch (err) {
      console.error('Failed to register:', err);
      setError(err instanceof Error ? err.message : 'Failed to register');
    } finally {
      setRegistering(false);
    }
  };

  const formatWorkshopCode = (value: string) => {
    const cleaned = value.replace(/[^A-Za-z0-9]/g, '').toUpperCase();
    if (cleaned.length > 5) {
      return `${cleaned.slice(0, 5)}-${cleaned.slice(5, 9)}`;
    }
    return cleaned;
  };

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
          <Button variant="ghost" className="mb-4" onClick={() => navigate('/join')}>
            <ArrowLeft className="w-4 h-4 mr-2" />
            Back
          </Button>

          <Card className="min-h-[520px]">
          <CardHeader className="text-center">
            <div className="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4">
              <UserPlus className="w-8 h-8 text-green-600" />
            </div>
            <CardTitle>Register for Workshop</CardTitle>
            <CardDescription>Enter your details to get your access code</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleRegister} className="space-y-4">
              <div>
                <Label htmlFor="workshop-code">Workshop Code</Label>
                <Input
                  id="workshop-code"
                  placeholder="XXXXX-XXXX"
                  value={workshopCode}
                  onChange={(e) => setWorkshopCode(formatWorkshopCode(e.target.value))}
                  maxLength={10}
                  className="text-xl tracking-wider text-center uppercase font-mono"
                  required
                  disabled={!!initialWorkshopCode}
                />
                <p className="text-sm text-gray-500 mt-1">Code provided by your instructor</p>
              </div>

              <div>
                <Label htmlFor="email">Email Address</Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                />
              </div>

              <div>
                <Label htmlFor="name">Your Name</Label>
                <Input
                  id="name"
                  placeholder="e.g., Alex Smith"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
              </div>

              {error && (
                <div className="p-3 bg-red-50 border border-red-200 rounded-lg">
                  <p className="text-sm text-red-600">{error}</p>
                </div>
              )}

              <Button type="submit" className="w-full" disabled={registering}>
                {registering ? (
                  <>
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    Registering...
                  </>
                ) : (
                  <>
                    <UserPlus className="w-4 h-4 mr-2" />
                    Register
                  </>
                )}
              </Button>
            </form>

            <div className="mt-6 p-4 bg-green-50 rounded-lg">
              <p className="text-sm text-gray-700 font-medium">After registering:</p>
              <ul className="text-sm text-gray-600 mt-2 space-y-1">
                <li>You'll receive a personal access code</li>
                <li>Use this code to join and rejoin anytime</li>
                <li>Your workspace will be saved</li>
              </ul>
            </div>
          </CardContent>
        </Card>
        </div>
      </div>
    </div>
  );
}
