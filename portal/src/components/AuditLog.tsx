import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api/api';
import type { AuditLogEntry } from '../api/types';
import {
  User,
  History,
  ArrowRight,
  Shield,
  Eye,
  PlusCircle,
  RefreshCw,
  Filter,
  CheckCircle2
} from 'lucide-react';

const ACTION_OPTIONS = [
  { value: 'ALL',                label: 'ALL ACTIONS' },
  { value: 'bank.status_change', label: 'BANK STATUS' },
  { value: 'bank.create',        label: 'BANK CREATIONS' },
  { value: 'certificate.create', label: 'CERTIFICATE CREATIONS' },
  { value: 'certificate.revoke', label: 'CERTIFICATE REVOCATIONS' },
];

const getActionColor = (action: string) => {
  if (action === 'bank.status_change')  return 'text-amber-400 bg-amber-950/40 border border-amber-900/30';
  if (action === 'bank.create')         return 'text-indigo-400 bg-indigo-950/40 border border-indigo-900/30';
  if (action === 'certificate.create')  return 'text-emerald-400 bg-emerald-950/40 border border-emerald-900/30';
  if (action === 'certificate.revoke')  return 'text-rose-400 bg-rose-950/40 border border-rose-900/30';
  return 'text-slate-300 bg-slate-800/40 border border-slate-700/30';
};

const getActionIcon = (action: string) => {
  if (action === 'bank.status_change')  return RefreshCw;
  if (action === 'bank.create')         return PlusCircle;
  if (action === 'certificate.create')  return Shield;
  if (action === 'certificate.revoke')  return History;
  return Eye;
};

const renderVisualDiff = (log: AuditLogEntry) => {
  const diff = log.diff;
  if (!diff) return null;

  if (log.action === 'bank.status_change') {
    return (
      <div className="flex items-center gap-3 text-xs font-mono mt-2 bg-[#0a0d16] p-2.5 rounded-lg border border-slate-800 max-w-md">
        <span className="text-gray-400">Changed:</span>
        <span className="bg-rose-950/40 border border-rose-900/30 text-rose-400 px-2 py-0.5 rounded line-through">
          {String(diff.before ?? diff.old_status ?? '—')}
        </span>
        <ArrowRight className="h-3 w-3 text-gray-500" />
        <span className="bg-emerald-950/40 border border-emerald-500/30 text-emerald-400 px-2 py-0.5 rounded">
          {String(diff.after ?? diff.new_status ?? '—')}
        </span>
        {diff.reason && (
          <span className="text-gray-500 italic truncate max-w-[120px]" title={String(diff.reason)}>
            ({diff.reason})
          </span>
        )}
      </div>
    );
  }

  return (
    <div className="text-[10px] font-mono mt-2 bg-[#04060b] p-3 rounded-lg border border-slate-900 max-w-lg overflow-x-auto text-indigo-300">
      <span className="text-gray-500 block mb-1 font-sans font-semibold">Entity snapshot variables:</span>
      {Object.entries(diff).map(([key, val]) => (
        <div key={key} className="flex gap-2">
          <span className="text-slate-500">{key}:</span>
          <span className="text-gray-300 truncate max-w-xs">{String(val)}</span>
        </div>
      ))}
    </div>
  );
};

