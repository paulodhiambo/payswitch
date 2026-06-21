import React, { useState, useEffect } from 'react';
import { Clock } from 'lucide-react';

export type Step = 'RECEIVED' | 'VALIDATED' | 'QUOTED' | 'SCREENED' | 'RESERVED' | 'COMMITTED' | 'SETTLED' | 'ABORTED';
const ORDER: Step[] = ['RECEIVED', 'VALIDATED', 'QUOTED', 'SCREENED', 'RESERVED', 'COMMITTED', 'SETTLED'];

interface TimelineEntry {
  event: Step;
  occurredAt: string;
  detail?: Record<string, any> | null;
}

interface TransactionTimelineProps {
  timeline: TimelineEntry[];
  currentStatus: Step;
  abortReason?: string | null;
  expiresAt?: string | null;
}

const ReservationCountdown: React.FC<{ expiresAt: string }> = ({ expiresAt }) => {
  const calculateTime = () => {
    const diff = new Date(expiresAt).getTime() - Date.now();
    return diff <= 0 ? 0 : Math.ceil(diff / 1000);
  };

  const [timeLeft, setTimeLeft] = useState<number>(() => calculateTime());

  useEffect(() => {
    const interval = setInterval(() => {
      const remaining = calculateTime();
      setTimeLeft(remaining);
      if (remaining <= 0) {
        clearInterval(interval);
      }
    }, 1000);

    return () => clearInterval(interval);
  }, [expiresAt]);

  if (timeLeft <= 0) return <span className="text-[10px] text-rose-400 font-bold font-mono">LOCK EXPIRED</span>;

  return (
    <span className="inline-flex items-center gap-1 text-[10px] text-amber-400 font-bold font-mono mt-1 bg-amber-950/40 border border-amber-900/30 px-1.5 py-0.5 rounded animate-pulse">
      <Clock className="h-3 w-3" />
      <span>{timeLeft}s LOCK</span>
    </span>
  );
};

