import { useState, useEffect } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import Modal from '../components/Modal';
import { useToast } from '../context/ToastContext';

export default function Users() {
  const { showToast } = useToast();
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [baseDN, setBaseDN] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [formLoading, setFormLoading] = useState(false);
  const [form, setForm] = useState({ uid: '', cn: '', sn: '', mail: '', password: '' });

  const fetchUsers = async () => {
    setLoading(true);
    try {
      const config = await api.getConfig();
      const base = config?.directory?.baseDN || 'dc=example,dc=com';
      setBaseDN(base);

      const data = await api.searchEntries({
        baseDN: base,
        scope: 'sub',
        filter: '(|(objectClass=person)(objectClass=inetOrgPerson)(objectClass=organizationalPerson)(objectClass=user))',
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

  const handleDelete = async (dn) => {
    if (!confirm(`Delete user ${dn}?`)) return;
    try {
      await api.deleteEntry(dn);
      showToast('User deleted', 'success');
      fetchUsers();
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!form.uid || !form.cn || !form.sn) {
      showToast('UID, CN and SN are required', 'error');
      return;
    }

    setFormLoading(true);
    try {
      const dn = `uid=${form.uid},ou=users,${baseDN}`;
      await api.addEntry(dn, {
        objectClass: ['inetOrgPerson', 'organizationalPerson', 'person', 'top'],
        uid: [form.uid],
        cn: [form.cn],
        sn: [form.sn],
        ...(form.mail && { mail: [form.mail] }),
        ...(form.password && { userPassword: [form.password] })
      });
      showToast('User created', 'success');
      setShowModal(false);
      setForm({ uid: '', cn: '', sn: '', mail: '', password: '' });
      fetchUsers();
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setFormLoading(false);
    }
  };

  const getAttr = (entry, name) => {
    const attrs = entry.attributes || {};
    const val = attrs[name] || attrs[name.toLowerCase()];
    return Array.isArray(val) ? val[0] : val || '';
  };

  return (
    <div>
      <Header 
        title="Users" 
        action={
          <button
            onClick={() => setShowModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg"
          >
            <Plus className="w-4 h-4" />
            Add User
          </button>
        }
      />

      {loading ? (
        <div className="text-center py-8 text-zinc-500">Loading...</div>
      ) : users.length === 0 ? (
        <div className="text-center py-8 text-zinc-500">No users found</div>
      ) : (
        <div className="bg-zinc-800 border border-zinc-700 rounded-lg overflow-hidden">
          <table className="min-w-full divide-y divide-zinc-700">
            <thead className="bg-zinc-900">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">Name</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">Email</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">DN</th>
                <th className="px-6 py-3 text-right text-xs font-medium text-zinc-400 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-700">
              {users.map((user) => (
                <tr key={user.dn} className="hover:bg-zinc-700/50">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="font-medium text-zinc-100">{getAttr(user, 'cn')}</div>
                    <div className="text-sm text-zinc-500">{getAttr(user, 'uid')}</div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-zinc-400">
                    {getAttr(user, 'mail') || '-'}
                  </td>
                  <td className="px-6 py-4 text-sm text-zinc-500 font-mono">
                    {user.dn}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right">
                    <button
                      onClick={() => handleDelete(user.dn)}
                      className="text-red-400 hover:text-red-300"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal isOpen={showModal} onClose={() => setShowModal(false)} title="Add User">
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">User ID (uid) *</label>
            <input
              type="text"
              value={form.uid}
              onChange={(e) => setForm({ ...form, uid: e.target.value })}
              className="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md text-zinc-100 focus:outline-none focus:border-blue-500"
              placeholder="jdoe"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">Full Name (cn) *</label>
            <input
              type="text"
              value={form.cn}
              onChange={(e) => setForm({ ...form, cn: e.target.value })}
              className="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md text-zinc-100 focus:outline-none focus:border-blue-500"
              placeholder="John Doe"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">Surname (sn) *</label>
            <input
              type="text"
              value={form.sn}
              onChange={(e) => setForm({ ...form, sn: e.target.value })}
              className="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md text-zinc-100 focus:outline-none focus:border-blue-500"
              placeholder="Doe"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">Email</label>
            <input
              type="email"
              value={form.mail}
              onChange={(e) => setForm({ ...form, mail: e.target.value })}
              className="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md text-zinc-100 focus:outline-none focus:border-blue-500"
              placeholder="jdoe@example.com"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">Password</label>
            <input
              type="password"
              value={form.password}
              onChange={(e) => setForm({ ...form, password: e.target.value })}
              className="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md text-zinc-100 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div className="text-sm text-zinc-500">
            DN: <span className="font-mono text-zinc-400">uid={form.uid || '...'},ou=users,{baseDN}</span>
          </div>
          <div className="flex gap-3 pt-2">
            <button
              type="submit"
              disabled={formLoading}
              className="flex-1 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-md disabled:opacity-50"
            >
              {formLoading ? 'Creating...' : 'Create User'}
            </button>
            <button
              type="button"
              onClick={() => setShowModal(false)}
              className="px-4 py-2 bg-zinc-700 text-zinc-300 rounded-md hover:bg-zinc-600"
            >
              Cancel
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
