// cmd/main.go
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/Gautam-J/Folio/internal/config"
	"github.com/Gautam-J/Folio/internal/handlers"
	"github.com/Gautam-J/Folio/internal/render"
	"github.com/Gautam-J/Folio/internal/weather"
)

const outputDir = "generated"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	weatherClient := weather.NewClient(weather.DefaultBaseURL)

	renderer, err := render.NewRenderer("templates/weather.html", outputDir)
	if err != nil {
		slog.Error("failed to init renderer", "error", err)
		os.Exit(1)
	}

	server := handlers.NewServer(
		cfg.AccessToken,
		cfg.Latitude,
		cfg.Longitude,
		cfg.RefreshRateSeconds,
		weatherClient,
		renderer,
		outputDir,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/display", server.HandleDisplay)
	mux.Handle("GET /images/", server.ImagesHandler())

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
