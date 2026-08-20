package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"strings"

	"followitup/internal/server"
)

//go:embed all:frontend-dist
var frontendDist embed.FS

func main() {
	// 兼容两种参数形式：`-config <path>`（README/服务注册常用）与裸参数 `<path>`
	cfgPath := "config.yaml"
	args := os.Args[1:]
	if len(args) >= 2 && (args[0] == "-config" || args[0] == "--config") {
		cfgPath = args[1]
	} else if len(args) >= 1 && !strings.HasPrefix(args[0], "-") {
		cfgPath = args[0]
	}

	// 尝试加载嵌入式前端文件
	var frontendFS fs.FS
	if f, err := fs.Sub(frontendDist, "frontend-dist"); err == nil {
		frontendFS = f
	}

	opts := server.Options{
		ConfigPath: cfgPath,
		FrontendFS: frontendFS,
	}

	if err := server.Run(opts); err != nil {
		log.Fatalf("[Server] 启动失败: %v", err)
	}
}
