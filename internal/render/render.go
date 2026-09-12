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

	"github.com/Gautam-J/Folio/internal/models"
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

func (r *Renderer) Render(ctx context.Context, data models.WeatherData, width, height int) (string, error) {
	var htmlBuf bytes.Buffer
	if err := r.tmpl.Execute(&htmlBuf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	htmlPath := filepath.Join(r.outputDir, "render.html")
	if err := os.WriteFile(htmlPath, htmlBuf.Bytes(), 0o644); err != nil {
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
