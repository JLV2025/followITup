import { create } from "zustand";
import api from "../api/client";
import { toGanttTask, toGanttLink, fromGanttTask, fromGanttLink } from "../api/gantt-adapter";
import type { GanttTask, GanttLink } from "../api/gantt-adapter";

/** 聚焦信息：某用户正在查看/编辑某任务 */
export interface FocusInfo {
  userName: string;
  color: string;
  expires: number;
}

const USER_COLORS = [
  "#5B8DEF", "#F5A623", "#7ED321", "#D0021B", "#BD10E0",
  "#4A90D9", "#F8E71C", "#50E3C2", "#9013FE", "#FF6B6B",
];

const userColorMap = new Map<string, string>();
let colorIndex = 0;

function getUserColor(userName: string): string {
  if (!userColorMap.has(userName)) {
    userColorMap.set(userName, USER_COLORS[colorIndex % USER_COLORS.length]);
    colorIndex++;
  }
  return userColorMap.get(userName)!;
}

export interface BaselineMeta {
  created_at: string;
  created_by: string;
  task_count: number;
}

interface GanttState {
  tasks: GanttTask[];
  links: GanttLink[];
  loading: boolean;
  readonly: boolean;
  focusMap: Record<number, FocusInfo>;
  baselineMeta: BaselineMeta | null;
  fetchBaselineMeta: (projectId: number) => Promise<void>;
  createBaseline: (projectId: number) => Promise<boolean>;
  clearBaseline: (projectId: number) => Promise<boolean>;
  fetchData: (projectId: number, readonly: boolean) => Promise<void>;
  updateTask: (id: number, changes: Partial<GanttTask>, projectId: number) => Promise<boolean>;
  addLink: (link: GanttLink, projectId: number) => Promise<void>;
  deleteLink: (linkId: number, projectId: number) => Promise<void>;
  setFocus: (taskId: number, userName: string) => void;
  clearFocus: (taskId: number) => void;
  pruneExpired: () => void;
}

export const useGanttStore = create<GanttState>((set, get) => ({
  tasks: [],
  links: [],
  loading: true,
  readonly: true,
  focusMap: {},
  baselineMeta: null,

  fetchData: async (projectId, readonly) => {
    set({ loading: true, readonly });
    try {
      const res = await api.get(`/api/projects/${projectId}/tasks`);
      const data = res.data.data;
      const tasks = (data.tasks || []).map((t: any) => toGanttTask(t, readonly));
      const links = (data.dependencies || []).map((d: any) => toGanttLink(d));
      set({ tasks, links, loading: false });
    } catch {
      set({ loading: false });
    }
  },

  fetchBaselineMeta: async (projectId) => {
    try {
      const res = await api.get(`/api/projects/${projectId}/baseline`);
      set({ baselineMeta: res.data.data || null });
    } catch {
      set({ baselineMeta: null });
    }
  },

  createBaseline: async (projectId) => {
    try {
      await api.post(`/api/projects/${projectId}/baseline`);
      const s = get();
      await s.fetchData(projectId, s.readonly);
      await s.fetchBaselineMeta(projectId);
      return true;
    } catch {
      return false;
    }
  },

  clearBaseline: async (projectId) => {
    try {
      await api.delete(`/api/projects/${projectId}/baseline`);
      const s = get();
      await s.fetchData(projectId, s.readonly);
      set({ baselineMeta: null });
      return true;
    } catch {
      return false;
    }
  },

  updateTask: async (id, changes, projectId) => {
    const task = get().tasks.find((t) => t.id === id);
    if (!task) return false;

    const updatedGantt = { ...task, ...changes };
    const payload = fromGanttTask(updatedGantt);

    try {
      await api.put(`/api/projects/${projectId}/tasks/${id}`, payload);
      // 调度器现已同步执行，PUT 返回后数据库已是最新状态
      const fullRes = await api.get(`/api/projects/${projectId}/tasks`);
      const data = fullRes.data.data;
      set({
        tasks: (data.tasks || []).map((t: any) => toGanttTask(t, get().readonly)),
        links: (data.dependencies || []).map((d: any) => toGanttLink(d)),
      });
      return true;
    } catch (err: any) {
      if (err.response?.status === 409) {
        alert("任务已被他人修改，数据已刷新");
        get().fetchData(projectId, get().readonly);
      }
      return false;
    }
  },

  addLink: async (link, projectId) => {
    const payload = fromGanttLink(link);
    try {
      await api.post(`/api/projects/${projectId}/dependencies`, payload);
      get().fetchData(projectId, get().readonly);
    } catch {
      // ignore
    }
  },

  deleteLink: async (linkId, projectId) => {
    try {
      await api.delete(`/api/projects/${projectId}/dependencies/${linkId}`);
      get().fetchData(projectId, get().readonly);
    } catch {
      // ignore
    }
  },

  setFocus: (taskId, userName) => {
    set((s) => ({
      focusMap: {
        ...s.focusMap,
        [taskId]: {
          userName,
          color: getUserColor(userName),
          expires: Date.now() + 15000,
        },
      },
    }));
  },

  clearFocus: (taskId) => {
    set((s) => {
      const next = { ...s.focusMap };
      delete next[taskId];
      return { focusMap: next };
    });
  },

  pruneExpired: () => {
    const now = Date.now();
    const next: Record<number, FocusInfo> = {};
    let changed = false;
    for (const [id, info] of Object.entries(get().focusMap)) {
      if (info.expires > now) {
        next[Number(id)] = info;
      } else {
        changed = true;
      }
    }
    if (changed) set({ focusMap: next });
  },
}));
