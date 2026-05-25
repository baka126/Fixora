import type { DashboardRemediation } from '../types';

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
