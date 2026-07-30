import { useEffect, useState } from "react";
import { useParams, Link, Outlet, useLocation } from "react-router-dom";
import api from "../api/client";
import { useAuthStore } from "../stores/authStore";

interface Project {
  id: number;
  name: string;
  description: string;
  start_date: string;
  end_date: string;
  status: string;
}

export default function ProjectDetail() {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const [project, setProject] = useState<Project | null>(null);

  useEffect(() => {
    api.get(`/api/projects/${id}`).then((res) => setProject(res.data.data));
  }, [id]);

  if (!project) return <p className="text-secondary">加载中...</p>;

  const currentTab = location.pathname.endsWith("/list") ? "list" : "gantt";

  return (
    <div className="project-detail">
      {/* 项目头部 */}
      <div className="project-header">
        <Link to="/" className="back-link">← 看板</Link>
        <div>
          <h1>{project.name}</h1>
          <p className="text-secondary">{project.description}</p>
        </div>
        <div className="project-tabs">
          <Link
            to={`/project/${id}`}
            className={`tab ${currentTab === "gantt" ? "tab-active" : ""}`}
          >
            甘特图
          </Link>
          <Link
            to={`/project/${id}/list`}
            className={`tab ${currentTab === "list" ? "tab-active" : ""}`}
          >
            任务列表
          </Link>
        </div>
      </div>

      {/* 子路由内容 */}
      <Outlet context={{ project, isLoggedIn }} />
    </div>
  );
}
