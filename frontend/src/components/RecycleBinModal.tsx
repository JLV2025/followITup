import { getErrorMessage } from "../utils/errorMsg";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import api from "../api/client";
import i18n from "../i18n";

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
  const { t } = useTranslation();
  const [tasks, setTasks] = useState<DeletedTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [restoringId, setRestoringId] = useState<number | null>(null);

  const fetchDeleted = async () => {
    try {
      const res = await api.get(`/api/projects/${projectId}/tasks/deleted`);
      setTasks(res.data.data || []);
    } catch {
      setError(i18n.t("recycleBin.errLoad"));
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
      alert(i18n.t("recycleBin.restored", { name: task.name }));
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
          <h2>{t("recycleBin.title", { name: projectName })}</h2>
          <button className="btn btn-ghost btn-sm" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          {error && <div className="form-error">{error}</div>}
          {loading ? (
            <p className="text-secondary">{t("common.loading")}</p>
          ) : tasks.length === 0 ? (
            <p className="text-secondary">{t("recycleBin.empty")}</p>
          ) : (
            <div className="dep-list">
              {tasks.map((dt) => (
                <div className="dep-item" key={dt.id}>
                  <div className="dep-item-main">
                    <span className="dep-item-name">
                      {dt.name}
                      {dt.task_type === "milestone" && <em className="tag">{t("recycleBin.milestoneTag")}</em>}
                    </span>
                    <span className="dep-item-detail">
                      {dt.start_date || "—"} ~ {dt.end_date || "—"} · {dt.duration_days}{t("recycleBin.days")} · {t("recycleBin.progress")} {dt.progress_pct}% · {t("recycleBin.origOrder")} #{dt.sort_order + 1}
                    </span>
                    <span className="dep-item-detail">{t("recycleBin.deletedAt", { date: dt.deleted_at?.slice(0, 10) })}</span>
                  </div>
                  <button
                    className="btn btn-primary btn-sm"
                    disabled={restoringId === dt.id}
                    onClick={() => handleRestore(dt)}
                  >
                    {restoringId === dt.id ? t("recycleBin.restoring") : t("recycleBin.restore")}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="modal-actions">
          <button className="btn btn-ghost" onClick={onClose}>{t("common.close")}</button>
        </div>
      </div>
    </div>
  );
}
