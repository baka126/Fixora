import { useMemo } from 'react';
import type { ReactNode } from 'react';
import {
  AlertCircle,
  Calculator,
  CheckCircle2,
  CircleDollarSign,
  Cpu,
  Gauge,
  HardDrive,
  Server,
  ShieldAlert,
  TrendingDown,
} from 'lucide-react';
import { useStore } from '../store/useStore';
import type { DashboardNodeCost, DashboardWorkloadView } from '../types';

type CostRow = {
  workload: DashboardWorkloadView;
  requested: number;
  allocated: number;
  idleGap: number;
  efficiency: number | null;
};

export const FinOps = () => {
  const dashboard = useStore((state) => state.dashboard);

  const clusterCostMo = dashboard?.clusterCostMo || 0;
  const predictions = dashboard?.predictions || [];
  const nodeCosts = dashboard?.nodeCosts || [];
  const totalPreventionCostMo = predictions.reduce((acc, p) => acc + (p.preventionCostMo || 0), 0);
  const totalDowntimeRiskHr = predictions.reduce((acc, p) => acc + (p.downtimeRiskHr || 0), 0);
  const requestedClusterCostMo = nodeCosts.reduce((acc, node) => acc + (node.requestedMonthlyCost || 0), 0);
  const estimatedIdleCostMo = Math.max(clusterCostMo - requestedClusterCostMo, 0);
  const pricedNodes = nodeCosts.filter((node) => node.status === 'priced' && node.monthlyCost > 0).length;
  const pricingCoverage = nodeCosts.length ? pricedNodes / nodeCosts.length : 0;

  const workloadRows = useMemo(() => {
    const workloads = dashboard?.workloads || [];
    return workloads
      .map((workload): CostRow => {
        const requested = workload.cost?.requestedMonthlyCost || 0;
        const allocated = workload.cost?.allocatedMonthlyCost || workload.cost?.monthlyCost || 0;
        return {
          workload,
          requested,
          allocated,
          idleGap: Math.max(allocated - requested, 0),
          efficiency: allocated > 0 ? requested / allocated : null,
        };
      })
      .filter((row) => row.requested > 0 || row.allocated > 0)
      .sort((a, b) => b.idleGap - a.idleGap || b.requested - a.requested)
      .slice(0, 15);
  }, [dashboard?.workloads]);

  const unpricedNodes = nodeCosts.filter((node) => node.status !== 'priced' || node.monthlyCost <= 0);

  return (
    <div className="min-h-[calc(100vh-76px)] p-3 sm:p-4">
      <div className="grid grid-cols-1 items-start gap-4 2xl:grid-cols-[minmax(0,1fr)_420px]">
        <div className="min-w-0 space-y-4">
          <header className="rounded-lg border border-[#e5e7eb] bg-white p-5 shadow-sm">
            <div className="flex items-center gap-3">
              <div className="grid h-10 w-10 place-items-center rounded-lg bg-[#f0fdf4] text-[#16a34a]">
                <CircleDollarSign className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <h1 className="text-[20px] font-bold text-[#111827]">FinOps & Cost Analytics</h1>
                <p className="text-[13px] text-[#647084]">Track cluster spend, pricing coverage, resource request efficiency, and remediation financial impact.</p>
              </div>
            </div>
          </header>

          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
            <SummaryCard
              title="Projected Cluster Cost"
              value={formatCurrency(clusterCostMo)}
              subtitle="Monthly node compute estimate"
              icon={<Server className="h-4 w-4" />}
              tone="blue"
            />
            <SummaryCard
              title="Estimated Idle Capacity"
              value={formatCurrency(estimatedIdleCostMo)}
              subtitle="Node cost not covered by requests"
              icon={<TrendingDown className="h-4 w-4" />}
              tone="orange"
            />
            <SummaryCard
              title="Downtime Risk Prevented"
              value={formatCurrency(totalDowntimeRiskHr)}
              subtitle="Hourly risk value avoided"
              icon={<ShieldAlert className="h-4 w-4" />}
              tone="green"
            />
            <SummaryCard
              title="Pricing Coverage"
              value={nodeCosts.length ? `${Math.round(pricingCoverage * 100)}%` : 'n/a'}
              subtitle={`${pricedNodes}/${nodeCosts.length} nodes priced`}
              icon={<Calculator className="h-4 w-4" />}
              tone={pricingCoverage === 1 ? 'green' : 'orange'}
            />
          </div>

          {unpricedNodes.length > 0 && (
            <section className="rounded-lg border border-[#fed7aa] bg-[#fff7ed] p-4 text-[13px] text-[#9a3412]">
              <div className="flex items-start gap-2">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <div className="font-semibold">Some nodes could not be priced</div>
                  <p className="mt-1 leading-5">
                    {unpricedNodes.length} node{unpricedNodes.length === 1 ? '' : 's'} are missing cloud metadata or pricing. Cost totals may be incomplete until instance type, region, and provider are detected.
                  </p>
                </div>
              </div>
            </section>
          )}

          <section className="rounded-lg border border-[#e5e7eb] bg-white shadow-sm">
            <div className="border-b border-[#e5e7eb] px-4 py-3">
              <h2 className="text-[14px] font-semibold text-[#111827]">Top Cost Opportunities</h2>
              <p className="mt-0.5 text-[12px] text-[#647084]">Workloads ranked by allocated node share versus requested resource cost. Prometheus usage is required before opening a true right-sizing PR.</p>
            </div>
            {workloadRows.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[980px] text-left text-[12px]">
                  <thead className="bg-[#f8fafc] text-[#475569]">
                    <tr>
                      <th className="px-4 py-3 font-semibold">Workload</th>
                      <th className="px-4 py-3 font-semibold">Kind</th>
                      <th className="px-4 py-3 font-semibold">Requested / Mo</th>
                      <th className="px-4 py-3 font-semibold">Allocated Share / Mo</th>
                      <th className="px-4 py-3 font-semibold">Idle Gap</th>
                      <th className="px-4 py-3 font-semibold">Request Efficiency</th>
                      <th className="px-4 py-3 font-semibold">Pricing Source</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[#e5e7eb]">
                    {workloadRows.map((row) => (
                      <CostOpportunityRow key={row.workload.id} row={row} />
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <EmptyState message="No workload cost data is available yet. Fixora needs node pricing plus workload resource requests before this table can calculate cost opportunities." />
            )}
          </section>
        </div>

        <div className="min-w-0 space-y-4">
          <section className="rounded-lg border border-[#e5e7eb] bg-white shadow-sm">
            <div className="border-b border-[#e5e7eb] px-4 py-3">
              <h2 className="text-[14px] font-semibold text-[#111827]">Node Cost Breakdown</h2>
              <p className="mt-0.5 text-[12px] text-[#647084]">Monthly node price and resource requests scheduled on each node.</p>
            </div>
            <div className="max-h-[calc(100vh-255px)] divide-y divide-[#e5e7eb] overflow-y-auto">
              {nodeCosts.length > 0 ? nodeCosts.map((node) => (
                <NodeCostCard key={node.name} node={node} />
              )) : (
                <EmptyState message="No node cost data reported. Verify cluster access and pricing provider configuration." compact />
              )}
            </div>
          </section>

          <section className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-sm">
            <div className="mb-3 flex items-center gap-2 text-[14px] font-semibold text-[#111827]">
              <Gauge className="h-4 w-4 text-[#2563eb]" />
              Cost Model Notes
            </div>
            <div className="space-y-2 text-[12px] leading-5 text-[#647084]">
              <p><strong className="text-[#111827]">Requested cost</strong> is derived from Kubernetes resource requests and node pricing.</p>
              <p><strong className="text-[#111827]">Allocated share</strong> is the workload’s prorated share of priced node capacity. It is not real usage.</p>
              <p><strong className="text-[#111827]">Right-sizing PRs</strong> should use Prometheus usage percentiles before lowering requests.</p>
            </div>
          </section>

          <SummaryCard
            title="Prevention Cost"
            value={formatCurrency(totalPreventionCostMo)}
            subtitle="Cost of AI remediation/month"
            icon={<Calculator className="h-4 w-4" />}
            tone="orange"
          />
        </div>
      </div>
    </div>
  );
};

const CostOpportunityRow = ({ row }: { row: CostRow }) => {
  const { workload } = row;
  const efficiency = row.efficiency == null ? 0 : Math.min(Math.max(row.efficiency, 0), 1);
  const source = workload.cost?.pricingSource || 'Unknown';
  return (
    <tr className="hover:bg-[#f8fafc]">
      <td className="px-4 py-3">
        <div className="max-w-[260px]">
          <div className="truncate font-semibold text-[#111827]" title={`${workload.workload.namespace}/${workload.workload.name}`}>
            {workload.workload.namespace}/{workload.workload.name}
          </div>
          <div className="truncate text-[11px] text-[#647084]">
            {formatResourceRequests(workload)}
          </div>
        </div>
      </td>
      <td className="px-4 py-3 text-[#475569]">{workload.workload.kind}</td>
      <td className="px-4 py-3 font-semibold text-[#111827]">{formatCurrency(row.requested)}</td>
      <td className="px-4 py-3 font-semibold text-[#2563eb]">{formatCurrency(row.allocated)}</td>
      <td className="px-4 py-3">
        <span className={`rounded-md px-2 py-1 text-[11px] font-semibold ${row.idleGap > 0 ? 'bg-[#fff7ed] text-[#ea580c]' : 'bg-[#f0fdf4] text-[#15803d]'}`}>
          {formatCurrency(row.idleGap)}
        </span>
      </td>
      <td className="px-4 py-3">
        <div className="flex min-w-[130px] items-center gap-2">
          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-[#e5e7eb]">
            <div className="h-full rounded-full bg-[#16a34a]" style={{ width: `${Math.round(efficiency * 100)}%` }} />
          </div>
          <span className="w-9 text-right font-medium text-[#475569]">{row.efficiency == null ? 'n/a' : `${Math.round(efficiency * 100)}%`}</span>
        </div>
      </td>
      <td className="px-4 py-3 text-[#647084]">
        <span className="line-clamp-2 max-w-[280px]" title={source}>{source}</span>
      </td>
    </tr>
  );
};

const NodeCostCard = ({ node }: { node: DashboardNodeCost }) => {
  const requestRatio = node.monthlyCost > 0 ? Math.min((node.requestedMonthlyCost || 0) / node.monthlyCost, 1) : 0;
  const priced = node.status === 'priced' && node.monthlyCost > 0;
  return (
    <div className="p-4 text-[12px]">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate font-semibold text-[#111827]" title={node.name}>{node.name}</div>
          <div className="mt-1 flex flex-wrap gap-1.5">
            <StatusChip status={node.status} priced={priced} />
            {node.pods !== undefined && <span className="rounded-md bg-[#f1f5f9] px-2 py-0.5 text-[11px] text-[#475569]">{node.pods} pods</span>}
          </div>
        </div>
        <div className="text-right font-semibold text-[#2563eb]">{priced ? `${formatCurrency(node.monthlyCost)}/mo` : 'unpriced'}</div>
      </div>
      <div className="mt-3 grid grid-cols-2 gap-2 text-[#475569]">
        <IconValue icon={<Server className="h-3.5 w-3.5" />} label={node.instanceType || 'unknown'} />
        <IconValue icon={<Gauge className="h-3.5 w-3.5" />} label={node.region || 'unknown'} />
        <IconValue icon={<Cpu className="h-3.5 w-3.5" />} label={`${formatNumber(node.cpuRequestedCores)} cores requested`} />
        <IconValue icon={<HardDrive className="h-3.5 w-3.5" />} label={`${formatNumber((node.memoryRequestedMiB || 0) / 1024)} GiB requested`} />
      </div>
      {priced && (
        <div className="mt-3">
          <div className="mb-1 flex justify-between text-[11px] text-[#647084]">
            <span>Request cost coverage</span>
            <span>{formatCurrency(node.requestedMonthlyCost || 0)} / {formatCurrency(node.monthlyCost)}</span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-[#e5e7eb]">
            <div className="h-full rounded-full bg-[#2563eb]" style={{ width: `${Math.round(requestRatio * 100)}%` }} />
          </div>
        </div>
      )}
    </div>
  );
};

const SummaryCard = ({ title, value, subtitle, icon, tone }: { title: string; value: string; subtitle: string; icon: ReactNode; tone: 'blue' | 'green' | 'orange' }) => {
  const tones = {
    blue: 'bg-[#eff6ff] text-[#2563eb] border-[#bfdbfe]',
    green: 'bg-[#f0fdf4] text-[#16a34a] border-[#bbf7d0]',
    orange: 'bg-[#fff7ed] text-[#ea580c] border-[#fed7aa]',
  };

  return (
    <div className="rounded-lg border border-[#e5e7eb] bg-white p-4 shadow-sm">
      <div className="flex items-center gap-3">
        <div className={`grid h-10 w-10 place-items-center rounded-lg border ${tones[tone]}`}>
          {icon}
        </div>
        <div className="min-w-0">
          <div className="text-[12px] font-medium text-[#647084]">{title}</div>
          <div className="truncate text-[20px] font-bold text-[#111827]">{value}</div>
        </div>
      </div>
      <div className="mt-3 text-[11px] text-[#94a3b8]">{subtitle}</div>
    </div>
  );
};

const StatusChip = ({ status, priced }: { status: string; priced: boolean }) => (
  <span className={`inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[11px] font-medium ${priced ? 'bg-[#dcfce7] text-[#15803d]' : 'bg-[#f1f5f9] text-[#647084]'}`}>
    {priced ? <CheckCircle2 className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
    {status || 'unknown'}
  </span>
);

const IconValue = ({ icon, label }: { icon: ReactNode; label: string }) => (
  <div className="flex min-w-0 items-center gap-1.5">
    <span className="text-[#94a3b8]">{icon}</span>
    <span className="truncate" title={label}>{label}</span>
  </div>
);

const EmptyState = ({ message, compact = false }: { message: string; compact?: boolean }) => (
  <div className={`grid place-items-center px-6 text-center text-[13px] leading-5 text-[#647084] ${compact ? 'min-h-[160px] py-6' : 'min-h-[260px] py-8'}`}>
    <div className="max-w-md rounded-lg border border-dashed border-[#cbd5e1] bg-[#f8fafc] p-5">{message}</div>
  </div>
);

const formatCurrency = (value: number) => {
  if (!Number.isFinite(value) || value <= 0) return '$0.00';
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    maximumFractionDigits: value >= 100 ? 0 : 2,
  }).format(value);
};

const formatNumber = (value?: number) => {
  if (!Number.isFinite(value || 0)) return '0.00';
  return (value || 0).toFixed((value || 0) >= 10 ? 1 : 2);
};

const formatResourceRequests = (workload: DashboardWorkloadView) => {
  const cpu = workload.cost?.cpuRequestedCores;
  const memory = workload.cost?.memoryRequestedMiB;
  const parts = [];
  if (cpu) parts.push(`${formatNumber(cpu)} cores`);
  if (memory) parts.push(`${formatNumber(memory / 1024)} GiB`);
  return parts.length ? parts.join(' · ') : 'resource requests unavailable';
};
