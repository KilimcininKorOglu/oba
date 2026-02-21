import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Save } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import LoadingSpinner from '../components/LoadingSpinner';
import { useToast } from '../context/ToastContext';

export default function UserForm() {
  const navigate = useNavigate();
  const { showToast } = useToast();
  const [saving, setSaving] = useState(false);
  const [baseDN, setBaseDN] = useState('');
  const [formData, setFormData] = useState({
    username: '',
    firstName: '',
    lastName: '',
    email: '',
    password: '',
    ou: 'users'
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
    if (!formData.username || !formData.password) {
      showToast('Username and password are required', 'error');
      return;
    }

    setSaving(true);
    try {
      const dn = `cn=${formData.username},ou=${formData.ou},${baseDN}`;
      const attributes = {
        objectClass: ['top', 'person', 'organizationalPerson', 'inetOrgPerson'],
        cn: [formData.username],
        sn: [formData.lastName || formData.username],
        userPassword: [formData.password]
      };

      if (formData.firstName) {
        attributes.givenName = [formData.firstName];
      }
      if (formData.email) {
        attributes.mail = [formData.email];
      }

      await api.addEntry(dn, attributes);
      showToast('User created', 'success');
      navigate('/users');
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <Header title="Add User" />

      <form onSubmit={handleSubmit} className="max-w-2xl">
        <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-6 space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-zinc-300 mb-2">Username *</label>
              <input
                type="text"
                value={formData.username}
                onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                placeholder="john.doe"
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
                placeholder="users"
                className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-zinc-300 mb-2">First Name</label>
              <input
                type="text"
                value={formData.firstName}
                onChange={(e) => setFormData({ ...formData, firstName: e.target.value })}
                placeholder="John"
                className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-zinc-300 mb-2">Last Name</label>
              <input
                type="text"
                value={formData.lastName}
                onChange={(e) => setFormData({ ...formData, lastName: e.target.value })}
                placeholder="Doe"
                className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Email</label>
            <input
              type="email"
              value={formData.email}
              onChange={(e) => setFormData({ ...formData, email: e.target.value })}
              placeholder="john.doe@example.com"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Password *</label>
            <input
              type="password"
              value={formData.password}
              onChange={(e) => setFormData({ ...formData, password: e.target.value })}
              placeholder="Enter password"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
              required
            />
          </div>

          <div className="pt-2 text-sm text-zinc-500">
            User will be created at: <span className="font-mono text-zinc-400">cn={formData.username || '...'},{formData.ou ? `ou=${formData.ou},` : ''}{baseDN}</span>
          </div>
        </div>

        <div className="flex items-center gap-4 mt-6">
          <button
            type="submit"
            disabled={saving}
            className="flex items-center gap-2 px-6 py-2 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 text-white rounded-lg"
          >
            {saving ? <LoadingSpinner size="sm" /> : <Save className="w-4 h-4" />}
            {saving ? 'Creating...' : 'Create User'}
          </button>
          <button
            type="button"
            onClick={() => navigate('/users')}
            className="px-6 py-2 text-zinc-400 hover:text-zinc-100"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
