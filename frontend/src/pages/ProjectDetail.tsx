import { useEffect, useState } from "react";
import { useParams, Link, NavLink, Outlet } from "react-router-dom";
import api from "../api/client";

interface Project {
  id: number;
  name: string;
  description: string;
  owner: string;
  start_date: string;
  end_date: string;
  status: string;
  schedule_direction: string;
}

export default function ProjectDetail() {
  const { id } = useParams<{ id: string }>();
  const [project, setProject] = useState<Project | null>(null);
  const [hasProgress, setHasProgress] = useState(false);
  // 用户列表（所有者下拉，可手输兜底）
  const [userOptions, setUserOptions] = useState<{ display_name: string; email: string }[]>([]);

  useEffect(() => {
    api.get(`/api/projects/${id}`).then((res) => setProject(res.data.data));
    // 并行请求任务列表，检查是否有进度 > 0 的任务
    api.get(`/api/projects/${id}/tasks`).then((res) => {
      const tasks: any[] = res.data?.data?.tasks || [];
      setHasProgress(tasks.some((t: any) => (t.progress_pct || 0) > 0));
    });
    // 用户列表
    api.get("/api/users").then((res) => setUserOptions(res.data.data || [])).catch(() => {});
  }, [id]);

  // 保存项目开始/结束日期（后端会触发全项目重排）
  const saveDate = async (field: "start_date" | "end_date", value: string) => {
    if (!project) return;
    try {
      await api.put(`/api/projects/${id}`, { ...project, [field]: value });
      // 后端已全项目重排并落库。整页重载加载最新数据——最可靠，
      // 不依赖任何前端事件/路由机制（改日期是低频操作，重载无感知）
      window.location.reload();
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
        {/* 项目所有者：仅可挑选系统用户（邮件通知用）；修改后未开始任务自动改派 */}
        <label className="direction-date direction-owner">
          项目所有者
          <select
            value={project.owner}
            onChange={async (e) => {
              const owner = e.target.value;
              try {
                await api.put(`/api/projects/${id}`, { ...project, owner });
                setProject({ ...project, owner });
              } catch (err: any) {
                alert(err?.response?.data?.error?.message || "所有者修改失败");
              }
            }}
          >
            {userOptions.map((u) => (
              <option key={u.email} value={u.display_name || u.email}>
                {u.display_name || u.email}
              </option>
            ))}
          </select>
        </label>
      </div>

      {/* 视图切换 */}
      <div className="project-tabs">
        <NavLink to={`/project/${id}`} end className={({ isActive }) => `project-tab${isActive ? " active" : ""}`}>
          甘特图
        </NavLink>
        <NavLink to={`/project/${id}/resources`} className={({ isActive }) => `project-tab${isActive ? " active" : ""}`}>
          资源视图
        </NavLink>
      </div>

      {/* 子路由内容 */}
      <Outlet context={{ project }} />
    </div>
  );
}
