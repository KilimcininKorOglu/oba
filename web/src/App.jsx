import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { ToastProvider } from './context/ToastContext';
import Layout from './components/Layout';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Entries from './pages/Entries';
import EntryDetail from './pages/EntryDetail';
import EntryForm from './pages/EntryForm';
import Search from './pages/Search';
import Bulk from './pages/Bulk';
import Compare from './pages/Compare';
import ACL from './pages/ACL';
import Config from './pages/Config';

function ProtectedRoute({ children }) {
  const { isAuthenticated, loading } = useAuth();
  if (loading) return null;
  return isAuthenticated ? children : <Navigate to="/login" />;
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/" element={<ProtectedRoute><Layout /></ProtectedRoute>}>
        <Route index element={<Dashboard />} />
        <Route path="entries" element={<Entries />} />
        <Route path="entries/new" element={<EntryForm />} />
        <Route path="entries/:dn" element={<EntryDetail />} />
        <Route path="entries/:dn/edit" element={<EntryForm />} />
        <Route path="search" element={<Search />} />
        <Route path="bulk" element={<Bulk />} />
        <Route path="compare" element={<Compare />} />
        <Route path="acl" element={<ACL />} />
        <Route path="config" element={<Config />} />
      </Route>
    </Routes>
  );
}

export default function App() {
  return (
    <ToastProvider>
      <AuthProvider>
        <BrowserRouter>
          <AppRoutes />
        </BrowserRouter>
      </AuthProvider>
    </ToastProvider>
  );
}
