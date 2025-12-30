import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Users, Copy, Check, ArrowLeft, StopCircle, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { api } from '@/lib/api';
import type { Workshop, Session } from '@/lib/types';

export function WorkshopView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [workshop, setWorkshop] = useState<Workshop | null>(null);
  const [learners, setLearners] = useState<Session[]>([]);
  const [connectedCount, setConnectedCount] = useState(0);
  const [copied, setCopied] = useState(false);
  const [loading, setLoading] = useState(true);
  const [stopping, setStopping] = useState(false);

  useEffect(() => {
    if (id) {
      loadWorkshop();
      const interval = setInterval(loadLearners, 5000);
      return () => clearInterval(interval);
    }
  }, [id]);

  const loadWorkshop = async () => {
    try {
      const data = await api.getWorkshop(id!);
      setWorkshop(data.workshop);
      loadLearners();
    } catch (err) {
      console.error('Failed to load workshop:', err);
      navigate('/');
    } finally {
      setLoading(false);
    }
  };

  const loadLearners = async () => {
    try {
      const data = await api.getWorkshopLearners(id!);
      setLearners(data.learners);
      setConnectedCount(typeof data.connected === 'number' ? data.connected : data.learners.length);
    } catch (err) {
      console.error('Failed to load learners:', err);
    }
  };

  const handleCopyCode = () => {
    if (workshop) {
      navigator.clipboard.writeText(workshop.code);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleStopWorkshop = async () => {
    if (!confirm('Are you sure you want to stop this workshop? All learners will be disconnected.')) return;

    setStopping(true);
    try {
      await api.stopWorkshop(id!);
      navigate('/');
    } catch (err) {
      console.error('Failed to stop workshop:', err);
      alert(err instanceof Error ? err.message : 'Failed to stop workshop');
      setStopping(false);
    }
  };

  const getTimeSince = (dateStr: string) => {
    const date = new Date(dateStr);
    const minutes = Math.floor((Date.now() - date.getTime()) / 1000 / 60);
    if (minutes < 1) return 'just now';
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
      error: 'bg-red-100 text-red-800',
    };
    return styles[status] || styles.created;
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <p className="text-gray-500">Loading...</p>
      </div>
    );
  }

  if (!workshop) {
    return null;
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b">
        <div className="max-w-7xl mx-auto px-4 py-6">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-4">
              <Button className="w-full sm:w-auto" variant="ghost" onClick={() => navigate('/')}>
                <ArrowLeft className="w-4 h-4 mr-2" />
                Dashboard
              </Button>
              <div>
                <h1 className="text-3xl text-gray-900">{workshop.name}</h1>
                <p className="text-gray-600 mt-1">Managing workshop session</p>
              </div>
            </div>
            <Button
              variant="destructive"
              onClick={handleStopWorkshop}
              disabled={stopping || workshop.status !== 'running'}
              className="w-full sm:w-auto"
            >
              <StopCircle className="w-4 h-4 mr-2" />
              {stopping ? 'Stopping...' : 'End Workshop'}
            </Button>
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 py-8">
        <div className="grid lg:grid-cols-3 gap-6">
          {/* Workshop Info */}
          <div className="lg:col-span-1 space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>Join Information</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div>
                    <p className="text-sm text-gray-600 mb-2">Workshop Code</p>
                    <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                      <div className="flex-1 p-4 bg-indigo-50 rounded-lg text-center w-full">
                        <p className="text-3xl tracking-wider text-indigo-900 font-mono">{workshop.code}</p>
                      </div>
                      <Button className="self-end sm:self-auto" variant="outline" size="icon" onClick={handleCopyCode}>
                        {copied ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4" />}
                      </Button>
                    </div>
                  </div>

                  <div className="pt-4 border-t">
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-sm text-gray-600">Connected Learners</span>
                      <span className="text-2xl text-gray-900">{connectedCount}</span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-gray-600">Total Capacity</span>
                      <span className="text-lg text-gray-700">{workshop.seats}</span>
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
                    <span className="text-sm text-gray-600">Status</span>
                    <span className={`text-xs px-2 py-1 rounded-full ${getStatusBadge(workshop.status)}`}>
                      {workshop.status}
                    </span>
                  </div>
                  {workshop.vm_ip && (
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-gray-600">VM IP</span>
                      <span className="text-sm font-mono text-gray-700">{workshop.vm_ip}</span>
                    </div>
                  )}
                  {workshop.endpoint && (
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-gray-600">Endpoint</span>
                      <span className="text-sm font-mono text-gray-700 break-all sm:truncate max-w-full sm:max-w-[180px]">
                        {workshop.endpoint}
                      </span>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Learners List */}
          <div className="lg:col-span-2">
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Users className="w-5 h-5" />
                    <CardTitle>Joined Learners</CardTitle>
                  </div>
                  <Button variant="ghost" size="sm" onClick={loadLearners}>
                    <RefreshCw className="w-4 h-4" />
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                {learners.length > 0 ? (
                  <div className="space-y-2">
                    {learners.map((learner) => (
                      <div
                        key={learner.odehash}
                        className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between p-4 bg-gray-50 rounded-lg"
                      >
                        <div className="flex items-center gap-3">
                          <div className="w-8 h-8 rounded-full bg-indigo-100 flex items-center justify-center text-indigo-700 font-medium">
                            {learner.seat}
                          </div>
                          <div>
                            <p className="text-gray-900">{learner.name || `Learner ${learner.seat}`}</p>
                            <p className="text-sm text-gray-500">
                              Joined {getTimeSince(learner.joined_at)}
                            </p>
                          </div>
                        </div>
                        <span className="text-xs font-mono text-gray-400">{learner.odehash}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="text-center py-8">
                    <p className="text-gray-500">No learners have joined yet</p>
                    <p className="text-sm text-gray-400 mt-1">
                      Share the workshop code with your learners
                    </p>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
}
