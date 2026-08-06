import { useEffect, useRef, useState } from "react";
import { useOutletContext, useParams } from "react-router-dom";
import { gantt } from "dhtmlx-gantt";
import "dhtmlx-gantt/codebase/dhtmlxgantt.css";
import { useGanttStore } from "../stores/ganttStore";
import { useAuthStore } from "../stores/authStore";
import { wsClient } from "../api/ws-client";
import api from "../api/client";
import TaskDetailModal from "../components/TaskDetailModal";
import RecycleBinModal from "../components/RecycleBinModal";

interface Task {
  id: number;
  project_id: number;
  parent_id: number | null;
  name: string;
  description: string;
  task_type: string;
  status: string;
  priority: string;
  assignee: string;
  start_date: string;
  end_date: string;
  duration_days: number;
  progress_pct: number;
  actual_start: string;
  actual_end: string;
  baseline_start_date: string;
  baseline_end_date: string;
  manual_scheduled: boolean;
  constraint_type: string;
  constraint_date: string;
  sort_order: number;
  version: number;
}

const USER_COLORS = [
  "#2C6E6A", "#0891B2", "#C88C4A", "#7C9A92", "#4A7C7A",
  "#5B8DEF", "#A78BFA", "#4D9B91", "#D97706", "#6B9E9A",
];

const userColorCache = new Map<string, string>();
let colorSeq = 0;
function getUserColor(name: string): string {
  if (!userColorCache.has(name)) {
    userColorCache.set(name, USER_COLORS[colorSeq % USER_COLORS.length]);
    colorSeq++;
  }
  return userColorCache.get(name)!;
}

const ZOOM_LABELS = ["日", "3日", "周", "2周", "月", "2月", "季", "年"];
const ZOOM_LEVELS: dhtmlxZoomLevel[] = [
  { name: "day", scale_height: 40, min_column_width: 80, scales: [{ unit: "day", step: 1, format: "%d" }] },
  { name: "3days", scale_height: 40, min_column_width: 60, scales: [{ unit: "day", step: 3, format: "%m/%d" }] },
  { name: "week", scale_height: 40, min_column_width: 60,
    scales: [{ unit: "week", step: 1, format: "W%W" }, { unit: "day", step: 1, format: "%a" }] },
  { name: "2weeks", scale_height: 40, min_column_width: 50,
    scales: [{ unit: "week", step: 2, format: "W%W-%W" }, { unit: "day", step: 1, format: "%d" }] },
  { name: "month", scale_height: 40, min_column_width: 90,
    scales: [{ unit: "month", step: 1, format: "%M" }, { unit: "day", step: 1, format: "%d" }] },
  { name: "2months", scale_height: 40, min_column_width: 70,
    scales: [{ unit: "month", step: 2, format: "%M-%M" }, { unit: "day", step: 1, format: "%d" }] },
  // 季度/年级别用单层 scale：每列 = 一个 step 单位（季度=3个月/年=1个月），列宽由 min_column_width 决定。
  // 双层 scale 时最细层（天）固定每格 1 天宽，月层只合并标签——短项目看不出区别（曾被误报"季档无效"）
  { name: "quarter", scale_height: 40, min_column_width: 80,
    scales: [{ unit: "month", step: 3, format: "%M" }] },
  { name: "year", scale_height: 40, min_column_width: 50, scales: [{ unit: "month", step: 1, format: "%m月" }] },
];

type dhtmlxZoomLevel = {
  name: string;
  scale_height: number;
  min_column_width: number;
  scales: Array<{ unit: string; step: number; format: string }>;
};

