import axios from 'axios';

export interface DevIdentity {
  sub: string;
  username: string;
  role: string;
  participantId: string;
}

// Mutated by the store's login/logout to inject Authentik-style headers in dev.
// In production, these headers are injected by the Authentik reverse proxy upstream.
export const devIdentity: DevIdentity = {
  sub: '',
  username: '',
  role: '',
  participantId: '',
};

let csrfToken = '';

export function setCsrfToken(t: string): void {
  csrfToken = t;
}

// When served through Kong (/portal/*), use /portal/api/v1.
// In dev mode (vite proxy on localhost:5173), use /api/v1.
const baseURL = window.location.pathname.startsWith('/portal')
  ? '/portal/api/v1'
  : '/api/v1';

export const apiClient = axios.create({
  baseURL,
  headers: { 'Content-Type': 'application/json' },
});

apiClient.interceptors.request.use((config) => {
  if (devIdentity.sub) {
    config.headers['X-authentik-uid'] = devIdentity.sub;
    config.headers['X-authentik-username'] = devIdentity.username;
    config.headers['X-User-Role'] = devIdentity.role;
    if (devIdentity.participantId) {
      config.headers['X-Participant-Id'] = devIdentity.participantId;
    }
  }
  const method = config.method?.toLowerCase() ?? '';
  if (csrfToken && !['get', 'head', 'options'].includes(method)) {
    config.headers['X-CSRF-Token'] = csrfToken;
  }
  return config;
});
