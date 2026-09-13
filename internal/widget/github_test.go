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

type stubGitHubStatsFetcher struct {
	data models.GitHubStatsData
	err  error
}

func (s stubGitHubStatsFetcher) Get(ctx context.Context) (models.GitHubStatsData, error) {
	return s.data, s.err
}

func TestGitHubStatsWidget_Render(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "github.html")
	content := `<span class="label">{{.ContributionsThisYear}} contributions</span>`
	if err := os.WriteFile(tmplPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	fetcher := stubGitHubStatsFetcher{data: models.GitHubStatsData{ContributionsThisYear: 224}}
	w, err := NewGitHubStatsWidget(fetcher, tmplPath)
	if err != nil {
		t.Fatalf("NewGitHubStatsWidget: %v", err)
	}

	html, err := w.Render(context.Background())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	got := string(html)
	if !strings.Contains(got, "224 contributions") {
		t.Errorf("expected contributions in output, got: %s", got)
	}
}

func TestGitHubStatsWidget_Render_FetchError(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "github.html")
	if err := os.WriteFile(tmplPath, []byte(`{{.ContributionsThisYear}}`), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	w, err := NewGitHubStatsWidget(stubGitHubStatsFetcher{err: errors.New("boom")}, tmplPath)
	if err != nil {
		t.Fatalf("NewGitHubStatsWidget: %v", err)
	}

	if _, err := w.Render(context.Background()); err == nil {
		t.Fatal("expected error when fetch fails")
	}
}

func TestGitHubStatsWidget_Render_BadTemplatePath(t *testing.T) {
	if _, err := NewGitHubStatsWidget(stubGitHubStatsFetcher{}, "/nonexistent/path.html"); err == nil {
		t.Fatal("expected error for nonexistent template path")
	}
}
