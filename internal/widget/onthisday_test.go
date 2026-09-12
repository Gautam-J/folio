package widget

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gautam-J/Folio/internal/models"
)

type stubOnThisDayFetcher struct {
	data models.OnThisDayData
	err  error
}

func (s stubOnThisDayFetcher) Get(ctx context.Context) (models.OnThisDayData, error) {
	return s.data, s.err
}

func TestOnThisDayWidget_Render(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "onthisday.html")
	content := `<span class="label">{{.Text}}</span>`
	if err := os.WriteFile(tmplPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	fetcher := stubOnThisDayFetcher{data: models.OnThisDayData{Text: "490 BC – Battle of Marathon"}}
	w, err := NewOnThisDayWidget(fetcher, tmplPath)
	if err != nil {
		t.Fatalf("NewOnThisDayWidget: %v", err)
	}

	html, err := w.Render(context.Background())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	got := string(html)
	if !strings.Contains(got, "490 BC – Battle of Marathon") {
		t.Errorf("expected event text in output, got: %s", got)
	}
}

func TestOnThisDayWidget_Render_FetchError(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "onthisday.html")
	if err := os.WriteFile(tmplPath, []byte(`{{.Text}}`), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	w, err := NewOnThisDayWidget(stubOnThisDayFetcher{err: errors.New("boom")}, tmplPath)
	if err != nil {
		t.Fatalf("NewOnThisDayWidget: %v", err)
	}

	if _, err := w.Render(context.Background()); err == nil {
		t.Fatal("expected error when fetch fails")
	}
}

func TestOnThisDayWidget_Render_BadTemplatePath(t *testing.T) {
	if _, err := NewOnThisDayWidget(stubOnThisDayFetcher{}, "/nonexistent/path.html"); err == nil {
		t.Fatal("expected error for nonexistent template path")
	}
}
