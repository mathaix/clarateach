import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, PlayCircle, Clock, Users, Trash2, Eye, Loader2, Cpu, Container } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Layout } from '@/components/Layout';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import type { Workshop } from '@/lib/types';

export function Dashboard() {
  const navigate = useNavigate();
  const { isAuthenticated, loading: authLoading } = useAuth();
  const [workshops, setWorkshops] = useState<Workshop[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [creating, setCreating] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    seats: '10',
    api_key: '',
    runtime_type: 'docker' as 'docker' | 'firecracker',
  });

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      navigate('/login');
      return;
    }
    if (!authLoading && isAuthenticated) {
      loadWorkshops();
      // Poll for updates every 5 seconds to show status changes
      const interval = setInterval(loadWorkshops, 5000);
      return () => clearInterval(interval);
    }
  }, [authLoading, isAuthenticated, navigate]);

  const loadWorkshops = async () => {
    try {
      const data = await api.listWorkshops();
      setWorkshops(data.workshops || []);
    } catch (err) {
      console.error('Failed to load workshops:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateWorkshop = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);

    try {
      await api.createWorkshop({
        name: formData.name,
        seats: parseInt(formData.seats),
        api_key: formData.api_key,
        runtime_type: formData.runtime_type,
      });
      // Close form and reset state immediately
      setShowCreateForm(false);
      setFormData({ name: '', seats: '10', api_key: '', runtime_type: 'docker' });
      setCreating(false);
      // Then refresh the list
      await loadWorkshops();
    } catch (err) {
      console.error('Failed to create workshop:', err);
      alert(err instanceof Error ? err.message : 'Failed to create workshop');
      setCreating(false);
    }
  };

  const handleDeleteWorkshop = async (id: string) => {
    if (!confirm('Are you sure you want to delete this workshop?')) return;

    try {
      await api.deleteWorkshop(id);
      loadWorkshops();
    } catch (err) {
      console.error('Failed to delete workshop:', err);
      alert(err instanceof Error ? err.message : 'Failed to delete workshop');
    }
  };

  const handleStartWorkshop = async (id: string) => {
    try {
      await api.startWorkshop(id);
      loadWorkshops();
    } catch (err) {
      console.error('Failed to start workshop:', err);
      alert(err instanceof Error ? err.message : 'Failed to start workshop');
    }
  };

  const getTimeSince = (dateStr: string) => {
    const date = new Date(dateStr);
    const minutes = Math.floor((Date.now() - date.getTime()) / 1000 / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ${minutes % 60}m ago`;
  };

  const getStatusBadge = (status: string) => {
    const styles: Record<string, string> = {
      created: 'bg-gray-100 text-gray-800',
      provisioning: 'bg-yellow-100 text-yellow-800',
      running: 'bg-green-100 text-green-800',
      stopping: 'bg-orange-100 text-orange-800',
      stopped: 'bg-gray-100 text-gray-800',
      deleting: 'bg-red-100 text-red-800',
      deleted: 'bg-gray-200 text-gray-500',
      error: 'bg-red-100 text-red-800',
    };
    return styles[status] || styles.created;
  };

  if (authLoading || loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center py-12">
          <Loader2 className="w-8 h-8 animate-spin text-indigo-600" />
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      {/* Page Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Your Workshops</h1>
          <p className="text-gray-600 mt-1">Create and manage your workshop sessions</p>
        </div>
        <Button className="w-full sm:w-auto" onClick={() => setShowCreateForm(!showCreateForm)}>
          <Plus className="w-4 h-4 mr-2" />
          Create Workshop
        </Button>
      </div>

      <div>
        {/* Create Form */}
        {showCreateForm && (
          <Card className="mb-8">
            <CardHeader>
              <CardTitle>Create New Workshop</CardTitle>
              <CardDescription>Set up a new workshop session for your learners</CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleCreateWorkshop} className="space-y-4">
                <div>
                  <Label htmlFor="name">Workshop Name</Label>
                  <Input
                    id="name"
                    placeholder="e.g., Python Basics Workshop"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    required
                  />
                </div>
                <div>
                  <Label htmlFor="seats">Number of Seats</Label>
                  <Input
                    id="seats"
                    type="number"
                    min="1"
                    max="10"
                    value={formData.seats}
                    onChange={(e) => setFormData({ ...formData, seats: e.target.value })}
                    required
                  />
                  <p className="text-sm text-gray-500 mt-1">Each learner gets their own workspace</p>
                </div>
                <div>
                  <Label htmlFor="api_key">Anthropic API Key</Label>
                  <Input
                    id="api_key"
                    type="password"
                    placeholder="sk-ant-..."
                    value={formData.api_key}
                    onChange={(e) => setFormData({ ...formData, api_key: e.target.value })}
                    required
                  />
                  <p className="text-sm text-gray-500 mt-1">Used for Claude Code in learner workspaces</p>
                </div>
                <div>
                  <Label htmlFor="runtime_type">Runtime Type</Label>
                  <select
                    id="runtime_type"
                    value={formData.runtime_type}
                    onChange={(e) => setFormData({ ...formData, runtime_type: e.target.value as 'docker' | 'firecracker' })}
                    className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <option value="docker">Docker (Standard)</option>
                    <option value="firecracker">Firecracker (MicroVMs)</option>
                  </select>
                  <p className="text-sm text-gray-500 mt-1">Choose the container runtime for workspaces</p>
                </div>
                <div className="flex flex-col gap-2 sm:flex-row">
                  <Button className="w-full sm:w-auto" type="submit" disabled={creating}>
                    {creating ? 'Creating...' : 'Create Workshop'}
                  </Button>
                  <Button className="w-full sm:w-auto" type="button" variant="outline" onClick={() => setShowCreateForm(false)}>
                    Cancel
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        )}

        {/* Workshops List */}
        <div className="mt-8">
          {workshops.length > 0 ? (
            <div className="grid gap-4">
              {workshops.map((workshop) => (
                <Card key={workshop.id} className="hover:shadow-md transition-shadow">
                  <CardContent className="p-6">
                    <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                      <div className="flex-1">
                        <div className="flex items-center gap-3 mb-2">
                          <h3 className="text-lg text-gray-900">{workshop.name}</h3>
                          <span className={`text-xs px-2 py-1 rounded-full ${getStatusBadge(workshop.status)}`}>
                            {workshop.status}
                          </span>
                          <span className={`text-xs px-2 py-1 rounded-full flex items-center gap-1 ${
                            workshop.runtime_type === 'firecracker'
                              ? 'bg-purple-100 text-purple-800'
                              : 'bg-blue-100 text-blue-800'
                          }`}>
                            {workshop.runtime_type === 'firecracker' ? (
                              <><Cpu className="w-3 h-3" /> MicroVM</>
                            ) : (
                              <><Container className="w-3 h-3" /> Docker</>
                            )}
                          </span>
                        </div>
                        <div className="flex flex-wrap items-center gap-4 text-sm text-gray-600">
                          <div className="flex items-center gap-2">
                            <Users className="w-4 h-4" />
                            <span>{workshop.seats} seats</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <Clock className="w-4 h-4" />
                            <span>Created {getTimeSince(workshop.created_at)}</span>
                          </div>
                        </div>
                        <div className="mt-3 p-3 bg-gray-50 rounded-lg inline-block">
                          <p className="text-xs text-gray-600 mb-1">Join Code</p>
                          <p className="text-2xl tracking-wider text-gray-900 font-mono">{workshop.code}</p>
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-2 w-full sm:w-auto">
                        {workshop.status === 'created' && (
                          <Button className="w-full sm:w-auto" onClick={() => handleStartWorkshop(workshop.id)}>
                            <PlayCircle className="w-4 h-4 mr-2" />
                            Start
                          </Button>
                        )}
                        {['running', 'provisioning', 'stopped', 'stopping', 'deleted', 'deleting'].includes(workshop.status) && (
                          <Button className="w-full sm:w-auto" variant={workshop.status === 'deleted' ? 'outline' : 'default'} onClick={() => navigate(`/workshop/${workshop.id}`)}>
                            <Eye className="w-4 h-4 mr-2" />
                            View
                          </Button>
                        )}
                        {!['deleting', 'deleted'].includes(workshop.status) && (
                          <Button
                            variant="outline"
                            size="icon"
                            className="self-start sm:self-auto"
                            onClick={() => handleDeleteWorkshop(workshop.id)}
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        )}
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : (
            <Card>
              <CardContent className="p-12 text-center">
                <p className="text-gray-500">No workshops yet. Create one to get started!</p>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </Layout>
  );
}
