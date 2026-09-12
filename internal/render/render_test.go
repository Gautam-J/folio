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
)

func TestRender_ProducesPNGAtRequestedSize(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRenderer("../../templates/weather.html", dir)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	data := models.WeatherData{
		Temperature: 22.5,
		Humidity:    50,
		WindSpeed:   3.2,
		Description: "Clear sky",
		FetchedAt:   time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filename, err := r.Render(ctx, data, 1072, 1448)
	if err != nil {
		t.Fatalf("Render: %v", err)
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
