import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Plus, Trash2, Save } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import LoadingSpinner from '../components/LoadingSpinner';
import { useToast } from '../context/ToastContext';

export default function EntryForm() {
  const { dn } = useParams();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const isEdit = !!dn;
  const decodedDN = dn ? decodeURIComponent(dn) : '';

  const [loading, setLoading] = useState(isEdit);
  const [saving, setSaving] = useState(false);
  const [formDN, setFormDN] = useState('');
  const [attributes, setAttributes] = useState([{ key: 'objectClass', values: [''] }]);

  useEffect(() => {
    if (isEdit) {
      const fetchEntry = async () => {
        try {
          const data = await api.getEntry(decodedDN);
          setFormDN(data.dn);
          const attrs = Object.entries(data.attributes || {}).map(([key, values]) => ({
            key,
            values: Array.isArray(values) ? values : [values]
          }));
          setAttributes(attrs.length > 0 ? attrs : [{ key: '', values: [''] }]);
        } catch (err) {
          showToast(err.message, 'error');
        } finally {
          setLoading(false);
        }
      };
      fetchEntry();
    }
  }, [isEdit, decodedDN]);

  const addAttribute = () => {
    setAttributes([...attributes, { key: '', values: [''] }]);
  };

  const removeAttribute = (index) => {
    setAttributes(attributes.filter((_, i) => i !== index));
  };

  const updateAttributeKey = (index, key) => {
    const newAttrs = [...attributes];
    newAttrs[index].key = key;
    setAttributes(newAttrs);
  };

  const addValue = (attrIndex) => {
    const newAttrs = [...attributes];
    newAttrs[attrIndex].values.push('');
    setAttributes(newAttrs);
  };

  const removeValue = (attrIndex, valueIndex) => {
    const newAttrs = [...attributes];
    newAttrs[attrIndex].values = newAttrs[attrIndex].values.filter((_, i) => i !== valueIndex);
    setAttributes(newAttrs);
  };

  const updateValue = (attrIndex, valueIndex, value) => {
    const newAttrs = [...attributes];
    newAttrs[attrIndex].values[valueIndex] = value;
    setAttributes(newAttrs);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSaving(true);

    const attrObj = {};
    attributes.forEach(attr => {
      if (attr.key && attr.values.some(v => v)) {
        attrObj[attr.key] = attr.values.filter(v => v);
      }
    });

    try {
      if (isEdit) {
        const original = await api.getEntry(decodedDN);
        const changes = [];
        
        Object.keys(attrObj).forEach(key => {
          if (!original.attributes[key]) {
            changes.push({ operation: 'add', attribute: key, values: attrObj[key] });
          } else if (JSON.stringify(original.attributes[key]) !== JSON.stringify(attrObj[key])) {
            changes.push({ operation: 'replace', attribute: key, values: attrObj[key] });
          }
        });
        
        Object.keys(original.attributes).forEach(key => {
          if (!attrObj[key]) {
            changes.push({ operation: 'delete', attribute: key, values: [] });
          }
        });

        if (changes.length > 0) {
          await api.modifyEntry(decodedDN, changes);
        }
        showToast('Entry updated', 'success');
      } else {
        await api.addEntry(formDN, attrObj);
        showToast('Entry created', 'success');
      }
      navigate('/entries');
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setSaving(false);
    }
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
      <Header title={isEdit ? 'Edit Entry' : 'Add Entry'} />

      <form onSubmit={handleSubmit} className="max-w-3xl">
        <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-6 mb-6">
          <label className="block text-sm font-medium text-zinc-300 mb-2">Distinguished Name (DN)</label>
          <input
            type="text"
            value={formDN}
            onChange={(e) => setFormDN(e.target.value)}
            disabled={isEdit}
            placeholder="cn=newuser,ou=users,dc=example,dc=com"
            className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500 disabled:opacity-50"
            required
          />
        </div>

        <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-6 mb-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-medium text-zinc-100">Attributes</h2>
            <button
              type="button"
              onClick={addAttribute}
              className="flex items-center gap-2 px-3 py-1 text-sm text-blue-500 hover:text-blue-400"
            >
              <Plus className="w-4 h-4" />
              Add Attribute
            </button>
          </div>

          <div className="space-y-4">
            {attributes.map((attr, attrIndex) => (
              <div key={attrIndex} className="p-4 bg-zinc-900 rounded-lg">
                <div className="flex items-center gap-4 mb-3">
                  <input
                    type="text"
                    value={attr.key}
                    onChange={(e) => updateAttributeKey(attrIndex, e.target.value)}
                    placeholder="Attribute name"
                    className="flex-1 px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500 text-sm"
                  />
                  <button
                    type="button"
                    onClick={() => removeAttribute(attrIndex)}
                    className="p-2 text-zinc-400 hover:text-red-500"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
                <div className="space-y-2">
                  {attr.values.map((value, valueIndex) => (
                    <div key={valueIndex} className="flex items-center gap-2">
                      <input
                        type="text"
                        value={value}
                        onChange={(e) => updateValue(attrIndex, valueIndex, e.target.value)}
                        placeholder="Value"
                        className="flex-1 px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500 text-sm"
                      />
                      {attr.values.length > 1 && (
                        <button
                          type="button"
                          onClick={() => removeValue(attrIndex, valueIndex)}
                          className="p-2 text-zinc-400 hover:text-red-500"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      )}
                    </div>
                  ))}
                  <button
                    type="button"
                    onClick={() => addValue(attrIndex)}
                    className="text-sm text-blue-500 hover:text-blue-400"
                  >
                    + Add value
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-4">
          <button
            type="submit"
            disabled={saving}
            className="flex items-center gap-2 px-6 py-2 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 text-white rounded-lg"
          >
            {saving ? <LoadingSpinner size="sm" /> : <Save className="w-4 h-4" />}
            {saving ? 'Saving...' : 'Save Entry'}
          </button>
          <button
            type="button"
            onClick={() => navigate(-1)}
            className="px-6 py-2 text-zinc-400 hover:text-zinc-100"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
