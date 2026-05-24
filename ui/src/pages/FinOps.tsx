import { useMemo } from 'react';
import {
  Calculator,
  CircleDollarSign,
  Server,
  ShieldAlert,
} from 'lucide-react';
import { useStore } from '../store/useStore';

export const FinOps = () => {
  const dashboard = useStore((state) => state.dashboard);

  const clusterCostMo = dashboard?.clusterCostMo || 0;
  
  const predictions = dashboard?.predictions || [];
  const totalPreventionCostMo = predictions.reduce((acc, p) => acc + (p.preventionCostMo || 0), 0);
  const totalDowntimeRiskHr = predictions.reduce((acc, p) => acc + (p.downtimeRiskHr || 0), 0);

  const nodeCosts = dashboard?.nodeCosts || [];
  const workloads = dashboard?.workloads || [];

  const workloadsWithCost = useMemo(() => {
    return workloads
      .filter(w => w.cost?.requestedMonthlyCost || w.cost?.monthlyCost)
      .sort((a, b) => (b.cost?.requestedMonthlyCost || 0) - (a.cost?.requestedMonthlyCost || 0))
      .slice(0, 15);
  }, [workloads]);

  return (
    <div className="grid min-h-[calc(100vh-76px)] grid-cols-1 items-start gap-4 p-4 xl:grid-cols-[1fr_400px]">
      <div className="space-y-4">
        <header className="rounded-lg border border-[#e5e7eb] bg-white p-5 shadow-sm">
          <div className="flex items-center gap-3">
            <div className="grid h-10 w-10 place-items-center rounded-lg bg-[#f0fdf4] text-[#16a34a]">
              <CircleDollarSign className="h-5 w-5" />
            </div>
            <div>
              <h1 className="text-[20px] font-bold text-[#111827]">FinOps & Cost Analytics</h1>
              <p className="text-[13px] text-[#647084]">Track cluster spending, right-size workloads, and measure the financial impact of automated remediation.</p>
            </div>
          </div>
        </header>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <SummaryCard 
            title="Projected Cluster Cost" 
            value={`$${clusterCostMo.toFixed(2)}`} 
            subtitle="Monthly compute estimate"
            icon={<Server className="h-4 w-4" />}
            tone="blue"
          />
          <SummaryCard 
            title="Downtime Risk Prevented" 
            value={`$${totalDowntimeRiskHr.toFixed(2)}`} 
            subtitle="Hourly risk value avoided"
            icon={<ShieldAlert className="h-4 w-4" />}
            tone="green"
          />
          <SummaryCard 
            title="Prevention Cost" 
            value={`$${totalPreventionCostMo.toFixed(2)}`} 
            subtitle="Cost of AI remediation/mo"
            icon={<Calculator className="h-4 w-4" />}
            tone="orange"
          />
        </div>

        <section className="rounded-lg border border-[#e5e7eb] bg-white shadow-sm">
          <div className="border-b border-[#e5e7eb] px-4 py-3">
            <h2 className="text-[14px] font-semibold text-[#111827]">Top Wasted Resources (Right-Sizing)</h2>
            <p className="mt-0.5 text-[12px] text-[#647084]">Workloads sorted by requested monthly cost vs actual usage.</p>
          </div>
          {workloadsWithCost.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-[12px]">
                <thead className="bg-[#f8fafc] text-[#475569]">
                  <tr>
                    <th className="px-4 py-3 font-semibold">Workload</th>
                    <th className="px-4 py-3 font-semibold">Kind</th>
                    <th className="px-4 py-3 font-semibold">Requested / Mo</th>
                    <th className="px-4 py-3 font-semibold">Est. Actual / Mo</th>
                    <th className="px-4 py-3 font-semibold">Pricing Source</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[#e5e7eb]">
                  {workloadsWithCost.map(w => {
                    const requested = w.cost?.requestedMonthlyCost || 0;
                    const actual = w.cost?.monthlyCost || 0;
                    return (
                      <tr key={w.id} className="hover:bg-[#f8fafc]">
                        <td className="px-4 py-3 font-medium text-[#111827]">{w.workload.namespace}/{w.workload.name}</td>
                        <td className="px-4 py-3 text-[#475569]">{w.workload.kind}</td>
                        <td className="px-4 py-3 font-semibold text-[#dc2626]">${requested.toFixed(2)}</td>
                        <td className="px-4 py-3 font-medium text-[#16a34a]">${actual.toFixed(2)}</td>
                        <td className="px-4 py-3 text-[#647084]">{w.cost?.pricingSource || 'Unknown'}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="p-8 text-center text-[13px] text-[#647084]">
              No workload cost data available. Ensure metrics server and finops provider are configured.
            </div>
          )}
        </section>
      </div>

      <div className="space-y-4">
        <section className="rounded-lg border border-[#e5e7eb] bg-white shadow-sm">
          <div className="border-b border-[#e5e7eb] px-4 py-3">
            <h2 className="text-[14px] font-semibold text-[#111827]">Node Cost Breakdown</h2>
          </div>
          <div className="divide-y divide-[#e5e7eb]">
            {nodeCosts.length > 0 ? nodeCosts.map(node => (
              <div key={node.name} className="p-4 text-[12px]">
                <div className="flex items-center justify-between">
                  <span className="font-semibold text-[#111827]">{node.name}</span>
                  <span className="font-semibold text-[#2563eb]">${node.monthlyCost.toFixed(2)}/mo</span>
                </div>
                <div className="mt-2 grid grid-cols-2 gap-2 text-[#475569]">
                  <div>Type: <span className="font-medium text-[#111827]">{node.instanceType}</span></div>
                  <div>Region: <span className="font-medium text-[#111827]">{node.region}</span></div>
                  <div>CPU: <span className="font-medium text-[#111827]">{node.cpuRequestedCores?.toFixed(2) || 'N/A'} cores</span></div>
                  <div>RAM: <span className="font-medium text-[#111827]">{node.memoryRequestedMiB ? (node.memoryRequestedMiB / 1024).toFixed(2) : 'N/A'} GB</span></div>
                </div>
              </div>
            )) : (
              <div className="p-6 text-center text-[12px] text-[#647084]">No node cost data reported.</div>
            )}
          </div>
        </section>
      </div>
    </div>
  );
};

const SummaryCard = ({ title, value, subtitle, icon, tone }: { title: string, value: string, subtitle: string, icon: React.ReactNode, tone: 'blue' | 'green' | 'orange' }) => {
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
        <div>
          <div className="text-[12px] font-medium text-[#647084]">{title}</div>
          <div className="text-[20px] font-bold text-[#111827]">{value}</div>
        </div>
      </div>
      <div className="mt-3 text-[11px] text-[#94a3b8]">{subtitle}</div>
    </div>
  );
};
