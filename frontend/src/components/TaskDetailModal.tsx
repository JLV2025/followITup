import { getErrorMessage } from "../utils/errorMsg";
import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import api from "../api/client";
import { formatDateShort } from "../utils/date";
import { statusLabel, priorityLabel } from "../utils/labels";
import i18n from "../i18n";
import MultiUserSelect from "./MultiUserSelect";

interface Task {
  id: number;
  project_id: number;
  parent_id: number | null;
  name: string;
  description: string;
  task_type: string;
  status: string;
  priority: string;
  assignee: string;
  assignee_ids?: number[];
  start_date: string;
  end_date: string;
  duration_days: number;
  progress_pct: number;
  actual_start: string;
  actual_end: string;
  baseline_start_date: string;
  baseline_end_date: string;
  manual_scheduled: boolean;
  constraint_type: string;
  constraint_date: string;
  sort_order: number;
  version: number;
}

interface Dependency {
  id: number;
  predecessor_id: number;
  successor_id: number;
  dep_type: string;
  lag_days: number;
}

interface Props {
  projectId: number;
  task: Task | null; // null = 新建
  allTasks: Task[];
  rowNumbers?: Record<number, number>; // id → 项目内行号（甘特图 # 列顺序）
  onClose: () => void;
  onSaved: () => void;
}

const DEP_TYPES = ["FS", "SS", "FF", "SF"];
const STATUSES = ["open", "in_progress", "completed", "delayed"];
const PRIORITIES = ["low", "medium", "high", "critical"];

