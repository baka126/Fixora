import { ExternalLink, Loader2 } from 'lucide-react';
import type { DashboardRemediation } from '../types';
import { remediationManualActions } from '../utils/remediationActions';

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
      {row.prUrl && <ActionExternal href={row.prUrl} label="Open PR" />}
      {row.revertPrUrl && <ActionExternal href={row.revertPrUrl} label="Open revert PR" />}
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

const ActionExternal = ({ href, label }: { href: string; label: string }) => (
  <a href={href} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 font-medium text-[#2563eb]">
    {label}
    <ExternalLink className="h-3.5 w-3.5" />
  </a>
);
