import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Activity,
  AlertCircle,
  BellOff,
  CheckCircle2,
  ChevronLeft,
  Clock,
  Database,
  Eye,
  Gauge,
  GitPullRequestArrow,
  LineChart,
  Loader2,
  RefreshCw,
  ShieldCheck,
  SlidersHorizontal,
  TrendingUp,
  Zap,
} from 'lucide-react';
import { apiClient } from '../api/client';
import { useStore } from '../store/useStore';
import type { DashboardIntegration, DashboardPrediction, DashboardState } from '../types';

type PredictionAction = 'watch' | 'dismiss' | 'mute' | 'sensitivity' | 'rerun';

export const Predictions = () => {
  const dashboard = useStore((state) => state.dashboard);
  const searchQuery = useStore((state) => state.searchQuery);
  const timeRange = useStore((state) => state.timeRange);
  const [riskFilter, setRiskFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const predictions = useMemo(
    () => filterPredictions(dashboard?.predictions || [], searchQuery, timeRange, riskFilter, statusFilter),
    [dashboard?.predictions, searchQuery, timeRange, riskFilter, statusFilter],
  );
  const predictionStats = useMemo(() => buildPredictionStats(dashboard?.predictions || []), [dashboard?.predictions]);

  return (
    <div className="min-h-[calc(100vh-76px)] space-y-4 p-3 sm:p-4">
      <header className="rounded-lg border border-[#e5e7eb] bg-white p-5 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex items-start gap-3">
            <span className="grid h-11 w-11 place-items-center rounded-lg bg-[#eff6ff] text-[#2563eb]">
              <LineChart className="h-5 w-5" />
            </span>
            <div>
              <h1 className="text-[20px] font-semibold text-[#111827]">Predictive Risk Signals</h1>
              <p className="mt-1 max-w-3xl text-[13px] leading-5 text-[#647084]">
                Early-warning operations view for memory growth, OOM risk, restart trends, throttling, disk pressure, and instability signals before they become incidents.
              </p>
            </div>
          </div>
          <PredictionReadiness integrations={dashboard?.integrations || []} predictions={dashboard?.predictions || []} />
        </div>
      </header>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-5">
        <KpiCard title="Active predictions" value={`${predictionStats.active}`} detail={`${predictionStats.total} total signals`} icon={<Activity className="h-4 w-4" />} tone="blue" />
        <KpiCard title="High-risk workloads" value={`${predictionStats.highRisk}`} detail="critical or high risk" icon={<AlertCircle className="h-4 w-4" />} tone="red" />
        <KpiCard title="Risk avoided" value={formatCurrency(predictionStats.riskAvoided)} detail="estimated hourly risk" icon={<ShieldCheck className="h-4 w-4" />} tone="green" />
        <KpiCard title="Prediction accuracy" value={predictionStats.accuracyLabel} detail="requires outcome history" icon={<Gauge className="h-4 w-4" />} tone="purple" />
        <KpiCard title="Waiting for data" value={`${predictionStats.waiting}`} detail="signals needing lookback" icon={<Clock className="h-4 w-4" />} tone="orange" />
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_390px]">
        <section className="rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
          <div className="border-b border-[#e5e7eb] px-4 py-3">
            <h2 className="text-[14px] font-semibold text-[#111827]">Trend Watch</h2>
            <p className="mt-0.5 text-[12px] text-[#647084]">Current predictive signals grouped by failure mode. Forecast lines appear when the backend returns metric series.</p>
          </div>
          <TrendGrid predictions={dashboard?.predictions || []} />
        </section>

        <section className="rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
          <div className="border-b border-[#e5e7eb] px-4 py-3">
            <h2 className="text-[14px] font-semibold text-[#111827]">Prediction Health</h2>
          </div>
          <ReadinessPanel dashboard={dashboard} />
        </section>
      </div>

      <section className="overflow-hidden rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
        <div className="flex flex-col gap-3 border-b border-[#e5e7eb] px-4 py-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 className="text-[14px] font-semibold text-[#111827]">Prediction Queue</h2>
            <p className="mt-0.5 text-[12px] text-[#647084]">Workloads ranked by early failure risk, confidence, evidence, time to impact, and action eligibility.</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <label className="inline-flex h-9 items-center gap-2 rounded-md border border-[#e5e7eb] px-3 text-[12px] font-medium text-[#374151]">
              <SlidersHorizontal className="h-4 w-4" />
              <select value={riskFilter} onChange={(event) => setRiskFilter(event.target.value)} className="bg-transparent outline-none">
                <option value="all">All risks</option>
                <option value="high">High risk</option>
                <option value="medium">Medium risk</option>
                <option value="low">Low risk</option>
              </select>
            </label>
            <label className="inline-flex h-9 items-center rounded-md border border-[#e5e7eb] px-3 text-[12px] font-medium text-[#374151]">
              <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} className="bg-transparent outline-none">
                <option value="all">All statuses</option>
                <option value="observing">Observing</option>
                <option value="candidate">Candidate</option>
                <option value="pr_opened">PR opened</option>
                <option value="dismissed">Dismissed</option>
              </select>
            </label>
          </div>
        </div>
        <PredictionTable rows={predictions} />
      </section>
    </div>
  );
};

