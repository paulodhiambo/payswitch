import React, { useState, useMemo } from 'react';
import { usePortalStore } from '../store/portalStore';
import {
  UserPlus,
  ShieldAlert,
  Mail,
  Edit3,
  Building
} from 'lucide-react';
import { generateUUID } from '../api/mockData';

type UserRole = 'SWITCH_ADMIN' | 'SWITCH_OPS' | 'BANK_ADMIN' | 'BANK_OPERATOR' | 'BANK_VIEWER';

interface MockUser {
  id: string;
  email: string;
  role: UserRole;
  participantId: string | null; // BIC or null for switch staff
  status: 'ACTIVE' | 'SUSPENDED';
  createdAt: string;
}

// BIC → display name for the participant scope column
const BIC_NAMES: Record<string, string> = {
  BANKUS33: 'First National Bank',
  EQBLKENA: 'Equity Bank Kenya',
  KCBLKENX: 'KCB Bank Kenya',
  SCBLKENA: 'Standard Chartered Kenya',
  ABSAKENA: 'Absa Bank Kenya',
  COOPKENA: 'Co-operative Bank of Kenya',
};

const INITIAL_USERS: MockUser[] = [
  { id: 'usr-admin-uuid',    email: 'admin@payment-switch.example.com',     role: 'SWITCH_ADMIN',    participantId: null,       status: 'ACTIVE', createdAt: '2025-11-20T10:00:00Z' },
  { id: 'usr-ops-uuid',      email: 'monitoring@payment-switch.example.com', role: 'SWITCH_OPS',      participantId: null,       status: 'ACTIVE', createdAt: '2025-11-22T08:00:00Z' },
  { id: 'usr-alice-uuid',    email: 'alice@equity.ke',                        role: 'BANK_ADMIN',      participantId: 'BANKUS33', status: 'ACTIVE', createdAt: '2026-01-16T09:00:00Z' },
  { id: 'usr-bob-uuid',      email: 'bob@equity.ke',                          role: 'BANK_OPERATOR',   participantId: 'BANKUS33', status: 'ACTIVE', createdAt: '2026-01-18T10:15:00Z' },
  { id: 'usr-david-uuid',    email: 'david@equity.ke',                        role: 'BANK_VIEWER',     participantId: 'BANKUS33', status: 'ACTIVE', createdAt: '2026-01-20T11:00:00Z' },
  { id: 'usr-kcb-admin-uuid',email: 'jane@kcb.ke',                            role: 'BANK_ADMIN',      participantId: 'EQBLKENA', status: 'ACTIVE', createdAt: '2026-02-11T08:00:00Z' },
  { id: 'usr-kcb-ops-uuid',  email: 'mike@kcb.ke',                            role: 'BANK_OPERATOR',   participantId: 'EQBLKENA', status: 'ACTIVE', createdAt: '2026-02-12T09:00:00Z' },
];

const getRoleBadge = (role: UserRole) => {
  switch (role) {
    case 'SWITCH_ADMIN':   return 'bg-violet-950/60 text-violet-400 border border-violet-900/40';
    case 'SWITCH_OPS':     return 'bg-blue-950/60 text-blue-400 border border-blue-900/40';
    case 'BANK_ADMIN':     return 'bg-emerald-950/60 text-emerald-400 border border-emerald-900/40';
    case 'BANK_OPERATOR':  return 'bg-amber-950/60 text-amber-400 border border-amber-900/40';
    case 'BANK_VIEWER':    return 'bg-slate-800/80 text-slate-300 border border-slate-700/50';
  }
};

