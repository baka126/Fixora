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
  const open = rows.filter((row) => /generated|pending|pr_opened|observing/.test(row.status)).length;
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
              </div>
              {row.failureReason && <p className="mt-3 line-clamp-2 rounded-md bg-[#fef2f2] px-3 py-2 text-[12px] text-[#991b1b]">{row.failureReason}</p>}
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
    <Table headers={['Controller', 'App', 'Repository', 'Revision', 'Path', 'Type', 'Workloads']}>
      {rows.map((row) => (
        <tr key={row.id} className="border-t border-[#e5e7eb]">
          <td className="px-4 py-3 font-medium">{row.controller}</td>
          <td className="px-4 py-3">{row.app || 'Unknown'}</td>
          <td className="px-4 py-3">{row.repo}</td>
          <td className="px-4 py-3">{row.revision}</td>
          <td className="px-4 py-3">{row.path || 'Repository root'}</td>
          <td className="px-4 py-3">{row.manifestType || 'Unknown'}</td>
          <td className="px-4 py-3">{row.workloads}</td>
        </tr>
      ))}
    </Table>
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
                <div className="mt-0.5 truncate text-[11px] text-[#647084]">{node.vendor} · {node.region} · {node.instanceType}</div>
              </div>
              <div className="text-right">
                <div className="text-[13px] font-semibold text-[#111827]">{node.monthlyCost > 0 ? formatCurrency(node.monthlyCost) : '-'}</div>
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
  ]));

const filterGitOpsSources = (rows: DashboardGitOpsSource[], query: string) =>
  rows.filter((row) => matchesQuery(query, [row.controller, row.app, row.namespace, row.repo, row.revision, row.path, row.manifestType, row.overlay]));

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