export const PredictionDetails = () => {
  const { predictionId = '' } = useParams();
  const navigate = useNavigate();
  const { dashboard, setDashboard } = useStore();
  const [loading, setLoading] = useState(!dashboard);
  const [error, setError] = useState('');

  useEffect(() => {
    if (dashboard) return;
    let mounted = true;
    apiClient
      .get('/dashboard')
      .then(({ data }: { data: DashboardState }) => {
        if (!mounted) return;
        setDashboard(data);
        setError('');
      })
      .catch(() => {
        if (mounted) setError('Failed to load prediction details.');
      })
      .finally(() => {
        if (mounted) setLoading(false);
      });
    return () => {
      mounted = false;
    };
  }, [dashboard, setDashboard]);

  const decodedId = decodeURIComponent(predictionId);
  const prediction = (dashboard?.predictions || []).find((item) => item.id === decodedId) || null;

  if (loading && !dashboard) {
    return (
      <div className="grid min-h-[calc(100vh-76px)] place-items-center text-[#647084]">
        <div className="flex items-center gap-2 text-[14px]">
          <Loader2 className="h-5 w-5 animate-spin" />
          Loading prediction details...
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-[calc(100vh-76px)] space-y-4 p-3 sm:p-4">
      <button
        type="button"
        onClick={() => navigate('/predictions')}
        className="inline-flex h-9 items-center gap-2 rounded-md border border-[#e5e7eb] bg-white px-3 text-[12px] font-semibold text-[#374151] hover:bg-[#f8fafc]"
      >
        <ChevronLeft className="h-4 w-4" />
        Back to predictions
      </button>

      {error && !dashboard ? (
        <div className="rounded-lg border border-[#fecaca] bg-[#fef2f2] p-4 text-[13px] text-[#b91c1c]">{error}</div>
      ) : prediction ? (
        <PredictionDetailContent prediction={prediction} />
      ) : (
        <section className="rounded-lg border border-[#e5e7eb] bg-white">
          <EmptyState title="Prediction not found" message="This prediction is not present in the current dashboard snapshot. It may have expired, been dismissed, or become an incident." />
        </section>
      )}
    </div>
  );
};

const PredictionDetailContent = ({ prediction }: { prediction: DashboardPrediction }) => {
  const signal = predictionSignal(prediction);
  const status = prediction.status || 'observing';
  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-[#e5e7eb] bg-white p-5 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <RiskChip risk={prediction.risk} />
              <StatusChip status={status} />
              <span className="rounded-md bg-[#f1f5f9] px-2 py-1 text-[11px] font-semibold text-[#475569]">{signal}</span>
            </div>
            <h1 className="mt-3 text-[22px] font-semibold text-[#111827]">{prediction.namespace}/{prediction.podName}</h1>
            <p className="mt-1 text-[13px] text-[#647084]">{prediction.recommendedAction || recommendedAction(prediction)}</p>
          </div>
          <PredictionActions prediction={prediction} />
        </div>
      </section>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1.25fr)_minmax(360px,0.75fr)]">
        <div className="min-w-0 space-y-4">
          <DetailCard title="Forecast & Metric Samples" icon={<LineChart className="h-4 w-4" />}>
            <ForecastPanel prediction={prediction} large />
          </DetailCard>
          <DetailCard title="Evidence Chain" icon={<Database className="h-4 w-4" />}>
            <EvidencePanel prediction={prediction} />
          </DetailCard>
          <DetailCard title="AI Explanation" icon={<Zap className="h-4 w-4" />}>
            <div className="space-y-3 p-4 text-[13px] leading-6 text-[#475569]">
              <KeyValue label="Confidence" value={formatConfidence(prediction)} />
              <KeyValue label="Reason" value={prediction.confidenceReason || confidenceReason(prediction)} />
              <KeyValue label="Recommendation" value={prediction.recommendedAction || recommendedAction(prediction)} />
            </div>
          </DetailCard>
        </div>
        <div className="min-w-0 space-y-4">
          <DetailCard title="Suggested PR" icon={<GitPullRequestArrow className="h-4 w-4" />}>
            <div className="space-y-3 p-4 text-[13px]">
              <KeyValue label="Eligibility" value={prediction.autoFixEligible ? 'Auto-fix eligible' : 'Requires confirmation or more evidence'} />
              <KeyValue label="Patch type" value={patchTypeForPrediction(prediction)} />
              <KeyValue label="Risk avoided" value={formatCurrency(prediction.downtimeRiskHr || 0)} />
              <KeyValue label="Prevention cost" value={formatCurrency(prediction.preventionCostMo || 0)} />
            </div>
          </DetailCard>
          <DetailCard title="Outcome Feedback" icon={<CheckCircle2 className="h-4 w-4" />}>
            <div className="space-y-3 p-4 text-[13px]">
              <KeyValue label="Outcome" value={prediction.outcome || 'No outcome recorded yet'} />
              <KeyValue label="Accuracy state" value={prediction.outcome ? prediction.outcome : 'Pending validation'} />
              <KeyValue label="Last scan" value={prediction.lastScanAt ? formatTime(prediction.lastScanAt) : 'Not reported'} />
            </div>
          </DetailCard>
          <DetailCard title="Safe Actions" icon={<ShieldCheck className="h-4 w-4" />}>
            <div className="p-4">
              <PredictionActions prediction={prediction} vertical />
            </div>
          </DetailCard>
        </div>
      </div>
    </div>
  );
};

