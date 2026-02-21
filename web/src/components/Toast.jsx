import { X, CheckCircle, AlertCircle, AlertTriangle, Info } from 'lucide-react';
import { useToast } from '../context/ToastContext';

const icons = {
  success: CheckCircle,
  error: AlertCircle,
  warning: AlertTriangle,
  info: Info
};

const colors = {
  success: 'bg-zinc-800 border-green-500 text-green-500',
  error: 'bg-zinc-800 border-red-500 text-red-500',
  warning: 'bg-zinc-800 border-yellow-500 text-yellow-500',
  info: 'bg-zinc-800 border-blue-500 text-blue-500'
};

export default function Toast() {
  const { toasts, removeToast } = useToast();

  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-2">
      {toasts.map(toast => {
        const Icon = icons[toast.type] || Info;
        return (
          <div
            key={toast.id}
            className={`flex items-center gap-3 px-4 py-3 rounded-lg border ${colors[toast.type] || colors.info}`}
          >
            <Icon className="w-5 h-5 flex-shrink-0" />
            <span className="text-sm text-zinc-100">{toast.message}</span>
            <button
              onClick={() => removeToast(toast.id)}
              className="ml-2 text-zinc-400 hover:text-zinc-100"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        );
      })}
    </div>
  );
}
