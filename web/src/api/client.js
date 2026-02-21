class ObaAPI {
  constructor(baseURL = '/api/v1') {
    this.baseURL = baseURL;
    this.token = localStorage.getItem('token');
  }

  setToken(token) {
    this.token = token;
    if (token) {
      localStorage.setItem('token', token);
    } else {
      localStorage.removeItem('token');
    }
  }

  async request(method, path, body = null) {
    const headers = { 'Content-Type': 'application/json' };
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const response = await fetch(`${this.baseURL}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : null
    });

    if (response.status === 401) {
      this.setToken(null);
      window.location.href = '/login';
      throw new Error('Unauthorized');
    }

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Request failed' }));
      throw new Error(error.message || 'Request failed');
    }

    if (response.status === 204) return null;
    return response.json();
  }

  async login(dn, password) {
    const data = await this.request('POST', '/auth/bind', { dn, password });
    if (data.success && data.token) {
      this.setToken(data.token);
    }
    return data;
  }

  logout() {
    this.setToken(null);
  }

  getHealth() {
    return this.request('GET', '/health');
  }

  getStats() {
    return this.request('GET', '/stats');
  }

  getActivities(limit = 10) {
    return this.request('GET', `/activities?limit=${limit}`);
  }

  getEntry(dn) {
    return this.request('GET', `/entries/${encodeURIComponent(dn)}`);
  }

  searchEntries(params) {
    const query = new URLSearchParams(params).toString();
    return this.request('GET', `/search?${query}`);
  }

  addEntry(dn, attributes) {
    return this.request('POST', '/entries', { dn, attributes });
  }

  modifyEntry(dn, changes) {
    return this.request('PATCH', `/entries/${encodeURIComponent(dn)}`, { changes });
  }

  deleteEntry(dn) {
    return this.request('DELETE', `/entries/${encodeURIComponent(dn)}`);
  }

  moveEntry(dn, newRDN, deleteOldRDN = true, newSuperior = null) {
    return this.request('POST', `/entries/${encodeURIComponent(dn)}/move`, {
      newRDN,
      deleteOldRDN,
      newSuperior
    });
  }

  disableEntry(dn) {
    return this.request('POST', `/entries/${encodeURIComponent(dn)}/disable`);
  }

  enableEntry(dn) {
    return this.request('POST', `/entries/${encodeURIComponent(dn)}/enable`);
  }

  unlockEntry(dn) {
    return this.request('POST', `/entries/${encodeURIComponent(dn)}/unlock`);
  }

  getLockStatus(dn) {
    return this.request('GET', `/entries/${encodeURIComponent(dn)}/lock-status`);
  }

  compare(dn, attribute, value) {
    return this.request('POST', '/compare', { dn, attribute, value });
  }

  bulk(operations, stopOnError = false) {
    return this.request('POST', '/bulk', { operations, stopOnError });
  }

  getACL() {
    return this.request('GET', '/acl');
  }

  getACLRules() {
    return this.request('GET', '/acl/rules');
  }

  getACLRule(index) {
    return this.request('GET', `/acl/rules/${index}`);
  }

  addACLRule(rule, index = -1) {
    return this.request('POST', '/acl/rules', { rule, index });
  }

  updateACLRule(index, rule) {
    return this.request('PUT', `/acl/rules/${index}`, rule);
  }

  deleteACLRule(index) {
    return this.request('DELETE', `/acl/rules/${index}`);
  }

  setDefaultPolicy(policy) {
    return this.request('PUT', '/acl/default', { policy });
  }

  reloadACL() {
    return this.request('POST', '/acl/reload');
  }

  saveACL() {
    return this.request('POST', '/acl/save');
  }

  validateACL(config) {
    return this.request('POST', '/acl/validate', config);
  }

  getConfig() {
    return this.request('GET', '/config');
  }

  getConfigSection(section) {
    return this.request('GET', `/config/${section}`);
  }

  updateConfigSection(section, data) {
    return this.request('PATCH', `/config/${section}`, data);
  }

  reloadConfig() {
    return this.request('POST', '/config/reload');
  }

  saveConfig() {
    return this.request('POST', '/config/save');
  }

  validateConfig(config) {
    return this.request('POST', '/config/validate', config);
  }

  async getLogs(params = {}) {
    const query = new URLSearchParams(params).toString();
    const headers = { 'Authorization': `Bearer ${this.token}` };
    
    const response = await fetch(`${this.baseURL}/logs?${query}`, { headers });
    
    if (response.status === 503) {
      return { entries: [], total_count: 0, disabled: true };
    }
    
    if (!response.ok) {
      throw new Error('Failed to fetch logs');
    }
    
    return response.json();
  }

  async getLogStats() {
    const headers = { 'Authorization': `Bearer ${this.token}` };
    const response = await fetch(`${this.baseURL}/logs/stats`, { headers });
    if (!response.ok) return null;
    return response.json();
  }

  async clearLogs() {
    return this.request('DELETE', '/logs');
  }

  async exportLogs(params = {}) {
    const query = new URLSearchParams(params).toString();
    const headers = { 'Authorization': `Bearer ${this.token}` };
    
    const response = await fetch(`${this.baseURL}/logs/export?${query}`, { headers });
    if (!response.ok) throw new Error('Export failed');
    return response.blob();
  }

  async streamSearch(params, onEntry) {
    const query = new URLSearchParams(params).toString();
    const headers = {};
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const response = await fetch(`${this.baseURL}/search/stream?${query}`, { headers });

    if (!response.ok) {
      throw new Error('Stream search failed');
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop();

      for (const line of lines) {
        if (line.trim()) {
          const entry = JSON.parse(line);
          onEntry(entry);
        }
      }
    }
  }

  // Cluster endpoints
  getClusterStatus() {
    return this.request('GET', '/cluster/status');
  }

  getClusterHealth() {
    return this.request('GET', '/cluster/health');
  }

  getClusterLeader() {
    return this.request('GET', '/cluster/leader');
  }
}

export const api = new ObaAPI();
export default api;
