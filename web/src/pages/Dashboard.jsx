import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Activity, Clock, Users, BarChart3, Search, Plus, Settings } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import LoadingSpinner from '../components/LoadingSpinner';

function formatUptime(uptime) {
  if (!uptime) return 'N/A';
  
  const match = uptime.match(/^(?:(\d+)h)?(?:(\d+)m)?(?:[\d.]+s)?$/);
  if (!match) return uptime;
  
  const hours = parseInt(match[1] || '0');
  const minutes = parseInt(match[2] || '0');
  
  if (hours > 24) {
    const days = Math.floor(hours / 24);
    const remainingHours = hours % 24;
    return `${days}d ${remainingHours}h ${minutes}m`;
  }
  
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  
  return `${minutes}m`;
}

export default function Dashboard() {
  const [health, setHealth] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const fetchHealth = async () => {
    try {
      const data = await api.getHealth();
      setHealth(data);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchHealth();
    const interval = setInterval(fetchHealth, 30000);
    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-500/10 border border-red-500/50 rounded-lg p-4 text-red-500">
        Failed to load health data: {error}
      </div>
    );
  }

  const stats = [
    { label: 'Status', value: health?.status?.toUpperCase() || 'N/A', icon: Activity, color: health?.status === 'ok' ? 'text-green-500' : 'text-red-500' },
    { label: 'Uptime', value: formatUptime(health?.uptime), icon: Clock, color: 'text-blue-500' },
    { label: 'Connections', value: health?.connections?.toString() || '0', icon: Users, color: 'text-yellow-500' },
    { label: 'Requests', value: health?.requests?.toString() || '0', icon: BarChart3, color: 'text-purple-500' },
  ];

  const quickActions = [
    { label: 'Search Entries', to: '/search', icon: Search },
    { label: 'Add Entry', to: '/entries/new', icon: Plus },
    { label: 'View Config', to: '/config', icon: Settings },
  ];

  return (
    <div>
      <Header title="Dashboard" />

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {stats.map((stat, i) => (
          <div key={i} className="bg-zinc-800 rounded-lg border border-zinc-700 p-4">
            <div className="flex items-center gap-3 mb-2">
              <stat.icon className={`w-5 h-5 ${stat.color}`} />
              <span className="text-sm text-zinc-400">{stat.label}</span>
            </div>
            <p className={`text-2xl font-semibold ${stat.color}`}>{stat.value}</p>
          </div>
        ))}
      </div>

      <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-6 mb-8">
        <h2 className="text-lg font-medium text-zinc-100 mb-4">Server Information</h2>
        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <span className="text-zinc-400">Version:</span>
            <span className="ml-2 text-zinc-100">{health?.version || 'N/A'}</span>
          </div>
          <div>
            <span className="text-zinc-400">Start Time:</span>
            <span className="ml-2 text-zinc-100">{health?.startTime ? new Date(health.startTime).toLocaleString() : 'N/A'}</span>
          </div>
        </div>
      </div>

      <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-6">
        <h2 className="text-lg font-medium text-zinc-100 mb-4">Quick Actions</h2>
        <div className="flex flex-wrap gap-3">
          {quickActions.map((action, i) => (
            <Link
              key={i}
              to={action.to}
              className="flex items-center gap-2 px-4 py-2 bg-zinc-700 hover:bg-zinc-600 rounded-lg text-sm text-zinc-100"
            >
              <action.icon className="w-4 h-4" />
              {action.label}
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
