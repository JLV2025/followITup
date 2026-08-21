/** 统一日期格式化 — 数据层始终 YYYY-MM-DD，仅展示层转换 */
import i18n from "../i18n";

/** 将 YYYY-MM-DD 或 Date → M/D/YYYY（语言中立，全站统一） */
export function formatDate(iso: string | Date | null | undefined): string {
  if (!iso) return "—";
  const d = typeof iso === "string" ? new Date(iso) : new Date(iso);
  if (isNaN(d.getTime())) return String(iso).slice(0, 10);
  return `${d.getMonth() + 1}/${d.getDate()}/${d.getFullYear()}`;
}

/** 将 YYYY-MM-DD 或 Date → 短日期（en: "Aug 02" / zh: "8月02日"），月份词随语言 */
export function formatDateShort(iso: string | Date | null | undefined): string {
  if (!iso) return "";
  const d = typeof iso === "string" ? new Date(iso) : new Date(iso);
  if (isNaN(d.getTime())) return String(iso).slice(0, 10);
  const m = d.getMonth();
  const day = String(d.getDate()).padStart(2, "0");
  return i18n.t("date.shortFormat", { month: i18n.t(`months.short.${m}`), day });
}

/** 将任意日期值标准化为 YYYY-MM-DD（用于 API 调用） */
export function fmtDateAPI(d: unknown): string {
  if (!d) return new Date().toISOString().slice(0, 10);
  if (typeof d === "string") return d.slice(0, 10);
  const dt = new Date(d as any);
  return isNaN(dt.getTime()) ? new Date().toISOString().slice(0, 10) : dt.toISOString().slice(0, 10);
}

// ============================================================================
// 工作日计算（镜像后端 scheduler/calendar.go 语义：AddWorkDays 不含起点 / CountWorkDays 含首尾）
// ============================================================================

/** 工作日历映射：date → "holiday"（节假日，从工作日排除）| "workday"（补班，周末计工作日） */
export type CalMap = Record<string, "holiday" | "workday">;

/** 某天是否为工作日：周一~五默认是（holiday 排除）；周六日默认否（workday 补班计入） */
export function isWorkDay(date: string, cal?: CalMap): boolean {
  const type = cal?.[date];
  if (type === "holiday") return false;
  if (type === "workday") return true;
  const d = new Date(`${date}T00:00:00`);
  if (isNaN(d.getTime())) return false;
  const day = d.getDay();
  return day !== 0 && day !== 6;
}

/** 日期 + n 天（日历日，YYYY-MM-DD） */
function shiftDay(date: string, n: number): string {
  const d = new Date(`${date}T00:00:00`);
  d.setDate(d.getDate() + n);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/** start 后 n 个工作日（不含 start 当天；与后端 AddWorkDays 一致） */
export function addWorkDays(start: string, n: number, cal?: CalMap): string {
  let cur = start;
  let remaining = n;
  while (remaining > 0) {
    cur = shiftDay(cur, 1);
    if (isWorkDay(cur, cal)) remaining--;
  }
  return cur;
}

/** end 前 n 个工作日（不含 end 当天；与后端 SubWorkDays 一致） */
export function subWorkDays(end: string, n: number, cal?: CalMap): string {
  let cur = end;
  let remaining = n;
  while (remaining > 0) {
    cur = shiftDay(cur, -1);
    if (isWorkDay(cur, cal)) remaining--;
  }
  return cur;
}

/** start~end 之间的工作日数（含首尾；与后端 CountWorkDays 一致） */
export function countWorkDays(start: string, end: string, cal?: CalMap): number {
  if (!start || !end || end < start) return 0;
  let count = 0;
  let cur = start;
  while (cur <= end) {
    if (isWorkDay(cur, cal)) count++;
    cur = shiftDay(cur, 1);
  }
  return count;
}
