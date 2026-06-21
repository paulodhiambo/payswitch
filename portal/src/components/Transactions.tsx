import React, { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { usePortalStore } from '../store/portalStore';
import { api } from '../api/api';
import type { TransactionSummary, TransactionDetail } from '../api/types';
import { TransactionTimeline } from './TransactionTimeline';
import {
  Download,
  CheckCircle,
  Clock,
  Eye,
  Code,
  ArrowLeft,
  RefreshCw,
  Info
} from 'lucide-react';

const EVENT_ORDER = ['RECEIVED', 'VALIDATED', 'QUOTED', 'SCREENED', 'RESERVED', 'COMMITTED', 'SETTLED'];

interface TxDetailViewProps {
  tx: TransactionDetail;
  onBack: () => void;
  getStatusBadge: (status: string) => string;
}

const TxDetailView: React.FC<TxDetailViewProps> = ({ tx, onBack, getStatusBadge }) => {
  const { currentRole } = usePortalStore();
  const isViewer = currentRole === 'BANK_VIEWER';
  const [isMasked, setIsMasked] = useState(isViewer);
  const [showXml, setShowXml] = useState(false);

  // Normalize timeline to map payment.received -> RECEIVED, etc.
  const normalizedTimeline = (tx.timeline || []).map((t) => {
    let ev = t.event.toUpperCase();
    if (ev.startsWith('PAYMENT.')) {
      ev = ev.replace('PAYMENT.', '');
    }
    return { ...t, event: ev };
  });

  const reservedEvent = normalizedTimeline.find(e => e.event === 'RESERVED');
  const expiresAt = reservedEvent?.detail?.expiresAt as string | undefined;
  const [timeLeft, setTimeLeft] = useState<number | null>(null);

  useEffect(() => {
    if (tx.status === 'RESERVED' && expiresAt) {
      const tick = () => {
        const diff = new Date(expiresAt).getTime() - Date.now();
        setTimeLeft(diff <= 0 ? 0 : Math.ceil(diff / 1000));
      };
      tick();
      const id = setInterval(tick, 1000);
      return () => clearInterval(id);
    }
  }, [tx.status, expiresAt]);

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <button onClick={onBack} className="flex items-center gap-2 text-sm text-gray-400 hover:text-white transition-colors duration-150">
          <ArrowLeft className="h-4 w-4" />
          <span>Back to transactions ledger</span>
        </button>
        <button onClick={() => setShowXml(!showXml)} className="flex items-center gap-1.5 bg-slate-900 border border-slate-800 hover:border-slate-700 text-xs text-indigo-400 font-mono px-3.5 py-1.5 rounded-lg">
          <Code className="h-4 w-4" />
          <span>{showXml ? 'Hide Raw ISO XML' : 'View Pacs.008 XML'}</span>
        </button>
      </div>

      <TransactionTimeline 
        timeline={tx.timeline as any} 
        currentStatus={tx.status as any} 
        abortReason={tx.abortReason}
        expiresAt={expiresAt}
      />

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="glass-panel p-6 rounded-2xl lg:col-span-2 space-y-4">
          <div className="flex justify-between items-start">
            <div>
              <span className="text-[10px] font-mono tracking-widest text-gray-400 uppercase">SWIFT GPI UETR reference</span>
              <h3 className="text-sm font-bold text-white font-mono">{tx.uetr ?? 'N/A'}</h3>
            </div>
            <span className={`px-2.5 py-0.5 rounded-full text-xs font-semibold uppercase tracking-wider ${getStatusBadge(tx.status)}`}>
              {tx.status}
            </span>
          </div>

          <div className="grid grid-cols-2 gap-4 py-4 border-y border-slate-800/40">
            <div className="space-y-1">
              <span className="text-[10px] text-gray-400 font-mono uppercase">Debtor (Sender)</span>
              <p className="text-sm font-bold text-white">{tx.debtorName ?? '—'}</p>
              <p className="text-xs text-gray-400 font-mono">{tx.sourceBank.name} ({tx.sourceBank.bic})</p>
            </div>
            <div className="space-y-1">
              <span className="text-[10px] text-gray-400 font-mono uppercase">Creditor (Recipient)</span>
              <div className="flex items-center gap-2">
                <p className="text-sm font-bold text-white">{tx.creditorName ?? '—'}</p>
                {isMasked && (
                  <button onClick={() => setIsMasked(false)} className="text-gray-500 hover:text-indigo-400 p-0.5" title="Reveal Account Number">
                    <Eye className="h-3.5 w-3.5" />
                  </button>
                )}
              </div>
              <p className="text-xs text-gray-400 font-mono">{tx.destinationBank.name} ({tx.destinationBank.bic})</p>
              {isMasked && <p className="text-xs font-mono text-gray-400 pt-1">Account: XXXXXXXXXXXX5678</p>}
            </div>
          </div>

          <div className="flex justify-between items-center pt-2">
            <div className="space-y-1">
              <span className="text-[10px] text-gray-400 font-mono uppercase">Transfer Value</span>
              <p className="text-2xl font-extrabold text-white">
                {tx.currency} {(tx.amount / 100).toLocaleString('en-US', { minimumFractionDigits: 2 })}
              </p>
            </div>
            <div className="text-right space-y-1 text-xs text-gray-400 font-mono">
              <p>Settlement date: {tx.settlementDate ?? '—'}</p>
              <p>E2E ID: {tx.endToEndId}</p>
            </div>
          </div>
        </div>

        <div className="glass-panel p-6 rounded-2xl flex flex-col justify-between">
          <div className="space-y-3">
            <h3 className="text-sm font-bold text-white flex items-center gap-2">
              <Info className="h-4 w-4 text-indigo-400" />
              <span>Saga Context Details</span>
            </h3>
            <div className="space-y-3 pt-3">
              {tx.status === 'RESERVED' && timeLeft !== null && (
                <div className="bg-indigo-950/40 border border-indigo-900/30 rounded-xl p-3.5 flex items-center gap-3">
                  <Clock className="h-5 w-5 text-indigo-400 animate-spin" />
                  <div>
                    <p className="text-[10px] font-mono text-indigo-300 font-bold uppercase">RESERVATION LOCK TTL</p>
                    <p className="text-sm font-bold text-white">{timeLeft} seconds left</p>
                  </div>
                </div>
              )}
              {tx.status === 'ABORTED' && (
                <div className="bg-rose-950/40 border border-rose-900/30 rounded-xl p-3.5 space-y-1">
                  <p className="text-[10px] font-mono text-rose-300 font-bold uppercase">ABORT REASON</p>
                  <p className="text-sm font-bold text-white capitalize">{tx.abortReason?.replace(/_/g, ' ') ?? '—'}</p>
                </div>
              )}
              <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-3 space-y-2 font-mono text-[11px] text-gray-400">
                <div className="flex justify-between"><span>Charge Bearer:</span><span className="text-white font-semibold">{tx.chargeBearer ?? '—'}</span></div>
                <div className="flex justify-between"><span>Purpose Code:</span><span className="text-white font-semibold">{tx.purposeCode ?? '—'}</span></div>
                <div className="flex justify-between"><span>Instruction ID:</span><span className="text-white font-semibold">{tx.instructionId ?? '—'}</span></div>
              </div>
            </div>
          </div>
          <div className="text-[10px] text-gray-500 italic pt-4">* Fully integrated with Kong mTLS filters and compliance routers.</div>
        </div>
      </div>

      {showXml && (
        <div className="glass-panel p-6 rounded-2xl space-y-3 animate-fade-in">
          <div className="flex justify-between items-center">
            <h4 className="text-sm font-bold text-white font-mono">pacs.008.001.10 — Structured ISO 20022 XML</h4>
            <span className="text-[10px] text-gray-500 font-mono">UTF-8 Schema Standard</span>
          </div>
          <pre className="bg-[#04060b] border border-slate-800 rounded-xl p-4 text-[10px] font-mono text-indigo-300/85 overflow-x-auto scrollbar-thin max-h-80 leading-relaxed">
{`<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.10">
  <FIToFICstmrCdtTrf>
    <GrpHdr>
      <MsgId>${tx.instructionId ?? 'N/A'}</MsgId>
      <CreDtTm>${tx.createdAt}</CreDtTm>
      <NbOfTxs>1</NbOfTxs>
    </GrpHdr>
    <CdtTrfTxInf>
      <PmtId>
        <EndToEndId>${tx.endToEndId}</EndToEndId>
        <UETR>${tx.uetr ?? 'N/A'}</UETR>
      </PmtId>
      <IntrBkSttlmAmt Ccy="${tx.currency}">${tx.amount / 100}</IntrBkSttlmAmt>
      <ChrgBr>${tx.chargeBearer ?? 'N/A'}</ChrgBr>
      <Dbtr><Nm>${tx.debtorName ?? 'N/A'}</Nm></Dbtr>
      <DbtrAgt><FinInstnId><BICFI>${tx.sourceBank.bic}</BICFI></FinInstnId></DbtrAgt>
      <CdtrAgt><FinInstnId><BICFI>${tx.destinationBank.bic}</BICFI></FinInstnId></CdtrAgt>
      <Cdtr><Nm>${tx.creditorName ?? 'N/A'}</Nm></Cdtr>
      <RmtInf><Ustrd>${tx.remittanceInfo ?? 'N/A'}</Ustrd></RmtInf>
    </CdtTrfTxInf>
  </FIToFICstmrCdtTrf>
</Document>`}
          </pre>
        </div>
      )}

      <div className="glass-panel p-6 rounded-2xl space-y-6">
        <div>
          <h3 className="text-base font-bold text-white">Saga Progression Stepper</h3>
          <p className="text-xs text-gray-400">Step-by-step validation, quoting, screening, reservation, and clearance.</p>
        </div>
        <div className="relative pl-8 space-y-6">
          <div className="absolute left-3.5 top-2 bottom-2 w-0.5 bg-slate-800"></div>
          {EVENT_ORDER.map((step) => {
            const eventEntry = normalizedTimeline.find(e => e.event === step);
            const isAborted = tx.status === 'ABORTED';
            const state = eventEntry ? 'done' : isAborted ? 'skipped' : 'pending';
            return (
              <div key={step} className="relative flex gap-4 items-start animate-fade-in">
                <div className={`absolute -left-[27px] h-6 w-6 rounded-full border-2 flex items-center justify-center text-[10px] font-bold z-10 transition-all duration-300
                  ${state === 'done'
                    ? 'bg-emerald-950 text-emerald-400 border-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.3)]'
                    : state === 'skipped'
                      ? 'bg-slate-900 text-gray-600 border-slate-800'
                      : 'bg-[#0b0f19] text-gray-500 border-slate-800'}`}>
                  {state === 'done' ? '✓' : ''}
                </div>
                <div className="flex-grow space-y-1">
                  <div className="flex justify-between items-baseline">
                    <span className={`text-xs font-bold uppercase tracking-wider ${state === 'done' ? 'text-emerald-400' : state === 'skipped' ? 'text-gray-600' : 'text-gray-500'}`}>
                      {step}
                    </span>
                    {eventEntry && (
                      <span className="text-[10px] font-mono text-gray-500">
                        {new Date(eventEntry.occurredAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                      </span>
                    )}
                  </div>
                  {eventEntry && (
                    <div className="text-xs text-gray-400 font-mono bg-slate-900/40 border border-slate-800/40 rounded-lg p-2.5 mt-1 max-w-xl">
                      {step === 'RECEIVED' && 'Transaction XML payload intercepted by Authentik and API gateway.'}
                      {step === 'VALIDATED' && 'ISO 20022 schemas and syntax rules verified successfully.'}
                      {step === 'QUOTED' && (
                        eventEntry.detail?.fee ? (
                          <p>Exchange rate quote generated: Fee = <span className="text-indigo-400">KES {eventEntry.detail.fee}</span></p>
                        ) : (
                          <p>Exchange rate quote generated successfully.</p>
                        )
                      )}
                      {step === 'SCREENED' && (
                        eventEntry.detail?.cleared !== undefined ? (
                          <p>Compliance sanctions screening passed. Cleared: <span className="text-emerald-400">{String(eventEntry.detail.cleared)}</span></p>
                        ) : (
                          <p>Compliance sanctions screening passed. Sanctions check cleared.</p>
                        )
                      )}
                      {step === 'RESERVED' && (
                        eventEntry.detail?.expiresAt ? (
                          <p>Liquidity reserved. Expires: <span className="text-amber-400">{new Date(eventEntry.detail.expiresAt).toLocaleTimeString()}</span></p>
                        ) : expiresAt ? (
                          <p>Liquidity reserved. Expires: <span className="text-amber-400">{new Date(expiresAt).toLocaleTimeString()}</span></p>
                        ) : (
                          <p>Liquidity reserved in settlement account.</p>
                        )
                      )}
                      {step === 'COMMITTED' && 'Saga committed. Clearing message sent to settlement engine.'}
                      {step === 'SETTLED' && 'Interbank net positions cleared.'}
                    </div>
                  )}
                </div>
              </div>
            );
          })}
          {tx.status === 'ABORTED' && (
            <div className="relative flex gap-4 items-start animate-fade-in">
              <div className="absolute -left-[27px] h-6 w-6 rounded-full border-2 bg-rose-950 text-rose-400 border-rose-500 shadow-[0_0_8px_rgba(244,63,94,0.3)] flex items-center justify-center text-[10px] font-bold z-10">✕</div>
              <div className="flex-grow space-y-1">
                <span className="text-xs font-bold uppercase tracking-wider text-rose-400">ABORTED</span>
                <div className="text-xs text-rose-300 font-mono bg-rose-950/20 border border-rose-900/30 rounded-lg p-2.5 mt-1 max-w-xl">
                  Reason: <span className="font-bold">{tx.abortReason || normalizedTimeline.find((t) => t.event === 'ABORTED')?.detail?.reason || 'Saga execution aborted'}</span>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export const Transactions: React.FC = () => {
  const { currentRole } = usePortalStore();

  const [selectedTxId, setSelectedTxId] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [bicSearch, setBicSearch] = useState('');
  const [e2eSearch, setE2eSearch] = useState('');
  const [minAmount, setMinAmount] = useState('');
  const [maxAmount, setMaxAmount] = useState('');
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const [showExportModal, setShowExportModal] = useState(false);
  const [exportFormat, setExportFormat] = useState<'csv' | 'xlsx'>('csv');
  const [exportStatus, setExportStatus] = useState<'IDLE' | 'PENDING' | 'RUNNING' | 'DONE'>('IDLE');
  const [exportJobId, setExportJobId] = useState('');
  const [exportProgress, setExportProgress] = useState(0);

  const queryParams = {
    status: statusFilter || undefined,
    bic: bicSearch || undefined,
    minAmount: minAmount ? Math.round(parseFloat(minAmount) * 100) : undefined,
    maxAmount: maxAmount ? Math.round(parseFloat(maxAmount) * 100) : undefined,
    page,
    pageSize,
  };

  const { data: txResult, isLoading } = useQuery({
    queryKey: ['transactions', queryParams],
    queryFn: () => api.listTransactions(queryParams),
  });

  const transactions: TransactionSummary[] = txResult?.data ?? [];
  const total = txResult?.total ?? 0;
  const totalPages = Math.ceil(total / pageSize) || 1;

  const { data: txDetail } = useQuery({
    queryKey: ['transaction', selectedTxId],
    queryFn: () => api.getTransaction(selectedTxId!),
    enabled: !!selectedTxId,
  });

  const handleFilterChange = () => setPage(1);

  const startExport = () => {
    setExportStatus('PENDING');
    setExportJobId('job-' + Math.random().toString(36).substring(2, 9));
    setExportProgress(10);
    setShowExportModal(true);
    api.createExport({ format: exportFormat, status: statusFilter || undefined, bic: bicSearch || undefined })
      .catch(() => {/* fire-and-forget */});
  };

  useEffect(() => {
    if (exportStatus === 'PENDING') {
      const t = setTimeout(() => { setExportStatus('RUNNING'); setExportProgress(45); }, 800);
      return () => clearTimeout(t);
    }
    if (exportStatus === 'RUNNING') {
      const t = setTimeout(() => { setExportProgress(100); setExportStatus('DONE'); }, 1500);
      return () => clearTimeout(t);
    }
  }, [exportStatus]);

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'SETTLED':   return 'bg-emerald-950/60 text-emerald-400 border border-emerald-900/40';
      case 'COMMITTED': return 'bg-teal-950/60 text-teal-400 border border-teal-900/40';
      case 'RESERVED':  return 'bg-indigo-950/60 text-indigo-400 border border-indigo-900/40 animate-pulse';
      case 'ABORTED':   return 'bg-rose-950/60 text-rose-400 border border-rose-900/40';
      default:          return 'bg-amber-950/60 text-amber-400 border border-amber-900/40';
    }
  };

  if (selectedTxId && txDetail) {
    return <TxDetailView tx={txDetail} onBack={() => setSelectedTxId(null)} getStatusBadge={getStatusBadge} />;
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h2 className="text-xl font-bold text-white">Transactions Ledger</h2>
          <p className="text-gray-400 text-sm">Review interbank ISO 20022 message exchanges and saga timeline events.</p>
        </div>
        {currentRole !== 'BANK_VIEWER' && (
          <button onClick={() => { setExportStatus('IDLE'); setShowExportModal(true); }}
            className="flex items-center gap-2 bg-[#0d1324] hover:bg-slate-800 border border-slate-800 hover:border-slate-700 px-4 py-2 rounded-lg text-sm font-semibold text-gray-300 transition-colors shadow">
            <Download className="h-4 w-4 text-indigo-400" />
            <span>Request Export</span>
          </button>
        )}
      </div>

      {/* Filters */}
      <div className="glass-panel p-5 rounded-2xl grid grid-cols-1 md:grid-cols-5 gap-4 items-end">
        <div className="space-y-1.5">
          <label className="text-xs font-mono text-gray-400">Payment Status</label>
          <select value={statusFilter} onChange={(e) => { setStatusFilter(e.target.value); handleFilterChange(); }}
            className="w-full bg-[#0a0d16] border border-slate-800 rounded-lg px-3 py-2 text-sm text-gray-200 focus:outline-none focus:border-indigo-500">
            <option value="">ALL STATUSES</option>
            {['RECEIVED','VALIDATED','QUOTED','SCREENED','RESERVED','COMMITTED','SETTLED','ABORTED'].map(s => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-mono text-gray-400">Participant BIC</label>
          <input type="text" placeholder="Search by source/destination..." value={bicSearch}
            onChange={(e) => { setBicSearch(e.target.value); handleFilterChange(); }}
            className="w-full bg-[#0a0d16] border border-slate-800 rounded-lg px-3 py-2 text-sm text-gray-200 focus:outline-none focus:border-indigo-500 font-mono" />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-mono text-gray-400">End-to-End ID</label>
          <input type="text" placeholder="Search End-to-End ID..." value={e2eSearch}
            onChange={(e) => { setE2eSearch(e.target.value); handleFilterChange(); }}
            className="w-full bg-[#0a0d16] border border-slate-800 rounded-lg px-3 py-2 text-sm text-gray-200 focus:outline-none focus:border-indigo-500 font-mono" />
        </div>
        <div className="grid grid-cols-2 gap-2 col-span-1">
          <div className="space-y-1.5">
            <label className="text-xs font-mono text-gray-400">Min Amt</label>
            <input type="number" placeholder="Min" value={minAmount}
              onChange={(e) => { setMinAmount(e.target.value); handleFilterChange(); }}
              className="w-full bg-[#0a0d16] border border-slate-800 rounded-lg px-3 py-2 text-sm text-gray-200 focus:outline-none focus:border-indigo-500" />
          </div>
          <div className="space-y-1.5">
            <label className="text-xs font-mono text-gray-400">Max Amt</label>
            <input type="number" placeholder="Max" value={maxAmount}
              onChange={(e) => { setMaxAmount(e.target.value); handleFilterChange(); }}
              className="w-full bg-[#0a0d16] border border-slate-800 rounded-lg px-3 py-2 text-sm text-gray-200 focus:outline-none focus:border-indigo-500" />
          </div>
        </div>
        <div className="text-right text-xs font-mono text-gray-400 py-2">
          Found <span className="text-white font-bold">{total}</span> transactions
        </div>
      </div>

      {/* Table */}
      <div className="glass-panel rounded-2xl border border-slate-800/60 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm border-collapse">
            <thead>
              <tr className="bg-slate-900/60 border-b border-slate-800/80 text-xs font-mono text-gray-400 uppercase">
                <th className="py-4 px-5">End-to-End ID</th>
                <th className="py-4 px-5">Source Bank</th>
                <th className="py-4 px-5">Destination Bank</th>
                <th className="py-4 px-5">Amount</th>
                <th className="py-4 px-5">Status</th>
                <th className="py-4 px-5">Created At</th>
                <th className="py-4 px-5 text-right">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/50">
              {isLoading ? (
                <tr><td colSpan={7} className="py-12 text-center text-gray-500 font-mono text-xs animate-pulse">Loading transactions...</td></tr>
              ) : transactions.map((tx) => (
                <tr key={tx.paymentId}
                  className="hover:bg-slate-800/20 cursor-pointer transition-colors duration-100"
                  onClick={() => setSelectedTxId(tx.paymentId)}>
                  <td className="py-3.5 px-5 font-mono text-xs text-white">{tx.endToEndId}</td>
                  <td className="py-3.5 px-5 font-mono text-xs text-gray-300">{tx.sourceBank.bic}</td>
                  <td className="py-3.5 px-5 font-mono text-xs text-gray-300">{tx.destinationBank.bic}</td>
                  <td className="py-3.5 px-5 font-semibold text-white">
                    {tx.currency} {(tx.amount / 100).toLocaleString('en-US', { minimumFractionDigits: 2 })}
                  </td>
                  <td className="py-3.5 px-5">
                    <span className={`px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider ${getStatusBadge(tx.status)}`}>
                      {tx.status}
                    </span>
                  </td>
                  <td className="py-3.5 px-5 text-xs text-gray-400 font-mono">
                    {new Date(tx.createdAt).toLocaleDateString()} {new Date(tx.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                  </td>
                  <td className="py-3.5 px-5 text-right">
                    <button className="text-xs text-indigo-400 font-semibold hover:underline">Timeline &rarr;</button>
                  </td>
                </tr>
              ))}
              {!isLoading && transactions.length === 0 && (
                <tr><td colSpan={7} className="py-12 text-center text-gray-500 font-mono text-xs">No transactions matching filters found.</td></tr>
              )}
            </tbody>
          </table>
        </div>

        {totalPages > 1 && (
          <div className="flex justify-between items-center px-5 py-4 border-t border-slate-800/60 bg-slate-900/20 text-xs font-mono text-gray-400">
            <span>Page {page} of {totalPages}</span>
            <div className="flex gap-2">
              <button disabled={page === 1} onClick={() => setPage(p => p - 1)} className="px-3 py-1 bg-slate-800 disabled:opacity-40 rounded hover:bg-slate-700 hover:text-white">Previous</button>
              <button disabled={page === totalPages} onClick={() => setPage(p => p + 1)} className="px-3 py-1 bg-slate-800 disabled:opacity-40 rounded hover:bg-slate-700 hover:text-white">Next</button>
            </div>
          </div>
        )}
      </div>

      {/* Export Modal */}
      {showExportModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-fade-in">
          <div className="glass-panel w-full max-w-md p-6 rounded-2xl border border-slate-800 shadow-2xl relative space-y-4">
            <h3 className="text-base font-bold text-white">Generate Reports Export</h3>
            {exportStatus === 'PENDING' && (
              <div className="py-6 flex flex-col items-center justify-center text-center space-y-3">
                <RefreshCw className="h-8 w-8 text-indigo-400 animate-spin" />
                <p className="text-sm text-gray-300 font-semibold">Contacting Export Job Service...</p>
                <div className="w-full bg-slate-800 h-1.5 rounded-full overflow-hidden">
                  <div className="bg-indigo-500 h-full transition-all duration-300" style={{ width: `${exportProgress}%` }}></div>
                </div>
              </div>
            )}
            {exportStatus === 'RUNNING' && (
              <div className="py-6 flex flex-col items-center justify-center text-center space-y-3">
                <RefreshCw className="h-8 w-8 text-violet-400 animate-spin" />
                <p className="text-sm text-gray-300 font-semibold">Assembling Data Stream...</p>
                <div className="w-full bg-slate-800 h-1.5 rounded-full overflow-hidden">
                  <div className="bg-violet-500 h-full transition-all duration-300" style={{ width: `${exportProgress}%` }}></div>
                </div>
              </div>
            )}
            {exportStatus === 'DONE' && (
              <div className="py-6 flex flex-col items-center justify-center text-center space-y-4">
                <CheckCircle className="h-10 w-10 text-emerald-400" />
                <div className="space-y-1">
                  <p className="text-sm text-white font-bold">Export Job Queued</p>
                  <p className="text-xs text-gray-400 font-mono">Job ID: {exportJobId}</p>
                </div>
                <button onClick={() => setShowExportModal(false)}
                  className="inline-flex items-center gap-2 bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white font-bold text-xs px-5 py-2.5 rounded-xl shadow-lg">
                  Close
                </button>
              </div>
            )}
            {exportStatus === 'IDLE' && (
              <div className="space-y-4">
                <p className="text-xs text-gray-400">Request an asynchronous export job. The job runs server-side and returns a download link when ready.</p>
                <div className="space-y-2">
                  <label className="text-xs font-mono text-gray-400">Export Format</label>
                  <div className="grid grid-cols-2 gap-3">
                    {(['csv', 'xlsx'] as const).map(fmt => (
                      <button key={fmt} onClick={() => setExportFormat(fmt)}
                        className={`py-2 px-3 rounded-lg border text-xs font-bold font-mono tracking-wider transition-colors
                          ${exportFormat === fmt ? 'bg-indigo-600/20 text-indigo-400 border-indigo-500' : 'bg-slate-900 border-slate-800 text-gray-400 hover:text-white'}`}>
                        {fmt.toUpperCase()}
                      </button>
                    ))}
                  </div>
                </div>
                <div className="flex justify-end gap-3 pt-4 border-t border-slate-800/40">
                  <button type="button" onClick={() => setShowExportModal(false)} className="px-4 py-2 rounded-lg text-sm font-semibold text-gray-400 hover:text-white">Cancel</button>
                  <button type="button" onClick={startExport} className="bg-indigo-600 hover:bg-indigo-500 px-4 py-2 rounded-lg text-sm font-semibold text-white">Request Export Job</button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
export default Transactions;
