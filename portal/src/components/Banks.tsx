import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { usePortalStore } from '../store/portalStore';
import { api } from '../api/api';
import type { Bank, Certificate } from '../api/types';
import {
  Building2,
  ShieldCheck,
  ShieldAlert,
  Plus,
  Trash2,
  Upload,
  AlertTriangle,
  ArrowLeft,
  CheckCircle,
  FileCheck2
} from 'lucide-react';

const STATUS_ORDER: Bank['status'][] = ['APPLICATION', 'SANDBOX', 'CERTIFICATION', 'PRODUCTION_ACTIVE'];

const getStatusBadgeClass = (status: Bank['status']) => {
  switch (status) {
    case 'PRODUCTION_ACTIVE': return 'bg-emerald-950/60 text-emerald-400 border border-emerald-900/40';
    case 'SUSPENDED':         return 'bg-rose-950/60 text-rose-400 border border-rose-900/40';
    case 'CERTIFICATION':     return 'bg-indigo-950/60 text-indigo-400 border border-indigo-900/40';
    case 'SANDBOX':           return 'bg-amber-950/60 text-amber-400 border border-amber-900/40';
    case 'APPLICATION':       return 'bg-slate-800/80 text-slate-300 border border-slate-700/50';
    case 'DECOMMISSIONED':    return 'bg-gray-900/80 text-gray-500 border border-gray-800';
  }
};

