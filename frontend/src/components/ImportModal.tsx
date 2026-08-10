import { getErrorMessage } from "../utils/errorMsg";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import api from "../api/client";
import i18n from "../i18n";

/** CSV 任务批量导入弹窗
 *  - 文件选择(UTF-8 或 GBK 自动识别,Excel 导出兼容)
 *  - 模板下载(UTF-8 BOM,Excel 双击不乱码)
 *  - 导入结果(成功行/跳过行/逐行错误)
 */
export default function ImportModal({
  projectId,
  onClose,
  onImported,
}: {
  projectId: number;
  onClose: () => void;
  onImported: () => void;
}) {
  const { t } = useTranslation();
  const [fileName, setFileName] = useState("");
  const [content, setContent] = useState("");
  const [importing, setImporting] = useState(false);
  const [result, setResult] = useState<{ imported: number; skipped: number; errors: string[] } | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  // 读取文件:优先 UTF-8(去 BOM);若出现替换字符则按 GBK 重解(中文 Excel 导出默认 GBK)
  const handleFile = async (f: File) => {
    setResult(null);
    setFileName(f.name);
    const buf = await f.arrayBuffer();
    let text = new TextDecoder("utf-8").decode(buf).replace(/^﻿/, "");
    if (text.includes("�")) {
      try {
        text = new TextDecoder("gbk").decode(buf).replace(/^﻿/, "");
      } catch {
        /* GBK 解码失败保持 UTF-8 结果,由后端校验拦截 */
      }
    }
    setContent(text);
  };

  // 模板下载:表头 + 示例两行(UTF-8 BOM 保证 Excel 识别;状态列值用后端收录的英文词,双语通用)
  const downloadTemplate = () => {
    const en = i18n.language === "en";
    const tpl = en ? [
      "Task,WBS,Duration(days),Start Date,Assignee,Progress(%),Status",
      "Project Kickoff,1,5,2026-08-10,Zhang San;Li Si,50,in_progress",
      "Requirements Survey,1.1,10,2026-08-17,Li Si,50,in_progress",
      "Requirements Review,1.1.1,2,2026-08-31,Li Si,0,open",
      "Launch & Go Live,2,0,2026-10-01,Wang Wu,0,open",
    ].join("\n") : [
      "任务名,WBS编号,工期(天),开始日期,负责人,进度(%),状态",
      "项目立项,1,5,2026-08-10,张三;李四,50,进行中",
      "需求调研,1.1,10,2026-08-17,李四,50,进行中",
      "需求评审,1.1.1,2,2026-08-31,李四,0,未开始",
      "验收上线,2,0,2026-10-01,王五,0,未开始",
    ].join("\n");
    const blob = new Blob(["﻿" + tpl], { type: "text/csv;charset=utf-8" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = i18n.t("importModal.templateName");
    a.click();
    URL.revokeObjectURL(a.href);
  };

  const doImport = async () => {
    if (!content.trim()) return;
    setImporting(true);
    setResult(null);
    try {
      const res = await api.post(`/api/projects/${projectId}/tasks/import`, { csv: content });
      setResult(res.data.data || { imported: 0, skipped: 0, errors: [] });
      onImported(); // 通知父组件刷新甘特图
    } catch (err: any) {
      alert(getErrorMessage(err, "common.unknownError"));
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card" style={{ maxWidth: 520 }} onClick={(e) => e.stopPropagation()}>
        <h2 className="modal-title">{t("importModal.title")}</h2>

        <div className="modal-section-title" style={{ marginTop: 4 }}>{t("importModal.sectionFile")}</div>
        <div style={{ display: "flex", gap: 8, alignItems: "center", marginBottom: 12 }}>
          <input
            ref={fileRef}
            type="file"
            accept=".csv,.txt"
            style={{ display: "none" }}
            onChange={(e) => { const f = e.target.files?.[0]; if (f) handleFile(f); }}
          />
          <button className="btn btn-primary btn-sm" onClick={() => fileRef.current?.click()}>
            {t("importModal.selectFile")}
          </button>
          <span className="text-secondary" style={{ fontSize: 13 }}>{fileName || t("importModal.fileNone")}</span>
        </div>

        <div className="modal-section-title">{t("importModal.sectionFormat")}</div>
        <table style={{ width: "100%", fontSize: 12, borderCollapse: "collapse", marginBottom: 12 }}>
          <thead>
            <tr style={{ color: "var(--text-secondary)", textAlign: "left" }}>
              <th style={{ padding: "2px 6px" }}>{t("importModal.colCol")}</th>
              <th style={{ padding: "2px 6px" }}>{t("importModal.colDesc")}</th>
            </tr>
          </thead>
          <tbody>
            <tr><td style={{ padding: "2px 6px" }}>{t("importModal.rowName")}</td><td style={{ padding: "2px 6px" }}>{t("importModal.rowNameDesc")}</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>{t("importModal.rowWbs")}</td><td style={{ padding: "2px 6px" }}>{t("importModal.rowWbsDesc")}</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>{t("importModal.rowDuration")}</td><td style={{ padding: "2px 6px" }}>{t("importModal.rowDurationDesc")}</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>{t("importModal.rowStart")}</td><td style={{ padding: "2px 6px" }}>{t("importModal.rowStartDesc")}</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>{t("importModal.rowAssignee")}</td><td style={{ padding: "2px 6px" }}>{t("importModal.rowAssigneeDesc")}</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>{t("importModal.rowProgress")}</td><td style={{ padding: "2px 6px" }}>{t("importModal.rowProgressDesc")}</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>{t("importModal.rowStatus")}</td><td style={{ padding: "2px 6px" }}>{t("importModal.rowStatusDesc")}</td></tr>
          </tbody>
        </table>

        <p className="text-secondary" style={{ fontSize: 12, marginBottom: 12 }}>{t("importModal.assigneeHint")}</p>

        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <button className="btn btn-ghost btn-sm" onClick={downloadTemplate}>{t("importModal.downloadTemplate")}</button>
          <div className="modal-actions" style={{ gap: 8 }}>
            <button className="btn btn-ghost btn-sm" onClick={onClose}>{t("importModal.cancel")}</button>
            <button className="btn btn-primary btn-sm" disabled={!content.trim() || importing} onClick={doImport}>
              {importing ? t("importModal.importing") : t("importModal.startImport")}
            </button>
          </div>
        </div>

        {result && (
          <div className="import-result" style={{ marginTop: 12, fontSize: 13 }}>
            <p>
              {t("importModal.imported", { n: result.imported })}
              {result.skipped > 0 && <>{t("importModal.skipped", { n: result.skipped })}</>}
            </p>
            {result.errors.length > 0 && (
              <ul style={{ margin: "6px 0 0", paddingLeft: 18, fontSize: 12, color: "var(--text-secondary)", maxHeight: 140, overflowY: "auto" }}>
                {result.errors.map((e, i) => <li key={i}>{e}</li>)}
              </ul>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
