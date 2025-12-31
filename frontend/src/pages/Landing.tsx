import { useNavigate } from 'react-router-dom';
import { GraduationCap, Users, LogIn } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useAuth } from '@/lib/auth';

export function Landing() {
  const navigate = useNavigate();
  const { isAuthenticated, user, logout } = useAuth();

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100">
      {/* Header */}
      <header className="bg-white border-b">
        <div className="max-w-7xl mx-auto px-4 h-16 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <GraduationCap className="w-7 h-7 text-indigo-600" />
              <span className="text-xl font-semibold text-gray-900">ClaraTeach</span>
            </div>
            <div className="flex items-center gap-3">
              {isAuthenticated ? (
                <>
                  <span className="text-sm text-gray-600 hidden sm:inline">
                    Hi, {user?.name}
                  </span>
                  <Button variant="outline" size="sm" onClick={() => navigate('/dashboard')}>
                    Dashboard
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => logout()}>
                    Sign out
                  </Button>
                </>
              ) : (
                <>
                  <Button variant="ghost" size="sm" onClick={() => navigate('/login')}>
                    Sign in
                  </Button>
                  <Button size="sm" onClick={() => navigate('/signup')}>
                    Sign up
                  </Button>
                </>
              )}
            </div>
        </div>
      </header>

      {/* Hero */}
      <div className="max-w-7xl mx-auto px-4 py-12 pb-24 sm:py-16">
        <div className="text-center mb-12">
          <h1 className="text-4xl sm:text-5xl font-bold text-gray-900 mb-4">
            Live Coding Workshops
          </h1>
          <p className="text-xl text-gray-600 max-w-2xl mx-auto">
            Create interactive coding workshops where every learner gets their own
            private workspace with Claude Code assistance.
          </p>
        </div>

        {/* Role Selection */}
        <div className="grid md:grid-cols-2 gap-8 max-w-4xl mx-auto">
          <Card className="hover:shadow-lg transition-shadow cursor-pointer" onClick={() => navigate('/dashboard')}>
            <CardHeader className="text-center pb-2">
              <div className="w-16 h-16 bg-indigo-100 rounded-full flex items-center justify-center mx-auto mb-4">
                <Users className="w-8 h-8 text-indigo-600" />
              </div>
              <CardTitle className="text-2xl">I'm an Instructor</CardTitle>
              <CardDescription className="text-base">
                Create and manage workshops for your learners
              </CardDescription>
            </CardHeader>
            <CardContent className="text-center">
              <ul className="text-sm text-gray-600 space-y-2 mb-6 text-left">
                <li className="flex items-center gap-2">
                  <span className="w-1.5 h-1.5 bg-indigo-500 rounded-full" />
                  Create workshop sessions with custom seat counts
                </li>
                <li className="flex items-center gap-2">
                  <span className="w-1.5 h-1.5 bg-indigo-500 rounded-full" />
                  Share join codes with your learners
                </li>
                <li className="flex items-center gap-2">
                  <span className="w-1.5 h-1.5 bg-indigo-500 rounded-full" />
                  Monitor learner progress in real-time
                </li>
              </ul>
              <Button className="w-full">
                <Users className="w-4 h-4 mr-2" />
                Go to Dashboard
              </Button>
            </CardContent>
          </Card>

          <Card className="hover:shadow-lg transition-shadow cursor-pointer" onClick={() => navigate('/join')}>
            <CardHeader className="text-center pb-2">
              <div className="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4">
                <LogIn className="w-8 h-8 text-green-600" />
              </div>
              <CardTitle className="text-2xl">I'm a Learner</CardTitle>
              <CardDescription className="text-base">
                Join an existing workshop with a code
              </CardDescription>
            </CardHeader>
            <CardContent className="text-center">
              <ul className="text-sm text-gray-600 space-y-2 mb-6 text-left">
                <li className="flex items-center gap-2">
                  <span className="w-1.5 h-1.5 bg-green-500 rounded-full" />
                  Enter the code provided by your instructor
                </li>
                <li className="flex items-center gap-2">
                  <span className="w-1.5 h-1.5 bg-green-500 rounded-full" />
                  Get your own private coding workspace
                </li>
                <li className="flex items-center gap-2">
                  <span className="w-1.5 h-1.5 bg-green-500 rounded-full" />
                  Use Claude Code to help you learn
                </li>
              </ul>
              <Button variant="outline" className="w-full">
                <LogIn className="w-4 h-4 mr-2" />
                Join Workshop
              </Button>
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Footer */}
      <div className="mt-8 md:mt-0 md:fixed bottom-0 left-0 right-0 bg-white border-t py-4">
        <div className="max-w-7xl mx-auto px-4 text-center text-sm text-gray-500">
          Built by ClaraMaps
        </div>
      </div>
    </div>
  );
}
