# TRMNL BYOS Weather Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go server that answers KOReader's `trmnl-koreader` plugin `GET /api/display` requests with a freshly rendered weather PNG, sized to the Kindle's screen, styled with TRMNL's framework CSS.

**Architecture:** A single Go binary with four internal packages — `config` (YAML load/validate), `weather` (Open-Meteo client with in-memory cache), `render` (HTML template → headless-Chromium screenshot), `handlers` (HTTP orchestration) — wired together in `cmd/main.go`, deployed as a systemd service on the Pi that already runs `sysinfo-server`.

**Tech Stack:** Go 1.24.1, standard library `net/http` (no web framework — only two routes), `gopkg.in/yaml.v3`, `github.com/chromedp/chromedp`, system Chromium at `/usr/bin/chromium`.

**Spec:** `docs/superpowers/specs/2026-09-12-trmnl-kindle-weather-server-design.md`

## Global Constraints

- Go module `github.com/Gautam-J/Folio`, Go 1.24.1.
- LAN-only deployment: no TLS, no reverse proxy, no tunnel.
- Auth is a single shared `access-token` header value, read from `config.yaml` (not committed — see Task 6).
- Chromium binary path is fixed: `/usr/bin/chromium` (confirmed present on this Pi).
- Weather source is Open-Meteo (`https://api.open-meteo.com/v1/forecast`), no API key.
- The display image is always written to `generated/current.png`, overwritten in place on each successful render; a failed render must leave the previous file untouched.
- No Docker. Deployed as a systemd service, following the existing `sysinfo-server.service` pattern (`User=gautam`, `Restart=always`, `RestartSec=5`).

---

### Task 1: Project scaffolding and data models

**Files:**
- Create: `go.mod`
- Create: `internal/models/models.go`

**Interfaces:**
- Produces: `models.WeatherData{Temperature float64, Humidity int, WindSpeed float64, Description string, FetchedAt time.Time}`
- Produces: `models.DisplayResponse{Status int, ImageURL string, Filename string, RefreshRate int, Error string}` (JSON tags: `status`, `image_url,omitempty`, `filename,omitempty`, `refresh_rate,omitempty`, `error,omitempty`)

- [ ] **Step 1: Initialize the Go module**

