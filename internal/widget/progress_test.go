package widget

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProgressWidget_Render(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "progress.html")
	content := `<div data-week="{{.Week}}" data-month="{{.Month}}" data-year="{{.Year}}"></div>`
	if err := os.WriteFile(tmplPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	// 2026-07-02 12:00 UTC: Thursday, exactly halfway through both the
	// Monday-start week and the (non-leap) year; 1.5 of July's 31 days in.
	fixed := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	w, err := NewProgressWidget(tmplPath, func() time.Time { return fixed })
	if err != nil {
		t.Fatalf("NewProgressWidget: %v", err)
	}

	html, err := w.Render(context.Background())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	got := string(html)
	for _, want := range []string{`data-week="50"`, `data-month="5"`, `data-year="50"`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got: %s", want, got)
		}
	}
}

func TestProgressWidget_Render_BadTemplatePath(t *testing.T) {
	if _, err := NewProgressWidget("/nonexistent/path.html", time.Now); err == nil {
		t.Fatal("expected error for nonexistent template path")
	}
}
