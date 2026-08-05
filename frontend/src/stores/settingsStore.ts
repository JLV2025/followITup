import { create } from "zustand";
import api from "../api/client";

export type DisplayMode = "calendar" | "fiscal";

interface SettingsState {
  displayMode: DisplayMode;
  fiscalStartMonth: number;
  setDisplayMode: (mode: DisplayMode) => void;
}

const STORAGE_KEY = "followitup-settings";

function loadDisplayMode(): DisplayMode {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const data = JSON.parse(raw);
      if (data.displayMode === "fiscal" || data.displayMode === "calendar") {
        return data.displayMode;
      }
    }
  } catch {
    // 解析失败时使用默认值
  }
  return "calendar";
}

function saveToStorage(displayMode: DisplayMode) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ displayMode }));
  } catch {
    // localStorage 不可用时静默忽略
  }
}

/** 财年起始月从系统配置读取（管理员在系统设置页修改），不再本地存储 */
async function loadFiscalStartMonth(): Promise<number> {
  try {
    const res = await api.get("/api/settings");
    const m = res.data?.data?.fiscal_start_month;
    return typeof m === "number" && m >= 1 && m <= 12 ? m : 4;
  } catch {
    return 4;
  }
}

export const useSettingsStore = create<SettingsState>((set) => {
  // 启动时异步拉取财年配置（未登录/失败时默认 4 月）
  loadFiscalStartMonth().then((m) => set({ fiscalStartMonth: m }));

  return {
    displayMode: loadDisplayMode(),
    fiscalStartMonth: 4,

    setDisplayMode: (mode: DisplayMode) => {
      set((s) => {
        const next = { ...s, displayMode: mode };
        saveToStorage(mode);
        return { displayMode: mode };
      });
    },
  };
});

// --- 纯工具函数（不依赖 Zustand，方便测试和复用） ---

/** 根据日期和财年起始月计算财年编号 */
export function fiscalYearFromDate(date: Date, startMonth: number): number {
  const year = date.getFullYear();
  if (startMonth === 1) return year - 2000;
  if (date.getMonth() + 1 >= startMonth) return year - 2000 + 1;
  return year - 2000;
}

/** 返回当前财年编号 */
export function currentFiscalYear(startMonth: number): number {
  return fiscalYearFromDate(new Date(), startMonth);
}

/** 返回可选的财年编号列表 */
export function availableFiscalYears(startMonth: number): number[] {
  const current = currentFiscalYear(startMonth);
  return [current - 2, current - 1, current, current + 1, current + 2];
}

/** 返回可选的日历年列表 */
export function availableCalendarYears(count: number = 5): number[] {
  const current = new Date().getFullYear();
  const years: number[] = [];
  for (let i = count - 1; i >= 0; i--) {
    years.push(current - i);
  }
  return years;
}

/** 财年标签 */
export function fiscalYearLabel(fy: number): string {
  return `FY${fy}`;
}
