// internal/widget/weather.go
package widget

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

	"github.com/Gautam-J/Folio/internal/models"
)

var _ Widget = (*WeatherWidget)(nil)

// WeatherFetcher is the subset of weather.Client's interface WeatherWidget
// needs. weather.Client already satisfies this without modification.
type WeatherFetcher interface {
	Get(ctx context.Context, lat, lon float64) (models.WeatherData, error)
}

type WeatherWidget struct {
	fetcher WeatherFetcher
	lat     float64
	lon     float64
	tmpl    *template.Template
}

func NewWeatherWidget(fetcher WeatherFetcher, lat, lon float64, templatePath string) (*WeatherWidget, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse weather widget template: %w", err)
	}
	return &WeatherWidget{fetcher: fetcher, lat: lat, lon: lon, tmpl: tmpl}, nil
}

func (w *WeatherWidget) Render(ctx context.Context) (template.HTML, error) {
	data, err := w.fetcher.Get(ctx, w.lat, w.lon)
	if err != nil {
		return "", fmt.Errorf("fetch weather: %w", err)
	}

	var buf bytes.Buffer
	if err := w.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute weather template: %w", err)
	}
	return template.HTML(buf.String()), nil
}
