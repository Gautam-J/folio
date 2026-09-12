// internal/render/render_dashboard_test.go
package render

import (
	"context"
	"errors"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gautam-J/Folio/internal/widget"
)

type stubWidget struct {
	html template.HTML
	err  error
}

func (s stubWidget) Render(ctx context.Context) (template.HTML, error) {
	return s.html, s.err
}

func TestBuildDashboardHTML_ComposesWidgetsAndIsolatesErrors(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "dashboard.html")
	tmplContent := `<div id="main">{{.Main}}</div><div id="corner">{{.Corner}}</div>`
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	r, err := NewRenderer(tmplPath, dir)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	widgets := []widget.Widget{
		stubWidget{html: template.HTML("<p>weather</p>")},
		stubWidget{err: errors.New("boom")},
	}

	got, err := r.buildDashboardHTML(context.Background(), widgets, 800)
	if err != nil {
		t.Fatalf("buildDashboardHTML: %v", err)
	}
	if !strings.Contains(got, "<p>weather</p>") {
		t.Errorf("expected successful widget's fragment in output, got: %s", got)
	}
	if strings.Contains(got, "boom") {
		t.Errorf("erroring widget's error text leaked into output: %s", got)
	}
	if !strings.Contains(got, `<div id="corner"></div>`) {
		t.Errorf("expected empty corner slot for the erroring widget, got: %s", got)
	}
}
