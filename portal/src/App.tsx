import { useEffect, useRef, useState } from 'react';
import { usePortalStore } from './store/portalStore';
import { Login } from './components/Login';
import Layout from './components/Layout';
import Dashboard from './components/Dashboard';
import Banks from './components/Banks';
import Transactions from './components/Transactions';
import Settlement from './components/Settlement';
import Users from './components/Users';
import AuditLog from './components/AuditLog';
import { api } from './api/api';
import { setCsrfToken } from './api/client';

function App() {
  const activePage = usePortalStore((state) => state.activePage);
  const isAuthenticated = usePortalStore((state) => state.isAuthenticated);
  const [isInitializing, setIsInitializing] = useState(true);
  // Track whether we've already pushed the sentinel history entry.
  const historyGuardActive = useRef(false);

  useEffect(() => {
    const initAuth = async () => {
      try {
        const token = await api.getCsrfToken();
        setCsrfToken(token);
        const me = await api.getMe();
        
        // Populate store with backend claims
        usePortalStore.setState({
          isAuthenticated: true,
          currentRole: me.role,
          currentParticipantId: me.participantId || null,
          currentBankId: me.bankId || null,
          currentBankName: me.bankName || null,
          currentUserEmail: me.username + '@switch.local',
          activePage: 'dashboard',
          activeSubPage: 'list',
          activeEntityId: null,
        });
      } catch (err) {
        // Failed: not authenticated or in direct dev mode (forces manual login)
      } finally {
        setIsInitializing(false);
      }
    };
    initAuth();
  }, []);

  // Back-button guard: prevent authenticated users from navigating back to login.
  useEffect(() => {
    if (!isAuthenticated) {
      historyGuardActive.current = false;
      return;
    }

    // Push a sentinel entry so the user has something to go "forward" to;
    // this also means pressing Back from the dashboard lands on this entry
    // (same URL), so a popstate fires but we stay on the app.
    if (!historyGuardActive.current) {
      window.history.pushState({ appState: 'authenticated' }, '', window.location.href);
      historyGuardActive.current = true;
    }

    const handlePopState = () => {
      // If the user is still authenticated and presses back, push them
      // forward again — they cannot leave the app via the browser back button.
      const currentlyAuthenticated = usePortalStore.getState().isAuthenticated;
      if (currentlyAuthenticated) {
        window.history.pushState({ appState: 'authenticated' }, '', window.location.href);
      }
    };

    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, [isAuthenticated]);

  if (isInitializing) {
    return (
      <div className="min-h-screen bg-[#06080e] flex items-center justify-center">
        <div className="text-gray-400 font-mono text-xs animate-pulse">Initializing Switch Console...</div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Login />;
  }

  const renderPage = () => {
    switch (activePage) {
      case 'dashboard':
        return <Dashboard />;
      case 'banks':
        return <Banks />;
      case 'transactions':
        return <Transactions />;
      case 'settlement':
        return <Settlement />;
      case 'users':
        return <Users />;
      case 'audit-log':
        return <AuditLog />;
      default:
        return <Dashboard />;
    }
  };

  return <Layout>{renderPage()}</Layout>;
}

export default App;