export const TransactionTimeline: React.FC<TransactionTimelineProps> = ({ timeline, currentStatus, abortReason, expiresAt }) => {
  const aborted = currentStatus === 'ABORTED';

  // Normalize timeline to map payment.received -> RECEIVED, etc.
  const normalizedTimeline = (timeline || []).map((t) => {
    let ev = t.event.toUpperCase();
    if (ev.startsWith('PAYMENT.')) {
      ev = ev.replace('PAYMENT.', '');
    }
    return { ...t, event: ev as Step };
  });

  const getStepState = (step: Step) => {
    const entry = normalizedTimeline.find((t) => t.event === step);
    if (entry) return 'done';
    if (aborted) return 'skipped';
    return 'pending';
  };

  const formatTime = (isoString: string) => {
    if (!isoString) return '';
    return new Date(isoString).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  };

  return (
    <div className="glass-panel p-6 rounded-2xl space-y-6 overflow-hidden glow-indigo">
      <div className="flex justify-between items-center border-b border-slate-800/40 pb-4">
        <div>
          <h4 className="text-sm font-bold text-white uppercase tracking-wider">Saga State Timeline</h4>
          <p className="text-xs text-gray-400 mt-0.5">Real-time interbank settlement orchestration progress.</p>
        </div>
        <div className="flex items-center gap-4 text-xs font-mono">
          <div className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-emerald-400"></span>
            <span className="text-gray-400">Done</span>
          </div>
          <div className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-indigo-500 animate-pulse"></span>
            <span className="text-gray-400">In-Flight</span>
          </div>
          <div className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-slate-700"></span>
            <span className="text-gray-400">Pending</span>
          </div>
        </div>
      </div>

      <div className="relative py-4">
        {/* Progress connecting lines */}
        <div className="absolute top-[38px] left-[6%] right-[6%] h-[2px] bg-slate-800/80 -z-10"></div>
        <div 
          className="absolute top-[38px] left-[6%] h-[2px] bg-gradient-to-r from-emerald-500 to-indigo-500 -z-10 transition-all duration-500 ease-in-out"
          style={{ 
            width: `${aborted 
              ? Math.max(0, (normalizedTimeline.length - 2) / (ORDER.length - 1)) * 88 
              : Math.max(0, (normalizedTimeline.length - 1) / (ORDER.length - 1)) * 88}%` 
          }}
        ></div>

        <ol className="flex justify-between items-start w-full relative">
          {ORDER.map((step, idx) => {
            const entry = normalizedTimeline.find((t) => t.event === step);
            const state = getStepState(step);
            const isActive = currentStatus === step;

            return (
              <li 
                key={step} 
                className="flex flex-col items-center text-center group px-1 min-w-[90px] md:min-w-[120px]"
              >
                {/* Visual Circle Node */}
                <div 
                  className={`h-11 w-11 rounded-full flex items-center justify-center border font-mono text-xs z-10 transition-all duration-300
                    ${state === 'done' 
                      ? 'bg-emerald-950/80 text-emerald-400 border-emerald-500 shadow-[0_0_12px_rgba(16,185,129,0.3)]' 
                      : isActive
                        ? 'bg-indigo-600 text-white border-indigo-400 scale-110 shadow-[0_0_15px_rgba(99,102,241,0.5)] animate-pulse-subtle'
                        : state === 'skipped'
                          ? 'bg-slate-900/60 text-slate-600 border-slate-800/80 cursor-not-allowed opacity-60'
                          : 'bg-[#0f172a] text-slate-500 border-slate-800'}`}
                  title={step}
                >
                  {state === 'done' ? (
                    <span className="text-emerald-400 font-bold">✓</span>
                  ) : (
                    <span>0{idx + 1}</span>
                  )}
                </div>

                {/* Text Labels */}
                <span className={`text-[10px] md:text-xs font-bold mt-3 tracking-wide transition-colors duration-200
                  ${isActive ? 'text-indigo-400' : state === 'done' ? 'text-emerald-400' : 'text-gray-500'}`}>
                  {step}
                </span>

                {/* Subtext info */}
                {entry && (
                  <time className="text-[10px] text-gray-500 font-mono mt-0.5">
                    {formatTime(entry.occurredAt)}
                  </time>
                )}

                {/* Countdown display for reservation */}
                {step === 'RESERVED' && (entry?.detail?.expiresAt || expiresAt) && currentStatus === 'RESERVED' && (
                  <ReservationCountdown expiresAt={(entry?.detail?.expiresAt || expiresAt) as string} />
                )}

                {/* Step specific variables previews */}
                {entry && (
                  <div className="absolute top-[85px] hidden group-hover:block bg-[#0f172a] border border-slate-800 text-[10px] text-gray-300 font-mono p-2 rounded-lg shadow-xl max-w-[160px] text-left z-20 pointer-events-none animate-fade-in">
                    {step === 'RECEIVED' && 'XML payload received.'}
                    {step === 'VALIDATED' && 'ISO 20022 schemas verified.'}
                    {step === 'QUOTED' && (
                      entry.detail?.fee ? `Fee: KES ${entry.detail.fee}` : 'Quote generated.'
                    )}
                    {step === 'SCREENED' && (
                      entry.detail?.cleared !== undefined ? `Cleared: ${entry.detail.cleared}` : 'Compliance cleared.'
                    )}
                    {step === 'RESERVED' && (
                      entry.detail?.expiresAt ? `Lock expires at ${new Date(entry.detail.expiresAt).toLocaleTimeString()}` : expiresAt ? `Lock expires at ${new Date(expiresAt).toLocaleTimeString()}` : 'Liquidity reserved.'
                    )}
                    {step === 'COMMITTED' && 'Saga committed.'}
                    {step === 'SETTLED' && 'Settlement complete.'}
                  </div>
                )}
              </li>
            );
          })}

          {/* Aborted state node */}
          {aborted && (
            <li className="flex flex-col items-center text-center px-1 min-w-[90px] md:min-w-[120px]">
              <div className="h-11 w-11 rounded-full bg-rose-950/80 text-rose-400 border-rose-500 shadow-[0_0_12px_rgba(244,63,94,0.3)] flex items-center justify-center text-xs font-bold z-10">
                ✕
              </div>
              <span className="text-[10px] md:text-xs font-bold text-rose-400 mt-3 tracking-wide">
                ABORTED
              </span>
              <time className="text-[10px] text-gray-500 font-mono mt-0.5">
                {formatTime(normalizedTimeline.find((t) => t.event === 'ABORTED')?.occurredAt || '')}
              </time>
              <span className="text-[10px] text-rose-300 italic font-medium mt-0.5 break-words max-w-[100px] truncate" title={abortReason || normalizedTimeline.find((t) => t.event === 'ABORTED')?.detail?.reason || ''}>
                {abortReason || normalizedTimeline.find((t) => t.event === 'ABORTED')?.detail?.reason || 'Saga execution aborted'}
              </span>
            </li>
          )}
        </ol>
      </div>
      
      {/* Spacer to allow hover boxes to display without clipping */}
      <div className="h-6"></div>
    </div>
  );
};
export default TransactionTimeline;
