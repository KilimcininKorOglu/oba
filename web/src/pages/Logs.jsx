import { useState, useEffect } from 'react';
import { RefreshCw, Download, Trash2, Search } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import Table from '../components/Table';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../context/ToastContext';
import { formatDate } from '../utils/dateFormat';

export default function Logs() {
  const { showToast } = useToast();
  const [logs, setLogs] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);
  const [showClear, setShowClear] = useState(false);
  const [filters, setFilters] = useState({
    level: '',
    source: '',
    search: ''
  });
  const pageSize = 100;
  const [pagination, setPagination] = useState({ offset: 0, total: 0, hasMore: false });

  const fetchLogs = async (offset = 0) => {
    setLoading(true);
    try {
      const params = {};
      if (filters.level) params.level = filters.level;
      if (filters.source) params.source = filters.source;
      if (filters.search) params.search = filters.search;
      params.limit = pageSize.toString();
      params.offset = offset.toString();

      const data = await api.getLogs(params);
      
      if (data.disabled) {
        showToast('Log storage is not enabled', 'warning');
        setLogs([]);
        return;
      }

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
      const data = await api.getLogStats();
      setStats(data);
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
      await api.clearLogs();
      showToast('Logs cleared', 'success');
      fetchLogs();
      fetchStats();
    } catch (err) {
      showToast(err.message, 'error');
    }
    setShowClear(false);
  };

  const handleExport = async (format) => {
    try {
      const params = { format };
      if (filters.level) params.level = filters.level;
      if (filters.search) params.search = filters.search;

      const blob = await api.exportLogs(params);
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
          {formatDate(row.timestamp)}
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
      header: 'Source',
      render: (row) => (
        <span className={`text-xs px-2 py-0.5 rounded ${row.source === 'ldap' ? 'bg-purple-500/20 text-purple-400' : 'bg-green-500/20 text-green-400'}`}>
          {row.source || '-'}
        </span>
      )
    },
    {
      header: 'Node',
      render: (row) => {
        const nodeId = row.fields?.nodeId;
        if (!nodeId) return <span className="text-xs text-zinc-500">-</span>;
        return (
          <span className="text-xs px-2 py-0.5 rounded bg-cyan-500/20 text-cyan-400">
            {nodeId}
          </span>
        );
      }
    },
    {
      header: 'User',
      render: (row) => (
        <span className="text-xs text-zinc-300 font-mono">{row.user || '-'}</span>
      )
    },
    {
      header: 'Message',
      render: (row) => {
        const fields = row.fields || {};
        const details = [];
        if (fields.method) details.push(`method=${fields.method}`);
        if (fields.path) details.push(`path=${fields.path}`);
        if (fields.status !== undefined) details.push(`status=${fields.status}`);
        if (fields.dn) details.push(`dn=${fields.dn}`);
        if (fields.baseDN) details.push(`baseDN=${fields.baseDN}`);
        if (fields.scope) details.push(`scope=${fields.scope}`);
        if (fields.filter) details.push(`filter=${fields.filter}`);
        if (fields.code) details.push(`code=${fields.code}`);
        if (fields.error) details.push(`error=${fields.error}`);
        if (fields.results !== undefined) details.push(`results=${fields.results}`);
        if (fields.changes !== undefined) details.push(`changes=${fields.changes}`);
        
        return (
          <div>
            <span className="text-sm text-zinc-100">{row.message}</span>
            {details.length > 0 && (
              <div className="text-xs text-zinc-500 mt-0.5 font-mono">
                {details.join(' | ')}
              </div>
            )}
          </div>
        );
      }
    },
    {
      header: 'Client IP',
      render: (row) => {
        const fields = row.fields || {};
        // REST logs have remoteAddr, LDAP logs have request_id
        if (row.source === 'rest' && fields.remoteAddr) {
          // Extract IP from "192.168.1.100:54321" format
          const ip = fields.remoteAddr.split(':')[0];
          return <span className="text-xs text-zinc-500 font-mono">{ip}</span>;
        }
        if (row.request_id) {
          return <span className="text-xs text-zinc-500 font-mono">{row.request_id}</span>;
        }
        return <span className="text-xs text-zinc-500">-</span>;
      }
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
          <select
            value={filters.source}
            onChange={(e) => setFilters({ ...filters, source: e.target.value })}
            className="px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 focus:outline-none focus:border-blue-500"
          >
            <option value="">All Sources</option>
            <option value="ldap">LDAP</option>
            <option value="rest">REST</option>
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
            limit: pageSize,
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
