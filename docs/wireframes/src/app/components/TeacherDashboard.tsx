import { useState } from 'react';
import { Plus, PlayCircle, Clock, Users } from 'lucide-react';
import { Button } from './ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Label } from './ui/label';

interface ClassSession {
  id: string;
  name: string;
  code: string;
  status: 'active' | 'scheduled';
  learners: number;
  maxSeats: number;
  createdAt: Date;
}

interface TeacherDashboardProps {
  onStartClass: (classData: { name: string; maxSeats: number }) => void;
  onViewClass: (classId: string) => void;
}

export function TeacherDashboard({ onStartClass, onViewClass }: TeacherDashboardProps) {
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [className, setClassName] = useState('');
  const [maxSeats, setMaxSeats] = useState('30');
  
  const [activeSessions] = useState<ClassSession[]>([
    {
      id: '1',
      name: 'Web Development 101',
      code: 'ABC123',
      status: 'active',
      learners: 12,
      maxSeats: 30,
      createdAt: new Date(Date.now() - 1000 * 60 * 15)
    }
  ]);

  const handleCreateClass = (e: React.FormEvent) => {
    e.preventDefault();
    if (className.trim()) {
      onStartClass({ name: className, maxSeats: parseInt(maxSeats) });
      setClassName('');
      setMaxSeats('30');
      setShowCreateForm(false);
    }
  };

  const getTimeSince = (date: Date) => {
    const minutes = Math.floor((Date.now() - date.getTime()) / 1000 / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ${minutes % 60}m ago`;
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b">
        <div className="max-w-7xl mx-auto px-4 py-6">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-3xl text-gray-900">Teacher Dashboard</h1>
              <p className="text-gray-600 mt-1">Manage your live classes</p>
            </div>
            <Button onClick={() => setShowCreateForm(!showCreateForm)}>
              <Plus className="w-4 h-4 mr-2" />
              Create Class
            </Button>
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 py-8">
        {/* Create Class Form */}
        {showCreateForm && (
          <Card className="mb-8">
            <CardHeader>
              <CardTitle>Create New Class</CardTitle>
              <CardDescription>Start a new live session for your learners</CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleCreateClass} className="space-y-4">
                <div>
                  <Label htmlFor="className">Class Name</Label>
                  <Input
                    id="className"
                    placeholder="e.g., Web Development 101"
                    value={className}
                    onChange={(e) => setClassName(e.target.value)}
                    required
                  />
                </div>
                <div>
                  <Label htmlFor="maxSeats">Maximum Seats</Label>
                  <Input
                    id="maxSeats"
                    type="number"
                    min="1"
                    max="100"
                    value={maxSeats}
                    onChange={(e) => setMaxSeats(e.target.value)}
                    required
                  />
                  <p className="text-sm text-gray-500 mt-1">
                    Each seat gets a private workspace
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button type="submit">
                    <PlayCircle className="w-4 h-4 mr-2" />
                    Start Class
                  </Button>
                  <Button type="button" variant="outline" onClick={() => setShowCreateForm(false)}>
                    Cancel
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        )}

        {/* Active Sessions */}
        <div className="mb-8">
          <h2 className="text-xl mb-4 text-gray-900">Active Classes</h2>
          {activeSessions.length > 0 ? (
            <div className="grid gap-4">
              {activeSessions.map((session) => (
                <Card key={session.id} className="hover:shadow-md transition-shadow">
                  <CardContent className="p-6">
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        <div className="flex items-center gap-3 mb-2">
                          <h3 className="text-lg text-gray-900">{session.name}</h3>
                          <span className="bg-green-100 text-green-800 text-xs px-2 py-1 rounded-full">
                            Active
                          </span>
                        </div>
                        <div className="flex items-center gap-6 text-sm text-gray-600">
                          <div className="flex items-center gap-2">
                            <Users className="w-4 h-4" />
                            <span>{session.learners} / {session.maxSeats} learners</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <Clock className="w-4 h-4" />
                            <span>Started {getTimeSince(session.createdAt)}</span>
                          </div>
                        </div>
                        <div className="mt-3 p-3 bg-gray-50 rounded-lg inline-block">
                          <p className="text-xs text-gray-600 mb-1">Join Code</p>
                          <p className="text-2xl tracking-wider text-gray-900">{session.code}</p>
                        </div>
                      </div>
                      <Button onClick={() => onViewClass(session.id)}>
                        View Class
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : (
            <Card>
              <CardContent className="p-12 text-center">
                <p className="text-gray-500">No active classes. Create one to get started!</p>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
