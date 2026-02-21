import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { 
  Activity, Clock, Users, BarChart3, Search, Plus, Settings,
  Database, Shield, Cpu, HardDrive, Lock, UserX, AlertTriangle,
  LogIn, FileSearch, FilePlus, FileEdit, Trash2, GitCompare,
  RefreshCw
} from 'lucide-react';
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

function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatNumber(num) {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
  return num.toString();
}

function StatCard({ icon: Icon, label, value, color, subValue }) {
  return (
    <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-4">
      <div className="flex items-center gap-3 mb-2">
        <Icon className={`w-5 h-5 ${color}`} />
        <span className="text-sm text-zinc-400">{label}</span>
      </div>
      <p className={`text-2xl font-semibold ${color}`}>{value}</p>
      {subValue && <p className="text-xs text-zinc-500 mt-1">{subValue}</p>}
    </div>
  );
}

function OperationCard({ icon: Icon, label, count, color }) {
  return (
    <div className="flex items-center gap-3 p-3 bg-zinc-700/50 rounded-lg">
      <Icon className={`w-4 h-4 ${color}`} />
      <span className="text-sm text-zinc-300 flex-1">{label}</span>
      <span className="text-sm font-medium text-zinc-100">{formatNumber(count)}</span>
    </div>
  );
}

function ActivityItem({ activity }) {
  const getIcon = (type) => {
    switch (type?.toLowerCase()) {
      case 'info': return <Activity className="w-4 h-4 text-blue-400" />;
      case 'warn': return <AlertTriangle className="w-4 h-4 text-yellow-400" />;
      case 'error': return <AlertTriangle className="w-4 h-4 text-red-400" />;
      default: return <Activity className="w-4 h-4 text-zinc-400" />;
    }
  };

  const formatTime = (timestamp) => {
    if (!timestamp) return '';
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now - date;
    
    if (diff < 60000) return 'just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    return date.toLocaleDateString();
  };

  return (
    <div className="flex items-start gap-3 p-3 hover:bg-zinc-700/30 rounded-lg transition-colors">
      {getIcon(activity.type)}
      <div className="flex-1 min-w-0">
        <p className="text-sm text-zinc-200 truncate">{activity.message}</p>
        <div className="flex items-center gap-2 mt-1">
          {activity.user && (
            <span className="text-xs text-zinc-500">{activity.user}</span>
          )}
          {activity.source && (
            <span className="text-xs text-zinc-600">({activity.source})</span>
          )}
        </div>
      </div>
      <span className="text-xs text-zinc-500 whitespace-nowrap">
        {formatTime(activity.timestamp)}
      </span>
    </div>
  );
}

