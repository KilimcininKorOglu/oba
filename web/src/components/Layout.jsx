import { Outlet } from 'react-router-dom';
import Sidebar from './Sidebar';
import Toast from './Toast';

export default function Layout() {
  return (
    <div className="min-h-screen bg-zinc-900">
      <Sidebar />
      <main className="ml-64 p-8">
        <Outlet />
      </main>
      <Toast />
    </div>
  );
}
