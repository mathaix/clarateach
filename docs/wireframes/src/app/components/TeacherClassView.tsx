import { useState } from 'react';
import { Users, Copy, Check, ArrowLeft, StopCircle } from 'lucide-react';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';

interface Learner {
  id: string;
  name: string;
  joinedAt: Date;
  status: 'active' | 'disconnected';
}

interface TeacherClassViewProps {
  classCode: string;
  className: string;
  onBack: () => void;
  onEndClass: () => void;
}

export function TeacherClassView({ classCode, className, onBack, onEndClass }: TeacherClassViewProps) {
  const [copied, setCopied] = useState(false);
  const [learners] = useState<Learner[]>([
    { id: '1', name: 'Alex Johnson', joinedAt: new Date(Date.now() - 1000 * 60 * 10), status: 'active' },
    { id: '2', name: 'Sam Lee', joinedAt: new Date(Date.now() - 1000 * 60 * 8), status: 'active' },
    { id: '3', name: 'Jordan Kim', joinedAt: new Date(Date.now() - 1000 * 60 * 5), status: 'active' },
    { id: '4', name: 'Taylor Swift', joinedAt: new Date(Date.now() - 1000 * 60 * 12), status: 'disconnected' },
  ]);

  const handleCopyCode = () => {
    navigator.clipboard.writeText(classCode);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const getTimeSince = (date: Date) => {
    const minutes = Math.floor((Date.now() - date.getTime()) / 1000 / 60);
    if (minutes < 1) return 'just now';
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ${minutes % 60}m ago`;
  };

  const activeLearners = learners.filter(l => l.status === 'active').length;

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b">
        <div className="max-w-7xl mx-auto px-4 py-6">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <Button variant="ghost" onClick={onBack}>
                <ArrowLeft className="w-4 h-4 mr-2" />
                Dashboard
              </Button>
              <div>
                <h1 className="text-3xl text-gray-900">{className}</h1>
                <p className="text-gray-600 mt-1">Managing active class session</p>
              </div>
            </div>
            <Button variant="destructive" onClick={onEndClass}>
              <StopCircle className="w-4 h-4 mr-2" />
              End Class
            </Button>
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 py-8">
        <div className="grid lg:grid-cols-3 gap-6">
          {/* Class Info */}
          <div className="lg:col-span-1 space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>Join Information</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div>
                    <p className="text-sm text-gray-600 mb-2">Class Code</p>
                    <div className="flex items-center gap-2">
                      <div className="flex-1 p-4 bg-indigo-50 rounded-lg text-center">
                        <p className="text-3xl tracking-wider text-indigo-900">{classCode}</p>
                      </div>
                      <Button
                        variant="outline"
                        size="icon"
                        onClick={handleCopyCode}
                      >
                        {copied ? (
                          <Check className="w-4 h-4 text-green-600" />
                        ) : (
                          <Copy className="w-4 h-4" />
                        )}
                      </Button>
                    </div>
                  </div>

                  <div className="pt-4 border-t">
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-sm text-gray-600">Active Learners</span>
                      <span className="text-2xl text-gray-900">{activeLearners}</span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-gray-600">Total Joined</span>
                      <span className="text-lg text-gray-700">{learners.length}</span>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Workshop Status</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-gray-600">Workshop Machine</span>
                    <span className="bg-green-100 text-green-800 text-xs px-2 py-1 rounded-full">
                      Running
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-gray-600">Front Door</span>
                    <span className="bg-green-100 text-green-800 text-xs px-2 py-1 rounded-full">
                      Active
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-gray-600">Secure Vault</span>
                    <span className="bg-green-100 text-green-800 text-xs px-2 py-1 rounded-full">
                      Connected
                    </span>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Learners List */}
          <div className="lg:col-span-2">
            <Card>
              <CardHeader>
                <div className="flex items-center gap-2">
                  <Users className="w-5 h-5" />
                  <CardTitle>Joined Learners</CardTitle>
                </div>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {learners.map((learner) => (
                    <div
                      key={learner.id}
                      className="flex items-center justify-between p-4 bg-gray-50 rounded-lg hover:bg-gray-100 transition-colors"
                    >
                      <div className="flex items-center gap-3">
                        <div className={`w-2 h-2 rounded-full ${
                          learner.status === 'active' ? 'bg-green-500' : 'bg-gray-400'
                        }`} />
                        <div>
                          <p className="text-gray-900">{learner.name}</p>
                          <p className="text-sm text-gray-500">
                            Joined {getTimeSince(learner.joinedAt)}
                          </p>
                        </div>
                      </div>
                      <span className={`text-xs px-2 py-1 rounded-full ${
                        learner.status === 'active'
                          ? 'bg-green-100 text-green-800'
                          : 'bg-gray-200 text-gray-600'
                      }`}>
                        {learner.status === 'active' ? 'Active' : 'Disconnected'}
                      </span>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
}
