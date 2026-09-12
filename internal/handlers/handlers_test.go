// internal/handlers/handlers_test.go
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Gautam-J/Folio/internal/models"
)

type stubWeather struct {
	data models.WeatherData
	err  error
}

func (s stubWeather) Get(ctx context.Context, lat, lon float64) (models.WeatherData, error) {
	return s.data, s.err
}

type stubRenderer struct {
	filename string
	err      error
}

func (s stubRenderer) Render(ctx context.Context, data models.WeatherData, width, height int) (string, error) {
	return s.filename, s.err
}

func newTestRequest(token, width, height string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/display", nil)
	if token != "" {
		req.Header.Set("access-token", token)
	}
	if width != "" {
		req.Header.Set("png-width", width)
	}
	if height != "" {
		req.Header.Set("png-height", height)
	}
	return req
}

func TestHandleDisplay_Success(t *testing.T) {
	srv := NewServer("secret", 12.9, 77.6, 1800,
		stubWeather{data: models.WeatherData{Temperature: 21.5}},
		stubRenderer{filename: "current.png"},
		t.TempDir(),
	)

	req := newTestRequest("secret", "1072", "1448")
	rec := httptest.NewRecorder()

	srv.HandleDisplay(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp models.DisplayResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasPrefix(resp.Filename, "current-") || !strings.HasSuffix(resp.Filename, ".png") {
		t.Errorf("Filename = %q, want a cache-busting name like %q", resp.Filename, "current-<timestamp>.png")
	}
	if !strings.HasSuffix(resp.ImageURL, "/images/current.png") {
		t.Errorf("ImageURL = %q, want it to end with %q", resp.ImageURL, "/images/current.png")
	}
	if resp.RefreshRate != 1800 {
		t.Errorf("RefreshRate = %d, want 1800", resp.RefreshRate)
	}
	if resp.Status != 0 {
		t.Errorf("Status = %d, want 0", resp.Status)
	}
}

func TestHandleDisplay_InvalidToken(t *testing.T) {
	srv := NewServer("secret", 12.9, 77.6, 1800, stubWeather{}, stubRenderer{}, t.TempDir())

	req := newTestRequest("wrong", "1072", "1448")
	rec := httptest.NewRecorder()

	srv.HandleDisplay(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHandleDisplay_MissingDimensions(t *testing.T) {
	srv := NewServer("secret", 12.9, 77.6, 1800, stubWeather{}, stubRenderer{}, t.TempDir())

	req := newTestRequest("secret", "", "")
	rec := httptest.NewRecorder()

	srv.HandleDisplay(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleDisplay_WeatherError(t *testing.T) {
	srv := NewServer("secret", 12.9, 77.6, 1800,
		stubWeather{err: errors.New("boom")}, stubRenderer{}, t.TempDir(),
	)

	req := newTestRequest("secret", "1072", "1448")
	rec := httptest.NewRecorder()

	srv.HandleDisplay(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

func TestHandleDisplay_RenderError(t *testing.T) {
	srv := NewServer("secret", 12.9, 77.6, 1800,
		stubWeather{data: models.WeatherData{Temperature: 20}},
		stubRenderer{err: errors.New("boom")},
		t.TempDir(),
	)

	req := newTestRequest("secret", "1072", "1448")
	rec := httptest.NewRecorder()

	srv.HandleDisplay(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
