import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import {
  Background,
  Controls,
  MarkerType,
  MiniMap,
  Panel,
  Position,
  ReactFlow,
  type Edge,
  type Node,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  AlertCircle,
  Boxes,
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
  Maximize2,
  Network,
  RefreshCw,
  Server,
  ShieldCheck,
  TriangleAlert,
  X,
} from 'lucide-react';
import { apiClient } from '../api/client';
import { useStore } from '../store/useStore';
import type {
  DashboardDependencyEdge,
  DashboardDependencyNode,
  DashboardEvidence,
  DashboardGitOpsMapping,
  DashboardGuardrail,
  DashboardIncident,
  DashboardKPI,
  DashboardPipelineStage,
  DashboardRecommendedPR,
  DashboardState,
  DashboardWorkload,
} from '../types';

const toneStyles: Record<string, { text: string; border: string; bg: string; line: string }> = {
  critical: { text: 'text-[#dc2626]', border: 'border-[#fecaca]', bg: 'bg-[#fef2f2]', line: '#ef4444' },
  warning: { text: 'text-[#f97316]', border: 'border-[#fed7aa]', bg: 'bg-[#fff7ed]', line: '#f97316' },
  success: { text: 'text-[#15803d]', border: 'border-[#bbf7d0]', bg: 'bg-[#f0fdf4]', line: '#16a34a' },
  info: { text: 'text-[#2563eb]', border: 'border-[#bfdbfe]', bg: 'bg-[#eff6ff]', line: '#2563eb' },
};

