'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

const navItems = [
  { href: '/dashboard', label: 'Dashboard', icon: '📊' },
  { href: '/api-keys', label: 'API Keys', icon: '🔑' },
  { href: '/transactions', label: 'Transactions', icon: '💳' },
  { href: '/webhooks', label: 'Webhooks', icon: '🔔' },
  { href: '/subscriptions', label: 'Subscriptions', icon: '📦' },
  { href: '/invoices', label: 'Invoices', icon: '📄' },
  { href: '/admin', label: 'Admin', icon: '⚙️' },
];

export function Sidebar() {
  const pathname = usePathname();

  const handleLogout = () => {
    localStorage.removeItem('token');
    window.location.href = '/login';
  };

  return (
    <nav aria-label="Main navigation"
      className="w-64 bg-gray-900 text-white min-h-screen p-4 flex flex-col">
      <div className="mb-8">
        <h1 className="text-xl font-bold">Payment Platform</h1>
      </div>
      <ul className="flex-1 space-y-1" role="list">
        {navItems.map((item) => {
          const active = pathname === item.href;
          return (
            <li key={item.href}>
              <Link href={item.href}
                aria-current={active ? 'page' : undefined}
                className={`flex items-center gap-3 px-3 py-2 rounded text-sm ${
                  active ? 'bg-gray-700 text-white' : 'text-gray-300 hover:bg-gray-800'
                }`}>
                <span aria-hidden="true">{item.icon}</span>
                {item.label}
              </Link>
            </li>
          );
        })}
      </ul>
      <button
        onClick={handleLogout}
        className="mt-auto px-3 py-2 text-sm text-gray-300 hover:bg-gray-800 rounded w-full text-left"
      >
        Logout
      </button>
    </nav>
  );
}
