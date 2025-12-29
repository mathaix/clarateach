import { Users, GraduationCap } from 'lucide-react';
import { Button } from './ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';

interface LandingPageProps {
  onSelectRole: (role: 'teacher' | 'learner') => void;
}

export function LandingPage({ onSelectRole }: LandingPageProps) {
  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 flex items-center justify-center p-4">
      <div className="max-w-4xl w-full">
        <div className="text-center mb-12">
          <h1 className="text-5xl mb-4 text-indigo-900">ClaraTeach</h1>
          <p className="text-xl text-gray-700">Live, hands-on learning with private practice spaces</p>
        </div>

        <div className="grid md:grid-cols-2 gap-6">
          {/* Teacher Card */}
          <Card className="hover:shadow-xl transition-shadow cursor-pointer" onClick={() => onSelectRole('teacher')}>
            <CardHeader>
              <div className="w-16 h-16 bg-indigo-100 rounded-full flex items-center justify-center mb-4">
                <GraduationCap className="w-8 h-8 text-indigo-600" />
              </div>
              <CardTitle>I'm a Teacher</CardTitle>
              <CardDescription>Create and manage live classes</CardDescription>
            </CardHeader>
            <CardContent>
              <ul className="space-y-2 text-sm text-gray-600 mb-6">
                <li className="flex items-start gap-2">
                  <span className="text-indigo-600 mt-1">✓</span>
                  <span>Start classes in seconds</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-indigo-600 mt-1">✓</span>
                  <span>Monitor learner progress</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-indigo-600 mt-1">✓</span>
                  <span>Manage class sessions</span>
                </li>
              </ul>
              <Button className="w-full" onClick={() => onSelectRole('teacher')}>
                Get Started
              </Button>
            </CardContent>
          </Card>

          {/* Learner Card */}
          <Card className="hover:shadow-xl transition-shadow cursor-pointer" onClick={() => onSelectRole('learner')}>
            <CardHeader>
              <div className="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mb-4">
                <Users className="w-8 h-8 text-green-600" />
              </div>
              <CardTitle>I'm a Learner</CardTitle>
              <CardDescription>Join a class with your code</CardDescription>
            </CardHeader>
            <CardContent>
              <ul className="space-y-2 text-sm text-gray-600 mb-6">
                <li className="flex items-start gap-2">
                  <span className="text-green-600 mt-1">✓</span>
                  <span>Private practice space</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-green-600 mt-1">✓</span>
                  <span>Hands-on coding environment</span>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-green-600 mt-1">✓</span>
                  <span>Rejoin anytime during class</span>
                </li>
              </ul>
              <Button className="w-full" variant="outline" onClick={() => onSelectRole('learner')}>
                Join Class
              </Button>
            </CardContent>
          </Card>
        </div>

        <div className="mt-12 text-center text-sm text-gray-600">
          <p>✨ Temporary workspaces keep costs low</p>
          <p>🔒 Privacy-first: learners are separated from each other</p>
        </div>
      </div>
    </div>
  );
}
