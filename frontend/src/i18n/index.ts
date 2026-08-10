/** i18n 初始化：i18next + react-i18next，双语（zh/en），localStorage 持久化。
 * 语言切换策略：setLanguage 写 localStorage + changeLanguage + 调用方整页刷新
 * （dhtmlx gantt 的 scale/tooltip/缩放标签在初始化时固化，无干净二次初始化路径）。 */
import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import zh from "./locales/zh";
import en from "./locales/en";

export const LANGS = ["zh", "en"] as const;
export type Lang = (typeof LANGS)[number];
export const LANG_KEY = "followitup-lang"; // 与 followitup-settings 同前缀，独立 key（i18n 初始化早于 store）

/** 读取持久化语言；非法/缺失回退中文 */
function loadLang(): Lang {
  try {
    const v = localStorage.getItem(LANG_KEY);
    if (v === "zh" || v === "en") return v;
  } catch {
    /* localStorage 不可用时静默回退 */
  }
  return "zh";
}

i18n.use(initReactI18next).init({
  resources: {
    zh: { translation: zh },
    en: { translation: en },
  },
  lng: loadLang(),
  fallbackLng: "zh",
  interpolation: { escapeValue: false }, // React 默认防 XSS，无需 i18next 转义
});

// html lang 与标题随语言同步
i18n.on("languageChanged", (l) => {
  document.documentElement.lang = l;
  document.title = i18n.t("app.title");
});

/** 切换语言（调用方随后 window.location.reload() 以重建 gantt 等初始化时固化的部分） */
export function setLanguage(l: Lang): void {
  try {
    localStorage.setItem(LANG_KEY, l);
  } catch {
    /* 忽略写入失败，仅本次会话生效 */
  }
  i18n.changeLanguage(l);
}

export default i18n;
