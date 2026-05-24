import {
  Activity,
  AlertCircle,
  CheckCircle2,
  Database,
  ExternalLink,
  FileCode2,
  FileText,
  GitPullRequestArrow,
  LineChart,
  Settings,
  Edit2,
  Users,
  GitBranch,
  ShieldAlert,
} from 'lucide-react';
import type { ReactNode } from 'react';
import { useState } from 'react';
import { useStore } from '../store/useStore';
import { AuditDetailPanel } from '../components/AuditDetailPanel';
import { DiffEditorPanel } from '../components/DiffEditorPanel';
import { UserManagementPanel } from '../components/UserManagementPanel';
import type {
  DashboardAuditEvent,
  DashboardGitOpsSource,
  DashboardNodeCost,
  DashboardPrediction,
  DashboardRemediation,
  DashboardSettingsSection,
} from '../types';

type PageKind = 'remediations' | 'gitops' | 'predictions' | 'audit' | 'settings';

const pageMeta: Record<PageKind, { title: string; subtitle: string; icon: typeof Activity }> = {
  remediations: {
    title: 'Remediations',
    subtitle: 'Track generated fixes, approval state, opened pull requests, and closed-loop outcomes.',
    icon: GitPullRequestArrow,
  },
  gitops: {
    title: 'GitOps Sources',
    subtitle: 'Review the ArgoCD, Flux, repository, revision, path, and overlay mapping Fixora has discovered.',
    icon: FileCode2,
  },
  predictions: {
    title: 'Predictions',
    subtitle: 'Watch early risk signals such as memory growth trends before they become incidents.',
    icon: LineChart,
  },
  audit: {
    title: 'Audit',
    subtitle: 'Follow investigations, alerts, policy decisions, and remediation events in chronological order.',
    icon: FileText,
  },
  settings: {
    title: 'Settings',
    subtitle: 'See which providers, secrets, and guardrail integrations are configured without exposing sensitive values.',
    icon: Settings,
  },
};

