import { useEffect, useRef } from 'react';
import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import {
  Activity,
  Bell,
  CalendarDays,
  CheckCircle2,
  ChevronDown,
  Database,
  FileCode2,
  FileText,
  GitPullRequestArrow,
  LineChart,
  LogOut,
  Search,
  Settings,
  ShieldCheck,
  TriangleAlert,
} from 'lucide-react';
import { apiClient } from '../api/client';
import { useWebSocket } from '../hooks/useWebSocket';
import { useStore } from '../store/useStore';
import type { DashboardIntegration } from '../types';

const navItems = [
  { to: '/', icon: TriangleAlert, label: 'Incidents' },
  { to: '/remediations', icon: GitPullRequestArrow, label: 'Remediations' },
  { to: '/gitops', icon: FileCode2, label: 'GitOps Sources' },
  { to: '/predictions', icon: LineChart, label: 'Predictions' },
  { to: '/audit', icon: FileText, label: 'Audit' },
  { to: '/settings', icon: Settings, label: 'Settings' },
];

const integrationIcon = (name: string) => {
  const lower = name.toLowerCase();
  if (lower.includes('postgres')) return Database;
  if (lower.includes('argo') || lower.includes('flux')) return FileCode2;
  if (lower.includes('prometheus')) return Activity;
  return ShieldCheck;
};

const integrationLabel = (integration: DashboardIntegration) => {
  if (integration.status === 'ok') return 'Healthy';
  if (integration.status === 'error') return 'Error';
  return integration.detail || 'Not configured';
};

