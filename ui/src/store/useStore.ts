import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, DashboardState } from '../types';

interface AppState {
  user: User | null;
  token: string | null;
  dashboard: DashboardState | null;
  realtimeStatus: 'connecting' | 'connected' | 'polling' | 'disconnected';
  realtimeMessage: string;
  selectedCluster: string;
  searchQuery: string;
  timeRange: string;
  setUser: (user: User | null) => void;
  setToken: (token: string | null) => void;
  setDashboard: (data: DashboardState | null) => void;
  setRealtimeStatus: (status: AppState['realtimeStatus'], message?: string) => void;
  setSelectedCluster: (cluster: string) => void;
  setSearchQuery: (query: string) => void;
  setTimeRange: (range: string) => void;
  logout: () => void;
}

export const useStore = create<AppState>()(
  persist(
    (set) => ({
      user: null,
      token: null,
      dashboard: null,
      realtimeStatus: 'disconnected',
      realtimeMessage: 'Realtime updates are not connected.',
      selectedCluster: '',
      searchQuery: '',
      timeRange: 'Last 24h',
      setUser: (user) => set({ user }),
      setToken: (token) => set({ token }),
      setDashboard: (dashboard) => set((state) => {
        const environment = dashboard?.environment || '';
        const selectedCluster = !state.selectedCluster || isGenericClusterName(state.selectedCluster)
          ? environment || 'cluster'
          : state.selectedCluster;
        return {
          dashboard,
          selectedCluster,
          timeRange: state.timeRange || dashboard?.timeRange || 'Last 24h',
        };
      }),
      setRealtimeStatus: (realtimeStatus, realtimeMessage) => set({
        realtimeStatus,
        realtimeMessage: realtimeMessage || realtimeStatusMessage(realtimeStatus),
      }),
      setSelectedCluster: (selectedCluster) => set({ selectedCluster }),
      setSearchQuery: (searchQuery) => set({ searchQuery }),
      setTimeRange: (timeRange) => set({ timeRange }),
      logout: () => set({ user: null, token: null, dashboard: null, realtimeStatus: 'disconnected', realtimeMessage: 'Realtime updates are not connected.', selectedCluster: '', searchQuery: '', timeRange: 'Last 24h' }),
    }),
    {
      name: 'fixora-storage',
      partialize: (state) => ({ user: state.user, token: state.token }), // Only persist auth
    }
  )
);

const realtimeStatusMessage = (status: AppState['realtimeStatus']) => {
  switch (status) {
    case 'connecting':
      return 'Connecting realtime dashboard stream...';
    case 'connected':
      return 'Realtime dashboard stream connected.';
    case 'polling':
      return 'Realtime stream unavailable; using safe polling fallback.';
    case 'disconnected':
    default:
      return 'Realtime updates are not connected.';
  }
};

const isGenericClusterName = (value: string) => {
  const normalized = value.trim().toLowerCase();
  return normalized === '' || normalized === 'cluster' || normalized === 'default' || normalized === 'default-cluster';
};
