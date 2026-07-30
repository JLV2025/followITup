import { useEffect, useRef, useState } from "react";
import { useParams } from "react-router-dom";
import { gantt } from "dhtmlx-gantt";
import "dhtmlx-gantt/codebase/dhtmlxgantt.css";
import { useGanttStore } from "../stores/ganttStore";
import { useAuthStore } from "../stores/authStore";
import { wsClient } from "../api/ws-client";
import api from "../api/client";
import TaskDetailModal from "../components/TaskDetailModal";

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
  manual_scheduled: boolean;
  constraint_type: string;
  constraint_date: string;
  sort_order: number;
  version: number;
}

const USER_COLORS = [
  "#5B8DEF", "#F5A623", "#7ED321", "#D0021B", "#BD10E0",
  "#4A90D9", "#F8E71C", "#50E3C2", "#9013FE", "#FF6B6B",
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

const ZOOM_LABELS = ["日", "周", "月", "季", "年"];
const ZOOM_LEVELS = ["day", "week", "month", "quarter", "year"];

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
  const [zoomLevel, setZoomLevel] = useState(2);

  const [modalTask, setModalTask] = useState<Task | null>(null);
  const [showModal, setShowModal] = useState(false);
  const [allTasks, setAllTasks] = useState<Task[]>([]);

  const {
    tasks, links, focusMap, loading,
    fetchData, addLink, deleteLink,
    setFocus, clearFocus, pruneExpired,
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
      else if (msg.type === "task_update") fetchData(projectId, readonlyRef.current);
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

  /** 缩放 */
  const handleZoom = (dir: -1 | 1) => {
    const newLevel = zoomLevel + dir;
    if (newLevel < 0 || newLevel >= ZOOM_LEVELS.length) return;
    setZoomLevel(newLevel);
    try {
      (gantt.ext as any).zoom.setLevel(ZOOM_LEVELS[newLevel]);
    } catch { /* ignore */ }
  };

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
    gantt.config.row_height = 36;
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
      { name: "id_col", label: "#", width: 46, align: "center",
        template: function (task: Record<string, any>) {
          return `<span style="color:var(--text-muted);font-size:12px;">${task.id}</span>`;
        } as any,
      },
      { name: "text", label: "任务名称", width: "*", tree: true,
        template: function (task: Record<string, any>) {
          const statusColors: Record<string, string> = {
            open: "#9ca3af", in_progress: "#3b82f6", completed: "#22c55e", delayed: "#ef4444",
          };
          const color = statusColors[task.status] || "#9ca3af";
          const isParent = gantt.hasChild(task.id);
          const nameStyle = isParent ? "font-weight:600;color:var(--text-primary);" : "";
          return `<span style="display:inline-flex;align-items:center;gap:6px;">
            <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${color};flex-shrink:0;"></span>
            <span style="${nameStyle}">${task.text || ""}</span>
            ${isParent ? '<span style="font-size:10px;color:var(--text-muted);">▾</span>' : ""}
          </span>`;
        } as any,
      },
      { name: "progress_bar", label: "进度", width: 90, align: "center",
        template: function (task: Record<string, any>) {
          const pct = Math.round((task.progress || 0) * 100);
          return `<div style="display:flex;align-items:center;gap:6px;padding:0 4px;">
            <div style="flex:1;height:6px;border-radius:3px;background:#f3f4f6;overflow:hidden;">
              <div style="height:100%;width:${pct}%;border-radius:3px;background:${pct >= 100 ? '#22c55e' : '#3b82f6'};"></div>
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

    (gantt.templates as any).progress_text = function (_s: Date, _e: Date, task: Record<string, any>) {
      return Math.round((task.progress || 0) * 100) + "%";
    };

    // 任务条样式：父任务深色粗体、超期红色
    (gantt.templates as any).task_class = function (_s: Date, _e: Date, task: Record<string, any>) {
      const classes: string[] = [];
      if (task.status === "delayed") classes.push("gantt-task-delayed");
      if (gantt.hasChild(task.id)) classes.push("gantt-task-parent");
      const fm = useGanttStore.getState().focusMap;
      if (fm[task.id as number]) classes.push("gantt-task-focus");
      return classes.join(" ");
    };

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

    // 今日线
    try {
      gantt.plugins({ marker: true });
      (gantt as any).addMarker({ start_date: new Date(), css: "today-marker", title: "今天" });
    } catch (_) {}

    // 缩放
    try {
      (gantt.ext as any).zoom.init({
        levels: [
          { name: "day", scale_height: 50, min_column_width: 80, scales: [{ unit: "day", step: 1, format: "%d" }] },
          { name: "week", scale_height: 50, min_column_width: 60,
            scales: [{ unit: "week", step: 1, format: "W%W" }, { unit: "day", step: 1, format: "%a" }] },
          { name: "month", scale_height: 50, min_column_width: 120,
            scales: [{ unit: "month", step: 1, format: "%M" }, { unit: "day", step: 1, format: "%d" }] },
          { name: "quarter", scale_height: 50, min_column_width: 100,
            scales: [{ unit: "month", step: 3, format: "%M" }, { unit: "day", step: 1, format: "%d" }] },
          { name: "year", scale_height: 50, min_column_width: 80, scales: [{ unit: "month", step: 1, format: "%m月" }] },
        ],
      });
    } catch (_) {}

    gantt.init(containerRef.current);
    setGanttReady(true);

    // === 事件 ===

    // 拖拽排序后同步 sort_order
    gantt.attachEvent("onAfterTaskDrag", async function (id: unknown, mode: string) {
      if (readonlyRef.current || mode !== "move") return;
      try {
        const task = gantt.getTask(Number(id)) as Record<string, any> | null;
        if (!task) return;
        const parent = task.parent || 0;
        const siblings: { taskId: number; order: number }[] = [];
        gantt.eachTask(function (t: Record<string, any>) {
          if ((t.parent || 0) === parent) siblings.push({ taskId: t.id as number, order: (t as any).$index || 0 });
        });
        for (const s of siblings) {
          await api.put(`/api/projects/${projectId}/tasks/${s.taskId}`, { sort_order: s.order, version: 0 });
        }
        fetchData(projectId, readonlyRef.current);
      } catch { /* ignore */ }
    });

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

    // 协作聚焦
    gantt.attachEvent("onTaskClick", function (id: unknown) { handleTaskClick(Number(id)); return true; });
    gantt.attachEvent("onEmptyClick", function () { handleEmptyClick(); return true; });

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
    }
  }, [tasks, links, readonly, loading]);

  // 协作聚焦重绘
  useEffect(() => {
    if (!ganttReady) return;
    gantt.render();
  }, [focusMap]);

  return (
    <div className="gantt-wrapper">
      <div className="gantt-toolbar">
        <div className="gantt-toolbar-left">
          {!readonly && (
            <button className="btn btn-primary btn-sm" onClick={handleAddTask}>
              + 添加任务
            </button>
          )}
          <span className="gantt-toolbar-hint">双击任务编辑详情 · 双击连线删除</span>
        </div>
        <div className="gantt-toolbar-right">
          <span className="gantt-zoom-label">缩放</span>
          <button className="btn-zoom" onClick={() => handleZoom(-1)} title="缩小">−</button>
          <span className="gantt-zoom-level">{ZOOM_LABELS[zoomLevel]}</span>
          <button className="btn-zoom" onClick={() => handleZoom(1)} title="放大">+</button>
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
          onClose={() => { setShowModal(false); setModalTask(null); }}
          onSaved={handleModalSaved}
        />
      )}
    </div>
  );
}
