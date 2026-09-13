// cmd/main.go
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Gautam-J/Folio/internal/config"
	"github.com/Gautam-J/Folio/internal/github"
	"github.com/Gautam-J/Folio/internal/handlers"
	"github.com/Gautam-J/Folio/internal/onthisday"
	"github.com/Gautam-J/Folio/internal/quote"
	"github.com/Gautam-J/Folio/internal/render"
	"github.com/Gautam-J/Folio/internal/weather"
	"github.com/Gautam-J/Folio/internal/widget"
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

	weatherWidget, err := widget.NewWeatherWidget(weatherClient, cfg.Latitude, cfg.Longitude, "templates/widgets/weather.html")
	if err != nil {
		slog.Error("failed to init weather widget", "error", err)
		os.Exit(1)
	}

	dateTimeWidget, err := widget.NewDateTimeWidget("templates/widgets/datetime.html", time.Now)
	if err != nil {
		slog.Error("failed to init date/time widget", "error", err)
		os.Exit(1)
	}

	quoteClient := quote.NewClient(quote.DefaultBaseURL)

	quoteWidget, err := widget.NewQuoteWidget(quoteClient, "templates/widgets/quote.html")
	if err != nil {
		slog.Error("failed to init quote widget", "error", err)
		os.Exit(1)
	}

	progressWidget, err := widget.NewProgressWidget("templates/widgets/progress.html", time.Now)
	if err != nil {
		slog.Error("failed to init progress widget", "error", err)
		os.Exit(1)
	}

	onThisDayClient := onthisday.NewClient(onthisday.DefaultBaseURL)

	onThisDayWidget, err := widget.NewOnThisDayWidget(onThisDayClient, "templates/widgets/onthisday.html")
	if err != nil {
		slog.Error("failed to init on-this-day widget", "error", err)
		os.Exit(1)
	}

	githubClient := github.NewClient(github.DefaultBaseURL, cfg.GitHubUsername, cfg.GitHubToken)

	githubStatsWidget, err := widget.NewGitHubStatsWidget(githubClient, "templates/widgets/github.html")
	if err != nil {
		slog.Error("failed to init github stats widget", "error", err)
		os.Exit(1)
	}

	widgets := []widget.Widget{weatherWidget, dateTimeWidget, quoteWidget, progressWidget, onThisDayWidget, githubStatsWidget}

	renderer, err := render.NewRenderer("templates/dashboard.html", outputDir)
	if err != nil {
		slog.Error("failed to init renderer", "error", err)
		os.Exit(1)
	}

	server := handlers.NewServer(
		cfg.AccessToken,
		cfg.RefreshRateSeconds,
		widgets,
		renderer,
		outputDir,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/display", server.HandleDisplay)
	mux.Handle("GET /images/", server.ImagesHandler())

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("starting server", "addr", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
