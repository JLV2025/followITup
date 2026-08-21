import { getErrorMessage } from "../utils/errorMsg";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useAuthStore } from "../stores/authStore";
import { useDashboardStore } from "../stores/dashboardStore";
import { useSettingsStore, availableCalendarYears, availableFiscalYears, fiscalYearLabel, currentFiscalYear } from "../stores/settingsStore";
import api from "../api/client";
import { formatDate } from "../utils/date";
import { statusLabel } from "../utils/labels";
import MultiUserSelect from "../components/MultiUserSelect";
import i18n from "../i18n";

export default function Dashboard() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const loadFromStorage = useAuthStore((s) => s.loadFromStorage);

  const { stats, projects, period, loading, fetchStats, fetchProjects, setPeriod } =
    useDashboardStore();

  const { displayMode, fiscalStartMonth, setDisplayMode } = useSettingsStore();

  // 创建项目模态框状态
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createForm, setCreateForm] = useState({ name: "", owner_ids: [] as number[], start_date: "", end_date: "", schedule_direction: "forward", description: "" });
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [createError, setCreateError] = useState("");
  // 用户列表（项目所有者下拉，可手输兜底）
  const [userOptions, setUserOptions] = useState<{ id: number; display_name: string }[]>([]);
  // 时间线用全量项目（按排期日期过滤所选年度）；状态总览仍按创建时间过滤
  const [timelineProjects, setTimelineProjects] = useState<any[]>([]);
  // 我的待办：登录用户负责的任务 + 未来 N 天内开始的任务（窗口可选 7/14/30 天）
  const [myTodo, setMyTodo] = useState<{ mine: any[]; starting: any[] }>({ mine: [], starting: [] });
  const [todoDays, setTodoDays] = useState(7);
  const [todoView, setTodoView] = useState<"task" | "project">("task");

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
  // 月份刻度：所选年度内固定 12 个月（财年从 startMonth 起跨年），
  // 标签定位在每格中心（首末格留空避免 JUL/AUG 被画框边缘裁掉）；月份词随语言
  const marks = (() => {
    const arr: { x: number; label: string }[] = [];
    const base = new Date(Date.parse(yearStart + "T00:00:00Z"));
    for (let i = 0; i < 12; i++) {
      const s = new Date(base);
      s.setUTCMonth(s.getUTCMonth() + i);
      const e = new Date(s);
      if (i < 11) {
        e.setUTCMonth(e.getUTCMonth() + 1);
      } else {
        e.setTime(Date.parse(yearEnd + "T00:00:00Z")); // 第 12 个月结束 = 年度末
      }
      // 标签取实际月份（财年从 startMonth 起，第一格是 4 月而非 1 月）
      arr.push({ x: (pct(s.toISOString().slice(0, 10)) + pct(e.toISOString().slice(0, 10))) / 2, label: i18n.t(`months.upper.${s.getUTCMonth()}`) });
    }
    return arr;
  })();
  // 月份分格线：每月边界 13 条（含画框首尾边界），与月份标签同基准
  const gridlines = (() => {
    const arr: number[] = [pct(yearStart)];
    const base = new Date(Date.parse(yearStart + "T00:00:00Z"));
    for (let i = 1; i < 12; i++) {
      const d = new Date(base);
      d.setUTCMonth(d.getUTCMonth() + i);
      arr.push(pct(d.toISOString().slice(0, 10)));
    }
    arr.push(pct(yearEnd));
    return arr;
  })();
  // 时间线日期文字：去掉年份（画框已显示年度），紧凑 M/D 格式
  const fmtNoYear = (iso: string) => {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return String(iso).slice(5, 10);
    return `${d.getMonth() + 1}/${d.getDate()}`;
  };

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
    // 我的待办（需登录，窗口天数变化时重新拉取）
    // 注意：后端空结果时 mine/starting 可能为 null（Go nil slice 序列化），逐字段兜底防渲染崩溃
    if (isLoggedIn) {
      api.get(`/api/tasks/mine?days=${todoDays}&view=${todoView}`).then((res) => setMyTodo({
        mine: res.data?.data?.mine ?? [],
        starting: res.data?.data?.starting ?? [],
      })).catch(() => {});
    }
  }, [loadFromStorage, fetchStats, fetchProjects, displayMode, fiscalStartMonth, setPeriod, isLoggedIn, todoDays, todoView]);

  const periods = displayMode === "fiscal"
    ? availableFiscalYears(fiscalStartMonth)
    : availableCalendarYears(5);

  const periodLabel = (p: number) =>
    displayMode === "fiscal" ? fiscalYearLabel(p) : i18n.t("dashboard.yearLabel", { year: p });

  const handleToggleMode = () => {
    const nextMode = displayMode === "fiscal" ? "calendar" : "fiscal";
    setDisplayMode(nextMode);
    if (nextMode === "fiscal") {
      setPeriod(currentFiscalYear(fiscalStartMonth));
    } else {
      setPeriod(new Date().getFullYear());
    }
  };

  // 复制项目：深拷贝项目+任务+依赖，刷新列表
  const handleCopyProject = async (id: number) => {
    try {
      const res = await api.post(`/api/projects/${id}/copy`);
      const np = res.data.data;
      alert(i18n.t("dashboard.copied", { name: np.name }));
      fetchProjects();
    } catch (err: any) {
      alert(getErrorMessage(err, "common.unknownError"));
    }
  };

  const handleOpenCreate = () => {
    setCreateForm({ name: "", owner_ids: [], start_date: "", end_date: "", schedule_direction: "forward", description: "" });
    setCreateError("");
    setShowCreateModal(true);
  };

  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!createForm.name.trim()) {
      setCreateError(i18n.t("dashboard.errNameEmpty"));
      return;
    }
    if (createForm.owner_ids.length === 0) {
      setCreateError(i18n.t("dashboard.errOwnerEmpty"));
      return;
    }
    setCreateSubmitting(true);
    setCreateError("");
    try {
      await api.post("/api/projects", {
        name: createForm.name.trim(),
        owner_ids: createForm.owner_ids,
        description: createForm.description.trim(),
        start_date: createForm.start_date || null,
        end_date: createForm.end_date || null,
        schedule_direction: createForm.schedule_direction,
      });
      setShowCreateModal(false);
      fetchProjects();
      fetchStats();
    } catch (err: any) {
      setCreateError(getErrorMessage(err, "dashboard.errCreate"));
    } finally {
      setCreateSubmitting(false);
    }
  };

  // 状态灯三态色：未开始 = 灰，进行中 = 蓝，完成 = 绿（与进度条、时间线统一）
  // 完成判定基于 progress（与"已完成"tab 同一口径：防呆设计下 100% 必然整体完成；
  // 项目 status 字段恒为 active，不可作为完成依据）；风险红优先级最高
  const statusColor = (p: { has_risk: boolean; status: string; progress: number }) => {
    if (p.has_risk) return "var(--danger)";
    if (p.status === "archived" || isDone(p)) return "var(--success)";
    if (p.progress > 0) return "var(--accent)";
    return "var(--text-muted)";
  };
  // 进度条颜色：完成（100%）= 绿，进行中 = 蓝，未开始 = 灰（总览与时间线统一）
  const progressColor = (p: { progress: number }) =>
    isDone(p) ? "var(--success)" : p.progress > 0 ? "var(--accent)" : "var(--text-muted)";
  // 待办任务状态文本色（三态：未开始灰 / 进行中蓝 / 已延期红）
  const todoStatusColor = (s: string) =>
    s === "delayed" ? "var(--danger)" : s === "in_progress" ? "var(--accent)" : "var(--text-muted)";

  const statRiskClass = (stats?.at_risk ?? 0) > 0 ? "text-danger pulse-once" : "text-success";

  // 看板页占满视口宽度（突破 .main-content 的 1440px 居中限制，与甘特图页同模式）
  useEffect(() => {
    const mainEl = document.querySelector(".main-content");
    if (mainEl) mainEl.classList.add("dashboard-page");
    return () => { if (mainEl) mainEl.classList.remove("dashboard-page"); };
  }, []);

  return (
    <div className="dashboard">
      {/* 头部 */}
      <div className="dashboard-header">
        <div className="dashboard-header-row">
          <div>
            <h1>{t("dashboard.title")}</h1>
            {isLoggedIn ? (
              <p className="text-secondary">
                {t("dashboard.welcome", { name: user?.display_name || user?.email })}
              </p>
            ) : (
              <p className="text-secondary">
                {t("dashboard.readonly")} <Link to="/login">{t("nav.login")}</Link> {t("dashboard.readonlyEdit")}
              </p>
            )}
          </div>
          {isLoggedIn && (
            <div style={{ display: "flex", gap: 8 }}>
              <button className="btn btn-primary" onClick={handleOpenCreate}>
                {t("dashboard.createProject")}
              </button>
              <button
                className="btn btn-ghost"
                onClick={async () => {
                  try {
                    const res = await api.get("/api/projects?deleted=1");
                    setDeletedProjects(res.data.data || []);
                    setShowRecycleBin(true);
                  } catch (err: any) {
                    alert(getErrorMessage(err, "common.unknownError"));
                  }
                }}
              >
                {t("dashboard.recycleBin")}
              </button>
            </div>
          )}
        </div>
      </div>

      {/* 统计卡片 */}
      <div className="stats-row">
        <div className="stat-card">
          <span className="stat-label">{t("dashboard.statActive")}</span>
          <span className="stat-value">{stats?.active_projects ?? 0}</span>
        </div>
        <div className="stat-card">
          <span className="stat-label">{t("dashboard.statAtRisk")}</span>
          <span className={`stat-value ${statRiskClass}`}>
            {stats?.at_risk ?? 0}
          </span>
        </div>
        <div className="stat-card">
          <span className="stat-label">{t("dashboard.statDueWeek")}</span>
          <span className="stat-value">{stats?.due_this_week ?? 0}</span>
        </div>
        <div className="stat-card">
          <span className="stat-label">{t("dashboard.statOverall")}</span>
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
              <circle cx="24" cy="24" r="20" fill="none" stroke="var(--surface-alt)" strokeWidth="4" />
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
        <div className="stat-card">
          <span className="stat-label">{t("dashboard.statPlanActual")}</span>
          <span className="stat-value">
            {(stats?.actual_days ?? 0) > 0
              ? `${stats?.planned_days ?? 0} / ${stats?.actual_days ?? 0}`
              : "—"}
          </span>
          {(stats?.actual_days ?? 0) > 0 && (stats?.actual_days ?? 0) > (stats?.planned_days ?? 0) && (
            <div className="stat-delta neg">
              Δ +{((stats?.actual_days ?? 0) - (stats?.planned_days ?? 0))}{t("dashboard.statPlanActualUnit")}
            </div>
          )}
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
          title={displayMode === "fiscal" ? t("dashboard.switchCalendar") : t("dashboard.switchFiscal")}
        >
          {displayMode === "fiscal" ? t("dashboard.modeFiscal") : t("dashboard.modeCalendar")}
        </button>
      </div>

      {/* 主体双栏 */}
      <div className="dashboard-main">
        {/* 左栏：项目卡片列表 */}
        <div className="dashboard-left">
          <div className="section-title-row">
            <h2 className="section-title">{t("dashboard.sectionOverview")}</h2>
            <select
              className="project-filter"
              value={projectFilter}
              onChange={(e) => setProjectFilter(e.target.value as any)}
            >
              <option value="active">{t("dashboard.filterActive")}</option>
              <option value="completed">{t("dashboard.filterCompleted")}</option>
            </select>
          </div>
          {loading ? (
            <p className="text-secondary">{t("common.loading")}</p>
          ) : numberedProjects.length === 0 ? (
            <div className="empty-state-small">
              <p>{t("dashboard.emptyProjects")}</p>
              {isLoggedIn && (
                <button className="btn btn-primary" onClick={handleOpenCreate}>
                  {t("dashboard.createFirst")}
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
                    <span className="project-name" title={p.name}>{p.name}</span>
                    {p.baseline_created_at && p.delay_days !== 0 && (
                      <span className={`delay-badge ${p.delay_days > 0 ? "neg" : "pos"}`}>
                        Δ {p.delay_days > 0 ? `+${p.delay_days}` : p.delay_days} {t("dashboard.delayDays")}
                      </span>
                    )}
                    <button
                      className="project-copy-btn"
                      title={t("dashboard.copyTitle")}
                      onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        handleCopyProject(p.id);
                      }}
                    >
                      ⧉
                    </button>
                    {/* 第一排靠右：负责人（小人头 + 定宽人名，超长省略） */}
                    <span className="project-owner" title={p.owner || t("dashboard.ownerTitle")}>
                      <span className="owner-icon">👤</span>
                      <span className="owner-name">{p.owner || "—"}</span>
                    </span>
                    <span className="project-link">{t("dashboard.detail")}</span>
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
                      {/* 倒排(锚点)= 截止 DL / 正排(推导)= 预计 ETA */}
                      {p.end_date
                        ? p.schedule_direction === "backward"
                          ? t("dashboard.dueBy", { date: formatDate(p.end_date) })
                          : t("dashboard.etaBy", { date: formatDate(p.end_date) })
                        : ""}
                    </span>
                  </div>
                </Link>
                {isLoggedIn && (
                  <button
                    className="card-delete"
                    title={t("dashboard.deleteTitle")}
                    onClick={async () => {
                      if (!confirm(i18n.t("dashboard.confirmDeleteProject", { name: p.name }))) return;
                      try {
                        await api.delete(`/api/projects/${p.id}`);
                        fetchProjects();
                        alert(i18n.t("dashboard.deleted", { name: p.name }));
                      } catch (err: any) {
                        alert(getErrorMessage(err, "common.unknownError"));
                      }
                    }}
                  >
                    {t("common.delete")}
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
            <h2 className="section-title">{t("dashboard.sectionTimeline")}</h2>
          </div>
          <div className="mini-gantt-placeholder">
            {timelineProjects.length === 0 ? (
              <p className="text-secondary">{t("dashboard.timelineEmpty")}</p>
            ) : datedProjects.length === 0 ? (
              <p className="text-secondary">{t("dashboard.timelineEmptyHint")}</p>
            ) : (
              <div className="mini-gantt-chart">
                {/* 第一行：年度标识（FY27 / CY2026，画框名字） */}
                <div className="mini-gantt-scale-row mini-gantt-year-row">
                  <span className="mini-gantt-no" />
                  <span className="mini-gantt-name" />
                  <div className="mini-gantt-scale">
                    <span className="mini-gantt-year-label">
                      {displayMode === "calendar" ? `CY${period}` : `FY${period}`}
                    </span>
                  </div>
                </div>
                {/* 第二行：12 个月份（格中心定位，首末格留空避免标签被画框边缘裁掉） */}
                <div className="mini-gantt-scale-row">
                  <span className="mini-gantt-no" />
                  <span className="mini-gantt-name" />
                  <div className="mini-gantt-scale">
                    {marks.map((m, i) => (
                      <span key={i} className="mini-gantt-mark" style={{ left: `${m.x}%`, transform: "translateX(-50%)" }}>
                        {m.label}
                      </span>
                    ))}
                  </div>
                </div>
                {/* 每个项目一条按真实日期定位的甘特条：轨道浅灰、条底深灰（排期跨度）、
                    未开始=全深灰、完成=全绿、进行中=深灰底+蓝段；跨年裁剪端加小箭头 */}
                {datedProjects.map((p) => {
                  const spanPct = pct(p.end_date) - pct(p.start_date);
                  const donePct = (spanPct * Math.min(100, p.progress)) / 100;
                  const clipLeft = p.start_date < yearStart; // 头被裁（年度外还有内容）
                  const clipRight = p.end_date > yearEnd;    // 尾被裁
                  const barBg = isDone(p) ? "var(--success)" : "var(--text-muted)";
                  return (
                    <div key={p.id} className="mini-gantt-row">
                      <span className="mini-gantt-no">#{p.no}</span>
                      <span className="mini-gantt-name">
                        <span className="mini-gantt-name-text">{p.name}</span>
                        <span className="mini-gantt-name-dates">
                          {fmtNoYear(p.start_date)} ~ {fmtNoYear(p.end_date)}
                        </span>
                      </span>
                      <div className="mini-gantt-track">
                        <div
                          className="mini-gantt-bar"
                          style={{
                            left: `${pct(p.start_date)}%`,
                            width: `${Math.max(spanPct, 1.5)}%`,
                            background: barBg,
                          }}
                        >
                          {clipLeft && <span className="mini-gantt-clip mini-gantt-clip-left">◀</span>}
                          {clipRight && <span className="mini-gantt-clip mini-gantt-clip-right">▶</span>}
                          {!isDone(p) && p.progress > 0 && (
                            <div className="mini-gantt-bar-done" style={{ width: `${donePct}%`, background: "var(--accent)" }} />
                          )}
                        </div>
                        {p.has_risk && <span className="mini-gantt-risk" title={t("dashboard.riskTitle")}>▼</span>}
                      </div>
                    </div>
                  );
                })}
                {/* overlay：月份分格线 + 今日线——与轨道同宽同基准（left: 左列宽），
                    修复此前百分比基准不一致导致的 today 线偏移 */}
                <div className="mini-gantt-overlay">
                  {gridlines.map((x, i) => (
                    <div key={i} className="mini-gantt-gridline" style={{ left: `${x}%` }} />
                  ))}
                  {todayInYear && (
                    <div className="mini-gantt-today" style={{ left: `${todayPct}%` }} title={t("dashboard.todayTitle")} />
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* 底部 */}
      <div className="dashboard-bottom">
        <div className="dashboard-bottom-col">
          <h3 className="section-title">{t("dashboard.sectionMilestones")}</h3>
          <p className="text-secondary">{t("dashboard.milestonesEmpty")}</p>
        </div>
        <div className="dashboard-bottom-col">
          <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 12 }}>
            <h3 className="section-title" style={{ marginBottom: 0 }}>{t("dashboard.sectionTodo")}</h3>
            {isLoggedIn && (
              <div className="todo-view-switch">
                <button
                  className={`todo-view-btn${todoView === "task" ? " active" : ""}`}
                  onClick={() => setTodoView("task")}
                >
                  {t("dashboard.todoViewTask")}
                </button>
                <button
                  className={`todo-view-btn${todoView === "project" ? " active" : ""}`}
                  onClick={() => setTodoView("project")}
                >
                  {t("dashboard.todoViewProject")}
                </button>
              </div>
            )}
          </div>
          {!isLoggedIn ? (
            <p className="text-secondary">{t("dashboard.todoLogin")}</p>
          ) : myTodo.mine.length === 0 && myTodo.starting.length === 0 ? (
            <p className="text-secondary">{t("dashboard.todoEmpty")}</p>
          ) : (
            <>
              {myTodo.mine.length > 0 && (
                <div className="todo-block">
                  <div className="todo-block-title">{t("dashboard.todoMine")}</div>
                  <table className="todo-table">
                    <thead>
                      <tr>
                        <th>{t("dashboard.colTask")}</th>
                        <th>{t("dashboard.colProject")}</th>
                        <th>{t("dashboard.colStatus")}</th>
                        <th>{t("dashboard.colStart")}</th>
                        <th>{t("dashboard.colEnd")}</th>
                        <th>{t("dashboard.colProgress")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {myTodo.mine.map((x) => (
                        <tr key={x.id}>
                          <td className="todo-task" title={x.name}>{x.name}</td>
                          <td>{x.project_name}</td>
                          <td><span style={{ color: todoStatusColor(x.status) }}>{statusLabel(x.status)}</span></td>
                          <td>{formatDate(x.start_date)}</td>
                          <td>{formatDate(x.end_date)}</td>
                          <td>{Math.round(x.progress_pct)}%</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              {myTodo.starting.length > 0 && (
                <div className="todo-block">
                  <div className="todo-block-title todo-block-title-row">
                    <span>{t("dashboard.todoStarting", { days: todoDays })}</span>
                    <select
                      className="todo-days-select"
                      value={todoDays}
                      onChange={(e) => setTodoDays(Number(e.target.value))}
                      title={t("dashboard.todoDaysHint")}
                    >
                      <option value={7}>7</option>
                      <option value={14}>14</option>
                      <option value={30}>30</option>
                    </select>
                  </div>
                  <table className="todo-table">
                    <thead>
                      <tr>
                        <th>{t("dashboard.colTask")}</th>
                        <th>{t("dashboard.colProject")}</th>
                        <th>{t("dashboard.colStart")}</th>
                        <th>{t("dashboard.colAssignee")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {myTodo.starting.map((x) => (
                        <tr key={x.id}>
                          <td className="todo-task" title={x.name}>{x.name}</td>
                          <td>{x.project_name}</td>
                          <td>{formatDate(x.start_date)}</td>
                          <td>{x.assignee || "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {/* 创建项目模态框 */}
      {showCreateModal && (
        <div className="modal-overlay" onClick={() => setShowCreateModal(false)}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <h2 className="modal-title">{t("dashboard.modalCreateTitle")}</h2>
            <form onSubmit={handleCreateSubmit}>
              <div className="form-group">
                <label htmlFor="project-name">{t("dashboard.nameRequired")}</label>
                <input
                  id="project-name"
                  type="text"
                  placeholder={t("dashboard.namePlaceholder")}
                  value={createForm.name}
                  onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                  autoFocus
                />
              </div>
              <div className="form-group">
                <label htmlFor="project-owner">{t("dashboard.ownerRequired")}</label>
                <MultiUserSelect
                  users={userOptions}
                  selectedIds={createForm.owner_ids}
                  onChange={(ids) => setCreateForm({ ...createForm, owner_ids: ids })}
                  placeholder={userOptions.length === 0 ? t("dashboard.ownerEmpty") : t("dashboard.ownerSelect")}
                />
                <span className="form-hint">{t("dashboard.ownerHint")}</span>
              </div>
              <div className="form-group">
                <label htmlFor="project-direction">{t("dashboard.direction")}</label>
                <select
                  id="project-direction"
                  value={createForm.schedule_direction}
                  onChange={(e) =>
                    setCreateForm({ ...createForm, schedule_direction: e.target.value })
                  }
                >
                  <option value="forward">{t("dashboard.directionForward")}</option>
                  <option value="backward">{t("dashboard.directionBackward")}</option>
                </select>
              </div>
              <div className="form-row">
                {createForm.schedule_direction === "forward" ? (
                  <div className="form-group">
                    <label htmlFor="project-start">{t("dashboard.startDate")}</label>
                    <input
                      id="project-start"
                      type="date"
                      value={createForm.start_date}
                      onChange={(e) => setCreateForm({ ...createForm, start_date: e.target.value })}
                    />
                  </div>
                ) : (
                  <div className="form-group">
                    <label htmlFor="project-end">{t("dashboard.endDate")}</label>
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
                <label htmlFor="project-desc">{t("dashboard.desc")}</label>
                <textarea
                  id="project-desc"
                  rows={3}
                  placeholder={t("dashboard.descPlaceholder")}
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
                  {t("common.cancel")}
                </button>
                <button type="submit" className="btn btn-primary" disabled={createSubmitting}>
                  {createSubmitting ? t("dashboard.creating") : t("dashboard.createSubmit")}
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
              <h2>{t("dashboard.recycleBin")}</h2>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowRecycleBin(false)}>×</button>
            </div>
            <div className="modal-body">
              {deletedProjects.length === 0 ? (
                <p className="text-secondary">{t("dashboard.recycleEmpty")}</p>
              ) : (
                <div className="dep-list">
                  {deletedProjects.map((p) => (
                    <div className="dep-item" key={p.id}>
                      <div className="dep-item-main">
                        <span className="dep-item-name">{p.name}</span>
                        <span className="dep-item-detail">
                          {p.description || "—"} · {p.schedule_direction === "backward" ? t("dashboard.directionBackward") : t("dashboard.directionForward")}
                        </span>
                        <span className="dep-item-detail">{t("dashboard.deletedAt", { date: p.deleted_at?.slice(0, 10) })}</span>
                      </div>
                      <button
                        className="btn btn-primary btn-sm"
                        onClick={async () => {
                          try {
                            await api.post(`/api/projects/${p.id}/restore`);
                            alert(i18n.t("dashboard.restored", { name: p.name }));
                            setDeletedProjects((prev) => prev.filter((x) => x.id !== p.id));
                            fetchProjects(); // 刷新看板项目列表
                          } catch (err: any) {
                            alert(getErrorMessage(err, "common.unknownError"));
                          }
                        }}
                      >
                        {t("dashboard.restore")}
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div className="modal-actions">
              <button className="btn btn-ghost" onClick={() => setShowRecycleBin(false)}>{t("common.close")}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
