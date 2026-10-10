//go:build windows

package main

import (
	"embed"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"log"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := &App{}
	err := wails.Run(&options.App{Title: "claudeFF · 个人工作空间", Width: 1280, Height: 880, MinWidth: 800, MinHeight: 600, AssetServer: &assetserver.Options{Assets: assets}, OnStartup: a.startup, OnShutdown: a.shutdown, OnBeforeClose: a.beforeClose, Bind: []interface{}{a}, SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "claudeff-personal-workspace-v2", OnSecondInstanceLaunch: func(data options.SecondInstanceData) { runtime.WindowUnminimise(a.ctx); runtime.WindowShow(a.ctx) }}})
	if err != nil {
		log.Print("桌面程序启动失败：", err)
	}
}
