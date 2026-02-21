import { useState, useEffect } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Plus, ChevronRight, Trash2, Eye } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import Table from '../components/Table';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../context/ToastContext';

export default function Entries() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [entries, setEntries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({ offset: 0, limit: 20, total: 0, hasMore: false });
  const [deleteTarget, setDeleteTarget] = useState(null);
  const { showToast } = useToast();

  const baseDN = searchParams.get('baseDN') || '';
  const offset = parseInt(searchParams.get('offset') || '0');

  const fetchEntries = async () => {
    setLoading(true);
    try {
      const params = { scope: 'one', limit: 20, offset };
      if (baseDN) params.baseDN = baseDN;
      else {
        const config = await api.getConfig();
        params.baseDN = config?.directory?.baseDN || 'dc=example,dc=com';
        setSearchParams({ baseDN: params.baseDN, offset: '0' });
      }
      const data = await api.searchEntries(params);
      setEntries(data.entries || []);
      setPagination({
        offset: data.offset || 0,
        limit: data.limit || 20,
        total: data.totalCount || 0,
        hasMore: data.hasMore || false
      });
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchEntries();
  }, [baseDN, offset]);

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await api.deleteEntry(deleteTarget);
      showToast('Entry deleted', 'success');
      fetchEntries();
    } catch (err) {
      showToast(err.message, 'error');
    }
    setDeleteTarget(null);
  };

  const handlePageChange = (newOffset) => {
    setSearchParams({ baseDN, offset: Math.max(0, newOffset).toString() });
  };

  const navigateToEntry = (dn) => {
    setSearchParams({ baseDN: dn, offset: '0' });
  };

  const breadcrumbs = baseDN ? baseDN.split(',').map((part, i, arr) => ({
    label: part,
    dn: arr.slice(i).join(',')
  })) : [];

  const columns = [
    {
      header: 'DN',
      render: (row) => (
        <span className="font-mono text-sm">{row.dn}</span>
      )
    },
    {
      header: 'Object Class',
      render: (row) => (
        <span className="text-zinc-400">
          {row.attributes?.objectClass?.join(', ') || 'N/A'}
        </span>
      )
    },
    {
      header: 'Actions',
      render: (row) => (
        <div className="flex items-center gap-2">
          <Link
            to={`/entries/${encodeURIComponent(row.dn)}`}
            className="p-1 text-zinc-400 hover:text-zinc-100"
            title="View"
          >
            <Eye className="w-4 h-4" />
          </Link>
          <button
            onClick={() => navigateToEntry(row.dn)}
            className="p-1 text-zinc-400 hover:text-zinc-100"
            title="Browse children"
          >
            <ChevronRight className="w-4 h-4" />
          </button>
          <button
            onClick={() => setDeleteTarget(row.dn)}
            className="p-1 text-zinc-400 hover:text-red-500"
            title="Delete"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      )
    }
  ];

  return (
    <div>
      <Header
        title="Entries"
        actions={
          <Link
            to="/entries/new"
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg text-sm"
          >
            <Plus className="w-4 h-4" />
            Add Entry
          </Link>
        }
      />

      {breadcrumbs.length > 0 && (
        <div className="flex items-center gap-2 mb-4 text-sm">
          <button
            onClick={() => setSearchParams({ baseDN: breadcrumbs[breadcrumbs.length - 1]?.dn || '', offset: '0' })}
            className="text-blue-500 hover:text-blue-400"
          >
            Root
          </button>
          {breadcrumbs.map((crumb, i) => (
            <span key={i} className="flex items-center gap-2">
              <ChevronRight className="w-4 h-4 text-zinc-600" />
              <button
                onClick={() => setSearchParams({ baseDN: crumb.dn, offset: '0' })}
                className={i === 0 ? 'text-zinc-100' : 'text-blue-500 hover:text-blue-400'}
              >
                {crumb.label}
              </button>
            </span>
          ))}
        </div>
      )}

      <div className="bg-zinc-800 rounded-lg border border-zinc-700">
        <Table
          columns={columns}
          data={entries}
          loading={loading}
          emptyTitle="No entries found"
          emptyDescription="This directory is empty or you don't have permission to view it."
          pagination={pagination}
          onPageChange={handlePageChange}
        />
      </div>

      <ConfirmDialog
        isOpen={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete Entry"
        message={`Are you sure you want to delete "${deleteTarget}"? This action cannot be undone.`}
      />
    </div>
  );
}
