// internal/widget/progress.go
package widget

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"time"
)

var _ Widget = (*ProgressWidget)(nil)

type progressTemplateData struct {
	Week  int
	Month int
	Year  int
}

// ProgressWidget has no external data source; Render computes how far
// through the current week (Monday-start), month, and year now() falls,
// each as a percentage rounded to the nearest whole number.
type ProgressWidget struct {
	tmpl *template.Template
	now  func() time.Time
}

func NewProgressWidget(templatePath string, now func() time.Time) (*ProgressWidget, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse progress widget template: %w", err)
	}
	return &ProgressWidget{tmpl: tmpl, now: now}, nil
}

func (w *ProgressWidget) Render(ctx context.Context) (template.HTML, error) {
	now := w.now()
	data := progressTemplateData{
		Week:  percentElapsed(now, weekStart(now), weekStart(now).AddDate(0, 0, 7)),
		Month: percentElapsed(now, monthStart(now), monthStart(now).AddDate(0, 1, 0)),
		Year:  percentElapsed(now, yearStart(now), yearStart(now).AddDate(1, 0, 0)),
	}

	var buf bytes.Buffer
	if err := w.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute progress template: %w", err)
	}
	return template.HTML(buf.String()), nil
}

func weekStart(t time.Time) time.Time {
	y, m, d := t.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	daysSinceMonday := (int(midnight.Weekday()) + 6) % 7
	return midnight.AddDate(0, 0, -daysSinceMonday)
}

func monthStart(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

func yearStart(t time.Time) time.Time {
	y, _, _ := t.Date()
	return time.Date(y, 1, 1, 0, 0, 0, 0, t.Location())
}

func percentElapsed(now, start, end time.Time) int {
	total := end.Sub(start)
	elapsed := now.Sub(start)
	pct := float64(elapsed) / float64(total) * 100
	return int(pct + 0.5)
}
