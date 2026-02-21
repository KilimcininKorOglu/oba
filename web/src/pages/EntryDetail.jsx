import { useState, useEffect } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, Edit, Trash2, Move } from 'lucide-react';
import api from '../api/client';
import Header from '../components/Header';
import LoadingSpinner from '../components/LoadingSpinner';
import ConfirmDialog from '../components/ConfirmDialog';
import Modal from '../components/Modal';
import { useToast } from '../context/ToastContext';

export default function EntryDetail() {
  const { dn } = useParams();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const [entry, setEntry] = useState(null);
  const [loading, setLoading] = useState(true);
  const [showDelete, setShowDelete] = useState(false);
  const [showMove, setShowMove] = useState(false);
  const [moveData, setMoveData] = useState({ newRDN: '', newSuperior: '', deleteOldRDN: true });

  const decodedDN = decodeURIComponent(dn);

  useEffect(() => {
    const fetchEntry = async () => {
      setLoading(true);
      try {
        const data = await api.getEntry(decodedDN);
        setEntry(data);
      } catch (err) {
        showToast(err.message, 'error');
      } finally {
        setLoading(false);
      }
    };
    fetchEntry();
  }, [decodedDN]);

  const handleDelete = async () => {
    try {
      await api.deleteEntry(decodedDN);
      showToast('Entry deleted', 'success');
      navigate('/entries');
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const handleMove = async () => {
    try {
      await api.moveEntry(decodedDN, moveData.newRDN, moveData.deleteOldRDN, moveData.newSuperior || null);
      showToast('Entry moved/renamed', 'success');
      navigate('/entries');
    } catch (err) {
      showToast(err.message, 'error');
    }
    setShowMove(false);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (!entry) {
    return (
      <div className="bg-red-500/10 border border-red-500/50 rounded-lg p-4 text-red-500">
        Entry not found
      </div>
    );
  }

  const parentDN = decodedDN.split(',').slice(1).join(',');

  return (
    <div>
      <Header
        title="Entry Detail"
        actions={
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowMove(true)}
              className="flex items-center gap-2 px-4 py-2 bg-zinc-700 hover:bg-zinc-600 text-white rounded-lg text-sm"
            >
              <Move className="w-4 h-4" />
              Move/Rename
            </button>
            <Link
              to={`/entries/${encodeURIComponent(decodedDN)}/edit`}
              className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg text-sm"
            >
              <Edit className="w-4 h-4" />
              Edit
            </Link>
            <button
              onClick={() => setShowDelete(true)}
              className="flex items-center gap-2 px-4 py-2 bg-red-500 hover:bg-red-600 text-white rounded-lg text-sm"
            >
              <Trash2 className="w-4 h-4" />
              Delete
            </button>
          </div>
        }
      />

      <Link
        to={`/entries?baseDN=${encodeURIComponent(parentDN)}`}
        className="flex items-center gap-2 text-sm text-blue-500 hover:text-blue-400 mb-4"
      >
        <ArrowLeft className="w-4 h-4" />
        Back to parent
      </Link>

      <div className="bg-zinc-800 rounded-lg border border-zinc-700 p-6 mb-6">
        <h2 className="text-sm font-medium text-zinc-400 mb-2">Distinguished Name</h2>
        <p className="font-mono text-zinc-100 break-all">{entry.dn}</p>
      </div>

      <div className="bg-zinc-800 rounded-lg border border-zinc-700">
        <div className="px-6 py-4 border-b border-zinc-700">
          <h2 className="text-lg font-medium text-zinc-100">Attributes</h2>
        </div>
        <div className="divide-y divide-zinc-700">
          {Object.entries(entry.attributes || {}).map(([key, values]) => (
            <div key={key} className="px-6 py-4">
              <div className="text-sm font-medium text-zinc-400 mb-1">{key}</div>
              <div className="space-y-1">
                {(Array.isArray(values) ? values : [values]).map((value, i) => (
                  <div key={i} className="text-zinc-100 font-mono text-sm break-all">
                    {key === 'userPassword' ? '********' : value}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>

      <ConfirmDialog
        isOpen={showDelete}
        onClose={() => setShowDelete(false)}
        onConfirm={handleDelete}
        title="Delete Entry"
        message={`Are you sure you want to delete "${decodedDN}"? This action cannot be undone.`}
      />

      <Modal
        isOpen={showMove}
        onClose={() => setShowMove(false)}
        title="Move/Rename Entry"
        footer={
          <>
            <button onClick={() => setShowMove(false)} className="px-4 py-2 text-sm text-zinc-400 hover:text-zinc-100">
              Cancel
            </button>
            <button
              onClick={handleMove}
              disabled={!moveData.newRDN}
              className="px-4 py-2 text-sm bg-blue-500 hover:bg-blue-600 disabled:opacity-50 text-white rounded-lg"
            >
              Move/Rename
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">New RDN</label>
            <input
              type="text"
              value={moveData.newRDN}
              onChange={(e) => setMoveData({ ...moveData, newRDN: e.target.value })}
              placeholder="cn=newname"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-2">New Superior (optional)</label>
            <input
              type="text"
              value={moveData.newSuperior}
              onChange={(e) => setMoveData({ ...moveData, newSuperior: e.target.value })}
              placeholder="ou=newparent,dc=example,dc=com"
              className="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-blue-500"
            />
          </div>
          <label className="flex items-center gap-2 text-sm text-zinc-300">
            <input
              type="checkbox"
              checked={moveData.deleteOldRDN}
              onChange={(e) => setMoveData({ ...moveData, deleteOldRDN: e.target.checked })}
              className="rounded border-zinc-700 bg-zinc-900"
            />
            Delete old RDN value
          </label>
        </div>
      </Modal>
    </div>
  );
}
