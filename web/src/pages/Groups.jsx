import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Plus, Trash2, Eye, Users } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import Table from '../components/Table';
import Modal from '../components/Modal';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../context/ToastContext';

export default function Groups() {
  const { showToast } = useToast();
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(true);
  const [baseDN, setBaseDN] = useState('');
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [membersModal, setMembersModal] = useState({ open: false, group: null, members: [] });

  const fetchGroups = async () => {
    setLoading(true);
    try {
      const config = await api.getConfig();
      const base = config?.directory?.baseDN || 'dc=example,dc=com';
      setBaseDN(base);

      const data = await api.searchEntries({
        baseDN: base,
        scope: 'sub',
        filter: '(|(objectClass=groupOfNames)(objectClass=groupOfUniqueNames)(objectClass=posixGroup))',
        limit: 1000
      });
      setGroups(data.entries || []);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchGroups();
  }, []);

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await api.deleteEntry(deleteTarget);
      showToast('Group deleted', 'success');
      fetchGroups();
    } catch (err) {
      showToast(err.message, 'error');
    }
    setDeleteTarget(null);
  };

  const showMembers = (group) => {
    const members = group.attributes?.member || 
                    group.attributes?.uniqueMember || 
                    group.attributes?.memberUid || [];
    setMembersModal({ open: true, group, members });
  };

  const extractCN = (dn) => {
    const match = dn.match(/^cn=([^,]+)/i);
    return match ? match[1] : dn;
  };

  const getOU = (dn) => {
    const match = dn.match(/ou=([^,]+)/i);
    return match ? match[1] : '-';
  };

  const getMemberCount = (group) => {
    const members = group.attributes?.member || 
                    group.attributes?.uniqueMember || 
                    group.attributes?.memberUid || [];
    return members.length;
  };

  const columns = [
    {
      header: 'Group Name',
      render: (row) => (
        <span className="font-medium text-zinc-100">{extractCN(row.dn)}</span>
      )
    },
    {
      header: 'Description',
      render: (row) => (
        <span className="text-zinc-400">{row.attributes?.description?.[0] || '-'}</span>
      )
    },
    {
      header: 'Members',
      render: (row) => {
        const count = getMemberCount(row);
        return (
          <button
            onClick={() => showMembers(row)}
            className="flex items-center gap-1 text-blue-400 hover:text-blue-300"
          >
            <Users className="w-4 h-4" />
            <span>{count}</span>
          </button>
        );
      }
    },
    {
      header: 'OU',
      render: (row) => (
        <span className="text-zinc-500 text-sm">{getOU(row.dn)}</span>
      )
    },
    {
      header: 'Actions',
      render: (row) => (
        <div className="flex items-center gap-1">
          <Link
            to={`/entries/${encodeURIComponent(row.dn)}`}
            className="p-1.5 text-zinc-400 hover:text-zinc-100"
            title="View"
          >
            <Eye className="w-4 h-4" />
          </Link>
          <button
            onClick={() => setDeleteTarget(row.dn)}
            className="p-1.5 text-zinc-400 hover:text-red-500"
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
        title="Groups"
        actions={
          <Link
            to="/groups/new"
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg text-sm"
          >
            <Plus className="w-4 h-4" />
            Add Group
          </Link>
        }
      />

      <div className="bg-zinc-800 rounded-lg border border-zinc-700">
        <Table
          columns={columns}
          data={groups}
          loading={loading}
          emptyTitle="No groups found"
          emptyDescription="Add groups to your directory to see them here."
        />
      </div>

      <ConfirmDialog
        isOpen={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete Group"
        message={`Are you sure you want to delete "${extractCN(deleteTarget || '')}"? This action cannot be undone.`}
      />

      <Modal
        isOpen={membersModal.open}
        onClose={() => setMembersModal({ open: false, group: null, members: [] })}
        title={`Members of ${extractCN(membersModal.group?.dn || '')}`}
      >
        <div className="max-h-96 overflow-y-auto">
          {membersModal.members.length === 0 ? (
            <p className="text-zinc-400 text-sm">No members in this group.</p>
          ) : (
            <ul className="space-y-2">
              {membersModal.members.map((member, i) => (
                <li key={i} className="flex items-center gap-2 p-2 bg-zinc-900 rounded">
                  <Users className="w-4 h-4 text-zinc-500" />
                  <span className="text-sm text-zinc-300 font-mono">{extractCN(member)}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </Modal>
    </div>
  );
}