export default function TaskDetailModal({ projectId, task, allTasks, rowNumbers, onClose, onSaved }: Props) {
  const { t } = useTranslation();
  const isNew = !task;
  // 父任务：起止日期/工期由子任务自动汇总，不允许直接编辑；也不支持设置前置任务
  const isParent = task ? allTasks.some((t) => t.parent_id === task.id) : false;
  // 行号 → id 反向映射（快速添加输入行号时解析）
  const rowToId: Record<number, number> = {};
  if (rowNumbers) {
    for (const [id, row] of Object.entries(rowNumbers)) rowToId[row] = Number(id);
  }
  const displayNo = (id: number) => rowNumbers?.[id] ?? id;

  // 表单状态
  const [name, setName] = useState("");
  const [parentId, setParentId] = useState<number | null>(null);
  const [taskType, setTaskType] = useState("task");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [duration, setDuration] = useState(1);

  // 现有用户列表（用于负责人下拉）
  const [users, setUsers] = useState<{ id: number; display_name: string }[]>([]);
  const [progress, setProgress] = useState(0);
  const [status, setStatus] = useState("open");
  const [priority, setPriority] = useState("medium");
  const [assigneeIds, setAssigneeIds] = useState<number[]>([]);
  const [manualScheduled, setManualScheduled] = useState(false);
  const [constraintType, setConstraintType] = useState("");
  const [constraintDate, setConstraintDate] = useState("");

  // 实际日期 + 基线偏差
  const [actualStart, setActualStart] = useState(task?.actual_start || "");
  const [actualEnd, setActualEnd] = useState(task?.actual_end || "");
  const baselineStartDate = task?.baseline_start_date || "";
  const baselineEndDate = task?.baseline_end_date || "";
  const diffDays = baselineStartDate && task?.start_date
    ? Math.round((new Date(task.start_date).getTime() - new Date(baselineStartDate).getTime()) / 86400000)
    : 0;

  // 前置任务
  const [deps, setDeps] = useState<Dependency[]>([]);
  const [depLoading, setDepLoading] = useState(false);

  // 快速添加：逗号/分号分隔的 ID
  const [quickPredIds, setQuickPredIds] = useState("");
  const [quickDepType, setQuickDepType] = useState("FS");
  const [quickLag, setQuickLag] = useState(0);

  // 单个添加（下拉选择）
  const [newPredId, setNewPredId] = useState<number | null>(null);
  const [newDepType, setNewDepType] = useState("FS");
  const [newLag, setNewLag] = useState(0);

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  // 加载用户列表（用于负责人下拉，全部登录用户可用的精简端点）
  useEffect(() => {
    api.get("/api/users").then((res) => {
      const list: { id: number; display_name: string }[] =
        (res.data.data || []).map((u: any) => ({ id: u.id, display_name: u.display_name || u.email }));
      setUsers(list);
    }).catch(() => {});
  }, []);

  // 加载任务数据
  useEffect(() => {
    if (task) {
      setName(task.name);
      setParentId(task.parent_id);
      setTaskType(task.task_type);
      setStartDate(task.start_date);
      setEndDate(task.end_date || "");
      setDuration(task.duration_days);
      setProgress(task.progress_pct);
      setStatus(task.status);
      setPriority(task.priority);
      setAssigneeIds(task.assignee_ids || []);
      setManualScheduled(task.manual_scheduled);
      setConstraintType(task.constraint_type || "");
      setConstraintDate(task.constraint_date || "");
      setActualStart(task.actual_start || "");
      setActualEnd(task.actual_end || "");
      loadDeps();
    } else {
      const today = new Date().toISOString().slice(0, 10);
      setStartDate(today);
      setEndDate(today);
      setDuration(1);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [task, projectId]);

  const loadDeps = async () => {
    if (!task) return;
    setDepLoading(true);
    try {
      const res = await api.get(`/api/projects/${projectId}/tasks`);
      const allDeps: Dependency[] = res.data.data.dependencies || [];
      setDeps(allDeps.filter((d) => d.successor_id === task.id));
    } catch { /* ignore */ }
    setDepLoading(false);
  };

  /** 快速添加前置任务：解析逗号/分号分隔的行号并逐个创建依赖 */
  const handleQuickAddPreds = async () => {
    if (!task || !quickPredIds.trim()) return;
    // 解析 "1,2;3 5" 这样的输入（行号 → 数据库 id）
    const ids = quickPredIds
      .split(/[,;，；\s]+/)
      .map((s) => parseInt(s.trim(), 10))
      .filter((n) => !isNaN(n) && n > 0 && n !== displayNo(task.id))
      .map((row) => rowToId[row])
      .filter((id) => id !== undefined && id !== task.id);
    if (ids.length === 0) {
      setError(i18n.t("taskDetail.errQuickPred"));
      return;
    }
    setError("");
    let added = 0;
    for (const predId of ids) {
      // 跳过已有的
      if (deps.some((d) => d.predecessor_id === predId)) continue;
      try {
        await api.post(`/api/projects/${projectId}/dependencies`, {
          predecessor_id: predId,
          successor_id: task.id,
          dep_type: quickDepType,
          lag_days: quickLag,
        });
        added++;
      } catch { /* ignore duplicates */ }
    }
    if (added > 0) {
      setQuickPredIds("");
      loadDeps();
    }
  };

  /** 从下拉选择单个添加 */
  const handleAddDep = async () => {
    if (!newPredId || !task) return;
    try {
      await api.post(`/api/projects/${projectId}/dependencies`, {
        predecessor_id: newPredId,
        successor_id: task.id,
        dep_type: newDepType,
        lag_days: newLag,
      });
      setNewPredId(null);
      setNewLag(0);
      loadDeps();
    } catch { /* ignore */ }
  };

  /** 删除前置关系 */
  const handleRemoveDep = async (depId: number) => {
    if (!task) return;
    try {
      await api.delete(`/api/projects/${projectId}/dependencies/${depId}`);
      setDeps((prev) => prev.filter((d) => d.id !== depId));
    } catch { /* ignore */ }
  };

  /** 删除任务 */
  const handleDelete = async () => {
    if (!task) return;
    const taskName = name || `#${task.id}`;
    if (!confirm(i18n.t("taskDetail.confirmDeleteTask", { name: taskName }))) return;
    setSaving(true);
    try {
      await api.delete(`/api/projects/${projectId}/tasks/${task.id}`);
      onSaved();
    } catch (err: any) {
      setError(getErrorMessage(err, "common.unknownError"));
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");

    // 提交函数：version 由调用方传入（支持 409 后自动重试）
    const submit = async (version: number) => {
      // 状态/进度联动防呆（保存兜底）：已完成↔100%；>0% → 进行中；0% 保持原状态
      let finalStatus = status;
      let finalProgress = progress;
      if (finalStatus === "completed") finalProgress = 100;
      if (finalProgress === 100) finalStatus = "completed";
      else if (finalProgress > 0) finalStatus = "in_progress";
      const payload = {
        name: name.trim() || i18n.t("taskDetail.untitled"),
        parent_id: parentId,
        task_type: taskType,
        start_date: startDate,
        end_date: endDate || startDate,
        duration_days: duration,
        progress_pct: finalProgress,
        status: finalStatus,
        priority,
        assignee_ids: assigneeIds,
        manual_scheduled: manualScheduled,
        constraint_type: constraintType || "",
        constraint_date: constraintDate || "",
        actual_start: actualStart || "",
        actual_end: actualEnd || "",
        sort_order: task?.sort_order ?? 0,
        version,
      };

      if (isNew) {
        await api.post(`/api/projects/${projectId}/tasks`, payload);
      } else {
        await api.put(`/api/projects/${projectId}/tasks/${task!.id}`, {
          ...payload,
          id: task!.id,
        });
      }
    };

    try {
      await submit(task?.version ?? 0);
      onSaved();
    } catch (err: any) {
      // 409 自冲突（如排序保存/其他会话递增了 version）：自动重取最新 version 重放一次，
      // 弹窗内已编辑的字段保持不变——"自己改自己"场景下重试几乎必然成功
      if (err.response?.status === 409 && !isNew) {
        try {
          const res = await api.get(`/api/projects/${projectId}/tasks/${task!.id}`);
          const fresh = res.data?.data;
          if (fresh && typeof fresh.version === "number" && fresh.version !== task!.version) {
            await submit(fresh.version);
            onSaved();
            return;
          }
        } catch { /* 重取失败走下方错误提示 */ }
        setError(i18n.t("taskDetail.errConflict"));
      } else {
        setError(i18n.t("taskDetail.errSave"));
      }
    } finally {
      setSaving(false);
    }
  };

  // 可选的父任务（排除自己）
  const availableParents = allTasks.filter((t) => {
    if (isNew) return t.task_type !== "milestone";
    return t.id !== task!.id && t.task_type !== "milestone";
  });

  // 缩进：设为上一行任务的子任务
  const handleIndent = () => {
    if (!task) return;
    const sorted = [...allTasks].sort((a, b) => a.sort_order - b.sort_order);
    const idx = sorted.findIndex((t) => t.id === task.id);
    if (idx > 0) {
      const prev = sorted[idx - 1];
      if (prev.id !== task.id) setParentId(prev.id);
    }
  };

  // 升级：脱离父任务
  const handleOutdent = () => {
    setParentId(null);
  };

  // 可选的前置任务（排除自己及已有依赖）
  const availablePreds = allTasks.filter((t) => {
    if (isNew) return false;
    return t.id !== task!.id && !deps.some((d) => d.predecessor_id === t.id);
  });

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="modal-card task-detail-modal"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="modal-title">{isNew ? t("taskDetail.titleNew") : t("taskDetail.titleEdit")}</h2>

        {error && <div className="form-error">{error}</div>}

        <div className="task-detail-grid">
        <div className="task-detail-col">
        {/* 左栏：基本信息 / 日期与进度 / 状态 */}
        {/* 基本信息 */}
        <div className="form-group">
          <label>{t("taskDetail.name")}</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
          />
        </div>

        <div className="form-row">
          <div className="form-group">
            <label>{t("taskDetail.parent")}</label>
            <div style={{ display: "flex", gap: 6 }}>
              <select
                value={parentId ?? ""}
                onChange={(e) =>
                  setParentId(e.target.value ? Number(e.target.value) : null)
                }
                style={{ flex: 1 }}
              >
                <option value="">{t("taskDetail.noParent")}</option>
                {availableParents.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name}
                  </option>
                ))}
              </select>
              {!isNew && (
                <>
                  <button
                    className="btn-indent-modal"
                    onClick={handleIndent}
                    title={t("taskDetail.indentTitle")}
                    type="button"
                  >
                    →
                  </button>
                  <button
                    className="btn-indent-modal"
                    onClick={handleOutdent}
                    disabled={!parentId}
                    title={t("taskDetail.outdentTitle")}
                    type="button"
                  >
                    ←
                  </button>
                </>
              )}
            </div>
          </div>
          <div className="form-group">
            <label>{t("taskDetail.taskType")}</label>
            <select
              value={taskType}
              onChange={(e) => setTaskType(e.target.value)}
            >
              <option value="task">{t("taskDetail.typeTask")}</option>
              <option value="milestone">{t("taskDetail.typeMilestone")}</option>
            </select>
          </div>
        </div>

        <hr className="modal-divider" />
        <h4 className="modal-section-title">{t("taskDetail.sectionDates")}</h4>

        <div className="form-row">
          <div className="form-group">
            <label>{t("taskDetail.startAuto")}</label>
            <input type="date" value={startDate} disabled />
          </div>
          <div className="form-group">
            <label>{t("taskDetail.endAuto")}</label>
            <input type="date" value={endDate} disabled />
          </div>
        </div>
        <div className="form-row">
          <div className="form-group">
            <label>{t("taskDetail.actualStart")}</label>
            <input type="date" value={actualStart} onChange={(e) => setActualStart(e.target.value)} />
          </div>
          <div className="form-group">
            <label>{t("taskDetail.actualEnd")}</label>
            <input type="date" value={actualEnd} onChange={(e) => setActualEnd(e.target.value)} />
          </div>
        </div>
        {baselineStartDate && (
          <div className="form-row baseline-diff-row">
            <span className="baseline-diff">
              基线: {formatDateShort(baselineStartDate)} ~ {formatDateShort(baselineEndDate)}
              <em className={`baseline-diff-badge ${diffDays > 0 ? "neg" : diffDays < 0 ? "pos" : ""}`}>
                &Delta; {diffDays > 0 ? `+${diffDays}` : diffDays} {t("taskDetail.diffDays")}
              </em>
            </span>
          </div>
        )}
        <div className="form-row">
          <div className="form-group">
            <label>{t("taskDetail.duration")}</label>
            <input
              type="number"
              min={1}
              value={duration}
              onChange={(e) => setDuration(Number(e.target.value) || 1)}
              disabled={isParent}
            />
          </div>
          <div className="form-group">
            <label>{t("taskDetail.progress")}</label>
            <input
              type="number"
              min={0}
              max={100}
              value={progress}
              onChange={(e) => {
                const v = Number(e.target.value);
                const p = Math.min(100, Math.max(0, isNaN(v) ? 0 : v));
                setProgress(p);
                // 联动防呆：100% ↔ 已完成；>0% → 进行中；0% → 待开始
                setStatus(p === 100 ? "completed" : p > 0 ? "in_progress" : "open");
              }}
            />
          </div>
        </div>

        <hr className="modal-divider" />
        <h4 className="modal-section-title">{t("taskDetail.sectionStatus")}</h4>

        <div className="form-row">
          <div className="form-group">
            <label>{t("taskDetail.status")}</label>
            <select
              value={status}
              onChange={(e) => {
                const s = e.target.value;
                setStatus(s);
                // 联动防呆：状态改为已完成 → 进度自动 100%
                if (s === "completed") setProgress(100);
              }}
            >
              {STATUSES.map((s) => (
                <option key={s} value={s}>
                  {statusLabel(s)}
                </option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label>{t("taskDetail.priority")}</label>
            <select
              value={priority}
              onChange={(e) => setPriority(e.target.value)}
            >
              {PRIORITIES.map((p) => (
                <option key={p} value={p}>
                  {priorityLabel(p)}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="form-group">
          <label>{t("taskDetail.assignee")}</label>
          <MultiUserSelect
            users={users}
            selectedIds={assigneeIds}
            onChange={setAssigneeIds}
          />
        </div>
        </div>{/* 左栏结束 */}

        <div className="task-detail-col">
        {/* 右栏：前置任务 / 约束 */}
        {/* 前置任务（编辑模式下可用） */}
        {!isNew && (
          <>
            <hr className="modal-divider" />
            <h4 className="modal-section-title">{t("taskDetail.sectionPred")}</h4>

            {isParent ? (
              <p style={{ fontSize: 12, color: "var(--text-muted)", marginTop: 6 }}>
                {t("taskDetail.parentNoPred")}
              </p>
            ) : (
            <>
            {/* 快速添加：逗号/分号分隔多个行号 */}
            <div className="dep-quick-add">
              <label style={{ fontSize: 12, color: "var(--text-secondary)", marginBottom: 4, display: "block" }}>
                {t("taskDetail.quickAddHint")}
              </label>
              <div className="dep-add-row">
                <input
                  type="text"
                  value={quickPredIds}
                  onChange={(e) => setQuickPredIds(e.target.value)}
                  placeholder={t("taskDetail.predPlaceholder")}
                  style={{ flex: 2 }}
                />
                <select
                  value={quickDepType}
                  onChange={(e) => setQuickDepType(e.target.value)}
                  style={{ width: 72 }}
                >
                  {DEP_TYPES.map((dt) => (
                    <option key={dt} value={dt}>{dt}</option>
                  ))}
                </select>
                <input
                  type="number"
                  value={quickLag}
                  onChange={(e) => setQuickLag(Number(e.target.value))}
                  min={0}
                  placeholder={t("taskDetail.lagPlaceholder")}
                  style={{ width: 56 }}
                />
                <button
                  className="btn btn-primary btn-sm"
                  onClick={handleQuickAddPreds}
                  disabled={!quickPredIds.trim()}
                >
                  {t("taskDetail.batchAdd")}
                </button>
              </div>
            </div>

            {/* 单个添加（下拉） */}
            {availablePreds.length > 0 && (
              <div className="dep-add-row" style={{ marginTop: 8 }}>
                <select
                  value={newPredId ?? ""}
                  onChange={(e) =>
                    setNewPredId(e.target.value ? Number(e.target.value) : null)
                  }
                  style={{ flex: 2 }}
                >
                  <option value="">{t("taskDetail.predSelect")}</option>
                  {availablePreds.map((t) => (
                    <option key={t.id} value={t.id}>
                      #{displayNo(t.id)} {t.name}
                    </option>
                  ))}
                </select>
                <select
                  value={newDepType}
                  onChange={(e) => setNewDepType(e.target.value)}
                  style={{ width: 72 }}
                >
                  {DEP_TYPES.map((dt) => (
                    <option key={dt} value={dt}>{dt}</option>
                  ))}
                </select>
                <input
                  type="number"
                  value={newLag}
                  onChange={(e) => setNewLag(Number(e.target.value))}
                  min={0}
                  placeholder={t("taskDetail.lagPlaceholder")}
                  style={{ width: 56 }}
                />
                <button
                  className="btn btn-primary btn-sm"
                  onClick={handleAddDep}
                  disabled={!newPredId}
                >
                  {t("taskDetail.add")}
                </button>
              </div>
            )}

            {/* 已有前置任务列表 */}
            {depLoading && (
              <p style={{ fontSize: 13, color: "var(--text-muted)", marginTop: 8 }}>{t("common.loading")}</p>
            )}
            {!depLoading && deps.length > 0 && (
              <div className="dep-list">
                {deps.map((d) => {
                  const pred = allTasks.find((t) => t.id === d.predecessor_id);
                  return (
                    <div key={d.id} className="dep-item">
                      <span className="dep-name">
                        {pred ? `#${displayNo(pred.id)} ${pred.name}` : i18n.t("taskDetail.deletedPred", { n: displayNo(d.predecessor_id) })}
                      </span>
                      <span className="dep-type-badge">{d.dep_type}</span>
                      {d.lag_days > 0 && (
                        <span className="dep-lag">+{d.lag_days}d</span>
                      )}
                      <button
                        className="btn-delete-dep"
                        onClick={() => handleRemoveDep(d.id)}
                        title={t("taskDetail.removePredTitle")}
                      >
                        ×
                      </button>
                    </div>
                  );
                })}
              </div>
            )}
            {!depLoading && deps.length === 0 && (
              <p style={{ fontSize: 13, color: "var(--text-muted)", marginTop: 4 }}>
                {t("taskDetail.noPreds")}
              </p>
            )}
            </>
            )}
          </>
        )}

        <hr className="modal-divider" />
        <h4 className="modal-section-title">{t("taskDetail.sectionConstraint")}</h4>
        <div className="form-group">
          <label className="checkbox-label">
            <input
              type="checkbox"
              checked={manualScheduled}
              onChange={(e) => setManualScheduled(e.target.checked)}
            />
            {t("taskDetail.manualScheduled")}
          </label>
        </div>
        {!isNew && (
          <div className="form-row">
            <div className="form-group">
              <label>{t("taskDetail.constraintDeadline")}</label>
              <select
                value={constraintType}
                onChange={(e) => setConstraintType(e.target.value)}
              >
                <option value="">{t("taskDetail.noConstraint")}</option>
                <option value="finish_no_later_than">{t("taskDetail.constraintNoLater")}</option>
                <option value="start_no_earlier_than">{t("taskDetail.constraintNoEarlier")}</option>
              </select>
            </div>
            <div className="form-group">
              <label>{t("taskDetail.constraintDate")}</label>
              <input
                type="date"
                value={constraintDate}
                onChange={(e) => setConstraintDate(e.target.value)}
              />
            </div>
          </div>
        )}
        </div>{/* 右栏结束 */}
        </div>{/* task-detail-grid 结束 */}

        <div className="modal-actions">
          {!isNew && (
            <button
              className="btn btn-delete-task"
              onClick={handleDelete}
              disabled={saving}
            >
              {saving ? t("taskDetail.deleting") : t("taskDetail.deleteTask")}
            </button>
          )}
          <div className="modal-actions-right">
            <button className="btn btn-link" onClick={onClose}>
              {t("common.cancel")}
            </button>
            <button
              className="btn btn-primary"
              onClick={handleSave}
              disabled={saving}
            >
              {saving ? t("taskDetail.saving") : isNew ? t("taskDetail.createTask") : t("taskDetail.saveEdit")}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
