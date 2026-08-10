/** 状态/优先级统一标签（各组件共用，消除 5 处重复映射；措辞统一：待开始/进行中/已完成/已延期） */
import i18n from "../i18n";

/** 任务状态中文/英文标签（未知值原样返回） */
export function statusLabel(s: string): string {
  return i18n.t(`status.${s}`, { defaultValue: s });
}

/** 优先级标签（未知值原样返回） */
export function priorityLabel(p: string): string {
  return i18n.t(`priority.${p}`, { defaultValue: p });
}
