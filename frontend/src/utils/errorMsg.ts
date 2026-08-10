/** 错误消息统一处理：前端按后端 error.code 映射翻译，查不到回退后端中文消息。
 * 回退链：errors.<CODE> 翻译键（动态码透传 detail）→ 后端 message 原样 → 网络层 err.message → fallbackKey */
import i18n from "../i18n";

interface ApiErrorShape {
  response?: {
    data?: {
      error?: { code?: string; message?: string };
      message?: string; // 兼容旧响应结构（缺 .error 层）
    };
  };
  message?: string;
}

export function getErrorMessage(
  err: unknown,
  fallbackKey = "common.unknownError",
  params: Record<string, unknown> = {}
): string {
  const e = err as ApiErrorShape | undefined;
  const data = e?.response?.data;
  const code = data?.error?.code;
  const backendMsg = data?.error?.message ?? data?.message;
  if (code && i18n.exists(`errors.${code}`)) {
    return i18n.t(`errors.${code}`, { ...params, detail: backendMsg ?? params.detail ?? "" });
  }
  if (backendMsg) return backendMsg; // 后端中文消息原样返回（后端保持中文的既定决策）
  if (e?.message) return e.message; // 网络层错误（timeout/断网等）
  return i18n.t(fallbackKey, params);
}
