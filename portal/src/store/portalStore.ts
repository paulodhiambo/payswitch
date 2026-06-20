import { create } from 'zustand';
import { devIdentity, setCsrfToken } from '../api/client';
import { api } from '../api/api';

type Page = 'dashboard' | 'banks' | 'transactions' | 'settlement' | 'users' | 'audit-log';
type SubPage = 'list' | 'detail' | 'onboarding';

interface PortalState {
  activePage: Page;
  activeSubPage: SubPage;
  activeEntityId: string | null;

  currentRole: string;
  currentParticipantId: string | null;  // BIC for bank roles; null for switch roles
  currentBankId: string | null;          // UUID of the participant's bank (for API calls)
  currentBankName: string | null;
  currentUserEmail: string | null;
  isSidebarOpen: boolean;
  isAuthenticated: boolean;

  navigate: (page: Page, subPage?: SubPage, entityId?: string | null) => void;
  toggleSidebar: () => void;
  login: (email: string, password: string) => Promise<boolean>;
  logout: () => void;
}

// Maps demo email → Authentik-style identity injected as request headers in dev.
// In production, Authentik populates these headers from its session cookie upstream.
const DEMO_ACCOUNTS: Record<string, { sub: string; username: string; role: string; participantId: string }> = {
  'admin@payment-switch.example.com':     { sub: 'dev-admin', username: 'admin', role: 'SWITCH_ADMIN',    participantId: '' },
  'monitoring@payment-switch.example.com':{ sub: 'dev-ops',   username: 'ops',   role: 'SWITCH_OPS',      participantId: '' },
  'alice@equity.ke':                       { sub: 'dev-alice', username: 'alice', role: 'BANK_ADMIN',      participantId: 'BANKUS33' },
  'bob@equity.ke':                         { sub: 'dev-bob',   username: 'bob',   role: 'BANK_OPERATOR',   participantId: 'BANKUS33' },
  'david@equity.ke':                       { sub: 'dev-david', username: 'david', role: 'BANK_VIEWER',     participantId: 'BANKUS33' },
};

const DEMO_PASSWORDS: Record<string, string> = {
  'admin@payment-switch.example.com':     'admin123',
  'monitoring@payment-switch.example.com':'ops123',
  'alice@equity.ke':                       'equity123',
  'bob@equity.ke':                         'operator123',
  'david@equity.ke':                       'viewer123',
};

export const usePortalStore = create<PortalState>((set) => ({
  activePage: 'dashboard',
  activeSubPage: 'list',
  activeEntityId: null,

  currentRole: 'SWITCH_ADMIN',
  currentParticipantId: null,
  currentBankId: null,
  currentBankName: null,
  currentUserEmail: null,
  isSidebarOpen: true,
  isAuthenticated: false,

  navigate: (page, subPage = 'list', entityId = null) => {
    set({ activePage: page, activeSubPage: subPage, activeEntityId: entityId });
  },

  toggleSidebar: () => set((s) => ({ isSidebarOpen: !s.isSidebarOpen })),

  login: async (email, password) => {
    const key = email.trim().toLowerCase();
    const creds = DEMO_ACCOUNTS[key];
    if (!creds || DEMO_PASSWORDS[key] !== password) {
      await new Promise(r => setTimeout(r, 500));
      return false;
    }

    devIdentity.sub = creds.sub;
    devIdentity.username = creds.username;
    devIdentity.role = creds.role;
    devIdentity.participantId = creds.participantId;

    try {
      const token = await api.getCsrfToken();
      setCsrfToken(token);

      const me = await api.getMe();
      set({
        isAuthenticated: true,
        currentRole: me.role,
        currentParticipantId: me.participantId || null,
        currentBankId: me.bankId || null,
        currentBankName: me.bankName || null,
        currentUserEmail: key,
        activePage: 'dashboard',
        activeSubPage: 'list',
        activeEntityId: null,
      });
      return true;
    } catch {
      devIdentity.sub = '';
      devIdentity.username = '';
      devIdentity.role = '';
      devIdentity.participantId = '';
      return false;
    }
  },

  logout: () => {
    devIdentity.sub = '';
    devIdentity.username = '';
    devIdentity.role = '';
    devIdentity.participantId = '';
    setCsrfToken('');
    set({
      isAuthenticated: false,
      currentRole: 'SWITCH_ADMIN',
      currentParticipantId: null,
      currentBankId: null,
      currentBankName: null,
      currentUserEmail: null,
      activePage: 'dashboard',
      activeSubPage: 'list',
      activeEntityId: null,
    });
  },
}));
