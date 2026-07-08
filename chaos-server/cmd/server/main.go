package main

import (
	"chaos-lib/config"
	"chaos-lib/routes"
	"chaos-lib/tasks"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	_ "net/http/pprof"
)

//go:embed web
var webFS embed.FS

func initEarlyLog() {
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execDir := filepath.Dir(execPath)
	logPath := filepath.Join(execDir, "logs", "startup.log")

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	log.SetOutput(file)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func setupPprof(cfg *config.PprofConfig) {
	if !cfg.Enabled {
		log.Println("🔇 Pprof 已禁用")
		return
	}

	go func() {
		log.Printf("🔍 Pprof 启动: http://%s", cfg.GetAddress())
		err := http.ListenAndServe(cfg.GetAddress(), nil)
		if err != nil {
			log.Printf("❌ Pprof 启动失败: %v", err)
		}
	}()
}

func main() {
	initEarlyLog()
	log.Printf("🚀 程序启动时间: %s", time.Now().Format("2006-01-02 15:04:05"))

	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ 程序崩溃: %v\n%s", r, string(debug.Stack()))
			execPath, _ := os.Executable()
			execDir := filepath.Dir(execPath)
			crashPath := filepath.Join(execDir, "crash.log")
			f, err := os.OpenFile(crashPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				defer f.Close()
				fmt.Fprintf(f, "[%s] 崩溃: %v\n%s\n", time.Now().Format("2006-01-02 15:04:05"), r, string(debug.Stack()))
			}
		}
	}()

	cfg := config.LoadConfig()
	log.Println("✅ 配置加载成功")

	config.InitLog()
	log.Println("✅ 日志初始化完成")

	setupPprof(&cfg.Pprof)
	log.Println("✅ Pprof 设置完成")

	tasks.Start()
	log.Println("✅ 后台任务启动完成")

	log.Printf("🌐 启动 HTTP 服务 %s [%s]", cfg.Server.GetAddress(), cfg.Environment)
	r := routes.SetupRouter(webFS)
	go r.Run(cfg.Server.GetAddress())

	log.Println("✅ HTTP 服务启动完成")

	select {}
}