export const Layout = () => {
  const { user, dashboard, setDashboard, logout } = useStore();
  const requestedDashboard = useRef(false);
  const navigate = useNavigate();
  useWebSocket();

  useEffect(() => {
    if (dashboard || requestedDashboard.current) return;
    requestedDashboard.current = true;
    apiClient
      .get('/dashboard')
      .then(({ data }) => setDashboard(data))
      .catch(() => undefined);
  }, [dashboard, setDashboard]);

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const metadata = dashboard?.metadata;
  const kpiByLabel = new Map((dashboard?.kpis || []).map((kpi) => [kpi.label, kpi.value]));
  const healthText = metadata?.systemHealth === 'operational' ? 'All systems operational' : metadata?.systemHealth || 'Waiting for telemetry';

  return (
    <div className="min-h-screen bg-[#f7f8fb] text-[#111827]">
      <aside className="fixed inset-y-0 left-0 z-20 flex w-[202px] flex-col border-r border-[#e5e7eb] bg-white">
        <div className="flex h-[76px] items-center gap-3 px-6">
          <div className="grid h-9 w-9 place-items-center rounded-lg bg-gradient-to-br from-[#ff7a1a] to-[#ef4444] text-white shadow-sm">
            <ShieldCheck className="h-5 w-5" />
          </div>
          <strong className="text-[28px] leading-none tracking-[-0.02em] text-[#111827]">Fixora</strong>
        </div>

        <nav className="flex-1 space-y-3 px-3 py-5">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                `flex h-10 items-center gap-3 rounded-md px-3 text-[14px] font-medium transition ${
                  isActive
                    ? 'bg-[#fde7e5] text-[#dc2626]'
                    : 'text-[#1f2937] hover:bg-[#f3f4f6] hover:text-[#111827]'
                }`
              }
            >
              <item.icon className="h-[18px] w-[18px]" />
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>

        <div className="m-3 rounded-lg border border-[#e5e7eb] bg-white p-3">
          <h3 className="mb-4 text-[12px] font-semibold text-[#111827]">Insights</h3>
          <div className="space-y-3 text-[12px]">
            <Insight label="MTTR (24h)" value="n/a" />
            <Insight label="PRs Merged (24h)" value={kpiByLabel.get('Auto-fix success') || 'n/a'} tone="green" />
            <Insight label="Risk Avoided (30d)" value={kpiByLabel.get('Monthly risk avoided') || 'n/a'} tone="green" />
            <Insight label="Auto-fix Success (30d)" value={kpiByLabel.get('Auto-fix success') || 'n/a'} tone="green" />
          </div>
          <div className="mt-8 border-t border-[#e5e7eb] pt-4 text-[12px]">
            <div className="font-semibold text-[#111827]">System Version</div>
            <div className="mt-2 text-[#4b5563]">v{metadata?.version || 'unknown'}</div>
            <div className="text-[#6b7280]">Build {metadata?.buildDate || 'pending'}</div>
            <div className="mt-5 flex items-center gap-2 text-[11px] text-[#4b5563]">
              <span className="h-2 w-2 rounded-full bg-[#16a34a]" />
              {healthText}
            </div>
          </div>
        </div>
      </aside>

      <div className="min-h-screen pl-[202px]">
        <header className="sticky top-0 z-10 flex h-[76px] items-center gap-4 border-b border-[#e5e7eb] bg-white px-4">
          <button className="flex h-10 min-w-[156px] items-center justify-between rounded-md border border-[#e5e7eb] bg-white px-4 text-[13px] font-medium text-[#111827]">
            <span className="flex items-center gap-2">
              <span className="h-3 w-3 rounded-full bg-[#15803d]" />
              {dashboard?.environment || 'cluster'}
            </span>
            <ChevronDown className="h-4 w-4 text-[#6b7280]" />
          </button>

          <div className="flex h-10 flex-1 items-center gap-3 rounded-md border border-[#e5e7eb] bg-white px-3 text-[#6b7280]">
            <Search className="h-4 w-4" />
            <input
              className="min-w-0 flex-1 bg-transparent text-[13px] outline-none placeholder:text-[#9ca3af]"
              placeholder="Search incidents, workloads, namespaces..."
            />
            <kbd className="rounded border border-[#e5e7eb] px-1.5 py-0.5 text-[11px] text-[#6b7280]">⌘ K</kbd>
          </div>

          <button className="flex h-10 min-w-[130px] items-center justify-between rounded-md border border-[#e5e7eb] bg-white px-4 text-[13px] font-medium">
            <span className="flex items-center gap-2">
              <CalendarDays className="h-4 w-4 text-[#374151]" />
              {dashboard?.timeRange || 'Last 24h'}
            </span>
            <ChevronDown className="h-4 w-4 text-[#6b7280]" />
          </button>

          <div className="hidden items-center gap-2 xl:flex">
            {(dashboard?.integrations || []).map((integration) => {
              const Icon = integrationIcon(integration.name);
              const healthy = integration.status === 'ok';
              return (
                <div key={integration.name} className="flex h-10 min-w-[95px] items-center gap-2 rounded-md border border-[#e5e7eb] bg-white px-3">
                  {healthy ? <CheckCircle2 className="h-4 w-4 text-[#16a34a]" /> : <Icon className="h-4 w-4 text-[#9ca3af]" />}
                  <div className="leading-tight">
                    <div className="text-[11px] font-semibold text-[#111827]">{integration.name}</div>
                    <div className={`text-[10px] font-medium ${healthy ? 'text-[#15803d]' : 'text-[#6b7280]'}`}>{integrationLabel(integration)}</div>
                  </div>
                </div>
              );
            })}
          </div>

          <button className="relative grid h-10 w-10 place-items-center rounded-md hover:bg-[#f3f4f6]" title="Notifications">
            <Bell className="h-5 w-5 text-[#111827]" />
            {!!metadata?.notifications && (
              <span className="absolute right-1.5 top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-[#dc2626] px-1 text-[10px] font-bold text-white">
                {metadata.notifications}
              </span>
            )}
          </button>

          <button onClick={handleLogout} className="grid h-10 w-10 place-items-center rounded-md hover:bg-[#f3f4f6]" title={`Logout ${user?.username || ''}`}>
            <LogOut className="h-4 w-4 text-[#6b7280]" />
          </button>
        </header>

        <main className="min-w-0">
          <Outlet />
        </main>
      </div>
    </div>
  );
};

const Insight = ({ label, value, tone }: { label: string; value: string; tone?: 'green' }) => (
  <div className="flex items-center justify-between gap-3">
    <span className="text-[#374151]">{label}</span>
    <strong className={tone === 'green' ? 'text-[#15803d]' : 'text-[#111827]'}>{value}</strong>
  </div>
);