export default function ProjectGantt({ readonly }: { readonly: boolean }) {
  const { id } = useParams<{ id: string }>();
  const projectId = Number(id);
  const containerRef = useRef<HTMLDivElement>(null);
  const initRef = useRef(false);
  const lastFocusedRef = useRef<number>(0);
  /** ref 存储最新 allTasks，避免 useEffect 闭包捕获过期值 */
  const allTasksRef = useRef<Task[]>([]);
  const readonlyRef = useRef(readonly);
  readonlyRef.current = readonly;

  const [ganttReady, setGanttReady] = useState(false);
  const [zoomLevel, setZoomLevel] = useState(4); // 默认"月"
  const autoZoomDoneRef = useRef(false);
  const selectedTaskRef = useRef<number | null>(null);
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null);

  const [modalTask, setModalTask] = useState<Task | null>(null);
  const [showModal, setShowModal] = useState(false);
  const [showRecycleBin, setShowRecycleBin] = useState(false);
  const [projectName, setProjectName] = useState("");
  const [allTasks, setAllTasks] = useState<Task[]>([]);
  const [rowNumbers, setRowNumbers] = useState<Record<number, number>>({});
  const [baselineMenuOpen, setBaselineMenuOpen] = useState(false);

  /** 收集甘特图当前展示顺序的 id → 行号映射（树深度优先，1 基） */
  const buildRowNumbers = () => {
    const map: Record<number, number> = {};
    let n = 0;
    gantt.eachTask(function (t: Record<string, any>) { map[t.id as number] = ++n; });
    setRowNumbers(map);
  };

  // 基线菜单外部点击关闭：菜单打开时注册 document 级 click 监听，关闭时清理
  useEffect(() => {
    if (!baselineMenuOpen) return;
    const handleClick = () => setBaselineMenuOpen(false);
    // 延迟注册避免同一事件循环中 toggle 按钮的 click 事件立即触发关闭
    const timer = setTimeout(() => document.addEventListener("click", handleClick), 0);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("click", handleClick);
    };
  }, [baselineMenuOpen]);

  const {
    tasks, links, focusMap, loading, baselineMeta,
    fetchData, addLink, deleteLink,
    setFocus, clearFocus, pruneExpired,
    fetchBaselineMeta, createBaseline, clearBaseline,
  } = useGanttStore();

  const user = useAuthStore((s) => s.user);
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const loadFromStorage = useAuthStore((s) => s.loadFromStorage);

  useEffect(() => { loadFromStorage(); }, [loadFromStorage]);

  useEffect(() => {
    if (!isLoggedIn || !user) return;
    wsClient.connect(projectId, user.id, user.display_name || user.email || "未知");
    return () => { wsClient.disconnect(); };
  }, [projectId, isLoggedIn, user]);

  useEffect(() => {
    const unsub = wsClient.subscribe((msg) => {
      if (msg.type === "task_focus" && msg.task_id && msg.user_name) setFocus(msg.task_id, msg.user_name);
      else if (msg.type === "task_blur" && msg.task_id) clearFocus(msg.task_id);
      else if (msg.type === "task_update" || msg.type === "reconnected") fetchData(projectId, readonlyRef.current);
    });
    return unsub;
  }, [projectId, fetchData, setFocus, clearFocus]);

  useEffect(() => {
    const timer = setInterval(() => pruneExpired(), 10000);
    return () => clearInterval(timer);
  }, [pruneExpired]);

  useEffect(() => {
    fetchData(projectId, readonly);
  }, [projectId, readonly, fetchData]);

  useEffect(() => {
    fetchBaselineMeta(projectId);
  }, [projectId, fetchBaselineMeta]);

  // 项目详情页页首修改项目开始/结束日期后，重新拉取任务数据（后端已全项目重排）
  // 双保险：Outlet context refreshKey + 全局 project-refresh 事件
  const outletCtx = useOutletContext<{ refreshKey?: number }>();
  const prevRefreshKey = useRef(outletCtx?.refreshKey);
  useEffect(() => {
    if (outletCtx?.refreshKey !== undefined && outletCtx.refreshKey !== prevRefreshKey.current) {
      prevRefreshKey.current = outletCtx.refreshKey;
      fetchData(projectId, readonlyRef.current);
    }
  }, [outletCtx?.refreshKey, projectId, fetchData]);
  useEffect(() => {
    const onProjectRefresh = (e: Event) => {
      const detail = (e as CustomEvent).detail as { projectId?: number };
      if (detail?.projectId === projectId) fetchData(projectId, readonlyRef.current);
    };
    window.addEventListener("project-refresh", onProjectRefresh);
    return () => window.removeEventListener("project-refresh", onProjectRefresh);
  }, [projectId, fetchData]);

  // 加载原始任务列表（用于弹窗）
  useEffect(() => {
    const loadAllTasks = async () => {
      try {
        const res = await api.get(`/api/projects/${projectId}/tasks`);
        const data = res.data.data.tasks || [];
        allTasksRef.current = data;
        setAllTasks(data);
      } catch { /* ignore */ }
    };
    loadAllTasks();
  }, [projectId]);

  // 加载项目名称（用于回收站弹窗标题）
  useEffect(() => {
    const loadProjectName = async () => {
      try {
        const res = await api.get(`/api/projects/${projectId}`);
        setProjectName(res.data.data?.name || "");
      } catch { /* ignore */ }
    };
    loadProjectName();
  }, [projectId]);

  const handleTaskClick = (id: number) => {
    if (!isLoggedIn) return;
    if (lastFocusedRef.current && lastFocusedRef.current !== id) wsClient.sendBlur(lastFocusedRef.current);
    wsClient.sendFocus(id);
    lastFocusedRef.current = id;
  };

  const handleEmptyClick = () => {
    if (lastFocusedRef.current) { wsClient.sendBlur(lastFocusedRef.current); lastFocusedRef.current = 0; }
  };

  /** 新建任务 */
  const handleAddTask = () => {
    if (readonly) return;
    buildRowNumbers();
    setModalTask(null);
    setShowModal(true);
  };

  /** 弹窗保存后刷新 */
  const handleModalSaved = () => {
    setShowModal(false);
    setModalTask(null);
    fetchData(projectId, readonly);
    api.get(`/api/projects/${projectId}/tasks`).then((res) => {
      const data = res.data.data.tasks || [];
      allTasksRef.current = data;
      setAllTasks(data);
    }).catch(() => {});
  };

  const handleZoom = (dir: -1 | 1) => {
    const newLevel = zoomLevel + dir;
    if (newLevel < 0 || newLevel >= ZOOM_LEVELS.length) return;
    setZoomLevel(newLevel);
    try {
      (gantt.ext as any).zoom.setLevel(ZOOM_LEVELS[newLevel].name);
    } catch { /* ignore */ }
  };

  /** 自动缩放：在当前页面宽度内完整显示项目全部时间范围，无滚动条 */
  const handleAutoZoom = () => {
    if (tasks.length === 0) return;
    const dates = tasks.map(t => new Date(t.start_date).getTime())
      .concat(tasks.map(t => new Date(t.end_date).getTime()));
    const min = new Date(Math.min(...dates));
    const max = new Date(Math.max(...dates));
    const rangeDays = Math.max(1, Math.ceil((max.getTime() - min.getTime()) / 86400000));
    // 可用图表宽度 = 容器宽度 - 左侧列表宽度 - 右侧留白
    const containerWidth = containerRef.current?.clientWidth || 1200;
    const chartWidth = Math.max(containerWidth - 270, 200);
    // 每个级别估算项目渲染宽度（cols × min_column_width），选最细且不超宽的级别
    let bestLevel = ZOOM_LEVELS.length - 1; // 默认最粗（年）
    for (let i = 0; i < ZOOM_LEVELS.length; i++) {
      const lv = ZOOM_LEVELS[i];
      const colWidth = lv.min_column_width;
      const unit = lv.scales[0].unit;
      const step = lv.scales[0].step;
      let daysPerCol: number;
      if (unit === "day") daysPerCol = step;
      else if (unit === "week") daysPerCol = step * 7;
      else if (unit === "month") daysPerCol = step * 30;
      else daysPerCol = step * 365;
      const cols = Math.ceil(rangeDays / daysPerCol);
      const projectPixelWidth = cols * colWidth;
      if (projectPixelWidth <= chartWidth) {
        bestLevel = i; // 选中的是最细的"能完整放下"的级别
        break;
      }
    }
    setZoomLevel(bestLevel);
    try {
      (gantt.ext as any).zoom.setLevel(ZOOM_LEVELS[bestLevel].name);
    } catch { /* ignore */ }
  };

  // 甘特图页面全屏宽度
  useEffect(() => {
    const mainEl = document.querySelector(".main-content");
    if (mainEl) mainEl.classList.add("gantt-page");
    return () => { if (mainEl) mainEl.classList.remove("gantt-page"); };
  }, []);

  // === 初始化甘特图（仅一次）===
  useEffect(() => {
    if (initRef.current || !containerRef.current) return;
    initRef.current = true;

    gantt.config.date_format = "%Y-%m-%d";
    gantt.config.readonly = readonly;
    gantt.config.drag_links = !readonly;
    gantt.config.drag_move = false;
    gantt.config.drag_resize = false;
    gantt.config.drag_progress = false;
    gantt.config.autosize = "y";
    gantt.config.row_height = 28;
    gantt.config.scale_height = 40;
    gantt.config.min_column_width = 40;
    gantt.config.open_tree_initially = true;
    gantt.config.order_branch = true;
    gantt.config.order_branch_free = false;

    gantt.config.date_grid = "%M %d";
    gantt.templates.date_grid = gantt.date.date_to_str("%M %d") as any;
    gantt.templates.grid_date_format = gantt.date.date_to_str("%M %d") as any;

    (gantt.templates as any).scale_cell_class = function (date: Date) {
      if (date.getDay() === 0 || date.getDay() === 6) return "weekend-cell";
      return "";
    };

    // 左侧列
    gantt.config.columns = [
      { name: "id_col", label: "#", width: 36, align: "center",
        template: function (task: Record<string, any>) {
          // 项目内行号：按当前树展示顺序 1..N 连续编号，与数据库 id 解耦
          return `<span style="color:var(--text-muted);font-size:11px;">${(task.$index ?? 0) + 1}</span>`;
        } as any,
      },
      { name: "text", label: "任务名称", width: 220, tree: true,
        template: function (task: Record<string, any>) {
          // 状态三态色（与 Dashboard/TaskListView 统一）：未开始灰 / 进行中蓝 / 完成绿 / 延迟红
          const statusColors: Record<string, string> = {
            open: "var(--text-muted)", in_progress: "var(--accent)", completed: "var(--success)", delayed: "var(--danger)",
          };
          const color = statusColors[task.status] || "var(--text-muted)";
          const isParent = gantt.hasChild(task.id);
          const nameStyle = isParent ? "font-weight:600;color:var(--text-primary);" : "";
          const taskText = task.text || "";
          return `<span style="display:inline-flex;align-items:center;gap:4px;font-size:12px;" title="${taskText.replace(/"/g, '&quot;')}">
            <span style="display:inline-block;width:6px;height:6px;border-radius:50%;background:${color};flex-shrink:0;"></span>
            <span style="${nameStyle}">${taskText}</span>
            ${isParent ? '<span style="font-size:9px;color:var(--text-muted);">▾</span>' : ""}
          </span>`;
        } as any,
      },
      { name: "duration_col", label: "时长", width: 56, align: "center",
        template: function (task: Record<string, any>) {
          if (task.type === "milestone") return '<span style="color:var(--text-muted);">◆</span>';
          const d = task.duration_days ?? task.duration;
          return `<span style="font-size:12px;color:var(--text-secondary);">${d ?? ""}${d ? "d" : ""}</span>`;
        } as any,
      },
      { name: "progress_bar", label: "进度", width: 90, align: "center",
        template: function (task: Record<string, any>) {
          const pct = Math.round((task.progress || 0) * 100);
          return `<div style="display:flex;align-items:center;gap:6px;padding:0 4px;">
            <div style="flex:1;height:6px;border-radius:3px;background:var(--surface-alt);overflow:hidden;">
              <div style="height:100%;width:${pct}%;border-radius:3px;background:${pct >= 100 ? "var(--success)" : pct > 0 ? "var(--accent)" : "var(--text-muted)"};"></div>
            </div>
            <span style="font-size:11px;color:var(--text-secondary);min-width:28px;text-align:right;">${pct}%</span>
          </div>`;
        } as any,
      },
      { name: "assignee_col", label: "负责人", width: 72, align: "center",
        template: function (task: Record<string, any>) {
          return task.assignee || '<span style="color:var(--text-muted);">—</span>';
        } as any,
      },
    ];

    // 任务条内不显示百分比（dhtmlx 默认 progress_text 模板即返回空串，删除自定义模板）
    // 左侧"进度"列（progress_bar 列模板）保留百分比显示

    // 任务条样式：父任务深色粗体、超期红色、关键路径红色标记、选中高亮
    (gantt.templates as any).task_class = function (_s: Date, _e: Date, task: Record<string, any>) {
      const classes: string[] = [];
      if (task.status === "delayed") classes.push("gantt-task-delayed");
      if (task.status === "completed") classes.push("gantt-task-completed");
      if (task.status === "in_progress") classes.push("gantt-task-inprogress");
      if (task.status === "open") classes.push("gantt-task-open");
      if (gantt.hasChild(task.id)) classes.push("gantt-task-parent");
      // 关键路径：TF=0 的叶子/手动任务红色左缘标记（父任务由子任务汇总，不标记）
      if (task.critical && !gantt.hasChild(task.id) && !task.manual_scheduled) classes.push("gantt-task-critical");
      if (selectedTaskRef.current === task.id) classes.push("gantt-row-selected");
      const fm = useGanttStore.getState().focusMap;
      if (fm[task.id as number]) classes.push("gantt-task-focus");
      return classes.join(" ");
    };

    // 基线层：基线条紧贴任务条顶边、实际执行条紧贴任务条底边（均在 28px 行内）
    // 使用 onGanttRender 替代 addTaskLayer（dhtmlx-gantt MIT Edition 不支持 addTaskLayer）
    // 坐标系：append 到 .gantt_bars_area（与 .gantt_task_line 同容器），left 通过任务条 offsetLeft + 日期像素差计算
    // 日期解析：用 new Date(y, m-1, d) 构造本地时间，避免 new Date("YYYY-MM-DD") 的 UTC 时区偏移
    gantt.attachEvent("onGanttRender", function () {
      var dataArea = document.querySelector(".gantt_data_area");
      if (!dataArea) return;
      // 清除上一轮渲染的层元素
      dataArea.querySelectorAll(".baseline-layer-bar, .actual-layer-bar").forEach(function (el: any) { el.remove(); });
      var barsArea = dataArea.querySelector(".gantt_bars_area") as HTMLElement;
      if (!barsArea) return;
      gantt.eachTask(function (task: Record<string, any>) {
        // 空日期防御：任务无有效 start_date 则跳过
        var taskStart = task.start_date;
        if (!taskStart || (taskStart instanceof Date && isNaN(taskStart.getTime()))) return;
        var taskStartPos = (gantt as any).posFromDate(taskStart);
        // 查找任务条元素（用于获取坐标系基准 offsetLeft）
        var line = barsArea.querySelector('.gantt_task_line[task_id="' + task.id + '"]') as HTMLElement;
        if (!line) return;

        // 基线条
        if (task.baseline_start_date && task.baseline_end_date) {
          var bsParts = task.baseline_start_date.split("-");
          var beParts = task.baseline_end_date.split("-");
          if (bsParts.length === 3 && beParts.length === 3) {
            var bsDate = new Date(parseInt(bsParts[0]), parseInt(bsParts[1]) - 1, parseInt(bsParts[2]));
            var beDate = new Date(parseInt(beParts[0]), parseInt(beParts[1]) - 1, parseInt(beParts[2]));
            if (!isNaN(bsDate.getTime()) && !isNaN(beDate.getTime())) {
              var bsPos = (gantt as any).posFromDate(bsDate);
              var bePos = (gantt as any).posFromDate(beDate);
              var el = document.createElement("div");
              var left = line.offsetLeft + (bsPos - taskStartPos);
              var width = Math.max(2, bePos - bsPos);
              // 静态样式（position/height/background/pointer-events/三角）在 components.css 的 .baseline-layer-bar；窄条隐藏三角
              el.className = "baseline-layer-bar" + (width < 10 ? " no-arrow" : "");
              // 基线条紧贴任务条顶边：任务条 offsetTop - 4px（任务条高 20px 在 28px 行内居中，task bar top = line.offsetTop）
              el.style.cssText = "left:" + left + "px; top:" + (line.offsetTop - 4) + "px; width:" + width + "px;";
              barsArea.appendChild(el);
            }
          }
        }
        // 实际执行条
        if (task.actual_start) {
          var asParts = task.actual_start.split("-");
          if (asParts.length === 3) {
            var asDate = new Date(parseInt(asParts[0]), parseInt(asParts[1]) - 1, parseInt(asParts[2]));
            if (!isNaN(asDate.getTime())) {
              var asPos = (gantt as any).posFromDate(asDate);
              // actual_end 是字符串（可能为空），end_date 是 Date 对象
              var aeStr = task.actual_end;
              var aePos = asPos;
              if (aeStr && typeof aeStr === "string") {
                var aeParts2 = aeStr.split("-");
                if (aeParts2.length === 3) {
                  var aeDate = new Date(parseInt(aeParts2[0]), parseInt(aeParts2[1]) - 1, parseInt(aeParts2[2]));
                  if (!isNaN(aeDate.getTime())) aePos = (gantt as any).posFromDate(aeDate);
                }
              } else if (task.end_date instanceof Date) {
                aePos = (gantt as any).posFromDate(task.end_date);
              }
              var el = document.createElement("div");
              var left = line.offsetLeft + (asPos - taskStartPos);
              var width = Math.max(2, aePos - asPos);
              // 静态样式（position/height/background/pointer-events/三角）在 components.css 的 .actual-layer-bar；窄条隐藏三角
              el.className = "actual-layer-bar" + (width < 10 ? " no-arrow" : "");
              // 实际执行条紧贴任务条底边：任务条 offsetTop + offsetHeight
              el.style.cssText = "left:" + left + "px; top:" + (line.offsetTop + line.offsetHeight) + "px; width:" + width + "px;";
              barsArea.appendChild(el);
            }
          }
        }
      });
    });

    // 协作聚焦层
    if (typeof (gantt as any).addTaskLayer === "function") {
      (gantt as any).addTaskLayer(function drawFocus(task: Record<string, any>): any {
        const fm = useGanttStore.getState().focusMap;
        const info = fm[task.id as number];
        if (!info) return false;
        const color = getUserColor(info.userName);
        const el = document.createElement("div");
        el.style.cssText = `position:absolute; top:-18px; left:2px; font-size:11px; color:${color}; font-weight:600; white-space:nowrap; pointer-events:none; text-shadow:0 1px 2px rgba(255,255,255,.8);`;
        el.textContent = "▎" + info.userName;
        return el;
      });
    }

    // 今日线：GPL 版无 marker 插件（addMarker 不存在），改为自绘（drawMergedLinks 中绘制）

    // 缩放
    try {
      (gantt.ext as any).zoom.init({ levels: ZOOM_LEVELS });
      (gantt.ext as any).zoom.setLevel(ZOOM_LEVELS[4].name); // 默认"月"
    } catch (_) {}

    // 甘特图条悬停提示
    gantt.templates.tooltip_text = function (_s: Date, _e: Date, task: Record<string, any>) {
      const statusMap: Record<string, string> = { open: "未开始", in_progress: "进行中", completed: "已完成", delayed: "已延期" };
      const status = statusMap[task.status] || task.status || "—";
      const days = Math.ceil((new Date(task.end_date).getTime() - new Date(task.start_date).getTime()) / 86400000) + 1;
      return [
        `<b>${task.text}</b>`,
        `状态: ${status}　进度: ${Math.round((task.progress || 0) * 100)}%`,
        `日期: ${task.start_date} → ${task.end_date}（${days}天）`,
        task.assignee ? `负责人: ${task.assignee}` : "",
      ].filter(Boolean).join("<br>");
    };

    // 隐藏默认连线，改用自定义合并连线层（MS Project 风格：多前置先汇合再连入）
    gantt.config.show_links = false;

    gantt.init(containerRef.current);

    // === 自定义合并连线层（SVG）===
    const NS = "http://www.w3.org/2000/svg";
    const criticalLinkColor = "var(--danger)"; // 关键路径连线（源/目标均 TF=0）
    const altLinkColor = "var(--accent)";      // 备选路径连线（有富余）
    const svgLayer = document.createElementNS(NS, "svg");
    svgLayer.setAttribute("class", "gantt-merged-links");
    svgLayer.setAttribute("style", "position:absolute;inset:0;width:100%;height:100%;pointer-events:none;z-index:5;overflow:visible;");
    containerRef.current!.appendChild(svgLayer);

    const drawMergedLinks = () => {
      const c = containerRef.current;
      if (!svgLayer || !c || svgLayer.parentElement !== c) return;
      svgLayer.innerHTML = "";
      // 今日线（自绘）：画在 .gantt_bars_area 内容层内部（与任务条同坐标系，
      // 随滚动移动，天然覆盖全部任务纵向范围）。x = 今天日期单元格正中
      // （posFromDate 返回今天 0 点=格左缘，加日宽一半）。不能用固定 SVG 层——
      // 内容是滚动容器，固定层与内容纵向不同步会错位
      try {
        const dataArea = c.querySelector(".gantt_data_area") as HTMLElement;
        const barsArea = dataArea ? (dataArea.querySelector(".gantt_bars_area") as HTMLElement) : null;
        if (barsArea) {
          const now = new Date();
          const tomorrow = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
          const dayWidth = (gantt as any).posFromDate(tomorrow) - (gantt as any).posFromDate(now);
          const todayX = (gantt as any).posFromDate(now) + dayWidth / 2;
          if (typeof todayX === "number" && !isNaN(todayX) && dayWidth > 0) {
            barsArea.querySelector(".today-marker-line")?.remove();
            const tl = document.createElement("div");
            tl.className = "today-marker-line";
            tl.style.cssText = `position:absolute;top:0;bottom:0;left:${todayX}px;width:0;border-left:3px solid var(--primary,#2C6E6A);z-index:5;pointer-events:none;`;
            barsArea.appendChild(tl);
          }
        }
      } catch { /* 位置计算失败时跳过今日线 */ }
      const links = gantt.getLinks();
      if (!links || links.length === 0) return;
      const ganttRect = c.getBoundingClientRect();
      // 折叠父任务时连线聚合：任务条不可见（在折叠子树内，display:none）时，
      // 把它参与的连线提升到最近可见祖先（折叠的父任务）的条上绘制——折叠不中断
      const isVisible = (id: number) => {
        const n = gantt.getTaskNode(id);
        return !!n && n.offsetParent !== null; // display:none 的节点 offsetParent 为 null
      };
      const resolveVisible = (id: number): number => {
        let cur: any = gantt.getTask(id);
        let guard = 0;
        while (cur && !isVisible(cur.id) && guard < 20) {
          cur = cur.parent ? gantt.getTask(cur.parent) : null;
          guard++;
        }
        return cur ? Number(cur.id) : id;
      };
      // 收集所有可见任务条位置（相对 gantt 容器；折叠子树内任务不参与障碍检测）
      const barRects = new Map<number, { left: number; right: number; top: number; bottom: number; mid: number }>();
      gantt.eachTask((t: any) => {
        if (!isVisible(Number(t.id))) return;
        const n = gantt.getTaskNode(t.id);
        if (!n) return;
        const r = n.getBoundingClientRect();
        barRects.set(Number(t.id), {
          left: r.left - ganttRect.left,
          right: r.right - ganttRect.left,
          top: r.top - ganttRect.top,
          bottom: r.bottom - ganttRect.top,
          mid: r.top - ganttRect.top + r.height / 2,
        });
      });
      // 按后继任务分组：多前置合并画法，单前置标准 5 段折线。
      // 折叠时 source/target 提升到最近可见祖先（父任务条）→ 多个子任务依赖自动聚合到父任务
      const groups = new Map<number, any[]>();
      for (const l of links) {
        const tid = resolveVisible(Number(l.target));
        const sid = resolveVisible(Number(l.source));
        if (tid === sid) continue; // 提升后自连（同一折叠父任务内部的依赖，折叠时无外部意义）
        const arr = groups.get(tid) || [];
        arr.push(l);
        groups.set(tid, arr);
      }
      for (const [targetId, group] of groups) {
        const tRect = barRects.get(Number(targetId));
        if (!tRect) continue;
        const srcs = group
          .map((l) => ({ link: l, rect: barRects.get(resolveVisible(Number(l.source))) }))
          .filter((x) => x.rect) as Array<{ link: any; rect: { left: number; right: number; top: number; bottom: number; mid: number } }>;
        if (srcs.length === 0) continue;
        const srcIdSet = new Set(srcs.map((s) => resolveVisible(Number(s.link.source))));
        const ty = tRect.mid;
        const txEnd = tRect.left - 20;   // 目标侧垂直段 x（左缘外 20px）
        const txFinal = tRect.left - 8;  // 箭头起点（尖端 = 条左缘，绝不入条）
        // 线段是否穿过任意任务条（组内源/目标除外：端点接触是自然连接；贴边不算）
        const segHit = (x1: number, y1: number, x2: number, y2: number) => {
          const xMin = Math.min(x1, x2);
          const xMax = Math.max(x1, x2);
          const yMin2 = Math.min(y1, y2);
          const yMax2 = Math.max(y1, y2);
          for (const [bid, br] of barRects) {
            if (srcIdSet.has(bid) || bid === Number(targetId)) continue;
            // 水平段：x 为区间、y 为单点；垂直段：x 为单点、y 为区间（单点需严格在条内）
            const xIn = xMax - xMin > 0.5
              ? Math.max(xMin, br.left) < Math.min(xMax, br.right)
              : xMin > br.left + 0.5 && xMin < br.right - 0.5;
            const yIn = yMax2 - yMin2 > 0.5
              ? Math.max(yMin2, br.top) < Math.min(yMax2, br.bottom)
              : yMin2 > br.top + 0.5 && yMin2 < br.bottom - 0.5;
            if (xIn && yIn) return true;
          }
          return false;
        };
        // 空隙中央：在 [lo, hi] 上取所有条 y 区间并集的补集（空隙带），
        // 选最接近 (lo+hi)/2 且各线段都不穿条的中央（自动避开中间的条）
        const findGapMidY = (lo: number, hi: number, hMin: number, hMax: number, ok: (y: number) => boolean) => {
          const occ: Array<[number, number]> = [];
          for (const [, br] of barRects) {
            if (br.right > hMin && br.left < hMax) occ.push([br.top, br.bottom]);
          }
          occ.sort((a, b) => a[0] - b[0]);
          const merged: Array<[number, number]> = [];
          for (const [a, b] of occ) {
            const last = merged[merged.length - 1];
            if (last && a <= last[1]) last[1] = Math.max(last[1], b);
            else merged.push([a, b]);
          }
          const bands: Array<[number, number]> = [];
          let cursor = lo;
          for (const [a, b] of merged) {
            const s2 = Math.max(a, lo);
            const e2 = Math.min(b, hi);
            if (s2 >= e2) continue;
            if (s2 > cursor) bands.push([cursor, s2]);
            cursor = Math.max(cursor, e2);
          }
          if (cursor < hi) bands.push([cursor, hi]);
          const target = (lo + hi) / 2;
          let best = target;
          let bestDist = Infinity;
          for (const [a, b] of bands) {
            const cand = (a + b) / 2;
            const dist = Math.abs(cand - target);
            if (dist < bestDist) { bestDist = dist; best = cand; }
            if (ok(cand)) return cand;
          }
          return best;
        };
        // 折线点序列（连续重复点合并），拐角用圆弧过渡。
        // 半径 5px，但受相邻段长约束：段长 < 2r 时自动缩小（否则短段两端圆弧重叠成直角）
        const LINK_CORNER_R = 5;
        const toPath = (pts: Array<[number, number]>) => {
          const out: Array<[number, number]> = [];
          for (const p of pts) {
            const last = out[out.length - 1];
            if (!last || last[0] !== p[0] || last[1] !== p[1]) out.push(p);
          }
          if (out.length < 3) return "M " + out.map((p) => p.join(" ")).join(" L ");
          const r = LINK_CORNER_R;
          let d = `M ${out[0][0]} ${out[0][1]}`;
          for (let i = 1; i < out.length - 1; i++) {
            const p0 = out[i - 1];
            const p1 = out[i];
            const p2 = out[i + 1];
            const segIn = Math.hypot(p1[0] - p0[0], p1[1] - p0[1]);
            const segOut = Math.hypot(p2[0] - p1[0], p2[1] - p1[1]);
            // 有效半径：全局半径受相邻段长一半约束（短段自动缩小，避免圆弧重叠）
            const rr = Math.min(r, segIn / 2, segOut / 2);
            if (rr <= 0) {
              d += ` L ${p1[0]} ${p1[1]}`;
              continue;
            }
            // 进入/离开方向单位向量
            const ux = (p1[0] - p0[0]) / segIn;
            const uy = (p1[1] - p0[1]) / segIn;
            const vx = (p2[0] - p1[0]) / segOut;
            const vy = (p2[1] - p1[1]) / segOut;
            // 圆角起点 = 拐角沿进入方向后退 rr；终点 = 沿离开方向前进 rr。
            // 用二次贝塞尔 Q(控制点=拐角)圆滑过渡——无 A 命令 sweep 方向的渲染歧义
            const sx = p1[0] - ux * rr;
            const sy = p1[1] - uy * rr;
            const ex = p1[0] + vx * rr;
            const ey = p1[1] + vy * rr;
            d += ` L ${sx} ${sy} Q ${p1[0]} ${p1[1]} ${ex} ${ey}`;
          }
          const last = out[out.length - 1];
          d += ` L ${last[0]} ${last[1]}`;
          return d;
        };
        // 连线颜色：源和目标都在关键路径（TF=0）→ 红；否则（备选/富余路径）→ 蓝
        const isCritical = (id: number) => {
          const t = gantt.getTask(id);
          return !!(t && (t as any).critical);
        };
        const linkColorFor = (link: any) =>
          isCritical(Number(link.source)) && isCritical(Number(link.target)) ? criticalLinkColor : altLinkColor;
        const targetCritical = isCritical(Number(targetId));
        const sharedColor = targetCritical ? criticalLinkColor : altLinkColor;

        // 构建绘制段 [{ d, color, link }]：多前置时每条源线独立画到汇合点，共享段单独画
        const segs: Array<{ d: string; color: string; link: any }> = [];
        if (srcs.length === 1) {
          // 单条：源右缘 → 右 20 → 下到空隙中央 → 左到目标外 20 → 下到目标中线 → 连入
          const s = srcs[0].rect;
          const sx = s.right;
          const sy = s.mid;
          const v1x = sx + 20;
          const midY = findGapMidY(Math.min(sy, ty), Math.max(sy, ty), Math.min(v1x, txEnd), Math.max(v1x, txEnd), (y) =>
            !segHit(v1x, sy, v1x, y) && !segHit(v1x, y, txEnd, y) && !segHit(txEnd, y, txEnd, ty));
          segs.push({
            d: toPath([[sx, sy], [v1x, sy], [v1x, midY], [txEnd, midY], [txEnd, ty], [txFinal, ty]]),
            color: linkColorFor(srcs[0].link),
            link: srcs[0].link,
          });
        } else {
          // 多前置合并：公共右边界（时间最长的条右缘）+20 处汇合下折；
          // 公共下边界（任务列表最下面的源底边）与目标之间的空隙中央水平穿过
          const comX = Math.max(...srcs.map((s) => s.rect.right)) + 20;
          const comBottom = Math.max(...srcs.map((s) => s.rect.bottom));
          const midY = findGapMidY(comBottom, tRect.top, Math.min(comX, txEnd), Math.max(comX, txEnd), (y) =>
            srcs.every((s) => !segHit(comX, s.rect.mid, comX, y)) &&
            !segHit(comX, y, txEnd, y) && !segHit(txEnd, y, txEnd, ty));
          for (const s of srcs) {
            segs.push({
              d: toPath([[s.rect.right, s.rect.mid], [comX, s.rect.mid], [comX, midY]]),
              color: linkColorFor(s.link),
              link: s.link,
            });
          }
          // 共享段（汇合后 → 目标）：颜色随目标是否关键
          segs.push({
            d: toPath([[comX, midY], [txEnd, midY], [txEnd, ty], [txFinal, ty]]),
            color: sharedColor,
            link: null,
          });
        }
        // 画路径
        for (const seg of segs) {
          const path = document.createElementNS(NS, "path");
          path.setAttribute("d", seg.d);
          path.setAttribute("fill", "none");
          path.setAttribute("stroke", seg.color);
          path.setAttribute("stroke-width", "1.5");
          path.setAttribute("stroke-linejoin", "round");
          path.style.pointerEvents = "stroke";
          path.style.cursor = "pointer";
          path.addEventListener("dblclick", (e) => {
            e.stopPropagation();
            if (readonlyRef.current) return;
            if (seg.link) {
              if (window.confirm("删除此依赖关系？")) {
                gantt.deleteLink(seg.link.id);
                setTimeout(drawMergedLinks, 50);
              }
            } else if (window.confirm(`删除这 ${group.length} 条依赖关系？`)) {
              for (const l of group) gantt.deleteLink(l.id);
              setTimeout(drawMergedLinks, 50);
            }
          });
          svgLayer.appendChild(path);
        }
        // 箭头（尖端 = 条左缘；颜色随目标是否关键；双击删除整组依赖）
        const arrow = document.createElementNS(NS, "polygon");
        arrow.setAttribute("points", `${txFinal},${ty - 4} ${txFinal + 8},${ty} ${txFinal},${ty + 4}`);
        arrow.setAttribute("fill", sharedColor);
        arrow.addEventListener("dblclick", (e) => {
          e.stopPropagation();
          if (readonlyRef.current) return;
          if (window.confirm(`删除这 ${group.length} 条依赖关系？`)) {
            for (const l of group) gantt.deleteLink(l.id);
            setTimeout(drawMergedLinks, 50);
          }
        });
        svgLayer.appendChild(arrow);
      }
    };

    // 数据渲染/滚动/缩放/折叠展开后重绘合并连线
    gantt.attachEvent("onDataRender", () => setTimeout(drawMergedLinks, 0));
    gantt.attachEvent("onGanttScroll", () => setTimeout(drawMergedLinks, 0));
    gantt.attachEvent("onCollapse", () => setTimeout(drawMergedLinks, 0));
    gantt.attachEvent("onExpand", () => setTimeout(drawMergedLinks, 0));
    (window as any).__drawMergedLinks = drawMergedLinks;

    setGanttReady(true);
    selectedTaskRef.current = null;

    // === 事件 ===

    // 行拖拽排序后同步 sort_order（全局重排：按当前树展示顺序重新编号，跳过未变任务）
    // 注意：dhtmlx 行排序拖拽触发 onRowDragEnd（此时已本地重排），任务条拖拽已被禁用（drag_move=false）
    const saveRowOrder = async () => {
      if (readonlyRef.current) return;
      try {
        // eachTask 深度优先遍历 = 展示顺序
        const order: number[] = [];
        gantt.eachTask(function (t: Record<string, any>) { order.push(t.id as number); });
        let changed = 0;
        for (let idx = 0; idx < order.length; idx++) {
          const g = gantt.getTask(order[idx]) as Record<string, any>;
          if (g.sort_order === idx) continue; // 未变动的任务跳过，减少请求
          try {
            await api.patch(`/api/projects/${projectId}/tasks/${order[idx]}/sort_order`, {
              sort_order: idx,
              version: g.version ?? 0,
            });
            changed++;
          } catch (err: any) {
            if (err.response?.status === 409) {
              alert("排序保存失败：任务已被他人修改，数据已刷新");
            } else {
              alert("排序保存失败，请重试");
            }
            fetchData(projectId, readonlyRef.current);
            return;
          }
        }
        if (changed > 0) fetchData(projectId, readonlyRef.current);
      } catch { /* ignore */ }
    };
    gantt.attachEvent("onRowDragEnd", function () { saveRowOrder(); });

    // 拖拽连线创建依赖
    gantt.attachEvent("onAfterLinkAdd", async function (_id: unknown, link: { source: any; target: any; type: any }) {
      if (readonlyRef.current) return;
      await addLink({ id: 0, source: Number(link.source), target: Number(link.target), type: String(link.type || "0"), lag: 0 }, projectId);
    });

    // 双击连线 → 确认删除（setTimeout 避免 dhtmlx 内部状态冲突）
    gantt.attachEvent("onLinkDblClick", function (id: unknown) {
      if (readonlyRef.current) return false;
      if (!confirm("删除此依赖关系？")) return false;
      const linkId = Number(id);
      setTimeout(() => { deleteLink(linkId, projectId); }, 50);
      return false;
    });

    // 双击任务 → 打开详情弹窗（使用 ref 避免闭包过期）
    gantt.attachEvent("onTaskDblClick", function (id: unknown) {
      if (readonlyRef.current) return true;
      const tid = Number(id);
      const liveTasks = allTasksRef.current;
      const t = liveTasks.find((t) => t.id === tid);
      if (t) {
        buildRowNumbers();
        setModalTask(t);
        setShowModal(true);
        return false; // 阻止默认 lightbox
      }
      return true;
    });

    // 阻止父任务连线
    gantt.attachEvent("onBeforeLinkAdd", function (_id: unknown, link: { source: any; target: any }) {
      const sid = Number(link.source);
      const tid = Number(link.target);
      if (gantt.hasChild(sid) || gantt.hasChild(tid)) {
        alert("父任务不接受依赖连线，请对子任务建立依赖关系");
        return false;
      }
      return true;
    });

    // 协作聚焦 + 行选中（通过 state 触发 render，不干扰双击）
    gantt.attachEvent("onTaskClick", function (id: unknown) {
      handleTaskClick(Number(id));
      selectedTaskRef.current = Number(id);
      setSelectedTaskId(Number(id));
      return true;
    });
    gantt.attachEvent("onEmptyClick", function () {
      handleEmptyClick();
      selectedTaskRef.current = null;
      setSelectedTaskId(null);
      return true;
    });

    return () => { gantt.clearAll(); initRef.current = false; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 数据变化时刷新甘特图
  useEffect(() => {
    if (!ganttReady || loading) return;
    gantt.clearAll();
    gantt.config.readonly = readonly;
    gantt.config.drag_links = !readonly;
    if (tasks.length > 0) {
      (gantt as any).parse({ data: tasks, links: links });
      // dhtmlx 按 end-start 计算 duration（结束日不计入），1 天任务会算出 0 而甘特条不可见——
      // 用后端工期强制覆盖，保证所有任务的甘特条宽度正确
      gantt.eachTask((t: any) => {
        if (typeof t.duration_days === "number") {
          t.duration = Math.max(1, t.duration_days);
        }
      });
      gantt.render();
      // 自定义合并连线重绘（依赖数据变化后）
      setTimeout(() => (window as any).__drawMergedLinks?.(), 50);
      if (!autoZoomDoneRef.current) {
        autoZoomDoneRef.current = true;
        setTimeout(() => {
          try { handleAutoZoom(); } catch { /* ignore */ }
        }, 100);
      }
    }
  }, [tasks, links, readonly, loading]);

  // 协作聚焦重绘
  useEffect(() => {
    if (!ganttReady) return;
    gantt.render();
  }, [focusMap]);

  // 行选中重绘（通过 useEffect 延迟 render，不干扰双击事件）
  useEffect(() => {
    if (!ganttReady) return;
    gantt.render();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedTaskId]);

  return (
    <div className="gantt-wrapper">
      <div className="gantt-toolbar">
        <div className="gantt-toolbar-left">
          {!readonly && (
            <button className="btn btn-primary btn-sm" onClick={handleAddTask}>
              + 添加任务
            </button>
          )}
          <button
            className="btn btn-ghost btn-sm"
            onClick={() => fetchData(projectId, readonly)}
            title="重新加载数据"
          >
            ↻ 刷新
          </button>
          {!readonly && (
            <button
              className="btn btn-ghost btn-sm"
              title="恢复已删除的任务"
              onClick={() => setShowRecycleBin(true)}
            >
              回收站
            </button>
          )}
          <span className="gantt-toolbar-hint">双击任务编辑详情 · 双击连线删除</span>
        </div>
        <div className="gantt-toolbar-right">
          <div className="baseline-menu-wrap">
            <button
              className={`btn-zoom btn-baseline${baselineMeta ? " has-baseline" : ""}`}
              onClick={(e) => { e.stopPropagation(); setBaselineMenuOpen(!baselineMenuOpen); }}
              title="基线管理"
            >
              基线{baselineMeta ? " ✓" : " ▾"}
            </button>
            {baselineMenuOpen && (
              <div className="baseline-menu" onClick={(e) => e.stopPropagation()}>
                {baselineMeta ? (
                  <>
                    <div className="baseline-menu-info">
                      创建: {baselineMeta.created_at} · {baselineMeta.created_by}
                      <br />快照 {baselineMeta.task_count} 个任务
                    </div>
                    <button
                      className="baseline-menu-item"
                      onClick={async () => {
                        if (!window.confirm("重新创建基线将覆盖当前基线，确定？")) return;
                        const ok = await createBaseline(projectId);
                        if (ok) setBaselineMenuOpen(false);
                      }}
                    >
                      重新创建基线
                    </button>
                    <button
                      className="baseline-menu-item danger"
                      onClick={async () => {
                        if (!window.confirm("清除基线后无法恢复，确定？")) return;
                        const ok = await clearBaseline(projectId);
                        if (ok) setBaselineMenuOpen(false);
                      }}
                    >
                      清除基线
                    </button>
                  </>
                ) : (
                  <button
                    className="baseline-menu-item"
                    onClick={async () => {
                      const ok = await createBaseline(projectId);
                      if (ok) setBaselineMenuOpen(false);
                    }}
                  >
                    创建基线（快照当前计划）
                  </button>
                )}
              </div>
            )}
          </div>
          <span className="gantt-toolbar-sep" />
          <span className="gantt-zoom-label">缩放</span>
          <button className="btn-zoom" onClick={() => handleZoom(-1)} title="缩小">−</button>
          <span className="gantt-zoom-level">{ZOOM_LABELS[zoomLevel]}</span>
          <button className="btn-zoom" onClick={() => handleZoom(1)} title="放大">+</button>
          <button className="btn-zoom btn-auto-zoom" onClick={handleAutoZoom} title="自动适应项目时间范围">⊡</button>
        </div>
      </div>

      {loading && <div className="gantt-loading">加载甘特图...</div>}
      {!loading && tasks.length === 0 && (
        <div className="gantt-loading">{readonly ? "暂无任务" : "点击「+ 添加任务」开始规划"}</div>
      )}

      <div ref={containerRef} className="gantt-container" style={{ display: loading ? "none" : "block" }} />

      {showModal && (
        <TaskDetailModal
          projectId={projectId}
          task={modalTask}
          allTasks={allTasks}
          rowNumbers={rowNumbers}
          onClose={() => { setShowModal(false); setModalTask(null); }}
          onSaved={handleModalSaved}
        />
      )}

      {showRecycleBin && (
        <RecycleBinModal
          projectId={projectId}
          projectName={projectName || "项目"}
          onClose={() => setShowRecycleBin(false)}
          onRestored={() => fetchData(projectId, readonly)}
        />
      )}
    </div>
  );
}
