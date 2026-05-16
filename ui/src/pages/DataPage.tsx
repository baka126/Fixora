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
} from 'lucide-react';
import type { ReactNode } from 'react';
import { useState } from 'react';
import { useStore } from '../store/useStore';
import { AuditDetailPanel } from '../components/AuditDetailPanel';
import type {
  DashboardAuditEvent,
  DashboardGitOpsSource,
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
  const meta = pageMeta[kind];
  const Icon = meta.icon;
  const [selectedAuditId, setSelectedAuditId] = useState<string | null>(null);

  return (
    <div className="p-4">
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
        {kind === 'remediations' && <Remediations rows={dashboard?.remediations || []} />}
        {kind === 'gitops' && <GitOpsSources rows={dashboard?.gitopsSources || []} />}
        {kind === 'predictions' && <Predictions rows={dashboard?.predictions || []} />}
        {kind === 'audit' && <AuditEvents rows={dashboard?.auditEvents || []} onSelect={setSelectedAuditId} />}
        {kind === 'settings' && <SettingsSections rows={dashboard?.settingsSections || []} />}
      </section>

      {selectedAuditId && (
        <AuditDetailPanel
          eventId={selectedAuditId}
          onClose={() => setSelectedAuditId(null)}
        />
      )}
    </div>
  );
};

const Remediations = ({ rows }: { rows: DashboardRemediation[] }) => {
  if (!rows.length) return <Empty title="No remediations recorded yet" message="Generated patches, pending approvals, PR links, and validation outcomes will appear here." />;
  return (
    <Table headers={['Status', 'Workload', 'Repository', 'Branch', 'Strategy', 'Age', 'PR']}>
      {rows.map((row) => (
        <tr key={row.id} className="border-t border-[#e5e7eb]">
          <td className="px-4 py-3"><Status value={row.status} /></td>
          <td className="px-4 py-3 font-medium">{row.workload.kind}/{row.workload.name}</td>
          <td className="px-4 py-3">{row.repository || 'Not mapped'}</td>
          <td className="px-4 py-3">{row.headBranch || row.baseBranch || 'Pending'}</td>
          <td className="px-4 py-3">{row.strategy || 'Pending'}</td>
          <td className="px-4 py-3">{row.age}</td>
          <td className="px-4 py-3">{row.prUrl ? <External href={row.prUrl} label="Open" /> : 'Not opened'}</td>
        </tr>
      ))}
    </Table>
  );
};

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
    <Table headers={['Risk', 'Namespace', 'Pod', 'Growth Rate', 'Last Alert']}>
      {rows.map((row) => (
        <tr key={row.id} className="border-t border-[#e5e7eb]">
          <td className="px-4 py-3"><Status value={row.risk} /></td>
          <td className="px-4 py-3">{row.namespace}</td>
          <td className="px-4 py-3 font-medium">{row.podName}</td>
          <td className="px-4 py-3">{Math.round(row.lastGrowthRate * 100)}%</td>
          <td className="px-4 py-3">{row.lastAlertAge}</td>
        </tr>
      ))}
    </Table>
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
