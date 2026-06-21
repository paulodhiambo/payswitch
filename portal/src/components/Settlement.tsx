import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { usePortalStore } from '../store/portalStore';
import { api } from '../api/api';
import type { SettlementWindow } from '../api/types';
import {
  Coins,
  ArrowUpRight,
  ArrowDownLeft,
  Calendar,
  Clock,
  ArrowLeft,
  Building
} from 'lucide-react';

const getStatusBadge = (status: string) => {
  switch (status) {
    case 'OPEN':     return 'bg-indigo-950/60 text-indigo-400 border border-indigo-900/40 animate-pulse';
    case 'CLOSED':   return 'bg-amber-950/60 text-amber-400 border border-amber-900/40';
    case 'SETTLED':  return 'bg-emerald-950/60 text-emerald-400 border border-emerald-900/40';
    case 'DISPUTED': return 'bg-rose-950/60 text-rose-400 border border-rose-900/40';
    default:         return 'bg-slate-800/60 text-slate-300 border border-slate-700/40';
  }
};

interface DetailViewProps {
  win: SettlementWindow;
  onBack: () => void;
  isBankScoped: boolean;
  currentParticipantId: string | null;
}

const WindowDetailView: React.FC<DetailViewProps> = ({ win, onBack, isBankScoped, currentParticipantId }) => {
  const totalTransactions = win.positions?.reduce((sum, p) => sum + p.transactionCount, 0) ?? 0;

  return (
    <div className="space-y-6">
      <button onClick={onBack} className="flex items-center gap-2 text-sm text-gray-400 hover:text-white transition-colors duration-150">
        <ArrowLeft className="h-4 w-4" />
        <span>Back to settlement windows</span>
      </button>

      <div className="glass-panel p-6 rounded-2xl flex flex-col md:flex-row md:items-center justify-between gap-6">
        <div className="flex items-center gap-4">
          <div className="bg-gradient-to-br from-indigo-500 to-violet-600 p-3.5 rounded-2xl text-white glow-indigo">
            <Coins className="h-8 w-8" />
          </div>
          <div>
            <div className="flex items-center gap-3">
              <h2 className="text-xl font-bold text-white font-mono leading-none">Window: {win.id}</h2>
              <span className={`px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wider ${getStatusBadge(win.status)}`}>
                {win.status}
              </span>
            </div>
            <p className="text-xs font-mono text-gray-400 tracking-wider mt-1.5 flex flex-wrap gap-x-4">
              <span>Opened: {new Date(win.openedAt).toLocaleString()}</span>
              {win.closedAt && <span>Closed: {new Date(win.closedAt).toLocaleString()}</span>}
              <span>Currency: {win.currency}</span>
            </p>
          </div>
        </div>
        <div className="flex items-center gap-6 font-mono border-t md:border-t-0 pt-4 md:pt-0 border-slate-800/40">
          <div className="space-y-1">
            <span className="text-[10px] text-gray-500 uppercase font-semibold">Consolidated Volume</span>
            <p className="text-lg font-bold text-white">{totalTransactions} Txs</p>
          </div>
        </div>
      </div>

      <div className="glass-panel rounded-2xl border border-slate-800/60 overflow-hidden">
        <div className="px-5 py-4 border-b border-slate-800/80 bg-slate-900/40">
          <h3 className="text-sm font-bold text-white">Net Clearing Balances</h3>
          <p className="text-xs text-gray-400">Position ledger detailing assets sent, assets received, and aggregate interbank positions.</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm border-collapse">
            <thead>
              <tr className="bg-slate-950/20 border-b border-slate-800/80 text-xs font-mono text-gray-400 uppercase">
                <th className="py-4 px-5">Participant Bank</th>
                <th className="py-4 px-5">BIC Scope</th>
                <th className="py-4 px-5 text-right">Liquidity Sent</th>
                <th className="py-4 px-5 text-right">Liquidity Received</th>
                <th className="py-4 px-5 text-right">Net Clearing Position</th>
                <th className="py-4 px-5 text-right">Transferred Txs</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {(win.positions ?? [])
                .filter(pos => !isBankScoped || pos.bic === currentParticipantId)
                .map((pos) => {
                  const isOwnBank = pos.bic === currentParticipantId;
                  const isNetPositive = pos.netMinorUnits >= 0;
                  return (
                    <tr key={pos.bic} className={`hover:bg-slate-800/10 ${isOwnBank ? 'bg-indigo-950/10' : ''}`}>
                      <td className="py-4 px-5 font-semibold text-white">
                        <div className="flex items-center gap-2">
                          <Building className="h-4 w-4 text-gray-500" />
                          <span>{pos.bankName}</span>
                          {isOwnBank && (
                            <span className="bg-indigo-950 text-indigo-400 border border-indigo-900/30 text-[9px] font-bold px-1.5 py-0.5 rounded font-mono">MY BANK</span>
                          )}
                        </div>
                      </td>
                      <td className="py-4 px-5 font-mono text-xs text-gray-400">{pos.bic}</td>
                      <td className="py-4 px-5 text-right font-mono text-xs text-gray-300">
                        {win.currency} {(pos.sentMinorUnits / 100).toLocaleString('en-US', { minimumFractionDigits: 2 })}
                      </td>
                      <td className="py-4 px-5 text-right font-mono text-xs text-gray-300">
                        {win.currency} {(pos.receivedMinorUnits / 100).toLocaleString('en-US', { minimumFractionDigits: 2 })}
                      </td>
                      <td className={`py-4 px-5 text-right font-semibold font-mono text-xs ${isNetPositive ? 'text-emerald-400' : 'text-rose-400'}`}>
                        {isNetPositive ? '+' : ''}{(pos.netMinorUnits / 100).toLocaleString('en-US', { minimumFractionDigits: 2 })}
                      </td>
                      <td className="py-4 px-5 text-right font-mono text-xs text-white">{pos.transactionCount} Txs</td>
                    </tr>
                  );
                })}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export const Settlement: React.FC = () => {
  const { currentRole, currentParticipantId } = usePortalStore();
  const [selectedWindowId, setSelectedWindowId] = useState<string | null>(null);
  const isBankScoped = currentRole.startsWith('BANK_');

  const { data: windowsResult, isLoading } = useQuery({
    queryKey: ['settlement-windows'],
    queryFn: () => api.listSettlementWindows({ pageSize: 50 }),
  });

  const { data: windowDetail } = useQuery({
    queryKey: ['settlement-window', selectedWindowId],
    queryFn: () => api.getSettlementWindow(selectedWindowId!),
    enabled: !!selectedWindowId,
  });

  const windows = windowsResult?.data ?? [];

  if (selectedWindowId && windowDetail) {
    return (
      <WindowDetailView
        win={windowDetail}
        onBack={() => setSelectedWindowId(null)}
        isBankScoped={isBankScoped}
        currentParticipantId={currentParticipantId}
      />
    );
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div>
        <h2 className="text-xl font-bold text-white">Settlement Windows</h2>
        <p className="text-gray-400 text-sm">Review interbank net settlement positions and clearing windows.</p>
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center h-40 text-gray-500 font-mono text-xs animate-pulse">
          Loading settlement windows...
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-6">
          {windows.map((win) => (
            <div
              key={win.id}
              onClick={() => setSelectedWindowId(win.id)}
              className="glass-panel p-6 rounded-2xl glass-panel-hover cursor-pointer flex flex-col md:flex-row md:items-center justify-between gap-6"
            >
              <div className="flex items-center gap-4">
                <div className="bg-slate-800/80 p-3 rounded-xl border border-slate-700/50 text-indigo-400 flex-shrink-0">
                  <Coins className="h-6 w-6" />
                </div>
                <div className="space-y-1">
                  <div className="flex items-center gap-2.5">
                    <h3 className="text-sm font-bold text-white font-mono">{win.id.substring(0, 18)}...</h3>
                    <span className={`px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wider ${getStatusBadge(win.status)}`}>
                      {win.status}
                    </span>
                  </div>
                  <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-400 font-mono">
                    <span className="flex items-center gap-1"><Calendar className="h-3 w-3" /> Date: {win.settlementDate}</span>
                    <span className="flex items-center gap-1"><Clock className="h-3 w-3" /> Opened: {new Date(win.openedAt).toLocaleTimeString()}</span>
                    {win.closedAt && (
                      <span className="flex items-center gap-1"><Clock className="h-3 w-3" /> Closed: {new Date(win.closedAt).toLocaleTimeString()}</span>
                    )}
                    <span className="text-slate-600">Currency: {win.currency}</span>
                  </div>
                </div>
              </div>

              <div className="flex items-center gap-6 md:text-right font-mono">
                {win.netPositionMinorUnits !== null && isBankScoped ? (
                  <div className="space-y-1">
                    <span className="text-[10px] text-gray-500 uppercase font-semibold">Net Interbank Position</span>
                    <p className={`text-sm font-bold flex items-center gap-1 justify-end
                      ${win.netPositionMinorUnits >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}>
                      {win.netPositionMinorUnits >= 0
                        ? <ArrowUpRight className="h-4 w-4" />
                        : <ArrowDownLeft className="h-4 w-4" />}
                      <span>{win.currency} {Math.abs(win.netPositionMinorUnits / 100).toLocaleString('en-US', { minimumFractionDigits: 2 })}</span>
                      <span className="text-[10px] text-gray-400">{win.netPositionMinorUnits >= 0 ? 'Receivable' : 'Payable'}</span>
                    </p>
                  </div>
                ) : (
                  <div className="space-y-1">
                    <span className="text-[10px] text-gray-500 uppercase font-semibold">Network Summary</span>
                    <p className="text-xs text-white">Net Clearings Active</p>
                  </div>
                )}
                <span className="text-indigo-400 font-semibold text-xs ml-2 hover:underline hidden md:inline">View details &rarr;</span>
              </div>
            </div>
          ))}

          {windows.length === 0 && (
            <div className="flex flex-col items-center justify-center p-12 text-gray-500 font-mono text-xs text-center">
              <Coins className="h-8 w-8 mb-2 opacity-30" />
              <p>No settlement windows found.</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
};
export default Settlement;
