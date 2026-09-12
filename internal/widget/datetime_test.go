package widget

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDateTimeWidget_Render(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "datetime.html")
	content := `<span class="label">{{.Now.Format "Jan 2"}}</span>` +
		`<span class="value value--large">{{.Now.Format "15:04"}}</span>` +
		`<span class="label">Updated {{.Now.Format "15:04"}}</span>`
	if err := os.WriteFile(tmplPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	fixed := time.Date(2026, 9, 12, 14, 30, 0, 0, time.UTC)
	w, err := NewDateTimeWidget(tmplPath, func() time.Time { return fixed })
	if err != nil {
		t.Fatalf("NewDateTimeWidget: %v", err)
	}

	html, err := w.Render(context.Background())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	got := string(html)
	if !strings.Contains(got, "Sep 12") {
		t.Errorf("expected date %q in output, got: %s", "Sep 12", got)
	}
	if !strings.Contains(got, "14:30") {
		t.Errorf("expected time %q in output, got: %s", "14:30", got)
	}
	if !strings.Contains(got, "Updated 14:30") {
		t.Errorf("expected 'Updated 14:30' in output, got: %s", got)
	}
}

func TestDateTimeWidget_Render_BadTemplatePath(t *testing.T) {
	if _, err := NewDateTimeWidget("/nonexistent/path.html", time.Now); err == nil {
		t.Fatal("expected error for nonexistent template path")
	}
}
