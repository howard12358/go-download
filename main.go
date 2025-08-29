package main

import (
	"context"
	_ "embed"
	"errors"
	"github.com/getlantern/systray"
	"go-download/internal/core/route"
	"go-download/internal/core/service"
	"go-download/internal/core/sse"
	"log"
	"net/http"
	"runtime"
	"time"
)

// 把生成的 icon.icns 放到 resources 中并编译进二进制
//
//go:embed build/resources/icon.icns
var darwinIcon []byte

//go:embed build/resources/icon.ico
var windowsIcon []byte

type App struct {
	server *http.Server
	hub    *sse.Hub
	svc    *service.DownloadService
}

func NewApp() *App {
	hub := sse.NewHub()
	svc := service.NewDownloadService(hub)
	return &App{
		hub: hub,
		svc: svc,
	}
}

func (a *App) onReady() {
	// 设置图标与提示
	if runtime.GOOS == "windows" {
		systray.SetIcon(windowsIcon)
	} else if runtime.GOOS == "darwin" {
		systray.SetIcon(darwinIcon)
	}

	systray.SetTooltip("Go Download (运行中)")

	// 菜单项（第一个只是状态不可点击也可以响应）
	mStatus := systray.AddMenuItem("正在运行", "应用当前状态：正在运行")
	_ = mStatus // 如果不需要交互可忽略

	// 分隔线
	systray.AddSeparator()

	// 退出菜单
	mQuit := systray.AddMenuItem("退出", "退出应用")

	// 启动后端
	go a.startBackend()

	// 监听菜单事件
	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func (a *App) onExit() {
	// 优雅关闭 http server
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.server.Shutdown(ctx); err != nil {
			log.Printf("server shutdown error: %v\n", err)
		} else {
			log.Println("server stopped gracefully")
		}
	}
}

func (a *App) startBackend() {
	r := route.SetupRouter(a.hub, a.svc)
	a.server = &http.Server{
		Addr:    ":11235",
		Handler: r,
	}

	log.Printf("starting go-download server on %s...\n", a.server.Addr)
	if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("failed to run server: %v", err)
	}
}

func main() {
	app := NewApp()
	systray.Run(app.onReady, app.onExit)
}