const PredictionTable = ({ rows }: { rows: DashboardPrediction[] }) => {
  const navigate = useNavigate();
  if (!rows.length) {
    return (
      <EmptyState
        title="No predictions available yet"
        message="Fixora did not find current future-risk candidates for the selected filters. Check Prediction Health for scanner, Prometheus, lookback, and metric coverage status."
      />
    );
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[1180px] text-left text-[12px]">
        <thead className="bg-[#f8fafc] text-[#475569]">
          <tr>
            <th className="px-4 py-3 font-semibold">Workload</th>
            <th className="px-4 py-3 font-semibold">Signal</th>
            <th className="px-4 py-3 font-semibold">Risk</th>
            <th className="px-4 py-3 font-semibold">Confidence</th>
            <th className="px-4 py-3 font-semibold">Evidence</th>
            <th className="px-4 py-3 font-semibold">Time to impact</th>
            <th className="px-4 py-3 font-semibold">Recommended action</th>
            <th className="px-4 py-3 font-semibold">Status</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-[#e5e7eb]">
          {rows.map((row) => (
            <tr key={row.id} onClick={() => navigate(`/predictions/${encodeURIComponent(row.id)}`)} className="cursor-pointer hover:bg-[#f8fafc]">
              <td className="px-4 py-3">
                <div className="font-semibold text-[#111827]">{row.namespace}/{row.podName}</div>
                <div className="text-[11px] text-[#647084]">last signal {row.lastAlertAge || 'unknown'}</div>
              </td>
              <td className="px-4 py-3 text-[#475569]">{predictionSignal(row)}</td>
              <td className="px-4 py-3"><RiskChip risk={row.risk} /></td>
              <td className="px-4 py-3">
                <ConfidenceBar value={predictionConfidence(row)} />
              </td>
              <td className="px-4 py-3">
                <span className="line-clamp-2 max-w-[260px] text-[#475569]" title={predictionEvidence(row)}>{predictionEvidence(row)}</span>
              </td>
              <td className="px-4 py-3 text-[#475569]">{row.timeToImpact || inferTimeToImpact(row)}</td>
              <td className="px-4 py-3">
                <span className="line-clamp-2 max-w-[280px] text-[#475569]" title={row.recommendedAction || recommendedAction(row)}>{row.recommendedAction || recommendedAction(row)}</span>
              </td>
              <td className="px-4 py-3"><StatusChip status={row.status || 'observing'} /></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

const TrendGrid = ({ predictions }: { predictions: DashboardPrediction[] }) => {
  const signalCards = [
    { id: 'memory', title: 'Memory Growth', matcher: /memory|oom|leak/i, icon: <TrendingUp className="h-4 w-4" /> },
    { id: 'restart', title: 'Restart Trend', matcher: /restart|crash/i, icon: <RefreshCw className="h-4 w-4" /> },
    { id: 'cpu', title: 'CPU Throttling', matcher: /cpu|throttl/i, icon: <Gauge className="h-4 w-4" /> },
    { id: 'disk', title: 'Disk Pressure', matcher: /disk|storage|volume|pvc/i, icon: <Database className="h-4 w-4" /> },
  ];
  return (
    <div className="grid gap-3 p-4 md:grid-cols-2">
      {signalCards.map((card) => {
        const rows = predictions.filter((prediction) => card.matcher.test(`${prediction.signalType || ''} ${prediction.risk || ''} ${prediction.recommendedAction || ''}`));
        return <TrendCard key={card.id} title={card.title} icon={card.icon} rows={rows} />;
      })}
    </div>
  );
};

const TrendCard = ({ title, icon, rows }: { title: string; icon: ReactNode; rows: DashboardPrediction[] }) => {
  const top = rows[0];
  return (
    <article className="rounded-lg border border-[#e5e7eb] bg-[#f8fafc] p-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-[13px] font-semibold text-[#111827]">{icon}{title}</div>
        <span className="rounded-full bg-white px-2 py-0.5 text-[11px] font-semibold text-[#647084]">{rows.length}</span>
      </div>
      {top ? (
        <div className="mt-3">
          <ForecastPanel prediction={top} />
          <div className="mt-2 truncate text-[12px] font-medium text-[#111827]">{top.namespace}/{top.podName}</div>
          <div className="text-[11px] text-[#647084]">{predictionEvidence(top)}</div>
        </div>
      ) : (
        <div className="mt-3 rounded-md border border-dashed border-[#cbd5e1] bg-white p-4 text-[12px] text-[#647084]">
          No active {title.toLowerCase()} predictions for this time range.
        </div>
      )}
    </article>
  );
};

const ForecastPanel = ({ prediction, large = false }: { prediction: DashboardPrediction; large?: boolean }) => {
  const series = prediction.metricSeries?.length ? prediction.metricSeries : prediction.forecastSeries || [];
  if (!series.length) {
    const growth = Math.max(0, Math.min(prediction.lastGrowthRate || 0, 1));
    return (
      <div className={`rounded-md border border-[#e5e7eb] bg-white p-3 ${large ? 'min-h-[220px]' : ''}`}>
        <div className="mb-2 flex items-center justify-between text-[11px] text-[#647084]">
          <span>Growth signal</span>
          <span>{Math.round(growth * 100)}%</span>
        </div>
        <div className="h-2 overflow-hidden rounded-full bg-[#e5e7eb]">
          <div className="h-full rounded-full bg-[#2563eb]" style={{ width: `${Math.round(growth * 100)}%` }} />
        </div>
        {large && (
          <div className="mt-8 grid min-h-[120px] place-items-center rounded-md border border-dashed border-[#cbd5e1] bg-[#f8fafc] px-4 text-center text-[12px] leading-5 text-[#647084]">
            Forecast series are not available in this dashboard payload yet. Backend can populate metricSeries and forecastSeries for full charts.
          </div>
        )}
      </div>
    );
  }
  return (
    <div className={`rounded-md border border-[#e5e7eb] bg-white p-3 ${large ? 'min-h-[220px]' : ''}`}>
      <svg viewBox="0 0 220 72" className="h-24 w-full overflow-visible">
        <polyline points={sparklinePoints(series, 220, 72)} fill="none" stroke="#2563eb" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </div>
  );
};

const ReadinessPanel = ({ dashboard }: { dashboard: DashboardState | null }) => {
  const prometheus = dashboard?.integrations?.find((item) => /prometheus/i.test(item.name));
  const predictions = dashboard?.predictions || [];
  const watched = dashboard?.workloads?.length || dashboard?.incidents?.length || 0;
  return (
    <div className="space-y-3 p-4 text-[12px]">
      <ReadinessRow label="Prometheus" value={prometheus?.status || 'unknown'} good={/ok|healthy/i.test(prometheus?.status || '')} detail={prometheus?.detail} />
      <ReadinessRow label="Prediction scanner" value="configured by backend" good />
      <ReadinessRow label="Required lookback" value="from backend settings" good={false} detail="Expose requiredLookback for exact scanner window." />
      <ReadinessRow label="Workloads watched" value={`${watched}`} good={watched > 0} />
      <ReadinessRow label="Last scan" value={latestPredictionScan(predictions)} good={predictions.some((item) => item.lastScanAt)} />
      <ReadinessRow label="Current result" value={predictions.length ? `${predictions.length} active signal${predictions.length === 1 ? '' : 's'}` : 'no risks found'} good />
    </div>
  );
};

const PredictionReadiness = ({ integrations, predictions }: { integrations: DashboardIntegration[]; predictions: DashboardPrediction[] }) => {
  const prometheus = integrations.find((item) => /prometheus/i.test(item.name));
  const healthy = /ok|healthy/i.test(prometheus?.status || '');
  return (
    <div className={`rounded-lg border px-3 py-2 text-[12px] ${healthy ? 'border-[#bbf7d0] bg-[#f0fdf4] text-[#15803d]' : 'border-[#fed7aa] bg-[#fff7ed] text-[#9a3412]'}`}>
      <div className="font-semibold">{healthy ? 'Prediction inputs ready' : 'Prediction inputs limited'}</div>
      <div>{prometheus?.name || 'Prometheus'}: {prometheus?.status || 'unknown'} · {predictions.length} active</div>
    </div>
  );
};

const PredictionActions = ({ prediction, vertical = false }: { prediction: DashboardPrediction; vertical?: boolean }) => {
  const [selectedAction, setSelectedAction] = useState<PredictionAction | null>(null);
  const actions: Array<{ id: PredictionAction; label: string; icon: ReactNode; enabled: boolean; hint: string }> = [
    { id: 'watch', label: 'Watch only', icon: <Eye className="h-3.5 w-3.5" />, enabled: true, hint: 'Keep observing this prediction without opening a PR.' },
    { id: 'dismiss', label: 'Dismiss', icon: <CheckCircle2 className="h-3.5 w-3.5" />, enabled: false, hint: 'Needs backend action endpoint.' },
    { id: 'mute', label: 'Mute workload', icon: <BellOff className="h-3.5 w-3.5" />, enabled: false, hint: 'Needs backend policy endpoint.' },
    { id: 'sensitivity', label: 'Increase sensitivity', icon: <SlidersHorizontal className="h-3.5 w-3.5" />, enabled: false, hint: 'Needs backend scanner policy endpoint.' },
    { id: 'rerun', label: 'Re-run scan', icon: <RefreshCw className="h-3.5 w-3.5" />, enabled: false, hint: 'Needs backend scan trigger endpoint.' },
  ];
  return (
    <div className={`flex ${vertical ? 'flex-col' : 'flex-wrap'} gap-2`}>
      {actions.map((action) => (
        <button
          key={action.id}
          type="button"
          disabled={!action.enabled}
          onClick={() => setSelectedAction(action.id)}
          title={action.enabled ? action.hint : `${action.hint} (${prediction.id})`}
          className={`inline-flex h-8 items-center justify-center gap-1.5 rounded-md border px-2.5 text-[12px] font-semibold ${
            action.enabled
              ? selectedAction === action.id
                ? 'border-[#2563eb] bg-[#eff6ff] text-[#2563eb]'
                : 'border-[#e5e7eb] bg-white text-[#374151] hover:bg-[#f8fafc]'
              : 'cursor-not-allowed border-[#e5e7eb] bg-[#f8fafc] text-[#94a3b8]'
          }`}
        >
          {action.icon}
          {action.label}
        </button>
      ))}
    </div>
  );
};

const EvidencePanel = ({ prediction }: { prediction: DashboardPrediction }) => {
  const evidence = prediction.evidence || [];
  const rows = evidence.length ? evidence : [
    { label: 'Risk', value: prediction.risk, icon: 'risk' },
    { label: 'Growth', value: `${Math.round((prediction.lastGrowthRate || 0) * 100)}% recent growth`, icon: 'trend' },
    { label: 'Age', value: prediction.lastAlertAge || 'unknown', icon: 'clock' },
  ];
  return (
    <div className="divide-y divide-[#e5e7eb]">
      {rows.map((item) => (
        <div key={`${item.label}-${item.value}`} className="grid grid-cols-[120px_1fr] gap-3 px-4 py-3 text-[13px]">
          <span className="font-semibold text-[#111827]">{item.label}</span>
          <span className="break-words text-[#475569]">{item.value}</span>
        </div>
      ))}
    </div>
  );
};

const DetailCard = ({ title, icon, children }: { title: string; icon: ReactNode; children: ReactNode }) => (
  <section className="overflow-hidden rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
    <header className="flex h-11 items-center gap-2 border-b border-[#e5e7eb] px-4 text-[13px] font-semibold text-[#111827]">{icon}{title}</header>
    {children}
  </section>
);

const KpiCard = ({ title, value, detail, icon, tone }: { title: string; value: string; detail: string; icon: ReactNode; tone: 'blue' | 'red' | 'green' | 'purple' | 'orange' }) => {
  const classes = {
    blue: 'bg-[#eff6ff] text-[#2563eb] border-[#bfdbfe]',
    red: 'bg-[#fef2f2] text-[#dc2626] border-[#fecaca]',
    green: 'bg-[#f0fdf4] text-[#15803d] border-[#bbf7d0]',
    purple: 'bg-[#f5f3ff] text-[#7c3aed] border-[#ddd6fe]',
    orange: 'bg-[#fff7ed] text-[#ea580c] border-[#fed7aa]',
  }[tone];
  return (
    <article className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
      <div className={`inline-flex items-center gap-2 rounded-md border px-2 py-1 text-[12px] font-semibold ${classes}`}>{icon}{title}</div>
      <div className="mt-3 text-[26px] font-semibold leading-none text-[#111827]">{value}</div>
      <div className="mt-2 text-[12px] text-[#647084]">{detail}</div>
    </article>
  );
};

const RiskChip = ({ risk }: { risk: string }) => {
  const normalized = risk.toLowerCase();
  const classes = /critical|high|oom|leak/.test(normalized)
    ? 'border-[#fecaca] bg-[#fee2e2] text-[#dc2626]'
    : /medium|warning/.test(normalized)
      ? 'border-[#fed7aa] bg-[#ffedd5] text-[#ea580c]'
      : 'border-[#bbf7d0] bg-[#dcfce7] text-[#15803d]';
  return <span className={`inline-flex rounded-md border px-2 py-1 text-[11px] font-semibold ${classes}`}>{risk || 'unknown'}</span>;
};

const StatusChip = ({ status }: { status: string }) => {
  const normalized = status.toLowerCase();
  const classes = /dismissed|expired/.test(normalized)
    ? 'bg-[#f1f5f9] text-[#647084]'
    : /pr|candidate/.test(normalized)
      ? 'bg-[#eff6ff] text-[#2563eb]'
      : 'bg-[#f0fdf4] text-[#15803d]';
  return <span className={`rounded-md px-2 py-1 text-[11px] font-semibold ${classes}`}>{status}</span>;
};

const ConfidenceBar = ({ value }: { value: number }) => (
  <div className="flex min-w-[120px] items-center gap-2">
    <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-[#e5e7eb]">
      <div className="h-full rounded-full bg-[#16a34a]" style={{ width: `${Math.max(0, Math.min(value, 100))}%` }} />
    </div>
    <span className="w-9 text-right font-semibold text-[#475569]">{value}%</span>
  </div>
);

const ReadinessRow = ({ label, value, good, detail }: { label: string; value: string; good: boolean; detail?: string }) => (
  <div className="rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-3">
    <div className="flex items-center justify-between gap-3">
      <span className="font-semibold text-[#111827]">{label}</span>
      <span className={`rounded-md px-2 py-0.5 text-[11px] font-semibold ${good ? 'bg-[#dcfce7] text-[#15803d]' : 'bg-[#fff7ed] text-[#ea580c]'}`}>{value}</span>
    </div>
    {detail && <div className="mt-1 text-[#647084]">{detail}</div>}
  </div>
);

const EmptyState = ({ title, message }: { title: string; message: string }) => (
  <div className="grid min-h-[300px] place-items-center px-6 py-12 text-center">
    <div className="max-w-xl rounded-lg border border-dashed border-[#cbd5e1] bg-[#f8fafc] px-6 py-6">
      <div className="mx-auto grid h-12 w-12 place-items-center rounded-lg bg-white text-[#94a3b8] shadow-sm">
        <Database className="h-6 w-6" />
      </div>
      <h2 className="mt-4 text-[15px] font-semibold text-[#111827]">{title}</h2>
      <p className="mt-1 text-[13px] leading-5 text-[#647084]">{message}</p>
    </div>
  </div>
);

const KeyValue = ({ label, value }: { label: string; value: string }) => (
  <div className="grid grid-cols-[130px_1fr] gap-3">
    <span className="font-semibold text-[#111827]">{label}</span>
    <span className="min-w-0 break-words text-[#475569]">{value || 'Unknown'}</span>
  </div>
);

const filterPredictions = (rows: DashboardPrediction[], query: string, range: string, risk: string, status: string) => {
  const normalizedQuery = query.trim().toLowerCase();
  return rows.filter((row) => {
    if (range !== 'All time' && !withinTimeRange(row.lastAlertAge, range)) return false;
    if (risk !== 'all' && !row.risk.toLowerCase().includes(risk)) return false;
    if (status !== 'all' && (row.status || 'observing').toLowerCase() !== status) return false;
    if (!normalizedQuery) return true;
    return [
      row.namespace,
      row.podName,
      row.risk,
      row.signalType,
      row.recommendedAction,
      row.status,
      row.outcome,
    ].filter(Boolean).join(' ').toLowerCase().includes(normalizedQuery);
  });
};

const buildPredictionStats = (rows: DashboardPrediction[]) => {
  const outcomes = rows.filter((row) => row.outcome);
  const positive = outcomes.filter((row) => /true_positive|prevented|validated|success/i.test(row.outcome || '')).length;
  return {
    total: rows.length,
    active: rows.filter((row) => !/dismissed|expired/i.test(row.status || '')).length,
    highRisk: rows.filter((row) => /critical|high|oom|leak/i.test(row.risk)).length,
    riskAvoided: rows.reduce((sum, row) => sum + (row.downtimeRiskHr || 0), 0),
    waiting: rows.filter((row) => /waiting|insufficient/i.test(`${row.status || ''} ${row.metricCoverage || ''}`)).length,
    accuracyLabel: outcomes.length ? `${Math.round((positive / outcomes.length) * 100)}%` : 'n/a',
  };
};

const predictionSignal = (prediction: DashboardPrediction) => {
  if (prediction.signalType) return prediction.signalType;
  const text = `${prediction.risk} ${prediction.recommendedAction || ''}`.toLowerCase();
  if (/oom|memory|leak/.test(text)) return 'Memory growth';
  if (/restart|crash/.test(text)) return 'Restart trend';
  if (/cpu|throttl/.test(text)) return 'CPU throttling';
  if (/disk|storage|pvc|volume/.test(text)) return 'Disk pressure';
  return 'Runtime risk';
};

const predictionConfidence = (prediction: DashboardPrediction) => {
  if (prediction.confidence !== undefined) return prediction.confidence;
  const growth = prediction.lastGrowthRate || 0;
  if (growth >= 0.75) return 85;
  if (growth >= 0.5) return 70;
  if (growth > 0) return 55;
  return 0;
};

const predictionEvidence = (prediction: DashboardPrediction) => {
  if (prediction.evidence?.length) return prediction.evidence.map((item) => `${item.label}: ${item.value}`).join(' · ');
  if (prediction.lastGrowthRate > 0) return `Growth rate ${Math.round(prediction.lastGrowthRate * 100)}%, last signal ${prediction.lastAlertAge || 'unknown'}`;
  return 'Metric evidence not included in dashboard payload yet.';
};

const recommendedAction = (prediction: DashboardPrediction) => {
  const signal = predictionSignal(prediction).toLowerCase();
  if (signal.includes('memory')) return 'Review memory trend and open a right-sizing or leak-prevention PR only after forecast evidence is available.';
  if (signal.includes('restart')) return 'Inspect restart pattern, recent deploys, and logs before opening a remediation PR.';
  if (signal.includes('cpu')) return 'Check throttling and CPU requests before recommending resource changes.';
  if (signal.includes('disk')) return 'Review volume growth and retention before recommending storage changes.';
  return 'Continue observing until enough evidence is available for a safe remediation.';
};

const confidenceReason = (prediction: DashboardPrediction) => {
  if (prediction.confidence !== undefined) return 'Provided by backend prediction analyzer.';
  if (prediction.lastGrowthRate > 0) return 'Estimated from the last growth-rate signal until backend exposes confidenceReason.';
  return 'Confidence is unavailable because metric series were not included.';
};

const inferTimeToImpact = (prediction: DashboardPrediction) => {
  if (!prediction.lastGrowthRate) return 'Not enough data';
  if (prediction.lastGrowthRate >= 0.75) return '< 24h if trend continues';
  if (prediction.lastGrowthRate >= 0.5) return '1-3d if trend continues';
  return 'watching';
};

const patchTypeForPrediction = (prediction: DashboardPrediction) => {
  const signal = predictionSignal(prediction).toLowerCase();
  if (signal.includes('memory') || signal.includes('cpu')) return 'resources';
  if (signal.includes('disk')) return 'storage policy';
  return 'manual review';
};

const latestPredictionScan = (rows: DashboardPrediction[]) => {
  const latest = rows.map((row) => row.lastScanAt).filter(Boolean).sort().at(-1);
  return latest ? formatTime(latest) : 'not reported';
};

const sparklinePoints = (series: number[], width: number, height: number) => {
  const min = Math.min(...series);
  const max = Math.max(...series);
  const span = max - min || 1;
  return series.map((value, index) => {
    const x = series.length === 1 ? width : (index / (series.length - 1)) * width;
    const y = height - ((value - min) / span) * height;
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  }).join(' ');
};

const withinTimeRange = (age: string, range: string) => {
  if (!age || range === 'All time') return true;
  const minutes = ageToMinutes(age);
  if (minutes === null) return true;
  if (range.includes('24h')) return minutes <= 24 * 60;
  if (range.includes('7d')) return minutes <= 7 * 24 * 60;
  if (range.includes('30d')) return minutes <= 30 * 24 * 60;
  return true;
};

const ageToMinutes = (age: string) => {
  const match = age.match(/(\d+)\s*(m|h|d)/i);
  if (!match) return null;
  const value = Number(match[1]);
  if (match[2].toLowerCase() === 'm') return value;
  if (match[2].toLowerCase() === 'h') return value * 60;
  return value * 24 * 60;
};

const formatCurrency = (value: number) => {
  if (!Number.isFinite(value) || value <= 0) return '$0.00';
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    maximumFractionDigits: value >= 100 ? 0 : 2,
  }).format(value);
};

const formatConfidence = (prediction: DashboardPrediction) => `${predictionConfidence(prediction)}%`;

const formatTime = (value: string) => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
};
