import { useEffect, useState } from "react";
import { useParams, Link, Outlet } from "react-router-dom";
import api from "../api/client";

interface Project {
  id: number;
  name: string;
  description: string;
  start_date: string;
  end_date: string;
  status: string;
  schedule_direction: string;
}

export default function ProjectDetail() {
  const { id } = useParams<{ id: string }>();
  const [project, setProject] = useState<Project | null>(null);
  const [hasProgress, setHasProgress] = useState(false);
  // 项目开始/结束日期变化时递增，通知甘特图等子路由重新拉取任务数据
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    api.get(`/api/projects/${id}`).then((res) => setProject(res.data.data));
    // 并行请求任务列表，检查是否有进度 > 0 的任务
    api.get(`/api/projects/${id}/tasks`).then((res) => {
      const tasks: any[] = res.data?.data?.tasks || [];
      setHasProgress(tasks.some((t: any) => (t.progress_pct || 0) > 0));
    });
  }, [id]);

  // 保存项目开始/结束日期（后端会触发全项目重排）
  const saveDate = async (field: "start_date" | "end_date", value: string) => {
    if (!project) return;
    try {
      await api.put(`/api/projects/${id}`, { ...project, [field]: value });
      setProject({ ...project, [field]: value });
      setRefreshKey((k) => k + 1); // 通知子路由刷新（重排已落库）
    } catch (err: any) {
      alert(err?.response?.data?.error?.message || "项目日期更新失败");
    }
  };

  if (!project) return <p className="text-secondary">加载中...</p>;

  return (
    <div className="project-detail">
      {/* 项目头部 */}
      <div className="project-header">
        <Link to="/" className="back-link">← 看板</Link>
        <div>
          <h1>{project.name}</h1>
          <p className="text-secondary">{project.description}</p>
        </div>
      </div>

      {/* 排程方向 */}
      <div className="project-direction-row">
        {project.schedule_direction === "backward" ? (
          <span className="badge badge-blue">倒排（基于完成日期）</span>
        ) : (
          <span className="badge">正排（基于开始日期）</span>
        )}
        <select
          className="direction-select"
          value={project.schedule_direction}
          disabled={hasProgress}
          onChange={async (e) => {
            const dir = e.target.value;
            try {
              await api.put(`/api/projects/${id}`, { ...project, schedule_direction: dir });
              setProject({ ...project, schedule_direction: dir });
            } catch (err: any) {
              alert(err?.response?.data?.error?.message || "排程方向修改失败");
              setProject({ ...project }); // 回弹原值
            }
          }}
        >
          <option value="forward">正排</option>
          <option value="backward">倒排</option>
        </select>
        {hasProgress && (
          <span className="direction-hint">项目已有任务进度，排程方向不可修改</span>
        )}
        {/* 项目锚点日期：正排编辑开始日期，倒排编辑结束日期（保存后全项目重排） */}
        {project.schedule_direction === "forward" ? (
          <label className="direction-date">
            项目开始
            <input
              type="date"
              value={project.start_date || ""}
              onChange={(e) => saveDate("start_date", e.target.value)}
            />
          </label>
        ) : (
          <label className="direction-date">
            项目结束
            <input
              type="date"
              value={project.end_date || ""}
              onChange={(e) => saveDate("end_date", e.target.value)}
            />
          </label>
        )}
      </div>

      {/* 子路由内容 */}
      <Outlet context={{ project, refreshKey }} />
    </div>
  );
}
