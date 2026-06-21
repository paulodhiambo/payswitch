import React from 'react';
import { usePortalStore } from '../store/portalStore';
import {
  LayoutDashboard,
  Building2,
  ArrowLeftRight,
  Coins,
  Users as UsersIcon,
  FileText,
  Menu,
  X,
  ChevronRight,
  User as UserIcon,
  LogOut
} from 'lucide-react';

interface LayoutProps {
  children: React.ReactNode;
}

export const Layout: React.FC<LayoutProps> = ({ children }) => {
  const {
    currentRole,
    currentParticipantId,
    currentBankName,
    isSidebarOpen,
    toggleSidebar,
    activePage,
    navigate,
    currentUserEmail,
    logout
  } = usePortalStore();

  const navItems = [
    { id: 'dashboard',   label: 'Dashboard',           icon: LayoutDashboard, roles: ['SWITCH_ADMIN', 'SWITCH_OPS', 'BANK_ADMIN', 'BANK_OPERATOR', 'BANK_VIEWER'] },
    { id: 'banks',       label: 'Participant Banks',    icon: Building2,       roles: ['SWITCH_ADMIN', 'SWITCH_OPS', 'BANK_ADMIN', 'BANK_OPERATOR', 'BANK_VIEWER'] },
    { id: 'transactions',label: 'Transactions Ledger',  icon: ArrowLeftRight,  roles: ['SWITCH_ADMIN', 'SWITCH_OPS', 'BANK_ADMIN', 'BANK_OPERATOR', 'BANK_VIEWER'] },
    { id: 'settlement',  label: 'Settlement Windows',   icon: Coins,           roles: ['SWITCH_ADMIN', 'SWITCH_OPS', 'BANK_ADMIN', 'BANK_OPERATOR'] },
    { id: 'users',       label: 'Users Manager',        icon: UsersIcon,       roles: ['SWITCH_ADMIN', 'BANK_ADMIN'] },
    { id: 'audit-log',   label: 'Audit Log',            icon: FileText,        roles: ['SWITCH_ADMIN'] },
  ];

  const visibleNavItems = navItems.filter(item => item.roles.includes(currentRole));
  const displayName = currentUserEmail || 'user@example.com';

  return (
    <div className="min-h-screen flex flex-col bg-[#080b13] text-[#f3f4f6]">
      {/* Dev Header Info Bar */}
      <div className="bg-gradient-to-r from-violet-950 via-slate-900 to-indigo-950 border-b border-violet-900/40 px-4 py-2 flex items-center justify-center text-xs font-mono text-violet-200">
        <div className="flex items-center gap-2">
          <span className="flex h-2 w-2 rounded-full bg-violet-400 animate-pulse"></span>
          <span>DEV SANDBOX MODE &bull; Proxy → localhost:8090</span>
        </div>
      </div>

      <div className="flex-1 flex overflow-hidden">
        {/* Sidebar */}
        <aside
          className={`glass-panel border-y-0 border-l-0 border-r border-[#1e293b]/40 flex flex-col transition-all duration-300 z-30
            ${isSidebarOpen ? 'w-64' : 'w-16'}`}
        >
          <div className="h-16 flex items-center justify-between px-4 border-b border-[#1e293b]/40">
            {isSidebarOpen ? (
              <div className="flex items-center gap-2">
                <div className="bg-gradient-to-br from-indigo-500 to-violet-600 p-2 rounded-lg glow-indigo">
                  <Coins className="h-5 w-5 text-white" />
                </div>
                <div className="flex flex-col">
                  <span className="font-bold tracking-wider text-sm bg-gradient-to-r from-indigo-200 to-violet-200 bg-clip-text text-transparent">PAYSYS SWITCH</span>
                  <span className="text-[10px] text-gray-400 font-mono tracking-widest uppercase">Admin Portal</span>
                </div>
              </div>
            ) : (
              <div className="mx-auto bg-gradient-to-br from-indigo-500 to-violet-600 p-2 rounded-lg">
                <Coins className="h-5 w-5 text-white" />
              </div>
            )}
            {isSidebarOpen && (
              <button
                onClick={toggleSidebar}
                className="text-gray-400 hover:text-white p-1 rounded-md hover:bg-slate-800/40"
              >
                <X className="h-4 w-4" />
              </button>
            )}
          </div>

          <nav className="flex-1 px-3 py-4 space-y-1">
            {visibleNavItems.map((item) => {
              const Icon = item.icon;
              const isActive = activePage === item.id;
              return (
                <button
                  key={item.id}
                  onClick={() => navigate(item.id as any)}
                  className={`w-full flex items-center px-3 py-2.5 rounded-lg text-sm font-medium transition-all group duration-200
                    ${isActive
                      ? 'bg-gradient-to-r from-indigo-600/20 to-violet-600/20 text-indigo-400 border-l-2 border-indigo-500'
                      : 'text-gray-400 hover:text-gray-200 hover:bg-slate-800/20'}`}
                >
                  <Icon className={`h-5 w-5 flex-shrink-0 ${isActive ? 'text-indigo-400' : 'text-gray-400 group-hover:text-gray-200'}`} />
                  {isSidebarOpen && (
                    <span className="ml-3 transition-opacity duration-200">{item.label}</span>
                  )}
                  {isSidebarOpen && isActive && (
                    <ChevronRight className="ml-auto h-4 w-4 text-indigo-400/80 animate-pulse" />
                  )}
                </button>
              );
            })}
          </nav>

          <div className="p-4 border-t border-[#1e293b]/40">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3 min-w-0">
                <div className="h-9 w-9 rounded-full bg-slate-800 flex items-center justify-center border border-slate-700 flex-shrink-0">
                  <UserIcon className="h-4 w-4 text-indigo-400" />
                </div>
                {isSidebarOpen && (
                  <div className="flex-1 min-w-0">
                    <p className="text-xs font-semibold text-gray-200 truncate" title={displayName}>{displayName}</p>
                    <p className="text-[10px] text-gray-400 uppercase font-mono truncate">
                      {currentBankName || 'Switch Authority'}
                    </p>
                  </div>
                )}
              </div>
              {isSidebarOpen && (
                <button
                  onClick={logout}
                  className="text-gray-400 hover:text-rose-400 p-1.5 rounded-md hover:bg-slate-800/40 transition-colors flex-shrink-0 animate-fade-in"
                  title="Sign Out"
                >
                  <LogOut className="h-4 w-4" />
                </button>
              )}
            </div>
            {!isSidebarOpen && (
              <button
                onClick={logout}
                className="mt-3 mx-auto flex h-9 w-9 items-center justify-center text-gray-400 hover:text-rose-400 rounded-md hover:bg-slate-800/40 transition-colors animate-fade-in"
                title="Sign Out"
              >
                <LogOut className="h-4 w-4" />
              </button>
            )}
          </div>
        </aside>

        <div className="flex-1 flex flex-col min-w-0 overflow-y-auto">
          <header className="h-16 flex items-center justify-between px-6 border-b border-[#1e293b]/40 bg-[#080b13]/80 backdrop-blur-md sticky top-0 z-20">
            <div className="flex items-center gap-3">
              {!isSidebarOpen && (
                <button
                  onClick={toggleSidebar}
                  className="text-gray-400 hover:text-white p-1 rounded-md hover:bg-slate-800/40 mr-2"
                >
                  <Menu className="h-5 w-5" />
                </button>
              )}
              <h1 className="text-lg font-bold tracking-tight text-white capitalize">
                {activePage.replace('-', ' ')}
              </h1>
            </div>

            <div className="flex items-center gap-4">
              <div className="hidden md:flex items-center gap-2 bg-slate-900/60 border border-slate-800 px-3 py-1.5 rounded-full text-xs text-gray-400 font-mono">
                <span className="text-indigo-400">Headers:</span>
                <span>Role={currentRole}</span>
                {currentParticipantId && (
                  <>
                    <span className="text-slate-600">|</span>
                    <span>BIC={currentParticipantId}</span>
                  </>
                )}
              </div>

              <div className={`px-2.5 py-1 rounded-full text-xs font-medium flex items-center gap-1.5
                ${currentParticipantId ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-900/40' : 'bg-indigo-950/60 text-indigo-400 border border-indigo-900/40'}`}>
                <span className="h-1.5 w-1.5 rounded-full bg-current"></span>
                {currentParticipantId ? `${currentParticipantId} Active` : 'Switch System'}
              </div>
            </div>
          </header>

          <main className="flex-1 p-6 md:p-8 max-w-7xl w-full mx-auto animate-fade-in">
            {children}
          </main>
        </div>
      </div>
    </div>
  );
};
export default Layout;
