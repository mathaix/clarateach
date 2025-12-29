import { useState } from 'react';
import { LogIn, ArrowLeft } from 'lucide-react';
import { Button } from './ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Label } from './ui/label';

interface LearnerJoinProps {
  onJoin: (code: string, name: string) => void;
  onBack: () => void;
}

export function LearnerJoin({ onJoin, onBack }: LearnerJoinProps) {
  const [classCode, setClassCode] = useState('');
  const [learnerName, setLearnerName] = useState('');
  const [error, setError] = useState('');

  const handleJoin = (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!classCode.trim() || !learnerName.trim()) {
      setError('Please fill in all fields');
      return;
    }

    if (classCode.length !== 6) {
      setError('Class code must be 6 characters');
      return;
    }

    onJoin(classCode.toUpperCase(), learnerName);
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-green-50 to-teal-100 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <Button
          variant="ghost"
          className="mb-4"
          onClick={onBack}
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          Back
        </Button>

        <Card>
          <CardHeader className="text-center">
            <div className="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4">
              <LogIn className="w-8 h-8 text-green-600" />
            </div>
            <CardTitle>Join a Class</CardTitle>
            <CardDescription>
              Enter the class code from your teacher to get started
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleJoin} className="space-y-4">
              <div>
                <Label htmlFor="learnerName">Your Name</Label>
                <Input
                  id="learnerName"
                  placeholder="e.g., Alex Smith"
                  value={learnerName}
                  onChange={(e) => setLearnerName(e.target.value)}
                  required
                />
              </div>

              <div>
                <Label htmlFor="classCode">Class Code</Label>
                <Input
                  id="classCode"
                  placeholder="ABC123"
                  value={classCode}
                  onChange={(e) => setClassCode(e.target.value.toUpperCase())}
                  maxLength={6}
                  className="text-2xl tracking-wider text-center uppercase"
                  required
                />
                <p className="text-sm text-gray-500 mt-1">
                  6-character code from your teacher
                </p>
              </div>

              {error && (
                <div className="p-3 bg-red-50 border border-red-200 rounded-lg">
                  <p className="text-sm text-red-600">{error}</p>
                </div>
              )}

              <Button type="submit" className="w-full">
                <LogIn className="w-4 h-4 mr-2" />
                Join Class
              </Button>
            </form>

            <div className="mt-6 p-4 bg-blue-50 rounded-lg">
              <p className="text-sm text-gray-700">
                <strong>What happens next:</strong>
              </p>
              <ul className="text-sm text-gray-600 mt-2 space-y-1">
                <li>✓ You'll get your own private workspace</li>
                <li>✓ Your work is saved during the class</li>
                <li>✓ You can rejoin if disconnected</li>
              </ul>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
