package server

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"followitup/internal/api"
	"followitup/internal/auth"
	"followitup/internal/db"
	"followitup/internal/ws"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Options 服务器启动选项
type Options struct {
	ConfigPath string
	FrontendFS fs.FS // 可选：前端静态文件，nil 则从磁盘读取
}

// Run 启动服务器
func Run(opts Options) error {
	cfgPath := opts.ConfigPath
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	// 加载配置
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 初始化数据库
	database, err := db.Open(cfg.Server.DataDir)
	if err != nil {
		return fmt.Errorf("数据库初始化失败: %w", err)
	}
	defer database.Close()

	// 初始化认证服务
	authSvc := auth.NewService(database.Conn, cfg.Auth.JWTSecret, cfg.Auth.SessionHours, cfg.Auth.BcryptCost)

	// 首次运行时创建管理员账号
	if err := authSvc.InitAdmin("admin@followitup.local", "admin123", "管理员"); err != nil {
		log.Printf("[WARN] 管理员初始化: %v", err)
	}

	// 初始化鉴权中间件
	authMid := auth.NewMiddleware(cfg.Auth.JWTSecret)

	// 注册路由
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// CORS 中间件
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// 注册认证 API
	authHandler := api.NewAuthHandler(authSvc, authMid)
	authHandler.RegisterRoutes(r)

	// 注册项目与看板 API
	projectHandler := api.NewProjectHandler(database.Conn, authMid, cfg.Fiscal.YearStartMonth)
	projectHandler.RegisterRoutes(r)

	// 初始化 WebSocket Hub
	wsHub := ws.NewHub()
	r.Get("/ws/projects/{id}", wsHub.HandleWebSocket)

	// 注册任务 API（注入 Hub 以支持实时广播）
	taskHandler := api.NewTaskHandler(database.Conn, authMid, wsHub)
	taskHandler.RegisterRoutes(r)

	// 注册工作日历 API
	calHandler := api.NewCalendarHandler(database.Conn, authMid)
	calHandler.RegisterRoutes(r)

	// 托管前端静态文件
	if err := mountFrontend(r, opts.FrontendFS); err != nil {
		log.Printf("[WARN] 前端托管: %v", err)
	}

	// 启动
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("[Server] FollowITup v0.8.0 启动于 http://localhost%s", addr)
	return http.ListenAndServe(addr, r)
}

// mountFrontend 托管前端静态文件
func mountFrontend(r *chi.Mux, frontendFS fs.FS) error {
	var fileServer http.Handler
	var frontendDir string

	if frontendFS != nil {
		// 嵌入式文件系统
		fileServer = http.FileServer(http.FS(frontendFS))
	} else {
		// 从磁盘读取（开发模式）
		frontendDir = findFrontendDir()
		if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
			return fmt.Errorf("前端目录不存在: %s", frontendDir)
		}
		log.Printf("[Server] 开发模式: 托管前端文件 %s", frontendDir)
		fileServer = http.FileServer(http.Dir(frontendDir))
	}

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// API 路径不处理
		if len(r.URL.Path) >= 4 && r.URL.Path[:5] == "/api/" {
			return
		}
		// 静态资源直接返回
		if stringsHasSuffix(r.URL.Path, ".js", ".css", ".png", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".gif", ".jpg", ".jpeg", ".webp") {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA 回退：所有其他路径返回 index.html
		if frontendFS != nil {
			data, err := fs.ReadFile(frontendFS, "index.html")
			if err != nil {
				http.Error(w, "前端文件未找到", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}
		http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
	})

	return nil
}

// findFrontendDir 查找前端构建产物目录
func findFrontendDir() string {
	// 多路径尝试
	candidates := []string{
		"frontend-dist",
		"../frontend/dist",
		"../../frontend/dist",
	}
	// 相对于可执行文件
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, c := range candidates {
			p := filepath.Join(dir, c)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	// 相对于工作目录
	return "../frontend/dist"
}

func stringsHasSuffix(s string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}
