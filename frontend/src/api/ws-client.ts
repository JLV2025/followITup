/**
 * WebSocket 客户端 — 实时协作
 * 连接 /ws/projects/{id}，收发任务变更广播和聚焦状态
 */

export type WsMessage = {
  type: string;
  project_id: number;
  user_id?: number;
  user_name?: string;
  task_id?: number;
  data?: unknown;
};

type Listener = (msg: WsMessage) => void;

class WsClient {
  private ws: WebSocket | null = null;
  private listeners: Set<Listener> = new Set();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private projectId: number = 0;
  private userId: number = 0;
  private userName: string = "";

  connect(projectId: number, userId: number, userName: string) {
    if (this.ws && this.projectId === projectId) return;

    this.disconnect();
    this.projectId = projectId;
    this.userId = userId;
    this.userName = userName;

    const proto = location.protocol === "https:" ? "wss" : "ws";
    const url = `${proto}://${location.host}/ws/projects/${projectId}?user_id=${userId}&user_name=${encodeURIComponent(userName)}`;

    this.ws = new WebSocket(url);
    this.ws.onopen = () => {
      console.log("[WS] 已连接到项目", projectId);
      // 每 25 秒发送心跳
      this.heartbeatTimer = setInterval(() => this.sendHeartbeat(), 25000);
    };
    this.ws.onclose = () => {
      console.log("[WS] 连接断开，3秒后重连");
      this.clearHeartbeat();
      this.reconnectTimer = setTimeout(() => this.connect(projectId, userId, userName), 3000);
    };
    this.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as WsMessage;
        this.listeners.forEach((fn) => fn(msg));
      } catch {
        // ignore
      }
    };
    this.ws.onerror = () => {
      // onclose 会处理
    };
  }

  disconnect() {
    this.clearHeartbeat();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
  }

  subscribe(fn: Listener) {
    this.listeners.add(fn);
    return () => { this.listeners.delete(fn); };
  }

  /** 通知其他用户：我开始聚焦某个任务 */
  sendFocus(taskId: number) {
    this.send({ type: "task_focus", task_id: taskId });
  }

  /** 通知其他用户：我已离开某个任务 */
  sendBlur(taskId: number) {
    this.send({ type: "task_blur", task_id: taskId });
  }

  private sendHeartbeat() {
    this.send({ type: "ping" });
  }

  private send(partial: Partial<WsMessage>) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(JSON.stringify({
      project_id: this.projectId,
      user_id: this.userId,
      user_name: this.userName,
      ...partial,
    }));
  }

  private clearHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }
}

export const wsClient = new WsClient();
