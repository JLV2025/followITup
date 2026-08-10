import { getErrorMessage } from "../utils/errorMsg";
import { useEffect, useState } from "react";
import api from "../api/client";

interface DeletedTask {
  id: number;
  name: string;
  task_type: string;
  start_date: string;
  end_date: string;
  duration_days: number;
  progress_pct: number;
  status: string;
  assignee: string;
  sort_order: number;
  deleted_at: string;
}

interface Props {
  projectId: number;
  projectName: string;
  onClose: () => void;
  onRestored: () => void;
}

export default function RecycleBinModal({ projectId, projectName, onClose, onRestored }: Props) {
  const [tasks, setTasks] = useState<DeletedTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [restoringId, setRestoringId] = useState<number | null>(null);

  const fetchDeleted = async () => {
    try {
      const res = await api.get(`/api/projects/${projectId}/tasks/deleted`);
      setTasks(res.data.data || []);
    } catch {
      setError("加载已删除任务失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDeleted();
  }, [projectId]);

  const handleRestore = async (task: DeletedTask) => {
    setRestoringId(task.id);
    try {
      await api.post(`/api/projects/${projectId}/tasks/${task.id}/restore`);
      await fetchDeleted();
      onRestored();
      alert(`「${task.name}」已恢复，任务回到甘特图（显式依赖需手动重连）`);
    } catch (err: any) {
      setError(getErrorMessage(err, "common.unknownError"));
    } finally {
      setRestoringId(null);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-title">
          <h2>回收站 · {projectName}</h2>
          <button className="btn btn-ghost btn-sm" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          {error && <div className="form-error">{error}</div>}
          {loading ? (
            <p className="text-secondary">加载中...</p>
          ) : tasks.length === 0 ? (
            <p className="text-secondary">没有已删除的任务</p>
          ) : (
            <div className="dep-list">
              {tasks.map((t) => (
                <div className="dep-item" key={t.id}>
                  <div className="dep-item-main">
                    <span className="dep-item-name">
                      {t.name}
                      {t.task_type === "milestone" && <em className="tag">里程碑</em>}
                    </span>
                    <span className="dep-item-detail">
                      {t.start_date || "—"} ~ {t.end_date || "—"} · {t.duration_days}天 · 进度 {t.progress_pct}% · 原排序 #{t.sort_order + 1}
                    </span>
                    <span className="dep-item-detail">删除于 {t.deleted_at?.slice(0, 10)}</span>
                  </div>
                  <button
                    className="btn btn-primary btn-sm"
                    disabled={restoringId === t.id}
                    onClick={() => handleRestore(t)}
                  >
                    {restoringId === t.id ? "恢复中..." : "恢复"}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="modal-actions">
          <button className="btn btn-ghost" onClick={onClose}>关闭</button>
        </div>
      </div>
    </div>
  );
}
