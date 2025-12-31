import { useState } from 'react';
import { useNavigate, useParams, useLocation } from 'react-router-dom';
import { CheckCircle, Copy, Check, ArrowRight, Bookmark } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';

export function Registered() {
  const navigate = useNavigate();
  const { code } = useParams<{ code: string }>();
  const location = useLocation();
  const alreadyRegistered = (location.state as { alreadyRegistered?: boolean })?.alreadyRegistered;

  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    if (code) {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleLaunch = () => {
    navigate(`/s/${code}`);
  };

  const workspaceUrl = `${window.location.origin}/s/${code}`;

  return (
    <div className="min-h-screen bg-gradient-to-br from-green-50 to-emerald-100 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <Card>
          <CardHeader className="text-center">
            <div className="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4">
              <CheckCircle className="w-10 h-10 text-green-600" />
            </div>
            <CardTitle className="text-2xl">
              {alreadyRegistered ? 'Already Registered!' : 'Registration Complete!'}
            </CardTitle>
            <CardDescription>
              {alreadyRegistered
                ? 'You were already registered. Here\'s your access code.'
                : 'Save your access code to join anytime.'}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* Access Code Display */}
            <div className="bg-white border-2 border-green-200 rounded-xl p-6 text-center">
              <p className="text-sm text-gray-500 mb-2">Your Access Code</p>
              <div className="flex items-center justify-center gap-2">
                <span className="text-3xl font-mono font-bold tracking-wider text-gray-900">
                  {code}
                </span>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleCopy}
                  className="h-10 w-10 p-0"
                >
                  {copied ? (
                    <Check className="w-5 h-5 text-green-600" />
                  ) : (
                    <Copy className="w-5 h-5 text-gray-400" />
                  )}
                </Button>
              </div>
            </div>

            {/* Warning */}
            <div className="bg-amber-50 border border-amber-200 rounded-lg p-4">
              <div className="flex gap-3">
                <Bookmark className="w-5 h-5 text-amber-600 flex-shrink-0 mt-0.5" />
                <div>
                  <p className="text-sm font-medium text-amber-800">Save this code!</p>
                  <p className="text-sm text-amber-700 mt-1">
                    You'll need it to access your workspace. Bookmark this page or copy the code.
                  </p>
                </div>
              </div>
            </div>

            {/* Launch Button */}
            <Button onClick={handleLaunch} className="w-full h-12 text-lg" size="lg">
              Launch Workshop
              <ArrowRight className="w-5 h-5 ml-2" />
            </Button>

            {/* Bookmark Link */}
            <div className="text-center">
              <p className="text-sm text-gray-500 mb-2">Or bookmark this link:</p>
              <code className="text-xs bg-gray-100 px-3 py-2 rounded-lg block overflow-x-auto">
                {workspaceUrl}
              </code>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
