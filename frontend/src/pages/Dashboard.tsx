import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useAuthStore } from "../stores/authStore";
import { useDashboardStore } from "../stores/dashboardStore";
import { useSettingsStore, availableCalendarYears, availableFiscalYears, fiscalYearLabel, currentFiscalYear } from "../stores/settingsStore";
import api from "../api/client";
import { formatDate } from "../utils/date";

export default function Dashboard() {
  const user = useAuthStore((s) => s.user);
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const loadFromStorage = useAuthStore((s) => s.loadFromStorage);

  const { stats, projects, period, loading, fetchStats, fetchProjects, setPeriod } =
    useDashboardStore();

  const { displayMode, fiscalStartMonth, setDisplayMode } = useSettingsStore();

  // 创建项目模态框状态
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createForm, setCreateForm] = useState({ name: "", owner: "", start_date: "", end_date: "", schedule_direction: "forward", description: "" });
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [createError, setCreateError] = useState("");
  // 用户列表（项目所有者下拉，可手输兜底）
  const [userOptions, setUserOptions] = useState<{ display_name: string; email: string }[]>([]);
  // 时间线用全量项目（按排期日期过滤所选年度）；状态总览仍按创建时间过滤
  const [timelineProjects, setTimelineProjects] = useState<any[]>([]);

  // 回收站模态框状态
  const [showRecycleBin, setShowRecycleBin] = useState(false);
  const [deletedProjects, setDeletedProjects] = useState<any[]>([]);

  // 项目筛选：仅两个状态——进行中（默认，全量显示不受年度影响）/ 已完成（按所选年度过滤）
  const [projectFilter, setProjectFilter] = useState<"active" | "completed">("active");
  // 已完成判定：完成度 100%（防呆设计下必然整体完成）
  const isDone = (p: any) => (p.progress ?? 0) >= 100;
  // 所选年度范围：自然年 = 1/1~12/31；财年 FY{n} = 前一年 startMonth ~ 本年 (startMonth-1) 月末
  //（状态总览"已完成"按此过滤，时间线概览亦复用）
  const DAY_MS = 86400000;
  const { start: yearStart, end: yearEnd } = (() => {
    if (displayMode === "calendar") return { start: `${period}-01-01`, end: `${period}-12-31` };
    const y = 2000 + period;
    if (fiscalStartMonth === 1) return { start: `${y}-01-01`, end: `${y}-12-31` };
    const m = String(fiscalStartMonth).padStart(2, "0");
    const end = new Date(Date.parse(`${y}-${m}-01T00:00:00Z`) - DAY_MS).toISOString().slice(0, 10);
    return { start: `${y - 1}-${m}-01`, end };
  })();
  // 进行中：全量（不管跨年）；已完成：按结束日期落在所选年度内（归档视角）
  const filteredProjects = projects.filter((p) =>
    projectFilter === "completed"
      ? isDone(p) && !!p.end_date && p.end_date >= yearStart && p.end_date <= yearEnd
      : !isDone(p)
  );
  const numberedProjects = filteredProjects.map((p, idx) => ({ ...p, no: idx + 1 }));

  // ===== 时间线概览：按所选年度（自然年/财年）固定范围的迷你甘特图 =====
  // DAY_MS / yearStart / yearEnd 已在状态总览过滤处定义，此处复用
  const todayStr = () => {
    const d = new Date();
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  };
  const dayDiff = (a: string, b: string) =>
    Math.round((Date.parse(b) - Date.parse(a)) / DAY_MS);
  const today = todayStr();
  const totalDays = dayDiff(yearStart, yearEnd);
  const pct = (s: string) => Math.min(100, Math.max(0, (dayDiff(yearStart, s) / totalDays) * 100));
  // 过滤：完全不在所选年度内 → 不显示；跨年的条裁到年度边界（显示头/尾）
  // 编号沿用全量列表老→新序号（与状态总览同一套编号体系）
  const datedProjects = timelineProjects
    .map((p, idx) => ({ ...p, no: idx + 1 }))
    .filter((p) => p.start_date && p.end_date && !(p.start_date > yearEnd || p.end_date < yearStart));
  const todayPct = pct(today);
  const todayInYear = today >= yearStart && today <= yearEnd;
  // 月份刻度：所选年度内固定 12 个月（财年从 startMonth 起跨年）
  const marks = (() => {
    const arr: { x: number; label: string }[] = [];
    const cur = new Date(Date.parse(yearStart + "T00:00:00Z"));
    const firstLabel = displayMode === "calendar" ? `${period}年` : `FY${period}`;
    for (let i = 0; i < 12; i++) {
      arr.push({
        x: pct(cur.toISOString().slice(0, 10)),
        label: i === 0 ? firstLabel : `${cur.getUTCMonth() + 1}月`,
      });
      cur.setUTCMonth(cur.getUTCMonth() + 1);
    }
    return arr;
  })();

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
    // 加载用户列表用于所有者下拉
    api.get("/api/users").then((res) => setUserOptions(res.data.data || [])).catch(() => {});
    // 全量项目（不带年度参数）：时间线按排期日期过滤所选年度
    api.get("/api/dashboard/projects").then((res) => setTimelineProjects(res.data.data || [])).catch(() => {});
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
    setCreateForm({ name: "", owner: "", start_date: "", end_date: "", schedule_direction: "forward", description: "" });
    setCreateError("");
    setShowCreateModal(true);
  };

  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!createForm.name.trim()) {
      setCreateError("项目名称不能为空");
      return;
    }
    if (!createForm.owner.trim()) {
      setCreateError("请选择项目所有者");
      return;
    }
    setCreateSubmitting(true);
    setCreateError("");
    try {
      await api.post("/api/projects", {
        name: createForm.name.trim(),
        owner: createForm.owner.trim(),
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
  // 进度条颜色：完成（100%）= 绿，进行中 = 蓝（总览与时间线统一）
  const progressColor = (p: { progress: number }) => (isDone(p) ? "var(--success)" : "var(--accent)");

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
      </div>

      {/* 主体双栏 */}
      <div className="dashboard-main">
        {/* 左栏：项目卡片列表 */}
        <div className="dashboard-left">
          <div className="section-title-row">
            <h2 className="section-title">项目状态总览</h2>
            <select
              className="project-filter"
              value={projectFilter}
              onChange={(e) => setProjectFilter(e.target.value as any)}
            >
              <option value="active">进行中</option>
              <option value="completed">已完成</option>
            </select>
          </div>
          {loading ? (
            <p className="text-secondary">加载中...</p>
          ) : numberedProjects.length === 0 ? (
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
              {numberedProjects.map((p) => (
                <div className="project-card-wrap" key={p.id}>
                <Link
                  to={`/project/${p.id}`}
                  className={`project-card ${p.has_risk ? "has-risk" : ""}`}
                >
                  <div className="project-card-header">
                    <span
                      className="status-dot"
                      style={{ background: statusColor(p) }}
                    />
                    <span className="project-no">#{p.no}</span>
                    <span className="project-name">{p.name}</span>
                    {p.baseline_created_at && p.delay_days !== 0 && (
                      <span className={`delay-badge ${p.delay_days > 0 ? "neg" : "pos"}`}>
                        Δ {p.delay_days > 0 ? `+${p.delay_days}` : p.delay_days} 天
                      </span>
                    )}
                    {/* 项目所有者：第一排，右缘与第二排百分比数字对齐（正上方）；
                        图标固定宽 + 名字区固定 15 字母宽（≈90px），位置不随名字长度浮动 */}
                    <span className="project-owner" title="项目所有者">
                      <span className="owner-icon">👤</span>
                      <span className="owner-name">{p.owner || "—"}</span>
                    </span>
                    <span className="project-link">详情 →</span>
                  </div>
                  <div className="project-card-body">
                    <div className="project-card-progress">
                      <div className="progress-bar">
                        <div
                          className="progress-fill"
                          style={{
                            width: `${p.progress}%`,
                            background: progressColor(p),
                          }}
                        />
                      </div>
                      <span className="progress-text">{Math.round(p.progress)}%</span>
                    </div>
                    {/* 恒渲染占位：保证有无截止日期时进度条长度统一 */}
                    <span className="project-card-end">
                      {p.end_date ? `截止: ${formatDate(p.end_date)}` : ""}
                    </span>
                  </div>
                </Link>
                {isLoggedIn && (
                  <button
                    className="card-delete"
                    title="删除项目（可在首页回收站恢复）"
                    onClick={async () => {
                      if (!confirm(`确认删除项目「${p.name}」？\n\n项目内任务不会被删除，删除后可在首页「回收站」恢复。`)) return;
                      try {
                        await api.delete(`/api/projects/${p.id}`);
                        fetchProjects();
                        alert(`项目「${p.name}」已删除，可在「回收站」恢复`);
                      } catch (err: any) {
                        alert(err?.response?.data?.error?.message || "删除失败");
                      }
                    }}
                  >
                    删除
                  </button>
                )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 右栏：迷你甘特图 */}
        <div className="dashboard-right">
          <div className="section-title-row">
            <h2 className="section-title">时间线概览</h2>
          </div>
          <div className="mini-gantt-placeholder">
            {timelineProjects.length === 0 ? (
              <p className="text-secondary">该年度内没有排期项目</p>
            ) : datedProjects.length === 0 ? (
              <p className="text-secondary">该年度内没有排期项目（未设置日期范围的项目不参与时间线）</p>
            ) : (
              <div className="mini-gantt-chart">
                {/* 顶部共享月份刻度 */}
                <div className="mini-gantt-scale-row">
                  <span className="mini-gantt-no" />
                  <span className="mini-gantt-name" />
                  <div className="mini-gantt-scale">
                    {marks.map((m, i) => (
                      <span
                        key={i}
                        className="mini-gantt-mark"
                        style={{
                          left: `${m.x}%`,
                          // 首刻度左对齐、末刻度右对齐，避免标签溢出卡片边界
                          transform:
                            i === 0
                              ? "translateX(0)"
                              : i === marks.length - 1
                                ? "translateX(-100%)"
                                : "translateX(-50%)",
                        }}
                      >
                        {m.label}
                      </span>
                    ))}
                  </div>
                </div>
                {/* 每个项目一条按真实日期定位的甘特条 */}
                {datedProjects.map((p) => {
                  const spanPct = pct(p.end_date) - pct(p.start_date);
                  const donePct = (spanPct * Math.min(100, p.progress)) / 100;
                  return (
                    <div key={p.id} className="mini-gantt-row">
                      <span className="mini-gantt-no">#{p.no}</span>
                      <span className="mini-gantt-name">
                        <span className="mini-gantt-name-text">{p.name}</span>
                        <span className="mini-gantt-name-dates">
                          {formatDate(p.start_date)} ~ {formatDate(p.end_date)}
                        </span>
                      </span>
                      <div className="mini-gantt-track">
                        <div
                          className="mini-gantt-bar"
                          style={{
                            left: `${pct(p.start_date)}%`,
                            width: `${Math.max(spanPct, 1.5)}%`,
                          }}
                        >
                          <div
                            className="mini-gantt-bar-done"
                            style={{ width: `${donePct}%`, background: progressColor(p) }}
                          />
                        </div>
                      </div>
                    </div>
                  );
                })}
                {/* 月份刻度竖线 + 今日线（仅当今天在所选年度内显示） */}
                {marks.map((m, i) => (
                  <div key={i} className="mini-gantt-gridline" style={{ left: `${m.x}%` }} />
                ))}
                {todayInYear && (
                  <div className="mini-gantt-today" style={{ left: `${todayPct}%` }} title="今日" />
                )}
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
                <label htmlFor="project-owner">项目所有者 *</label>
                <select
                  id="project-owner"
                  value={createForm.owner}
                  onChange={(e) => setCreateForm({ ...createForm, owner: e.target.value })}
                >
                  <option value="">{userOptions.length === 0 ? "（暂无用户，请先在用户管理创建）" : "请选择"}</option>
                  {userOptions.map((u) => (
                    <option key={u.email} value={u.display_name || u.email}>
                      {u.display_name || u.email}
                    </option>
                  ))}
                </select>
                <span className="form-hint">所有者必须是系统用户（发邮件通知用），未开始任务默认取该所有者</span>
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