export const Users: React.FC = () => {
  const { currentRole, currentParticipantId } = usePortalStore();
  const [users, setUsers] = useState<MockUser[]>(INITIAL_USERS);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);

  const [createForm, setCreateForm] = useState({
    email: '',
    role: 'BANK_VIEWER' as UserRole,
    participantId: currentParticipantId ?? '',
  });
  const [editRole, setEditRole] = useState<UserRole>('BANK_VIEWER');

  const isSwitchAdmin = currentRole === 'SWITCH_ADMIN';
  const isBankAdmin = currentRole === 'BANK_ADMIN';
  const activeUserToEdit = users.find(u => u.id === selectedUserId);

  const filteredUsers = useMemo(() => {
    if (isSwitchAdmin || currentRole === 'SWITCH_OPS') return users;
    return users.filter(u => u.participantId === currentParticipantId);
  }, [users, currentRole, currentParticipantId, isSwitchAdmin]);

  const resolveBankName = (participantId: string | null) => {
    if (!participantId) return 'Switch Authority';
    return BIC_NAMES[participantId] ?? participantId;
  };

  const handleCreateSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!createForm.email || !createForm.role) return;
    const newUser: MockUser = {
      id: generateUUID(),
      email: createForm.email,
      role: createForm.role,
      participantId: isBankAdmin ? (currentParticipantId ?? null) : (createForm.participantId || null),
      status: 'ACTIVE',
      createdAt: new Date().toISOString(),
    };
    setUsers(prev => [...prev, newUser]);
    setCreateForm({ email: '', role: isBankAdmin ? 'BANK_VIEWER' : 'SWITCH_ADMIN', participantId: currentParticipantId ?? '' });
    setShowCreateModal(false);
  };

  const handleEditSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedUserId || !editRole) return;
    setUsers(prev => prev.map(u => u.id === selectedUserId ? { ...u, role: editRole } : u));
    setShowEditModal(false);
    setSelectedUserId(null);
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h2 className="text-xl font-bold text-white">Portal User Directory</h2>
          <p className="text-gray-400 text-sm">Provision access, configure roles, and inspect group memberships via Authentik APIs.</p>
        </div>
        {(isSwitchAdmin || isBankAdmin) && (
          <button
            onClick={() => {
              setCreateForm(prev => ({
                ...prev,
                role: isBankAdmin ? 'BANK_VIEWER' : 'SWITCH_ADMIN',
                participantId: currentParticipantId ?? ''
              }));
              setShowCreateModal(true);
            }}
            className="flex items-center gap-2 bg-gradient-to-r from-indigo-600 to-violet-600 hover:from-indigo-500 hover:to-violet-500 px-4 py-2 rounded-lg text-sm font-semibold text-white shadow-lg glow-indigo transition-all duration-200"
          >
            <UserPlus className="h-4 w-4" />
            <span>Create User Invitation</span>
          </button>
        )}
      </div>

      <div className="glass-panel rounded-2xl border border-slate-800/60 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm border-collapse">
            <thead>
              <tr className="bg-slate-900/60 border-b border-slate-800/80 text-xs font-mono text-gray-400 uppercase">
                <th className="py-4 px-5">User Account Email</th>
                <th className="py-4 px-5">Participant Scope</th>
                <th className="py-4 px-5">Assigned Group Role</th>
                <th className="py-4 px-5">Status</th>
                <th className="py-4 px-5">Created At</th>
                {(isSwitchAdmin || isBankAdmin) && <th className="py-4 px-5 text-right">Actions</th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {filteredUsers.map((u) => (
                <tr key={u.id} className="hover:bg-slate-800/10">
                  <td className="py-3.5 px-5 font-medium text-white">
                    <div className="flex items-center gap-2.5">
                      <Mail className="h-4 w-4 text-gray-500 flex-shrink-0" />
                      <span>{u.email}</span>
                    </div>
                  </td>
                  <td className="py-3.5 px-5 text-xs text-gray-300 font-mono">
                    {resolveBankName(u.participantId)}
                  </td>
                  <td className="py-3.5 px-5">
                    <span className={`px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider ${getRoleBadge(u.role)}`}>
                      {u.role.replace('_', ' ')}
                    </span>
                  </td>
                  <td className="py-3.5 px-5">
                    <span className="flex items-center gap-1.5 text-xs text-emerald-400 font-mono">
                      <span className="h-1.5 w-1.5 rounded-full bg-emerald-400"></span>
                      <span>{u.status}</span>
                    </span>
                  </td>
                  <td className="py-3.5 px-5 text-xs text-gray-400 font-mono">
                    {new Date(u.createdAt).toLocaleDateString()}
                  </td>
                  {(isSwitchAdmin || isBankAdmin) && (
                    <td className="py-3.5 px-5 text-right">
                      {(isSwitchAdmin || (isBankAdmin && u.participantId === currentParticipantId && u.role !== 'BANK_ADMIN')) && (
                        <button
                          onClick={() => { setSelectedUserId(u.id); setEditRole(u.role); setShowEditModal(true); }}
                          className="text-gray-400 hover:text-white p-1 rounded hover:bg-slate-800/40 transition-colors"
                          title="Modify Role Group"
                        >
                          <Edit3 className="h-4 w-4 text-indigo-400" />
                        </button>
                      )}
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-fade-in">
          <div className="glass-panel w-full max-w-md p-6 rounded-2xl border border-slate-800 shadow-2xl space-y-4">
            <h3 className="text-base font-bold text-white">Create Portal User Invitation</h3>
            <form onSubmit={handleCreateSubmit} className="space-y-4">
              <div className="space-y-1">
                <label className="text-xs font-semibold text-gray-400">User Email Address</label>
                <input
                  type="email"
                  required
                  placeholder="e.g. employee@equity.ke"
                  value={createForm.email}
                  onChange={(e) => setCreateForm({ ...createForm, email: e.target.value })}
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500"
                />
              </div>

              {isSwitchAdmin ? (
                <div className="space-y-1">
                  <label className="text-xs font-semibold text-gray-400">Participant Scope Link</label>
                  <select
                    value={createForm.participantId}
                    onChange={(e) => setCreateForm({ ...createForm, participantId: e.target.value })}
                    className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500"
                  >
                    <option value="">Switch System Staff (Global Scope)</option>
                    {Object.entries(BIC_NAMES).map(([bic, name]) => (
                      <option key={bic} value={bic}>{name} ({bic})</option>
                    ))}
                  </select>
                </div>
              ) : (
                <div className="space-y-1 bg-slate-900/60 border border-slate-800 p-2.5 rounded-lg text-xs flex items-center gap-2 text-gray-400">
                  <Building className="h-4 w-4 text-indigo-400" />
                  <span>Participant Scope pre-locked to: <b>{resolveBankName(currentParticipantId)}</b></span>
                </div>
              )}

              <div className="space-y-1">
                <label className="text-xs font-semibold text-gray-400">Assigned Access Role</label>
                <select
                  value={createForm.role}
                  onChange={(e) => setCreateForm({ ...createForm, role: e.target.value as UserRole })}
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500"
                >
                  {isSwitchAdmin && (
                    <>
                      <option value="SWITCH_ADMIN">SWITCH_ADMIN (Full ops)</option>
                      <option value="SWITCH_OPS">SWITCH_OPS (Global read-only)</option>
                      <option value="BANK_ADMIN">BANK_ADMIN (Manage Own Bank)</option>
                    </>
                  )}
                  <option value="BANK_OPERATOR">BANK_OPERATOR (Transfer reports, view ledger)</option>
                  <option value="BANK_VIEWER">BANK_VIEWER (Read-only ledger)</option>
                </select>
              </div>

              <div className="flex justify-end gap-3 pt-4 border-t border-slate-800/40">
                <button type="button" onClick={() => setShowCreateModal(false)} className="px-4 py-2 rounded-lg text-sm font-semibold text-gray-400 hover:text-white">Cancel</button>
                <button type="submit" className="bg-indigo-600 hover:bg-indigo-500 px-4 py-2 rounded-lg text-sm font-semibold text-white shadow-lg shadow-indigo-600/30">Send Invite Email</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {showEditModal && activeUserToEdit && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-fade-in">
          <div className="glass-panel w-full max-w-sm p-6 rounded-2xl border border-slate-800 shadow-2xl space-y-4">
            <div>
              <h3 className="text-base font-bold text-white">Modify User Access Role</h3>
              <p className="text-xs text-gray-400">Update group memberships for: <b>{activeUserToEdit.email}</b></p>
            </div>
            <form onSubmit={handleEditSubmit} className="space-y-4">
              <div className="space-y-1">
                <label className="text-xs font-semibold text-gray-400">Role Group Assignment</label>
                <select
                  value={editRole}
                  onChange={(e) => setEditRole(e.target.value as UserRole)}
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500"
                >
                  {isSwitchAdmin && (
                    <>
                      <option value="SWITCH_ADMIN">SWITCH_ADMIN</option>
                      <option value="SWITCH_OPS">SWITCH_OPS</option>
                      <option value="BANK_ADMIN">BANK_ADMIN</option>
                    </>
                  )}
                  <option value="BANK_OPERATOR">BANK_OPERATOR</option>
                  <option value="BANK_VIEWER">BANK_VIEWER</option>
                </select>
              </div>

              {isBankAdmin && editRole === 'BANK_ADMIN' && (
                <div className="text-[10px] text-rose-400 flex items-center gap-1">
                  <ShieldAlert className="h-3 w-3" />
                  <span>Escalation to BANK_ADMIN is forbidden.</span>
                </div>
              )}

              <div className="flex justify-end gap-3 pt-4 border-t border-slate-800/40">
                <button type="button" onClick={() => { setShowEditModal(false); setSelectedUserId(null); }} className="px-4 py-2 rounded-lg text-sm font-semibold text-gray-400 hover:text-white">Cancel</button>
                <button
                  type="submit"
                  disabled={isBankAdmin && editRole === 'BANK_ADMIN'}
                  className="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:hover:bg-indigo-600 px-4 py-2 rounded-lg text-sm font-semibold text-white shadow-lg"
                >
                  Commit Role Update
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
export default Users;
