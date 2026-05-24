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
  Loader2,
} from 'lucide-react';
import type { ReactNode } from 'react';
import { useState } from 'react';
import { apiClient } from '../api/client';
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
  DashboardState,
  DashboardTimelineStep,
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
  const setDashboard = useStore((state) => state.setDashboard);
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
        {kind === 'remediations' && <Remediations rows={filteredRemediations} onEdit={setEditingRemediationId} onDashboardRefresh={setDashboard} />}
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

const KANBAN_COLUMNS = [
  { id: 'review', label: 'Needs Review', match: /generated|dry_run|pending_approval/i },
  { id: 'pr_open', label: 'PR Open', match: /pr_opened/i },
  { id: 'deploying', label: 'Deploying & Observing', match: /awaiting_apply|observing/i },
  { id: 'closed', label: 'Closed', match: /succeeded|reverted|failed|dismissed/i },
];

const Remediations = ({ rows, onEdit, onDashboardRefresh }: { rows: DashboardRemediation[]; onEdit: (id: number) => void; onDashboardRefresh: (data: DashboardState | null) => void }) => {
  const [runningAction, setRunningAction] = useState<string | null>(null);
  const [actionError, setActionError] = useState('');
  if (!rows.length) return <Empty title="No remediations recorded yet" message="Generated patches, pending approvals, PR links, and validation outcomes will appear here." />;
  const open = rows.filter((row) => /pending_approval|pr_opened|awaiting_apply|observing/.test(row.status)).length;
  const failed = rows.filter((row) => /failed/.test(row.status)).length;
  const succeeded = rows.filter((row) => /succeeded|reverted/.test(row.status)).length;
  
  // Fallback for unmapped statuses
  const mappedIds = new Set(KANBAN_COLUMNS.flatMap(c => rows.filter(r => c.match.test(r.status)).map(r => r.id)));
  const unmappedRows = rows.filter(r => !mappedIds.has(r.id));

  return (
    <div className="space-y-4 p-4">
      {actionError && (
        <div className="rounded-md border border-[#fed7aa] bg-[#fff7ed] px-3 py-2 text-[12px] text-[#9a3412]">{actionError}</div>
      )}
      <div className="grid gap-3 md:grid-cols-3">
        <RemediationStat icon={<GitBranch className="h-4 w-4" />} label="Active workflow" value={`${open}`} tone="text-[#2563eb]" />
        <RemediationStat icon={<CheckCircle2 className="h-4 w-4" />} label="Validated or reverted" value={`${succeeded}`} tone="text-[#15803d]" />
        <RemediationStat icon={<ShieldAlert className="h-4 w-4" />} label="Needs attention" value={`${failed}`} tone="text-[#dc2626]" />
      </div>

      <div className="flex items-start gap-4 overflow-x-auto pb-4">
        {KANBAN_COLUMNS.map(col => {
          const colRows = rows.filter(r => col.match.test(r.status));
          if (col.id === 'review' && unmappedRows.length > 0) {
            colRows.push(...unmappedRows); // Append any unknowns to review
          }
          return (
            <div key={col.id} className="flex min-w-[340px] max-w-[380px] flex-1 shrink-0 flex-col gap-3 rounded-lg border border-[#e5e7eb] bg-[#f8fafc] p-3 shadow-inner">
              <div className="flex items-center justify-between px-1">
                <h3 className="text-[13px] font-semibold text-[#111827] uppercase">{col.label}</h3>
                <span className="rounded-full bg-[#e2e8f0] px-2 py-0.5 text-[11px] font-bold text-[#475569]">{colRows.length}</span>
              </div>
              <div className="flex flex-col gap-3">
                {colRows.map(row => (
                  <RemediationCard 
                    key={row.id} 
                    row={row} 
                    onEdit={onEdit} 
                    runningAction={runningAction} 
                    setRunningAction={setRunningAction} 
                    onDashboardRefresh={onDashboardRefresh}
                    setActionError={setActionError}
                  />
                ))}
                {colRows.length === 0 && (
                  <div className="rounded-md border border-dashed border-[#cbd5e1] p-6 text-center text-[12px] text-[#647084]">No items</div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

const RemediationCard = ({ 
  row, 
  onEdit, 
  runningAction, 
  setRunningAction, 
  onDashboardRefresh,
  setActionError
}: { 
  row: DashboardRemediation; 
  onEdit: (id: number) => void; 
  runningAction: string | null; 
  setRunningAction: (action: string | null) => void;
  onDashboardRefresh: (data: DashboardState | null) => void;
  setActionError: (err: string) => void;
}) => {
  const canEdit = row.prUrl && !remediationClosed(row.status);
  const statusNote = row.failureReason || remediationStatusNote(row.status);
  const actionLabel = remediationActionLabel(row);
  return (
    <article className="rounded-lg border border-[#e5e7eb] bg-white p-3 shadow-sm transition-all hover:border-[#bfdbfe] hover:shadow-md">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <Status value={row.status} />
            <span className="text-[10px] font-medium text-[#647084]">{row.age}</span>
          </div>
          <h3 className="mt-2 truncate text-[13px] font-semibold text-[#111827]" title={`${row.workload.kind}/${row.workload.name}`}>{row.workload.kind}/{row.workload.name}</h3>
          <p className="mt-0.5 truncate text-[11px] text-[#647084]" title={`${row.repository} · ${row.strategy}`}>{row.repository || 'Repository not mapped'} · {row.strategy || 'strategy pending'}</p>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {!row.prUrl && remediationNeedsPR(row.status) && (
            <span className="rounded bg-[#f8fafc] px-1.5 py-0.5 text-[10px] font-medium text-[#647084]">
              {row.status === 'dry_run' ? 'Dry-run' : 'No PR'}
            </span>
          )}
          {canEdit && (
            <button
              onClick={() => onEdit(row.id)}
              className="grid h-7 w-7 place-items-center rounded border border-[#bfdbfe] bg-[#eff6ff] text-[#2563eb]"
              title="Edit remediation diff"
            >
              <Edit2 className="h-3.5 w-3.5" />
            </button>
          )}
          {row.prUrl && <External href={row.prUrl} label="PR" />}
        </div>
      </div>
      <RemediationProgress status={row.status} />
      <div className="mt-3 grid grid-cols-2 gap-2 border-t border-[#f1f5f9] pt-2 text-[11px]">
        <MiniKV label="Branch" value={row.headBranch || row.baseBranch || 'Pending'} />
        <MiniKV label="Namespace" value={row.workload.namespace} />
        <MiniKV label="Files" value={`${row.files?.length || 0}`} />
        <MiniKV label="GitOps" value={row.gitops?.manifestType || 'Unknown'} />
      </div>
      {statusNote && (
        <p className={`mt-2 line-clamp-3 rounded-md px-2 py-1.5 text-[10px] leading-relaxed ${remediationNoteClass(row.status, Boolean(row.failureReason))}`} title={statusNote}>
          {statusNote}
        </p>
      )}
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        <span className={`rounded-md px-1.5 py-0.5 text-[10px] font-semibold ${remediationActionClass(row.status, Boolean(row.prUrl))}`}>{actionLabel}</span>
      </div>
      <div className="mt-2 text-[11px]">
        <RemediationTimeline row={row} />
      </div>
      <RemediationActions
        row={row}
        runningAction={runningAction}
        onRun={async (action) => {
          const key = `${row.id}:${action}`;
          setRunningAction(key);
          setActionError('');
          try {
            await apiClient.post(`/remediations/${row.id}/actions/${action}`);
            const { data } = await apiClient.get<DashboardState>('/dashboard');
            onDashboardRefresh(data);
          } catch (err) {
            const message = actionErrorMessage(err);
            setActionError(message);
          } finally {
            setRunningAction(null);
          }
        }}
      />
    </article>
  );
};

const RemediationStat = ({ icon, label, value, tone }: { icon: ReactNode; label: string; value: string; tone: string }) => (
  <div className="rounded-lg border border-[#e5e7eb] bg-[#f8fafc] p-3">
    <div className={`flex items-center gap-2 text-[12px] font-medium ${tone}`}>{icon}{label}</div>
    <div className="mt-2 text-2xl font-semibold text-[#111827]">{value}</div>
  </div>
);

const remediationSteps = [
  { id: 'generated', label: 'Generated' },
  { id: 'pr_opened', label: 'PR opened' },
  { id: 'awaiting_apply', label: 'Awaiting apply' },
  { id: 'observing', label: 'Observing' },
  { id: 'production_failed', label: 'Prod failed' },
  { id: 'revert_opened', label: 'Revert' },
  { id: 'succeeded', label: 'Done' },
];

const RemediationProgress = ({ status }: { status: string }) => {
  const activeIndex = remediationStepIndex(status);
  return (
    <div className="mt-4 rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-2">
      <div className="grid grid-cols-7 gap-1">
        {remediationSteps.map((step, index) => {
          const active = index <= activeIndex && activeIndex >= 0;
          const failed = /failed/.test(status) && step.id === 'production_failed';
          const revert = /revert/.test(status) && step.id === 'revert_opened';
          const done = /succeeded|reverted|dismissed/.test(status) && step.id === 'succeeded';
          const classes = failed
            ? 'bg-[#fee2e2] text-[#dc2626] border-[#fecaca]'
            : revert
              ? 'bg-[#ffedd5] text-[#ea580c] border-[#fed7aa]'
              : done
                ? 'bg-[#dcfce7] text-[#15803d] border-[#bbf7d0]'
                : active
              ? 'bg-[#eff6ff] text-[#2563eb] border-[#bfdbfe]'
              : 'bg-white text-[#94a3b8] border-[#e5e7eb]';
          return (
            <div key={step.id} className={`rounded border px-1.5 py-1 text-center text-[10px] font-semibold ${classes}`}>
              {step.label}
            </div>
          );
        })}
      </div>
    </div>
  );
};

const RemediationTimeline = ({ row }: { row: DashboardRemediation }) => {
  const steps = row.timeline?.length ? row.timeline : fallbackRemediationTimeline(row.status);
  return (
    <div className="mt-4 rounded-md border border-[#e5e7eb] bg-white p-3">
      <div className="mb-2 text-[11px] font-semibold uppercase text-[#647084]">Timeline</div>
      <div className="space-y-2">
        {steps.map((step) => (
          <div key={step.id} className="grid grid-cols-[18px_1fr] gap-2 text-[12px]">
            <span className={`mt-1 h-2.5 w-2.5 rounded-full ${timelineDotClass(step.status)}`} />
            <span className="min-w-0">
              <span className="flex flex-wrap items-center gap-2">
                <span className="font-semibold text-[#111827]">{step.label}</span>
                {step.current && <span className="rounded bg-[#eff6ff] px-1.5 py-0.5 text-[10px] font-semibold text-[#2563eb]">current</span>}
                {step.age && <span className="text-[11px] text-[#94a3b8]">{step.age}</span>}
                {step.url && <External href={step.url} label="Link" />}
              </span>
              {step.detail && <span className="mt-0.5 block text-[#647084]">{step.detail}</span>}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};

export const RemediationActions = ({
  row,
  runningAction,
  onRun,
}: {
  row: DashboardRemediation;
  runningAction: string | null;
  onRun: (action: string) => Promise<void>;
}) => {
  const actions = remediationManualActions(row);
  if (!actions.length && !row.prUrl && !row.revertPrUrl) return null;
  return (
    <div className="mt-4 flex flex-wrap items-center gap-2 border-t border-[#e5e7eb] pt-3">
      {row.prUrl && <External href={row.prUrl} label="Open PR" />}
      {row.revertPrUrl && <External href={row.revertPrUrl} label="Open revert PR" />}
      {actions.map((action) => {
        const key = `${row.id}:${action.id}`;
        const running = runningAction === key;
        return (
          <button
            key={action.id}
            type="button"
            onClick={() => onRun(action.id)}
            disabled={!!runningAction}
            className={`inline-flex h-8 items-center gap-1.5 rounded-md border px-2.5 text-[11px] font-semibold disabled:opacity-60 ${action.className}`}
            title={action.title}
          >
            {running && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
            {action.label}
          </button>
        );
      })}
    </div>
  );
};

export const remediationManualActions = (row: DashboardRemediation) => {
  const status = (row.status || '').toLowerCase();
  const actions: Array<{ id: string; label: string; title: string; className: string }> = [];
  if (status === 'awaiting_apply') {
    actions.push({ id: 'mark-applied', label: 'Mark applied', title: 'Confirm the PR changes are live in the cluster.', className: 'border-[#bbf7d0] bg-[#f0fdf4] text-[#15803d]' });
    actions.push({ id: 'rerun-validation', label: 'Re-run validation', title: 'Check whether the merged fix is applied and healthy now.', className: 'border-[#bfdbfe] bg-[#eff6ff] text-[#2563eb]' });
  }
  if (status === 'observing') {
    actions.push({ id: 'rerun-validation', label: 'Re-run validation', title: 'Run post-apply health validation immediately.', className: 'border-[#bfdbfe] bg-[#eff6ff] text-[#2563eb]' });
  }
  if (status === 'production_failed' || status === 'revert_failed') {
    actions.push({ id: 'open-revert', label: 'Open revert PR', title: 'Open a rollback PR from the stored pre-remediation file contents.', className: 'border-[#fecaca] bg-[#fef2f2] text-[#dc2626]' });
  }
  if (!/succeeded|reverted|dismissed/.test(status)) {
    actions.push({ id: 'dismiss', label: 'Dismiss', title: 'Close this remediation workflow without further automated action.', className: 'border-[#e5e7eb] bg-white text-[#475569]' });
  }
  return actions;
};

const remediationStepIndex = (status: string) => {
  const normalized = (status || '').toLowerCase();
  if (/revert/.test(normalized)) return remediationSteps.findIndex((step) => step.id === 'revert_opened');
  if (/production_failed|failed/.test(normalized)) return remediationSteps.findIndex((step) => step.id === 'production_failed');
  if (/succeeded|reverted|dismissed/.test(normalized)) return remediationSteps.findIndex((step) => step.id === 'succeeded');
  const index = remediationSteps.findIndex((step) => normalized.includes(step.id));
  if (index >= 0) return index;
  if (/pending/.test(normalized)) return 1;
  if (/opened/.test(normalized)) return 2;
  return 0;
};

const timelineDotClass = (status: string) => {
  if (/failed/.test(status || '')) return 'bg-[#ef4444]';
  if (/revert/.test(status || '')) return 'bg-[#f97316]';
  if (/dismissed/.test(status || '')) return 'bg-[#94a3b8]';
  if (/completed/.test(status || '')) return 'bg-[#16a34a]';
  if (/active/.test(status || '')) return 'bg-[#2563eb]';
  return 'bg-[#cbd5e1]';
};

const fallbackRemediationTimeline = (status: string): DashboardTimelineStep[] => {
  const labels = ['Generated', 'PR opened', 'Merged', 'Applied', 'Observed', 'Result'];
  const current = remediationStepIndex(status);
  return labels.map((label, index) => ({
    id: label.toLowerCase().replace(/\s+/g, '-'),
    label,
    status: index < current ? 'completed' : index === current ? remediationTimelineStatus(status) : 'pending',
    current: index === current,
  }));
};

const remediationTimelineStatus = (status: string) => {
  if (/production_failed|pr_failed|revert_failed/.test(status || '')) return 'failed';
  if (/revert_opened/.test(status || '')) return 'revert';
  if (/dismissed/.test(status || '')) return 'dismissed';
  if (/succeeded|reverted/.test(status || '')) return 'completed';
  return 'active';
};

const actionErrorMessage = (err: unknown) => {
  if (typeof err === 'object' && err && 'response' in err) {
    const response = (err as { response?: { data?: unknown } }).response;
    if (typeof response?.data === 'string' && response.data.trim()) return response.data.trim();
  }
  return 'Remediation action failed.';
};

const MiniKV = ({ label, value }: { label: string; value?: string }) => (
  <div className="min-w-0">
    <div className="text-[11px] font-semibold uppercase text-[#94a3b8]">{label}</div>
    <div className="truncate text-[#334155]">{value || 'Unknown'}</div>
  </div>
);

const GitOpsSources = ({ rows }: { rows: DashboardGitOpsSource[] }) => {
  if (!rows.length) return <Empty title="No GitOps sources mapped yet" message="ArgoCD and Flux source mappings will appear after Fixora correlates workloads to repositories." />;
  return (
    <div className="grid gap-4 p-4 xl:grid-cols-2">
      {rows.map((row) => (
        <article key={row.id} className="overflow-hidden rounded-lg border border-[#e5e7eb] bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
          <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[#e5e7eb] bg-[#f8fafc] p-4">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="rounded-md bg-[#eff6ff] px-2 py-1 text-[11px] font-semibold text-[#2563eb]">{row.controller || 'GitOps'}</span>
                <Status value={row.healthStatus || row.driftStatus || 'unknown'} />
                {row.syncStatus && <Status value={row.syncStatus} />}
              </div>
              <h3 className="mt-3 truncate text-[15px] font-semibold text-[#111827]">{row.app || row.helm?.releaseName || row.path || 'Unknown application'}</h3>
              <p className="mt-1 truncate text-[12px] text-[#647084]">{row.controllerNamespace || row.namespace || 'controller namespace unknown'} · {readableManifestType(row.manifestType)}</p>
            </div>
            <GitOpsWorkloads source={row} />
          </div>

          <div className="grid gap-3 p-4 text-[12px] md:grid-cols-2">
            <MiniKV label="Repository" value={row.repo || 'Unknown'} />
            <MiniKV label="Path" value={row.path || 'Repository root'} />
            <MiniKV label="Target revision" value={row.revision || 'Unknown'} />
            <MiniKV label="Live revision" value={row.reconciledRevision || 'Not reported'} />
            <MiniKV label="Sync" value={row.syncStatus || 'Not reported'} />
            <MiniKV label="Operation" value={row.operationStatus || 'Not reported'} />
            <MiniKV label="Drift" value={row.driftStatus || 'Unknown'} />
            <MiniKV label="Last sync" value={row.lastSyncAt ? formatTime(row.lastSyncAt) : 'Not reported'} />
          </div>

          {row.helm ? (
            <div className="border-t border-[#e5e7eb] p-4">
              <div className="mb-3 flex flex-wrap items-center gap-2">
                <span className="rounded-md bg-[#f0fdf4] px-2 py-1 text-[11px] font-semibold text-[#15803d]">Helm release</span>
                <span className="text-[12px] font-semibold text-[#111827]">{row.helm.releaseName || row.app || 'Unknown release'}</span>
                <span className="text-[12px] text-[#647084]">{helmChartLabel(row)}</span>
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                <OrderedMiniList title="Values files" items={row.helm.valueFiles || []} empty="No values files reported" />
                <OrderedMiniList title="Values from" items={row.helm.valuesFrom || []} empty="No valuesFrom refs reported" />
              </div>
              <div className="mt-3">
                <div className="mb-2 text-[11px] font-semibold uppercase text-[#647084]">Affected workloads under release</div>
                <GitOpsWorkloadList source={row} expanded />
              </div>
            </div>
          ) : (
            <div className="border-t border-[#e5e7eb] p-4">
              <div className="mb-2 text-[11px] font-semibold uppercase text-[#647084]">Affected workloads</div>
              <GitOpsWorkloadList source={row} expanded />
            </div>
          )}
        </article>
      ))}
    </div>
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
      <GitOpsWorkloadList source={{ ...source, workloadRefs: visible }} hidden={!open ? refs.length - visible.length : 0} />
    </div>
  );
};

const GitOpsWorkloadList = ({ source, expanded = false, hidden = 0 }: { source: DashboardGitOpsSource; expanded?: boolean; hidden?: number }) => {
  const refs = source.workloadRefs || [];
  if (!refs.length) return <span className="text-[12px] text-[#647084]">No workload refs captured yet.</span>;
  return (
    <div className={`flex flex-wrap gap-1.5 ${expanded ? 'max-h-36 overflow-y-auto rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-2' : 'mt-2 max-w-[320px]'}`}>
      {refs.map((workload) => (
        <span key={`${workload.namespace}/${workload.kind}/${workload.name}`} className="rounded-md border border-[#e5e7eb] bg-white px-2 py-1 text-[11px] text-[#334155]">
          {workload.namespace ? `${workload.namespace}/` : ''}{workload.kind}/{workload.name}
        </span>
      ))}
      {hidden > 0 && (
        <span className="rounded-md bg-[#f1f5f9] px-2 py-1 text-[11px] text-[#647084]">+{hidden} more</span>
      )}
    </div>
  );
};

const OrderedMiniList = ({ title, items, empty }: { title: string; items: string[]; empty: string }) => (
  <div className="rounded-md border border-[#e5e7eb] bg-[#f8fafc] p-3">
    <div className="mb-2 text-[11px] font-semibold uppercase text-[#647084]">{title}</div>
    {items.length ? (
      <div className="space-y-1">
        {items.map((item, index) => (
          <div key={`${title}-${item}-${index}`} className="flex items-center gap-2 text-[12px] text-[#334155]">
            <span className="grid h-4 w-4 shrink-0 place-items-center rounded-full bg-white text-[10px] font-semibold text-[#2563eb]">{index + 1}</span>
            <span className="min-w-0 truncate">{item}</span>
          </div>
        ))}
      </div>
    ) : (
      <div className="text-[12px] text-[#94a3b8]">{empty}</div>
    )}
  </div>
);

const Predictions = ({ rows }: { rows: DashboardPrediction[] }) => {
  if (!rows.length) return <Empty title="No predictions available yet" message="Predictive signals will populate after enough metrics have been stored." />;
  const savings = rows.reduce((sum, row) => sum + Math.abs(Math.min(row.preventionCostMo || 0, 0)), 0);
  const highRisk = rows.filter((row) => /high|critical|oom|risk/i.test(row.risk)).length;

  return (
    <div>
      <div className="grid gap-3 p-4 md:grid-cols-3">
        <RemediationStat icon={<LineChart className="h-4 w-4" />} label="Predictive signals" value={`${rows.length}`} tone="text-[#2563eb]" />
        <RemediationStat icon={<ShieldAlert className="h-4 w-4" />} label="High-risk signals" value={`${highRisk}`} tone="text-[#dc2626]" />
        <RemediationStat icon={<Database className="h-4 w-4" />} label="Potential savings" value={formatCurrency(savings)} tone="text-[#15803d]" />
      </div>
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
    </div>
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

import { CircleDashed, CircleDot, AlertTriangle, ArrowRight } from 'lucide-react';

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
            <h2 className="text-[15px] font-semibold text-[#111827]">GitOps Pipeline Health</h2>
            <p className="mt-1 text-[12px] text-[#647084]">Visualizing the drift and sync state from Git Commit through ArgoCD/Flux down to live Kubernetes Workloads.</p>
          </div>
          <span className="rounded-md bg-[#eff6ff] px-2 py-1 text-[11px] font-medium text-[#2563eb]">{rows.length} pipelines</span>
        </div>
        <div className="max-h-[500px] space-y-4 overflow-y-auto pr-1">
          {rows.map((row) => (
            <GitOpsPipelineRow key={row.id} row={row} />
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

const GitOpsPipelineRow = ({ row }: { row: DashboardGitOpsSource }) => {
  // Determine Sync State
  const isSynced = (row.syncStatus || '').toLowerCase() === 'synced' || row.revision === row.reconciledRevision;
  const isSyncing = (row.syncStatus || '').toLowerCase() === 'syncing' || (row.operationStatus && row.operationStatus !== 'Succeeded');
  const syncTone = isSynced ? 'text-[#15803d]' : isSyncing ? 'text-[#2563eb]' : 'text-[#ea580c]';
  const SyncIcon = isSynced ? CheckCircle2 : isSyncing ? CircleDashed : CircleDot;

  // Determine Drift/Health State
  const isHealthy = (row.healthStatus || '').toLowerCase() === 'healthy' && (row.driftStatus || '').toLowerCase() !== 'drifted';
  const isDrifted = (row.driftStatus || '').toLowerCase() === 'drifted' || (row.healthStatus || '').toLowerCase() === 'degraded';
  const healthTone = isHealthy ? 'text-[#15803d]' : isDrifted ? 'text-[#dc2626]' : 'text-[#94a3b8]';
  const HealthIcon = isHealthy ? CheckCircle2 : isDrifted ? AlertTriangle : CircleDashed;

  return (
    <div className="rounded-lg border border-[#e5e7eb] bg-[#fbfdff] p-4">
      <div className="mb-3 flex items-center justify-between">
        <div className="font-semibold text-[#111827]">{row.app || row.namespace || 'Application'}</div>
        <div className="flex gap-2">
           <span className="rounded bg-[#eff6ff] px-2 py-0.5 text-[10px] font-semibold text-[#2563eb] uppercase">{row.controller || 'GitOps'}</span>
           <span className="rounded bg-[#f8fafc] border border-[#e5e7eb] px-2 py-0.5 text-[10px] font-semibold text-[#647084] uppercase">{readableManifestType(row.manifestType)}</span>
        </div>
      </div>
      
      <div className="flex flex-col md:flex-row md:items-center gap-2">
        {/* Step 1: Git Source */}
        <div className="flex-1 min-w-0 rounded-md border border-[#e5e7eb] bg-white p-3 shadow-sm">
          <div className="flex items-center gap-2 mb-2">
            <GitBranch className="h-4 w-4 text-[#647084]" />
            <span className="text-[12px] font-semibold text-[#374151]">Source Git Commit</span>
          </div>
          <div className="truncate text-[11px] font-mono text-[#2563eb]" title={row.repo}>{row.repo}</div>
          <div className="mt-1 flex justify-between text-[10px] text-[#647084]">
            <span className="truncate">{row.path || '/'}</span>
            <span className="font-mono">{row.revision?.slice(0, 7) || 'HEAD'}</span>
          </div>
        </div>

        <ArrowRight className="hidden md:block h-5 w-5 text-[#cbd5e1] shrink-0" />

        {/* Step 2: Controller Sync */}
        <div className="flex-1 min-w-0 rounded-md border border-[#e5e7eb] bg-white p-3 shadow-sm">
          <div className="flex items-center gap-2 mb-2">
            <SyncIcon className={`h-4 w-4 ${syncTone}`} />
            <span className="text-[12px] font-semibold text-[#374151]">Controller Sync</span>
          </div>
          <div className="truncate text-[12px] text-[#475569]">Target: {row.reconciledRevision?.slice(0, 7) || row.revision?.slice(0, 7) || 'Unknown'}</div>
          <div className={`mt-1 text-[10px] font-semibold uppercase ${syncTone}`}>
            {row.syncStatus || (isSynced ? 'Synced' : 'Pending')}
          </div>
        </div>

        <ArrowRight className="hidden md:block h-5 w-5 text-[#cbd5e1] shrink-0" />

        {/* Step 3: Live Workload Health */}
        <div className="flex-1 min-w-0 rounded-md border border-[#e5e7eb] bg-white p-3 shadow-sm">
          <div className="flex items-center gap-2 mb-2">
            <HealthIcon className={`h-4 w-4 ${healthTone}`} />
            <span className="text-[12px] font-semibold text-[#374151]">Live K8s State</span>
          </div>
          <div className="truncate text-[12px] text-[#475569]">{row.workloads || 0} Workloads Managed</div>
          <div className="mt-1 flex gap-2 text-[10px] font-semibold uppercase">
             <span className={`${healthTone}`}>{row.healthStatus || (isHealthy ? 'Healthy' : 'Unknown')}</span>
             {row.driftStatus && <span className={row.driftStatus.toLowerCase() === 'drifted' ? 'text-[#dc2626]' : 'text-[#647084]'}>({row.driftStatus})</span>}
          </div>
        </div>
      </div>
      
      {row.lastSyncAt && (
         <div className="mt-3 text-right text-[10px] text-[#94a3b8]">Last attempt: {new Date(row.lastSyncAt).toLocaleString()}</div>
      )}
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
            <td className="px-4 py-2.5 font-medium">{row.type}</td>
            <td className="px-4 py-2.5"><Status value={row.status} /></td>
            <td className="max-w-[240px] px-4 py-2.5"><div className="truncate" title={row.subject}>{row.subject}</div></td>
            <td className="max-w-[520px] px-4 py-2.5"><div className="line-clamp-2" title={row.detail}>{row.detail}</div></td>
            <td className="px-4 py-2.5">{formatTime(row.timestamp)}</td>
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
    <table className="w-full min-w-[760px] text-left text-[12px]">
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
  const neutral = /pending|unknown|not|dry/.test(lower);
  const classes = good
    ? 'border-[#bbf7d0] bg-[#dcfce7] text-[#15803d]'
    : bad
      ? 'border-[#fecaca] bg-[#fee2e2] text-[#dc2626]'
      : neutral
        ? 'border-[#e5e7eb] bg-[#f1f5f9] text-[#475569]'
        : 'border-[#fed7aa] bg-[#ffedd5] text-[#ea580c]';
  return <span className={`inline-flex max-w-[170px] truncate rounded-md border px-2 py-1 text-[11px] font-semibold ${classes}`} title={value || 'Pending'}>{value || 'Pending'}</span>;
};

const External = ({ href, label }: { href: string; label: string }) => (
  <a href={href} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 font-medium text-[#2563eb]">
    {label}
    <ExternalLink className="h-3.5 w-3.5" />
  </a>
);

const Empty = ({ title, message }: { title: string; message: string }) => (
  <div className="grid min-h-[300px] place-items-center px-6 py-12 text-center">
    <div className="max-w-md rounded-lg border border-dashed border-[#cbd5e1] bg-[#f8fafc] px-6 py-6">
      <div className="mx-auto grid h-12 w-12 place-items-center rounded-lg bg-white text-[#94a3b8] shadow-sm">
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
  rows.filter((row) => matchesQuery(query, [
    row.controller,
    row.app,
    row.namespace,
    row.repo,
    row.revision,
    row.path,
    row.manifestType,
    row.overlay,
    row.healthStatus,
    row.syncStatus,
    row.operationStatus,
    row.driftStatus,
    row.reconciledRevision,
    row.controllerNamespace,
    row.helm?.chart,
    row.helm?.releaseName,
    row.helm?.valueFiles?.join(' '),
    ...(row.workloadRefs || []).map((workload) => `${workload.namespace} ${workload.kind} ${workload.name}`),
  ]));

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

const remediationActionLabel = (row: DashboardRemediation) => {
  if (row.prUrl) {
    if (row.status === 'awaiting_apply') return 'Merged, waiting for rollout';
    if (row.status === 'observing') return 'Live validation running';
    if (/succeeded|reverted/.test(row.status)) return 'Validated';
    if (/failed/.test(row.status)) return 'Needs operator review';
    return 'Open pull request';
  }
  if (row.status === 'dry_run') return 'Dry-run only';
  if (row.status === 'pending_approval') return 'Approval required';
  if (row.status === 'pr_failed') return 'PR creation failed';
  return 'PR not opened yet';
};

const remediationActionClass = (status: string, hasPR: boolean) => {
  if (/failed/.test(status || '')) return 'bg-[#fee2e2] text-[#dc2626]';
  if (/succeeded|reverted/.test(status || '')) return 'bg-[#dcfce7] text-[#15803d]';
  if (hasPR) return 'bg-[#eff6ff] text-[#2563eb]';
  if (/dry_run|pending_approval/.test(status || '')) return 'bg-[#ffedd5] text-[#ea580c]';
  return 'bg-[#f1f5f9] text-[#647084]';
};

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
    case 'awaiting_apply':
      return 'The PR was merged. Fixora is waiting until ArgoCD, Flux, Helm, kubectl, or another deploy path applies the suggested changes before it starts health validation or offers a revert.';
    case 'observing':
      return 'The suggested changes are live in the cluster. Fixora is observing workload health before closing the remediation.';
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
