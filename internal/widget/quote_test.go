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

type stubQuoteFetcher struct {
	data models.QuoteData
	err  error
}

func (s stubQuoteFetcher) Get(ctx context.Context) (models.QuoteData, error) {
	return s.data, s.err
}

func TestQuoteWidget_Render(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "quote.html")
	content := `<span class="label">{{.Quote}} &mdash; {{.Author}}</span>`
	if err := os.WriteFile(tmplPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	fetcher := stubQuoteFetcher{data: models.QuoteData{Quote: "Stay hungry, stay foolish.", Author: "Steve Jobs"}}
	w, err := NewQuoteWidget(fetcher, tmplPath)
	if err != nil {
		t.Fatalf("NewQuoteWidget: %v", err)
	}

	html, err := w.Render(context.Background())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	got := string(html)
	if !strings.Contains(got, "Stay hungry, stay foolish.") {
		t.Errorf("expected quote in output, got: %s", got)
	}
	if !strings.Contains(got, "Steve Jobs") {
		t.Errorf("expected author in output, got: %s", got)
	}
}

func TestQuoteWidget_Render_FetchError(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "quote.html")
	if err := os.WriteFile(tmplPath, []byte(`{{.Quote}}`), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	w, err := NewQuoteWidget(stubQuoteFetcher{err: errors.New("boom")}, tmplPath)
	if err != nil {
		t.Fatalf("NewQuoteWidget: %v", err)
	}

	if _, err := w.Render(context.Background()); err == nil {
		t.Fatal("expected error when fetch fails")
	}
}

func TestQuoteWidget_Render_BadTemplatePath(t *testing.T) {
	if _, err := NewQuoteWidget(stubQuoteFetcher{}, "/nonexistent/path.html"); err == nil {
		t.Fatal("expected error for nonexistent template path")
	}
}
