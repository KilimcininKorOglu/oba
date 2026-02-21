import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Plus, Trash2, Eye, Key, Lock, Unlock } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import Table from '../components/Table';
import Modal from '../components/Modal';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../context/ToastContext';

export default function Users() {
  const { showToast } = useToast();
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [baseDN, setBaseDN] = useState('');
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [passwordModal, setPasswordModal] = useState({ open: false, dn: '', newPassword: '' });

  const fetchUsers = async () => {
    setLoading(true);
    try {
      const config = await api.getConfig();
      const base = config?.directory?.baseDN || 'dc=example,dc=com';
      setBaseDN(base);

      const data = await api.searchEntries({
        baseDN: base,
        scope: 'sub',
        filter: '(objectClass=person)',
        limit: 1000
      });
      setUsers(data.entries || []);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await api.deleteEntry(deleteTarget);
      showToast('User deleted', 'success');
      fetchUsers();
    } catch (err) {
      showToast(err.message, 'error');
    }
    setDeleteTarget(null);
  };

  const handlePasswordChange = async () => {
    if (!passwordModal.dn || !passwordModal.newPassword) return;
    try {
      await api.modifyEntry(passwordModal.dn, [
        { operation: 'replace', attribute: 'userPassword', values: [passwordModal.newPassword] }
      ]);
      showToast('Password changed', 'success');
      setPasswordModal({ open: false, dn: '', newPassword: '' });
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const extractCN = (dn) => {
    const match = dn.match(/^cn=([^,]+)/i);
    return match ? match[1] : dn;
  };

  const getOU = (dn) => {
    const match = dn.match(/ou=([^,]+)/i);
    return match ? match[1] : '-';
  };

  const columns = [
    {
      header: 'Username',
      render: (row) => (
        <span className="font-medium text-zinc-100">{extractCN(row.dn)}</span>
      )
    },
    {
      header: 'Full Name',
      render: (row) => {
        const cn = row.attributes?.cn?.[0] || '';
        const sn = row.attributes?.sn?.[0] || '';
        const givenName = row.attributes?.givenName?.[0] || '';
        return <span className="text-zinc-400">{givenName && sn ? `${givenName} ${sn}` : cn}</span>;
      }
    },
    {
      header: 'Email',
      render: (row) => (
        <span className="text-zinc-400">{row.attributes?.mail?.[0] || '-'}</span>
      )
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
            onClick={() => setPasswordModal({ open: true, dn: row.dn, newPassword: '' })}
            className="p-1.5 text-zinc-400 hover:text-yellow-500"
            title="Change Password"
          >
            <Key className="w-4 h-4" />
          </button>
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
        title="Users"
        actions={
          <Link
            to="/users/new"
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg text-sm"
          >
            <Plus className="w-4 h-4" />
            Add User
          </Link>
        }
      />

      <div className="bg-zinc-800 rounded-lg border border-zinc-700">
        <Table
          columns={columns}
          data={users}
          loading={loading}
          emptyTitle="No users found"
          emptyDescription="Add users to your directory to see them here."
        />
      </div>

      <ConfirmDialog
        isOpen={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="Delete User"
        message={`Are you sure you want to delete "${extractCN(deleteTarget || '')}"? This action cannot be undone.`}
      />

      <Modal
        isOpen={passwordModal.open}
        onClose={() => setPasswordModal({ open: false, dn: '', newPassword: '' })}
        title="Change Password"
        footer={
          <>
            <button
              onClick={() => setPasswordModal({ open: false, dn: '', newPassword: '' })}
              className="px-4 py-2 text-sm text-zinc-400 hover:text-zinc-100"
            >
              Cancel
            </button>
            <button
              onClick={handlePasswordChange}
              className="px-4 py-2 text-sm bg-blue-500 hover:bg-blue-600 text-white rounded-lg"
            >
              Change Password
            </button>
          </>
        }
      >
        <div>
          <p className="text-sm text-zinc-400 mb-4">
            Changing password for: <span className="text-zinc-100">{extractCN(passwordModal.dn)}</span>
          </p>
          <label className="block text-sm font-medium text-zinc-300 mb-2">New Password</label>
          <input
            type="password"
            value={passwordModal.newPassword}
            onChange={(e) => setPasswordModal({ ...passwordModal, newPassword: e.target.value })}
            placeholder="Enter new password"
            className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
          />
        </div>
      </Modal>
    </div>
  );
}