export const DataPage = ({ kind }: { kind: PageKind }) => {
  const dashboard = useStore((state) => state.dashboard);
  const currentUser = useStore((state) => state.user);
  const searchQuery = useStore((state) => state.searchQuery);
  const timeRange = useStore((state) => state.timeRange);
  const meta = pageMeta[kind];
  const Icon = meta.icon;
  const [selectedAuditId, setSelectedAuditId] = useState<string | null>(null);
  const [editingRemediationId, setEditingRemediationId] = useState<number | null>(null);
  const [showUserManagement, setShowUserManagement] = useState(false);
  const hasClusterCost = Boolean((dashboard?.clusterCostMo || 0) > 0 || (dashboard?.activeNodes || 0) > 0);
  const filteredRemediations = filterRemediations(dashboard?.remediations || [], searchQuery, timeRange);
  const filteredGitOps = filterGitOpsSources(dashboard?.gitopsSources || [], searchQuery);
  const filteredPredictions = filterPredictions(dashboard?.predictions || [], searchQuery, timeRange);
  const filteredAuditEvents = filterAuditEvents(dashboard?.auditEvents || [], searchQuery, timeRange);
  const filteredSettings = filterSettings(dashboard?.settingsSections || [], searchQuery);

  const formatCurrency = (val: number | undefined) => {
    if (val === undefined || isNaN(val)) return '-';
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(val);
  };

  return (
    <div className="p-4">
      {kind === 'settings' && currentUser?.role === 'admin' && (
        <div className="mb-4">
          <button
            onClick={() => setShowUserManagement(true)}
            className="flex w-full items-center justify-between rounded-lg border border-[#e5e7eb] bg-white p-5 shadow-[0_1px_2px_rgba(15,23,42,0.04)] hover:bg-[#f8fafc] transition-colors"
          >
            <div className="flex items-center gap-3">
              <div className="rounded-lg bg-[#eff6ff] p-2 text-[#2563eb]">
                <Users className="h-5 w-5" />
              </div>
              <div className="text-left">
                <h3 className="text-[14px] font-semibold text-[#111827]">User & Access Management</h3>
                <p className="mt-1 text-[12px] text-[#647084]">Manage users, roles, and group memberships</p>
              </div>
            </div>
            <span className="text-sm font-medium text-[#2563eb]">Manage &rarr;</span>
          </button>
        </div>
      )}

      {kind === 'predictions' && (
        <div className="mb-4 grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(360px,0.8fr)]">
          <div className="rounded-lg border border-[#e5e7eb] bg-white p-5 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
            <h3 className="text-[13px] font-medium text-[#647084]">Total Cluster Compute Cost</h3>
            {hasClusterCost ? (
              <p className="mt-1 text-2xl font-semibold text-[#111827]">{formatCurrency(dashboard?.clusterCostMo)}<span className="text-sm text-[#647084] font-normal"> /mo</span></p>
            ) : (
              <p className="mt-2 text-[13px] text-[#647084]">Unavailable until Fixora can identify node pricing.</p>
            )}
          </div>
          <NodeCostList nodes={dashboard?.nodeCosts || []} activeNodes={dashboard?.activeNodes || 0} />
        </div>
      )}

      {kind === 'gitops' && <GitOpsTopology rows={filteredGitOps} />}

      {kind === 'audit' && <AuditRCAOverview rows={filteredAuditEvents} />}

      <section className="overflow-hidden rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
        <header className="flex items-center justify-between border-b border-[#e5e7eb] px-5 py-4">
          <div className="flex items-start gap-3">
            <span className="grid h-10 w-10 place-items-center rounded-lg bg-[#eff6ff] text-[#2563eb]">
              <Icon className="h-5 w-5" />
            </span>
            <div>
              <h1 className="text-[18px] font-semibold text-[#111827]">{meta.title}</h1>
              <p className="mt-1 text-[13px] text-[#647084]">{meta.subtitle}</p>
            </div>
          </div>
        </header>
        {kind === 'remediations' && <Remediations rows={filteredRemediations} onEdit={setEditingRemediationId} />}
        {kind === 'gitops' && <GitOpsSources rows={filteredGitOps} />}
        {kind === 'predictions' && <Predictions rows={filteredPredictions} />}
        {kind === 'audit' && <AuditEvents rows={filteredAuditEvents} onSelect={setSelectedAuditId} />}
        {kind === 'settings' && <SettingsSections rows={filteredSettings} />}
      </section>

      {selectedAuditId && (
        <AuditDetailPanel
          eventId={selectedAuditId}
          onClose={() => setSelectedAuditId(null)}
        />
      )}

      {editingRemediationId && (
        <DiffEditorPanel
          remediationId={editingRemediationId}
          onClose={() => setEditingRemediationId(null)}
        />
      )}

      {showUserManagement && (
        <UserManagementPanel onClose={() => setShowUserManagement(false)} />
      )}
    </div>
  );
};

