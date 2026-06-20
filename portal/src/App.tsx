import { usePortalStore } from './store/portalStore';
import { Login } from './components/Login';
import Layout from './components/Layout';
import Dashboard from './components/Dashboard';
import Banks from './components/Banks';
import Transactions from './components/Transactions';
import Settlement from './components/Settlement';
import Users from './components/Users';
import AuditLog from './components/AuditLog';

function App() {
  const activePage = usePortalStore((state) => state.activePage);
  const isAuthenticated = usePortalStore((state) => state.isAuthenticated);

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
