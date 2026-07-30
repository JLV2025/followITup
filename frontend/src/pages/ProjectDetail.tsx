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
}

export default function ProjectDetail() {
  const { id } = useParams<{ id: string }>();
  const [project, setProject] = useState<Project | null>(null);

  useEffect(() => {
    api.get(`/api/projects/${id}`).then((res) => setProject(res.data.data));
  }, [id]);

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

      {/* 子路由内容 */}
      <Outlet context={{ project }} />
    </div>
  );
}
