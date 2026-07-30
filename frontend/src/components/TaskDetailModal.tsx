import { useState, useEffect } from "react";
import api from "../api/client";

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
  start_date: string;
  end_date: string;
  duration_days: number;
  progress_pct: number;
  actual_start: string;
  actual_end: string;
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
  onClose: () => void;
  onSaved: () => void;
}

const DEP_TYPES = ["FS", "SS", "FF", "SF"];
const STATUSES = ["open", "in_progress", "completed", "delayed"];
const PRIORITIES = ["low", "medium", "high", "critical"];
const STATUS_LABELS: Record<string, string> = {
  open: "待开始", in_progress: "进行中", completed: "已完成", delayed: "已延期",
};
const PRIORITY_LABELS: Record<string, string> = {
  low: "低", medium: "中", high: "高", critical: "紧急",
};

export default function TaskDetailModal({ projectId, task, allTasks, onClose, onSaved }: Props) {
  const isNew = !task;

  // 表单状态
  const [name, setName] = useState("");
  const [parentId, setParentId] = useState<number | null>(null);
  const [taskType, setTaskType] = useState("task");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [duration, setDuration] = useState(1);

  // 现有用户列表（用于负责人下拉）
  const [users, setUsers] = useState<{ name: string }[]>([]);
  const [progress, setProgress] = useState(0);
  const [status, setStatus] = useState("open");
  const [priority, setPriority] = useState("medium");
  const [assignee, setAssignee] = useState("");
  const [manualScheduled, setManualScheduled] = useState(false);
  const [constraintType, setConstraintType] = useState("");
  const [constraintDate, setConstraintDate] = useState("");

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

  // 加载用户列表（用于负责人下拉）
  useEffect(() => {
    api.get("/api/admin/users").then((res) => {
      const list: { name: string }[] = (res.data.data || []).map((u: any) => ({ name: u.display_name || u.email }));
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
      setAssignee(task.assignee || "");
      setManualScheduled(task.manual_scheduled);
      setConstraintType(task.constraint_type || "");
      setConstraintDate(task.constraint_date || "");
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

  /** 快速添加前置任务：解析逗号/分号分隔的 ID 并逐个创建依赖 */
  const handleQuickAddPreds = async () => {
    if (!task || !quickPredIds.trim()) return;
    // 解析 "1,2;3 5" 这样的输入
    const ids = quickPredIds
      .split(/[,;，；\s]+/)
      .map((s) => parseInt(s.trim(), 10))
      .filter((n) => !isNaN(n) && n > 0 && n !== task.id);
    if (ids.length === 0) {
      setError("请输入有效的前置任务序号（不能是自己的 ID）");
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

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      const payload = {
        name: name.trim() || "未命名",
        parent_id: parentId,
        task_type: taskType,
        start_date: startDate,
        end_date: endDate || startDate,
        duration_days: duration,
        progress_pct: progress,
        status,
        priority,
        assignee: assignee.trim(),
        manual_scheduled: manualScheduled,
        constraint_type: constraintType || "",
        constraint_date: constraintDate || "",
        sort_order: task?.sort_order ?? 0,
        version: task?.version ?? 0,
      };

      if (isNew) {
        await api.post(`/api/projects/${projectId}/tasks`, payload);
      } else {
        await api.put(`/api/projects/${projectId}/tasks/${task!.id}`, {
          ...payload,
          id: task!.id,
        });
      }
      onSaved();
    } catch (err: any) {
      if (err.response?.status === 409) {
        setError("任务已被他人修改，请关闭窗口重试");
      } else {
        setError("保存失败，请重试");
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
        <h2 className="modal-title">{isNew ? "新建任务" : "编辑任务"}</h2>

        {error && <div className="form-error">{error}</div>}

        {/* 基本信息 */}
        <div className="form-group">
          <label>任务名称</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
          />
        </div>

        <div className="form-row">
          <div className="form-group">
            <label>父任务</label>
            <div style={{ display: "flex", gap: 6 }}>
              <select
                value={parentId ?? ""}
                onChange={(e) =>
                  setParentId(e.target.value ? Number(e.target.value) : null)
                }
                style={{ flex: 1 }}
              >
                <option value="">无（顶级任务）</option>
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
                    title="缩进 — 设为上一行任务的子任务"
                    type="button"
                  >
                    →
                  </button>
                  <button
                    className="btn-indent-modal"
                    onClick={handleOutdent}
                    disabled={!parentId}
                    title="升级 — 脱离父任务"
                    type="button"
                  >
                    ←
                  </button>
                </>
              )}
            </div>
          </div>
          <div className="form-group">
            <label>任务类型</label>
            <select
              value={taskType}
              onChange={(e) => setTaskType(e.target.value)}
            >
              <option value="task">任务</option>
              <option value="milestone">里程碑</option>
            </select>
          </div>
        </div>

        <hr className="modal-divider" />
        <h4 className="modal-section-title">日期与进度</h4>

        <div className="form-row">
          <div className="form-group">
            <label>开始日期</label>
            <input
              type="date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label>结束日期</label>
            <input
              type="date"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
            />
          </div>
        </div>
        <div className="form-row">
          <div className="form-group">
            <label>工期（工作日）</label>
            <input
              type="number"
              min={1}
              value={duration}
              onChange={(e) => setDuration(Number(e.target.value) || 1)}
            />
          </div>
          <div className="form-group">
            <label>进度 (%)</label>
            <input
              type="number"
              min={0}
              max={100}
              value={progress}
              onChange={(e) => {
                const v = Number(e.target.value);
                setProgress(Math.min(100, Math.max(0, isNaN(v) ? 0 : v)));
              }}
            />
          </div>
        </div>

        <hr className="modal-divider" />
        <h4 className="modal-section-title">状态</h4>

        <div className="form-row">
          <div className="form-group">
            <label>状态</label>
            <select
              value={status}
              onChange={(e) => setStatus(e.target.value)}
            >
              {STATUSES.map((s) => (
                <option key={s} value={s}>
                  {STATUS_LABELS[s]}
                </option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label>优先级</label>
            <select
              value={priority}
              onChange={(e) => setPriority(e.target.value)}
            >
              {PRIORITIES.map((p) => (
                <option key={p} value={p}>
                  {PRIORITY_LABELS[p]}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="form-group">
          <label>负责人</label>
          <input
            value={assignee}
            onChange={(e) => setAssignee(e.target.value)}
            placeholder="输入或选择姓名"
            list="assignee-list"
            autoComplete="off"
          />
          <datalist id="assignee-list">
            {users.map((u, i) => (
              <option key={i} value={u.name} />
            ))}
          </datalist>
        </div>

        {/* 前置任务（编辑模式下可用） */}
        {!isNew && (
          <>
            <hr className="modal-divider" />
            <h4 className="modal-section-title">前置任务</h4>

            {/* 快速添加：逗号/分号分隔多个 ID */}
            <div className="dep-quick-add">
              <label style={{ fontSize: 12, color: "var(--text-secondary)", marginBottom: 4, display: "block" }}>
                快速添加（多个序号用逗号/分号分隔，如 "2, 3, 5"）
              </label>
              <div className="dep-add-row">
                <input
                  type="text"
                  value={quickPredIds}
                  onChange={(e) => setQuickPredIds(e.target.value)}
                  placeholder="输入前置任务序号..."
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
                  placeholder="延迟"
                  style={{ width: 56 }}
                />
                <button
                  className="btn btn-primary btn-sm"
                  onClick={handleQuickAddPreds}
                  disabled={!quickPredIds.trim()}
                >
                  批量添加
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
                  <option value="">逐个选择...</option>
                  {availablePreds.map((t) => (
                    <option key={t.id} value={t.id}>
                      #{t.id} {t.name}
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
                  placeholder="延迟"
                  style={{ width: 56 }}
                />
                <button
                  className="btn btn-primary btn-sm"
                  onClick={handleAddDep}
                  disabled={!newPredId}
                >
                  添加
                </button>
              </div>
            )}

            {/* 已有前置任务列表 */}
            {depLoading && (
              <p style={{ fontSize: 13, color: "var(--text-muted)", marginTop: 8 }}>加载中...</p>
            )}
            {!depLoading && deps.length > 0 && (
              <div className="dep-list">
                {deps.map((d) => {
                  const pred = allTasks.find((t) => t.id === d.predecessor_id);
                  return (
                    <div key={d.id} className="dep-item">
                      <span className="dep-name">
                        {pred ? `#${pred.id} ${pred.name}` : `#${d.predecessor_id}（已删除）`}
                      </span>
                      <span className="dep-type-badge">{d.dep_type}</span>
                      {d.lag_days > 0 && (
                        <span className="dep-lag">+{d.lag_days}d</span>
                      )}
                      <button
                        className="btn-delete-dep"
                        onClick={() => handleRemoveDep(d.id)}
                        title="删除此前置关系"
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
                暂无前置任务
              </p>
            )}
          </>
        )}

        <hr className="modal-divider" />
        <h4 className="modal-section-title">约束</h4>
        <div className="form-group">
          <label className="checkbox-label">
            <input
              type="checkbox"
              checked={manualScheduled}
              onChange={(e) => setManualScheduled(e.target.checked)}
            />
            手动排程（不受依赖关系影响）
          </label>
        </div>
        {!isNew && (
          <div className="form-row">
            <div className="form-group">
              <label>截止日期约束</label>
              <select
                value={constraintType}
                onChange={(e) => setConstraintType(e.target.value)}
              >
                <option value="">无约束</option>
                <option value="finish_no_later_than">不晚于</option>
                <option value="start_no_earlier_than">不早于</option>
              </select>
            </div>
            <div className="form-group">
              <label>约束日期</label>
              <input
                type="date"
                value={constraintDate}
                onChange={(e) => setConstraintDate(e.target.value)}
              />
            </div>
          </div>
        )}

        <div className="modal-actions">
          <button className="btn btn-link" onClick={onClose}>
            取消
          </button>
          <button
            className="btn btn-primary"
            onClick={handleSave}
            disabled={saving}
          >
            {saving ? "保存中..." : isNew ? "创建任务" : "确认修改"}
          </button>
        </div>
      </div>
    </div>
  );
}
