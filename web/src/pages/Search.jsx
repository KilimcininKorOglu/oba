import { useState } from 'react';
import { Search as SearchIcon, Download } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import Table from '../components/Table';
import { useToast } from '../context/ToastContext';

export default function Search() {
  const { showToast } = useToast();
  const [params, setParams] = useState({
    baseDN: '',
    scope: 'sub',
    filter: '',
    attributes: '',
    limit: '100',
    offset: '0',
    timeLimit: ''
  });
  const [results, setResults] = useState(null);
  const [loading, setLoading] = useState(false);

  const handleSearch = async (e) => {
    e?.preventDefault();
    if (!params.baseDN) {
      showToast('Base DN is required', 'error');
      return;
    }

    setLoading(true);
    try {
      const searchParams = { baseDN: params.baseDN, scope: params.scope };
      if (params.filter) searchParams.filter = params.filter;
      if (params.attributes) searchParams.attributes = params.attributes;
      if (params.limit) searchParams.limit = params.limit;
      if (params.offset) searchParams.offset = params.offset;
      if (params.timeLimit) searchParams.timeLimit = params.timeLimit;

      const data = await api.searchEntries(searchParams);
      setResults(data);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  const handlePageChange = async (newOffset) => {
    const newParams = { ...params, offset: Math.max(0, newOffset).toString() };
    setParams(newParams);
    
    setLoading(true);
    try {
      const searchParams = { baseDN: newParams.baseDN, scope: newParams.scope };
      if (newParams.filter) searchParams.filter = newParams.filter;
      if (newParams.attributes) searchParams.attributes = newParams.attributes;
      if (newParams.limit) searchParams.limit = newParams.limit;
      searchParams.offset = newParams.offset;
      if (newParams.timeLimit) searchParams.timeLimit = newParams.timeLimit;

      const data = await api.searchEntries(searchParams);
      setResults(data);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  const exportJSON = () => {
    if (!results?.entries) return;
    const blob = new Blob([JSON.stringify(results.entries, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'ldap-search-results.json';
    a.click();
    URL.revokeObjectURL(url);
  };

  const columns = [
    { header: 'DN', render: (row) => <span className="font-mono text-sm">{row.dn}</span> },
    {
      header: 'Attributes',
      render: (row) => (
        <span className="text-zinc-400 text-sm">
          {Object.keys(row.attributes || {}).slice(0, 3).join(', ')}
          {Object.keys(row.attributes || {}).length > 3 && '...'}
        </span>
      )
    }
  ];

  return (
    <div>
      <Header title="Search" />

      <form onSubmit={handleSearch} className="bg-zinc-800 rounded-lg border border-zinc-700 p-6 mb-6">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Base DN *</label>
            <input
              type="text"
              value={params.baseDN}
              onChange={(e) => setParams({ ...params, baseDN: e.target.value })}
              placeholder="dc=example,dc=com"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Scope</label>
            <select
              value={params.scope}
              onChange={(e) => setParams({ ...params, scope: e.target.value })}
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 focus:outline-none focus:border-blue-500"
            >
              <option value="base">Base</option>
              <option value="one">One Level</option>
              <option value="sub">Subtree</option>
            </select>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Filter</label>
            <input
              type="text"
              value={params.filter}
              onChange={(e) => setParams({ ...params, filter: e.target.value })}
              placeholder="(objectClass=*)"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Attributes (comma-separated)</label>
            <input
              type="text"
              value={params.attributes}
              onChange={(e) => setParams({ ...params, attributes: e.target.value })}
              placeholder="cn,mail,sn"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Limit</label>
            <input
              type="number"
              value={params.limit}
              onChange={(e) => setParams({ ...params, limit: e.target.value })}
              placeholder="100"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Offset</label>
            <input
              type="number"
              value={params.offset}
              onChange={(e) => setParams({ ...params, offset: e.target.value })}
              placeholder="0"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Time Limit (seconds)</label>
            <input
              type="number"
              value={params.timeLimit}
              onChange={(e) => setParams({ ...params, timeLimit: e.target.value })}
              placeholder="0"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>

        <button
          type="submit"
          disabled={loading}
          className="flex items-center gap-2 px-6 py-2 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 text-white rounded-lg"
        >
          <SearchIcon className="w-4 h-4" />
          {loading ? 'Searching...' : 'Search'}
        </button>
      </form>

      {results && (
        <div className="bg-zinc-800 rounded-lg border border-zinc-700">
          <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-700">
            <span className="text-sm text-zinc-400">
              Found {results.totalCount || 0} entries
            </span>
            <button
              onClick={exportJSON}
              className="flex items-center gap-2 text-sm text-blue-500 hover:text-blue-400"
            >
              <Download className="w-4 h-4" />
              Export JSON
            </button>
          </div>
          <Table
            columns={columns}
            data={results.entries || []}
            loading={loading}
            emptyTitle="No results"
            emptyDescription="No entries match your search criteria."
            pagination={{
              offset: results.offset || 0,
              limit: results.limit || 100,
              total: results.totalCount || 0,
              hasMore: results.hasMore || false
            }}
            onPageChange={handlePageChange}
          />
        </div>
      )}
    </div>
  );
}
