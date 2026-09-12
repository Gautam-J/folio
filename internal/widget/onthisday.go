// internal/widget/onthisday.go
package widget

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

	"github.com/Gautam-J/Folio/internal/models"
)

var _ Widget = (*OnThisDayWidget)(nil)

// OnThisDayFetcher is the subset of onthisday.Client's interface
// OnThisDayWidget needs. onthisday.Client already satisfies this without
// modification.
type OnThisDayFetcher interface {
	Get(ctx context.Context) (models.OnThisDayData, error)
}

type OnThisDayWidget struct {
	fetcher OnThisDayFetcher
	tmpl    *template.Template
}

func NewOnThisDayWidget(fetcher OnThisDayFetcher, templatePath string) (*OnThisDayWidget, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse on-this-day widget template: %w", err)
	}
	return &OnThisDayWidget{fetcher: fetcher, tmpl: tmpl}, nil
}

func (w *OnThisDayWidget) Render(ctx context.Context) (template.HTML, error) {
	data, err := w.fetcher.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch on-this-day event: %w", err)
	}

	var buf bytes.Buffer
	if err := w.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute on-this-day template: %w", err)
	}
	return template.HTML(buf.String()), nil
}
