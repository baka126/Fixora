import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Activity,
  AlertCircle,
  Boxes,
  Brain,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronsUpDown,
  Circle,
  Code2,
  Container,
  Database,
  ExternalLink,
  FileCode2,
  FileText,
  Filter,
  GitPullRequestArrow,
  Layers3,
  Loader2,
  Network,
  RefreshCw,
  Server,
  ShieldCheck,
  TriangleAlert,
} from 'lucide-react';
import { apiClient } from '../api/client';
import { useStore } from '../store/useStore';
import { InteractiveGraph } from '../components/InteractiveGraph';
import { RecurringIssuesWidget } from '../components/RecurringIssuesWidget';
import { RemediationActions } from '../components/RemediationActions';
import { remediationManualActions } from '../utils/remediationActions';
import type {
  DashboardEvidence,
  DashboardGitOpsMapping,
  DashboardGuardrail,
  DashboardIncident,
  DashboardKPI,
  DashboardPipelineStage,
  DashboardRecommendedPR,
  DashboardRemediation,
  DashboardState,
  DashboardWorkload,
  InvestigationDetail,
} from '../types';

const toneStyles: Record<string, { text: string; border: string; bg: string; line: string }> = {
  critical: { text: 'text-[#dc2626]', border: 'border-[#fecaca]', bg: 'bg-[#fef2f2]', line: '#ef4444' },
  warning: { text: 'text-[#f97316]', border: 'border-[#fed7aa]', bg: 'bg-[#fff7ed]', line: '#f97316' },
  success: { text: 'text-[#15803d]', border: 'border-[#bbf7d0]', bg: 'bg-[#f0fdf4]', line: '#16a34a' },
  info: { text: 'text-[#2563eb]', border: 'border-[#bfdbfe]', bg: 'bg-[#eff6ff]', line: '#2563eb' },
};

export const Dashboard = () => {
  const { dashboard, selectedCluster, searchQuery, timeRange, setDashboard } = useStore();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(!dashboard);
  const [error, setError] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(dashboard?.incidents?.[0]?.id || null);
  const [refreshing, setRefreshing] = useState(false);
  const [severityFilter, setSeverityFilter] = useState('all');
  const [kindFilter, setKindFilter] = useState('all');
  const [highConfidenceOnly, setHighConfidenceOnly] = useState(false);
  const [incidentPage, setIncidentPage] = useState(1);

  const refreshDashboard = (showRefresh = true) => {
    if (showRefresh) setRefreshing(true);
    return apiClient
      .get('/dashboard')
      .then(({ data }: { data: DashboardState }) => {
        setDashboard(data);
        setSelectedId((current) => current || data.incidents?.[0]?.id || null);
        setError('');
      })
      .catch(() => {
        setError('Failed to load dashboard data.');
      })
      .finally(() => {
        setLoading(false);
        setRefreshing(false);
      });
  };

  useEffect(() => {
    let mounted = true;
    apiClient
      .get('/dashboard')
      .then(({ data }: { data: DashboardState }) => {
        if (!mounted) return;
        setDashboard(data);
        setSelectedId((current) => current || data.incidents?.[0]?.id || null);
        setError('');
      })
      .catch(() => {
        if (mounted) setError('Failed to load dashboard data.');
      })
      .finally(() => {
        if (mounted) setLoading(false);
      });
    return () => {
      mounted = false;
    };
  }, [setDashboard]);

  const incidents = dashboard?.incidents || [];
  const filteredIncidents = filterIncidents(incidents, {
    cluster: selectedCluster || dashboard?.environment || 'cluster',
    query: searchQuery,
    timeRange,
    severity: severityFilter,
    kind: kindFilter,
    highConfidenceOnly,
  });
  const selectedIncident = filteredIncidents.find((incident) => incident.id === selectedId) || filteredIncidents[0] || null;

  if (loading && !dashboard) {
    return (
      <div className="grid min-h-[calc(100vh-76px)] place-items-center text-[#647084]">
        <div className="flex items-center gap-2 text-[14px]">
          <Loader2 className="h-5 w-5 animate-spin" />
          Loading dashboard...
        </div>
      </div>
    );
  }

  if (error && !dashboard) {
    return (
      <div className="p-4">
        <div className="flex items-start gap-3 rounded-lg border border-[#fecaca] bg-[#fef2f2] p-4 text-[#b91c1c]">
          <AlertCircle className="mt-0.5 h-5 w-5" />
          <div>
            <h3 className="font-semibold">Connection Error</h3>
            <p className="text-[13px]">{error}</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-[calc(100vh-76px)] p-3 sm:p-4">
      <section className="min-w-0 self-start space-y-3">
        {!!dashboard?.availability?.length && <AvailabilityBanner availability={dashboard.availability} />}

        <div className="grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-3">
          {(dashboard?.kpis || []).map((kpi) => (
            <KpiCard key={kpi.label} kpi={kpi} />
          ))}
        </div>

        <IncidentTable
          incidents={filteredIncidents}
          totalIncidents={incidents.length}
          selectedId={selectedIncident?.id || null}
          onSelect={(id) => {
            setSelectedId(id);
            navigate(`/incidents/${encodeURIComponent(id)}`);
          }}
          page={incidentPage}
          onPageChange={setIncidentPage}
          severityFilter={severityFilter}
          onSeverityFilterChange={setSeverityFilter}
          kindFilter={kindFilter}
          onKindFilterChange={setKindFilter}
          highConfidenceOnly={highConfidenceOnly}
          onHighConfidenceOnlyChange={setHighConfidenceOnly}
          refreshing={refreshing}
          onRefresh={() => refreshDashboard(true)}
        />

        <div className="grid grid-cols-1 gap-3 2xl:grid-cols-[minmax(620px,1fr)_minmax(320px,420px)]">
          <RemediationPipeline stages={dashboard?.pipeline || []} />
          <InteractiveGraph incident={selectedIncident} />
        </div>

        <RecurringIssuesWidget remediations={dashboard?.remediations || []} />
      </section>
    </div>
  );
};

export const IncidentDetails = () => {
  const { incidentId = '' } = useParams();
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
        if (mounted) setError('Failed to load incident details.');
      })
      .finally(() => {
        if (mounted) setLoading(false);
      });
    return () => {
      mounted = false;
    };
  }, [dashboard, setDashboard]);

  const decodedId = decodeURIComponent(incidentId);
  const incident = (dashboard?.incidents || []).find((item) => item.id === decodedId) || null;

  if (loading && !dashboard) {
    return (
      <div className="grid min-h-[calc(100vh-76px)] place-items-center text-[#647084]">
        <div className="flex items-center gap-2 text-[14px]">
          <Loader2 className="h-5 w-5 animate-spin" />
          Loading incident details...
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-[calc(100vh-76px)] space-y-4 p-3 sm:p-4">
      <button
        type="button"
        onClick={() => navigate('/')}
        className="inline-flex h-9 items-center gap-2 rounded-md border border-[#e5e7eb] bg-white px-3 text-[12px] font-semibold text-[#374151] hover:bg-[#f8fafc]"
      >
        <ChevronLeft className="h-4 w-4" />
        Back to incidents
      </button>

      {error && !dashboard ? (
        <div className="rounded-lg border border-[#fecaca] bg-[#fef2f2] p-4 text-[13px] text-[#b91c1c]">{error}</div>
      ) : (
        <IncidentDetailContent
          incident={incident}
          remediations={dashboard?.remediations || []}
          onDashboardRefresh={setDashboard}
          mode="page"
        />
      )}
    </div>
  );
};

