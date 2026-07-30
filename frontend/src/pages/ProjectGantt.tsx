import { useEffect, useRef, useCallback, useState } from "react";
import { useParams } from "react-router-dom";
import { gantt } from "dhtmlx-gantt";
import "dhtmlx-gantt/codebase/dhtmlxgantt.css";
import { useGanttStore } from "../stores/ganttStore";
import { useAuthStore } from "../stores/authStore";
import { wsClient } from "../api/ws-client";
import api from "../api/client";
import { fmtDateAPI } from "../utils/date";

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

export default function ProjectGantt({ readonly }: { readonly: boolean }) {
  const { id } = useParams<{ id: string }>();
  const projectId = Number(id);
  const containerRef = useRef<HTMLDivElement>(null);
  const initRef = useRef(false);
  const lastFocusedRef = useRef<number>(0);
  const [ganttReady, setGanttReady] = useState(false);

  const { tasks, links, focusMap, loading, fetchData, updateTask, addLink, deleteLink, setFocus, clearFocus, pruneExpired } =
    useGanttStore();

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
      else if (msg.type === "task_update") fetchData(projectId, readonly);
    });
    return unsub;
  }, [projectId, readonly, fetchData, setFocus, clearFocus]);

  useEffect(() => {
    const timer = setInterval(() => pruneExpired(), 10000);
    return () => clearInterval(timer);
  }, [pruneExpired]);

  useEffect(() => {
    fetchData(projectId, readonly);
  }, [projectId, readonly, fetchData]);

  const handleTaskClick = useCallback((id: number) => {
    if (!isLoggedIn) return;
    if (lastFocusedRef.current && lastFocusedRef.current !== id) wsClient.sendBlur(lastFocusedRef.current);
    wsClient.sendFocus(id);
    lastFocusedRef.current = id;
  }, [isLoggedIn]);

  const handleEmptyClick = useCallback(() => {
    if (lastFocusedRef.current) { wsClient.sendBlur(lastFocusedRef.current); lastFocusedRef.current = 0; }
  }, []);

  /** 添加新任务（React 按钮，不依赖 gantt 事件） */
  const handleAddTask = async () => {
    if (readonly) return;
    const name = window.prompt("任务名称:", "新任务");
    if (name == null) return; // 取消
    const assignee = window.prompt("负责人（可留空）:", "") || "";
    try {
      await api.post(`/api/projects/${projectId}/tasks`, {
        name: name.trim() || "新任务",
        assignee: assignee.trim(),
        task_type: "task",
        status: "open",
        priority: "medium",
        start_date: fmtDateAPI(null),
        duration_days: 1,
      });
      fetchData(projectId, readonly);
    } catch { /* ignore */ }
  };

  // 初始化甘特图（仅一次）
  useEffect(() => {
    if (initRef.current || !containerRef.current) return;
    initRef.current = true;

    gantt.config.date_format = "%Y-%m-%d";
    gantt.config.readonly = readonly;
    gantt.config.drag_links = !readonly;
    gantt.config.drag_move = !readonly;
    gantt.config.drag_resize = !readonly;
    gantt.config.drag_progress = !readonly;
    gantt.config.autosize = "y";
    gantt.config.row_height = 36;
    gantt.config.open_tree_initially = true;
    gantt.config.order_branch = true;
    gantt.config.order_branch_free = false;

    // 显示格式：Aug 02
    gantt.config.date_grid = "%M %d";
    gantt.templates.date_grid = gantt.date.date_to_str("%M %d") as any;
    gantt.templates.grid_date_format = gantt.date.date_to_str("%M %d") as any;
    // 周末灰色背景
    (gantt.templates as any).scale_cell_class = function (date: Date) {
      if (date.getDay() === 0 || date.getDay() === 6) return "weekend-cell";
      return "";
    };

    gantt.config.columns = [
      { name: "text", label: "任务名称", width: "*", tree: true },
      { name: "start_date", label: "开始", width: 100, align: "center" },
      { name: "assignee", label: "负责人", width: 80, align: "center",
        template: function (task: Record<string, any>) {
          const val = task.assignee || "";
          if (gantt.config.readonly) return `<span>${val || "—"}</span>`;
          return `<span class="cell-editable" style="cursor:pointer;display:inline-block;width:100%"
            onclick="event.stopPropagation();var n=prompt('负责人:', '${val.replace(/'/g, "\\'")}');if(n!=null){var g=window.gantt;var t=g.getTask(${task.id});t.assignee=n;g.updateTask(${task.id});g.refreshTask(${task.id});}" title="点击编辑">${val || "—"}</span>`;
        } as any,
      },
      { name: "duration", label: "天数", width: 60, align: "center" },
    ];

    (gantt.templates as any).progress_text = function (_s: Date, _e: Date, task: Record<string, any>) {
      return Math.round((task.progress || 0) * 100) + "%";
    };

    (gantt.templates as any).task_class = function (_s: Date, _e: Date, task: Record<string, any>) {
      const classes: string[] = [];
      if (task.status === "delayed") classes.push("gantt-task-delayed");
      const fm = useGanttStore.getState().focusMap;
      if (fm[task.id as number]) classes.push("gantt-task-focus");
      return classes.join(" ");
    };

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

    try {
      gantt.plugins({ marker: true });
      (gantt as any).addMarker({ start_date: new Date(), css: "today-marker", title: "今天" });
    } catch (_) {}

    gantt.init(containerRef.current);
    setGanttReady(true);

    gantt.attachEvent("onAfterTaskUpdate", async function (id: number, _item: unknown) {
      if (gantt.config.readonly) return;
      const task = gantt.getTask(id) as Record<string, unknown>;
      if (!task) return;
      await updateTask(id, {
        start_date: fmtDateAPI(task.start_date),
        end_date: fmtDateAPI(task.end_date),
        duration: Number(task.duration) || 1,
        progress: Number(task.progress) || 0,
        parent: Number(task.parent) || 0,
        assignee: String(task.assignee || ""),
      }, projectId);
    });

    gantt.attachEvent("onAfterLinkAdd", async function (_id: string | number, link: { source: string | number; target: string | number; type: string | number }) {
      if (gantt.config.readonly) return;
      await addLink({ id: 0, source: Number(link.source), target: Number(link.target), type: String(link.type || "0"), lag: 0 }, projectId);
    });
    gantt.attachEvent("onAfterLinkDelete", async function (id: string | number) {
      if (gantt.config.readonly) return;
      await deleteLink(Number(id), projectId);
    });

    gantt.attachEvent("onTaskClick", function (id: number) { handleTaskClick(id); return true; });
    gantt.attachEvent("onEmptyClick", function () { handleEmptyClick(); return true; });

    return () => { gantt.clearAll(); initRef.current = false; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!ganttReady || loading) return;
    gantt.clearAll();
    gantt.config.readonly = readonly;
    gantt.config.drag_links = !readonly;
    gantt.config.drag_move = !readonly;
    gantt.config.drag_resize = !readonly;
    gantt.config.drag_progress = !readonly;
    if (tasks.length > 0) {
      (gantt as any).parse({ data: tasks, links: links });
    }
  }, [tasks, links, readonly, loading]);

  useEffect(() => {
    if (!ganttReady) return;
    gantt.render();
  }, [focusMap]);

  return (
    <div className="gantt-wrapper">
      {!readonly && (
        <div style={{ display: "flex", justifyContent: "flex-end", padding: "8px 12px 0 0" }}>
          <button className="btn btn-primary btn-sm" onClick={handleAddTask}>
            + 添加任务
          </button>
        </div>
      )}
      {loading && <div className="gantt-loading">加载甘特图...</div>}
      {!loading && tasks.length === 0 && (
        <div className="gantt-loading">暂无任务，请在任务列表中创建</div>
      )}
      <div ref={containerRef} className="gantt-container" style={{ display: loading ? "none" : "block", height: readonly ? "calc(100vh - 200px)" : "calc(100vh - 240px)" }} />
    </div>
  );
}
