import React, { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api/api';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  BarChart,
  Bar
} from 'recharts';
import {
  Activity,
  CheckCircle2,
  Clock,
  TrendingUp,
  TrendingDown,
  DollarSign
} from 'lucide-react';

const COLORS = ['#8b5cf6', '#10b981', '#f59e0b', '#f43f5e', '#64748b'];

export const Dashboard: React.FC = () => {
  const [range, setRange] = useState<'24h' | '7d' | '30d'>('7d');

  const { data: summary, isLoading: summaryLoading } = useQuery({
    queryKey: ['dashboard-summary', range],
    queryFn: () => api.getDashboardSummary(range),
  });

  const { data: abortData } = useQuery({
    queryKey: ['abort-reasons', range],
    queryFn: () => api.getAbortReasons(range),
  });

  const chartData = summary?.buckets ?? [];
  const abortBreakdown = abortData ?? { total: 0, reasons: [] };

  const stats = useMemo(() => {
    let totalTx = 0, successCount = 0, abortCount = 0, totalAmt = 0;
    let avgLatencySum = 0, latencyCount = 0;

    chartData.forEach(bucket => {
      totalTx += bucket.totalTransactions;
      successCount += bucket.successCount;
      abortCount += bucket.abortCount;
      totalAmt += bucket.totalAmountMinorUnits;
      if (bucket.p99LatencyMs != null) {
        avgLatencySum += bucket.p99LatencyMs;
        latencyCount++;
      }
    });

    const successRate = totalTx > 0 ? (successCount / totalTx) * 100 : 100;
    const avgP99Latency = latencyCount > 0 ? Math.round(avgLatencySum / latencyCount) : 0;

    return {
      totalTx,
      successRate,
      abortCount,
      totalAmt: (totalAmt / 100).toLocaleString('en-US', { minimumFractionDigits: 2 }),
      avgP99Latency
    };
  }, [chartData]);

  const formatXAxis = (isoString: string) => {
    const date = new Date(isoString);
    if (range === '24h') {
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    }
    return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
  };

  if (summaryLoading) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-400 text-sm font-mono animate-pulse">
        Loading dashboard...
      </div>
    );
  }

  return (
    <div className="space-y-8 animate-slide-up">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight text-white">
            Payment Switch Global Dashboard
          </h2>
          <p className="text-gray-400 text-sm">
            Consolidated overview of all switch participants and clearings.
          </p>
        </div>

        <div className="flex items-center bg-[#0f172a] border border-[#1e293b] p-1 rounded-lg">
          {(['24h', '7d', '30d'] as const).map((r) => (
            <button
              key={r}
              onClick={() => setRange(r)}
              className={`px-3 py-1.5 rounded-md text-xs font-semibold uppercase tracking-wider transition-all duration-200
                ${range === r
                  ? 'bg-gradient-to-r from-indigo-600 to-violet-600 text-white shadow-md'
                  : 'text-gray-400 hover:text-white'}`}
            >
              {r}
            </button>
          ))}
        </div>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <div className="glass-panel p-6 rounded-2xl glow-indigo relative overflow-hidden group">
          <div className="absolute top-0 right-0 h-24 w-24 bg-indigo-500/5 rounded-bl-full group-hover:bg-indigo-500/10 transition-all duration-300"></div>
          <div className="flex justify-between items-start">
            <div className="space-y-2">
              <span className="text-xs font-mono uppercase tracking-wider text-gray-400">Total Transactions</span>
              <h3 className="text-3xl font-bold tracking-tight text-white">{stats.totalTx}</h3>
            </div>
            <div className="bg-indigo-500/10 p-3 rounded-xl border border-indigo-500/20">
              <Activity className="h-6 w-6 text-indigo-400 animate-pulse" />
            </div>
          </div>
          <div className="mt-4 flex items-center gap-1.5 text-xs text-emerald-400">
            <TrendingUp className="h-4 w-4" />
            <span>Across selected range</span>
          </div>
        </div>

        <div className="glass-panel p-6 rounded-2xl glow-emerald relative overflow-hidden group">
          <div className="absolute top-0 right-0 h-24 w-24 bg-emerald-500/5 rounded-bl-full group-hover:bg-emerald-500/10 transition-all duration-300"></div>
          <div className="flex justify-between items-start">
            <div className="space-y-2">
              <span className="text-xs font-mono uppercase tracking-wider text-gray-400">Cleared Value</span>
              <h3 className="text-2xl font-bold tracking-tight text-white truncate max-w-[160px]">
                {stats.totalAmt.split('.')[0]}
              </h3>
            </div>
            <div className="bg-emerald-500/10 p-3 rounded-xl border border-emerald-500/20">
              <DollarSign className="h-6 w-6 text-emerald-400" />
            </div>
          </div>
          <div className="mt-4 flex items-center gap-1.5 text-xs text-emerald-400">
            <TrendingUp className="h-4 w-4" />
            <span>Minor units / 100</span>
          </div>
        </div>

        <div className="glass-panel p-6 rounded-2xl glow-indigo relative overflow-hidden group">
          <div className="absolute top-0 right-0 h-24 w-24 bg-indigo-500/5 rounded-bl-full group-hover:bg-indigo-500/10 transition-all duration-300"></div>
          <div className="flex justify-between items-start">
            <div className="space-y-2">
              <span className="text-xs font-mono uppercase tracking-wider text-gray-400">Saga Success Rate</span>
              <h3 className="text-3xl font-bold tracking-tight text-emerald-400">
                {stats.successRate.toFixed(1)}%
              </h3>
            </div>
            <div className="bg-emerald-500/10 p-3 rounded-xl border border-emerald-500/20">
              <CheckCircle2 className="h-6 w-6 text-emerald-400" />
            </div>
          </div>
          <div className="mt-4 flex items-center gap-1.5 text-xs text-emerald-400">
            <TrendingUp className="h-4 w-4" />
            <span>Excellent SLA compliance</span>
          </div>
        </div>

        <div className="glass-panel p-6 rounded-2xl glow-rose relative overflow-hidden group">
          <div className="absolute top-0 right-0 h-24 w-24 bg-rose-500/5 rounded-bl-full group-hover:bg-rose-500/10 transition-all duration-300"></div>
          <div className="flex justify-between items-start">
            <div className="space-y-2">
              <span className="text-xs font-mono uppercase tracking-wider text-gray-400">p99 End-to-End Latency</span>
              <h3 className="text-3xl font-bold tracking-tight text-white">
                {stats.avgP99Latency > 0 ? `${(stats.avgP99Latency / 1000).toFixed(2)}s` : 'N/A'}
              </h3>
            </div>
            <div className="bg-rose-500/10 p-3 rounded-xl border border-rose-500/20">
              <Clock className="h-6 w-6 text-rose-400" />
            </div>
          </div>
          <div className="mt-4 flex items-center gap-1.5 text-xs text-rose-400">
            <TrendingDown className="h-4 w-4" />
            <span>Average across buckets</span>
          </div>
        </div>
      </div>

      {/* Charts Section */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="glass-panel p-6 rounded-2xl col-span-1 lg:col-span-2 space-y-4">
          <div className="flex justify-between items-center">
            <h3 className="text-base font-semibold text-white">Transaction Volumes & Value Trends</h3>
            <span className="text-xs text-gray-400 font-mono">Bucket: {range === '24h' ? '1 hour' : '1 day'}</span>
          </div>
          <div className="h-80 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <defs>
                  <linearGradient id="colorVolume" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.4} />
                    <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0.0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.03)" vertical={false} />
                <XAxis dataKey="periodStart" tickFormatter={formatXAxis} stroke="#475569" fontSize={10} tickLine={false} />
                <YAxis stroke="#475569" fontSize={10} tickLine={false} />
                <Tooltip
                  labelFormatter={(val) => `Period: ${new Date(val).toLocaleString()}`}
                  formatter={(value: any) => [value, 'Volume']}
                />
                <Area type="monotone" dataKey="totalTransactions" stroke="#8b5cf6" strokeWidth={2} fillOpacity={1} fill="url(#colorVolume)" />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div className="glass-panel p-6 rounded-2xl flex flex-col justify-between">
          <div className="space-y-1">
            <h3 className="text-base font-semibold text-white">Saga Abort Analysis</h3>
            <p className="text-xs text-gray-400">Main triggers for transaction cancellations.</p>
          </div>

          {abortBreakdown.total > 0 ? (
            <>
              <div className="h-48 flex justify-center items-center">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={abortBreakdown.reasons}
                      cx="50%" cy="50%"
                      innerRadius={55} outerRadius={75}
                      paddingAngle={3} dataKey="count"
                    >
                      {abortBreakdown.reasons.map((_entry, index) => (
                        <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip formatter={(count: any) => [count, 'Aborted']} />
                  </PieChart>
                </ResponsiveContainer>
              </div>
              <div className="space-y-2 mt-4">
                {abortBreakdown.reasons.map((item, idx) => (
                  <div key={item.category} className="flex items-center justify-between text-xs font-mono">
                    <div className="flex items-center gap-2">
                      <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: COLORS[idx % COLORS.length] }}></span>
                      <span className="capitalize text-gray-300">{item.category.replace(/_/g, ' ')}</span>
                    </div>
                    <div className="flex gap-3 text-right">
                      <span className="font-semibold text-white">{item.count}</span>
                      <span className="text-gray-500">({(item.percentage * 100).toFixed(0)}%)</span>
                    </div>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className="flex-1 flex flex-col items-center justify-center text-center space-y-2 text-gray-500 p-8">
              <CheckCircle2 className="h-8 w-8 text-emerald-500" />
              <p className="text-sm font-semibold">No Aborts Registered</p>
              <p className="text-xs">All sagas completed successfully.</p>
            </div>
          )}
        </div>
      </div>

      <div className="glass-panel p-6 rounded-2xl space-y-4">
        <h3 className="text-base font-semibold text-white">Clearing Performance Analysis</h3>
        <div className="h-60 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.03)" vertical={false} />
              <XAxis dataKey="periodStart" tickFormatter={formatXAxis} stroke="#475569" fontSize={10} tickLine={false} />
              <YAxis stroke="#475569" fontSize={10} tickLine={false} label={{ value: 'Latency (ms)', angle: -90, position: 'insideLeft', fill: '#475569', offset: 5 }} />
              <Tooltip
                labelFormatter={(val) => `Period: ${new Date(val).toLocaleString()}`}
                formatter={(value: any) => [value ? `${value} ms` : 'N/A', 'p99 SLA Latency']}
              />
              <Bar dataKey="p99LatencyMs" fill="#6366f1" radius={[4, 4, 0, 0]}>
                {chartData.map((entry, index) => (
                  <Cell
                    key={`cell-${index}`}
                    fill={entry.p99LatencyMs != null && entry.p99LatencyMs > 1500 ? '#f43f5e' : '#6366f1'}
                  />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
};
export default Dashboard;
