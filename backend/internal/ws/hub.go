package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Message WebSocket 消息格式
type Message struct {
	Type      string          `json:"type"` // "task_update" | "task_focus" | "task_blur" | "ping"
	ProjectID int64           `json:"project_id"`
	UserID    int64           `json:"user_id,omitempty"`
	UserName  string          `json:"user_name,omitempty"`
	TaskID    int64           `json:"task_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// OnlineUser 在线用户信息
type OnlineUser struct {
	UserID   int64  `json:"user_id"`
	UserName string `json:"user_name"`
	TaskID   int64  `json:"task_id,omitempty"` // 当前聚焦的任务，0 表示未聚焦
}

// Client 一个 WebSocket 连接
type Client struct {
	Conn      *websocket.Conn
	UserID    int64
	UserName  string
	ProjectID int64
	Send      chan []byte
}

// Hub 管理所有 WebSocket 连接
type Hub struct {
	mu        sync.RWMutex
	rooms     map[int64]map[*Client]bool // projectID → clients
	register   chan *Client
	unregister chan *Client
	broadcast  chan Message
}

// NewHub 创建 Hub 实例
func NewHub() *Hub {
	h := &Hub{
		rooms:      make(map[int64]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Message, 256),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.rooms[client.ProjectID] == nil {
				h.rooms[client.ProjectID] = make(map[*Client]bool)
			}
			h.rooms[client.ProjectID][client] = true
			h.mu.Unlock()
			log.Printf("[WS] 用户 %s 加入项目 %d (在线: %d)", client.UserName, client.ProjectID, len(h.rooms[client.ProjectID]))

		case client := <-h.unregister:
			h.mu.Lock()
			if room, ok := h.rooms[client.ProjectID]; ok {
				if _, ok := room[client]; ok {
					delete(room, client)
					close(client.Send)
					if len(room) == 0 {
						delete(h.rooms, client.ProjectID)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("[WS] 用户 %s 离开项目 %d", client.UserName, client.ProjectID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			room := h.rooms[msg.ProjectID]
			// 复制以避免并发修改
			clients := make([]*Client, 0, len(room))
			for c := range room {
				clients = append(clients, c)
			}
			h.mu.RUnlock()

			data, _ := json.Marshal(msg)
			for _, c := range clients {
				select {
				case c.Send <- data:
				default:
					// 客户端发送缓冲区满，断开
					go func(cl *Client) { h.unregister <- cl }(c)
				}
			}
		}
	}
}

// HandleWebSocket 处理 WebSocket 升级请求
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	// 从 query 获取用户信息（由前端传入）
	userIDStr := r.URL.Query().Get("user_id")
	userName := r.URL.Query().Get("user_name")
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] 升级失败: %v", err)
		return
	}

	client := &Client{
		Conn:      conn,
		UserID:    userID,
		UserName:  userName,
		ProjectID: projectID,
		Send:      make(chan []byte, 64),
	}

	h.register <- client

	// 写 goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer func() {
			ticker.Stop()
			conn.Close()
			h.unregister <- client
		}()

		for {
			select {
			case msg, ok := <-client.Send:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-ticker.C:
				// 心跳
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// 读 goroutine（接收客户端消息）
	go func() {
		defer conn.Close()
		for {
			_, msgData, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg Message
			if err := json.Unmarshal(msgData, &msg); err != nil {
				continue
			}
			msg.UserID = client.UserID
			msg.UserName = client.UserName
			msg.ProjectID = client.ProjectID
			// 聚焦/失焦不广播给发送者自己
			if msg.Type == "task_focus" || msg.Type == "task_blur" {
				h.broadcastExcept(msg, client)
			} else {
				h.broadcast <- msg
			}
		}
	}()
}

// BroadcastTaskUpdate 广播任务变更（由 API 调用）
func (h *Hub) BroadcastTaskUpdate(projectID int64, userID int64, userName string, taskID int64, data json.RawMessage) {
	h.broadcast <- Message{
		Type:      "task_update",
		ProjectID: projectID,
		UserID:    userID,
		UserName:  userName,
		TaskID:    taskID,
		Data:      data,
	}
}

// broadcastExcept 广播给房间内除 sender 外的所有客户端
func (h *Hub) broadcastExcept(msg Message, sender *Client) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	room := h.rooms[msg.ProjectID]
	data, _ := json.Marshal(msg)
	for c := range room {
		if c == sender {
			continue
		}
		select {
		case c.Send <- data:
		default:
			go func(cl *Client) { h.unregister <- cl }(c)
		}
	}
}

// GetOnlineCount 获取项目在线人数
func (h *Hub) GetOnlineCount(projectID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[projectID])
}

// GetOnlineUsers 获取项目在线用户列表
func (h *Hub) GetOnlineUsers(projectID int64) []OnlineUser {
	h.mu.RLock()
	defer h.mu.RUnlock()

	room := h.rooms[projectID]
	users := make([]OnlineUser, 0, len(room))
	for c := range room {
		users = append(users, OnlineUser{
			UserID:   c.UserID,
			UserName: c.UserName,
		})
	}
	return users
}
