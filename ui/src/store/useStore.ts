import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, DashboardState } from '../types';

interface AppState {
  user: User | null;
  token: string | null;
  dashboard: DashboardState | null;
  selectedCluster: string;
  searchQuery: string;
  timeRange: string;
  setUser: (user: User | null) => void;
  setToken: (token: string | null) => void;
  setDashboard: (data: DashboardState | null) => void;
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
      selectedCluster: '',
      searchQuery: '',
      timeRange: 'Last 24h',
      setUser: (user) => set({ user }),
      setToken: (token) => set({ token }),
      setDashboard: (dashboard) => set((state) => ({
        dashboard,
        selectedCluster: state.selectedCluster || dashboard?.environment || 'cluster',
        timeRange: state.timeRange || dashboard?.timeRange || 'Last 24h',
      })),
      setSelectedCluster: (selectedCluster) => set({ selectedCluster }),
      setSearchQuery: (searchQuery) => set({ searchQuery }),
      setTimeRange: (timeRange) => set({ timeRange }),
      logout: () => set({ user: null, token: null, dashboard: null, selectedCluster: '', searchQuery: '', timeRange: 'Last 24h' }),
    }),
    {
      name: 'fixora-storage',
      partialize: (state) => ({ user: state.user, token: state.token }), // Only persist auth
    }
  )
);
