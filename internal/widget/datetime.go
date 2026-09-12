// internal/widget/datetime.go
package widget

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"time"
)

var _ Widget = (*DateTimeWidget)(nil)

type dateTimeTemplateData struct {
	Now time.Time
}

// DateTimeWidget has no external data source, so unlike WeatherWidget
// (whose FetchedAt can lag behind render time when serving a cached
// result) its "current time" and "last updated" are always the same
// render-time value.
type DateTimeWidget struct {
	tmpl *template.Template
	now  func() time.Time
}

func NewDateTimeWidget(templatePath string, now func() time.Time) (*DateTimeWidget, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse datetime widget template: %w", err)
	}
	return &DateTimeWidget{tmpl: tmpl, now: now}, nil
}

func (w *DateTimeWidget) Render(ctx context.Context) (template.HTML, error) {
	var buf bytes.Buffer
	if err := w.tmpl.Execute(&buf, dateTimeTemplateData{Now: w.now()}); err != nil {
		return "", fmt.Errorf("execute datetime template: %w", err)
	}
	return template.HTML(buf.String()), nil
}
