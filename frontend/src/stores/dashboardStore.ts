import { create } from "zustand";
import api from "../api/client";
import { useSettingsStore } from "./settingsStore";

interface DashboardStats {
  active_projects: number;
  at_risk: number;
  due_this_week: number;
  overall_progress: number;
  baseline_progress: number;
  has_baseline: boolean;
}

interface ProjectSummary {
  id: number;
  name: string;
  description: string;
  start_date: string;
  end_date: string;
  status: string;
  task_count: number;
  completed_count: number;
  progress: number;
  next_milestone: string;
  risk_count: number;
  has_risk: boolean;
  baseline_created_at: string;
  delay_days: number;
}

interface DashboardState {
  stats: DashboardStats | null;
  projects: ProjectSummary[];
  period: number; // 自然年时为日历年（如 2026），财年时为财年编号（如 27）
  loading: boolean;
  fetchStats: () => Promise<void>;
  fetchProjects: () => Promise<void>;
  setPeriod: (period: number) => void;
}

export const useDashboardStore = create<DashboardState>((set, get) => ({
  stats: null,
  projects: [],
  period: new Date().getFullYear(),
  loading: false,

  fetchStats: async () => {
    const { period } = get();
    const { displayMode } = useSettingsStore.getState();
    const param = displayMode === "fiscal" ? `fy=${period}` : `year=${period}`;
    try {
      const res = await api.get(`/api/dashboard/stats?${param}`);
      set({ stats: res.data.data });
    } catch {
      // 后端不可达时使用占位数据
    }
  },

  fetchProjects: async () => {
    set({ loading: true });
    const { period } = get();
    const { displayMode } = useSettingsStore.getState();
    const param = displayMode === "fiscal" ? `fy=${period}` : `year=${period}`;
    try {
      const res = await api.get(`/api/dashboard/projects?${param}`);
      set({ projects: res.data.data || [], loading: false });
    } catch {
      set({ loading: false });
    }
  },

  setPeriod: (period: number) => {
    set({ period });
    get().fetchStats();
    get().fetchProjects();
  },
}));
