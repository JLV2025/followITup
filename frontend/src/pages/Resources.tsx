import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import api from "../api/client";
import { formatDate } from "../utils/date";

interface TaskRow {
  id: number;
  name: string;
  status: string;
  assignee: string;
  start_date: string;
  end_date: string;
  duration_days: number;
  progress_pct: number;
  task_type: string;
  parent_id: number | null;
}

/** 资源视图：按负责人分组汇总任务（叶子任务计入，父任务由子任务汇总不重复） */
export default function Resources() {
  const { id } = useParams<{ id: string }>();
  const [tasks, setTasks] = useState<TaskRow[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.get(`/api/projects/${id}/tasks`)
      .then((res) => { setTasks(res.data?.data?.tasks || []); setLoading(false); })
      .catch(() => setLoading(false));
  }, [id]);

  // 叶子 = 无子任务（顶层父任务如 WBS "1" 有子任务，不算叶子，否则其子任务被隐藏且计数/工时失真）
  const isParent = (id: number) => tasks.some((c) => c.parent_id === id);
  const isLeaf = (t: TaskRow) => !isParent(t.id);

  // 分组：负责人 → 叶子任务列表
  const groups = new Map<string, TaskRow[]>();
  for (const t of tasks) {
    if (!isLeaf(t)) continue; // 父任务不重复计入
    const key = t.assignee || "未分配";
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(t);
  }
  const sorted = Array.from(groups.entries()).sort((a, b) => a[0].localeCompare(b[0], "zh"));

  const statusText = (s: string) => ({ open: "未开始", in_progress: "进行中", completed: "已完成", delayed: "延迟" } as Record<string, string>)[s] || s || "未开始";
  const statusColor = (t: TaskRow) => {
    if (t.status === "completed") return "var(--success)";
    if (t.status === "delayed") return "var(--danger)";
    if (t.progress_pct > 0) return "var(--accent)";
    return "var(--text-muted)";
  };

  return (
    <div className="resources">
      <div className="section-title-row" style={{ marginBottom: 12 }}>
        <h2 className="section-title">资源视图</h2>
        <span className="text-secondary" style={{ fontSize: 13 }}>
          共 {sorted.length} 位负责人 · {tasks.filter(isLeaf).length} 个叶子任务
        </span>
      </div>

      {loading ? (
        <p className="text-secondary">加载中...</p>
      ) : sorted.length === 0 ? (
        <p className="text-secondary">暂无任务</p>
      ) : (
        <div className="resource-grid">
          {sorted.map(([owner, list]) => {
            const totalDays = list.reduce((s, t) => s + (t.duration_days || 0), 0);
            return (
              <div key={owner} className="resource-card">
                <div className="resource-card-header">
                  <span className="resource-avatar">👤</span>
                  <span className="resource-name">{owner}</span>
                  <span className="resource-summary">{list.length} 项 · {totalDays}d</span>
                </div>
                <div className="resource-tasks">
                  {list.map((t) => (
                    <div key={t.id} className="resource-task">
                      <div className="resource-task-row">
                        <span className="resource-task-name" title={t.name}>
                          {t.task_type === "milestone" ? "◆ " : ""}{t.name}
                        </span>
                        <span className="resource-task-status" style={{ color: statusColor(t) }}>
                          {statusText(t.status)}
                        </span>
                      </div>
                      <div className="resource-task-row">
                        <span className="text-secondary" style={{ fontSize: 11 }}>
                          {formatDate(t.start_date)} ~ {formatDate(t.end_date)}
                        </span>
                        <span className="text-secondary" style={{ fontSize: 11 }}>
                          {Math.round(t.progress_pct)}%
                        </span>
                      </div>
                      <div className="progress-bar" style={{ height: 5 }}>
                        <div
                          className="progress-fill"
                          style={{ width: `${t.progress_pct}%`, background: statusColor(t) }}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
