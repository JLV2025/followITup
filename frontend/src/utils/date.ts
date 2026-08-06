/** 统一日期格式化 — 数据层始终 YYYY-MM-DD，仅展示层转换 */

const MONTHS_SHORT = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/** 将 YYYY-MM-DD 或 Date → M/D/YYYY */
export function formatDate(iso: string | Date | null | undefined): string {
  if (!iso) return "—";
  const d = typeof iso === "string" ? new Date(iso) : new Date(iso);
  if (isNaN(d.getTime())) return String(iso).slice(0, 10);
  return `${d.getMonth() + 1}/${d.getDate()}/${d.getFullYear()}`;
}

/** 将 YYYY-MM-DD 或 Date → Aug 02 格式 */
export function formatDateShort(iso: string | Date | null | undefined): string {
  if (!iso) return "";
  const d = typeof iso === "string" ? new Date(iso) : new Date(iso);
  if (isNaN(d.getTime())) return String(iso).slice(0, 10);
  return `${MONTHS_SHORT[d.getMonth()]} ${String(d.getDate()).padStart(2, "0")}`;
}

/** 将任意日期值标准化为 YYYY-MM-DD（用于 API 调用） */
export function fmtDateAPI(d: unknown): string {
  if (!d) return new Date().toISOString().slice(0, 10);
  if (typeof d === "string") return d.slice(0, 10);
  const dt = new Date(d as any);
  return isNaN(dt.getTime()) ? new Date().toISOString().slice(0, 10) : dt.toISOString().slice(0, 10);
}
