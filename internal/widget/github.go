// internal/widget/github.go
package widget

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

	"github.com/Gautam-J/Folio/internal/models"
)

var _ Widget = (*GitHubStatsWidget)(nil)

// GitHubStatsFetcher is the subset of github.Client's interface
// GitHubStatsWidget needs. github.Client already satisfies this without
// modification.
type GitHubStatsFetcher interface {
	Get(ctx context.Context) (models.GitHubStatsData, error)
}

type GitHubStatsWidget struct {
	fetcher GitHubStatsFetcher
	tmpl    *template.Template
}

func NewGitHubStatsWidget(fetcher GitHubStatsFetcher, templatePath string) (*GitHubStatsWidget, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse github stats widget template: %w", err)
	}
	return &GitHubStatsWidget{fetcher: fetcher, tmpl: tmpl}, nil
}

func (w *GitHubStatsWidget) Render(ctx context.Context) (template.HTML, error) {
	data, err := w.fetcher.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch github stats: %w", err)
	}

	var buf bytes.Buffer
	if err := w.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute github stats template: %w", err)
	}
	return template.HTML(buf.String()), nil
}
