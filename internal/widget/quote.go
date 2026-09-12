// internal/widget/quote.go
package widget

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

	"github.com/Gautam-J/Folio/internal/models"
)

var _ Widget = (*QuoteWidget)(nil)

// QuoteFetcher is the subset of quote.Client's interface QuoteWidget needs.
// quote.Client already satisfies this without modification.
type QuoteFetcher interface {
	Get(ctx context.Context) (models.QuoteData, error)
}

type QuoteWidget struct {
	fetcher QuoteFetcher
	tmpl    *template.Template
}

func NewQuoteWidget(fetcher QuoteFetcher, templatePath string) (*QuoteWidget, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse quote widget template: %w", err)
	}
	return &QuoteWidget{fetcher: fetcher, tmpl: tmpl}, nil
}

func (w *QuoteWidget) Render(ctx context.Context) (template.HTML, error) {
	data, err := w.fetcher.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch quote: %w", err)
	}

	var buf bytes.Buffer
	if err := w.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute quote template: %w", err)
	}
	return template.HTML(buf.String()), nil
}
