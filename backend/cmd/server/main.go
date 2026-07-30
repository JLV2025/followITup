package main

import (
	"embed"
	"io/fs"
	"log"
	"os"

	"followitup/internal/server"
)

//go:embed all:frontend-dist
var frontendDist embed.FS

func main() {
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
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
