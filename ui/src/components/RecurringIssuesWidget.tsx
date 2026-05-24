import { useMemo, useState } from 'react';
import { Copy, Repeat, RefreshCw, TriangleAlert, ChevronDown, ChevronUp } from 'lucide-react';
import type { DashboardRemediation } from '../types';

interface RecurringIssuesWidgetProps {
  remediations: DashboardRemediation[];
}

export const RecurringIssuesWidget = ({ remediations }: RecurringIssuesWidgetProps) => {
  const [expandedRow, setExpandedRow] = useState<string | null>(null);

  const groups = useMemo(() => {
    const map = new Map<string, DashboardRemediation[]>();
    remediations.forEach(r => {
      if (!r.failureFingerprint) return;
      const list = map.get(r.failureFingerprint) || [];
      list.push(r);
      map.set(r.failureFingerprint, list);
    });

    return Array.from(map.entries())
      .map(([fingerprint, items]) => ({
        fingerprint,
        count: items.length,
        items,
        workload: items[0].workload,
        reason: items[0].failureReason || 'Unknown error',
      }))
      .filter(g => g.count > 1) // Only show recurring ones
      .sort((a, b) => b.count - a.count)
      .slice(0, 10); // Top 10
  }, [remediations]);

  if (!groups.length) {
    return (
      <section className="rounded-lg border border-[#e5e7eb] bg-white p-3 shadow-sm">
        <div className="mb-3 flex items-center gap-2">
          <Repeat className="h-4 w-4 text-[#2563eb]" />
          <h2 className="text-[14px] font-semibold text-[#111827]">Top Recurring Issues</h2>
        </div>
        <div className="rounded-md border border-dashed border-[#e5e7eb] p-6 text-center text-[12px] text-[#647084]">
          No recurring issues detected based on failure fingerprints.
        </div>
      </section>
    );
  }

  return (
    <section className="rounded-lg border border-[#e5e7eb] bg-white p-3 shadow-sm">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Repeat className="h-4 w-4 text-[#2563eb]" />
          <h2 className="text-[14px] font-semibold text-[#111827]">Top Recurring Issues</h2>
          <span className="rounded-full bg-[#eff6ff] px-2 py-0.5 text-[10px] font-semibold text-[#2563eb]">
            By Fingerprint
          </span>
        </div>
      </div>

      <div className="overflow-x-auto rounded-md border border-[#e5e7eb]">
        <table className="w-full text-left text-[12px]">
          <thead className="bg-[#f8fafc] text-[#475569]">
            <tr>
              <th className="px-3 py-2 font-semibold">Fingerprint</th>
              <th className="px-3 py-2 font-semibold">Occurrences</th>
              <th className="px-3 py-2 font-semibold">Workload</th>
              <th className="px-3 py-2 font-semibold">Failure Reason</th>
              <th className="px-3 py-2 font-semibold"></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#e5e7eb]">
            {groups.map(g => {
              const isExpanded = expandedRow === g.fingerprint;
              return (
                <optgroup key={g.fingerprint} className="bg-white">
                  <tr 
                    className="cursor-pointer hover:bg-[#f8fafc]"
                    onClick={() => setExpandedRow(isExpanded ? null : g.fingerprint)}
                  >
                    <td className="px-3 py-2">
                      <div className="flex items-center gap-1.5 font-mono text-[11px] text-[#475569]">
                        {g.fingerprint.slice(0, 8)}...
                        <button 
                          onClick={(e) => {
                            e.stopPropagation();
                            navigator.clipboard.writeText(g.fingerprint);
                          }}
                          className="hover:text-[#2563eb]"
                          title="Copy full fingerprint"
                        >
                          <Copy className="h-3 w-3" />
                        </button>
                      </div>
                    </td>
                    <td className="px-3 py-2">
                      <span className="inline-flex items-center gap-1 rounded-md bg-[#fee2e2] px-2 py-0.5 font-semibold text-[#dc2626]">
                        <RefreshCw className="h-3 w-3" />
                        {g.count}
                      </span>
                    </td>
                    <td className="px-3 py-2">
                      <div className="font-medium text-[#111827]">{g.workload.kind}/{g.workload.name}</div>
                      <div className="text-[11px] text-[#647084]">{g.workload.namespace}</div>
                    </td>
                    <td className="px-3 py-2">
                      <div className="max-w-[200px] truncate text-[#475569]" title={g.reason}>
                        {g.reason}
                      </div>
                    </td>
                    <td className="px-3 py-2 text-right">
                      <button className="p-1 text-[#94a3b8] hover:text-[#111827]">
                        {isExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                      </button>
                    </td>
                  </tr>
                  {isExpanded && (
                    <tr>
                      <td colSpan={5} className="bg-[#f8fafc] p-3 shadow-inner">
                        <div className="rounded-md border border-[#e5e7eb] bg-white p-3">
                          <h4 className="mb-2 text-[11px] font-semibold uppercase text-[#647084]">Recent Occurrences</h4>
                          <div className="space-y-2">
                            {g.items.slice(0, 5).map(item => (
                              <div key={item.id} className="flex items-center justify-between text-[11px]">
                                <div className="flex items-center gap-2">
                                  <TriangleAlert className="h-3 w-3 text-[#f97316]" />
                                  <span className="font-medium text-[#374151]">{item.title || item.strategy || 'Remediation'}</span>
                                  <span className="text-[#94a3b8]">· {item.age} ago</span>
                                </div>
                                <span className={`rounded-md px-2 py-0.5 font-medium ${item.status === 'succeeded' ? 'bg-[#dcfce7] text-[#15803d]' : item.status === 'failed' ? 'bg-[#fee2e2] text-[#dc2626]' : 'bg-[#f1f5f9] text-[#475569]'}`}>
                                  {item.status}
                                </span>
                              </div>
                            ))}
                            {g.items.length > 5 && (
                              <div className="text-[11px] text-[#647084]">
                                + {g.items.length - 5} more occurrences.
                              </div>
                            )}
                          </div>
                        </div>
                      </td>
                    </tr>
                  )}
                </optgroup>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
};
