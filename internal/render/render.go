// internal/render/render.go
package render

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Gautam-J/Folio/internal/widget"
	"github.com/chromedp/chromedp"
)

const chromiumExecPath = "/usr/bin/chromium"

type Renderer struct {
	tmpl      *template.Template
	outputDir string
}

func NewRenderer(templatePath, outputDir string) (*Renderer, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	return &Renderer{tmpl: tmpl, outputDir: outputDir}, nil
}

// dashboardData feeds templates/dashboard.html: Width drives --folio-scale
// (TRMNL's framework CSS is tuned for its fixed 800x480 baseline), and
// Main/Corner/Strip/Bottom are the four widget slots this milestone supports.
type dashboardData struct {
	Width  int
	Now    time.Time
	Main   template.HTML
	Corner template.HTML
	Strip  template.HTML
	Bottom template.HTML
}

// buildDashboardHTML renders each widget in order — widgets[0] into Main,
// widgets[1] (if present) into Corner, widgets[2] (if present) into Strip,
// widgets[3] (if present) into Bottom — and composes the dashboard shell
// around them. A widget that errors gets an empty fragment in its slot; it
// never fails the whole dashboard.
func (r *Renderer) buildDashboardHTML(ctx context.Context, widgets []widget.Widget, width int) (string, error) {
	var fragments [4]template.HTML
	for i, w := range widgets {
		if i >= len(fragments) {
			break
		}
		html, err := w.Render(ctx)
		if err != nil {
			slog.Error("widget render failed", "index", i, "error", err)
			continue
		}
		fragments[i] = html
	}

	var buf bytes.Buffer
	data := dashboardData{Width: width, Now: time.Now(), Main: fragments[0], Corner: fragments[1], Strip: fragments[2], Bottom: fragments[3]}
	if err := r.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func (r *Renderer) RenderDashboard(ctx context.Context, widgets []widget.Widget, width, height int) (string, error) {
	htmlStr, err := r.buildDashboardHTML(ctx, widgets, width)
	if err != nil {
		return "", err
	}

	htmlPath := filepath.Join(r.outputDir, "render.html")
	if err := os.WriteFile(htmlPath, []byte(htmlStr), 0o644); err != nil {
		return "", fmt.Errorf("write html: %w", err)
	}

	absHTMLPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return "", fmt.Errorf("resolve html path: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chromiumExecPath))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx, chromedp.WithErrorf(logChromedpError))
	defer cancelBrowser()

	var pngBuf []byte
	err = chromedp.Run(browserCtx,
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.Navigate("file://"+absHTMLPath),
		chromedp.CaptureScreenshot(&pngBuf),
	)
	if err != nil {
		return "", fmt.Errorf("chromedp render: %w", err)
	}

	filename := "current.png"
	pngPath := filepath.Join(r.outputDir, filename)
	if err := os.WriteFile(pngPath, pngBuf, 0o644); err != nil {
		return "", fmt.Errorf("write png: %w", err)
	}

	return filename, nil
}

// chromedp v0.10.0 (pinned to keep go.mod at go 1.24.1) ships a cdproto
// schema that predates the "Loopback" IPAddressSpace value this Pi's newer
// Chromium sends, so every render logs a harmless unmarshal error for it.
func logChromedpError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if strings.Contains(msg, "unknown IPAddressSpace value: Loopback") {
		return
	}
	slog.Error(msg)
}
