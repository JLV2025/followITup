import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useAuthStore } from "../stores/authStore";
import { useDashboardStore } from "../stores/dashboardStore";
import { useSettingsStore, availableCalendarYears, availableFiscalYears, fiscalYearLabel, currentFiscalYear } from "../stores/settingsStore";
import api from "../api/client";

export default function Dashboard() {
  const user = useAuthStore((s) => s.user);
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const loadFromStorage = useAuthStore((s) => s.loadFromStorage);

  const { stats, projects, period, loading, fetchStats, fetchProjects, setPeriod } =
    useDashboardStore();

  const { displayMode, fiscalStartMonth, setDisplayMode, setFiscalStartMonth } = useSettingsStore();

  // 创建项目模态框状态
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createForm, setCreateForm] = useState({ name: "", start_date: "", end_date: "", schedule_direction: "forward", description: "" });
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [createError, setCreateError] = useState("");

  // 回收站模态框状态
  const [showRecycleBin, setShowRecycleBin] = useState(false);
  const [deletedProjects, setDeletedProjects] = useState<any[]>([]);

  useEffect(() => {
    loadFromStorage();
    if (displayMode === "fiscal") {
      const current = useDashboardStore.getState();
      if (current.period >= 2000) {
        setPeriod(currentFiscalYear(fiscalStartMonth));
      }
    }
    fetchStats();
    fetchProjects();
  }, [loadFromStorage, fetchStats, fetchProjects, displayMode, fiscalStartMonth, setPeriod]);

  const periods = displayMode === "fiscal"
    ? availableFiscalYears(fiscalStartMonth)
    : availableCalendarYears(5);

  const periodLabel = (p: number) =>
    displayMode === "fiscal" ? fiscalYearLabel(p) : `${p} 年`;

  const handleToggleMode = () => {
    const nextMode = displayMode === "fiscal" ? "calendar" : "fiscal";
    setDisplayMode(nextMode);
    if (nextMode === "fiscal") {
      setPeriod(currentFiscalYear(fiscalStartMonth));
    } else {
      setPeriod(new Date().getFullYear());
    }
  };

  const handleOpenCreate = () => {
    setCreateForm({ name: "", start_date: "", end_date: "", schedule_direction: "forward", description: "" });
    setCreateError("");
    setShowCreateModal(true);
  };

  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!createForm.name.trim()) {
      setCreateError("项目名称不能为空");
      return;
    }
    setCreateSubmitting(true);
    setCreateError("");
    try {
      await api.post("/api/projects", {
        name: createForm.name.trim(),
        description: createForm.description.trim(),
        start_date: createForm.start_date || null,
        end_date: createForm.end_date || null,
        schedule_direction: createForm.schedule_direction,
      });
      setShowCreateModal(false);
      fetchProjects();
      fetchStats();
    } catch (err: any) {
      setCreateError(err.response?.data?.error?.message || "创建失败，请重试");
    } finally {
      setCreateSubmitting(false);
    }
  };

  const statusColor = (p: { has_risk: boolean; status: string; progress: number }) => {
    if (p.has_risk) return "var(--danger)";
    if (p.status === "completed" || p.status === "archived") return "var(--text-muted)";
    if (p.progress >= 80) return "var(--success)";
    if (p.progress >= 30) return "var(--accent)";
    return "var(--text-secondary)";
  };

  const statRiskClass = (stats?.at_risk ?? 0) > 0 ? "text-danger pulse-once" : "text-success";

  return (
    <div className="dashboard">
      {/* 头部 */}
      <div className="dashboard-header">
        <div className="dashboard-header-row">
          <div>
            <h1>项目看板</h1>
            {isLoggedIn ? (
              <p className="text-secondary">
                欢迎回来，{user?.display_name || user?.email}
              </p>
            ) : (
              <p className="text-secondary">
                只读模式 — <Link to="/login">登录</Link> 后可编辑
              </p>
            )}
          </div>
          {isLoggedIn && (
            <div style={{ display: "flex", gap: 8 }}>
              <button className="btn btn-primary" onClick={handleOpenCreate}>
                + 创建项目
              </button>
              <button
                className="btn btn-ghost"
                onClick={async () => {
                  try {
                    const res = await api.get("/api/projects?deleted=1");
                    setDeletedProjects(res.data.data || []);
                    setShowRecycleBin(true);
                  } catch (err: any) {
                    alert(err?.response?.data?.error?.message || "加载回收站失败");
                  }
                }}
              >
                回收站
              </button>
            </div>
          )}
        </div>
      </div>

      {/* 统计卡片 */}
      <div className="stats-row">
        <div className="stat-card">
          <span className="stat-label">活跃项目</span>
          <span className="stat-value">{stats?.active_projects ?? 0}</span>
        </div>
        <div className="stat-card">
          <span className="stat-label">⚠ 有风险</span>
          <span className={`stat-value ${statRiskClass}`}>
            {stats?.at_risk ?? 0}
          </span>
        </div>
        <div className="stat-card">
          <span className="stat-label">本周到期</span>
          <span className="stat-value">{stats?.due_this_week ?? 0}</span>
        </div>
        <div className="stat-card">
          <span className="stat-label">整体完成</span>
          <span className="stat-value">
            {stats?.overall_progress ?? 0}%
          </span>
          {stats && stats.has_baseline && (
            <div className={`stat-delta ${stats.overall_progress - stats.baseline_progress >= 0 ? "pos" : "neg"}`}>
              Δ {stats.overall_progress - stats.baseline_progress >= 0 ? "+" : ""}
              {Math.round(stats.overall_progress - stats.baseline_progress)}%
            </div>
          )}
          <div className="stat-progress-ring">
            <svg width="48" height="48" viewBox="0 0 48 48">
              <circle cx="24" cy="24" r="20" fill="none" stroke="var(--bg-light)" strokeWidth="4" />
              <circle
                cx="24" cy="24" r="20"
                fill="none"
                stroke={((stats?.overall_progress ?? 0) >= 80) ? "var(--success)" : "var(--accent)"}
                strokeWidth="4"
                strokeDasharray={`${((stats?.overall_progress ?? 0) / 100) * 126} 126`}
                strokeLinecap="round"
                transform="rotate(-90 24 24)"
              />
            </svg>
          </div>
        </div>
      </div>

      {/* 年度切换 + 模式开关 */}
      <div className="year-selector">
        <div className="year-selector-tabs">
          {periods.map((p) =>
            p === period ? (
              <span key={p} className="year-active">
                ● {periodLabel(p)}
              </span>
            ) : (
              <span key={p} className="year-inactive" onClick={() => setPeriod(p)}>
                {periodLabel(p)}
              </span>
            )
          )}
        </div>
        <button
          className="btn btn-sm btn-ghost"
          onClick={handleToggleMode}
          title={displayMode === "fiscal" ? "切换为自然年" : "切换为财年"}
        >
          {displayMode === "fiscal" ? "📅 财年" : "🗓 自然年"}
        </button>
        {displayMode === "fiscal" && (
          <select
            className="fiscal-month-select"
            value={fiscalStartMonth}
            onChange={(e) => setFiscalStartMonth(Number(e.target.value))}
            title="财年起始月份"
          >
            {["1月","2月","3月","4月","5月","6月","7月","8月","9月","10月","11月","12月"].map((label, i) => (
              <option key={i + 1} value={i + 1}>{label}起始</option>
            ))}
          </select>
        )}
      </div>

      {/* 主体双栏 */}
      <div className="dashboard-main">
        {/* 左栏：项目卡片列表 */}
        <div className="dashboard-left">
          <h2 className="section-title">项目状态总览</h2>
          {loading ? (
            <p className="text-secondary">加载中...</p>
          ) : projects.length === 0 ? (
            <div className="empty-state-small">
              <p>暂无项目数据</p>
              {isLoggedIn && (
                <button className="btn btn-primary" onClick={handleOpenCreate}>
                  + 创建第一个项目
                </button>
              )}
            </div>
          ) : (
            <div className="project-cards">
              {projects.map((p) => (
                <Link
                  to={`/project/${p.id}`}
                  key={p.id}
                  className={`project-card ${p.has_risk ? "has-risk" : ""}`}
                >
                  <div className="project-card-header">
                    <span
                      className="status-dot"
                      style={{ background: statusColor(p) }}
                    />
                    <span className="project-name">{p.name}</span>
                    {p.baseline_created_at && p.delay_days !== 0 && (
                      <span className={`delay-badge ${p.delay_days > 0 ? "neg" : "pos"}`}>
                        Δ {p.delay_days > 0 ? `+${p.delay_days}` : p.delay_days} 天
                      </span>
                    )}
                    <div className="project-card-progress">
                      <div className="progress-bar">
                        <div
                          className="progress-fill"
                          style={{
                            width: `${p.progress}%`,
                            background: statusColor(p),
                          }}
                        />
                      </div>
                      <span className="progress-text">{Math.round(p.progress)}%</span>
                    </div>
                    <span className="project-link">详情 →</span>
                  </div>
                  <div className="project-card-meta">
                    {p.next_milestone && (
                      <span>下个节点: {p.next_milestone}</span>
                    )}
                    {p.end_date && <span>截止: {p.end_date}</span>}
                    {p.has_risk && (
                      <span style={{ color: "var(--danger)" }}>⚠ {p.risk_count} 项超期</span>
                    )}
                  </div>
                </Link>
              ))}
            </div>
          )}
        </div>

        {/* 右栏：迷你甘特图 */}
        <div className="dashboard-right">
          <h2 className="section-title">时间线概览</h2>
          <div className="mini-gantt-placeholder">
            {projects.length === 0 ? (
              <p className="text-secondary">创建项目后将在此显示跨项目甘特图</p>
            ) : (
              <div className="mini-gantt-bars">
                {projects.map((p) => (
                  <div key={p.id} className="mini-gantt-row">
                    <span className="mini-gantt-name">{p.name}</span>
                    <div className="mini-gantt-track">
                      <div
                        className="mini-gantt-bar"
                        style={{
                          left: "10%",
                          width: `${Math.max(p.progress, 5)}%`,
                          background: statusColor(p),
                        }}
                      />
                    </div>
                  </div>
                ))}
                <div className="mini-gantt-today" title="今日" />
              </div>
            )}
          </div>
        </div>
      </div>

      {/* 底部 */}
      <div className="dashboard-bottom">
        <div className="dashboard-bottom-col">
          <h3 className="section-title">近期里程碑</h3>
          <p className="text-secondary">未来 14 天的里程碑将显示在此</p>
        </div>
        <div className="dashboard-bottom-col">
          <h3 className="section-title">我的待办</h3>
          {isLoggedIn ? (
            <p className="text-secondary">你还没有待办任务</p>
          ) : (
            <p className="text-secondary">登录后可查看个人待办</p>
          )}
        </div>
      </div>

      {/* 创建项目模态框 */}
      {showCreateModal && (
        <div className="modal-overlay" onClick={() => setShowCreateModal(false)}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <h2 className="modal-title">创建项目</h2>
            <form onSubmit={handleCreateSubmit}>
              <div className="form-group">
                <label htmlFor="project-name">项目名称 *</label>
                <input
                  id="project-name"
                  type="text"
                  placeholder="输入项目名称"
                  value={createForm.name}
                  onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                  autoFocus
                />
              </div>
              <div className="form-group">
                <label htmlFor="project-direction">排程方向</label>
                <select
                  id="project-direction"
                  value={createForm.schedule_direction}
                  onChange={(e) =>
                    setCreateForm({ ...createForm, schedule_direction: e.target.value })
                  }
                >
                  <option value="forward">正排（从开始日期向后排）</option>
                  <option value="backward">倒排（从完成日期向前排）</option>
                </select>
              </div>
              <div className="form-row">
                {createForm.schedule_direction === "forward" ? (
                  <div className="form-group">
                    <label htmlFor="project-start">开始日期</label>
                    <input
                      id="project-start"
                      type="date"
                      value={createForm.start_date}
                      onChange={(e) => setCreateForm({ ...createForm, start_date: e.target.value })}
                    />
                  </div>
                ) : (
                  <div className="form-group">
                    <label htmlFor="project-end">完成日期</label>
                    <input
                      id="project-end"
                      type="date"
                      value={createForm.end_date}
                      onChange={(e) => setCreateForm({ ...createForm, end_date: e.target.value })}
                    />
                  </div>
                )}
              </div>
              <div className="form-group">
                <label htmlFor="project-desc">描述</label>
                <textarea
                  id="project-desc"
                  rows={3}
                  placeholder="简要描述项目目标或范围"
                  value={createForm.description}
                  onChange={(e) => setCreateForm({ ...createForm, description: e.target.value })}
                />
              </div>
              {createError && <div className="form-error">{createError}</div>}
              <div className="modal-actions">
                <button
                  type="button"
                  className="btn btn-ghost"
                  onClick={() => setShowCreateModal(false)}
                >
                  取消
                </button>
                <button type="submit" className="btn btn-primary" disabled={createSubmitting}>
                  {createSubmitting ? "创建中..." : "创建项目"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* 回收站模态框 */}
      {showRecycleBin && (
        <div className="modal-overlay" onClick={() => setShowRecycleBin(false)}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <div className="modal-title">
              <h2>回收站</h2>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowRecycleBin(false)}>×</button>
            </div>
            <div className="modal-body">
              {deletedProjects.length === 0 ? (
                <p className="text-secondary">没有已删除的项目</p>
              ) : (
                <div className="dep-list">
                  {deletedProjects.map((p) => (
                    <div className="dep-item" key={p.id}>
                      <div className="dep-item-main">
                        <span className="dep-item-name">{p.name}</span>
                        <span className="dep-item-detail">
                          {p.description || "—"} · {p.schedule_direction === "backward" ? "倒排" : "正排"}
                        </span>
                        <span className="dep-item-detail">删除于 {p.deleted_at?.slice(0, 10)}</span>
                      </div>
                      <button
                        className="btn btn-primary btn-sm"
                        onClick={async () => {
                          try {
                            await api.post(`/api/projects/${p.id}/restore`);
                            alert(`项目「${p.name}」已恢复，项目内任务已全部恢复`);
                            setDeletedProjects((prev) => prev.filter((x) => x.id !== p.id));
                            fetchProjects(); // 刷新看板项目列表
                          } catch (err: any) {
                            alert(err?.response?.data?.error?.message || "恢复失败");
                          }
                        }}
                      >
                        恢复
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div className="modal-actions">
              <button className="btn btn-ghost" onClick={() => setShowRecycleBin(false)}>关闭</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
