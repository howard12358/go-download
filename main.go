package main

import (
	_ "embed"
	"github.com/getlantern/systray"
	"go-download/internal/route"
	"log"
	"runtime"
)

// 把生成的 icon.icns 放到 resources 中并编译进二进制
//
//go:embed build/resources/icon.icns
var darwinIcon []byte

//go:embed build/resources/icon.ico
var windowsIcon []byte

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
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

	go startBackend()

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

func onExit() {
}

func startBackend() {
	r := route.SetupRouter()
	port := ":11235"
	log.Printf("starting go-download server on %s...\n", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
