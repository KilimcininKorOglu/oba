import { useState, useEffect } from 'react';
import { RefreshCw, Download, Trash2, Search } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import Table from '../components/Table';
import LoadingSpinner from '../components/LoadingSpinner';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../context/ToastContext';

export default function Logs() {
  const { showToast } = useToast();
  const [logs, setLogs] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);
  const [showClear, setShowClear] = useState(false);
  const [filters, setFilters] = useState({
    level: '',
    search: '',
    limit: '100'
  });
  const [pagination, setPagination] = useState({ offset: 0, total: 0, hasMore: false });

  const fetchLogs = async (offset = 0) => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (filters.level) params.set('level', filters.level);
      if (filters.search) params.set('search', filters.search);
      params.set('limit', filters.limit);
      params.set('offset', offset.toString());

      const response = await fetch(`/api/v1/logs?${params}`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
      });

      if (!response.ok) {
        if (response.status === 503) {
          showToast('Log storage is not enabled', 'warning');
          setLogs([]);
          return;
        }
        throw new Error('Failed to fetch logs');
      }

      const data = await response.json();
      setLogs(data.entries || []);
      setPagination({
        offset: data.offset || 0,
        total: data.total_count || 0,
        hasMore: data.has_more || false
      });
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  const fetchStats = async () => {
    try {
      const response = await fetch('/api/v1/logs/stats', {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
      });
      if (response.ok) {
        const data = await response.json();
        setStats(data);
      }
    } catch {
      // ignore
    }
  };

  useEffect(() => {
    fetchLogs();
    fetchStats();
  }, []);

  const handleSearch = (e) => {
    e.preventDefault();
    fetchLogs(0);
  };

  const handleClear = async () => {
    try {
      const response = await fetch('/api/v1/logs', {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
      });
      if (response.ok) {
        showToast('Logs cleared', 'success');
        fetchLogs();
        fetchStats();
      }
    } catch (err) {
      showToast(err.message, 'error');
    }
    setShowClear(false);
  };

  const handleExport = async (format) => {
    try {
      const params = new URLSearchParams();
      if (filters.level) params.set('level', filters.level);
      if (filters.search) params.set('search', filters.search);
      params.set('format', format);

      const response = await fetch(`/api/v1/logs/export?${params}`, {
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
      });

      if (!response.ok) throw new Error('Export failed');

      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `logs.${format}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const levelColors = {
    debug: 'text-zinc-400',
    info: 'text-blue-400',
    warn: 'text-yellow-400',
    error: 'text-red-400'
  };

  const columns = [
    {
      header: 'Time',
      render: (row) => (
        <span className="text-zinc-400 text-xs">
          {new Date(row.timestamp).toLocaleString()}
        </span>
      )
    },
    {
      header: 'Level',
      render: (row) => (
        <span className={`text-xs font-medium uppercase ${levelColors[row.level] || 'text-zinc-400'}`}>
          {row.level}
        </span>
      )
    },
    {
      header: 'Message',
      render: (row) => (
        <span className="text-sm text-zinc-100">{row.message}</span>
      )
    },
    {
      header: 'Request ID',
      render: (row) => (
        <span className="text-xs text-zinc-500 font-mono">{row.request_id || '-'}</span>
      )
    }
  ];

  return (
    <div>
      <Header
        title="Logs"
        actions={
          <div className="flex items-center gap-2">
            <button
              onClick={() => fetchLogs(pagination.offset)}
              className="flex items-center gap-2 px-4 py-2 bg-zinc-700 hover:bg-zinc-600 text-white rounded-lg text-sm"
            >
              <RefreshCw className="w-4 h-4" />
              Refresh
            </button>
            <button
              onClick={() => handleExport('json')}
              className="flex items-center gap-2 px-4 py-2 bg-zinc-700 hover:bg-zinc-600 text-white rounded-lg text-sm"
            >
              <Download className="w-4 h-4" />
              Export
            </button>
            <button
              onClick={() => setShowClear(true)}
              className="flex items-center gap-2 px-4 py-2 bg-red-500 hover:bg-red-600 text-white rounded-lg text-sm"
            >
              <Trash2 className="w-4 h-4" />
              Clear
            </button>
          </div>
        }
      />

      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-4">
            <p className="text-sm text-zinc-400">Total Entries</p>
            <p className="text-2xl font-semibold text-zinc-100">{stats.total_entries}</p>
          </div>
          <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-4">
            <p className="text-sm text-zinc-400">Errors</p>
            <p className="text-2xl font-semibold text-red-500">{stats.by_level?.error || 0}</p>
          </div>
          <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-4">
            <p className="text-sm text-zinc-400">Warnings</p>
            <p className="text-2xl font-semibold text-yellow-500">{stats.by_level?.warn || 0}</p>
          </div>
          <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-4">
            <p className="text-sm text-zinc-400">Info</p>
            <p className="text-2xl font-semibold text-blue-500">{stats.by_level?.info || 0}</p>
          </div>
        </div>
      )}

      <form onSubmit={handleSearch} className="bg-zinc-800 rounded-lg border border-zinc-700 p-4 mb-6">
        <div className="flex flex-wrap gap-4">
          <div className="flex-1 min-w-48">
            <input
              type="text"
              value={filters.search}
              onChange={(e) => setFilters({ ...filters, search: e.target.value })}
              placeholder="Search messages..."
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
          <select
            value={filters.level}
            onChange={(e) => setFilters({ ...filters, level: e.target.value })}
            className="px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 focus:outline-none focus:border-blue-500"
          >
            <option value="">All Levels</option>
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warning</option>
            <option value="error">Error</option>
          </select>
          <button
            type="submit"
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg"
          >
            <Search className="w-4 h-4" />
            Search
          </button>
        </div>
      </form>

      <div className="bg-zinc-800 rounded-lg border border-zinc-700">
        <Table
          columns={columns}
          data={logs}
          loading={loading}
          emptyTitle="No logs found"
          emptyDescription="Log storage may be disabled or no logs match your filters."
          pagination={{
            offset: pagination.offset,
            limit: parseInt(filters.limit),
            total: pagination.total,
            hasMore: pagination.hasMore
          }}
          onPageChange={(newOffset) => fetchLogs(newOffset)}
        />
      </div>

      <ConfirmDialog
        isOpen={showClear}
        onClose={() => setShowClear(false)}
        onConfirm={handleClear}
        title="Clear Logs"
        message="Are you sure you want to clear all logs? This action cannot be undone."
      />
    </div>
  );
}
