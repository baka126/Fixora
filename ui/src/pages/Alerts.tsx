import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import {
  ChevronLeft,
  ChevronRight,
  Eye,
  Loader2,
  RefreshCw,
  ShieldCheck,
  TriangleAlert,
} from 'lucide-react';
import { apiClient } from '../api/client';
import { useStore } from '../store/useStore';
import type { DashboardActiveAlert } from '../types';

const pageSize = 10;

export const Alerts = () => {
  const searchQuery = useStore((state) => state.searchQuery);
  const timeRange = useStore((state) => state.timeRange);
  const [alerts, setAlerts] = useState<DashboardActiveAlert[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [page, setPage] = useState(1);
  const [includingId, setIncludingId] = useState<string | null>(null);

  const loadAlerts = (showRefresh = false) => {
    if (showRefresh) setRefreshing(true);
    setError('');
    apiClient
      .get('/alerts/active')
      .then(({ data }: { data: DashboardActiveAlert[] }) => setAlerts(Array.isArray(data) ? data : []))
      .catch((err) => {
        if (err.response?.status === 503) {
          setAlerts([]);
          setError('Alertmanager is not configured for active alert polling.');
          return;
        }
        setError('Failed to load active Alertmanager alerts.');
      })
      .finally(() => {
        setLoading(false);
        setRefreshing(false);
      });
  };

  useEffect(() => {
    let mounted = true;
    apiClient
      .get('/alerts/active')
      .then(({ data }: { data: DashboardActiveAlert[] }) => {
        if (mounted) setAlerts(Array.isArray(data) ? data : []);
      })
      .catch((err) => {
        if (!mounted) return;
        if (err.response?.status === 503) {
          setAlerts([]);
          setError('Alertmanager is not configured for active alert polling.');
          return;
        }
        setError('Failed to load active Alertmanager alerts.');
      })
      .finally(() => {
        if (mounted) setLoading(false);
      });
    return () => {
      mounted = false;
    };
  }, []);

  const filtered = useMemo(() => filterAlerts(alerts, searchQuery, timeRange), [alerts, searchQuery, timeRange]);
  const used = alerts.filter((alert) => alert.used).length;
  const notUsed = alerts.filter((alert) => !alert.used && alert.decision !== 'deduplicated').length;
  const deduped = alerts.filter((alert) => alert.decision === 'deduplicated').length;
  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize));
  const safePage = Math.min(Math.max(page, 1), pageCount);
  const pageAlerts = filtered.slice((safePage - 1) * pageSize, safePage * pageSize);

  const includeAlert = (alert: DashboardActiveAlert) => {
    setIncludingId(alert.id);
    setError('');
    apiClient
      .post(`/alerts/active/${encodeURIComponent(alert.id)}/include`)
      .then(({ data }: { data: DashboardActiveAlert }) => {
        setAlerts((current) => current.map((item) => (item.id === data.id ? data : item)));
      })
      .catch((err) => {
        setError(err.response?.data || alert.includeReason || 'Fixora could not include this alert.');
      })
      .finally(() => setIncludingId(null));
  };

  if (loading) {
    return (
      <div className="grid min-h-[calc(100vh-140px)] place-items-center text-[#647084]">
        <div className="flex items-center gap-2 text-[14px]">
          <Loader2 className="h-5 w-5 animate-spin" />
          Loading Alertmanager alerts...
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4 p-3 sm:p-4">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-[20px] font-semibold text-[#111827]">Alertmanager Active Alerts</h1>
          <p className="mt-1 text-[13px] text-[#647084]">Review scraped alerts, see why Fixora is or is not using them, and add safe alerts to the runtime watch list.</p>
        </div>
        <button
          onClick={() => loadAlerts(true)}
          disabled={refreshing}
          className="inline-flex h-9 items-center gap-2 rounded-md border border-[#e5e7eb] bg-white px-3 text-[12px] font-medium text-[#111827] hover:bg-[#f8fafc] disabled:opacity-60"
        >
          <RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </header>

      <div className="grid gap-3 md:grid-cols-3">
        <StatCard label="Used by Fixora" value={used} tone="text-[#15803d]" />
        <StatCard label="Not used" value={notUsed} tone="text-[#dc2626]" />
        <StatCard label="Deduplicated" value={deduped} tone="text-[#2563eb]" />
      </div>

      {error && (
        <div className="flex items-start gap-3 rounded-md border border-[#fed7aa] bg-[#fff7ed] p-3 text-[13px] text-[#9a3412]">
          <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <section className="overflow-hidden rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[#e5e7eb] px-4 py-3">
          <div className="text-[13px] text-[#647084]">
            Showing {filtered.length ? (safePage - 1) * pageSize + 1 : 0} to {Math.min(safePage * pageSize, filtered.length)} of {filtered.length} alerts
            {filtered.length !== alerts.length && <span> filtered from {alerts.length}</span>}
          </div>
          <div className="flex items-center gap-1">
            <PagerButton disabled={safePage === 1} onClick={() => setPage(safePage - 1)} icon={<ChevronLeft className="h-4 w-4" />} />
            {Array.from({ length: pageCount }, (_, index) => index + 1).slice(0, 7).map((pageNumber) => (
              <PagerButton key={pageNumber} label={`${pageNumber}`} active={pageNumber === safePage} onClick={() => setPage(pageNumber)} />
            ))}
            <PagerButton disabled={safePage === pageCount} onClick={() => setPage(safePage + 1)} icon={<ChevronRight className="h-4 w-4" />} />
          </div>
        </div>
        {pageAlerts.length ? (
          <div className="max-h-[calc(100vh-330px)] min-h-[360px] overflow-auto">
            <table className="w-full min-w-[1100px] table-fixed text-left text-[12px]">
              <thead className="sticky top-0 z-[1] bg-[#f8fafc] text-[#111827]">
                <tr className="border-b border-[#e5e7eb]">
                  <th className="w-[18%] px-3 py-3 font-semibold">Alert</th>
                  <th className="w-[18%] px-3 py-3 font-semibold">Resource</th>
                  <th className="w-[10%] px-3 py-3 font-semibold">State</th>
                  <th className="w-[12%] px-3 py-3 font-semibold">Fixora</th>
                  <th className="w-[29%] px-3 py-3 font-semibold">Reason</th>
                  <th className="w-[8%] px-3 py-3 font-semibold">Age</th>
                  <th className="w-[12%] px-3 py-3 font-semibold">Action</th>
                </tr>
              </thead>
              <tbody>
                {pageAlerts.map((alert) => (
                  <tr key={alert.id} className="border-b border-[#eef2f7] last:border-b-0 hover:bg-[#f8fafc]">
                    <td className="px-3 py-3 align-top">
                      <div className="flex min-w-0 items-start gap-2">
                        <TriangleAlert className={`mt-0.5 h-4 w-4 shrink-0 ${alert.used ? 'text-[#16a34a]' : 'text-[#f97316]'}`} />
                        <div className="min-w-0">
                          <div className="truncate font-semibold text-[#111827]">{alert.alertName}</div>
                          <div className="mt-1 flex items-center gap-1">
                            <SeverityPill value={alert.severity} />
                            {alert.namespace && <span className="truncate text-[11px] text-[#647084]">{alert.namespace}</span>}
                          </div>
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-3 align-top">
                      <div className="truncate font-medium text-[#111827]">{alert.resourceKind}/{alert.resourceName}</div>
                      {alert.podName && alert.resourceName !== alert.podName && (
                        <div className="mt-0.5 truncate text-[11px] text-[#647084]">Pod/{alert.podName}</div>
                      )}
                      {!!alert.labels?.length && (
                        <div className="mt-2 flex flex-wrap gap-1">
                          {alert.labels.slice(0, 4).map((label) => (
                            <span key={`${alert.id}-${label.key}`} className="rounded bg-[#f1f5f9] px-1.5 py-0.5 text-[10px] text-[#475569]">
                              {label.key}:{label.value}
                            </span>
                          ))}
                        </div>
                      )}
                    </td>
                    <td className="px-3 py-3 align-top">
                      <span className="rounded-md bg-[#f1f5f9] px-2 py-1 text-[11px] font-medium text-[#475569]">{alert.status}</span>
                    </td>
                    <td className="px-3 py-3 align-top">
                      <AlertDecisionPill alert={alert} />
                    </td>
                    <td className="px-3 py-3 align-top">
                      <div className="line-clamp-2 text-[#334155]" title={alert.reason}>{alert.reason}</div>
                      {alert.summary && <div className="mt-1 line-clamp-1 text-[11px] text-[#647084]" title={alert.summary}>{alert.summary}</div>}
                    </td>
                    <td className="px-3 py-3 align-top font-medium text-[#647084]">{alert.age || 'now'}</td>
                    <td className="px-3 py-3 align-top">
                      {alert.canInclude ? (
                        <button
                          onClick={() => includeAlert(alert)}
                          disabled={includingId === alert.id}
                          className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[#bbf7d0] bg-[#f0fdf4] px-2.5 text-[11px] font-semibold text-[#15803d] disabled:opacity-60"
                          title={alert.includeReason || 'Watch this alert'}
                        >
                          {includingId === alert.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Eye className="h-3.5 w-3.5" />}
                          Watch
                        </button>
                      ) : (
                        <span className="text-[11px] text-[#94a3b8]" title={alert.includeReason || undefined}>{alert.used ? 'Watching' : 'Unavailable'}</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="grid min-h-[360px] place-items-center px-6 py-12 text-center">
            <div className="max-w-md">
              <ShieldCheck className="mx-auto h-8 w-8 text-[#94a3b8]" />
              <h3 className="mt-3 text-[15px] font-semibold text-[#111827]">No active alerts found</h3>
              <p className="mt-1 text-[13px] text-[#647084]">Alerts will appear here as Alertmanager reports active firing or suppressed alerts.</p>
            </div>
          </div>
        )}
      </section>
    </div>
  );
};

const filterAlerts = (alerts: DashboardActiveAlert[], query: string, range: string) => {
  const normalizedQuery = query.trim().toLowerCase();
  const maxAgeMinutes = timeRangeToMinutes(range);
  return alerts.filter((alert) => {
    if (maxAgeMinutes !== Infinity && ageToMinutes(alert.age) > maxAgeMinutes) return false;
    if (!normalizedQuery) return true;
    const haystack = [
      alert.alertName,
      alert.severity,
      alert.namespace,
      alert.resourceKind,
      alert.resourceName,
      alert.podName,
      alert.status,
      alert.decision,
      alert.reason,
      alert.summary,
      ...(alert.labels || []).flatMap((label) => [label.key, label.value]),
    ].filter(Boolean).join(' ').toLowerCase();
    return haystack.includes(normalizedQuery);
  });
};

const timeRangeToMinutes = (range: string) => {
  switch (range) {
    case 'Last 1h':
      return 60;
    case 'Last 6h':
      return 360;
    case 'Last 7d':
      return 10080;
    case 'Last 30d':
      return 43200;
    case 'All time':
      return Infinity;
    case 'Last 24h':
    default:
      return 1440;
  }
};

const ageToMinutes = (age: string) => {
  const match = (age || '').trim().toLowerCase().match(/^(\d+)\s*(m|h|d)$/);
  if (!match) return 0;
  const value = Number(match[1]);
  if (match[2] === 'd') return value * 1440;
  if (match[2] === 'h') return value * 60;
  return value;
};

const AlertDecisionPill = ({ alert }: { alert: DashboardActiveAlert }) => {
  const tone = alert.used
    ? 'bg-[#dcfce7] text-[#15803d]'
    : alert.decision === 'deduplicated'
      ? 'bg-[#eff6ff] text-[#2563eb]'
      : 'bg-[#fee2e2] text-[#dc2626]';
  const label = alert.used ? 'Used' : alert.decision === 'deduplicated' ? 'Deduped' : 'Not used';
  return <span className={`rounded-md px-2 py-1 text-[11px] font-semibold ${tone}`}>{label}</span>;
};

const SeverityPill = ({ value }: { value: string }) => {
  const normalized = (value || 'unknown').toLowerCase();
  const tone = normalized === 'critical' || normalized === 'page'
    ? 'bg-[#fee2e2] text-[#dc2626]'
    : normalized === 'warning'
      ? 'bg-[#ffedd5] text-[#ea580c]'
      : 'bg-[#f1f5f9] text-[#475569]';
  return <span className={`rounded px-1.5 py-0.5 text-[10px] font-semibold ${tone}`}>{value || 'unknown'}</span>;
};

const StatCard = ({ label, value, tone }: { label: string; value: number; tone: string }) => (
  <div className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
    <div className="text-[12px] font-semibold text-[#647084]">{label}</div>
    <div className={`mt-2 text-3xl font-semibold ${tone}`}>{value}</div>
  </div>
);

const PagerButton = ({
  icon,
  label,
  active,
  disabled,
  onClick,
}: {
  icon?: ReactNode;
  label?: string;
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
}) => (
  <button
    disabled={disabled}
    onClick={onClick}
    className={`grid h-8 min-w-8 place-items-center rounded-md border px-2 text-[12px] ${
      active ? 'border-[#94a3b8] bg-white text-[#111827]' : 'border-[#e5e7eb] bg-white text-[#4b5563]'
    } disabled:cursor-not-allowed disabled:opacity-40`}
  >
    {icon || label}
  </button>
);