const Remediations = ({ rows, onEdit }: { rows: DashboardRemediation[]; onEdit: (id: number) => void }) => {
  if (!rows.length) return <Empty title="No remediations recorded yet" message="Generated patches, pending approvals, PR links, and validation outcomes will appear here." />;
  const open = rows.filter((row) => /pending_approval|pr_opened|observing/.test(row.status)).length;
  const failed = rows.filter((row) => /failed/.test(row.status)).length;
  const succeeded = rows.filter((row) => /succeeded|reverted/.test(row.status)).length;
  return (
    <div className="space-y-4 p-4">
      <div className="grid gap-3 md:grid-cols-3">
        <RemediationStat icon={<GitBranch className="h-4 w-4" />} label="Active workflow" value={`${open}`} tone="text-[#2563eb]" />
        <RemediationStat icon={<CheckCircle2 className="h-4 w-4" />} label="Validated or reverted" value={`${succeeded}`} tone="text-[#15803d]" />
        <RemediationStat icon={<ShieldAlert className="h-4 w-4" />} label="Needs attention" value={`${failed}`} tone="text-[#dc2626]" />
      </div>
      <div className="grid gap-3 xl:grid-cols-2">
        {rows.map((row) => {
          const canEdit = row.prUrl && !remediationClosed(row.status);
          const statusNote = row.failureReason || remediationStatusNote(row.status);
          return (
            <article key={row.id} className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <Status value={row.status} />
                    <span className="text-[11px] text-[#647084]">{row.age}</span>
                  </div>
                  <h3 className="mt-3 truncate text-[14px] font-semibold text-[#111827]">{row.workload.kind}/{row.workload.name}</h3>
                  <p className="mt-1 truncate text-[12px] text-[#647084]">{row.repository || 'Repository not mapped'} · {row.strategy || 'strategy pending'}</p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {!row.prUrl && remediationNeedsPR(row.status) && (
                    <span className="rounded-md bg-[#f8fafc] px-2 py-1 text-[11px] font-medium text-[#647084]">
                      {row.status === 'dry_run' ? 'No PR: dry-run' : 'No PR link'}
                    </span>
                  )}
                  {canEdit && (
                    <button
                      onClick={() => onEdit(row.id)}
                      className="grid h-8 w-8 place-items-center rounded-md border border-[#bfdbfe] bg-[#eff6ff] text-[#2563eb]"
                      title="Edit remediation diff"
                    >
                      <Edit2 className="h-4 w-4" />
                    </button>
                  )}
                  {row.prUrl && <External href={row.prUrl} label="PR" />}
                </div>
              </div>
              <div className="mt-4 grid grid-cols-2 gap-3 border-t border-[#e5e7eb] pt-3 text-[12px]">
                <MiniKV label="Branch" value={row.headBranch || row.baseBranch || 'Pending'} />
                <MiniKV label="Namespace" value={row.workload.namespace} />
                <MiniKV label="Files" value={`${row.files?.length || 0}`} />
                <MiniKV label="GitOps" value={row.gitops?.manifestType || 'Unknown'} />
                {row.gitops?.helm && (
                  <>
                    <MiniKV label="Chart" value={helmChartLabel(row.gitops)} />
                    <MiniKV label="Values" value={row.gitops.helm.valueFiles?.[0] || 'HelmRelease values'} />
                  </>
                )}
              </div>
              {statusNote && (
                <p className={`mt-3 line-clamp-3 rounded-md px-3 py-2 text-[12px] ${remediationNoteClass(row.status, Boolean(row.failureReason))}`}>
                  {statusNote}
                </p>
              )}
            </article>
          );
        })}
      </div>
    </div>
  );
};

const RemediationStat = ({ icon, label, value, tone }: { icon: ReactNode; label: string; value: string; tone: string }) => (
  <div className="rounded-lg border border-[#e5e7eb] bg-[#f8fafc] p-3">
    <div className={`flex items-center gap-2 text-[12px] font-medium ${tone}`}>{icon}{label}</div>
    <div className="mt-2 text-2xl font-semibold text-[#111827]">{value}</div>
  </div>
);

const MiniKV = ({ label, value }: { label: string; value?: string }) => (
  <div className="min-w-0">
    <div className="text-[11px] font-semibold uppercase text-[#94a3b8]">{label}</div>
    <div className="truncate text-[#334155]">{value || 'Unknown'}</div>
  </div>
);

