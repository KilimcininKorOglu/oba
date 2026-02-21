import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Save } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import LoadingSpinner from '../components/LoadingSpinner';
import { useToast } from '../context/ToastContext';

export default function GroupForm() {
  const navigate = useNavigate();
  const { showToast } = useToast();
  const [saving, setSaving] = useState(false);
  const [baseDN, setBaseDN] = useState('');
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    ou: 'groups'
  });

  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const config = await api.getConfig();
        setBaseDN(config?.directory?.baseDN || 'dc=example,dc=com');
      } catch {
        setBaseDN('dc=example,dc=com');
      }
    };
    fetchConfig();
  }, []);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!formData.name) {
      showToast('Group name is required', 'error');
      return;
    }

    setSaving(true);
    try {
      const dn = `cn=${formData.name},ou=${formData.ou},${baseDN}`;
      const attributes = {
        objectClass: ['top', 'groupOfNames'],
        cn: [formData.name],
        member: ['']
      };

      if (formData.description) {
        attributes.description = [formData.description];
      }

      await api.addEntry(dn, attributes);
      showToast('Group created', 'success');
      navigate('/groups');
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <Header title="Add Group" />

      <form onSubmit={handleSubmit} className="max-w-2xl">
        <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-6 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-zinc-300 mb-2">Group Name *</label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder="developers"
                className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-zinc-300 mb-2">Organizational Unit</label>
              <input
                type="text"
                value={formData.ou}
                onChange={(e) => setFormData({ ...formData, ou: e.target.value })}
                placeholder="groups"
                className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Description</label>
            <input
              type="text"
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              placeholder="Development team"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div className="pt-2 text-sm text-zinc-500">
            Group will be created at: <span className="font-mono text-zinc-400">cn={formData.name || '...'},{formData.ou ? `ou=${formData.ou},` : ''}{baseDN}</span>
          </div>
        </div>

        <div className="flex items-center gap-4 mt-6">
          <button
            type="submit"
            disabled={saving}
            className="flex items-center gap-2 px-6 py-2 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 text-white rounded-lg"
          >
            {saving ? <LoadingSpinner size="sm" /> : <Save className="w-4 h-4" />}
            {saving ? 'Creating...' : 'Create Group'}
          </button>
          <button
            type="button"
            onClick={() => navigate('/groups')}
            className="px-6 py-2 text-zinc-400 hover:text-zinc-100"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
