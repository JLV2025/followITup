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
