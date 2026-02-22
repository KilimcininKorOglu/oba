import { useState, useEffect } from 'react';
import { Plus, Trash2, Pencil, UserX, UserCheck, Unlock } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import Modal from '../components/Modal';
import MultiSelect from '../components/MultiSelect';
import { useToast } from '../context/ToastContext';

export default function Users() {
  const { showToast } = useToast();
  const [users, setUsers] = useState([]);
  const [allGroups, setAllGroups] = useState([]);
  const [lockedUsers, setLockedUsers] = useState({});
  const [loading, setLoading] = useState(true);
  const [baseDN, setBaseDN] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editUser, setEditUser] = useState(null);
  const [formLoading, setFormLoading] = useState(false);
  const [form, setForm] = useState({ uid: '', cn: '', sn: '', mail: '', password: '', groups: [] });

  const fetchData = async () => {
    setLoading(true);
    try {
      const config = await api.getConfig();
      const base = config?.directory?.baseDN || 'dc=example,dc=com';
      setBaseDN(base);

      const [usersData, groupsData] = await Promise.all([
        api.searchEntries({
          baseDN: base,
          scope: 'sub',
          filter: '(|(objectClass=person)(objectClass=inetOrgPerson)(objectClass=organizationalPerson)(objectClass=user))',
          limit: 1000
        }),
        api.searchEntries({
          baseDN: base,
          scope: 'sub',
          filter: '(|(objectClass=groupOfNames)(objectClass=groupOfUniqueNames)(objectClass=posixGroup)(objectClass=group))',
          limit: 1000
        })
      ]);

      setUsers(usersData.entries || []);
      setAllGroups(groupsData.entries || []);

      // Check lock status for each user
      const lockStatuses = {};
      for (const user of usersData.entries || []) {
        try {
          const status = await api.getLockStatus(user.dn);
          if (status.locked) {
            lockStatuses[user.dn] = true;
          }
        } catch {
          // Ignore errors
        }
      }
      setLockedUsers(lockStatuses);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleDelete = async (dn) => {
    if (!confirm(`Delete user ${dn}?`)) return;
    try {
      await api.deleteEntry(dn);
      showToast('User deleted', 'success');
      fetchData();
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const handleToggleStatus = async (user) => {
    const isDisabled = isUserDisabled(user);
    try {
      if (isDisabled) {
        await api.enableEntry(user.dn);
        showToast('User enabled', 'success');
      } else {
        await api.disableEntry(user.dn);
        showToast('User disabled', 'success');
      }
      fetchData();
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const handleUnlock = async (user) => {
    try {
      await api.unlockEntry(user.dn);
      setLockedUsers(prev => {
        const next = { ...prev };
        delete next[user.dn];
        return next;
      });
      showToast('Account unlocked', 'success');
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const getAttr = (entry, name) => {
    const attrs = entry.attributes || {};
    const val = attrs[name] || attrs[name.toLowerCase()];
    return Array.isArray(val) ? val : val ? [val] : [];
  };

  const isUserDisabled = (user) => {
    const disabled = getAttr(user, 'obaDisabled')[0] || getAttr(user, 'obadisabled')[0];
    if (!disabled) return false;
    const val = disabled.toLowerCase();
    return val === 'true' || val === '1' || val === 'yes';
  };

  const extractCN = (dn) => {
    const match = dn.match(/^cn=([^,]+)/i);
    return match ? match[1] : dn;
  };

  const getUserGroups = (userDN) => {
    const userDNLower = userDN.toLowerCase();
    return allGroups
      .filter(g => getAttr(g, 'member').some(m => m.toLowerCase() === userDNLower))
      .map(g => g.dn);
  };

  const openAddModal = () => {
    setEditUser(null);
    setForm({ uid: '', cn: '', sn: '', mail: '', password: '', groups: [] });
    setShowModal(true);
  };

  const openEditModal = (user) => {
    const getAttrVal = (name) => {
      const attrs = user.attributes || {};
      const val = attrs[name] || attrs[name.toLowerCase()];
      return Array.isArray(val) ? val[0] : val || '';
    };
    setEditUser(user);
    setForm({
      uid: getAttrVal('uid') || extractCN(user.dn),
      cn: getAttrVal('cn'),
      sn: getAttrVal('sn'),
      mail: getAttrVal('mail'),
      password: '',
      groups: getUserGroups(user.dn)
    });
    setShowModal(true);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!form.cn || !form.sn) {
      showToast('CN and SN are required', 'error');
      return;
    }

    setFormLoading(true);
    try {
      let userDN;
      
      if (editUser) {
        userDN = editUser.dn;
        const modifications = [];
        if (form.cn) modifications.push({ operation: 'replace', attribute: 'cn', values: [form.cn] });
        if (form.sn) modifications.push({ operation: 'replace', attribute: 'sn', values: [form.sn] });
        if (form.mail) modifications.push({ operation: 'replace', attribute: 'mail', values: [form.mail] });
        else modifications.push({ operation: 'delete', attribute: 'mail' });
        if (form.password) modifications.push({ operation: 'replace', attribute: 'userPassword', values: [form.password] });

        await api.modifyEntry(editUser.dn, modifications);
      } else {
        if (!form.uid) {
          showToast('UID is required', 'error');
          setFormLoading(false);
          return;
        }
        userDN = `uid=${form.uid},ou=users,${baseDN}`;
        await api.addEntry(userDN, {
          objectClass: ['inetOrgPerson', 'organizationalPerson', 'person', 'top'],
          uid: [form.uid],
          cn: [form.cn],
          sn: [form.sn],
          ...(form.mail && { mail: [form.mail] }),
          ...(form.password && { userPassword: [form.password] })
        });
      }

      const currentGroups = editUser ? getUserGroups(editUser.dn) : [];
      const newGroups = form.groups;

      for (const groupDN of currentGroups) {
        if (!newGroups.includes(groupDN)) {
          const group = allGroups.find(g => g.dn === groupDN);
          if (group) {
            const members = getAttr(group, 'member').filter(m => m !== userDN);
            if (members.length > 0) {
              await api.modifyEntry(groupDN, [{ operation: 'replace', attribute: 'member', values: members }]);
            }
          }
        }
      }

      for (const groupDN of newGroups) {
        if (!currentGroups.includes(groupDN)) {
          const group = allGroups.find(g => g.dn === groupDN);
          if (group) {
            const members = [...getAttr(group, 'member'), userDN];
            await api.modifyEntry(groupDN, [{ operation: 'replace', attribute: 'member', values: members }]);
          }
        }
      }

      showToast(editUser ? 'User updated' : 'User created', 'success');
      setShowModal(false);
      setForm({ uid: '', cn: '', sn: '', mail: '', password: '', groups: [] });
      setEditUser(null);
      fetchData();
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setFormLoading(false);
    }
  };

  const groupOptions = allGroups.map(group => ({
    value: group.dn,
    label: extractCN(group.dn)
  }));

  return (
    <div>
      <Header 
        title="Users" 
        actions={
          <button
            onClick={openAddModal}
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
                <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">Status</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">Groups</th>
                <th className="px-6 py-3 text-right text-xs font-medium text-zinc-400 uppercase">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-700">
              {users.map((user) => {
                const userGroups = getUserGroups(user.dn);
                const disabled = isUserDisabled(user);
                const locked = lockedUsers[user.dn];
                return (
                  <tr key={user.dn} className={`hover:bg-zinc-700/50 ${disabled ? 'opacity-60' : ''}`}>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="font-medium text-zinc-100">{getAttr(user, 'cn')[0]}</div>
                      <div className="text-sm text-zinc-500">{getAttr(user, 'uid')[0]}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-zinc-400">
                      {getAttr(user, 'mail')[0] || '-'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex flex-wrap gap-1">
                        {locked && (
                          <span className="px-2 py-1 text-xs rounded-full bg-orange-500/20 text-orange-400">
                            Locked
                          </span>
                        )}
                        <span className={`px-2 py-1 text-xs rounded-full ${
                          disabled 
                            ? 'bg-red-500/20 text-red-400' 
                            : 'bg-green-500/20 text-green-400'
                        }`}>
                          {disabled ? 'Disabled' : 'Active'}
                        </span>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      {userGroups.length > 0 ? (
                        <div className="flex flex-wrap gap-1">
                          {userGroups.map(gdn => (
                            <span key={gdn} className="px-2 py-0.5 bg-zinc-700 text-zinc-300 text-xs rounded">
                              {extractCN(gdn)}
                            </span>
                          ))}
                        </div>
                      ) : (
                        <span className="text-zinc-500">-</span>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right">
                      <div className="flex items-center justify-end gap-2">
                        {locked && (
                          <button
                            onClick={() => handleUnlock(user)}
                            className="text-orange-400 hover:text-orange-300"
                            title="Unlock account"
                          >
                            <Unlock className="w-4 h-4" />
                          </button>
                        )}
                        <button
                          onClick={() => handleToggleStatus(user)}
                          className={disabled ? 'text-green-400 hover:text-green-300' : 'text-yellow-400 hover:text-yellow-300'}
                          title={disabled ? 'Enable user' : 'Disable user'}
                        >
                          {disabled ? <UserCheck className="w-4 h-4" /> : <UserX className="w-4 h-4" />}
                        </button>
                        <button
                          onClick={() => openEditModal(user)}
                          className="text-zinc-400 hover:text-zinc-200"
                          title="Edit"
                        >
                          <Pencil className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => handleDelete(user.dn)}
                          className="text-red-400 hover:text-red-300"
                          title="Delete"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <Modal isOpen={showModal} onClose={() => setShowModal(false)} title={editUser ? 'Edit User' : 'Add User'}>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">User ID (uid) *</label>
            <input
              type="text"
              value={form.uid}
              onChange={(e) => setForm({ ...form, uid: e.target.value })}
              disabled={!!editUser}
              className="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md text-zinc-100 focus:outline-none focus:border-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
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
            <label className="block text-sm font-medium text-zinc-300 mb-1">
              {editUser ? 'New Password (leave empty to keep current)' : 'Password'}
            </label>
            <input
              type="password"
              value={form.password}
              onChange={(e) => setForm({ ...form, password: e.target.value })}
              className="w-full px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md text-zinc-100 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">Groups</label>
            <MultiSelect
              options={groupOptions}
              value={form.groups}
              onChange={(groups) => setForm({ ...form, groups })}
              placeholder="Select groups..."
            />
          </div>
          {!editUser && (
            <div className="text-sm text-zinc-500">
              DN: <span className="font-mono text-zinc-400">uid={form.uid || '...'},ou=users,{baseDN}</span>
            </div>
          )}
          <div className="flex gap-3 pt-2">
            <button
              type="submit"
              disabled={formLoading}
              className="flex-1 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-md disabled:opacity-50"
            >
              {formLoading ? (editUser ? 'Updating...' : 'Creating...') : (editUser ? 'Update User' : 'Create User')}
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