const GitOpsSources = ({ rows }: { rows: DashboardGitOpsSource[] }) => {
  if (!rows.length) return <Empty title="No GitOps sources mapped yet" message="ArgoCD and Flux source mappings will appear after Fixora correlates workloads to repositories." />;
  return (
    <Table headers={['Controller', 'App', 'Repository', 'Revision', 'Path', 'Type', 'Helm', 'Workloads']}>
      {rows.map((row) => (
        <tr key={row.id} className="border-t border-[#e5e7eb]">
          <td className="px-4 py-3 font-medium">{row.controller}</td>
          <td className="px-4 py-3">{row.app || 'Unknown'}</td>
          <td className="px-4 py-3">{row.repo}</td>
          <td className="px-4 py-3">{row.revision}</td>
          <td className="px-4 py-3">{row.path || 'Repository root'}</td>
          <td className="px-4 py-3">{readableManifestType(row.manifestType)}</td>
          <td className="px-4 py-3">
            {row.helm ? (
              <div className="max-w-[220px]">
                <div className="truncate font-medium text-[#111827]">{helmChartLabel(row)}</div>
                <div className="truncate text-[11px] text-[#647084]">{row.helm.valueFiles?.join(' → ') || row.helm.releaseName || 'values pending'}</div>
              </div>
            ) : (
              <span className="text-[#94a3b8]">Not Helm</span>
            )}
          </td>
          <td className="px-4 py-3">
            <GitOpsWorkloads source={row} />
          </td>
        </tr>
      ))}
    </Table>
  );
};

const GitOpsWorkloads = ({ source }: { source: DashboardGitOpsSource }) => {
  const [open, setOpen] = useState(false);
  const refs = source.workloadRefs || [];
  if (!refs.length) {
    return <span className="text-[#647084]">{source.workloads}</span>;
  }
  const visible = open ? refs : refs.slice(0, 3);
  return (
    <div className="min-w-[180px]">
      <button
        type="button"
        onClick={() => setOpen((current) => !current)}
        className="rounded-md bg-[#eff6ff] px-2 py-1 text-[11px] font-semibold text-[#2563eb] hover:bg-[#dbeafe]"
      >
        {source.workloads} workload{source.workloads === 1 ? '' : 's'}
      </button>
      <div className="mt-2 flex max-w-[320px] flex-wrap gap-1.5">
        {visible.map((workload) => (
          <span key={`${workload.namespace}/${workload.kind}/${workload.name}`} className="rounded-md border border-[#e5e7eb] bg-[#f8fafc] px-2 py-1 text-[11px] text-[#334155]">
            {workload.namespace ? `${workload.namespace}/` : ''}{workload.name}
          </span>
        ))}
        {!open && refs.length > visible.length && (
          <span className="rounded-md bg-[#f1f5f9] px-2 py-1 text-[11px] text-[#647084]">+{refs.length - visible.length} more</span>
        )}
      </div>
    </div>
  );
};

const Predictions = ({ rows }: { rows: DashboardPrediction[] }) => {
  if (!rows.length) return <Empty title="No predictions available yet" message="Predictive signals will populate after enough metrics have been stored." />;

  return (
    <Table headers={['Risk', 'Namespace', 'Pod', 'Growth Rate', 'Prevention Δ Cost', 'Downtime Risk', 'Last Alert']}>
      {rows.map((row) => (
        <tr key={row.id} className="border-t border-[#e5e7eb]">
          <td className="px-4 py-3"><Status value={row.risk} /></td>
          <td className="px-4 py-3">{row.namespace}</td>
          <td className="px-4 py-3 font-medium">{row.podName}</td>
          <td className="px-4 py-3">{Math.round(row.lastGrowthRate * 100)}%</td>
          <td className={`px-4 py-3 font-medium ${moneyDeltaClass(row.preventionCostMo)}`}>{formatCurrency(row.preventionCostMo)}<span className="text-[10px] text-[#647084] font-normal ml-1">/mo</span></td>
          <td className="px-4 py-3 text-[#dc2626] font-medium">{formatCurrency(row.downtimeRiskHr)}<span className="text-[10px] text-[#647084] font-normal ml-1">/hr</span></td>
          <td className="px-4 py-3">{row.lastAlertAge}</td>
        </tr>
      ))}
    </Table>
  );
};

