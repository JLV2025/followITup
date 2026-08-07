/**
 * dhtmlx-gantt 数据格式适配层
 * 将我们的 Task/Dependency 模型 ↔ dhtmlx-gantt 期望的 JSON 格式
 */

// dhtmlx-gantt 期望的任务格式
export interface GanttTask {
  id: number;
  parent: number;        // 0 = 顶级任务
  text: string;          // 任务名
  start_date: string;    // YYYY-MM-DD
  end_date: string;      // YYYY-MM-DD（dhtmlx 不使用 duration）
  duration: number;      // 天数
  progress: number;      // 0.0 ~ 1.0
  type?: string;         // "task" | "milestone" | "project"
  task_type?: string;    // 原始 task_type（task | milestone）
  $open?: boolean;       // 初始展开状态
  // 自定义扩展
  status?: string;
  assignee?: string;
  priority?: string;
  manual_scheduled?: boolean;
  constraint_type?: string;
  constraint_date?: string;
  baseline_start_date?: string;
  baseline_end_date?: string;
  actual_start?: string;
  actual_end?: string;
  version?: number;
  sort_order?: number;   // 项目内排序序号（拖拽持久化用）
  duration_days?: number; // 后端工期（parse 后强制覆盖 dhtmlx 按 end-start 算出的 duration）
  critical?: boolean;    // 关键路径任务（TF=0）
  $readonly?: boolean;   // 只读模式
}

// dhtmlx-gantt 期望的依赖格式
export interface GanttLink {
  id: number;
  source: number;        // predecessor_id
  target: number;        // successor_id
  type: string;          // "0"=FS, "1"=SS, "2"=FF, "3"=SF
  lag: number;           // lag_days
}

// 依赖类型映射
const DEP_TYPE_TO_GANTT: Record<string, string> = {
  FS: "0", SS: "1", FF: "2", SF: "3",
};

const GANTT_TO_DEP_TYPE: Record<string, string> = {
  "0": "FS", "1": "SS", "2": "FF", "3": "SF",
};

/**
 * 将后端 Task → dhtmlx-gantt Task
 */
export function toGanttTask(t: any, readonly: boolean): GanttTask {
  const startDate = t.start_date || "";
  // 若无 end_date 或格式异常（Date 对象被序列化），从 start_date + duration_days 推算
  let endDate = t.end_date || "";
  let duration = t.duration_days || 1;
  const isValidDate = (d: string) => /^\d{4}-\d{2}-\d{2}$/.test(d);
  if (!endDate || !isValidDate(endDate) || !startDate) {
    if (startDate && isValidDate(startDate)) {
      const s = new Date(startDate);
      if (!isNaN(s.getTime())) {
        s.setDate(s.getDate() + duration - 1);
        endDate = s.toISOString().slice(0, 10);
      }
    }
  } else if (endDate && startDate) {
    duration = Math.max(1, Math.round((new Date(endDate).getTime() - new Date(startDate).getTime()) / 86400000));
  }

  // 后端 end_date 已是独占式（结束日 = 开始 + 工期，如 1 天任务 8/5~8/6），
  // 与 dhtmlx 的 end-start 像素差语义一致，直接使用（里程碑保持点）

  return {
    id: t.id,
    parent: t.parent_id || 0,
    text: t.name || "",
    start_date: startDate,
    end_date: endDate,
    duration: duration,
    duration_days: t.duration_days || 1,
    progress: (t.progress_pct || 0) / 100,
    task_type: t.task_type || "task",
    type: t.task_type === "milestone" ? "milestone" : undefined,
    $open: true,  // 数据加载时默认展开所有分支
    status: t.status,
    assignee: t.assignee,
    priority: t.priority,
    manual_scheduled: t.manual_scheduled,
    constraint_type: t.constraint_type || "",
    constraint_date: t.constraint_date || "",
    baseline_start_date: t.baseline_start_date || "",
    baseline_end_date: t.baseline_end_date || "",
    actual_start: t.actual_start || "",
    actual_end: t.actual_end || "",
    version: t.version,
    sort_order: t.sort_order,
    critical: !!t.critical,
    $readonly: readonly,
  };
}

/**
 * 将 dhtmlx-gantt Task → 后端 Task（保存用）
 */
export function fromGanttTask(gt: GanttTask): any {
  return {
    id: gt.id,
    name: gt.text,
    parent_id: gt.parent || null,
    start_date: gt.start_date,
    end_date: gt.end_date,
    duration_days: gt.duration,
    progress_pct: Math.round((gt.progress || 0) * 100),
    task_type: gt.type === "milestone" ? "milestone" : "task",
    status: gt.status || "open",
    assignee: gt.assignee || "",
    priority: gt.priority || "medium",
    manual_scheduled: gt.manual_scheduled || false,
    constraint_type: gt.constraint_type || "",
    constraint_date: gt.constraint_date || "",
    version: gt.version || 0,
  };
}

/**
 * 将后端依赖 → dhtmlx-gantt 链接
 */
export function toGanttLink(d: any): GanttLink {
  return {
    id: d.id,
    source: d.predecessor_id,
    target: d.successor_id,
    type: DEP_TYPE_TO_GANTT[d.dep_type] || "0",
    lag: d.lag_days || 0,
  };
}

/**
 * 将 dhtmlx-gantt 链接 → 后端依赖
 */
export function fromGanttLink(gl: GanttLink): any {
  return {
    predecessor_id: gl.source,
    successor_id: gl.target,
    dep_type: GANTT_TO_DEP_TYPE[gl.type] || "FS",
    lag_days: gl.lag || 0,
  };
}