Run: `go mod init github.com/Gautam-J/Folio`
Expected: creates `go.mod` with `module github.com/Gautam-J/Folio` and `go 1.24.1` (or the toolchain's detected version).

- [ ] **Step 2: Write the data model types**

```go
// internal/models/models.go
package models

import "time"

type WeatherData struct {
	Temperature float64
	Humidity    int
	WindSpeed   float64
	Description string
	FetchedAt   time.Time
}

type DisplayResponse struct {
	Status      int    `json:"status"`
	ImageURL    string `json:"image_url,omitempty"`
	Filename    string `json:"filename,omitempty"`
	RefreshRate int    `json:"refresh_rate,omitempty"`
	Error       string `json:"error,omitempty"`
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 4: Commit**

```bash
git add go.mod internal/models/models.go
git commit -m "feat: scaffold Go module and weather/display data models"
```

---

### Task 2: Config package

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: none
- Produces: `config.Config{AccessToken string, Port int, Latitude float64, Longitude float64, RefreshRateSeconds int}`, `config.Load(path string) (*Config, error)`

- [ ] **Step 1: Add the YAML dependency**

Run: `go get gopkg.in/yaml.v3`

- [ ] **Step 2: Write the failing tests**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoad_Success(t *testing.T) {
	path := writeTempConfig(t, `
access_token: "secret123"
port: 8080
latitude: 12.9
longitude: 77.6
refresh_rate_seconds: 1800
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AccessToken != "secret123" {
		t.Errorf("AccessToken = %q, want %q", cfg.AccessToken, "secret123")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.Latitude != 12.9 || cfg.Longitude != 77.6 {
		t.Errorf("Latitude/Longitude = %v/%v, want 12.9/77.6", cfg.Latitude, cfg.Longitude)
	}
	if cfg.RefreshRateSeconds != 1800 {
		t.Errorf("RefreshRateSeconds = %d, want 1800", cfg.RefreshRateSeconds)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load("/nonexistent/config.yaml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_MissingAccessToken(t *testing.T) {
	path := writeTempConfig(t, `
port: 8080
refresh_rate_seconds: 1800
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing access_token")
	}
}

func TestLoad_MissingRefreshRate(t *testing.T) {
	path := writeTempConfig(t, `
access_token: "secret123"
port: 8080
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing/invalid refresh_rate_seconds")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/config/...`
Expected: FAIL — `Load` is undefined (package doesn't exist yet).

- [ ] **Step 4: Implement the config package**

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AccessToken        string  `yaml:"access_token"`
	Port               int     `yaml:"port"`
	Latitude           float64 `yaml:"latitude"`
	Longitude          float64 `yaml:"longitude"`
	RefreshRateSeconds int     `yaml:"refresh_rate_seconds"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("config: access_token is required")
	}
	if cfg.Port == 0 {
		return nil, fmt.Errorf("config: port is required")
	}
	if cfg.RefreshRateSeconds <= 0 {
		return nil, fmt.Errorf("config: refresh_rate_seconds must be positive")
	}

	return &cfg, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/config/...`
Expected: PASS (all 4 tests).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config/
git commit -m "feat: add config package with YAML load and validation"
```

---

### Task 3: Weather package (Open-Meteo client with cache)

**Files:**
- Create: `internal/weather/weather.go`
- Test: `internal/weather/weather_test.go`

**Interfaces:**
- Consumes: `models.WeatherData` (Task 1)
- Produces: `weather.DefaultBaseURL string`, `weather.NewClient(baseURL string) *Client`, `(*Client) Get(ctx context.Context, lat, lon float64) (models.WeatherData, error)`

- [ ] **Step 1: Write the failing tests**

```go
// internal/weather/weather_test.go
package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"current":{"temperature_2m":21.5,"relative_humidity_2m":55,"weather_code":3,"wind_speed_10m":4.2}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	data, err := client.Get(context.Background(), 12.9, 77.6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Temperature != 21.5 {
		t.Errorf("Temperature = %v, want 21.5", data.Temperature)
	}
	if data.Humidity != 55 {
		t.Errorf("Humidity = %v, want 55", data.Humidity)
	}
	if data.WindSpeed != 4.2 {
		t.Errorf("WindSpeed = %v, want 4.2", data.WindSpeed)
	}
	if data.Description != "Overcast" {
		t.Errorf("Description = %q, want %q", data.Description, "Overcast")
	}
}

func TestGet_ServerErrorNoCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.Get(context.Background(), 12.9, 77.6); err == nil {
		t.Fatal("expected error when no cache and server fails")
	}
}

func TestGet_ServerErrorFallsBackToCache(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"current":{"temperature_2m":18.0,"relative_humidity_2m":60,"weather_code":1,"wind_speed_10m":3.0}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	first, err := client.Get(context.Background(), 12.9, 77.6)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	second, err := client.Get(context.Background(), 12.9, 77.6)
	if err != nil {
		t.Fatalf("expected cached fallback, got error: %v", err)
	}
	if second.Temperature != first.Temperature {
		t.Errorf("expected cached data %v, got %v", first, second)
	}
}

func TestDescribeCode(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "Clear sky"},
		{61, "Slight rain"},
		{999, "Unknown"},
	}
	for _, tc := range cases {
		if got := describeCode(tc.code); got != tc.want {
			t.Errorf("describeCode(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/weather/...`
Expected: FAIL — package `weather` has no `Client`/`NewClient`/`describeCode`.

- [ ] **Step 3: Implement the weather package**

```go
// internal/weather/weather.go
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Gautam-J/Folio/internal/models"
)

const DefaultBaseURL = "https://api.open-meteo.com/v1/forecast"

type Client struct {
	baseURL    string
	httpClient *http.Client

	mu       sync.Mutex
	cached   models.WeatherData
	hasCache bool
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Get(ctx context.Context, lat, lon float64) (models.WeatherData, error) {
	url := fmt.Sprintf(
		"%s?latitude=%g&longitude=%g&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m",
		c.baseURL, lat, lon,
	)

	data, err := c.fetch(ctx, url)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.hasCache {
			return c.cached, nil
		}
		return models.WeatherData{}, err
	}

	c.mu.Lock()
	c.cached = data
	c.hasCache = true
	c.mu.Unlock()

	return data, nil
}

type openMeteoResponse struct {
	Current struct {
		Temperature      float64 `json:"temperature_2m"`
		RelativeHumidity int     `json:"relative_humidity_2m"`
		WeatherCode      int     `json:"weather_code"`
		WindSpeed        float64 `json:"wind_speed_10m"`
	} `json:"current"`
}

func (c *Client) fetch(ctx context.Context, url string) (models.WeatherData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return models.WeatherData{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.WeatherData{}, fmt.Errorf("request open-meteo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.WeatherData{}, fmt.Errorf("open-meteo returned status %d", resp.StatusCode)
	}

	var parsed openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return models.WeatherData{}, fmt.Errorf("decode open-meteo response: %w", err)
	}

	return models.WeatherData{
		Temperature: parsed.Current.Temperature,
		Humidity:    parsed.Current.RelativeHumidity,
		WindSpeed:   parsed.Current.WindSpeed,
		Description: describeCode(parsed.Current.WeatherCode),
		FetchedAt:   time.Now(),
	}, nil
}

var wmoDescriptions = map[int]string{
	0:  "Clear sky",
	1:  "Mainly clear",
	2:  "Partly cloudy",
	3:  "Overcast",
	45: "Fog",
	48: "Depositing rime fog",
	51: "Light drizzle",
	53: "Moderate drizzle",
	55: "Dense drizzle",
	61: "Slight rain",
	63: "Moderate rain",
	65: "Heavy rain",
	71: "Slight snow",
	73: "Moderate snow",
	75: "Heavy snow",
	80: "Slight rain showers",
	81: "Moderate rain showers",
	82: "Violent rain showers",
	95: "Thunderstorm",
	96: "Thunderstorm with slight hail",
	99: "Thunderstorm with heavy hail",
}

func describeCode(code int) string {
	if desc, ok := wmoDescriptions[code]; ok {
		return desc
	}
	return "Unknown"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/weather/...`
Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add internal/weather/
git commit -m "feat: add Open-Meteo weather client with cache fallback"
```

---

### Task 4: Render package (HTML template + headless Chromium screenshot)

**Files:**
- Create: `templates/weather.html`
- Create: `internal/render/render.go`
- Test: `internal/render/render_test.go` (integration-only, build-tagged)

**Interfaces:**
- Consumes: `models.WeatherData` (Task 1)
- Produces: `render.NewRenderer(templatePath, outputDir string) (*Renderer, error)`, `(*Renderer) Render(ctx context.Context, data models.WeatherData, width, height int) (filename string, err error)`

- [ ] **Step 1: Add the chromedp dependency**

Run: `go get github.com/chromedp/chromedp`

- [ ] **Step 2: Write the HTML template**

```html
<!-- templates/weather.html -->
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <link rel="stylesheet" href="https://usetrmnl.com/css/latest/plugins.css">
  <style>
    html, body { margin: 0; padding: 0; }
  </style>
</head>
<body class="environment trmnl">
  <div class="screen">
    <div class="view view--full">
      <div class="layout layout--col gap--large">
        <div class="columns">
          <div class="column">
            <span class="value value--xxlarge">{{printf "%.0f" .Temperature}}&deg;C</span>
            <span class="label">{{.Description}}</span>
          </div>
        </div>
        <div class="columns">
          <div class="column">
            <span class="label">Humidity: {{.Humidity}}%</span>
          </div>
          <div class="column">
            <span class="label">Wind: {{printf "%.1f" .WindSpeed}} km/h</span>
          </div>
        </div>
      </div>
      <div class="title_bar">
        <span class="title">Weather</span>
        <span class="instance">Updated {{.FetchedAt.Format "15:04"}}</span>
      </div>
    </div>
  </div>
</body>
</html>
```

Note: class names (`screen`, `view--full`, `layout--col`, `value--xxlarge`, `title_bar`, etc.) follow TRMNL's published framework conventions. Task 7's manual verification step includes visually checking the rendered PNG and adjusting classes against `https://trmnl.com/framework` if the layout looks off — that's a visual-polish pass, not a functional blocker.

- [ ] **Step 3: Write the failing integration test**

```go
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
```

- [ ] **Step 4: Run the integration test to verify it fails**

Run: `go test -tags=integration ./internal/render/...`
Expected: FAIL to compile — `NewRenderer`/`Renderer` don't exist yet.

- [ ] **Step 5: Implement the render package**

```go
// internal/render/render.go
package render

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/Gautam-J/Folio/internal/models"
	"github.com/chromedp/chromedp"
)

const chromiumExecPath = "/usr/bin/chromium"

type Renderer struct {
	tmpl      *template.Template
	outputDir string
}

func NewRenderer(templatePath, outputDir string) (*Renderer, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	return &Renderer{tmpl: tmpl, outputDir: outputDir}, nil
}

func (r *Renderer) Render(ctx context.Context, data models.WeatherData, width, height int) (string, error) {
	var htmlBuf bytes.Buffer
	if err := r.tmpl.Execute(&htmlBuf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	htmlPath := filepath.Join(r.outputDir, "render.html")
	if err := os.WriteFile(htmlPath, htmlBuf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("write html: %w", err)
	}

	absHTMLPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return "", fmt.Errorf("resolve html path: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chromiumExecPath))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	var pngBuf []byte
	err = chromedp.Run(browserCtx,
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.Navigate("file://"+absHTMLPath),
		chromedp.CaptureScreenshot(&pngBuf),
	)
	if err != nil {
		return "", fmt.Errorf("chromedp render: %w", err)
	}

	filename := "current.png"
	pngPath := filepath.Join(r.outputDir, filename)
	if err := os.WriteFile(pngPath, pngBuf, 0o644); err != nil {
		return "", fmt.Errorf("write png: %w", err)
	}

	return filename, nil
}
```

- [ ] **Step 6: Run the integration test to verify it passes**

Run: `go test -tags=integration ./internal/render/...`
Expected: PASS. (Requires Chromium at `/usr/bin/chromium` — already installed on this Pi.)

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum templates/weather.html internal/render/
git commit -m "feat: render weather HTML to PNG via headless Chromium"
```

---

### Task 5: Handlers package (HTTP orchestration)

**Files:**
- Create: `internal/handlers/handlers.go`
- Test: `internal/handlers/handlers_test.go`

**Interfaces:**
- Consumes: `models.WeatherData`, `models.DisplayResponse` (Task 1); matches `weather.Client.Get` (Task 3) and `render.Renderer.Render` (Task 4) signatures via local interfaces
- Produces: `handlers.WeatherFetcher`, `handlers.ImageRenderer` interfaces; `handlers.NewServer(accessToken string, lat, lon float64, refreshRate int, weather WeatherFetcher, renderer ImageRenderer, outputDir string) *Server`; `(*Server) HandleDisplay(w http.ResponseWriter, r *http.Request)`; `(*Server) ImagesHandler() http.Handler`

- [ ] **Step 1: Write the failing tests**

```go
// internal/handlers/handlers_test.go
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	if resp.Filename != "current.png" {
		t.Errorf("Filename = %q, want %q", resp.Filename, "current.png")
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handlers/...`
Expected: FAIL — package `handlers` has no `NewServer`/`Server`.

- [ ] **Step 3: Implement the handlers package**

```go
// internal/handlers/handlers.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Gautam-J/Folio/internal/models"
)

type WeatherFetcher interface {
	Get(ctx context.Context, lat, lon float64) (models.WeatherData, error)
}

type ImageRenderer interface {
	Render(ctx context.Context, data models.WeatherData, width, height int) (string, error)
}

type Server struct {
	accessToken string
	lat         float64
	lon         float64
	refreshRate int
	weather     WeatherFetcher
	renderer    ImageRenderer
	outputDir   string
}

func NewServer(accessToken string, lat, lon float64, refreshRate int, weather WeatherFetcher, renderer ImageRenderer, outputDir string) *Server {
	return &Server{
		accessToken: accessToken,
		lat:         lat,
		lon:         lon,
		refreshRate: refreshRate,
		weather:     weather,
		renderer:    renderer,
		outputDir:   outputDir,
	}
}

func (s *Server) HandleDisplay(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("access-token") != s.accessToken {
		writeJSON(w, http.StatusUnauthorized, models.DisplayResponse{
			Status: http.StatusUnauthorized,
			Error:  "invalid access token",
		})
		return
	}

	width, werr := strconv.Atoi(r.Header.Get("png-width"))
	height, herr := strconv.Atoi(r.Header.Get("png-height"))
	if werr != nil || herr != nil || width <= 0 || height <= 0 {
		writeJSON(w, http.StatusBadRequest, models.DisplayResponse{
			Status: http.StatusBadRequest,
			Error:  "missing or invalid png-width/png-height headers",
		})
		return
	}

	weatherData, err := s.weather.Get(r.Context(), s.lat, s.lon)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, models.DisplayResponse{
			Status: http.StatusBadGateway,
			Error:  "weather unavailable",
		})
		return
	}

	filename, err := s.renderer.Render(r.Context(), weatherData, width, height)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.DisplayResponse{
			Status: http.StatusInternalServerError,
			Error:  "render failed",
		})
		return
	}

	writeJSON(w, http.StatusOK, models.DisplayResponse{
		Status:      0,
		ImageURL:    fmt.Sprintf("http://%s/images/%s", r.Host, filename),
		Filename:    filename,
		RefreshRate: s.refreshRate,
	})
}

func (s *Server) ImagesHandler() http.Handler {
	return http.StripPrefix("/images/", http.FileServer(http.Dir(s.outputDir)))
}

func writeJSON(w http.ResponseWriter, status int, body models.DisplayResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handlers/...`
Expected: PASS (all 5 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/
git commit -m "feat: add /api/display and /images HTTP handlers"
```

---

### Task 6: Main wiring, config template, and Makefile

**Files:**
- Create: `cmd/main.go`
- Create: `config.example.yaml`
- Create: `.gitignore`
- Create: `Makefile`

**Interfaces:**
- Consumes: `config.Load` (Task 2), `weather.NewClient`/`weather.DefaultBaseURL` (Task 3), `render.NewRenderer` (Task 4), `handlers.NewServer` (Task 5)
- Produces: the `folio` binary (built to `bin/folio`)

- [ ] **Step 1: Write `.gitignore`**

```
bin/
generated/
config.yaml
```

- [ ] **Step 2: Write the config template**

```yaml
# config.example.yaml
# Copy to config.yaml and fill in real values before running.
access_token: "changeme"
port: 8080
latitude: 0.0
longitude: 0.0
refresh_rate_seconds: 1800
```

- [ ] **Step 3: Write `cmd/main.go`**

```go
// cmd/main.go
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/Gautam-J/Folio/internal/config"
	"github.com/Gautam-J/Folio/internal/handlers"
	"github.com/Gautam-J/Folio/internal/render"
	"github.com/Gautam-J/Folio/internal/weather"
)

const outputDir = "generated"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	weatherClient := weather.NewClient(weather.DefaultBaseURL)

	renderer, err := render.NewRenderer("templates/weather.html", outputDir)
	if err != nil {
		slog.Error("failed to init renderer", "error", err)
		os.Exit(1)
	}

	server := handlers.NewServer(
		cfg.AccessToken,
		cfg.Latitude,
		cfg.Longitude,
		cfg.RefreshRateSeconds,
		weatherClient,
		renderer,
		outputDir,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/display", server.HandleDisplay)
	mux.Handle("GET /images/", server.ImagesHandler())

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Write the Makefile**

```makefile
APP_NAME = folio
SRC = cmd/main.go
BIN_DIR = bin
EXEC = $(BIN_DIR)/$(APP_NAME)

.PHONY: all
all: build

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(EXEC) $(SRC)

.PHONY: run
run: build
	./$(EXEC)

.PHONY: test
test:
	go test ./...

.PHONY: test-integration
test-integration:
	go test -tags=integration ./...

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) generated

.PHONY: deps
deps:
	go mod tidy
```

- [ ] **Step 5: Build the binary**

Run: `make build`
Expected: `bin/folio` created, no errors.

- [ ] **Step 6: Run the full unit test suite**

Run: `make test`
Expected: PASS across `internal/config`, `internal/weather`, `internal/handlers` (render's integration test is excluded by default).

- [ ] **Step 7: Commit**

```bash
git add cmd/main.go config.example.yaml .gitignore Makefile
git commit -m "feat: wire main entrypoint, config template, and Makefile"
```

---

### Task 7: Systemd deployment and end-to-end verification

**Files:**
- Create: `deploy/folio.service`

**Interfaces:**
- Consumes: `bin/folio` (Task 6), `config.yaml` (created by the user in this task, from `config.example.yaml`)
- Produces: a running `folio.service` reachable from the Kindle over LAN

- [ ] **Step 1: Write the systemd unit file**

```ini
# deploy/folio.service
[Unit]
Description=TRMNL BYOS server for Kindle Paperwhite
After=network.target

[Service]
User=gautam
WorkingDirectory=/home/gautam/Folio
ExecStart=/home/gautam/Folio/bin/folio
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 2: Commit the unit file**

```bash
git add deploy/folio.service
git commit -m "chore: add systemd unit for folio weather server"
```

- [ ] **Step 3: Create and fill in the real config**

Run: `cp config.example.yaml config.yaml`
Then edit `config.yaml`: set `access_token` to a private random string, and `latitude`/`longitude` to your actual coordinates. This file is gitignored — it will not be committed.

- [ ] **Step 4: Find the Pi's LAN IP (needed for the KOReader plugin's Base URL)**

Run: `hostname -I`
Expected: prints one or more LAN IPs (e.g. `192.168.1.42`); note the one on your home network.

- [ ] **Step 5: Smoke-test the binary directly (before installing the service)**

Run: `make build && ./bin/folio &`
Then in another shell:
```bash
curl -i http://localhost:8080/api/display \
  -H "access-token: <the token from config.yaml>" \
  -H "png-width: 1072" \
  -H "png-height: 1448"
```
Expected: `200 OK`, JSON body with `"status":0`, an `image_url`, and `generated/current.png` exists and is non-empty (`ls -la generated/`). Stop the foreground process (`kill %1`) once confirmed.

- [ ] **Step 6: Install and enable the systemd service** (requires `sudo` — confirm before running on a shared/production Pi)

```bash
sudo cp deploy/folio.service /etc/systemd/system/folio.service
sudo systemctl daemon-reload
sudo systemctl enable --now folio
sudo systemctl status folio
```
Expected: `Active: active (running)`.

- [ ] **Step 7: Configure the KOReader plugin on the Kindle**

On the Kindle: Tools → TRMNL Display → Configure TRMNL:
- **Base URL:** `http://<LAN IP from Step 4>:8080`
- **API Key:** the `access_token` value from `config.yaml`
- **MAC address header name:** leave as default (`ID`)
- Leave refresh interval at default, or enable "use server refresh interval" to follow `refresh_rate_seconds` from `config.yaml`

- [ ] **Step 8: Verify on-device**

Trigger a manual refresh from the KOReader plugin menu. Expected: the Kindle displays the rendered weather PNG. Check `sudo journalctl -u folio -f` on the Pi while triggering to confirm the request was received and served `200`.

If the layout looks visually off against TRMNL's actual framework classes, adjust `templates/weather.html` against `https://trmnl.com/framework`, then repeat Steps 5 and 8 (no service restart needed for template changes — restart with `sudo systemctl restart folio` only if you change Go code).

---

## Self-Review Notes

- **Spec coverage:** protocol contract (Task 5), architecture pipeline (Tasks 3–5), config (Task 2), error-handling table (Task 5's five tests), testing strategy (unit tests per task + Task 4's tagged integration test), deployment (Task 7) — all covered.
- **Type consistency checked:** `weather.Client.Get` and `render.Renderer.Render` signatures match the `handlers.WeatherFetcher`/`handlers.ImageRenderer` interfaces exactly (same param/return types), verified against Task 1's `models.WeatherData`.
- **No placeholders:** all code blocks are complete; the only user-filled values are the two YAML fields in `config.yaml` (Task 7, Step 3), which is expected runtime configuration, not a plan gap.
