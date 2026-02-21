import { createContext, useContext, useState, useEffect } from 'react';
import api from '../api/client';

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem('token');
    const userDN = localStorage.getItem('userDN');
    if (token && userDN) {
      api.setToken(token);
      setUser({ dn: userDN });
      setIsAuthenticated(true);
    }
    setLoading(false);
  }, []);

  const login = async (dn, password) => {
    const result = await api.login(dn, password);
    if (result.success) {
      localStorage.setItem('userDN', dn);
      setUser({ dn });
      setIsAuthenticated(true);
    }
    return result;
  };

  const logout = () => {
    api.logout();
    localStorage.removeItem('userDN');
    setUser(null);
    setIsAuthenticated(false);
  };

  return (
    <AuthContext.Provider value={{ user, isAuthenticated, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}
