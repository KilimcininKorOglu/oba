import { ChevronLeft, ChevronRight } from 'lucide-react';
import LoadingSpinner from './LoadingSpinner';
import EmptyState from './EmptyState';

export default function Table({ columns, data, loading, emptyTitle, emptyDescription, pagination, onPageChange }) {
  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (!data || data.length === 0) {
    return <EmptyState title={emptyTitle || 'No data'} description={emptyDescription} />;
  }

  return (
    <div>
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-zinc-700">
              {columns.map((col, i) => (
                <th key={i} className="px-4 py-3 text-left text-sm font-medium text-zinc-400">
                  {col.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.map((row, rowIndex) => (
              <tr key={rowIndex} className="border-b border-zinc-800 hover:bg-zinc-800/50">
                {columns.map((col, colIndex) => (
                  <td key={colIndex} className="px-4 py-3 text-sm text-zinc-300">
                    {col.render ? col.render(row, rowIndex) : row[col.key]}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {pagination && (
        <div className="flex items-center justify-between px-4 py-3 border-t border-zinc-700">
          <span className="text-sm text-zinc-400">
            Showing {pagination.offset + 1} - {Math.min(pagination.offset + data.length, pagination.total)} of {pagination.total}
          </span>
          <div className="flex gap-2">
            <button
              onClick={() => onPageChange(pagination.offset - pagination.limit)}
              disabled={pagination.offset === 0}
              className="p-2 text-zinc-400 hover:text-zinc-100 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <ChevronLeft className="w-5 h-5" />
            </button>
            <button
              onClick={() => onPageChange(pagination.offset + pagination.limit)}
              disabled={!pagination.hasMore}
              className="p-2 text-zinc-400 hover:text-zinc-100 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <ChevronRight className="w-5 h-5" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
