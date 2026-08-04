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
      </div>

      {/* 子路由内容 */}
      <Outlet context={{ project }} />
    </div>
  );
}
