import { useState } from 'react';
import { GitCompare, CheckCircle, XCircle } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import LoadingSpinner from '../components/LoadingSpinner';
import { useToast } from '../context/ToastContext';

export default function Compare() {
  const { showToast } = useToast();
  const [dn, setDn] = useState('');
  const [attribute, setAttribute] = useState('');
  const [value, setValue] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState(null);

  const handleCompare = async (e) => {
    e.preventDefault();
    if (!dn || !attribute || !value) {
      showToast('All fields are required', 'error');
      return;
    }

    setLoading(true);
    setResult(null);

    try {
      const data = await api.compare(dn, attribute, value);
      setResult(data);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <Header title="Compare" />

      <form onSubmit={handleCompare} className="bg-zinc-800 rounded-lg border border-zinc-700 p-6 mb-6 max-w-2xl">
        <div className="space-y-4 mb-6">
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Distinguished Name (DN)</label>
            <input
              type="text"
              value={dn}
              onChange={(e) => setDn(e.target.value)}
              placeholder="cn=john,ou=users,dc=example,dc=com"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Attribute Name</label>
            <input
              type="text"
              value={attribute}
              onChange={(e) => setAttribute(e.target.value)}
              placeholder="mail"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">Value to Compare</label>
            <input
              type="text"
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder="john@example.com"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>

        <button
          type="submit"
          disabled={loading}
          className="flex items-center gap-2 px-6 py-2 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 text-white rounded-lg"
        >
          {loading ? <LoadingSpinner size="sm" /> : <GitCompare className="w-4 h-4" />}
          {loading ? 'Comparing...' : 'Compare'}
        </button>
      </form>

      {result !== null && (
        <div className={`bg-zinc-800 rounded-lg border p-6 max-w-2xl flex items-center gap-4 ${result.match ? 'border-green-500/50' : 'border-red-500/50'}`}>
          {result.match ? (
            <>
              <CheckCircle className="w-8 h-8 text-green-500" />
              <div>
                <p className="text-lg font-medium text-green-500">Match</p>
                <p className="text-sm text-zinc-400">The attribute value matches the provided value.</p>
              </div>
            </>
          ) : (
            <>
              <XCircle className="w-8 h-8 text-red-500" />
              <div>
                <p className="text-lg font-medium text-red-500">No Match</p>
                <p className="text-sm text-zinc-400">The attribute value does not match the provided value.</p>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
}
