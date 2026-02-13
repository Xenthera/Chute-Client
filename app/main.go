package main

import (
	"embed"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:ui/dist
var assets embed.FS

func main() {
	serverAddr := resolveServerAddr()
	app := NewApp(serverAddr)

	err := wails.Run(&options.App{
		Title:       "Chute",
		Width:       800,
		Height:      600,
		MinWidth:    800,
		MinHeight:   600,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.startup,
		OnShutdown:  app.shutdown,
		Bind:        []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}

func resolveServerAddr() string {
	if ip := strings.TrimSpace(os.Getenv("CHUTE_SERVER_IP")); ip != "" {
		return ip + ":8080"
	}
	return "localhost:8080"
}