const NodeCostList = ({ nodes, activeNodes }: { nodes: DashboardNodeCost[]; activeNodes: number }) => (
  <div className="rounded-lg border border-[#e5e7eb] bg-white p-5 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
    <div className="flex items-center justify-between">
      <h3 className="text-[13px] font-medium text-[#647084]">Node Cost Breakdown</h3>
      <span className="rounded-md bg-[#eff6ff] px-2 py-1 text-[11px] font-medium text-[#2563eb]">{activeNodes} nodes</span>
    </div>
    {nodes.length ? (
      <div className="mt-3 max-h-56 space-y-2 overflow-y-auto pr-1">
        {nodes.map((node) => (
          <div key={node.name} className="rounded-md border border-[#e5e7eb] px-3 py-2">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="truncate text-[13px] font-semibold text-[#111827]">{node.name}</div>
                <div className="mt-0.5 truncate text-[11px] text-[#647084]">{node.vendor} · {node.region}{node.zone ? `/${node.zone}` : ''} · {node.instanceType}</div>
                <div className="mt-1 text-[10px] text-[#647084]">
                  {node.pods || 0} pods · {formatCores(node.cpuRequestedCores || 0)} requested · {formatMemory(node.memoryRequestedMiB || 0)}
                </div>
              </div>
              <div className="text-right">
                <div className="text-[13px] font-semibold text-[#111827]">{node.monthlyCost > 0 ? formatCurrency(node.monthlyCost) : '-'}</div>
                {node.requestedMonthlyCost ? <div className="text-[10px] text-[#647084]">{formatCurrency(node.requestedMonthlyCost)} requested</div> : null}
                <div className={`text-[10px] font-medium ${node.status === 'priced' ? 'text-[#15803d]' : 'text-[#ea580c]'}`}>{node.status}</div>
              </div>
            </div>
            {node.pricingSource && <div className="mt-1 truncate text-[10px] text-[#94a3b8]">{node.pricingSource}</div>}
          </div>
        ))}
      </div>
    ) : (
      <p className="mt-2 text-[13px] text-[#647084]">Node pricing rows will appear after Fixora can list cluster nodes and resolve provider metadata.</p>
    )}
  </div>
);

const GitOpsTopology = ({ rows }: { rows: DashboardGitOpsSource[] }) => {
  if (!rows.length) return null;
  const helm = rows.filter((row) => /helm/i.test(row.manifestType) || row.helm).length;
  const kustomize = rows.filter((row) => /kustomize/i.test(row.manifestType)).length;
  const controllers = new Set(rows.map((row) => row.controller).filter(Boolean));
  return (
    <div className="mb-4 grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
      <section className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
        <div className="mb-4 flex items-center justify-between">
          <div>
            <h2 className="text-[15px] font-semibold text-[#111827]">GitOps Topology</h2>
            <p className="mt-1 text-[12px] text-[#647084]">Controller to repository to manifest source mapping, including Helm values and overlays when available.</p>
          </div>
          <span className="rounded-md bg-[#eff6ff] px-2 py-1 text-[11px] font-medium text-[#2563eb]">{rows.length} sources</span>
        </div>
        <div className="max-h-[300px] space-y-3 overflow-y-auto pr-1">
          {rows.map((row) => (
            <div key={row.id} className="grid gap-2 rounded-lg border border-[#e5e7eb] p-3 md:grid-cols-[180px_1fr_180px] md:items-center">
              <TopologyNode icon={<FileCode2 className="h-4 w-4" />} label={row.controller || 'GitOps'} detail={row.app || row.namespace || 'application'} tone="blue" />
              <div className="min-w-0 rounded-md border border-dashed border-[#cbd5e1] bg-[#f8fafc] px-3 py-2">
                <div className="truncate text-[12px] font-semibold text-[#111827]">{row.repo}</div>
                <div className="truncate text-[11px] text-[#647084]">{row.revision || 'revision unknown'} · {row.path || 'repository root'}</div>
              </div>
              <TopologyNode icon={<GitBranch className="h-4 w-4" />} label={readableManifestType(row.manifestType)} detail={row.overlay || row.helm?.valueFiles?.[0] || `${row.workloads} workloads`} tone={row.helm ? 'green' : 'orange'} />
            </div>
          ))}
        </div>
      </section>
      <section className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
        <h2 className="text-[15px] font-semibold text-[#111827]">Source Coverage</h2>
        <div className="mt-4 space-y-3">
          <CoverageRow label="Controllers" value={`${controllers.size}`} detail={Array.from(controllers).join(', ') || 'Unknown'} />
          <CoverageRow label="Helm-aware sources" value={`${helm}`} detail="chart, version, values files, or HelmRelease" />
          <CoverageRow label="Kustomize overlays" value={`${kustomize}`} detail="overlay and patch sources" />
          <CoverageRow label="Mapped workloads" value={`${rows.reduce((sum, row) => sum + (row.workloads || 0), 0)}`} detail="reported by dashboard snapshot" />
        </div>
      </section>
    </div>
  );
};

