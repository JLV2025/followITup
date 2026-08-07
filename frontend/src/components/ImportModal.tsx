import { useRef, useState } from "react";
import api from "../api/client";

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

  // 模板下载:表头 + 示例两行(UTF-8 BOM 保证 Excel 识别)
  const downloadTemplate = () => {
    const tpl = [
      "任务名,WBS编号,工期(天),开始日期,负责人,进度(%),状态",
      "项目立项,1,5,2026-08-10,张三,100,已完成",
      "需求调研,1.1,10,2026-08-17,李四,50,进行中",
      "需求评审,1.1.1,2,2026-08-31,李四,0,未开始",
      "验收上线,2,0,2026-10-01,王五,0,未开始",
    ].join("\n");
    const blob = new Blob(["﻿" + tpl], { type: "text/csv;charset=utf-8" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "任务导入模板.csv";
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
      alert(err?.response?.data?.error?.message || "导入失败");
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card" style={{ maxWidth: 520 }} onClick={(e) => e.stopPropagation()}>
        <h2 className="modal-title">导入任务 (CSV)</h2>

        <div className="modal-section-title" style={{ marginTop: 4 }}>1. 选择文件</div>
        <div style={{ display: "flex", gap: 8, alignItems: "center", marginBottom: 12 }}>
          <input
            ref={fileRef}
            type="file"
            accept=".csv,.txt"
            style={{ display: "none" }}
            onChange={(e) => { const f = e.target.files?.[0]; if (f) handleFile(f); }}
          />
          <button className="btn btn-primary btn-sm" onClick={() => fileRef.current?.click()}>
            选择文件
          </button>
          <span className="text-secondary" style={{ fontSize: 13 }}>{fileName || "未选择(支持 UTF-8 / GBK 编码)"}</span>
        </div>

        <div className="modal-section-title">2. 格式说明</div>
        <table style={{ width: "100%", fontSize: 12, borderCollapse: "collapse", marginBottom: 12 }}>
          <thead>
            <tr style={{ color: "var(--text-secondary)", textAlign: "left" }}>
              <th style={{ padding: "2px 6px" }}>列</th>
              <th style={{ padding: "2px 6px" }}>说明</th>
            </tr>
          </thead>
          <tbody>
            <tr><td style={{ padding: "2px 6px" }}>任务名</td><td style={{ padding: "2px 6px" }}>必填</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>WBS编号</td><td style={{ padding: "2px 6px" }}>层级用点分隔:1 / 1.1 / 1.2.1,父须在子之前</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>工期(天)</td><td style={{ padding: "2px 6px" }}>0 或空 = 里程碑;有子任务的父行工期自动按子任务汇总</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>开始日期</td><td style={{ padding: "2px 6px" }}>YYYY-MM-DD,可空(自动排程)</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>负责人</td><td style={{ padding: "2px 6px" }}>可空(默认项目所有者)</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>进度(%)</td><td style={{ padding: "2px 6px" }}>0-100,可空</td></tr>
            <tr><td style={{ padding: "2px 6px" }}>状态</td><td style={{ padding: "2px 6px" }}>未开始/进行中/已完成/延迟,可空(按进度推断)</td></tr>
          </tbody>
        </table>

        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <button className="btn btn-ghost btn-sm" onClick={downloadTemplate}>⬇ 下载模板</button>
          <div className="modal-actions" style={{ gap: 8 }}>
            <button className="btn btn-ghost btn-sm" onClick={onClose}>取消</button>
            <button className="btn btn-primary btn-sm" disabled={!content.trim() || importing} onClick={doImport}>
              {importing ? "导入中..." : "开始导入"}
            </button>
          </div>
        </div>

        {result && (
          <div className="import-result" style={{ marginTop: 12, fontSize: 13 }}>
            <p>
              导入完成:成功 <strong style={{ color: "var(--success)" }}>{result.imported}</strong> 行
              {result.skipped > 0 && <> · 跳过 <strong style={{ color: "var(--danger)" }}>{result.skipped}</strong> 行</>}
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
