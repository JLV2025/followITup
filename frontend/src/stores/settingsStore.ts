import { create } from "zustand";

export type DisplayMode = "calendar" | "fiscal";

interface SettingsState {
  displayMode: DisplayMode;
  fiscalStartMonth: number;
  setDisplayMode: (mode: DisplayMode) => void;
  setFiscalStartMonth: (month: number) => void;
}

const STORAGE_KEY = "followitup-settings";

function loadFromStorage(): { displayMode: DisplayMode; fiscalStartMonth: number } {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const data = JSON.parse(raw);
      return {
        displayMode: data.displayMode === "fiscal" ? "fiscal" : "calendar",
        fiscalStartMonth: typeof data.fiscalStartMonth === "number" && data.fiscalStartMonth >= 1 && data.fiscalStartMonth <= 12
          ? data.fiscalStartMonth
          : 4,
      };
    }
  } catch {
    // 解析失败时使用默认值
  }
  return { displayMode: "calendar", fiscalStartMonth: 4 };
}

function saveToStorage(state: SettingsState) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({
      displayMode: state.displayMode,
      fiscalStartMonth: state.fiscalStartMonth,
    }));
  } catch {
    // localStorage 不可用时静默忽略
  }
}

export const useSettingsStore = create<SettingsState>((set) => {
  const saved = loadFromStorage();
  return {
    displayMode: saved.displayMode,
    fiscalStartMonth: saved.fiscalStartMonth,

    setDisplayMode: (mode: DisplayMode) => {
      set((s) => {
        const next = { ...s, displayMode: mode };
        saveToStorage(next);
        return { displayMode: mode };
      });
    },

    setFiscalStartMonth: (month: number) => {
      if (month < 1 || month > 12) return;
      set((s) => {
        const next = { ...s, fiscalStartMonth: month };
        saveToStorage(next);
        return { fiscalStartMonth: month };
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