export const Dashboard = () => {
  const { dashboard, selectedCluster, searchQuery, timeRange, setDashboard } = useStore();
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
    <div className="grid min-h-[calc(100vh-76px)] grid-cols-1 gap-0 2xl:grid-cols-[minmax(720px,1fr)_392px]">
      <section className="min-w-0 space-y-4 p-3 sm:p-4">
        {!!dashboard?.availability?.length && <AvailabilityBanner availability={dashboard.availability} />}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-5">
          {(dashboard?.kpis || []).map((kpi) => (
            <KpiCard key={kpi.label} kpi={kpi} />
          ))}
        </div>

        <IncidentTable
          incidents={filteredIncidents}
          totalIncidents={incidents.length}
          selectedId={selectedIncident?.id || null}
          onSelect={setSelectedId}
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

        <div className="grid grid-cols-1 gap-3 xl:grid-cols-[minmax(0,1fr)_360px]">
          <RemediationPipeline stages={dashboard?.pipeline || []} />
          <DependencyGraph incident={selectedIncident} />
        </div>
      </section>

      <IncidentDrawer incident={selectedIncident} />
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

const KpiCard = ({ kpi }: { kpi: DashboardKPI }) => {
  const tone = toneStyles[kpi.tone || 'info'] || toneStyles.info;
  return (
    <div className="min-h-[124px] rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
      <div className="text-[13px] font-semibold text-[#111827]">{kpi.label}</div>
      <div className="mt-3 flex items-end justify-between gap-3">
        <div>
          <div className={`text-[28px] font-semibold leading-none ${tone.text}`}>{kpi.value}</div>
          <div className="mt-3 text-[11px] text-[#6b7280]">
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
          <div className="max-h-[348px] overflow-auto">
            <table className="w-full min-w-[850px] border-collapse text-left text-[12px]">
              <thead className="sticky top-0 z-[1] bg-[#f8fafc] text-[#111827]">
                <tr>
                  <th className="w-8 px-3 py-3" />
                  <th className="px-3 py-3 font-semibold">Workload</th>
                  <th className="px-3 py-3 font-semibold">Namespace</th>
                  <th className="px-3 py-3 font-semibold">Status</th>
                  <th className="px-3 py-3 font-semibold">Root Cause</th>
                  <th className="px-3 py-3 font-semibold">Confidence</th>
                  <th className="px-3 py-3 font-semibold">Source</th>
                  <th className="px-3 py-3 font-semibold">Age</th>
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
                      <td className="px-3 py-4">
                        {selected ? <Circle className="h-3.5 w-3.5 fill-white text-[#94a3b8]" /> : <Circle className="h-3.5 w-3.5 text-[#94a3b8]" />}
                      </td>
                      <td className="px-3 py-4">
                        <div className="flex items-center gap-2 font-medium text-[#111827]">
                          <WorkloadIcon workload={incident.workload} />
                          <div className="min-w-0">
                            <div className="truncate">{incident.workload.kind}/{incident.workload.name}</div>
                            {incident.workload.podName && incident.workload.podName !== incident.workload.name && (
                              <div className="truncate text-[11px] font-normal text-[#647084]">Pod/{incident.workload.podName}</div>
                            )}
                          </div>
                        </div>
                      </td>
                      <td className="px-3 py-4 text-[#111827]">{incident.workload.namespace}</td>
                      <td className="px-3 py-4">
                        <StatusChip value={incident.status} severity={incident.severity} />
                      </td>
                      <td className="max-w-[220px] px-3 py-4 text-[#111827]">{incident.cause || 'Pending root cause'}</td>
                      <td className="px-3 py-4">
                        <Confidence value={incident.confidence} />
                      </td>
                      <td className="px-3 py-4 text-[#111827]">{incident.source}</td>
                      <td className={`px-3 py-4 font-medium ${incident.severity === 'critical' ? 'text-[#dc2626]' : 'text-[#f97316]'}`}>{incident.age}</td>
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

const IncidentDrawer = ({ incident }: { incident: DashboardIncident | null }) => (
  <aside className="border-t border-[#e5e7eb] bg-white p-3 2xl:border-l 2xl:border-t-0">
    {!incident ? (
      <EmptyState
        icon={<FileText className="h-8 w-8" />}
        title="Select an incident"
        message="Incident evidence, GitOps mapping, recommended pull request, and guardrail status will appear here."
      />
    ) : (
      <div className="space-y-3">
        <div className="flex items-start justify-between gap-3 px-1 py-2">
          <div>
            <div className="flex items-center gap-2">
              <TriangleAlert className="h-5 w-5 text-[#dc2626]" />
              <h2 className="text-[18px] font-semibold text-[#111827]">{incident.workload.kind}/{incident.workload.name}</h2>
              <StatusChip value={incident.status} severity={incident.severity} />
            </div>
            <div className="mt-2 text-[12px] text-[#4b5563]">
              {incident.workload.namespace} <span className="mx-2">•</span> {incident.age || 'recent'} ago <span className="mx-2">•</span> {incident.priority || 'P2'}
              {incident.workload.podName && incident.workload.podName !== incident.workload.name && (
                <><span className="mx-2">•</span> logs from Pod/{incident.workload.podName}</>
              )}
            </div>
          </div>
          <button className="grid h-8 w-8 place-items-center rounded-md hover:bg-[#f3f4f6]" title="Close">
            <X className="h-4 w-4" />
          </button>
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
              {renderValidationGuardrail(incident.guardrails) && (
                <RenderValidationBadge guardrail={renderValidationGuardrail(incident.guardrails)!} helm={isHelmGitOps(incident.gitops?.manifestType)} />
              )}
              {incident.guardrails.map((guardrail) => (
                <GuardrailRow key={guardrail.label} guardrail={guardrail} />
              ))}
            </div>
          ) : (
            <MiniEmpty message="Guardrail results will appear after remediation analysis." />
          )}
        </DrawerCard>
      </div>
    )}
  </aside>
);

const DrawerCard = ({ icon, title, children }: { icon: ReactNode; title: string; children: ReactNode }) => (
  <section className="overflow-hidden rounded-lg border border-[#e5e7eb] bg-white">
    <header className="flex h-12 items-center justify-between border-b border-[#e5e7eb] px-3">
      <div className="flex items-center gap-2 text-[14px] font-semibold text-[#111827]">
        {icon}
        {title}
      </div>
      <ChevronsUpDown className="h-4 w-4 text-[#6b7280]" />
    </header>
    {children}
  </section>
);

const EvidenceRow = ({ item }: { item: DashboardEvidence }) => (
  <div className="grid grid-cols-[84px_1fr_auto] items-center gap-3 px-3 py-3 text-[12px]">
    <div className="font-semibold text-[#111827]">{item.label}</div>
    <div className="line-clamp-2 text-[#374151]">{item.value}</div>
    <div className="flex items-center gap-2">
      {!!item.count && <span className="rounded-full bg-[#dbeafe] px-2 py-0.5 text-[11px] font-semibold text-[#2563eb]">{item.count}</span>}
      {item.link && <button className="font-medium text-[#2563eb]">{item.link}</button>}
    </div>
  </div>
);

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
        <div className="min-w-[760px]">
          <div className="relative grid grid-cols-7 border-b border-[#e5e7eb] px-2 pb-2 pt-5">
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
          <div className="grid grid-cols-7">
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

const DependencyGraph = ({ incident }: { incident: DashboardIncident | null }) => {
  const nodes = incident?.graph || [];
  const edges = incident?.edges || [];
  const incidentId = incident?.id || null;
  const [graphState, setGraphState] = useState<GraphState>({ incidentId: null, selectedNodeId: null, collapsedNodeIds: new Set(), expanded: false });

  const activeGraphState = graphState.incidentId === incidentId
    ? graphState
    : { incidentId, selectedNodeId: nodes[0]?.id || null, collapsedNodeIds: new Set<string>(), expanded: false };
  const selectedNodeId = activeGraphState.selectedNodeId || nodes[0]?.id || null;
  const collapsedNodeIds = activeGraphState.collapsedNodeIds;
  const expanded = activeGraphState.expanded;

  const visibleNodeIds = visibleGraphNodeIds(nodes, edges, collapsedNodeIds);
  const visibleNodes = nodes.filter((node) => visibleNodeIds.has(node.id));
  const visibleEdges = edges.filter(([from, to]) => visibleNodeIds.has(from) && visibleNodeIds.has(to) && !collapsedNodeIds.has(from));
  const selectedNode = nodes.find((node) => node.id === selectedNodeId) || visibleNodes[0] || null;
  const hasChildren = !!selectedNode && edges.some(([from]) => from === selectedNode.id);

  const toggleSelectedChildren = () => {
    if (!selectedNode) return;
    setGraphState((current) => {
      const existing = graphStateForIncident(current, incidentId, nodes);
      const next = new Set(existing.collapsedNodeIds);
      if (next.has(selectedNode.id)) {
        next.delete(selectedNode.id);
      } else {
        next.add(selectedNode.id);
      }
      return { ...existing, collapsedNodeIds: next };
    });
  };

  return (
    <section className={`rounded-lg border border-[#e5e7eb] bg-white p-3 shadow-[0_1px_2px_rgba(15,23,42,0.04)] ${expanded ? 'fixed inset-2 z-40 sm:inset-4' : ''}`}>
      <div className="mb-3 flex items-center gap-2">
        <h2 className="text-[14px] font-semibold text-[#111827]">Dependency Graph</h2>
        <AlertCircle className="h-3.5 w-3.5 text-[#94a3b8]" />
        <button
          onClick={() => setGraphState((current) => graphStateForIncident(current, incidentId, nodes, { expanded: !expanded }))}
          className="ml-auto grid h-8 w-8 place-items-center rounded-md border border-[#e5e7eb] bg-white text-[#374151] hover:bg-[#f8fafc]"
          title={expanded ? 'Collapse graph' : 'Expand graph'}
        >
          {expanded ? <X className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
        </button>
      </div>
      {incident && nodes.length ? (
        <div className={`grid gap-3 ${expanded ? 'h-[calc(100%-44px)] grid-cols-1 xl:grid-cols-[1fr_300px]' : 'grid-cols-1'}`}>
          <GraphCanvas
            nodes={visibleNodes}
            edges={visibleEdges}
            selectedNodeId={selectedNode?.id || null}
            onSelect={(id) => setGraphState((current) => graphStateForIncident(current, incidentId, nodes, { selectedNodeId: id }))}
            expanded={expanded}
          />
          {!expanded && (
            <GraphFooter
              onExpand={() => setGraphState((current) => graphStateForIncident(current, incidentId, nodes, { expanded: true }))}
            />
          )}
          <div className={expanded ? 'block min-h-0 overflow-y-auto' : 'hidden'}>
            <ResourceDetailPanel
              node={selectedNode}
              nodes={nodes}
              edges={edges}
              incident={incident}
              isCollapsed={!!selectedNode && collapsedNodeIds.has(selectedNode.id)}
              hasChildren={hasChildren}
              hiddenCount={nodes.length - visibleNodes.length}
              onToggleChildren={toggleSelectedChildren}
              onExpandAll={() => setGraphState((current) => graphStateForIncident(current, incidentId, nodes, { collapsedNodeIds: new Set() }))}
            />
          </div>
        </div>
      ) : (
        <MiniEmpty message="No dependency graph has been captured for this workload yet." />
      )}
    </section>
  );
};

const GraphFooter = ({ onExpand }: { onExpand: () => void }) => (
  <div className="flex items-center justify-between px-1 pt-1">
    <div className="flex items-center gap-2 text-[12px] text-[#647084]">
      <span className="h-2 w-2 rounded-full bg-[#16a34a]" />
      Live view
    </div>
    <div className="flex items-center gap-2">
      <button
        onClick={onExpand}
        className="grid h-8 w-8 place-items-center rounded-md border border-[#e5e7eb] bg-white text-[#374151] hover:bg-[#f8fafc]"
        title="Expand graph"
      >
        <Maximize2 className="h-4 w-4" />
      </button>
      <button
        className="grid h-8 w-8 place-items-center rounded-md border border-[#e5e7eb] bg-white text-[#374151]"
        title="Zoom controls are available in expanded view"
        type="button"
      >
        +
      </button>
      <button
        className="grid h-8 w-8 place-items-center rounded-md border border-[#e5e7eb] bg-white text-[#374151]"
        title="Zoom controls are available in expanded view"
        type="button"
      >
        -
      </button>
    </div>
  </div>
);

const GraphCanvas = ({
  nodes,
  edges,
  selectedNodeId,
  onSelect,
  expanded,
}: {
  nodes: DashboardDependencyNode[];
  edges: DashboardDependencyEdge[];
  selectedNodeId: string | null;
  onSelect: (id: string) => void;
  expanded: boolean;
}) => {
  const layouted = layoutGraph(nodes, edges);
  const flowNodes = layouted.map<FlowNode>((node) => ({
    id: node.id,
    type: 'default',
    position: { x: node.x, y: node.y },
    data: { label: <FlowNodeLabel node={node} /> },
    style: flowNodeStyle(node, node.id === selectedNodeId, expanded),
    sourcePosition: Position.Bottom,
    targetPosition: Position.Top,
  }));
  const flowEdges = edges.map<Edge>(([from, to]) => ({
    id: `${from}-${to}`,
    source: from,
    target: to,
    type: 'smoothstep',
    animated: false,
    markerEnd: { type: MarkerType.ArrowClosed, color: '#94a3b8' },
    style: { stroke: '#94a3b8', strokeWidth: 1.5 },
  }));

  return (
    <div className={`${expanded ? 'h-full min-h-[420px]' : 'h-[270px]'} overflow-hidden rounded-md border border-[#e5e7eb] bg-[#fbfdff]`}>
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        onNodeClick={(_, node) => onSelect(node.id)}
        fitView
        fitViewOptions={{ padding: expanded ? 0.18 : 0.38, maxZoom: expanded ? 1.2 : 0.82 }}
        minZoom={0.2}
        maxZoom={expanded ? 1.8 : 0.95}
        nodesDraggable={false}
        panOnDrag={expanded}
        panOnScroll={expanded}
        zoomOnScroll={expanded}
        zoomOnPinch={expanded}
        zoomOnDoubleClick={expanded}
        preventScrolling={expanded}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#dbe3ef" gap={18} />
        {expanded && (
          <>
            <Controls position="bottom-right" showInteractive={false} />
            <MiniMap
              position="bottom-left"
              pannable
              zoomable
              nodeColor={(node) => {
                const original = nodes.find((item) => item.id === node.id);
                return graphNodeColor(original?.kind || original?.label || '');
              }}
              maskColor="rgba(241,245,249,0.72)"
            />
            <Panel position="top-left" className="rounded-md border border-[#e5e7eb] bg-white/90 px-2 py-1 text-[11px] font-medium text-[#475569] shadow-sm">
              Auto-layout · pan · zoom · select resources
            </Panel>
          </>
        )}
      </ReactFlow>
    </div>
  );
};

type FlowNode = Node<{ label: ReactNode }>;
type GraphState = {
  incidentId: string | null;
  selectedNodeId: string | null;
  collapsedNodeIds: Set<string>;
  expanded: boolean;
};

const graphStateForIncident = (
  current: GraphState,
  incidentId: string | null,
  nodes: DashboardDependencyNode[],
  patch?: Partial<GraphState>,
): GraphState => {
  const base = current.incidentId === incidentId
    ? current
    : { incidentId, selectedNodeId: nodes[0]?.id || null, collapsedNodeIds: new Set<string>(), expanded: false };
  return { ...base, ...patch, incidentId };
};

const FlowNodeLabel = ({ node }: { node: DashboardDependencyNode }) => {
  const color = graphNodeColor(node.kind || node.label);
  return (
    <div className="flex min-w-0 items-center gap-2 text-left text-[10px] leading-tight">
      <span className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-white" style={{ backgroundColor: color }}>
        {renderWorkloadGlyph(node.label, 'h-4 w-4')}
      </span>
      <span className="min-w-0">
        <span className="block truncate font-semibold text-[#111827]">{node.label}</span>
        <span className="block max-w-[92px] truncate text-[#475569]">{node.detail}</span>
      </span>
    </div>
  );
};

const ResourceDetailPanel = ({
  node,
  nodes,
  edges,
  incident,
  isCollapsed,
  hasChildren,
  hiddenCount,
  onToggleChildren,
  onExpandAll,
}: {
  node: DashboardDependencyNode | null;
  nodes: DashboardDependencyNode[];
  edges: DashboardDependencyEdge[];
  incident: DashboardIncident;
  isCollapsed: boolean;
  hasChildren: boolean;
  hiddenCount: number;
  onToggleChildren: () => void;
  onExpandAll: () => void;
}) => {
  if (!node) {
    return <MiniEmpty message="Select a graph node to inspect its Kubernetes resource context." />;
  }
  const relatedIds = edges
    .filter(([from, to]) => from === node.id || to === node.id)
    .map(([from, to]) => (from === node.id ? to : from));
  const relatedNodes = relatedIds
    .map((id) => nodes.find((candidate) => candidate.id === id))
    .filter((candidate): candidate is DashboardDependencyNode => !!candidate);

  return (
    <div className="rounded-md border border-[#e5e7eb] bg-white p-3 text-[12px]">
      <div className="flex items-start gap-2">
        <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-white" style={{ backgroundColor: graphNodeColor(node.kind || node.label) }}>
          {renderWorkloadGlyph(node.label, 'h-4 w-4')}
        </span>
        <div className="min-w-0">
          <div className="font-semibold text-[#111827]">{node.label}</div>
          <div className="truncate text-[#647084]">{node.detail}</div>
        </div>
      </div>
      <div className="mt-3 space-y-2 border-t border-[#e5e7eb] pt-3">
        <DetailLine label="Resource" value={`${node.label}/${node.detail}`} />
        <DetailLine label="Namespace" value={incident.workload.namespace} />
        <DetailLine label="Status" value={incident.status} />
        <DetailLine label="Root cause" value={incident.cause || 'Pending'} />
        <DetailLine label="Confidence" value={`${incident.confidence || 0}%`} />
        <DetailLine label="Signal" value={incident.source || 'Unknown'} />
        <DetailLine label="Risk" value={incident.risk || 'Unknown'} />
        {incident.gitops && <DetailLine label="GitOps" value={`${incident.gitops.controller} · ${incident.gitops.path || incident.gitops.repo}`} />}
        {incident.gitops?.helm && isHelmNode(node) && (
          <>
            <DetailLine label="Chart" value={helmChartLabel(incident.gitops)} />
            <DetailLine label="Release" value={incident.gitops.helm.releaseName || incident.gitops.app || 'Unknown'} />
            <DetailLine label="Values" value={(incident.gitops.helm.valueFiles || []).join(' → ') || 'Not reported'} />
          </>
        )}
        {incident.pr && <DetailLine label="PR" value={incident.pr.branch || incident.pr.title} />}
      </div>
      <div className="mt-3 border-t border-[#e5e7eb] pt-3">
        <div className="mb-2 font-semibold text-[#111827]">Related resources</div>
        {relatedNodes.length ? (
          <div className="space-y-1.5">
            {relatedNodes.map((related) => (
              <div key={related.id} className="flex items-center gap-2 rounded-md bg-[#f8fafc] px-2 py-1.5">
                <span className="grid h-5 w-5 place-items-center rounded text-white" style={{ backgroundColor: graphNodeColor(related.kind || related.label) }}>
                  {renderWorkloadGlyph(related.label, 'h-3 w-3')}
                </span>
                <span className="min-w-0 truncate text-[#475569]">{related.label}/{related.detail}</span>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-[#647084]">No direct dependencies recorded.</div>
        )}
      </div>
      <div className="mt-3 flex flex-wrap gap-2">
        {hasChildren && (
          <button onClick={onToggleChildren} className="rounded-md border border-[#e5e7eb] px-2 py-1 text-[11px] font-medium text-[#374151] hover:bg-[#f8fafc]">
            {isCollapsed ? 'Expand children' : 'Collapse children'}
          </button>
        )}
        {hiddenCount > 0 && (
          <button onClick={onExpandAll} className="rounded-md border border-[#bfdbfe] bg-[#eff6ff] px-2 py-1 text-[11px] font-medium text-[#2563eb]">
            Expand all ({hiddenCount} hidden)
          </button>
        )}
      </div>
    </div>
  );
};

const DetailLine = ({ label, value }: { label: string; value: string }) => (
  <div className="grid grid-cols-[72px_1fr] gap-2">
    <span className="font-semibold text-[#111827]">{label}</span>
    <span className="min-w-0 truncate text-[#475569]">{value || 'Unknown'}</span>
  </div>
);

const layoutGraph = (nodes: DashboardDependencyNode[], edges: DashboardDependencyEdge[]): DashboardDependencyNode[] => {
  if (nodes.length === 0) return [];

  const nodeIds = new Set(nodes.map((node) => node.id));
  const outgoing = new Map<string, string[]>();
  const incomingCount = new Map<string, number>();
  nodes.forEach((node) => incomingCount.set(node.id, 0));

  edges.forEach(([from, to]) => {
    if (!nodeIds.has(from) || !nodeIds.has(to)) return;
    outgoing.set(from, [...(outgoing.get(from) || []), to]);
    incomingCount.set(to, (incomingCount.get(to) || 0) + 1);
  });

  const roots = nodes.filter((node) => (incomingCount.get(node.id) || 0) === 0);
  const queue = roots.length ? roots.map((node) => node.id) : [nodes[0].id];
  const levels = new Map<string, number>();
  queue.forEach((id) => levels.set(id, 0));

  for (let index = 0; index < queue.length; index += 1) {
    const id = queue[index];
    const level = levels.get(id) || 0;
    for (const child of outgoing.get(id) || []) {
      const nextLevel = level + 1;
      if (!levels.has(child) || nextLevel > (levels.get(child) || 0)) {
        levels.set(child, nextLevel);
        queue.push(child);
      }
    }
  }

  nodes.forEach((node) => {
    if (!levels.has(node.id)) {
      levels.set(node.id, 0);
    }
  });

  const grouped = new Map<number, DashboardDependencyNode[]>();
  nodes.forEach((node) => {
    const level = levels.get(node.id) || 0;
    grouped.set(level, [...(grouped.get(level) || []), node]);
  });

  return nodes.map((node) => {
    const level = levels.get(node.id) || 0;
    const group = grouped.get(level) || [node];
    const index = group.findIndex((item) => item.id === node.id);
    const total = group.length;
    const x = index * 190 - ((total - 1) * 190) / 2;
    const y = level * 118;
    return { ...node, x, y };
  });
};

const visibleGraphNodeIds = (nodes: DashboardDependencyNode[], edges: DashboardDependencyEdge[], collapsed: Set<string>) => {
  const visible = new Set(nodes.map((node) => node.id));
  const children = new Map<string, string[]>();
  edges.forEach(([from, to]) => {
    children.set(from, [...(children.get(from) || []), to]);
  });

  const hideDescendants = (id: string) => {
    for (const child of children.get(id) || []) {
      if (!visible.has(child)) continue;
      visible.delete(child);
      hideDescendants(child);
    }
  };

  collapsed.forEach(hideDescendants);
  return visible;
};

const flowNodeStyle = (node: DashboardDependencyNode, selected: boolean, expanded: boolean) => ({
  width: expanded ? 150 : 142,
  minHeight: expanded ? 54 : 52,
  padding: expanded ? '8px' : '7px',
  borderRadius: 8,
  border: selected ? '2px solid #2563eb' : '1px solid #dbe3ef',
  boxShadow: selected ? '0 8px 24px rgba(37,99,235,0.16)' : '0 1px 2px rgba(15,23,42,0.04)',
  background: node.kind === 'active' ? '#eff6ff' : '#ffffff',
});

const graphNodeColor = (kind: string) => {
  const lower = kind.toLowerCase();
  if (lower.includes('helm') || lower.includes('chart')) return '#2563eb';
  if (lower.includes('active') || lower.includes('deployment')) return '#2563eb';
  if (lower.includes('warning') || lower.includes('config') || lower.includes('secret')) return '#f97316';
  if (lower.includes('stateful')) return '#0ea5e9';
  if (lower.includes('service')) return '#16a34a';
  return '#64748b';
};

const KeyValueRows = ({ rows, compact = false }: { rows: [string, string | undefined][]; compact?: boolean }) => (
  <div className={`space-y-2 text-[12px] ${compact ? '' : 'px-3 py-3'}`}>
    {rows.filter(([, value]) => !!value).map(([label, value]) => (
      <div key={label} className="grid grid-cols-[84px_1fr] gap-3">
        <span className="font-semibold text-[#111827]">{label}</span>
        <span className="min-w-0 truncate text-[#111827]">{value}</span>
      </div>
    ))}
  </div>
);

const RenderValidationBadge = ({ guardrail, helm }: { guardrail: DashboardGuardrail; helm?: boolean }) => {
  const tone = guardrailTone(guardrail.status);
  return (
    <div className="border-b border-[#e5e7eb] px-3 py-3">
      <div className="flex items-center justify-between gap-3 rounded-md border border-[#e5e7eb] bg-[#f8fafc] px-3 py-2 text-[12px]">
        <div className="min-w-0">
          <div className="font-semibold text-[#111827]">{renderValidationTitle(guardrail.status, helm)}</div>
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

const renderValidationGuardrail = (guardrails: DashboardGuardrail[]) =>
  guardrails.find((guardrail) => /render/i.test(guardrail.label));

const renderValidationTitle = (status: string, helm?: boolean) => {
  const prefix = helm ? 'Helm render' : 'Render validation';
  switch ((status || '').toLowerCase()) {
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

const isHelmNode = (node: DashboardDependencyNode) => /helm|chart|values|render/i.test(`${node.label} ${node.kind}`);

const helmChartLabel = (gitops: DashboardGitOpsMapping) => {
  const chart = gitops.helm?.chart || 'Unknown chart';
  return gitops.helm?.chartVersion ? `${chart}@${gitops.helm.chartVersion}` : chart;
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
  return (
    <span className={`inline-flex rounded px-2 py-0.5 text-[11px] font-medium ${critical ? 'border border-[#fca5a5] bg-[#fee2e2] text-[#dc2626]' : 'border border-[#fed7aa] bg-[#ffedd5] text-[#f97316]'}`}>
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
    className={`grid h-8 min-w-8 place-items-center rounded-md border px-2 text-[12px] ${
      active ? 'border-[#94a3b8] bg-white text-[#111827]' : 'border-[#e5e7eb] bg-white text-[#4b5563] disabled:text-[#cbd5e1]'
    }`}
  >
    {icon || label}
  </button>
);

const EmptyState = ({ icon, title, message }: { icon: ReactNode; title: string; message: string }) => (
  <div className="grid min-h-[180px] place-items-center px-6 py-10 text-center">
    <div className="max-w-md text-[#647084]">
      <div className="mx-auto mb-3 grid h-12 w-12 place-items-center rounded-full bg-[#f1f5f9] text-[#94a3b8]">{icon}</div>
      <strong className="text-[14px] text-[#111827]">{title}</strong>
      <p className="mt-1 text-[13px] leading-5">{message}</p>
    </div>
  </div>
);

const MiniEmpty = ({ message }: { message: string }) => <div className="px-3 py-4 text-[12px] leading-5 text-[#647084]">{message}</div>;
