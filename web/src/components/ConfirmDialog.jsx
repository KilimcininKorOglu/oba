import { AlertTriangle } from 'lucide-react';
import Modal from './Modal';

export default function ConfirmDialog({ isOpen, onClose, onConfirm, title, message, confirmText = 'Delete', destructive = true }) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      footer={
        <>
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-zinc-400 hover:text-zinc-100"
          >
            Cancel
          </button>
          <button
            onClick={() => { onConfirm(); onClose(); }}
            className={`px-4 py-2 text-sm rounded-lg ${destructive ? 'bg-red-500 hover:bg-red-600 text-white' : 'bg-blue-500 hover:bg-blue-600 text-white'}`}
          >
            {confirmText}
          </button>
        </>
      }
    >
      <div className="flex items-start gap-4">
        <div className={`p-2 rounded-full ${destructive ? 'bg-red-500/10' : 'bg-blue-500/10'}`}>
          <AlertTriangle className={`w-6 h-6 ${destructive ? 'text-red-500' : 'text-blue-500'}`} />
        </div>
        <p className="text-zinc-300">{message}</p>
      </div>
    </Modal>
  );
}
