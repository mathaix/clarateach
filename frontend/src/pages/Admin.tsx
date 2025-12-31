import { useState, useEffect } from 'react';
import { Server, Users, Clock, Copy, Download, ExternalLink, RefreshCw, AlertCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { api, type AdminWorkshopView, type VMWithWorkshop } from '@/lib/api';

export function Admin() {
  const [workshops, setWorkshops] = useState<AdminWorkshopView[]>([]);
  const [vms, setVMs] = useState<VMWithWorkshop[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    setLoading(true);
    setError(null);
    try {
      const [overviewData, vmsData] = await Promise.all([
        api.adminOverview(),
        api.listVMs(),
      ]);
      setWorkshops(overviewData.workshops || []);
      setVMs(vmsData.vms || []);
    } catch (err) {
      console.error('Failed to load admin data:', err);
      setError(err instanceof Error ? err.message : 'Failed to load data');
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = async (text: string, id: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  const getTimeSince = (dateStr: string) => {
    const date = new Date(dateStr);
    const minutes = Math.floor((Date.now() - date.getTime()) / 1000 / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ${minutes % 60}m ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
  };

  const getStatusBadge = (status: string) => {
    const styles: Record<string, string> = {
      provisioning: 'bg-yellow-100 text-yellow-800',
      running: 'bg-green-100 text-green-800',
      stopping: 'bg-orange-100 text-orange-800',
      terminated: 'bg-gray-100 text-gray-800',
      error: 'bg-red-100 text-red-800',
    };
    return styles[status] || 'bg-gray-100 text-gray-800';
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <p className="text-gray-500">Loading admin data...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <Card className="w-full max-w-md">
          <CardContent className="p-6 text-center">
            <AlertCircle className="w-12 h-12 text-red-500 mx-auto mb-4" />
            <p className="text-red-600 mb-4">{error}</p>
            <Button onClick={loadData}>Retry</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b">
        <div className="max-w-7xl mx-auto px-4 py-6">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h1 className="text-3xl text-gray-900">Admin Portal</h1>
              <p className="text-gray-600 mt-1">VM Management & Workshop Overview</p>
            </div>
            <Button variant="outline" onClick={loadData}>
              <RefreshCw className="w-4 h-4 mr-2" />
              Refresh
            </Button>
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 py-8 space-y-8">
        {/* Summary Stats */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <Card>
            <CardContent className="p-6">
              <div className="flex items-center gap-4">
                <div className="p-3 bg-blue-100 rounded-lg">
                  <Server className="w-6 h-6 text-blue-600" />
                </div>
                <div>
                  <p className="text-2xl font-semibold">{vms.length}</p>
                  <p className="text-sm text-gray-600">Active VMs</p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-6">
              <div className="flex items-center gap-4">
                <div className="p-3 bg-green-100 rounded-lg">
                  <Users className="w-6 h-6 text-green-600" />
                </div>
                <div>
                  <p className="text-2xl font-semibold">
                    {vms.reduce((sum, vm) => sum + vm.active_students, 0)}
                  </p>
                  <p className="text-sm text-gray-600">Active Students</p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-6">
              <div className="flex items-center gap-4">
                <div className="p-3 bg-purple-100 rounded-lg">
                  <Users className="w-6 h-6 text-purple-600" />
                </div>
                <div>
                  <p className="text-2xl font-semibold">
                    {vms.reduce((sum, vm) => sum + vm.total_seats, 0)}
                  </p>
                  <p className="text-sm text-gray-600">Total Seats</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* VMs List */}
        <div>
          <h2 className="text-xl mb-4 text-gray-900">Virtual Machines</h2>
          {vms.length > 0 ? (
            <div className="grid gap-4">
              {vms.map((vm) => (
                <Card key={vm.id} className="hover:shadow-md transition-shadow">
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <CardTitle className="text-lg">{vm.workshop_name}</CardTitle>
                        <span className={`text-xs px-2 py-1 rounded-full ${getStatusBadge(vm.status)}`}>
                          {vm.status}
                        </span>
                      </div>
                      <div className="flex items-center gap-2 text-sm text-gray-500">
                        <Clock className="w-4 h-4" />
                        <span>Created {getTimeSince(vm.created_at)}</span>
                      </div>
                    </div>
                    <CardDescription>
                      {vm.vm_name} ({vm.zone}) - {vm.machine_type}
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    {/* Stats Row */}
                    <div className="flex flex-wrap gap-6">
                      <div className="flex items-center gap-2">
                        <Users className="w-4 h-4 text-gray-500" />
                        <span className="text-sm">
                          <span className="font-semibold">{vm.active_students}</span>
                          <span className="text-gray-500"> / {vm.total_seats} students</span>
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <ExternalLink className="w-4 h-4 text-gray-500" />
                        <span className="text-sm font-mono">{vm.external_ip || 'Pending...'}</span>
                      </div>
                    </div>

                    {/* SSH Access */}
                    {vm.external_ip && (
                      <div className="bg-gray-50 rounded-lg p-4 space-y-3">
                        <h4 className="text-sm font-medium text-gray-700">SSH Access</h4>

                        {/* SSH Command */}
                        <div>
                          <label className="text-xs text-gray-500 block mb-1">SSH Command (after downloading key)</label>
                          <div className="flex items-center gap-2">
                            <code className="flex-1 bg-gray-900 text-green-400 px-3 py-2 rounded text-sm font-mono overflow-x-auto">
                              {vm.ssh_command}
                            </code>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => copyToClipboard(vm.ssh_command, `ssh-${vm.id}`)}
                            >
                              <Copy className="w-4 h-4" />
                              {copiedId === `ssh-${vm.id}` ? 'Copied!' : ''}
                            </Button>
                          </div>
                        </div>

                        {/* GCloud Command */}
                        <div>
                          <label className="text-xs text-gray-500 block mb-1">GCloud SSH (alternative)</label>
                          <div className="flex items-center gap-2">
                            <code className="flex-1 bg-gray-900 text-green-400 px-3 py-2 rounded text-sm font-mono overflow-x-auto">
                              {vm.gcloud_ssh}
                            </code>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => copyToClipboard(vm.gcloud_ssh, `gcloud-${vm.id}`)}
                            >
                              <Copy className="w-4 h-4" />
                              {copiedId === `gcloud-${vm.id}` ? 'Copied!' : ''}
                            </Button>
                          </div>
                        </div>

                        {/* Download Key Button */}
                        <div>
                          <a
                            href={api.getSSHKeyDownloadUrl(vm.workshop_id)}
                            download={`${vm.vm_name}.pem`}
                            className="inline-flex items-center gap-2 text-sm text-blue-600 hover:text-blue-800"
                          >
                            <Download className="w-4 h-4" />
                            Download SSH Private Key
                          </a>
                          <p className="text-xs text-gray-500 mt-1">
                            Save as ~/.ssh/{vm.vm_name}.pem and run: chmod 600 ~/.ssh/{vm.vm_name}.pem
                          </p>
                        </div>
                      </div>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : (
            <Card>
              <CardContent className="p-12 text-center">
                <Server className="w-12 h-12 text-gray-300 mx-auto mb-4" />
                <p className="text-gray-500">No VMs provisioned yet.</p>
                <p className="text-sm text-gray-400 mt-2">Start a workshop to provision a VM.</p>
              </CardContent>
            </Card>
          )}
        </div>

        {/* All Workshops Overview */}
        <div>
          <h2 className="text-xl mb-4 text-gray-900">All Workshops</h2>
          {workshops.length > 0 ? (
            <div className="grid gap-4">
              {workshops.map((item) => (
                <Card key={item.workshop.id}>
                  <CardContent className="p-4">
                    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                      <div>
                        <div className="flex items-center gap-2">
                          <h3 className="font-medium">{item.workshop.name}</h3>
                          <span className={`text-xs px-2 py-1 rounded-full ${getStatusBadge(item.workshop.status)}`}>
                            {item.workshop.status}
                          </span>
                        </div>
                        <p className="text-sm text-gray-500 mt-1">
                          Code: <span className="font-mono">{item.workshop.code}</span>
                        </p>
                      </div>
                      <div className="flex items-center gap-4 text-sm">
                        <div className="flex items-center gap-2">
                          <Users className="w-4 h-4 text-gray-500" />
                          <span>{item.active_students} / {item.total_seats}</span>
                        </div>
                        {item.vm && (
                          <span className="text-gray-500">
                            VM: {item.vm.external_ip || 'pending'}
                          </span>
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
                <p className="text-gray-500">No workshops created yet.</p>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
