import { useState } from 'react';
import { Play, Upload, CheckCircle, XCircle } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import LoadingSpinner from '../components/LoadingSpinner';
import { useToast } from '../context/ToastContext';

const sampleOperations = `[
  {
    "operation": "add",
    "dn": "cn=user1,ou=users,dc=example,dc=com",
    "attributes": {
      "objectClass": ["person", "inetOrgPerson"],
      "cn": ["user1"],
      "sn": ["User One"]
    }
  }
]`;

export default function Bulk() {
  const { showToast } = useToast();
  const [operations, setOperations] = useState(sampleOperations);
  const [stopOnError, setStopOnError] = useState(false);
  const [loading, setLoading] = useState(false);
  const [results, setResults] = useState(null);

  const handleImport = (e) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      setOperations(event.target?.result || '');
    };
    reader.readAsText(file);
  };

  const handleExecute = async () => {
    let ops;
    try {
      ops = JSON.parse(operations);
      if (!Array.isArray(ops)) {
        throw new Error('Operations must be an array');
      }
    } catch (err) {
      showToast('Invalid JSON: ' + err.message, 'error');
      return;
    }

    if (ops.length === 0) {
      showToast('No operations to execute', 'error');
      return;
    }

    setLoading(true);
    setResults(null);

    try {
      const data = await api.bulk(ops, stopOnError);
      setResults(data);
      if (data.success) {
        showToast(`All ${data.succeeded} operations completed successfully`, 'success');
      } else {
        showToast(`${data.succeeded} succeeded, ${data.failed} failed`, 'warning');
      }
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <Header title="Bulk Operations" />

      <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-6 mb-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-medium text-zinc-100">Operations JSON</h2>
          <label className="flex items-center gap-2 px-4 py-2 bg-zinc-700 hover:bg-zinc-600 rounded-lg text-sm text-zinc-100 cursor-pointer">
            <Upload className="w-4 h-4" />
            Import JSON
            <input type="file" accept=".json" onChange={handleImport} className="hidden" />
          </label>
        </div>

        <textarea
          value={operations}
          onChange={(e) => setOperations(e.target.value)}
          rows={15}
          className="w-full px-4 py-3 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 font-mono text-sm focus:outline-none focus:border-blue-500"
          placeholder="Enter operations JSON array..."
        />

        <div className="flex items-center justify-between mt-4">
          <label className="flex items-center gap-2 text-sm text-zinc-300">
            <input
              type="checkbox"
              checked={stopOnError}
              onChange={(e) => setStopOnError(e.target.checked)}
              className="rounded border-zinc-700 bg-zinc-900"
            />
            Stop on first error
          </label>

          <button
            onClick={handleExecute}
            disabled={loading}
            className="flex items-center gap-2 px-6 py-2 bg-blue-500 hover:bg-blue-600 disabled:opacity-50 text-white rounded-lg"
          >
            {loading ? <LoadingSpinner size="sm" /> : <Play className="w-4 h-4" />}
            {loading ? 'Executing...' : 'Execute'}
          </button>
        </div>
      </div>

      {results && (
        <div className="bg-zinc-800 rounded-lg border border-zinc-700">
          <div className="px-6 py-4 border-b border-zinc-700">
            <div className="flex items-center gap-4">
              <span className={`text-lg font-medium ${results.success ? 'text-green-500' : 'text-yellow-500'}`}>
                {results.success ? 'All operations completed' : 'Some operations failed'}
              </span>
              <span className="text-sm text-zinc-400">
                {results.succeeded} succeeded, {results.failed} failed
              </span>
            </div>
          </div>

          <div className="divide-y divide-zinc-700 max-h-96 overflow-y-auto">
            {results.results?.map((result, i) => (
              <div key={i} className="px-6 py-3 flex items-center gap-4">
                {result.success ? (
                  <CheckCircle className="w-5 h-5 text-green-500 flex-shrink-0" />
                ) : (
                  <XCircle className="w-5 h-5 text-red-500 flex-shrink-0" />
                )}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-zinc-100">{result.operation}</span>
                    <span className="text-sm text-zinc-400 truncate">{result.dn}</span>
                  </div>
                  {result.error && (
                    <p className="text-sm text-red-400 mt-1">{result.error}</p>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