const TopologyNode = ({ icon, label, detail, tone }: { icon: ReactNode; label: string; detail: string; tone: 'blue' | 'green' | 'orange' }) => {
  const classes = tone === 'green' ? 'bg-[#f0fdf4] text-[#15803d]' : tone === 'orange' ? 'bg-[#fff7ed] text-[#ea580c]' : 'bg-[#eff6ff] text-[#2563eb]';
  return (
    <div className="min-w-0 rounded-md border border-[#e5e7eb] bg-white p-2">
      <div className={`mb-2 inline-flex items-center gap-1.5 rounded px-2 py-1 text-[11px] font-semibold ${classes}`}>{icon}{label || 'Unknown'}</div>
      <div className="truncate text-[11px] text-[#647084]">{detail || 'Not reported'}</div>
    </div>
  );
};

const CoverageRow = ({ label, value, detail }: { label: string; value: string; detail: string }) => (
  <div className="rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-3">
    <div className="flex items-center justify-between gap-3">
      <span className="text-[12px] font-semibold text-[#111827]">{label}</span>
      <span className="text-[18px] font-semibold text-[#2563eb]">{value}</span>
    </div>
    <div className="mt-1 truncate text-[11px] text-[#647084]">{detail}</div>
  </div>
);

const AuditRCAOverview = ({ rows }: { rows: DashboardAuditEvent[] }) => {
  if (!rows.length) return null;
  const investigations = rows.filter((row) => row.type === 'Investigation');
  const failures = rows.filter((row) => /fail|reject|block/i.test(`${row.status} ${row.detail}`));
  return (
    <div className="mb-4 grid gap-3 md:grid-cols-3">
      <RemediationStat icon={<Activity className="h-4 w-4" />} label="Investigations" value={`${investigations.length}`} tone="text-[#2563eb]" />
      <RemediationStat icon={<ShieldAlert className="h-4 w-4" />} label="Blocked or failed decisions" value={`${failures.length}`} tone="text-[#dc2626]" />
      <RemediationStat icon={<FileText className="h-4 w-4" />} label="Audit events in range" value={`${rows.length}`} tone="text-[#15803d]" />
    </div>
  );
};

