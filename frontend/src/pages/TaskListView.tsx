import { useEffect, useState, useCallback, useRef } from "react";
import { useOutletContext, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import api from "../api/client";
import { useAuthStore } from "../stores/authStore"
import { formatDate } from "../utils/date";
import { statusLabel, priorityLabel } from "../utils/labels";
import i18n from "../i18n";

interface Task {
  id: number;
  project_id: number;
  parent_id: number | null;
  name: string;
  task_type: string;
  status: string;
  priority: string;
  assignee: string;
  start_date: string;
  end_date: string;
  duration_days: number;
  progress_pct: number;
  manual_scheduled: boolean;
  sort_order: number;
  version: number;
}


/** 为任务列表计算每行的可视化深度（递归查找 parent chain） */
function computeDepths(tasks: Task[]): Map<number, number> {
  const parentMap = new Map<number, number | null>();
  tasks.forEach((t) => parentMap.set(t.id, t.parent_id));
  const result = new Map<number, number>();
  const getDepth = (id: number): number => {
    if (result.has(id)) return result.get(id)!;
    const p = parentMap.get(id);
    if (p == null) {
      result.set(id, 0);
      return 0;
    }
    const d = getDepth(p) + 1;
    result.set(id, d);
    return d;
  };
  tasks.forEach((t) => getDepth(t.id));
  return result;
}

export default function TaskListView() {
  const { t: tr } = useTranslation(); // 注意:map 回调参数名为 t(任务对象),翻译函数改名 tr 避免遮蔽
  const { id } = useParams<{ id: string }>();
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingCell, setEditingCell] = useState<{
    taskId: number;
    field: string;
  } | null>(null);
  const [editValue, setEditValue] = useState("");

  const fetchTasks = useCallback(async () => {
    try {
      const res = await api.get(`/api/projects/${id}/tasks`);
      setTasks(res.data.data.tasks || []);
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    fetchTasks();
  }, [fetchTasks]);

  // 项目详情页页首修改项目开始/结束日期后,重新拉取任务数据(后端已全项目重排)
  // 双保险:Outlet context refreshKey + 全局 project-refresh 事件
  const outletCtx = useOutletContext<{ refreshKey?: number }>();
  const prevRefreshKey = useRef(outletCtx?.refreshKey);
  useEffect(() => {
    if (outletCtx?.refreshKey !== undefined && outletCtx.refreshKey !== prevRefreshKey.current) {
      prevRefreshKey.current = outletCtx.refreshKey;
      fetchTasks();
    }
  }, [outletCtx?.refreshKey, fetchTasks]);
  useEffect(() => {
    const onProjectRefresh = (e: Event) => {
      const detail = (e as CustomEvent).detail as { projectId?: number };
      if (detail?.projectId === id) fetchTasks();
    };
    window.addEventListener("project-refresh", onProjectRefresh);
    return () => window.removeEventListener("project-refresh", onProjectRefresh);
  }, [id, fetchTasks]);

  const startEdit = (task: Task, field: string) => {
    if (!isLoggedIn) return;
    setEditingCell({ taskId: task.id, field });
    setEditValue(String((task as any)[field] ?? ""));
  };

  const saveEdit = async () => {
    if (!editingCell) return;
    const task = tasks.find((t) => t.id === editingCell.taskId);
    if (!task) return;

    const updated = { ...task };
    const val = editValue.trim();

    switch (editingCell.field) {
      case "name":
        updated.name = val;
        break;
      case "start_date":
        updated.start_date = val;
        break;
      case "end_date":
        updated.end_date = val;
        break;
      case "duration_days":
        updated.duration_days = parseInt(val) || 1;
        break;
      case "progress_pct":
        updated.progress_pct = Math.min(100, Math.max(0, parseFloat(val) || 0));
        break;
      case "status":
        updated.status = val;
        break;
      case "priority":
        updated.priority = val;
        break;
    }

    try {
      const res = await api.put(
        `/api/projects/${id}/tasks/${task.id}`,
        updated
      );
      setTasks((prev) =>
        prev.map((t) => (t.id === task.id ? res.data.data : t))
      );
    } catch (err: any) {
      if (err.response?.status === 409) {
        alert(i18n.t("taskList.errConflict"));
        fetchTasks();
      }
    }
    setEditingCell(null);
  };

  const addTask = async () => {
    if (!isLoggedIn) return;
    try {
      const res = await api.post(`/api/projects/${id}/tasks`, {
        name: i18n.t("taskList.newTask"),
        task_type: "task",
        status: "open",
        priority: "medium",
        duration_days: 1,
        start_date: new Date().toISOString().slice(0, 10),
        end_date: new Date().toISOString().slice(0, 10),
      });
      setTasks((prev) => [...prev, res.data.data]);
    } catch {
      // ignore
    }
  };

  /** 缩进：将当前任务设为上一行任务的子任务 */
  const indentTask = async (task: Task, index: number) => {
    if (!isLoggedIn || index === 0) return;
    const parent = tasks[index - 1];
    // 防止把自己设为子孙的父任务
    if (parent.parent_id === task.id) return;
    try {
      await api.put(`/api/projects/${id}/tasks/${task.id}`, {
        ...task,
        parent_id: parent.id,
      });
      setTasks((prev) =>
        prev.map((t) => (t.id === task.id ? { ...t, parent_id: parent.id } : t))
      );
    } catch (err: any) {
      if (err.response?.status === 409) {
        alert(i18n.t("taskList.errConflict"));
        fetchTasks();
      }
    }
  };

  /** 升级：将当前任务提升为顶级任务 */
  const outdentTask = async (task: Task) => {
    if (!isLoggedIn || task.parent_id == null) return;
    try {
      await api.put(`/api/projects/${id}/tasks/${task.id}`, {
        ...task,
        parent_id: null,
      });
      setTasks((prev) =>
        prev.map((t) => (t.id === task.id ? { ...t, parent_id: null } : t))
      );
    } catch (err: any) {
      if (err.response?.status === 409) {
        alert(i18n.t("taskList.errConflict"));
        fetchTasks();
      }
    }
  };

  const deleteTask = async (task: Task) => {
    if (!isLoggedIn || !confirm(i18n.t("taskList.confirmDeleteTask", { name: task.name }))) return;
    try {
      await api.delete(`/api/projects/${id}/tasks/${task.id}`);
      setTasks((prev) => prev.filter((t) => t.id !== task.id));
    } catch {
      // ignore
    }
  };

  if (loading) return <p className="text-secondary p-4">{tr("taskList.loading")}</p>;

  const statusColor = (s: string) => {
    switch (s) {
      case "completed":
        return "var(--success)";
      case "delayed":
        return "var(--danger)";
      case "in_progress":
        return "var(--accent)";
      default:
        return "var(--text-muted)";
    }
  };

  const depths = computeDepths(tasks);

  return (
    <div className="task-list-view">
      {/* 工具栏 */}
      {isLoggedIn && (
        <div className="task-toolbar">
          <button className="btn btn-primary btn-sm" onClick={addTask}>
            {tr("taskList.addTask")}
          </button>
          <span className="text-secondary" style={{ fontSize: 12 }}>
            {tr("taskList.toolbarHint")}
          </span>
        </div>
      )}

      {/* 任务表 */}
      <table className="task-table">
        <thead>
          <tr>
            <th style={{ width: 40 }}>#</th>
            <th>{tr("taskList.colName")}</th>
            <th style={{ width: 80 }}>{tr("taskList.colStatus")}</th>
            <th style={{ width: 80 }}>{tr("taskList.colPriority")}</th>
            <th style={{ width: 100 }}>{tr("taskList.colAssignee")}</th>
            <th style={{ width: 60 }}>{tr("taskList.colDuration")}</th>
            <th style={{ width: 110 }}>{tr("taskList.colStart")}</th>
            <th style={{ width: 110 }}>{tr("taskList.colEnd")}</th>
            <th style={{ width: 100 }}>{tr("taskList.colProgress")}</th>
            {isLoggedIn && <th style={{ width: 80 }}></th>}
          </tr>
        </thead>
        <tbody>
          {tasks.map((t, idx) => {
            const depth = depths.get(t.id) || 0;
            return (
              <tr
                key={t.id}
                className={t.status === "delayed" ? "row-delayed" : ""}
              >
                <td className="cell-id">{t.id}</td>
                <td
                  className="cell-name"
                  style={{ paddingLeft: 8 + depth * 20 }}
                >
                  {editingCell?.taskId === t.id &&
                  editingCell.field === "name" ? (
                    <input
                      className="cell-input"
                      value={editValue}
                      onChange={(e) => setEditValue(e.target.value)}
                      onBlur={saveEdit}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") saveEdit();
                        if (e.key === "Escape") setEditingCell(null);
                      }}
                      autoFocus
                    />
                  ) : (
                    <span onClick={() => startEdit(t, "name")} title={tr("taskList.editTitle")}>
                      {depth > 0 && "└ "}
                      {t.task_type === "milestone" && "◆ "}{t.name}
                    </span>
                  )}
                </td>
                <td>
                  <span
                    className="status-badge"
                    style={{
                      background: statusColor(t.status) + "18",
                      color: statusColor(t.status),
                    }}
                  >
                    {statusLabel(t.status)}
                  </span>
                </td>
                <td>{priorityLabel(t.priority)}</td>
                {/* 多负责人后行内编辑体验差，编辑统一走详情弹窗 */}
                <td title={t.assignee}>{t.assignee || "—"}</td>
                <td>{t.duration_days ? `${t.duration_days}d` : "—"}</td>
                <td>{formatDate(t.start_date)}</td>
                <td>{formatDate(t.end_date)}</td>
                <td>
                  <div className="cell-progress">
                    <div className="progress-bar" style={{ width: 60 }}>
                      <div
                        className="progress-fill"
                        style={{ width: `${t.progress_pct}%` }}
                      />
                    </div>
                    <span
                      className="cell-editable"
                      onClick={() => startEdit(t, "progress_pct")}
                    >
                      {Math.round(t.progress_pct)}%
                    </span>
                  </div>
                </td>
                {isLoggedIn && (
                  <td>
                    <button
                      className="btn-indent"
                      onClick={() => indentTask(t, idx)}
                      title={tr("taskList.indentTitle")}
                      disabled={idx === 0 || tasks[idx - 1].parent_id === t.id}
                    >
                      →
                    </button>
                    <button
                      className="btn-indent"
                      onClick={() => outdentTask(t)}
                      title={tr("taskList.outdentTitle")}
                      disabled={t.parent_id == null}
                    >
                      ←
                    </button>
                    <button
                      className="btn-delete"
                      onClick={() => deleteTask(t)}
                      title={tr("taskList.deleteTitle")}
                    >
                      ×
                    </button>
                  </td>
                )}
              </tr>
            );
          })}
          {tasks.length === 0 && (
            <tr>
              <td
                colSpan={isLoggedIn ? 9 : 8}
                className="text-secondary"
                style={{ textAlign: "center", padding: 32 }}
              >
                {tr("taskList.empty")}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
