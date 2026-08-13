package main

import (
	"log/slog"
	"os"

	frontendassets "github.com/klaude/klaude/frontend"
	"github.com/klaude/klaude/internal/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := app.NewService(logger)

	err := wails.Run(&options.App{
		Title:     "Klaude",
		Width:     1440,
		Height:    900,
		MinWidth:  760,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: frontendassets.Files,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 14, B: 24, A: 1},
		Mac:              &mac.Options{TitleBar: mac.TitleBarHiddenInset()},
		Bind:             []interface{}{app.NewRPCService(service)},
		OnStartup:        service.Startup,
		OnShutdown:       service.Shutdown,
	})
	if err != nil {
		logger.Error("failed to run Klaude", "error", err)
		os.Exit(1)
	}
}
