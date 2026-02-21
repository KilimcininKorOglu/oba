// Date formatting utilities with server timezone support

let serverTimezone = null;

export function setServerTimezone(tz) {
  serverTimezone = tz;
}

export function getServerTimezone() {
  return serverTimezone;
}

export function formatDate(date, options = {}) {
  if (!date) return '';
  
  const d = new Date(date);
  if (isNaN(d.getTime())) return '';

  const defaultOptions = {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    ...options
  };

  // Use server timezone if available
  if (serverTimezone) {
    defaultOptions.timeZone = serverTimezone;
  }

  try {
    return d.toLocaleString(undefined, defaultOptions);
  } catch {
    // Fallback if timezone is invalid
    return d.toLocaleString();
  }
}

export function formatDateShort(date) {
  return formatDate(date, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
}

export function formatDateOnly(date) {
  return formatDate(date, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: undefined,
    minute: undefined,
    second: undefined
  });
}

export function formatTimeOnly(date) {
  return formatDate(date, {
    year: undefined,
    month: undefined,
    day: undefined,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
}

export function formatRelativeTime(date) {
  if (!date) return '';
  
  const d = new Date(date);
  if (isNaN(d.getTime())) return '';

  const now = new Date();
  const diff = now - d;

  if (diff < 60000) return 'just now';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}d ago`;
  
  return formatDateShort(date);
}

export function formatTimestamp(date) {
  if (!date) return '';
  
  const d = new Date(date);
  if (isNaN(d.getTime())) return '';

  // ISO format with server timezone indicator
  const formatted = formatDate(date);
  return serverTimezone ? `${formatted} (${serverTimezone})` : formatted;
}