const AvailabilityBanner = ({ availability }: { availability: DashboardState['availability'] }) => (
  <div className="rounded-lg border border-[#dbeafe] bg-[#eff6ff] px-4 py-3 text-[13px] text-[#1d4ed8]">
    <strong className="font-semibold">{availability[0].name}:</strong> {availability[0].message}
  </div>
);

const filterIncidents = (
  incidents: DashboardIncident[],
  filters: {
    cluster: string;
    query: string;
    timeRange: string;
    severity: string;
    kind: string;
    highConfidenceOnly: boolean;
  },
) => {
  const normalizedQuery = filters.query.trim().toLowerCase();
  const maxAgeMinutes = timeRangeToMinutes(filters.timeRange);

  return incidents.filter((incident) => {
    if (incident.cluster && filters.cluster && incident.cluster !== filters.cluster) {
      return false;
    }
    if (filters.severity !== 'all' && (incident.severity || '').toLowerCase() !== filters.severity) {
      return false;
    }
    if (filters.kind !== 'all' && incident.workload.kind !== filters.kind) {
      return false;
    }
    if (filters.highConfidenceOnly && (incident.confidence || 0) < 80) {
      return false;
    }
    if (maxAgeMinutes !== Infinity && ageToMinutes(incident.age) > maxAgeMinutes) {
      return false;
    }
    if (!normalizedQuery) {
      return true;
    }
    const haystack = [
      incident.cluster,
      incident.workload.kind,
      incident.workload.name,
      incident.workload.namespace,
      incident.workload.podName,
      incident.status,
      incident.cause,
      incident.source,
      incident.severity,
      incident.priority,
      incident.gitops?.app,
      incident.gitops?.repo,
      incident.gitops?.path,
      incident.pr?.branch,
    ].filter(Boolean).join(' ').toLowerCase();
    return haystack.includes(normalizedQuery);
  });
};

const timeRangeToMinutes = (range: string) => {
  switch (range) {
    case 'Last 1h':
      return 60;
    case 'Last 6h':
      return 6 * 60;
    case 'Last 7d':
      return 7 * 24 * 60;
    case 'Last 30d':
      return 30 * 24 * 60;
    case 'All time':
      return Infinity;
    case 'Last 24h':
    default:
      return 24 * 60;
  }
};

const ageToMinutes = (age: string) => {
  const trimmed = age.trim().toLowerCase();
  if (!trimmed || trimmed === 'now') return 0;
  const match = trimmed.match(/^(\d+)\s*(m|h|d)$/);
  if (!match) return 0;
  const value = Number(match[1]);
  if (match[2] === 'd') return value * 24 * 60;
  if (match[2] === 'h') return value * 60;
  return value;
};

const displayIncidentCause = (incident: DashboardIncident) => {
  const cause = (incident.rca?.summary || incident.cause || '').trim();
  if (isPlaceholderCause(cause)) return 'Root cause pending';
  return cause;
};

const isPlaceholderCause = (value?: string) => {
  const normalized = (value || '').trim();
  return !normalized || normalized === '{' || normalized === '}' || normalized === '{}' || normalized === '[]' || /^null$/i.test(normalized);
};

const shouldExpandIncidentCause = (value?: string) => {
  const normalized = (value || '').trim();
  return !isPlaceholderCause(normalized) && (normalized.length > 130 || normalized.includes('\n'));
};

const dashboardActionErrorMessage = (err: unknown) => {
  if (typeof err === 'object' && err && 'response' in err) {
    const response = (err as { response?: { data?: unknown } }).response;
    if (typeof response?.data === 'string' && response.data.trim()) return response.data.trim();
  }
  if (err instanceof Error && err.message) return err.message;
  return 'Failed to execute action.';
};

const KpiCard = ({ kpi }: { kpi: DashboardKPI }) => {
  const tone = toneStyles[kpi.tone || 'info'] || toneStyles.info;
  return (
    <div className="min-h-[108px] rounded-lg border border-[#e5e7eb] bg-white p-3 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
      <div className="text-[13px] font-semibold text-[#111827]">{kpi.label}</div>
      <div className="mt-3 flex items-end justify-between gap-3">
        <div>
          <div className={`text-[26px] font-semibold leading-none ${tone.text}`}>{kpi.value}</div>
          <div className="mt-2 text-[11px] leading-4 text-[#6b7280]">
            <span className={tone.text}>{kpi.delta ? '↑ ' : ''}{kpi.delta || ''}</span>
            {kpi.detail ? <span> {kpi.detail}</span> : null}
          </div>
        </div>
        <Sparkline values={kpi.trend} color={tone.line} />
      </div>
    </div>
  );
};