export default function Dashboard() {
  const [stats, setStats] = useState(null);
  const [activities, setActivities] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [refreshing, setRefreshing] = useState(false);

  const fetchData = async (showRefresh = false) => {
    try {
      if (showRefresh) setRefreshing(true);
      
      const [statsData, activitiesData] = await Promise.all([
        api.getStats(),
        api.getActivities(10)
      ]);
      
      setStats(statsData);
      setActivities(activitiesData.activities || []);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(() => fetchData(), 5000);
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
        Failed to load dashboard data: {error}
      </div>
    );
  }

  const quickActions = [
    { label: 'Search Entries', to: '/search', icon: Search },
    { label: 'Add Entry', to: '/entries/new', icon: Plus },
    { label: 'View Config', to: '/config', icon: Settings },
  ];

  return (
    <div>
      <Header 
        title="Dashboard" 
        actions={
          <button
            onClick={() => fetchData(true)}
            disabled={refreshing}
            className="flex items-center gap-2 px-3 py-1.5 text-sm bg-zinc-700 hover:bg-zinc-600 rounded-lg disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        }
      />

      {/* Server Status */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <StatCard 
          icon={Activity} 
          label="Status" 
          value={stats?.status?.toUpperCase() || 'N/A'} 
          color={stats?.status === 'ok' ? 'text-green-500' : 'text-red-500'} 
        />
        <StatCard 
          icon={Clock} 
          label="Uptime" 
          value={formatUptime(stats?.uptime)} 
          color="text-blue-500"
          subValue={stats?.startTime ? `Started: ${new Date(stats.startTime).toLocaleString()}` : null}
        />
        <StatCard 
          icon={Users} 
          label="Connections" 
          value={stats?.connections?.toString() || '0'} 
          color="text-yellow-500" 
        />
        <StatCard 
          icon={BarChart3} 
          label="Total Requests" 
          value={formatNumber(stats?.requests || 0)} 
          color="text-purple-500" 
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-6">
        {/* Storage Stats */}
        <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-5">
          <div className="flex items-center gap-2 mb-4">
            <Database className="w-5 h-5 text-cyan-500" />
            <h2 className="text-lg font-medium text-zinc-100">Storage</h2>
          </div>
          <div className="space-y-3">
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">Entries</span>
              <span className="text-sm font-medium text-zinc-100">{formatNumber(stats?.storage?.entryCount || 0)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">Indexes</span>
              <span className="text-sm font-medium text-zinc-100">{stats?.storage?.indexCount || 0}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">Pages (Used/Total)</span>
              <span className="text-sm font-medium text-zinc-100">
                {formatNumber(stats?.storage?.usedPages || 0)} / {formatNumber(stats?.storage?.totalPages || 0)}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">Buffer Pool</span>
              <span className="text-sm font-medium text-zinc-100">{stats?.storage?.bufferPoolSize || 0} pages</span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">Dirty Pages</span>
              <span className="text-sm font-medium text-zinc-100">{stats?.storage?.dirtyPages || 0}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">Active Transactions</span>
              <span className="text-sm font-medium text-zinc-100">{stats?.storage?.activeTransactions || 0}</span>
            </div>
          </div>
        </div>

        {/* Security Stats */}
        <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-5">
          <div className="flex items-center gap-2 mb-4">
            <Shield className="w-5 h-5 text-orange-500" />
            <h2 className="text-lg font-medium text-zinc-100">Security</h2>
          </div>
          <div className="space-y-4">
            <div className="flex items-center justify-between p-3 bg-zinc-700/50 rounded-lg">
              <div className="flex items-center gap-2">
                <Lock className="w-4 h-4 text-red-400" />
                <span className="text-sm text-zinc-300">Locked Accounts</span>
              </div>
              <span className={`text-lg font-semibold ${stats?.security?.lockedAccounts > 0 ? 'text-red-400' : 'text-green-400'}`}>
                {stats?.security?.lockedAccounts || 0}
              </span>
            </div>
            <div className="flex items-center justify-between p-3 bg-zinc-700/50 rounded-lg">
              <div className="flex items-center gap-2">
                <UserX className="w-4 h-4 text-yellow-400" />
                <span className="text-sm text-zinc-300">Disabled Accounts</span>
              </div>
              <span className="text-lg font-semibold text-zinc-100">
                {stats?.security?.disabledAccounts || 0}
              </span>
            </div>
            <div className="flex items-center justify-between p-3 bg-zinc-700/50 rounded-lg">
              <div className="flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 text-orange-400" />
                <span className="text-sm text-zinc-300">Failed Logins (24h)</span>
              </div>
              <span className={`text-lg font-semibold ${stats?.security?.failedLogins24h > 10 ? 'text-orange-400' : 'text-zinc-100'}`}>
                {stats?.security?.failedLogins24h || 0}
              </span>
            </div>
          </div>
        </div>

        {/* System Stats */}
        <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-5">
          <div className="flex items-center gap-2 mb-4">
            <Cpu className="w-5 h-5 text-pink-500" />
            <h2 className="text-lg font-medium text-zinc-100">System</h2>
          </div>
          <div className="space-y-3">
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">Version</span>
              <span className="text-sm font-medium text-zinc-100">{stats?.version || 'N/A'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">Goroutines</span>
              <span className="text-sm font-medium text-zinc-100">{stats?.system?.goRoutines || 0}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">Memory (Alloc)</span>
              <span className="text-sm font-medium text-zinc-100">{formatBytes(stats?.system?.memoryAlloc || 0)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">Memory (Sys)</span>
              <span className="text-sm font-medium text-zinc-100">{formatBytes(stats?.system?.memorySys || 0)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">GC Cycles</span>
              <span className="text-sm font-medium text-zinc-100">{stats?.system?.numGC || 0}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-zinc-400">CPUs</span>
              <span className="text-sm font-medium text-zinc-100">{stats?.system?.numCPU || 0}</span>
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        {/* LDAP Operations */}
        <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-5">
          <div className="flex items-center gap-2 mb-4">
            <BarChart3 className="w-5 h-5 text-indigo-500" />
            <h2 className="text-lg font-medium text-zinc-100">LDAP Operations</h2>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <OperationCard icon={LogIn} label="Binds" count={stats?.operations?.binds || 0} color="text-green-400" />
            <OperationCard icon={FileSearch} label="Searches" count={stats?.operations?.searches || 0} color="text-blue-400" />
            <OperationCard icon={FilePlus} label="Adds" count={stats?.operations?.adds || 0} color="text-cyan-400" />
            <OperationCard icon={FileEdit} label="Modifies" count={stats?.operations?.modifies || 0} color="text-yellow-400" />
            <OperationCard icon={Trash2} label="Deletes" count={stats?.operations?.deletes || 0} color="text-red-400" />
            <OperationCard icon={GitCompare} label="Compares" count={stats?.operations?.compares || 0} color="text-purple-400" />
          </div>
        </div>

        {/* Recent Activity */}
        <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <Activity className="w-5 h-5 text-emerald-500" />
              <h2 className="text-lg font-medium text-zinc-100">Recent Activity</h2>
            </div>
            <Link to="/logs" className="text-sm text-zinc-400 hover:text-zinc-200">
              View all
            </Link>
          </div>
          <div className="space-y-1 max-h-64 overflow-y-auto">
            {activities.length > 0 ? (
              activities.map((activity, i) => (
                <ActivityItem key={i} activity={activity} />
              ))
            ) : (
              <p className="text-sm text-zinc-500 text-center py-4">No recent activity</p>
            )}
          </div>
        </div>
      </div>

      {/* Quick Actions */}
      <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-5">
        <h2 className="text-lg font-medium text-zinc-100 mb-4">Quick Actions</h2>
        <div className="flex flex-wrap gap-3">
          {quickActions.map((action, i) => (
            <Link
              key={i}
              to={action.to}
              className="flex items-center gap-2 px-4 py-2 bg-zinc-700 hover:bg-zinc-600 rounded-lg text-sm text-zinc-100 transition-colors"
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
