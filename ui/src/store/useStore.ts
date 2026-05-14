import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, DashboardState } from '../types';

interface AppState {
  user: User | null;
  token: string | null;
  dashboard: DashboardState | null;
  setUser: (user: User | null) => void;
  setToken: (token: string | null) => void;
  setDashboard: (data: DashboardState | null) => void;
  logout: () => void;
}

export const useStore = create<AppState>()(
  persist(
    (set) => ({
      user: null,
      token: null,
      dashboard: null,
      setUser: (user) => set({ user }),
      setToken: (token) => set({ token }),
      setDashboard: (dashboard) => set({ dashboard }),
      logout: () => set({ user: null, token: null, dashboard: null }),
    }),
    {
      name: 'fixora-storage',
      partialize: (state) => ({ user: state.user, token: state.token }), // Only persist auth
    }
  )
);