const Sparkline = ({ values, color }: { values?: number[]; color: string }) => {
  if (!values || values.length < 2) {
    return <div className="h-8 w-20 rounded bg-[linear-gradient(180deg,transparent_48%,#e5e7eb_50%,transparent_52%)]" />;
  }
  const max = Math.max(...values);
  const min = Math.min(...values);
  const range = Math.max(max - min, 1);
  const points = values
    .map((value, index) => {
      const x = (index / (values.length - 1)) * 80;
      const y = 30 - ((value - min) / range) * 26;
      return `${x},${y}`;
    })
    .join(' ');
  return (
    <svg viewBox="0 0 80 32" className="h-8 w-20 overflow-visible">
      <polyline points={points} fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
};

const IncidentTable = ({
  incidents,
  totalIncidents,
  selectedId,
  onSelect,
  page,
  onPageChange,
  severityFilter,
  onSeverityFilterChange,
  kindFilter,
  onKindFilterChange,
  highConfidenceOnly,
  onHighConfidenceOnlyChange,
  refreshing,
  onRefresh,
}: {
  incidents: DashboardIncident[];
  totalIncidents: number;
  selectedId: string | null;
  onSelect: (id: string) => void;
  page: number;
  onPageChange: (page: number) => void;
  severityFilter: string;
  onSeverityFilterChange: (severity: string) => void;
  kindFilter: string;
  onKindFilterChange: (kind: string) => void;
  highConfidenceOnly: boolean;
  onHighConfidenceOnlyChange: (value: boolean) => void;
  refreshing: boolean;
  onRefresh: () => void;
}) => {
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [severityOpen, setSeverityOpen] = useState(false);
  const pageSize = 4;
  const pageCount = Math.max(1, Math.ceil(incidents.length / pageSize));
  const safePage = Math.min(Math.max(page, 1), pageCount);
  const start = (safePage - 1) * pageSize;
  const pageIncidents = incidents.slice(start, start + pageSize);
  const workloadKinds = Array.from(new Set(incidents.map((incident) => incident.workload.kind).filter(Boolean))).sort();
  const severityOptions = [
    { label: 'All Severities', value: 'all' },
    { label: 'Critical', value: 'critical' },
    { label: 'Warning', value: 'warning' },
    { label: 'Info', value: 'info' },
  ];

  return (
    <div className="overflow-hidden rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
      <div className="flex h-[60px] items-center justify-between border-b border-[#e5e7eb] px-4">
        <div className="flex items-center gap-2">
          <h2 className="text-[16px] font-semibold text-[#111827]">Active Incidents</h2>
          <span className="rounded-full bg-[#fee2e2] px-2 py-0.5 text-[11px] font-semibold text-[#ef4444]">{incidents.length}</span>
          {incidents.length !== totalIncidents && <span className="text-[11px] text-[#647084]">filtered from {totalIncidents}</span>}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setFiltersOpen((open) => !open)}
            className={`flex h-9 items-center gap-2 rounded-md border px-3 text-[12px] font-medium ${filtersOpen ? 'border-[#bfdbfe] bg-[#eff6ff] text-[#2563eb]' : 'border-[#e5e7eb] bg-white'}`}
          >
            <Filter className="h-4 w-4" />
            Filter
          </button>
          <div className="relative">
            <button
              onClick={() => setSeverityOpen((open) => !open)}
              className="flex h-9 min-w-[136px] items-center justify-between gap-2 rounded-md border border-[#e5e7eb] bg-white px-3 text-[12px] font-medium"
            >
              {severityOptions.find((option) => option.value === severityFilter)?.label || 'All Severities'}
              <ChevronDown className="h-4 w-4" />
            </button>
            {severityOpen && (
              <div className="absolute right-0 top-10 z-20 w-40 overflow-hidden rounded-lg border border-[#e5e7eb] bg-white shadow-[0_18px_50px_rgba(15,23,42,0.16)]">
                {severityOptions.map((option) => (
                  <button
                    key={option.value}
                    onClick={() => {
                      onSeverityFilterChange(option.value);
                      onPageChange(1);
                      setSeverityOpen(false);
                    }}
                    className={`block w-full px-3 py-2 text-left text-[12px] hover:bg-[#f8fafc] ${option.value === severityFilter ? 'font-semibold text-[#2563eb]' : 'text-[#111827]'}`}
                  >
                    {option.label}
                  </button>
                ))}
              </div>
            )}
          </div>
          <button onClick={onRefresh} className="grid h-9 w-9 place-items-center rounded-md border border-[#e5e7eb] bg-white" title="Refresh">
            <RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {filtersOpen && (
        <div className="flex flex-wrap items-center gap-3 border-b border-[#e5e7eb] bg-[#f8fafc] px-4 py-3 text-[12px]">
          <label className="flex items-center gap-2">
            <span className="font-medium text-[#374151]">Kind</span>
            <select
              value={kindFilter}
              onChange={(event) => {
                onKindFilterChange(event.target.value);
                onPageChange(1);
              }}
              className="h-8 rounded-md border border-[#e5e7eb] bg-white px-2 outline-none"
            >
              <option value="all">All workload types</option>
              {workloadKinds.map((kind) => <option key={kind} value={kind}>{kind}</option>)}
            </select>
          </label>
          <label className="flex items-center gap-2 font-medium text-[#374151]">
            <input
              type="checkbox"
              checked={highConfidenceOnly}
              onChange={(event) => {
                onHighConfidenceOnlyChange(event.target.checked);
                onPageChange(1);
              }}
              className="h-4 w-4 rounded border-[#cbd5e1]"
            />
            Confidence ≥ 80%
          </label>
          <button
            onClick={() => {
              onKindFilterChange('all');
              onHighConfidenceOnlyChange(false);
              onSeverityFilterChange('all');
              onPageChange(1);
            }}
            className="ml-auto rounded-md border border-[#e5e7eb] bg-white px-3 py-1.5 font-medium text-[#374151] hover:bg-[#f8fafc]"
          >
            Reset filters
          </button>
        </div>
      )}

      {incidents.length === 0 ? (
        <EmptyState
          icon={<TriangleAlert className="h-8 w-8" />}
          title="No matching incidents"
          message="No incidents match the current cluster, search, date, severity, or table filters."
        />
      ) : (
        <>
          <div className="max-h-[min(430px,calc(100vh-350px))] overflow-auto">
            <table className="w-full min-w-[760px] table-fixed border-collapse text-left text-[12px]">
              <thead className="sticky top-0 z-[1] bg-[#f8fafc] text-[#111827]">
                <tr>
                  <th className="w-8 px-3 py-3" />
                  <th className="w-[25%] px-3 py-3 font-semibold">Workload</th>
                  <th className="w-[10%] px-3 py-3 font-semibold">Namespace</th>
                  <th className="w-[15%] px-3 py-3 font-semibold">Status</th>
                  <th className="w-[33%] px-3 py-3 font-semibold">Root Cause</th>
                  <th className="w-[11%] px-3 py-3 font-semibold">Signal</th>
                  <th className="w-[5%] px-3 py-3 font-semibold">Age</th>
                </tr>
              </thead>
              <tbody>
                {pageIncidents.map((incident) => {
                  const selected = incident.id === selectedId;
                  return (
                    <tr
                      key={incident.id}
                      onClick={() => onSelect(incident.id)}
                      className={`cursor-pointer border-t border-[#e5e7eb] transition ${selected ? 'bg-[#fff1f2] shadow-[inset_3px_0_0_#ef4444]' : 'hover:bg-[#f9fafb]'}`}
                    >
                      <td className="px-3 py-3">
                        {selected ? <Circle className="h-3.5 w-3.5 fill-white text-[#94a3b8]" /> : <Circle className="h-3.5 w-3.5 text-[#94a3b8]" />}
                      </td>
                      <td className="px-3 py-3">
                        <div className="flex items-center gap-2 font-medium text-[#111827]">
                          <WorkloadIcon workload={incident.workload} />
                          <div className="min-w-0">
                            <div className="truncate" title={`${incident.workload.kind}/${incident.workload.name}`}>{incident.workload.kind}/{incident.workload.name}</div>
                            {incident.workload.podName && incident.workload.podName !== incident.workload.name && (
                              <div className="truncate text-[11px] font-normal text-[#647084]" title={`Pod/${incident.workload.podName}`}>Pod/{incident.workload.podName}</div>
                            )}
                          </div>
                        </div>
                      </td>
                      <td className="px-3 py-3 text-[#111827]" title={incident.workload.namespace}>{incident.workload.namespace}</td>
                      <td className="px-3 py-3">
                        <StatusChip value={incident.status} severity={incident.severity} />
                      </td>
                      <td className="px-3 py-3 text-[#111827]">
                        {isPlaceholderCause(displayIncidentCause(incident)) ? (
                          <span className="text-[#94a3b8]">Root cause pending</span>
                        ) : (
                        <ExpandableText
                          value={displayIncidentCause(incident)}
                          collapsedClassName="line-clamp-3"
                          buttonLabel="More"
                          alwaysExpandable={shouldExpandIncidentCause(displayIncidentCause(incident))}
                        />
                        )}
                      </td>
                      <td className="px-3 py-3">
                        <Confidence value={incident.confidence} />
                        <div className="mt-1 truncate text-[11px] text-[#647084]" title={incident.source}>{incident.source}</div>
                      </td>
                      <td className={`px-3 py-3 font-medium ${incident.severity === 'critical' ? 'text-[#dc2626]' : 'text-[#f97316]'}`}>{incident.age}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <div className="flex h-14 items-center justify-between border-t border-[#e5e7eb] px-4 text-[12px] text-[#4b5563]">
            <span>Showing {start + 1} to {Math.min(start + pageSize, incidents.length)} of {incidents.length} incidents</span>
            <div className="flex items-center gap-1">
              <PagerButton disabled={safePage === 1} onClick={() => onPageChange(safePage - 1)} icon={<ChevronLeft className="h-4 w-4" />} />
              {Array.from({ length: pageCount }, (_, index) => index + 1).map((pageNumber) => (
                <PagerButton key={pageNumber} label={`${pageNumber}`} active={pageNumber === safePage} onClick={() => onPageChange(pageNumber)} />
              ))}
              <PagerButton disabled={safePage === pageCount} onClick={() => onPageChange(safePage + 1)} icon={<ChevronRight className="h-4 w-4" />} />
            </div>
          </div>
        </>
      )}
    </div>
  );
};

const IncidentDetailContent = ({
  incident,
  remediations,
  onDashboardRefresh,
  mode = 'drawer',
}: { 
  incident: DashboardIncident | null; 
  remediations: DashboardRemediation[];
  onDashboardRefresh: (data: DashboardState | null) => void;
  mode?: 'drawer' | 'page';
}) => {
  const [runningAction, setRunningAction] = useState<string | null>(null);
  const [actionError, setActionError] = useState('');

  if (!incident) {
    return (
      <section className="rounded-lg border border-[#e5e7eb] bg-white">
        <EmptyState
          icon={<FileText className="h-8 w-8" />}
          title="Incident not found"
          message="The incident may have aged out of the dashboard window or been resolved. Return to the incident list to select a current item."
        />
      </section>
    );
  }

  // Find the most recent active remediation for this incident's workload
  const activeRemediation = remediations.find(r => r.workload.namespace === incident.workload.namespace && r.workload.name === incident.workload.name && remediationManualActions(r).length > 0);
  const detailGrid = mode === 'page'
    ? 'grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,0.85fr)]'
    : 'space-y-3';

  return (
    <section className="space-y-4">
      <div className={detailGrid}>
        <div className="space-y-3">
        {actionError && (
          <div className="rounded-md border border-[#fed7aa] bg-[#fff7ed] px-3 py-2 text-[12px] text-[#9a3412]">{actionError}</div>
        )}
        
        <div className="flex items-start justify-between gap-3 rounded-lg border border-[#e5e7eb] bg-white px-3 py-3 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-start gap-2">
              <TriangleAlert className="h-5 w-5 text-[#dc2626]" />
              <h2 className="min-w-0 flex-1 text-[17px] font-semibold leading-6 text-[#111827]" title={`${incident.workload.kind}/${incident.workload.name}`}>{incident.workload.kind}/{incident.workload.name}</h2>
              <StatusChip value={incident.status} severity={incident.severity} />
            </div>
            <div className="mt-2 text-[12px] text-[#4b5563]">
              {incident.workload.namespace} <span className="mx-2">•</span> {incident.age || 'recent'} ago <span className="mx-2">•</span> {incident.priority || 'P2'}
              {incident.workload.podName && incident.workload.podName !== incident.workload.name && (
                <><span className="mx-2">•</span> logs from Pod/{incident.workload.podName}</>
              )}
            </div>
          </div>
        </div>

        <DrawerCard icon={<Network className="h-4 w-4 text-[#2563eb]" />} title="Evidence Chain">
          {incident.evidence?.length ? (
            <div className="divide-y divide-[#e5e7eb]">
              {incident.evidence.map((item) => (
                <EvidenceRow key={`${item.label}-${item.value}`} item={item} />
              ))}
            </div>
          ) : (
            <MiniEmpty message="No evidence chain has been stored for this incident yet." />
          )}
        </DrawerCard>

        <DrawerCard icon={<FileText className="h-4 w-4 text-[#2563eb]" />} title="Log Patterns">
          <IncidentLogPatterns incident={incident} />
        </DrawerCard>

        <DrawerCard icon={<AlertCircle className="h-4 w-4 text-[#f97316]" />} title="RCA Explainability">
          <IncidentRCA incident={incident} />
          <IncidentAIDebug incident={incident} />
        </DrawerCard>

        {mode === 'page' && <InteractiveGraph incident={incident} />}
        </div>

        <div className="space-y-3">
        {activeRemediation && (
          <DrawerCard icon={<Activity className="h-4 w-4 text-[#2563eb]" />} title="Actionable Triage">
            <div className="px-3 pb-3">
              <div className="mt-2 text-[11px] text-[#647084]">Execute remediation workflow actions directly from the dashboard:</div>
              <RemediationActions
                row={activeRemediation}
                runningAction={runningAction}
                onRun={async (action) => {
                  const key = `${activeRemediation.id}:${action}`;
                  setRunningAction(key);
                  setActionError('');
                  try {
                    await apiClient.post(`/remediations/${activeRemediation.id}/actions/${action}`);
                    const { data } = await apiClient.get<DashboardState>('/dashboard');
                    onDashboardRefresh(data);
                  } catch (err: unknown) {
                    setActionError(dashboardActionErrorMessage(err));
                  } finally {
                    setRunningAction(null);
                  }
                }}
              />
            </div>
          </DrawerCard>
        )}

        <DrawerCard icon={<FileCode2 className="h-4 w-4 text-[#2563eb]" />} title="GitOps Mapping">
          {incident.gitops ? (
            <div>
              <KeyValueRows
                rows={[
                  ['Controller', incident.gitops.controller],
                  ['App', incident.gitops.app],
                  ['Repository', incident.gitops.repo],
                  ['Revision', incident.gitops.revision],
                  ['Path', incident.gitops.path],
                  ['Manifest Type', readableManifestType(incident.gitops.manifestType)],
                  ['Overlay', incident.gitops.overlay || ''],
                ]}
              />
              {isHelmGitOps(incident.gitops.manifestType) && <HelmMapping gitops={incident.gitops} />}
            </div>
          ) : (
            <MiniEmpty message="No GitOps source is mapped yet." />
          )}
        </DrawerCard>

        <DrawerCard icon={<GitPullRequestArrow className="h-4 w-4 text-[#2563eb]" />} title="Recommended PR">
          {incident.pr ? <RecommendedPR pr={incident.pr} /> : <MiniEmpty message="No safe remediation pull request is available yet." />}
        </DrawerCard>

        <DrawerCard icon={<ShieldCheck className="h-4 w-4 text-[#16a34a]" />} title="Guardrails">
          {incident.guardrails?.length ? (
            <div className="divide-y divide-[#e5e7eb]">
              {primarySafetyGuardrails(incident.guardrails).map((guardrail) => (
                <ValidationBadge key={guardrail.label} guardrail={guardrail} helm={isHelmGitOps(incident.gitops?.manifestType)} />
              ))}
              {incident.guardrails.filter((guardrail) => !isPrimarySafetyGuardrail(guardrail)).map((guardrail) => (
                <GuardrailRow key={guardrail.label} guardrail={guardrail} />
              ))}
            </div>
          ) : (
            <MiniEmpty message="Guardrail results will appear after remediation analysis." />
          )}
        </DrawerCard>

        <DrawerCard icon={<SlidersIcon />} title="Policy & SLO">
          <IncidentPolicy incident={incident} />
        </DrawerCard>
        </div>
      </div>
    </section>
  );
};

const DrawerCard = ({ icon, title, children }: { icon: ReactNode; title: string; children: ReactNode }) => (
  <section className="overflow-hidden rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
    <header className="flex h-10 items-center justify-between border-b border-[#e5e7eb] bg-white px-3">
      <div className="flex items-center gap-2 text-[13px] font-semibold text-[#111827]">
        {icon}
        {title}
      </div>
      <ChevronsUpDown className="h-4 w-4 text-[#6b7280]" />
    </header>
    {children}
  </section>
);

const EvidenceRow = ({ item }: { item: DashboardEvidence }) => (
  <div className="grid grid-cols-[104px_minmax(0,1fr)_auto] items-start gap-3 px-3 py-2.5 text-[12px] hover:bg-[#f8fafc]">
    <div className="font-semibold text-[#111827]">{item.label}</div>
    <ExpandableText
      value={item.value}
      collapsedClassName="line-clamp-2"
      preserveWhitespace={/event|log|trace|stack/i.test(item.label)}
      buttonLabel={`Read ${item.label}`}
    />
    <div className="flex items-center gap-2">
      {!!item.count && <span className="rounded-full bg-[#dbeafe] px-2 py-0.5 text-[11px] font-semibold text-[#2563eb]">{item.count}</span>}
      {item.link && <button className="font-medium text-[#2563eb]">{item.link}</button>}
    </div>
  </div>
);

const ExpandableText = ({
  value,
  collapsedClassName,
  preserveWhitespace = false,
  buttonLabel = 'Read more',
  alwaysExpandable = false,
}: {
  value: string;
  collapsedClassName: string;
  preserveWhitespace?: boolean;
  buttonLabel?: string;
  alwaysExpandable?: boolean;
}) => {
  const [open, setOpen] = useState(false);
  const isLong = alwaysExpandable || (value || '').length > 140 || (value || '').includes('\n');
  return (
    <div className="min-w-0">
      <div
        className={`${open ? 'max-h-72 overflow-auto rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-2' : collapsedClassName} ${preserveWhitespace ? 'whitespace-pre-wrap font-mono text-[11px] leading-5' : 'leading-5'} text-[#374151]`}
        title={!open ? value : undefined}
      >
        {value}
      </div>
      {isLong && (
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            setOpen((current) => !current);
          }}
          className="mt-1 text-[11px] font-semibold text-[#2563eb] hover:underline"
          aria-expanded={open}
        >
          {open ? 'Show less' : buttonLabel}
        </button>
      )}
    </div>
  );
};

const IncidentLogPatterns = ({ incident }: { incident: DashboardIncident }) => {
  const logItems = (incident.evidence || []).filter((item) => /log|stack|trace|event|pattern/i.test(`${item.label} ${item.icon} ${item.value}`));
  const backendPatterns = incident.logPatterns || [];
  if (!logItems.length && !backendPatterns.length) {
    return <MiniEmpty message="No log, event, or stack pattern has been attached to this incident yet." />;
  }
  return (
    <div className="space-y-2 px-3 py-3">
      {backendPatterns.slice(0, 4).map((item) => (
        <div key={`${item.source}-${item.pattern}`} className="rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-2">
          <div className="mb-1 flex items-center justify-between gap-2 text-[11px] font-semibold text-[#111827]">
            <span>{item.label}</span>
            {!!item.count && <span className="rounded-full bg-[#dbeafe] px-2 py-0.5 text-[#2563eb]">{item.count}</span>}
          </div>
          <pre className="max-h-28 overflow-auto whitespace-pre-wrap rounded bg-[#0f172a] p-2 text-[11px] leading-5 text-[#f8fafc]">{item.pattern}</pre>
        </div>
      ))}
      {logItems.slice(0, 3).map((item) => (
        <div key={`${item.label}-${item.value}`} className="rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-2">
          <div className="mb-1 flex items-center justify-between gap-2 text-[11px] font-semibold text-[#111827]">
            <span>{item.label}</span>
            {!!item.count && <span className="rounded-full bg-[#dbeafe] px-2 py-0.5 text-[#2563eb]">{item.count}</span>}
          </div>
          <pre className="max-h-28 overflow-auto whitespace-pre-wrap rounded bg-[#0f172a] p-2 text-[11px] leading-5 text-[#f8fafc]">{item.value}</pre>
        </div>
      ))}
    </div>
  );
};

const IncidentRCA = ({ incident }: { incident: DashboardIncident }) => (
  <div className="space-y-2 px-3 py-3 text-[12px]">
    <KeyValueRows
      compact
      rows={[
        ['Root cause', incident.rca?.summary || incident.cause || 'Pending'],
        ['Confidence', `${incident.rca?.confidence ?? incident.confidence ?? 0}%`],
        ['Signal', incident.rca?.signal || incident.source || 'Unknown'],
        ['Risk', incident.rca?.risk || incident.risk || 'Unknown'],
        ['Recommended action', incident.rca?.recommendedAction],
        ['Negative feedback', incident.rca?.negativeFeedback],
        ['Patch strategy', incident.pr?.strategy],
        ['Patch target', incident.pr?.patchTarget],
      ]}
    />
    {incident.pr?.reason && (
      <div className="rounded-md border border-[#dbeafe] bg-[#eff6ff] p-2 text-[#1e3a8a]">
        <div className="font-semibold">Why Fixora selected this remediation</div>
        <p className="mt-1 leading-5">{incident.pr.reason}</p>
      </div>
    )}
    {incident.pr?.summary?.length ? (
      <div className="rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-2">
        <div className="mb-1 text-[11px] font-semibold uppercase text-[#647084]">Remediation summary</div>
        <ul className="space-y-1 text-[#334155]">
          {incident.pr.summary.map((item) => <li key={item}>{item}</li>)}
        </ul>
      </div>
    ) : null}
  </div>
);

const IncidentAIDebug = ({ incident }: { incident: DashboardIncident }) => {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<InvestigationDetail | null>(null);
  const [error, setError] = useState('');

  const handleToggle = async () => {
    if (open) {
      setOpen(false);
      return;
    }
    setOpen(true);
    if (data) return;

    setLoading(true);
    const investigationId = incident.id.replace(/^investigation-/, '');
    try {
      const response = await apiClient.get<InvestigationDetail>(`/audit/investigations/${investigationId}`);
      setData(response.data);
    } catch {
      setError('Failed to load AI interaction details.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="border-t border-[#e5e7eb] px-3 py-3">
      <button 
        onClick={handleToggle}
        className="flex w-full items-center justify-between text-[12px] font-semibold text-[#111827] hover:text-[#2563eb]"
      >
        <span className="flex items-center gap-2">
          <Brain className="h-4 w-4" />
          Debug AI Interaction
        </span>
        <ChevronDown className={`h-4 w-4 transform transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>

      {open && (
        <div className="mt-3 space-y-3">
          {loading && <div className="text-[12px] text-[#647084]">Loading AI details...</div>}
          {error && <div className="text-[12px] text-[#dc2626]">{error}</div>}
          {data && (
            <>
              <div>
                <div className="mb-1 text-[11px] font-semibold uppercase text-[#647084]">Input Content (Prompt)</div>
                <div className="max-h-64 overflow-x-auto whitespace-pre-wrap rounded-md border border-[#1e293b] bg-[#0f172a] p-3 font-mono text-[11px] text-[#f8fafc]">
                  {data.aiPrompt || 'No AI prompt captured.'}
                </div>
              </div>
              <div>
                <div className="mb-1 text-[11px] font-semibold uppercase text-[#647084]">Raw AI Response</div>
                <div className="max-h-64 overflow-x-auto whitespace-pre-wrap rounded-md border border-[#e2e8f0] bg-[#f1f5f9] p-3 font-mono text-[11px] text-[#334155]">
                  {data.aiResponse || 'No AI response captured.'}
                </div>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
};

const IncidentPolicy = ({ incident }: { incident: DashboardIncident }) => {
  const requiresApproval = incident.pr?.approverRequired;
  const renderGuardrail = incident.guardrails?.find((guardrail) => /render/i.test(guardrail.label));
  const duplicateGuardrail = incident.guardrails?.find((guardrail) => /duplicate/i.test(guardrail.label));
  const policy = incident.policyState;
  return (
    <div className="space-y-2 px-3 py-3 text-[12px]">
      <KeyValueRows
        compact
        rows={[
          ['Mode', policy?.mode],
          ['Approval', policy?.approvalRequired || requiresApproval ? 'Required' : incident.pr ? 'Not required' : 'No PR candidate'],
          ['Render check', renderGuardrail?.status || 'Pending'],
          ['Duplicate PR', duplicateGuardrail?.status || 'Pending'],
          ['Availability SLO', policy?.availabilitySlo ? `${(policy.availabilitySlo * 100).toFixed(2)}%` : 'Not configured in dashboard metadata'],
          ['Burn rate', policy?.burnRateThreshold ? `${policy.burnRateThreshold}x` : 'Not configured in dashboard metadata'],
        ]}
      />
      <div className="rounded-md border border-dashed border-[#cbd5e1] bg-[#f8fafc] p-2 text-[11px] leading-5 text-[#647084]">
        SLO and remediation policy edits should use a dedicated safe settings API so the UI can show configured, missing, and last-rotated state without returning secret values.
      </div>
    </div>
  );
};

const SlidersIcon = () => <Code2 className="h-4 w-4 text-[#647084]" />;

const HelmMapping = ({ gitops }: { gitops: DashboardGitOpsMapping }) => {
  const helm = gitops.helm;
  const values = helm?.valueFiles || [];
  const valuesFrom = helm?.valuesFrom || [];
  return (
    <div className="border-t border-[#e5e7eb] px-3 py-3 text-[12px]">
      <div className="mb-2 flex items-center gap-2 font-semibold text-[#111827]">
        <Boxes className="h-4 w-4 text-[#2563eb]" />
        Helm source
      </div>
      <div className="grid gap-2">
        <KeyValueRows
          compact
          rows={[
            ['Release', helm?.releaseName],
            ['Namespace', helm?.namespace],
            ['Chart', helm?.chart],
            ['Version', helm?.chartVersion],
            ['Chart Repo', helm?.repoUrl],
          ]}
        />
        {values.length > 0 ? (
          <OrderedFileList title="Active values order" files={values} />
        ) : (
          <MiniInline message="No values files have been reported for this Helm source yet." />
        )}
        {valuesFrom.length > 0 && <OrderedFileList title="valuesFrom" files={valuesFrom} />}
      </div>
    </div>
  );
};

const OrderedFileList = ({ title, files }: { title: string; files: string[] }) => (
  <div className="rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-2">
    <div className="mb-1 text-[11px] font-semibold uppercase text-[#647084]">{title}</div>
    <div className="space-y-1">
      {files.slice(0, 6).map((file, index) => (
        <div key={`${file}-${index}`} className="flex items-center gap-2 text-[#334155]">
          <span className="grid h-4 w-4 shrink-0 place-items-center rounded-full bg-white text-[10px] font-semibold text-[#2563eb]">{index + 1}</span>
          <span className="truncate">{file}</span>
        </div>
      ))}
      {files.length > 6 && <div className="text-[11px] text-[#647084]">+{files.length - 6} more</div>}
    </div>
  </div>
);

const RecommendedPR = ({ pr }: { pr: DashboardRecommendedPR }) => (
  <div className="space-y-3 px-3 py-3 text-[12px]">
    <KeyValueRows
      rows={[
        ['Branch', pr.branch],
        ['Files', pr.fileCount ? `${pr.fileCount} files` : `${pr.files?.length || 0} files`],
        ['Strategy', pr.strategy],
        ['Patch Target', pr.patchTarget],
      ]}
    />
    {(pr.reason || pr.avoided || pr.summary?.length) && (
      <div className="rounded-md border border-[#dbeafe] bg-[#eff6ff] p-2 text-[#1e3a8a]">
        {pr.reason && <div className="font-medium">{pr.reason}</div>}
        {pr.avoided && <div className="mt-1 text-[11px]">Avoided: {pr.avoided}</div>}
        {!!pr.summary?.length && (
          <ul className="mt-2 space-y-1 text-[11px] text-[#334155]">
            {pr.summary.map((item) => <li key={item}>• {item}</li>)}
          </ul>
        )}
      </div>
    )}
    {!!pr.files?.length && (
      <div className="space-y-1 pl-[84px]">
        {pr.files.slice(0, 4).map((file) => (
          <div key={file} className="flex items-center gap-2 text-[#374151]">
            <FileText className="h-3.5 w-3.5 text-[#6b7280]" />
            <span className="truncate">{file}</span>
          </div>
        ))}
      </div>
    )}
    <div className="flex items-center gap-2">
      <span className="rounded-md bg-[#dcfce7] px-2 py-1 text-[11px] font-medium text-[#15803d]">{pr.risk || 'Risk pending'}</span>
      {pr.approverRequired && <span className="rounded-md bg-[#ffedd5] px-2 py-1 text-[11px] font-medium text-[#ea580c]">Approver required</span>}
      {pr.url && (
        <a className="ml-auto flex items-center gap-1 text-[12px] font-medium text-[#2563eb]" href={pr.url} target="_blank" rel="noreferrer">
          Open PR
          <ExternalLink className="h-3.5 w-3.5" />
        </a>
      )}
    </div>
  </div>
);

const RemediationPipeline = ({ stages }: { stages: DashboardPipelineStage[] }) => (
  <section className="rounded-lg border border-[#e5e7eb] bg-white p-3 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
    <div className="mb-3 flex items-center gap-2">
      <h2 className="text-[14px] font-semibold text-[#111827]">Closed-loop Remediation</h2>
      <AlertCircle className="h-3.5 w-3.5 text-[#94a3b8]" />
    </div>
    {stages.length === 0 ? (
      <MiniEmpty message="No remediation workflow state has been recorded yet." />
    ) : (
      <div className="overflow-x-auto rounded-md border border-[#e5e7eb] bg-white">
        <div style={{ minWidth: Math.max(stages.length * 132, 760) }}>
          <div className="relative grid border-b border-[#e5e7eb] px-2 pb-2 pt-5" style={{ gridTemplateColumns: `repeat(${stages.length}, minmax(118px, 1fr))` }}>
            <div className="absolute left-3 right-3 top-[30px] h-0.5 bg-[#d1d5db]" />
            {stages.map((stage) => {
              const stageTone = stage.state === 'failed' ? '#ef4444' : stage.state === 'succeeded' ? '#16a34a' : '#f97316';
              return (
                <div key={stage.id} className="relative z-[1] flex flex-col items-center gap-1">
                  <div className="text-[11px] font-semibold" style={{ color: stageTone }}>{stage.count}</div>
                  <span className="h-3 w-3 rounded-full border-2 border-white shadow-sm" style={{ background: stageTone }} />
                </div>
              );
            })}
          </div>
          <div className="grid" style={{ gridTemplateColumns: `repeat(${stages.length}, minmax(118px, 1fr))` }}>
            {stages.map((stage) => {
              const stageTone = stage.state === 'failed' ? '#ef4444' : stage.state === 'succeeded' ? '#16a34a' : '#f97316';
              const visibleItems = stage.items?.slice(0, 2) || [];
              const hidden = Math.max((stage.items?.length || 0) - visibleItems.length, 0);
              return (
                <div key={stage.id} className="min-h-[198px] border-r border-[#e5e7eb] last:border-r-0">
                  <div className="h-1" style={{ background: stageTone }} />
                  <div className="m-2 rounded-md border border-[#eef2f7] bg-[#f8fafc] px-2 py-3 text-center">
                    <div className="text-[12px] font-semibold leading-4 text-[#111827]">{stage.label}</div>
                    {stage.note && <div className="mt-1 truncate text-[10px] text-[#647084]">{stage.note}</div>}
                  </div>
                  <div className="space-y-2 px-2 pb-2">
                    {visibleItems.length ? (
                      visibleItems.map((item) => (
                        <a
                          key={item.id}
                          href={item.url || undefined}
                          target={item.url ? '_blank' : undefined}
                          rel="noreferrer"
                          className="block rounded border border-transparent p-1 text-[11px] hover:border-[#e5e7eb] hover:bg-[#f8fafc]"
                        >
                          <div className="truncate font-semibold text-[#111827]">{item.label}</div>
                          <div className="mt-0.5 truncate text-[#6b7280]">{item.detail || item.repository}</div>
                          <div className="mt-0.5 truncate text-[#94a3b8]">{item.age || item.repository}</div>
                        </a>
                      ))
                    ) : (
                      <div className="px-1 text-[11px] text-[#9ca3af]">No items</div>
                    )}
                    {hidden > 0 && <div className="px-1 pt-2 text-[11px] font-medium text-[#647084]">+{hidden} more</div>}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    )}
    <div className="mt-3 text-center">
      <a href="/remediations" className="inline-flex items-center gap-2 text-[12px] font-medium text-[#2563eb]">
        View all remediations
        <ChevronRight className="h-4 w-4" />
      </a>
    </div>
  </section>
);

const KeyValueRows = ({ rows, compact = false }: { rows: [string, string | undefined][]; compact?: boolean }) => (
  <div className={`space-y-2 text-[12px] ${compact ? '' : 'px-3 py-3'}`}>
    {rows.filter(([, value]) => !!value).map(([label, value]) => (
      <div key={label} className="grid grid-cols-[84px_1fr] gap-3">
        <span className="font-semibold text-[#111827]">{label}</span>
        <span className="min-w-0 break-words text-[#111827]">{value}</span>
      </div>
    ))}
  </div>
);

const ValidationBadge = ({ guardrail, helm }: { guardrail: DashboardGuardrail; helm?: boolean }) => {
  const tone = guardrailTone(guardrail.status);
  return (
    <div className="border-b border-[#e5e7eb] px-3 py-3">
      <div className="flex items-center justify-between gap-3 rounded-md border border-[#e5e7eb] bg-[#f8fafc] px-3 py-2 text-[12px]">
        <div className="min-w-0">
          <div className="font-semibold text-[#111827]">{validationBadgeTitle(guardrail, helm)}</div>
          {guardrail.detail && <div className="mt-0.5 truncate text-[11px] text-[#647084]">{guardrail.detail}</div>}
        </div>
        <span className={`shrink-0 rounded-md px-2 py-1 text-[11px] font-medium ${tone}`}>{guardrail.status}</span>
      </div>
    </div>
  );
};

const GuardrailRow = ({ guardrail }: { guardrail: DashboardGuardrail }) => {
  const tone = guardrailTone(guardrail.status);
  return (
    <div className="flex items-start justify-between gap-3 px-3 py-2 text-[12px]" title={guardrail.detail || undefined}>
      <div className="min-w-0">
        <span className="block text-[#374151]">{guardrail.label}</span>
        {guardrail.detail && <span className="mt-0.5 block truncate text-[11px] text-[#647084]">{guardrail.detail}</span>}
      </div>
      <span className={`shrink-0 rounded-md px-2 py-1 text-[11px] font-medium ${tone}`}>
        {guardrail.status}
      </span>
    </div>
  );
};

const primarySafetyGuardrails = (guardrails: DashboardGuardrail[]) =>
  guardrails.filter((guardrail) => isPrimarySafetyGuardrail(guardrail));

const isPrimarySafetyGuardrail = (guardrail: DashboardGuardrail) =>
  /render validation/i.test(guardrail.label) || /semantic target/i.test(guardrail.label);

const validationBadgeTitle = (guardrail: DashboardGuardrail, helm?: boolean) => {
  const isSemantic = /semantic target/i.test(guardrail.label);
  const prefix = isSemantic ? 'Semantic target check' : helm ? 'Helm render' : 'Render validation';
  switch ((guardrail.status || '').toLowerCase()) {
    case 'passed':
      return `${prefix} passed`;
    case 'failed':
      return `${prefix} failed`;
    case 'skipped':
      return `${prefix} skipped`;
    default:
      return `${prefix} pending`;
  }
};

const MiniInline = ({ message }: { message: string }) => (
  <div className="rounded-md border border-dashed border-[#d1d5db] bg-[#f8fafc] px-2 py-2 text-[11px] text-[#647084]">{message}</div>
);

const isHelmGitOps = (manifestType?: string) => /helm/i.test(manifestType || '');

const readableManifestType = (manifestType?: string) => {
  switch ((manifestType || '').toLowerCase()) {
    case 'flux-helmrelease':
      return 'Flux HelmRelease';
    case 'helm':
      return 'Helm';
    case 'kustomize':
      return 'Kustomize';
    case 'raw':
      return 'Raw manifests';
    default:
      return manifestType || 'Unknown';
  }
};

const guardrailTone = (status: string) => {
  switch ((status || '').toLowerCase()) {
    case 'passed':
      return 'bg-[#dcfce7] text-[#15803d]';
    case 'failed':
      return 'bg-[#fee2e2] text-[#dc2626]';
    case 'pending':
      return 'bg-[#ffedd5] text-[#ea580c]';
    case 'skipped':
      return 'bg-[#f1f5f9] text-[#475569]';
    default:
      return 'bg-[#f1f5f9] text-[#475569]';
  }
};

const WorkloadIcon = ({ workload }: { workload: DashboardWorkload }) => {
  const color = workloadColor(workload.kind);
  return (
    <span className="grid h-6 w-6 place-items-center rounded-full text-white" style={{ backgroundColor: color }}>
      {renderWorkloadGlyph(workload.kind, 'h-4 w-4')}
    </span>
  );
};

const renderWorkloadGlyph = (kind: string, className: string) => {
  const lower = kind.toLowerCase();
  if (lower.includes('values')) return <FileCode2 className={className} />;
  if (lower.includes('render')) return <Code2 className={className} />;
  if (lower.includes('stateful')) return <Database className={className} />;
  if (lower.includes('daemon')) return <Server className={className} />;
  if (lower.includes('helm')) return <Boxes className={className} />;
  if (lower.includes('config')) return <FileCode2 className={className} />;
  if (lower.includes('secret')) return <ShieldCheck className={className} />;
  if (lower.includes('service')) return <Network className={className} />;
  if (lower.includes('ingress')) return <Layers3 className={className} />;
  if (lower.includes('pod')) return <Container className={className} />;
  return <Code2 className={className} />;
};

const workloadColor = (kind: string) => {
  const lower = kind.toLowerCase();
  if (lower.includes('stateful')) return '#0ea5e9';
  if (lower.includes('daemon')) return '#1d4ed8';
  if (lower.includes('helm')) return '#2563eb';
  if (lower.includes('config')) return '#16a34a';
  if (lower.includes('secret')) return '#7c3aed';
  return '#2563eb';
};

const StatusChip = ({ value, severity }: { value: string; severity?: string }) => {
  const critical = severity === 'critical' || /crash|fail|error|oom|pending/i.test(value || '');
  const warning = /warn|degraded|unready|image|pull/i.test(value || '');
  return (
    <span
      className={`inline-flex max-w-[220px] shrink-0 truncate rounded-md border px-2 py-0.5 text-[11px] font-semibold ${
        critical
          ? 'border-[#fca5a5] bg-[#fee2e2] text-[#dc2626]'
          : warning
            ? 'border-[#fed7aa] bg-[#ffedd5] text-[#f97316]'
            : 'border-[#bfdbfe] bg-[#eff6ff] text-[#2563eb]'
      }`}
      title={value || 'Unknown'}
    >
      {value || 'Unknown'}
    </span>
  );
};

const Confidence = ({ value }: { value: number }) => (
  <div className="w-[88px]">
    <div className="mb-1 text-[12px] font-medium text-[#111827]">{value || 0}%</div>
    <div className="h-1.5 rounded-full bg-[#e5e7eb]">
      <div className="h-1.5 rounded-full bg-[#16a34a]" style={{ width: `${Math.max(0, Math.min(value || 0, 100))}%` }} />
    </div>
  </div>
);

const PagerButton = ({ label, icon, active, disabled, onClick }: { label?: string; icon?: ReactNode; active?: boolean; disabled?: boolean; onClick?: () => void }) => (
  <button
    disabled={disabled}
    onClick={onClick}
    className={`grid h-8 min-w-8 place-items-center rounded-md border px-2 text-[12px] transition ${
      active ? 'border-[#94a3b8] bg-white text-[#111827] shadow-sm' : 'border-[#e5e7eb] bg-white text-[#4b5563] hover:bg-[#f8fafc] disabled:text-[#cbd5e1] disabled:hover:bg-white'
    }`}
  >
    {icon || label}
  </button>
);

const EmptyState = ({ icon, title, message }: { icon: ReactNode; title: string; message: string }) => (
  <div className="grid min-h-[180px] place-items-center px-6 py-10 text-center">
    <div className="max-w-md rounded-lg border border-dashed border-[#cbd5e1] bg-[#f8fafc] px-6 py-6 text-[#647084]">
      <div className="mx-auto mb-3 grid h-12 w-12 place-items-center rounded-lg bg-white text-[#94a3b8] shadow-sm">{icon}</div>
      <strong className="text-[14px] text-[#111827]">{title}</strong>
      <p className="mt-1 text-[13px] leading-5">{message}</p>
    </div>
  </div>
);

const MiniEmpty = ({ message }: { message: string }) => <div className="m-3 rounded-md border border-dashed border-[#cbd5e1] bg-[#f8fafc] px-3 py-3 text-[12px] leading-5 text-[#647084]">{message}</div>;
