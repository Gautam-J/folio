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

type stubFetcher struct {
	data models.WeatherData
	err  error
}

func (s stubFetcher) Get(ctx context.Context, lat, lon float64) (models.WeatherData, error) {
	return s.data, s.err
}

func TestWeatherWidget_Render(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "weather.html")
	content := `<span class="value value--xxlarge">{{printf "%.0f" .Temperature}}&deg;C</span>` +
		`<span class="label">{{.Description}}</span>`
	if err := os.WriteFile(tmplPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	fetcher := stubFetcher{data: models.WeatherData{Temperature: 20.0, Description: "Clear sky"}}
	w, err := NewWeatherWidget(fetcher, 12.9, 77.6, tmplPath)
	if err != nil {
		t.Fatalf("NewWeatherWidget: %v", err)
	}

	html, err := w.Render(context.Background())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	got := string(html)
	if !strings.Contains(got, "20&deg;C") {
		t.Errorf("expected temperature in output, got: %s", got)
	}
	if !strings.Contains(got, "Clear sky") {
		t.Errorf("expected description in output, got: %s", got)
	}
}

func TestWeatherWidget_Render_FetchError(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "weather.html")
	if err := os.WriteFile(tmplPath, []byte(`{{.Temperature}}`), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	w, err := NewWeatherWidget(stubFetcher{err: errors.New("boom")}, 12.9, 77.6, tmplPath)
	if err != nil {
		t.Fatalf("NewWeatherWidget: %v", err)
	}

	if _, err := w.Render(context.Background()); err == nil {
		t.Fatal("expected error when fetch fails")
	}
}
