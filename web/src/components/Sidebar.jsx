import { NavLink } from 'react-router-dom';
import { LayoutDashboard, FolderTree, Users, UsersRound, Search, Layers, GitCompare, Shield, Settings, FileText, LogOut } from 'lucide-react';
import { useAuth } from '../context/AuthContext';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/users', icon: Users, label: 'Users' },
  { to: '/groups', icon: UsersRound, label: 'Groups' },
  { to: '/entries', icon: FolderTree, label: 'Entries' },
  { to: '/search', icon: Search, label: 'Search' },
  { to: '/bulk', icon: Layers, label: 'Bulk Operations' },
  { to: '/compare', icon: GitCompare, label: 'Compare' },
];

const adminItems = [
  { to: '/acl', icon: Shield, label: 'ACL Rules' },
  { to: '/config', icon: Settings, label: 'Config' },
  { to: '/logs', icon: FileText, label: 'Logs' },
];

export default function Sidebar() {
  const { user, logout } = useAuth();

  const extractUsername = (dn) => {
    if (!dn) return '';
    const match = dn.match(/^cn=([^,]+)/i);
    return match ? match[1] : dn;
  };

  const linkClass = ({ isActive }) =>
    `flex items-center gap-3 px-4 py-2 text-sm rounded-lg transition-colors ${
      isActive ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800/50'
    }`;

  return (
    <aside className="fixed left-0 top-0 h-screen w-64 bg-zinc-950 border-r border-zinc-800 flex flex-col">
      <div className="px-6 py-5 border-b border-zinc-800">
        <h1 className="text-xl font-semibold text-zinc-100">Oba Admin</h1>
      </div>

      <nav className="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
        {navItems.map(item => (
          <NavLink key={item.to} to={item.to} className={linkClass} end={item.to === '/'}>
            <item.icon className="w-5 h-5" />
            {item.label}
          </NavLink>
        ))}

        <div className="my-4 border-t border-zinc-800" />

        {adminItems.map(item => (
          <NavLink key={item.to} to={item.to} className={linkClass}>
            <item.icon className="w-5 h-5" />
            {item.label}
          </NavLink>
        ))}
      </nav>

      <div className="px-3 py-4 border-t border-zinc-800">
        <div className="px-4 py-2 mb-2">
          <p className="text-xs text-zinc-500">Logged in as</p>
          <p className="text-sm text-zinc-300 truncate">{extractUsername(user?.dn)}</p>
        </div>
        <button
          onClick={logout}
          className="flex items-center gap-3 px-4 py-2 w-full text-sm text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800/50 rounded-lg"
        >
          <LogOut className="w-5 h-5" />
          Logout
        </button>
      </div>
    </aside>
  );
}
