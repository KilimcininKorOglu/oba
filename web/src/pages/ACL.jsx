import { useState, useEffect } from 'react';
import { Plus, Edit, Trash2, ChevronUp, ChevronDown, RefreshCw, Save } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import Table from '../components/Table';
import Modal from '../components/Modal';
import ConfirmDialog from '../components/ConfirmDialog';
import LoadingSpinner from '../components/LoadingSpinner';
import { useToast } from '../context/ToastContext';

const emptyRule = {
  target: '',
  subject: '',
  scope: 'subtree',
  rights: [],
  attributes: [],
  deny: false
};

const allRights = ['read', 'write', 'add', 'delete', 'search', 'compare', 'all'];

export default function ACL() {
  const { showToast } = useToast();
  const [acl, setAcl] = useState(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [editIndex, setEditIndex] = useState(null);
  const [formData, setFormData] = useState(emptyRule);
  const [deleteIndex, setDeleteIndex] = useState(null);

  const fetchACL = async () => {
    setLoading(true);
    try {
      const data = await api.getACL();
      setAcl(data);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchACL();
  }, []);

  const handlePolicyChange = async (policy) => {
    try {
      await api.setDefaultPolicy(policy);
      showToast('Default policy updated', 'success');
      fetchACL();
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const handleReload = async () => {
    try {
      await api.reloadACL();
      showToast('ACL reloaded from file', 'success');
      fetchACL();
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.saveACL();
      showToast('ACL saved to file', 'success');
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const openAddForm = () => {
    setFormData(emptyRule);
    setEditIndex(null);
    setShowForm(true);
  };

  const openEditForm = (index) => {
    const rule = acl.rules[index];
    setFormData({
      target: rule.target || '',
      subject: rule.subject || '',
      scope: rule.scope || 'subtree',
      rights: rule.rights || [],
      attributes: rule.attributes || [],
      deny: rule.deny || false
    });
    setEditIndex(index);
    setShowForm(true);
  };

  const handleFormSubmit = async () => {
    const rule = {
      ...formData,
      attributes: formData.attributes.length > 0 ? formData.attributes : undefined
    };

    try {
      if (editIndex !== null) {
        await api.updateACLRule(editIndex, rule);
        showToast('Rule updated', 'success');
      } else {
        await api.addACLRule(rule);
        showToast('Rule added', 'success');
      }
      setShowForm(false);
      fetchACL();
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const handleDelete = async () => {
    if (deleteIndex === null) return;
    try {
      await api.deleteACLRule(deleteIndex);
      showToast('Rule deleted', 'success');
      fetchACL();
    } catch (err) {
      showToast(err.message, 'error');
    }
    setDeleteIndex(null);
  };

  const handleMove = async (index, direction) => {
    const newIndex = direction === 'up' ? index - 1 : index + 1;
    if (newIndex < 0 || newIndex >= acl.rules.length) return;

    const rule = acl.rules[index];
    try {
      await api.deleteACLRule(index);
      await api.addACLRule(rule, newIndex);
      fetchACL();
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const toggleRight = (right) => {
    if (formData.rights.includes(right)) {
      setFormData({ ...formData, rights: formData.rights.filter(r => r !== right) });
    } else {
      setFormData({ ...formData, rights: [...formData.rights, right] });
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  const columns = [
    { header: '#', render: (_, i) => <span className="text-zinc-500">{i}</span> },
    { header: 'Target', render: (row) => <span className="font-mono text-sm">{row.target}</span> },
    { header: 'Subject', render: (row) => <span className="font-mono text-sm">{row.subject}</span> },
    { header: 'Scope', key: 'scope' },
    {
      header: 'Rights',
      render: (row) => (
        <div className="flex flex-wrap gap-1">
          {row.rights?.map(r => (
            <span key={r} className={`px-2 py-0.5 text-xs rounded ${row.deny ? 'bg-red-500/20 text-red-400' : 'bg-green-500/20 text-green-400'}`}>
              {r}
            </span>
          ))}
        </div>
      )
    },
    {
      header: 'Actions',
      render: (row, i) => (
        <div className="flex items-center gap-1">
          <button onClick={() => handleMove(i, 'up')} disabled={i === 0} className="p-1 text-zinc-400 hover:text-zinc-100 disabled:opacity-30">
            <ChevronUp className="w-4 h-4" />
          </button>
          <button onClick={() => handleMove(i, 'down')} disabled={i === acl.rules.length - 1} className="p-1 text-zinc-400 hover:text-zinc-100 disabled:opacity-30">
            <ChevronDown className="w-4 h-4" />
          </button>
          <button onClick={() => openEditForm(i)} className="p-1 text-zinc-400 hover:text-zinc-100">
            <Edit className="w-4 h-4" />
          </button>
          <button onClick={() => setDeleteIndex(i)} className="p-1 text-zinc-400 hover:text-red-500">
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      )
    }
  ];

  return (
    <div>
      <Header
        title="ACL Rules"
        actions={
          <div className="flex items-center gap-2">
            <button onClick={handleReload} className="flex items-center gap-2 px-4 py-2 bg-zinc-700 hover:bg-zinc-600 text-white rounded-lg text-sm">
              <RefreshCw className="w-4 h-4" />
              Reload
            </button>
            <button onClick={handleSave} disabled={saving} className="flex items-center gap-2 px-4 py-2 bg-zinc-700 hover:bg-zinc-600 text-white rounded-lg text-sm">
              {saving ? <LoadingSpinner size="sm" /> : <Save className="w-4 h-4" />}
              Save
            </button>
            <button onClick={openAddForm} className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg text-sm">
              <Plus className="w-4 h-4" />
              Add Rule
            </button>
          </div>
        }
      />

      <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-4 mb-6">
        <div className="flex items-center gap-4">
          <span className="text-sm text-zinc-400">Default Policy:</span>
          <div className="flex gap-2">
            <button
              onClick={() => handlePolicyChange('allow')}
              className={`px-4 py-1 text-sm rounded-lg ${acl?.defaultPolicy === 'allow' ? 'bg-green-500 text-white' : 'bg-zinc-700 text-zinc-300 hover:bg-zinc-600'}`}
            >
              Allow
            </button>
            <button
              onClick={() => handlePolicyChange('deny')}
              className={`px-4 py-1 text-sm rounded-lg ${acl?.defaultPolicy === 'deny' ? 'bg-red-500 text-white' : 'bg-zinc-700 text-zinc-300 hover:bg-zinc-600'}`}
            >
              Deny
            </button>
          </div>
        </div>
      </div>

      <div className="bg-zinc-800 rounded-lg border border-zinc-700">
        <Table
          columns={columns}
          data={acl?.rules || []}
          loading={false}
          emptyTitle="No ACL rules"
          emptyDescription="Add rules to control access to your directory."
        />
      </div>

      <Modal
        isOpen={showForm}
        onClose={() => setShowForm(false)}
        title={editIndex !== null ? 'Edit Rule' : 'Add Rule'}
        footer={
          <>
            <button onClick={() => setShowForm(false)} className="px-4 py-2 text-sm text-zinc-400 hover:text-zinc-100">
              Cancel
            </button>
            <button onClick={handleFormSubmit} className="px-4 py-2 text-sm bg-blue-500 hover:bg-blue-600 text-white rounded-lg">
              {editIndex !== null ? 'Update' : 'Add'}
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Target DN</label>
            <input
              type="text"
              value={formData.target}
              onChange={(e) => setFormData({ ...formData, target: e.target.value })}
              placeholder="dc=example,dc=com or *"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Subject</label>
            <input
              type="text"
              value={formData.subject}
              onChange={(e) => setFormData({ ...formData, subject: e.target.value })}
              placeholder="authenticated, anonymous, self, or DN"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Scope</label>
            <select
              value={formData.scope}
              onChange={(e) => setFormData({ ...formData, scope: e.target.value })}
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 focus:outline-none focus:border-blue-500"
            >
              <option value="base">Base</option>
              <option value="one">One Level</option>
              <option value="subtree">Subtree</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Rights</label>
            <div className="flex flex-wrap gap-2">
              {allRights.map(right => (
                <button
                  key={right}
                  type="button"
                  onClick={() => toggleRight(right)}
                  className={`px-3 py-1 text-sm rounded-lg ${formData.rights.includes(right) ? 'bg-blue-500 text-white' : 'bg-zinc-700 text-zinc-300 hover:bg-zinc-600'}`}
                >
                  {right}
                </button>
              ))}
            </div>
          </div>
          <label className="flex items-center gap-2 text-sm text-zinc-300">
            <input
              type="checkbox"
              checked={formData.deny}
              onChange={(e) => setFormData({ ...formData, deny: e.target.checked })}
              className="rounded border-zinc-700 bg-zinc-900"
            />
            Deny rule (instead of allow)
          </label>
        </div>
      </Modal>

      <ConfirmDialog
        isOpen={deleteIndex !== null}
        onClose={() => setDeleteIndex(null)}
        onConfirm={handleDelete}
        title="Delete Rule"
        message="Are you sure you want to delete this ACL rule?"
      />
    </div>
  );
}
