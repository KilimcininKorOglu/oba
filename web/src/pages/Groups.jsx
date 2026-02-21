import { useState, useEffect } from 'react';
import { Plus, Trash2, Users, Pencil } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import Modal from '../components/Modal';
import { useToast } from '../context/ToastContext';

export default function Groups() {
  const { showToast } = useToast();
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(true);
  const [baseDN, setBaseDN] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editGroup, setEditGroup] = useState(null);
  const [formLoading, setFormLoading] = useState(false);
  const [form, setForm] = useState({ cn: '', description: '', members: '' });

  const fetchGroups = async () => {
    setLoading(true);
    try {
      const config = await api.getConfig();
      const base = config?.directory?.baseDN || 'dc=example,dc=com';
      setBaseDN(base);

      const data = await api.searchEntries({
        baseDN: base,
        scope: 'sub',
        filter: '(|(objectClass=groupOfNames)(objectClass=groupOfUniqueNames)(objectClass=posixGroup)(objectClass=group))',
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

  const handleDelete = async (dn) => {
    if (!confirm(`Delete group ${dn}?`)) return;
    try {
      await api.deleteEntry(dn);
      showToast('Group deleted', 'success');
      fetchGroups();
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const getAttr = (entry, name) => {
    const attrs = entry.attributes || {};
    const val = attrs[name] || attrs[name.toLowerCase()];
    return Array.isArray(val) ? val : val ? [val] : [];
  };

  const extractCN = (dn) => {
    const match = dn.match(/^cn=([^,]+)/i);
    return match ? match[1] : dn;
  };

  const openAddModal = () => {
    setEditGroup(null);
    setForm({ cn: '', description: '', members: '' });
    setShowModal(true);
  };

  const openEditModal = (group) => {
    setEditGroup(group);
    setForm({
      cn: extractCN(group.dn),
      description: getAttr(group, 'description')[0] || '',
      members: getAttr(group, 'member').join('\n')
    });
    setShowModal(true);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!form.cn) {
      showToast('Group name is required', 'error');
      return;
    }

    setFormLoading(true);
    try {
      const members = form.members.split('\n').map(m => m.trim()).filter(m => m);

      if (editGroup) {
        const modifications = [];
        if (form.description) {
          modifications.push({ operation: 'replace', attribute: 'description', values: [form.description] });
        } else {
          modifications.push({ operation: 'delete', attribute: 'description' });
        }
        if (members.length > 0) {
          modifications.push({ operation: 'replace', attribute: 'member', values: members });
        }

        await api.modifyEntry(editGroup.dn, modifications);
        showToast('Group updated', 'success');
      } else {
        const dn = `cn=${form.cn},ou=groups,${baseDN}`;
        await api.addEntry(dn, {
          objectClass: ['groupOfNames', 'top'],
          cn: [form.cn],
          member: members.length > 0 ? members : [`cn=admin,${baseDN}`],
          ...(form.description && { description: [form.description] })
        });
        showToast('Group created', 'success');
      }
      setShowModal(false);
      setForm({ cn: '', description: '', members: '' });
      setEditGroup(null);
      fetchGroups();
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setFormLoading(false);
    }
  };

  return (
    <div>
      <Header 
        title="Groups" 
        actions={
          <button
            onClick={openAddModal}
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg"
          >
            <Plus className="w-4 h-4" />
            Add Group
          </button>
        }
      />

      {loading ? (
        <div className="text-center py-8 text-zinc-500">Loading...</div>
      ) : groups.length === 0 ? (
        <div className="text-center py-8 text-zinc-500">No groups found</div>
      ) : (
        <div className="bg-zinc-800 border border-zinc-700 rounded-lg overflow-hidden">
          <table className="min-w-full divide-y divide-zinc-700">
            <thead className="bg-zinc-900">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">Name</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">Members</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">DN</th>
                <th className="px-6 py-3 text-right text-xs font-medium text-zinc-400 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-700">
              {groups.map((group) => (
                <tr key={group.dn} className="hover:bg-zinc-700/50">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="font-medium text-zinc-100">{extractCN(group.dn)}</div>
                    <div className="text-sm text-zinc-500">{getAttr(group, 'description')[0] || ''}</div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2 text-zinc-400">
                      <Users className="w-4 h-4" />
                      {getAttr(group, 'member').length} member(s)
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm text-zinc-500 font-mono">
                    {group.dn}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => openEditModal(group)}
                        className="text-zinc-400 hover:text-zinc-200"
                        title="Edit"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleDelete(group.dn)}
                        className="text-red-400 hover:text-red-300"
                        title="Delete"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal isOpen={showModal} onClose={() => setShowModal(false)} title={editGroup ? 'Edit Group' : 'Add Group'}>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">Group Name (cn) *</label>
            <input
              type="text"
              value={form.cn}
              onChange={(e) => setForm({ ...form, cn: e.target.value })}
              disabled={!!editGroup}
              className="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md text-zinc-100 focus:outline-none focus:border-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
              placeholder="developers"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">Description</label>
            <input
              type="text"
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              className="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md text-zinc-100 focus:outline-none focus:border-blue-500"
              placeholder="Development team"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">Members (one DN per line)</label>
            <textarea
              value={form.members}
              onChange={(e) => setForm({ ...form, members: e.target.value })}
              rows={4}
              className="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md text-zinc-100 focus:outline-none focus:border-blue-500"
              placeholder={`uid=jdoe,ou=users,${baseDN}`}
            />
          </div>
          {!editGroup && (
            <div className="text-sm text-zinc-500">
              DN: <span className="font-mono text-zinc-400">cn={form.cn || '...'},ou=groups,{baseDN}</span>
            </div>
          )}
          <div className="flex gap-3 pt-2">
            <button
              type="submit"
              disabled={formLoading}
              className="flex-1 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-md disabled:opacity-50"
            >
              {formLoading ? (editGroup ? 'Updating...' : 'Creating...') : (editGroup ? 'Update Group' : 'Create Group')}
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
