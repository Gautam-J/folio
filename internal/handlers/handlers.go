// internal/handlers/handlers.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Gautam-J/Folio/internal/models"
	"github.com/Gautam-J/Folio/internal/widget"
)

type DashboardRenderer interface {
	RenderDashboard(ctx context.Context, widgets []widget.Widget, width, height int) (string, error)
}

type Server struct {
	accessToken string
	refreshRate int
	widgets     []widget.Widget
	renderer    DashboardRenderer
	outputDir   string
}

func NewServer(accessToken string, refreshRate int, widgets []widget.Widget, renderer DashboardRenderer, outputDir string) *Server {
	return &Server{
		accessToken: accessToken,
		refreshRate: refreshRate,
		widgets:     widgets,
		renderer:    renderer,
		outputDir:   outputDir,
	}
}

func (s *Server) HandleDisplay(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("access-token") != s.accessToken {
		writeJSON(w, http.StatusUnauthorized, models.DisplayResponse{
			Status: http.StatusUnauthorized,
			Error:  "invalid access token",
		})
		return
	}

	width, werr := strconv.Atoi(r.Header.Get("png-width"))
	height, herr := strconv.Atoi(r.Header.Get("png-height"))
	if werr != nil || herr != nil || width <= 0 || height <= 0 {
		writeJSON(w, http.StatusBadRequest, models.DisplayResponse{
			Status: http.StatusBadRequest,
			Error:  "missing or invalid png-width/png-height headers",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	filename, err := s.renderer.RenderDashboard(ctx, s.widgets, width, height)
	if err != nil {
		slog.Error("render failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.DisplayResponse{
			Status: http.StatusInternalServerError,
			Error:  "render failed",
		})
		return
	}

	// The plugin invalidates its local image cache purely by comparing this
	// field to the last response's — not by content or HTTP headers — so it
	// must change on every render even though the served file is always
	// overwritten in place at the same URL.
	cacheBustFilename := fmt.Sprintf("current-%d.png", time.Now().Unix())

	slog.Info("served display request", "filename", filename)
	writeJSON(w, http.StatusOK, models.DisplayResponse{
		Status:      0,
		ImageURL:    fmt.Sprintf("http://%s/images/%s", r.Host, filename),
		Filename:    cacheBustFilename,
		RefreshRate: s.refreshRate,
	})
}

func (s *Server) ImagesHandler() http.Handler {
	return http.StripPrefix("/images/", http.FileServer(http.Dir(s.outputDir)))
}

func writeJSON(w http.ResponseWriter, status int, body models.DisplayResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