export const Banks: React.FC = () => {
  const { currentRole, currentBankId } = usePortalStore();
  const queryClient = useQueryClient();

  const isSwitchAdmin = currentRole === 'SWITCH_ADMIN';
  const isBankAdmin = currentRole === 'BANK_ADMIN';
  const isBankRole = currentRole.startsWith('BANK_');

  const [selectedBankId, setSelectedBankId] = useState<string | null>(null);
  const [showOnboardModal, setShowOnboardModal] = useState(false);
  const [newBankForm, setNewBankForm] = useState({ bic: '', name: '', country: 'US', settlementAccount: '', notes: '' });
  const [pemInput, setPemInput] = useState('');
  const [showSuspendModal, setShowSuspendModal] = useState(false);
  const [suspendReason, setSuspendReason] = useState('');

  // For bank roles, auto-use their bank; for switch admins, use click selection
  const activeBankId = isBankRole ? currentBankId : selectedBankId;

  // List of all banks (switch roles only)
  const { data: banksResult, isLoading: banksLoading } = useQuery({
    queryKey: ['banks'],
    queryFn: () => api.listBanks({ pageSize: 100 }),
    enabled: !isBankRole,
  });
  const banks = banksResult?.data ?? [];

  // Detail for the active bank
  const { data: activeBank, isLoading: bankLoading } = useQuery({
    queryKey: ['bank', activeBankId],
    queryFn: () => api.getBank(activeBankId!),
    enabled: !!activeBankId,
  });

  // Certificates for the active bank
  const { data: activeCerts = [] } = useQuery({
    queryKey: ['certs', activeBankId],
    queryFn: () => api.listCertificates(activeBankId!),
    enabled: !!activeBankId,
  });

  const canManageCert = isSwitchAdmin || (isBankAdmin && activeBankId === currentBankId);

  // Mutations
  const createBankMut = useMutation({
    mutationFn: api.createBank,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['banks'] }); setShowOnboardModal(false); setNewBankForm({ bic: '', name: '', country: 'US', settlementAccount: '', notes: '' }); },
  });

  const updateStatusMut = useMutation({
    mutationFn: ({ bankId, status, reason }: { bankId: string; status: string; reason?: string }) =>
      api.updateBankStatus(bankId, status, reason),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['bank', activeBankId] }); queryClient.invalidateQueries({ queryKey: ['banks'] }); },
  });

  const registerCertMut = useMutation({
    mutationFn: ({ bankId, pem }: { bankId: string; pem: string }) => api.createCertificate(bankId, pem),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['certs', activeBankId] }); setPemInput(''); },
  });

  const revokeCertMut = useMutation({
    mutationFn: ({ bankId, certId }: { bankId: string; certId: string }) => api.revokeCertificate(bankId, certId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['certs', activeBankId] }),
  });

  const handleOnboardSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newBankForm.bic || !newBankForm.name || !newBankForm.settlementAccount) return;
    createBankMut.mutate(newBankForm);
  };

  const handleRegisterCert = (e: React.FormEvent) => {
    e.preventDefault();
    if (!activeBankId || !pemInput.trim()) return;
    registerCertMut.mutate({ bankId: activeBankId, pem: pemInput });
  };

  const promoteStatus = () => {
    if (!activeBank || !activeBankId) return;
    const idx = STATUS_ORDER.indexOf(activeBank.status as Bank['status']);
    if (idx !== -1 && idx < STATUS_ORDER.length - 1) {
      updateStatusMut.mutate({ bankId: activeBankId, status: STATUS_ORDER[idx + 1] });
    }
  };

  const handleSuspend = () => {
    if (!activeBankId || !suspendReason.trim()) return;
    updateStatusMut.mutate({ bankId: activeBankId, status: 'SUSPENDED', reason: suspendReason });
    setSuspendReason('');
    setShowSuspendModal(false);
  };

  const handleResume = () => {
    if (!activeBankId) return;
    updateStatusMut.mutate({ bankId: activeBankId, status: 'PRODUCTION_ACTIVE' });
  };

  // Detail view for a specific bank
  if (activeBankId) {
    if (bankLoading) {
      return <div className="flex items-center justify-center h-64 text-gray-400 text-sm font-mono animate-pulse">Loading bank...</div>;
    }
    if (!activeBank) {
      return <div className="text-gray-400 text-sm font-mono p-8">Bank not found.</div>;
    }

    const isSuspended = activeBank.status === 'SUSPENDED';

    return (
      <div className="space-y-6">
        {!isBankRole && (
          <button
            onClick={() => setSelectedBankId(null)}
            className="flex items-center gap-2 text-sm text-gray-400 hover:text-white transition-colors duration-150"
          >
            <ArrowLeft className="h-4 w-4" />
            <span>Back to participants ledger</span>
          </button>
        )}

        {/* Header */}
        <div className="glass-panel p-6 rounded-2xl flex flex-col md:flex-row md:items-center justify-between gap-6">
          <div className="flex items-center gap-4">
            <div className="bg-gradient-to-br from-indigo-500 to-violet-600 p-3.5 rounded-2xl text-white glow-indigo">
              <Building2 className="h-8 w-8" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h2 className="text-2xl font-bold text-white leading-tight">{activeBank.name}</h2>
                <span className={`px-2.5 py-0.5 rounded-full text-xs font-semibold uppercase tracking-wider ${getStatusBadgeClass(activeBank.status as Bank['status'])}`}>
                  {activeBank.status.replace(/_/g, ' ')}
                </span>
              </div>
              <p className="text-sm font-mono text-gray-400 tracking-wider mt-1">
                BIC: {activeBank.bic} &bull; Country: {activeBank.country} &bull; Account: <span className="text-indigo-300 font-semibold">{activeBank.settlementAccount}</span>
              </p>
            </div>
          </div>

          <div className="flex gap-3 flex-wrap">
            {isSwitchAdmin && activeBank.status !== 'PRODUCTION_ACTIVE' && activeBank.status !== 'SUSPENDED' && activeBank.status !== 'DECOMMISSIONED' && (
              <button
                onClick={promoteStatus}
                disabled={updateStatusMut.isPending}
                className="flex items-center gap-1.5 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs px-4 py-2 rounded-lg transition-colors shadow disabled:opacity-50"
              >
                <FileCheck2 className="h-4 w-4" />
                <span>Promote Status</span>
              </button>
            )}
            {isSwitchAdmin && activeBank.status === 'PRODUCTION_ACTIVE' && (
              <button
                onClick={() => setShowSuspendModal(true)}
                className="flex items-center gap-1.5 bg-rose-600 hover:bg-rose-500 text-white font-semibold text-xs px-4 py-2 rounded-lg transition-colors shadow"
              >
                <ShieldAlert className="h-4 w-4" />
                <span>Suspend Participant</span>
              </button>
            )}
            {isSwitchAdmin && isSuspended && (
              <button
                onClick={handleResume}
                disabled={updateStatusMut.isPending}
                className="flex items-center gap-1.5 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs px-4 py-2 rounded-lg transition-colors shadow disabled:opacity-50"
              >
                <ShieldCheck className="h-4 w-4" />
                <span>Resume Operational Status</span>
              </button>
            )}
          </div>
        </div>

        {/* Onboarding Wizard */}
        <div className="glass-panel p-6 rounded-2xl">
          <h3 className="text-sm font-mono uppercase tracking-wider text-gray-400 mb-6">Onboarding Progress</h3>
          <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
            {STATUS_ORDER.map((step, index) => {
              const currentIdx = STATUS_ORDER.indexOf(activeBank.status as Bank['status']);
              const isCompleted = currentIdx >= index || activeBank.status === 'SUSPENDED';
              const isActive = activeBank.status === step;
              return (
                <div
                  key={step}
                  onClick={() => isSwitchAdmin && activeBankId && updateStatusMut.mutate({ bankId: activeBankId, status: step })}
                  className={`flex flex-col items-center text-center relative group ${isSwitchAdmin ? 'cursor-pointer' : ''}`}
                >
                  <div className={`h-10 w-10 rounded-full flex items-center justify-center border font-semibold text-sm z-10 transition-all duration-300
                    ${isActive
                      ? 'bg-indigo-600 text-white border-indigo-400 scale-110 shadow-lg shadow-indigo-600/40 animate-pulse-subtle'
                      : isCompleted
                        ? 'bg-emerald-950/80 text-emerald-400 border-emerald-500/60'
                        : 'bg-[#0f172a] text-gray-500 border-slate-800'}`}>
                    {isCompleted ? <CheckCircle className="h-5 w-5" /> : index + 1}
                  </div>
                  <span className={`text-xs font-bold mt-3 tracking-wide
                    ${isActive ? 'text-indigo-400' : isCompleted ? 'text-emerald-400' : 'text-gray-500'}`}>
                    {step.replace(/_/g, ' ')}
                  </span>
                  <p className="text-[10px] text-gray-400 max-w-[120px] mt-1">
                    {step === 'APPLICATION' && 'Registry initiated & verified.'}
                    {step === 'SANDBOX' && 'ISO20022 message testing.'}
                    {step === 'CERTIFICATION' && 'Compliance test scenarios.'}
                    {step === 'PRODUCTION_ACTIVE' && 'Live interbank gateway.'}
                  </p>
                </div>
              );
            })}
          </div>
        </div>

        {/* Certificates */}
        <div className="glass-panel p-6 rounded-2xl flex flex-col justify-between space-y-4">
          <div>
            <h3 className="text-base font-bold text-white">mTLS Certificate Authority</h3>
            <p className="text-xs text-gray-400">Registered X.509 certificates used by Kong to authorize payment gateway requests.</p>
          </div>

          <div className="space-y-3">
            {activeCerts.map((cert: Certificate) => (
              <div key={cert.id} className="bg-slate-900/60 border border-slate-800 rounded-xl p-4 space-y-3 relative overflow-hidden group">
                <div className="flex justify-between items-start">
                  <div className="space-y-0.5">
                    <span className={`px-2 py-0.5 rounded-full text-[9px] font-semibold tracking-wider uppercase
                      ${cert.status === 'ACTIVE'
                        ? 'bg-emerald-950/40 text-emerald-400 border border-emerald-900/30'
                        : 'bg-rose-950/40 text-rose-400 border border-rose-900/30'}`}>
                      {cert.status}
                    </span>
                    <p className="text-xs text-gray-200 font-mono truncate max-w-[320px] pt-1">{cert.subject}</p>
                  </div>
                  {canManageCert && cert.status === 'ACTIVE' && (
                    <button
                      onClick={() => activeBankId && revokeCertMut.mutate({ bankId: activeBankId, certId: cert.id })}
                      disabled={revokeCertMut.isPending}
                      className="text-gray-500 hover:text-rose-400 p-1.5 rounded-md hover:bg-slate-800/40 transition-colors disabled:opacity-50"
                      title="Revoke Certificate"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  )}
                </div>
                <div className="text-[10px] text-gray-500 space-y-0.5 font-mono">
                  <p className="truncate">SHA-256: {cert.fingerprint}</p>
                  <p>Expires: {new Date(cert.notAfter).toLocaleDateString()}</p>
                </div>
              </div>
            ))}

            {activeCerts.length === 0 && (
              <div className="flex flex-col items-center justify-center p-8 text-center space-y-2 border border-dashed border-slate-800 rounded-xl text-gray-500">
                <AlertTriangle className="h-8 w-8 text-amber-500" />
                <p className="text-xs font-semibold">No Certificates Registered</p>
                <p className="text-[10px]">Payment requests will be rejected by Kong at the gateway.</p>
              </div>
            )}
          </div>

          {canManageCert && (
            <form onSubmit={handleRegisterCert} className="space-y-3 pt-4 border-t border-slate-800/40">
              <label className="text-xs font-mono text-gray-400">Register PEM Certificate Block</label>
              <textarea
                required
                placeholder="-----BEGIN CERTIFICATE-----&#10;MIIBIjAN...&#10;-----END CERTIFICATE-----"
                value={pemInput}
                onChange={(e) => setPemInput(e.target.value)}
                className="w-full bg-[#0a0d16] border border-slate-800 rounded-lg p-2 text-xs font-mono text-violet-300 focus:outline-none focus:border-indigo-500 h-24 scrollbar-thin"
              />
              {registerCertMut.isError && (
                <p className="text-xs text-rose-400">{(registerCertMut.error as any)?.response?.data?.error ?? 'Certificate registration failed'}</p>
              )}
              <button
                type="submit"
                disabled={!pemInput.trim() || registerCertMut.isPending}
                className="w-full flex items-center justify-center gap-2 bg-slate-800 hover:bg-indigo-600 hover:text-white disabled:opacity-40 disabled:hover:bg-slate-800 disabled:hover:text-gray-400 px-4 py-2.5 rounded-lg text-xs font-bold text-indigo-400 transition-all duration-200"
              >
                <Upload className="h-4 w-4" />
                <span>{registerCertMut.isPending ? 'Registering...' : 'Register & Activate Certificate'}</span>
              </button>
            </form>
          )}
        </div>

        {/* Suspend Modal */}
        {showSuspendModal && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-fade-in">
            <div className="glass-panel w-full max-w-md p-6 rounded-2xl border border-slate-800 shadow-2xl relative">
              <h3 className="text-base font-bold text-white mb-2">Suspend Network Participant</h3>
              <p className="text-xs text-gray-400 mb-4">Are you sure you want to suspend {activeBank.name}? Kong will block all inbound payment packages.</p>
              <div className="space-y-4">
                <div className="space-y-1">
                  <label className="text-xs font-semibold text-gray-400">Suspension Reason (Required for audit log)</label>
                  <input
                    type="text" required
                    placeholder="e.g. Audit failure or compliance breach"
                    value={suspendReason}
                    onChange={(e) => setSuspendReason(e.target.value)}
                    className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500"
                  />
                </div>
                <div className="flex justify-end gap-3 pt-2">
                  <button type="button" onClick={() => { setShowSuspendModal(false); setSuspendReason(''); }} className="px-4 py-2 rounded-lg text-sm font-semibold text-gray-400 hover:text-white">Cancel</button>
                  <button type="button" onClick={handleSuspend} disabled={!suspendReason.trim() || updateStatusMut.isPending} className="bg-rose-600 hover:bg-rose-500 disabled:opacity-40 text-white font-semibold text-sm px-4 py-2 rounded-lg">
                    {updateStatusMut.isPending ? 'Suspending...' : 'Confirm Suspension'}
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    );
  }

  // List view (switch roles only)
  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h2 className="text-xl font-bold text-white">Participants Directory</h2>
          <p className="text-gray-400 text-sm">Onboard, configure, and monitor payment network participant banks.</p>
        </div>
        {isSwitchAdmin && (
          <button
            onClick={() => setShowOnboardModal(true)}
            className="flex items-center gap-2 bg-gradient-to-r from-indigo-600 to-violet-600 hover:from-indigo-500 hover:to-violet-500 px-4 py-2 rounded-lg text-sm font-semibold text-white shadow-lg glow-indigo transition-all duration-200"
          >
            <Plus className="h-4 w-4" />
            <span>Onboard Participant</span>
          </button>
        )}
      </div>

      {banksLoading ? (
        <div className="flex items-center justify-center h-48 text-gray-400 text-sm font-mono animate-pulse">Loading banks...</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {banks.map((b) => (
            <div
              key={b.id}
              onClick={() => setSelectedBankId(b.id)}
              className="glass-panel p-6 rounded-2xl glass-panel-hover cursor-pointer space-y-4 flex flex-col justify-between"
            >
              <div className="space-y-3">
                <div className="flex justify-between items-start">
                  <div className="bg-slate-800/80 p-2.5 rounded-xl border border-slate-700/50 text-indigo-400">
                    <Building2 className="h-6 w-6" />
                  </div>
                  <span className={`px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wider ${getStatusBadgeClass(b.status as Bank['status'])}`}>
                    {b.status.replace(/_/g, ' ')}
                  </span>
                </div>
                <div>
                  <h3 className="text-base font-bold text-white tracking-tight">{b.name}</h3>
                  <p className="text-xs text-gray-500 font-mono tracking-wider mt-0.5">BIC: {b.bic} &bull; {b.country}</p>
                </div>
                {b.notes && <p className="text-xs text-gray-400 line-clamp-2 italic">"{b.notes}"</p>}
              </div>
              <div className="pt-4 border-t border-slate-800/40 flex items-center justify-between text-xs text-gray-400 font-mono">
                <span>Created: {new Date(b.createdAt).toLocaleDateString()}</span>
                <span className="text-indigo-400">Manage &rarr;</span>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Onboard Modal */}
      {showOnboardModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-fade-in">
          <div className="glass-panel w-full max-w-lg p-6 rounded-2xl border border-slate-800 shadow-2xl relative">
            <h3 className="text-lg font-bold text-white mb-4">Onboard New Participant</h3>
            <form onSubmit={handleOnboardSubmit} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1">
                  <label className="text-xs font-semibold text-gray-400">Bank Name</label>
                  <input type="text" required placeholder="e.g. Cooperative Bank" value={newBankForm.name}
                    onChange={(e) => setNewBankForm({ ...newBankForm, name: e.target.value })}
                    className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div className="space-y-1">
                  <label className="text-xs font-semibold text-gray-400">SWIFT BIC</label>
                  <input type="text" required placeholder="e.g. COOPKENA" value={newBankForm.bic}
                    onChange={(e) => setNewBankForm({ ...newBankForm, bic: e.target.value.toUpperCase() })}
                    className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1">
                  <label className="text-xs font-semibold text-gray-400">Country Code</label>
                  <input type="text" required placeholder="US" maxLength={2} value={newBankForm.country}
                    onChange={(e) => setNewBankForm({ ...newBankForm, country: e.target.value.toUpperCase() })}
                    className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500" />
                </div>
                <div className="space-y-1">
                  <label className="text-xs font-semibold text-gray-400">Settlement Account</label>
                  <input type="text" required placeholder="e.g. USD-NOSTRO-001" value={newBankForm.settlementAccount}
                    onChange={(e) => setNewBankForm({ ...newBankForm, settlementAccount: e.target.value })}
                    className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 font-mono" />
                </div>
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold text-gray-400">Notes</label>
                <textarea placeholder="Additional registration context..." value={newBankForm.notes}
                  onChange={(e) => setNewBankForm({ ...newBankForm, notes: e.target.value })}
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-indigo-500 h-20" />
              </div>
              {createBankMut.isError && (
                <p className="text-xs text-rose-400">{(createBankMut.error as any)?.response?.data?.error ?? 'Failed to create bank'}</p>
              )}
              <div className="flex justify-end gap-3 pt-4 border-t border-slate-800/40">
                <button type="button" onClick={() => setShowOnboardModal(false)} className="px-4 py-2 rounded-lg text-sm font-semibold text-gray-400 hover:text-white">Cancel</button>
                <button type="submit" disabled={createBankMut.isPending}
                  className="bg-indigo-600 hover:bg-indigo-500 px-4 py-2 rounded-lg text-sm font-semibold text-white shadow-lg shadow-indigo-600/30 disabled:opacity-50">
                  {createBankMut.isPending ? 'Registering...' : 'Register Bank'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
export default Banks;