const AuditEvents = ({ rows, onSelect }: { rows: DashboardAuditEvent[]; onSelect: (id: string) => void }) => {
  if (!rows.length) return <Empty title="No audit events recorded yet" message="Investigations, alerts, and remediation decisions will be logged here." />;
  return (
    <Table headers={['Type', 'Status', 'Subject', 'Detail', 'Time']}>
      {rows.map((row) => {
        const isInvestigation = row.type === 'Investigation';
        return (
          <tr
            key={row.id}
            onClick={() => isInvestigation && onSelect(row.id)}
            className={`border-t border-[#e5e7eb] ${isInvestigation ? 'cursor-pointer hover:bg-[#f8fafc]' : ''}`}
          >
            <td className="px-4 py-3 font-medium">{row.type}</td>
            <td className="px-4 py-3"><Status value={row.status} /></td>
            <td className="px-4 py-3">{row.subject}</td>
            <td className="px-4 py-3">{row.detail}</td>
            <td className="px-4 py-3">{formatTime(row.timestamp)}</td>
          </tr>
        );
      })}
    </Table>
  );
};

const SettingsSections = ({ rows }: { rows: DashboardSettingsSection[] }) => {
  if (!rows.length) return <Empty title="No settings metadata available" message="Provider and secret configuration status will appear after the backend reports it." />;
  return (
    <div className="grid grid-cols-1 gap-3 p-4 md:grid-cols-2 xl:grid-cols-3">
      {rows.map((row) => {
        const configured = row.status === 'configured';
        return (
          <div key={row.name} className="rounded-lg border border-[#e5e7eb] p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h3 className="text-[14px] font-semibold text-[#111827]">{row.name}</h3>
                <p className="mt-1 text-[12px] leading-5 text-[#647084]">{row.description}</p>
              </div>
              {configured ? <CheckCircle2 className="h-5 w-5 text-[#16a34a]" /> : <AlertCircle className="h-5 w-5 text-[#f97316]" />}
            </div>
            <div className="mt-4">
              <Status value={row.status} />
            </div>
          </div>
        );
      })}
    </div>
  );
};

const Table = ({ headers, children }: { headers: string[]; children: ReactNode }) => (
  <div className="overflow-x-auto">
    <table className="w-full min-w-[760px] text-left text-[13px]">
      <thead className="bg-[#f8fafc] text-[12px] text-[#111827]">
        <tr>{headers.map((header) => <th key={header} className="px-4 py-3 font-semibold">{header}</th>)}</tr>
      </thead>
      <tbody>{children}</tbody>
    </table>
  </div>
);

const Status = ({ value }: { value: string }) => {
  const lower = (value || '').toLowerCase();
  const good = /ok|success|succeeded|configured|low/.test(lower);
  const bad = /fail|error|high|missing|critical/.test(lower);
  const classes = good
    ? 'bg-[#dcfce7] text-[#15803d]'
    : bad
      ? 'bg-[#fee2e2] text-[#dc2626]'
      : 'bg-[#ffedd5] text-[#ea580c]';
  return <span className={`rounded-md px-2 py-1 text-[11px] font-medium ${classes}`}>{value || 'Pending'}</span>;
};

const External = ({ href, label }: { href: string; label: string }) => (
  <a href={href} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 font-medium text-[#2563eb]">
    {label}
    <ExternalLink className="h-3.5 w-3.5" />
  </a>
);

const Empty = ({ title, message }: { title: string; message: string }) => (
  <div className="grid min-h-[300px] place-items-center px-6 py-12 text-center">
    <div className="max-w-md">
      <div className="mx-auto grid h-12 w-12 place-items-center rounded-full bg-[#f1f5f9] text-[#94a3b8]">
        <Database className="h-6 w-6" />
      </div>
      <h2 className="mt-4 text-[15px] font-semibold text-[#111827]">{title}</h2>
      <p className="mt-1 text-[13px] leading-5 text-[#647084]">{message}</p>
    </div>
  </div>
);

const formatTime = (timestamp: string) => {
  if (!timestamp) return '';
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return timestamp;
  return date.toLocaleString();
};

const formatCurrency = (val: number | undefined) => {
  if (val === undefined || Number.isNaN(val)) return '-';
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(val);
};

const formatCores = (val: number) => `${val.toFixed(val >= 1 ? 1 : 2)} CPU`;

const formatMemory = (mib: number) => {
  if (mib >= 1024) return `${(mib / 1024).toFixed(1)} GiB`;
  return `${Math.round(mib)} MiB`;
};

const moneyDeltaClass = (val: number | undefined) => {
  if (val === undefined || Number.isNaN(val) || Math.abs(val) < 0.01) return 'text-[#647084]';
  return val > 0 ? 'text-[#ea580c]' : 'text-[#15803d]';
};

const filterRemediations = (rows: DashboardRemediation[], query: string, range: string) =>
  rows.filter((row) => withinAge(row.age, range) && matchesQuery(query, [
    row.status,
    row.title,
    row.repository,
    row.baseBranch,
    row.headBranch,
    row.strategy,
    row.failureReason,
    row.workload.kind,
    row.workload.name,
    row.workload.namespace,
    row.gitops?.app,
    row.gitops?.path,
    row.gitops?.manifestType,
    row.gitops?.helm?.chart,
    row.gitops?.helm?.chartVersion,
    row.gitops?.helm?.releaseName,
    row.gitops?.helm?.valueFiles?.join(' '),
  ]));

const filterGitOpsSources = (rows: DashboardGitOpsSource[], query: string) =>
  rows.filter((row) => matchesQuery(query, [row.controller, row.app, row.namespace, row.repo, row.revision, row.path, row.manifestType, row.overlay, row.helm?.chart, row.helm?.releaseName, row.helm?.valueFiles?.join(' ')]));

const filterPredictions = (rows: DashboardPrediction[], query: string, range: string) =>
  rows.filter((row) => withinAge(row.lastAlertAge, range) && matchesQuery(query, [row.risk, row.namespace, row.podName]));

const filterAuditEvents = (rows: DashboardAuditEvent[], query: string, range: string) =>
  rows.filter((row) => withinTimestamp(row.timestamp, range) && matchesQuery(query, [row.type, row.status, row.subject, row.detail]));

const filterSettings = (rows: DashboardSettingsSection[], query: string) =>
  rows.filter((row) => matchesQuery(query, [row.name, row.description, row.status]));

const matchesQuery = (query: string, values: Array<string | undefined>) => {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return true;
  return values.filter(Boolean).join(' ').toLowerCase().includes(normalized);
};

const withinAge = (age: string, range: string) => ageToMinutes(age) <= timeRangeToMinutes(range);

const withinTimestamp = (timestamp: string, range: string) => {
  if (!timestamp || range === 'All time') return true;
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return true;
  return (Date.now() - date.getTime()) / 60000 <= timeRangeToMinutes(range);
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

const remediationClosed = (status: string) => /merged|closed|succeeded|reverted|production_failed|revert_failed/.test(status || '');

const remediationNeedsPR = (status: string) => /dry_run|generated|pr_failed|pending_approval/.test(status || '');

const remediationStatusNote = (status: string) => {
  switch (status) {
    case 'dry_run':
      return 'Dry-run mode generated a remediation plan; no pull request was created. Enable auto-fix or click-to-fix to create PRs.';
    case 'generated':
      return 'Remediation changes were generated, but no pull request has been opened yet.';
    case 'pending_approval':
      return 'Waiting for approval before Fixora opens the remediation PR.';
    case 'pr_failed':
      return 'Fixora could not open the remediation PR. Check backend logs and VCS configuration.';
    default:
      return '';
  }
};

const remediationNoteClass = (status: string, hasFailure: boolean) => {
  if (hasFailure || /failed/.test(status || '')) return 'bg-[#fef2f2] text-[#991b1b]';
  if (status === 'dry_run') return 'bg-[#eff6ff] text-[#1d4ed8]';
  return 'bg-[#f8fafc] text-[#475569]';
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

const helmChartLabel = (source: { helm?: { chart?: string; chartVersion?: string } }) => {
  const chart = source.helm?.chart || 'Unknown chart';
  return source.helm?.chartVersion ? `${chart}@${source.helm.chartVersion}` : chart;
};
