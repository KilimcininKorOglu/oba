import { useState, useEffect } from 'react';
import { RefreshCw, Save, Edit, Check, X } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import LoadingSpinner from '../components/LoadingSpinner';
import { useToast } from '../context/ToastContext';

const sections = ['server', 'logging', 'security', 'rest', 'storage'];

const hotReloadableFields = {
  logging: ['level', 'format'],
  server: ['maxConnections', 'readTimeout', 'writeTimeout'],
  security: ['rateLimit', 'passwordPolicy'],
  'security.rateLimit': ['enabled', 'maxAttempts', 'lockoutDuration'],
  'security.passwordPolicy': ['enabled', 'minLength', 'requireUppercase', 'requireLowercase', 'requireDigit', 'requireSpecial'],
  rest: ['rateLimit', 'tokenTTL', 'corsOrigins']
};

export default function Config() {
  const { showToast } = useToast();
  const [config, setConfig] = useState(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [activeSection, setActiveSection] = useState('server');
  const [editField, setEditField] = useState(null);
  const [editValue, setEditValue] = useState('');

  const fetchConfig = async () => {
    setLoading(true);
    try {
      const data = await api.getConfig();
      setConfig(data);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchConfig();
  }, []);

  const handleReload = async () => {
    try {
      await api.reloadConfig();
      showToast('Config reloaded from file', 'success');
      fetchConfig();
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.saveConfig();
      showToast('Config saved to file', 'success');
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const isHotReloadable = (section, field) => {
    const key = section;
    return hotReloadableFields[key]?.includes(field);
  };

  const startEdit = (field, value) => {
    setEditField(field);
    setEditValue(typeof value === 'object' ? JSON.stringify(value) : String(value));
  };

  const cancelEdit = () => {
    setEditField(null);
    setEditValue('');
  };

  const saveEdit = async (section, field) => {
    try {
      let value = editValue;
      if (editValue === 'true') value = true;
      else if (editValue === 'false') value = false;
      else if (!isNaN(Number(editValue)) && editValue !== '') value = Number(editValue);
      else {
        try {
          value = JSON.parse(editValue);
        } catch {
          // keep as string
        }
      }

      await api.updateConfigSection(section, { [field]: value });
      showToast('Config updated', 'success');
      fetchConfig();
    } catch (err) {
      showToast(err.message, 'error');
    }
    cancelEdit();
  };

  const renderValue = (value, field) => {
    if (field?.toLowerCase().includes('password') || field?.toLowerCase().includes('secret') || field?.toLowerCase().includes('key')) {
      return '********';
    }
    if (typeof value === 'boolean') return value ? 'true' : 'false';
    if (Array.isArray(value)) return value.join(', ');
    if (typeof value === 'object') return JSON.stringify(value);
    return String(value);
  };

  const renderSection = (sectionData, sectionName, prefix = '') => {
    if (!sectionData || typeof sectionData !== 'object') return null;

    return Object.entries(sectionData).map(([key, value]) => {
      const fullKey = prefix ? `${prefix}.${key}` : key;
      const isEditable = isHotReloadable(sectionName, key);
      const isEditing = editField === fullKey;

      if (typeof value === 'object' && !Array.isArray(value) && value !== null) {
        return (
          <div key={fullKey} className="mb-4">
            <h3 className="text-sm font-medium text-zinc-400 mb-2 capitalize">{key}</h3>
            <div className="pl-4 border-l border-zinc-700">
              {renderSection(value, `${sectionName}.${key}`, fullKey)}
            </div>
          </div>
        );
      }

      return (
        <div key={fullKey} className="flex items-center justify-between py-2 border-b border-zinc-700/50">
          <div className="flex items-center gap-2">
            <span className="text-sm text-zinc-300">{key}</span>
            {isEditable && (
              <span className="px-1.5 py-0.5 text-xs bg-blue-500/20 text-blue-400 rounded">hot-reload</span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {isEditing ? (
              <>
                <input
                  type="text"
                  value={editValue}
                  onChange={(e) => setEditValue(e.target.value)}
                  className="px-2 py-1 bg-zinc-900 border border-zinc-600 rounded text-sm text-zinc-100 w-48"
                  autoFocus
                />
                <button onClick={() => saveEdit(sectionName, key)} className="p-1 text-green-500 hover:text-green-400">
                  <Check className="w-4 h-4" />
                </button>
                <button onClick={cancelEdit} className="p-1 text-zinc-400 hover:text-zinc-100">
                  <X className="w-4 h-4" />
                </button>
              </>
            ) : (
              <>
                <span className="text-sm text-zinc-400 font-mono">{renderValue(value, key)}</span>
                {isEditable && (
                  <button onClick={() => startEdit(fullKey, value)} className="p-1 text-zinc-500 hover:text-zinc-100">
                    <Edit className="w-4 h-4" />
                  </button>
                )}
              </>
            )}
          </div>
        </div>
      );
    });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  return (
    <div>
      <Header
        title="Configuration"
        actions={
          <div className="flex items-center gap-2">
            <button onClick={handleReload} className="flex items-center gap-2 px-4 py-2 bg-zinc-700 hover:bg-zinc-600 text-white rounded-lg text-sm">
              <RefreshCw className="w-4 h-4" />
              Reload
            </button>
            <button onClick={handleSave} disabled={saving} className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg text-sm">
              {saving ? <LoadingSpinner size="sm" /> : <Save className="w-4 h-4" />}
              Save
            </button>
          </div>
        }
      />

      <div className="flex gap-6">
        <div className="w-48 flex-shrink-0">
          <nav className="space-y-1">
            {sections.map(section => (
              <button
                key={section}
                onClick={() => setActiveSection(section)}
                className={`w-full px-4 py-2 text-left text-sm rounded-lg capitalize ${activeSection === section ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800/50'}`}
              >
                {section}
              </button>
            ))}
          </nav>
        </div>

        <div className="flex-1 bg-zinc-800 rounded-lg border border-zinc-700 p-6">
          <h2 className="text-lg font-medium text-zinc-100 mb-4 capitalize">{activeSection}</h2>
          {config && config[activeSection] ? (
            renderSection(config[activeSection], activeSection)
          ) : (
            <p className="text-zinc-400">No configuration available for this section.</p>
          )}
        </div>
      </div>
    </div>
  );
}