export const AuditLog: React.FC = () => {
  const [actionFilter, setActionFilter] = useState('ALL');
  const [page, setPage] = useState(1);
  const pageSize = 20;

  const { data: result, isLoading } = useQuery({
    queryKey: ['audit-log', actionFilter, page],
    queryFn: () => api.listAuditLog({
      action: actionFilter !== 'ALL' ? actionFilter : undefined,
      page,
      pageSize,
    }),
  });

  const logs: AuditLogEntry[] = result?.data ?? [];
  const total = result?.total ?? 0;
  const totalPages = Math.ceil(total / pageSize) || 1;

  const handleFilterChange = (val: string) => {
    setActionFilter(val);
    setPage(1);
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h2 className="text-xl font-bold text-white">System Audit Log</h2>
          <p className="text-gray-400 text-sm">Immutable security audit trails representing all mutating portal actions.</p>
        </div>
        <div className="flex items-center gap-2 bg-[#0f172a] border border-[#1e293b] px-3 py-1.5 rounded-lg">
          <Filter className="h-4 w-4 text-indigo-400" />
          <select
            value={actionFilter}
            onChange={(e) => handleFilterChange(e.target.value)}
            className="bg-transparent text-xs font-mono text-gray-300 uppercase outline-none cursor-pointer"
          >
            {ACTION_OPTIONS.map(opt => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
        </div>
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center h-40 text-gray-500 font-mono text-xs animate-pulse">
          Loading audit log...
        </div>
      ) : (
        <>
          <div className="relative pl-8 space-y-6">
            <div className="absolute left-3.5 top-2 bottom-2 w-0.5 bg-slate-800/80"></div>

            {logs.map((log) => {
              const Icon = getActionIcon(log.action);
              const displayActor = log.actor.email || log.actor.id;
              return (
                <div key={log.id} className="relative flex gap-4 items-start animate-fade-in group">
                  <div className={`absolute -left-[27px] h-6 w-6 rounded-full flex items-center justify-center z-10 transition-all duration-300 ${getActionColor(log.action)}`}>
                    <Icon className="h-3.5 w-3.5" />
                  </div>

                  <div className="flex-1 space-y-2.5 glass-panel p-5 rounded-2xl border border-slate-800/60 shadow-lg max-w-3xl">
                    <div className="flex justify-between items-start gap-4">
                      <div className="space-y-1">
                        <span className="text-[10px] font-mono font-bold tracking-widest text-slate-500 uppercase">Actor Profile &bull; {log.actor.role}</span>
                        <p className="text-xs font-bold text-white flex items-center gap-1.5">
                          <User className="h-3 w-3 text-indigo-400" />
                          <span>{displayActor}</span>
                        </p>
                      </div>
                      <div className="text-right space-y-1">
                        <span className="text-[10px] font-mono text-slate-500">
                          {new Date(log.occurredAt).toLocaleDateString()} {new Date(log.occurredAt).toLocaleTimeString()}
                        </span>
                        <p className="text-[9px] font-mono text-gray-500 truncate max-w-[120px]" title={`Log ID: ${log.id}`}>
                          ID: {log.id.substring(0, 8)}...
                        </p>
                      </div>
                    </div>

                    <div className="text-xs text-gray-300">
                      Triggered Action:{' '}
                      <span className="font-semibold text-white font-mono uppercase tracking-wider">{log.action.replace(/\./g, ' ').replace(/_/g, ' ')}</span>
                      {' '}on target type{' '}
                      <span className="text-indigo-400 font-bold">{log.targetType}</span>
                      {' '}(ID: <span className="font-mono text-slate-500 text-[10px]">{log.targetId.substring(0, 8)}...</span>)
                    </div>

                    {renderVisualDiff(log)}
                  </div>
                </div>
              );
            })}

            {logs.length === 0 && (
              <div className="flex flex-col items-center justify-center p-12 text-center text-gray-500 font-mono text-xs max-w-xl">
                <CheckCircle2 className="h-8 w-8 text-emerald-500 mb-2" />
                <p>No audit entries match filter scope.</p>
              </div>
            )}
          </div>

          {totalPages > 1 && (
            <div className="flex justify-between items-center px-2 py-3 text-xs font-mono text-gray-400">
              <span>Page {page} of {totalPages} &bull; {total} entries total</span>
              <div className="flex gap-2">
                <button disabled={page === 1} onClick={() => setPage(p => p - 1)} className="px-3 py-1 bg-slate-800 disabled:opacity-40 rounded hover:bg-slate-700 hover:text-white">Previous</button>
                <button disabled={page === totalPages} onClick={() => setPage(p => p + 1)} className="px-3 py-1 bg-slate-800 disabled:opacity-40 rounded hover:bg-slate-700 hover:text-white">Next</button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
};
export default AuditLog;
