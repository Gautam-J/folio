// internal/render/render_test.go
//go:build integration

package render

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Gautam-J/Folio/internal/models"
	"github.com/Gautam-J/Folio/internal/widget"
)

type fakeWeatherFetcher struct{ data models.WeatherData }

func (f fakeWeatherFetcher) Get(ctx context.Context, lat, lon float64) (models.WeatherData, error) {
	return f.data, nil
}

type fakeQuoteFetcher struct{ data models.QuoteData }

func (f fakeQuoteFetcher) Get(ctx context.Context) (models.QuoteData, error) {
	return f.data, nil
}

func TestRenderDashboard_ProducesPNGAtRequestedSize(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRenderer("../../templates/dashboard.html", dir)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	weatherWidget, err := widget.NewWeatherWidget(
		fakeWeatherFetcher{data: models.WeatherData{
			Temperature: 22.5,
			Humidity:    50,
			WindSpeed:   3.2,
			Description: "Clear sky",
			FetchedAt:   time.Now(),
		}},
		0, 0,
		"../../templates/widgets/weather.html",
	)
	if err != nil {
		t.Fatalf("NewWeatherWidget: %v", err)
	}

	dateTimeWidget, err := widget.NewDateTimeWidget("../../templates/widgets/datetime.html", time.Now)
	if err != nil {
		t.Fatalf("NewDateTimeWidget: %v", err)
	}

	quoteWidget, err := widget.NewQuoteWidget(
		fakeQuoteFetcher{data: models.QuoteData{Quote: "Stay hungry, stay foolish.", Author: "Steve Jobs"}},
		"../../templates/widgets/quote.html",
	)
	if err != nil {
		t.Fatalf("NewQuoteWidget: %v", err)
	}

	progressWidget, err := widget.NewProgressWidget("../../templates/widgets/progress.html", time.Now)
	if err != nil {
		t.Fatalf("NewProgressWidget: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filename, err := r.RenderDashboard(ctx, []widget.Widget{weatherWidget, dateTimeWidget, quoteWidget, progressWidget}, 1072, 1448)
	if err != nil {
		t.Fatalf("RenderDashboard: %v", err)
	}

	pngPath := filepath.Join(dir, filename)
	info, err := os.Stat(pngPath)
	if err != nil {
		t.Fatalf("expected PNG at %s: %v", pngPath, err)
	}
	if info.Size() == 0 {
		t.Fatal("PNG file is empty")
	}
}
