import { useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import {
  Activity,
  AlertCircle,
  Boxes,
  Brain,
  Container,
  Database,
  FileCode2,
  FileText,
  GitPullRequestArrow,
  Layers3,
  LineChart,
  Network,
  Search,
  Server,
  ShieldCheck,
  SlidersHorizontal,
  TriangleAlert,
} from 'lucide-react';
import { useStore } from '../store/useStore';
import type {
  DashboardEvidence,
  DashboardGitOpsMapping,
  DashboardIncident,
  DashboardLogPattern,
  DashboardPrediction,
  DashboardRCA,
  DashboardRemediation,
  DashboardState,
  DashboardWorkload,
  DashboardWorkloadPolicy,
  DashboardWorkloadView,
} from '../types';

type WorkloadRecord = {
  key: string;
  cluster?: string;
  workload: DashboardWorkload;
  incidents: DashboardIncident[];
  remediations: DashboardRemediation[];
  predictions: DashboardPrediction[];
  gitops?: DashboardGitOpsMapping;
  incidentCount?: number;
  remediationCount?: number;
  predictionCount?: number;
  health?: string;
  status?: string;
  desired?: number;
  ready?: number;
  evidence?: DashboardEvidence[];
  logPatterns?: DashboardLogPattern[];
  rca?: DashboardRCA;
  policyState?: DashboardWorkloadPolicy;
  pods?: string[];
  services?: string[];
  ingresses?: string[];
  nodes?: string[];
};

export const Workloads = () => {
  const dashboard = useStore((state) => state.dashboard);
  const globalSearch = useStore((state) => state.searchQuery);
  const selectedCluster = useStore((state) => state.selectedCluster);
  const [kindFilter, setKindFilter] = useState('all');
  const [localSearch, setLocalSearch] = useState('');
  const workloads = useMemo(() => buildWorkloadRecords(dashboard), [dashboard]);
  const query = `${globalSearch} ${localSearch}`.trim();
  const kinds = Array.from(new Set(workloads.map((item) => item.workload.kind))).sort();
  const filtered = workloads.filter((item) => {
    if (kindFilter !== 'all' && item.workload.kind !== kindFilter) return false;
    if (selectedCluster && selectedCluster !== dashboard?.environment) {
      const inCluster = item.cluster === selectedCluster || item.incidents.some((incident) => incident.cluster === selectedCluster);
      if (!inCluster) return false;
    }
    return matchesQuery(query, [
      item.workload.kind,
      item.workload.name,
      item.workload.namespace,
      item.workload.podName,
      item.gitops?.app,
      item.gitops?.repo,
      item.gitops?.path,
      item.gitops?.helm?.chart,
      item.health,
      item.status,
      ...(item.pods || []),
      ...(item.services || []),
      ...(item.ingresses || []),
      ...(item.incidents || []).map((incident) => `${incident.status} ${incident.cause} ${incident.source}`),
      ...(item.remediations || []).map((remediation) => `${remediation.status} ${remediation.strategy} ${remediation.repository}`),
    ]);
  });
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const selected = filtered.find((item) => item.key === selectedKey) || filtered[0] || null;

  return (
    <div className="grid min-h-[calc(100vh-76px)] grid-cols-1 gap-4 p-4 xl:grid-cols-[minmax(0,1fr)_420px]">
      <section className="min-w-0 space-y-4">
        <div className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h1 className="text-[20px] font-semibold text-[#111827]">Workloads</h1>
              <p className="mt-1 text-[13px] text-[#647084]">A controller-aware inventory of workloads, GitOps sources, incidents, remediations, evidence, and policy state.</p>
            </div>
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <div className="flex h-9 min-w-[260px] items-center gap-2 rounded-md border border-[#e5e7eb] px-3 text-[#647084]">
                <Search className="h-4 w-4" />
                <input
                  value={localSearch}
                  onChange={(event) => setLocalSearch(event.target.value)}
                  className="min-w-0 flex-1 bg-transparent text-[13px] outline-none"
                  placeholder="Filter workloads, GitOps, incidents..."
                />
              </div>
              <label className="flex h-9 items-center gap-2 rounded-md border border-[#e5e7eb] px-3 text-[12px] font-medium text-[#374151]">
                <SlidersHorizontal className="h-4 w-4" />
                <select value={kindFilter} onChange={(event) => setKindFilter(event.target.value)} className="bg-transparent outline-none">
                  <option value="all">All workload types</option>
                  {kinds.map((kind) => <option key={kind} value={kind}>{kind}</option>)}
                </select>
              </label>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
          <SummaryCard label="Workloads tracked" value={`${filtered.length}`} detail={`${workloads.length} total discovered`} icon={<Boxes className="h-4 w-4" />} tone="blue" />
          <SummaryCard label="Active incidents" value={`${filtered.reduce((sum, item) => sum + workloadIncidentCount(item), 0)}`} detail="controller-level issue grouping" icon={<TriangleAlert className="h-4 w-4" />} tone="red" />
          <SummaryCard label="GitOps mapped" value={`${filtered.filter((item) => item.gitops).length}`} detail="ArgoCD, Flux, Helm, or repo source" icon={<FileCode2 className="h-4 w-4" />} tone="green" />
        </div>

        <div className="overflow-hidden rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
          {filtered.length ? (
            <div className="max-h-[calc(100vh-290px)] overflow-auto">
              <table className="w-full min-w-[920px] text-left text-[12px]">
                <thead className="sticky top-0 z-[1] bg-[#f8fafc] text-[#111827]">
                  <tr>
                    <th className="px-4 py-3 font-semibold">Workload</th>
                    <th className="px-4 py-3 font-semibold">Namespace</th>
                    <th className="px-4 py-3 font-semibold">Health</th>
                    <th className="px-4 py-3 font-semibold">GitOps Source</th>
                    <th className="px-4 py-3 font-semibold">Remediation</th>
                    <th className="px-4 py-3 font-semibold">Prediction</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((item) => (
                    <tr
                      key={item.key}
                      onClick={() => setSelectedKey(item.key)}
                      className={`cursor-pointer border-t border-[#e5e7eb] hover:bg-[#f8fafc] ${selected?.key === item.key ? 'bg-[#eff6ff] shadow-[inset_3px_0_0_#2563eb]' : ''}`}
                    >
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <WorkloadIcon workload={item.workload} />
                          <div className="min-w-0">
                            <div className="truncate font-semibold text-[#111827]">{item.workload.kind}/{item.workload.name}</div>
                            {item.workload.podName && <div className="truncate text-[11px] text-[#647084]">Pod signal: {item.workload.podName}</div>}
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-3">{item.workload.namespace}</td>
                      <td className="px-4 py-3"><HealthPill record={item} /></td>
                      <td className="px-4 py-3">
                        {item.gitops ? (
                          <div className="max-w-[280px]">
                            <div className="truncate font-medium text-[#111827]">{item.gitops.controller} · {item.gitops.app || item.gitops.repo}</div>
                            <div className="truncate text-[11px] text-[#647084]">{readableManifestType(item.gitops.manifestType)} · {item.gitops.path || item.gitops.repo}</div>
                          </div>
                        ) : <span className="text-[#94a3b8]">Not mapped</span>}
                      </td>
                      <td className="px-4 py-3"><RemediationStatus record={item} /></td>
                      <td className="px-4 py-3"><PredictionStatus record={item} /></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <EmptyState title="No workloads match the current filters" message="Workloads appear after incidents, GitOps mappings, remediations, or predictive signals are collected." />
          )}
        </div>
      </section>

      <WorkloadDetail record={selected} policyMode={dashboard?.policy?.mode || ''} />
    </div>
  );
};

const WorkloadDetail = ({ record, policyMode }: { record: WorkloadRecord | null; policyMode: string }) => {
  if (!record) {
    return <aside className="rounded-lg border border-[#e5e7eb] bg-white"><EmptyState title="Select a workload" message="Workload evidence, logs, remediation history, GitOps mapping, and policy context will appear here." /></aside>;
  }
  const latestIncident = record.incidents[0];
  const evidence = record.evidence?.length ? record.evidence : latestIncident?.evidence || [];
  const logPatterns = record.logPatterns?.length ? record.logPatterns : latestIncident?.logPatterns || [];
  const logEvidence = evidence.filter((item) => /log|stack|event|trace|pattern/i.test(`${item.label} ${item.icon}`));
  const activeRemediation = record.remediations[0];

  return (
    <aside className="min-w-0 space-y-3">
      <section className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
        <div className="flex items-start gap-3">
          <WorkloadIcon workload={record.workload} large />
          <div className="min-w-0">
            <h2 className="truncate text-[18px] font-semibold text-[#111827]">{record.workload.kind}/{record.workload.name}</h2>
            <p className="mt-1 text-[12px] text-[#647084]">{record.workload.namespace}{record.workload.podName ? ` · signal pod ${record.workload.podName}` : ''}</p>
          </div>
        </div>
        <div className="mt-4 grid grid-cols-3 gap-2 text-center text-[12px]">
          <MetricBox label="Incidents" value={`${workloadIncidentCount(record)}`} tone="red" />
          <MetricBox label="PRs" value={`${workloadRemediationCount(record)}`} tone="blue" />
          <MetricBox label="Predictions" value={`${workloadPredictionCount(record)}`} tone="green" />
        </div>
        {(record.desired || record.ready) !== undefined && (
          <div className="mt-3 rounded-md border border-[#e5e7eb] bg-[#f8fafc] px-3 py-2 text-[12px] text-[#475569]">
            Ready {record.ready ?? 0} / Desired {record.desired ?? 0}
          </div>
        )}
      </section>

      <DetailCard icon={<FileText className="h-4 w-4" />} title="Evidence & Log Patterns">
        {evidence.length ? (
          <div className="divide-y divide-[#e5e7eb]">
            {evidence.slice(0, 6).map((item) => <EvidenceLine key={`${item.label}-${item.value}`} item={item} />)}
          </div>
        ) : <MiniEmpty message="No evidence has been captured for this workload yet." />}
        {logEvidence.length > 0 && (
          <div className="border-t border-[#e5e7eb] p-3">
            <div className="mb-2 text-[11px] font-semibold uppercase text-[#647084]">Evidence snippets</div>
            <div className="space-y-2">
              {logEvidence.slice(0, 3).map((item) => (
                <pre key={`log-${item.label}-${item.value}`} className="max-h-28 overflow-auto rounded-md bg-[#0f172a] p-2 text-[11px] leading-5 text-[#f8fafc]">{item.value}</pre>
              ))}
            </div>
          </div>
        )}
        {logPatterns.length > 0 && (
          <div className="border-t border-[#e5e7eb] p-3">
            <div className="mb-2 text-[11px] font-semibold uppercase text-[#647084]">Backend log patterns</div>
            <div className="space-y-2">
              {logPatterns.slice(0, 4).map((item) => (
                <div key={`${item.source}-${item.pattern}`} className="rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-2 text-[12px]">
                  <div className="flex items-center justify-between gap-2 font-semibold text-[#111827]">
                    <span>{item.label}</span>
                    {!!item.count && <span className="rounded-full bg-[#dbeafe] px-2 py-0.5 text-[11px] text-[#2563eb]">{item.count}</span>}
                  </div>
                  <pre className="mt-1 max-h-24 overflow-auto whitespace-pre-wrap rounded bg-[#0f172a] p-2 text-[11px] leading-5 text-[#f8fafc]">{item.pattern}</pre>
                </div>
              ))}
            </div>
          </div>
        )}
      </DetailCard>

      <DetailCard icon={<Brain className="h-4 w-4" />} title="RCA & Remediation Explainability">
        {record.rca || latestIncident ? (
          <div className="space-y-3 p-3 text-[12px]">
            <KeyValue label="Root cause" value={record.rca?.summary || latestIncident?.cause || 'Pending'} />
            <KeyValue label="Confidence" value={`${record.rca?.confidence ?? latestIncident?.confidence ?? 0}%`} />
            <KeyValue label="Source" value={record.rca?.signal || latestIncident?.source || 'Unknown'} />
            <KeyValue label="Risk" value={record.rca?.risk || latestIncident?.risk || 'Unknown'} />
            {record.rca?.recommendedAction && <KeyValue label="Action" value={record.rca.recommendedAction} />}
            {record.rca?.negativeFeedback && <KeyValue label="Feedback" value={record.rca.negativeFeedback} />}
            {latestIncident?.pr?.reason && <KeyValue label="PR reason" value={latestIncident.pr.reason} />}
          </div>
        ) : <MiniEmpty message="RCA details appear after Fixora investigates this workload." />}
      </DetailCard>

      <DetailCard icon={<FileCode2 className="h-4 w-4" />} title="GitOps Topology">
        {record.gitops ? <GitOpsSummary gitops={record.gitops} /> : <MiniEmpty message="No GitOps source has been mapped for this workload." />}
      </DetailCard>

      <DetailCard icon={<GitPullRequestArrow className="h-4 w-4" />} title="Remediation Flow">
        {activeRemediation ? (
          <div className="space-y-2 p-3 text-[12px]">
            <KeyValue label="Status" value={activeRemediation.status} />
            <KeyValue label="Strategy" value={activeRemediation.strategy || 'Pending'} />
            <KeyValue label="Repository" value={activeRemediation.repository || 'Unknown'} />
            <KeyValue label="Branch" value={activeRemediation.headBranch || activeRemediation.baseBranch || 'Pending'} />
            {activeRemediation.prUrl && <a className="inline-flex items-center gap-1 font-medium text-[#2563eb]" href={activeRemediation.prUrl} target="_blank" rel="noreferrer">Open PR <GitPullRequestArrow className="h-3.5 w-3.5" /></a>}
          </div>
        ) : <MiniEmpty message="No remediation workflow is attached to this workload yet." />}
      </DetailCard>

      <DetailCard icon={<ShieldCheck className="h-4 w-4" />} title="Policy & SLO">
        <div className="space-y-2 p-3 text-[12px]">
          <KeyValue label="Fixora mode" value={record.policyState?.mode || policyMode || 'Unknown'} />
          <KeyValue label="Auto-fix gate" value={record.policyState?.autoFix ? 'Enabled by policy' : 'Requires policy or approval'} />
          <KeyValue label="Approval" value={record.policyState?.approvalRequired ? 'Required' : 'Not required'} />
          <KeyValue label="Availability SLO" value={record.policyState?.availabilitySlo ? `${(record.policyState.availabilitySlo * 100).toFixed(2)}%` : 'Not configured'} />
          <KeyValue label="Burn rate" value={record.policyState?.burnRateThreshold ? `${record.policyState.burnRateThreshold}x` : 'Not configured'} />
          <p className="rounded-md border border-dashed border-[#cbd5e1] bg-[#f8fafc] p-2 text-[#647084]">Policy edits still require safe settings APIs before the UI mutates remediation behavior.</p>
        </div>
      </DetailCard>
    </aside>
  );
};

const buildWorkloadRecords = (dashboard: DashboardState | null): WorkloadRecord[] => {
  if (dashboard?.workloads?.length) {
    return dashboard.workloads.map((view) => workloadRecordFromBackend(view, dashboard)).sort(sortWorkloadRecords);
  }

  const records = new Map<string, WorkloadRecord>();
  const ensure = (workload: DashboardWorkload) => {
    const key = workloadKey(workload);
    if (!records.has(key)) {
      records.set(key, { key, workload, incidents: [], remediations: [], predictions: [] });
    }
    return records.get(key)!;
  };

  (dashboard?.incidents || []).forEach((incident) => {
    const record = ensure(incident.workload);
    record.incidents.push(incident);
    record.gitops = record.gitops || incident.gitops;
    record.evidence = record.evidence || incident.evidence;
    record.logPatterns = record.logPatterns || incident.logPatterns;
    record.rca = record.rca || incident.rca;
    record.policyState = record.policyState || incident.policyState;
  });
  (dashboard?.remediations || []).forEach((remediation) => {
    const record = ensure(remediation.workload);
    record.remediations.push(remediation);
    record.gitops = record.gitops || remediation.gitops;
  });
  (dashboard?.predictions || []).forEach((prediction) => {
    const workload = { kind: 'Pod', name: prediction.podName, namespace: prediction.namespace };
    ensure(workload).predictions.push(prediction);
  });
  (dashboard?.gitopsSources || []).forEach((source) => {
    const workload = { kind: source.helm ? 'HelmRelease' : readableManifestType(source.manifestType), name: source.app || source.path || source.repo, namespace: source.namespace || source.helm?.namespace || 'unknown' };
    const record = ensure(workload);
    record.gitops = record.gitops || source;
  });

  return Array.from(records.values()).sort(sortWorkloadRecords);
};

const workloadRecordFromBackend = (view: DashboardWorkloadView, dashboard: DashboardState): WorkloadRecord => {
  const incident = view.latestIncidentId ? (dashboard.incidents || []).find((item) => item.id === view.latestIncidentId) : undefined;
  const remediations = (dashboard.remediations || []).filter((item) => workloadKey(item.workload) === workloadKey(view.workload));
  const predictions = (dashboard.predictions || []).filter((item) => item.namespace === view.workload.namespace && item.podName === view.workload.name);
  return {
    key: view.id,
    cluster: view.cluster,
    workload: view.workload,
    incidents: incident ? [incident] : [],
    remediations,
    predictions,
    gitops: view.gitops,
    incidentCount: view.incidents,
    remediationCount: view.remediations,
    predictionCount: view.predictions,
    health: view.health,
    status: view.status,
    desired: view.desired,
    ready: view.ready,
    evidence: view.evidence,
    logPatterns: view.logPatterns,
    rca: view.rca,
    policyState: view.policyState,
    pods: view.pods,
    services: view.services,
    ingresses: view.ingresses,
    nodes: view.nodes,
  };
};

const sortWorkloadRecords = (a: WorkloadRecord, b: WorkloadRecord) => {
  const aHot = workloadIncidentCount(a) ? 0 : 1;
  const bHot = workloadIncidentCount(b) ? 0 : 1;
  return aHot - bHot || a.workload.namespace.localeCompare(b.workload.namespace) || a.workload.name.localeCompare(b.workload.name);
};

const workloadKey = (workload: DashboardWorkload) => `${workload.namespace}/${workload.kind}/${workload.name}`;
const workloadIncidentCount = (record: WorkloadRecord) => record.incidentCount ?? record.incidents.length;
const workloadRemediationCount = (record: WorkloadRecord) => record.remediationCount ?? record.remediations.length;
const workloadPredictionCount = (record: WorkloadRecord) => record.predictionCount ?? record.predictions.length;

const SummaryCard = ({ label, value, detail, icon, tone }: { label: string; value: string; detail: string; icon: ReactNode; tone: 'blue' | 'red' | 'green' }) => {
  const classes = tone === 'red' ? 'text-[#dc2626] bg-[#fef2f2]' : tone === 'green' ? 'text-[#15803d] bg-[#f0fdf4]' : 'text-[#2563eb] bg-[#eff6ff]';
  return (
    <div className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
      <div className={`inline-flex items-center gap-2 rounded-md px-2 py-1 text-[12px] font-medium ${classes}`}>{icon}{label}</div>
      <div className="mt-3 text-[28px] font-semibold leading-none text-[#111827]">{value}</div>
      <div className="mt-2 text-[12px] text-[#647084]">{detail}</div>
    </div>
  );
};

const DetailCard = ({ icon, title, children }: { icon: ReactNode; title: string; children: ReactNode }) => (
  <section className="overflow-hidden rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
    <header className="flex h-11 items-center gap-2 border-b border-[#e5e7eb] px-3 text-[13px] font-semibold text-[#111827]">{icon}{title}</header>
    {children}
  </section>
);

const GitOpsSummary = ({ gitops }: { gitops: DashboardGitOpsMapping }) => (
  <div className="space-y-2 p-3 text-[12px]">
    <div className="flex items-center gap-2">
      <SourceNode label={gitops.controller || 'GitOps'} detail={gitops.app || gitops.namespace || 'application'} />
      <span className="h-px flex-1 bg-[#cbd5e1]" />
      <SourceNode label={readableManifestType(gitops.manifestType)} detail={gitops.path || gitops.repo} />
    </div>
    <KeyValue label="Repository" value={gitops.repo || 'Unknown'} />
    <KeyValue label="Revision" value={gitops.revision || 'Unknown'} />
    <KeyValue label="Overlay" value={gitops.overlay || 'Not reported'} />
    {gitops.helm && (
      <>
        <KeyValue label="Chart" value={gitops.helm.chartVersion ? `${gitops.helm.chart}@${gitops.helm.chartVersion}` : gitops.helm.chart || 'Unknown'} />
        <KeyValue label="Values" value={(gitops.helm.valueFiles || []).join(' -> ') || 'Not reported'} />
      </>
    )}
  </div>
);

const SourceNode = ({ label, detail }: { label: string; detail: string }) => (
  <div className="min-w-0 rounded-md border border-[#e5e7eb] bg-[#f8fafc] px-2 py-2">
    <div className="truncate font-semibold text-[#111827]">{label}</div>
    <div className="truncate text-[11px] text-[#647084]">{detail}</div>
  </div>
);

const HealthPill = ({ record }: { record: WorkloadRecord }) => {
  const incidents = workloadIncidentCount(record);
  const critical = record.incidents.some((incident) => incident.severity === 'critical') || /critical|degraded/i.test(record.health || '');
  const label = incidents ? `${incidents} active` : record.health || 'healthy';
  const classes = incidents ? critical ? 'bg-[#fee2e2] text-[#dc2626]' : 'bg-[#ffedd5] text-[#ea580c]' : /healthy/i.test(record.health || '') ? 'bg-[#dcfce7] text-[#15803d]' : 'bg-[#f1f5f9] text-[#475569]';
  return <span className={`rounded-md px-2 py-1 text-[11px] font-medium ${classes}`}>{label}</span>;
};

const RemediationStatus = ({ record }: { record: WorkloadRecord }) => {
  if (!workloadRemediationCount(record)) return <span className="text-[#94a3b8]">No PR workflow</span>;
  const latest = record.remediations[0];
  if (!latest) return <span className="rounded-md bg-[#eff6ff] px-2 py-1 text-[11px] font-medium text-[#2563eb]">{workloadRemediationCount(record)} workflows</span>;
  return <span className="rounded-md bg-[#eff6ff] px-2 py-1 text-[11px] font-medium text-[#2563eb]">{latest.status}</span>;
};

const PredictionStatus = ({ record }: { record: WorkloadRecord }) => {
  if (!workloadPredictionCount(record)) return <span className="text-[#94a3b8]">No prediction</span>;
  return <span className="rounded-md bg-[#ffedd5] px-2 py-1 text-[11px] font-medium text-[#ea580c]">{record.predictions[0]?.risk || `${workloadPredictionCount(record)} signals`}</span>;
};

const EvidenceLine = ({ item }: { item: DashboardEvidence }) => (
  <div className="grid grid-cols-[96px_1fr] gap-2 px-3 py-2 text-[12px]">
    <span className="font-semibold text-[#111827]">{item.label}</span>
    <span className="line-clamp-2 text-[#475569]">{item.value}</span>
  </div>
);

const KeyValue = ({ label, value }: { label: string; value: string }) => (
  <div className="grid grid-cols-[96px_1fr] gap-2">
    <span className="font-semibold text-[#111827]">{label}</span>
    <span className="min-w-0 break-words text-[#475569]">{value || 'Unknown'}</span>
  </div>
);

const MetricBox = ({ label, value, tone }: { label: string; value: string; tone: 'red' | 'blue' | 'green' }) => {
  const text = tone === 'red' ? 'text-[#dc2626]' : tone === 'green' ? 'text-[#15803d]' : 'text-[#2563eb]';
  return (
    <div className="rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-2">
      <div className={`text-[18px] font-semibold ${text}`}>{value}</div>
      <div className="text-[11px] text-[#647084]">{label}</div>
    </div>
  );
};

const WorkloadIcon = ({ workload, large = false }: { workload: DashboardWorkload; large?: boolean }) => (
  <span className={`grid shrink-0 place-items-center rounded-full bg-[#2563eb] text-white ${large ? 'h-10 w-10' : 'h-7 w-7'}`}>
    {workloadIcon(workload.kind, large ? 'h-5 w-5' : 'h-4 w-4')}
  </span>
);

const workloadIcon = (kind: string, className: string) => {
  const lower = kind.toLowerCase();
  if (lower.includes('stateful')) return <Database className={className} />;
  if (lower.includes('daemon')) return <Server className={className} />;
  if (lower.includes('helm')) return <Boxes className={className} />;
  if (lower.includes('service')) return <Network className={className} />;
  if (lower.includes('ingress')) return <Layers3 className={className} />;
  if (lower.includes('pod')) return <Container className={className} />;
  if (lower.includes('prediction')) return <LineChart className={className} />;
  return <Activity className={className} />;
};

const EmptyState = ({ title, message }: { title: string; message: string }) => (
  <div className="grid min-h-[260px] place-items-center px-6 py-12 text-center">
    <div className="max-w-md">
      <div className="mx-auto grid h-12 w-12 place-items-center rounded-full bg-[#f1f5f9] text-[#94a3b8]">
        <AlertCircle className="h-6 w-6" />
      </div>
      <h2 className="mt-4 text-[15px] font-semibold text-[#111827]">{title}</h2>
      <p className="mt-1 text-[13px] leading-5 text-[#647084]">{message}</p>
    </div>
  </div>
);

const MiniEmpty = ({ message }: { message: string }) => <div className="p-3 text-[12px] leading-5 text-[#647084]">{message}</div>;

const matchesQuery = (query: string, values: Array<string | undefined>) => {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return true;
  return values.filter(Boolean).join(' ').toLowerCase().includes(normalized);
};

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
