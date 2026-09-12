# Dashboard Mashup — Widget Framework Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Folio's single-widget renderer with a widget framework, proven by two panels: the existing weather widget (content unchanged, relocated) and a new date/time panel.

**Architecture:** A new `internal/widget` package defines a `Widget` interface (`Render(ctx) (template.HTML, error)`) that each panel implements self-containedly — its own data fetch, its own template. `internal/render.Renderer` gains `RenderDashboard`, which composes widget fragments into a new `templates/dashboard.html` shell instead of rendering `templates/weather.html` directly. `internal/handlers.Server` no longer knows about weather at all — it just holds a `[]widget.Widget` and asks the renderer to compose them.

**Tech Stack:** Go 1.24.1, `html/template`, `chromedp` v0.10.0 (existing pins, unchanged).

**Spec:** `docs/superpowers/specs/2026-09-12-dashboard-mashup-design.md`

## Global Constraints

- Go stays at 1.24.1 and `chromedp` stays at v0.10.0 — do not let `go get`/`go mod tidy` bump either (per project `CLAUDE.md`).
- `Widget` interface is exactly `Render(ctx context.Context) (template.HTML, error)` — no other methods (per spec).
- A widget's `Render` error must never fail the whole dashboard render — `RenderDashboard` substitutes an empty fragment for that slot and continues (per spec's error-isolation requirement).
- Weather's visual content (all six stat rows, labels, values) must be byte-for-byte unchanged from today's `templates/weather.html` — only relocated into a fragment template.
- **Ruling (spec gap, resolved here):** the spec's error table doesn't cover weather's cold-start case (no cache yet, fetch fails). Under the old design this returned `502`. Under the new widget-isolation design, `HandleDisplay` no longer fetches weather itself or knows widget internals, so this case now renders an empty weather panel with `200` instead of `502` — consistent with "one broken panel never blanks the whole dashboard." This is an intentional behavior change from the original weather-only spec; `TestHandleDisplay_WeatherError` (which asserted `502`) is removed in Task 4 because `HandleDisplay` can no longer distinguish this case.
- This milestone supports exactly two widget slots (`Main` and `Corner`, positional: `widgets[0]` → Main, `widgets[1]` → Corner). Placing more than two widgets into a real grid is explicitly out of scope — a future milestone's problem.

---

### Task 1: Widget interface and DateTimeWidget

**Files:**
- Create: `internal/widget/widget.go`
- Create: `internal/widget/datetime.go`
- Create: `internal/widget/datetime_test.go`
- Create: `templates/widgets/datetime.html`

**Interfaces:**
- Produces:
  - `type Widget interface { Render(ctx context.Context) (template.HTML, error) }`
  - `func NewDateTimeWidget(templatePath string, now func() time.Time) (*DateTimeWidget, error)`
  - `func (w *DateTimeWidget) Render(ctx context.Context) (template.HTML, error)`

- [ ] **Step 1: Write the failing test**

Create `internal/widget/datetime_test.go`:

```go
package widget

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDateTimeWidget_Render(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "datetime.html")
	content := `<span class="label">{{.Now.Format "Jan 2"}}</span>` +
		`<span class="value value--large">{{.Now.Format "15:04"}}</span>` +
		`<span class="label">Updated {{.Now.Format "15:04"}}</span>`
	if err := os.WriteFile(tmplPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	fixed := time.Date(2026, 9, 12, 14, 30, 0, 0, time.UTC)
	w, err := NewDateTimeWidget(tmplPath, func() time.Time { return fixed })
	if err != nil {
		t.Fatalf("NewDateTimeWidget: %v", err)
	}

	html, err := w.Render(context.Background())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	got := string(html)
	if !strings.Contains(got, "Sep 12") {
		t.Errorf("expected date %q in output, got: %s", "Sep 12", got)
	}
	if !strings.Contains(got, "14:30") {
		t.Errorf("expected time %q in output, got: %s", "14:30", got)
	}
	if !strings.Contains(got, "Updated 14:30") {
		t.Errorf("expected 'Updated 14:30' in output, got: %s", got)
	}
}

func TestDateTimeWidget_Render_BadTemplatePath(t *testing.T) {
	if _, err := NewDateTimeWidget("/nonexistent/path.html", time.Now); err == nil {
		t.Fatal("expected error for nonexistent template path")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/widget/... -run TestDateTimeWidget -v`
Expected: FAIL — `undefined: NewDateTimeWidget` (package doesn't exist yet)

- [ ] **Step 3: Write minimal implementation**

Create `internal/widget/widget.go`:

```go
// internal/widget/widget.go
package widget

import (
	"context"
	"html/template"
)

// Widget renders one self-contained dashboard panel: it owns both its data
// fetch and its own template. A caller composing multiple widgets must
// treat a Render error as "this panel is empty," never as a reason to
// fail the whole dashboard.
type Widget interface {
	Render(ctx context.Context) (template.HTML, error)
}
```

Create `internal/widget/datetime.go`:

```go
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
```

Create `templates/widgets/datetime.html`:

```html
<!-- templates/widgets/datetime.html -->
<div class="col gap--small">
  <span class="label">{{.Now.Format "Mon, Jan 2"}}</span>
  <span class="value value--large">{{.Now.Format "15:04"}}</span>
  <span class="label">Updated {{.Now.Format "15:04"}}</span>
</div>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/widget/... -run TestDateTimeWidget -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/widget/widget.go internal/widget/datetime.go internal/widget/datetime_test.go templates/widgets/datetime.html
git commit -m "feat: add widget interface and date/time widget"
```

---

### Task 2: WeatherWidget (migrate existing weather rendering)

**Files:**
- Create: `internal/widget/weather.go`
- Create: `internal/widget/weather_test.go`
- Create: `templates/widgets/weather.html`

**Interfaces:**
- Consumes: `models.WeatherData` (unchanged, from `internal/models`).
- Produces:
  - `type WeatherFetcher interface { Get(ctx context.Context, lat, lon float64) (models.WeatherData, error) }`
  - `func NewWeatherWidget(fetcher WeatherFetcher, lat, lon float64, templatePath string) (*WeatherWidget, error)`
  - `func (w *WeatherWidget) Render(ctx context.Context) (template.HTML, error)`

- [ ] **Step 1: Write the failing test**

Create `internal/widget/weather_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/widget/... -run TestWeatherWidget -v`
Expected: FAIL — `undefined: NewWeatherWidget`

- [ ] **Step 3: Write minimal implementation**

Create `internal/widget/weather.go`:

```go
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
```

Create `templates/widgets/weather.html` — this is today's `templates/weather.html` content with the outer `<html>/<head>/<style>/<body>` shell and the `{{.Width}}`-based font-scaling stripped out (scaling moves to the dashboard shell in Task 3, so it applies to every widget consistently, not just weather):

```html
<!-- templates/widgets/weather.html -->
<div class="layout layout--col gap--large">
  <div class="columns">
    <div class="column">
      <span class="value value--xxlarge">{{printf "%.0f" .Temperature}}&deg;C</span>
      <span class="label">{{.Description}}</span>
    </div>
  </div>
  <div class="columns columns--stat">
    <div class="column">
      <span class="label">Humidity: {{.Humidity}}%</span>
    </div>
    <div class="column">
      <span class="label">Wind: {{printf "%.1f" .WindSpeed}} km/h</span>
    </div>
  </div>
  <div class="columns columns--stat">
    <div class="column">
      <span class="label">High: {{printf "%.0f" .TempMax}}&deg;C</span>
    </div>
    <div class="column">
      <span class="label">Low: {{printf "%.0f" .TempMin}}&deg;C</span>
    </div>
  </div>
  <div class="columns columns--stat">
    <div class="column">
      <span class="label">Sunrise: {{.Sunrise.Format "15:04"}}</span>
    </div>
    <div class="column">
      <span class="label">Sunset: {{.Sunset.Format "15:04"}}</span>
    </div>
  </div>
  <div class="columns columns--stat">
    <div class="column">
      <span class="label">Rain chance: {{.PrecipProbability}}%</span>
    </div>
    <div class="column">
      <span class="label">Gusts: {{printf "%.0f" .WindGusts}} km/h</span>
    </div>
  </div>
  <div class="columns columns--stat">
    <div class="column">
      <span class="label">UV index: {{printf "%.1f" .UVIndex}}</span>
    </div>
    <div class="column">
      <span class="label">Cloud: {{.CloudCover}}%</span>
    </div>
  </div>
</div>
<div class="title_bar">
  <span class="title">Weather</span>
  <span class="instance">Updated {{.FetchedAt.Format "15:04"}}</span>
</div>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/widget/... -v`
Expected: PASS (all widget package tests, Task 1 and Task 2)

- [ ] **Step 5: Commit**

```bash
git add internal/widget/weather.go internal/widget/weather_test.go templates/widgets/weather.html
git commit -m "feat: add weather widget"
```

---

### Task 3: Renderer.RenderDashboard and the dashboard shell template

**Files:**
- Create: `templates/dashboard.html`
- Modify: `internal/render/render.go`
- Create: `internal/render/render_dashboard_test.go`
- Modify: `internal/render/render_test.go`
- Delete: `templates/weather.html` (fully replaced by `templates/dashboard.html` + `templates/widgets/weather.html`)

**Interfaces:**
- Consumes: `widget.Widget` (Task 1), `*widget.DateTimeWidget`/`*widget.WeatherWidget` (Tasks 1-2, for the integration test).
- Produces:
  - `func (r *Renderer) RenderDashboard(ctx context.Context, widgets []widget.Widget, width, height int) (string, error)` — replaces `Render`.
  - unexported `func (r *Renderer) buildDashboardHTML(ctx context.Context, widgets []widget.Widget, width int) (string, error)` — the fast-testable HTML composition step, factored out from the chromedp screenshot step.

- [ ] **Step 1: Write the failing test**

Create `internal/render/render_dashboard_test.go`:

```go
// internal/render/render_dashboard_test.go
package render

import (
	"context"
	"errors"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gautam-J/Folio/internal/widget"
)

type stubWidget struct {
	html template.HTML
	err  error
}

func (s stubWidget) Render(ctx context.Context) (template.HTML, error) {
	return s.html, s.err
}

func TestBuildDashboardHTML_ComposesWidgetsAndIsolatesErrors(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "dashboard.html")
	tmplContent := `<div id="main">{{.Main}}</div><div id="corner">{{.Corner}}</div>`
	if err := os.WriteFile(tmplPath, []byte(tmplContent), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	r, err := NewRenderer(tmplPath, dir)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	widgets := []widget.Widget{
		stubWidget{html: template.HTML("<p>weather</p>")},
		stubWidget{err: errors.New("boom")},
	}

	got, err := r.buildDashboardHTML(context.Background(), widgets, 800)
	if err != nil {
		t.Fatalf("buildDashboardHTML: %v", err)
	}
	if !strings.Contains(got, "<p>weather</p>") {
		t.Errorf("expected successful widget's fragment in output, got: %s", got)
	}
	if strings.Contains(got, "boom") {
		t.Errorf("erroring widget's error text leaked into output: %s", got)
	}
	if !strings.Contains(got, `<div id="corner"></div>`) {
		t.Errorf("expected empty corner slot for the erroring widget, got: %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/... -run TestBuildDashboardHTML -v`
Expected: FAIL — `undefined: buildDashboardHTML` (compile error)

- [ ] **Step 3: Write minimal implementation**

Replace the contents of `internal/render/render.go`:

```go
// internal/render/render.go
package render

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gautam-J/Folio/internal/widget"
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

// dashboardData feeds templates/dashboard.html: Width drives --folio-scale
// (TRMNL's framework CSS is tuned for its fixed 800x480 baseline), and
// Main/Corner are the two widget slots this milestone supports.
type dashboardData struct {
	Width  int
	Main   template.HTML
	Corner template.HTML
}

// buildDashboardHTML renders each widget in order — widgets[0] into the
// Main slot, widgets[1] (if present) into Corner — and composes the
// dashboard shell around them. A widget that errors gets an empty
// fragment in its slot; it never fails the whole dashboard.
func (r *Renderer) buildDashboardHTML(ctx context.Context, widgets []widget.Widget, width int) (string, error) {
	var fragments [2]template.HTML
	for i, w := range widgets {
		if i >= len(fragments) {
			break
		}
		html, err := w.Render(ctx)
		if err != nil {
			slog.Error("widget render failed", "index", i, "error", err)
			continue
		}
		fragments[i] = html
	}

	var buf bytes.Buffer
	data := dashboardData{Width: width, Main: fragments[0], Corner: fragments[1]}
	if err := r.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func (r *Renderer) RenderDashboard(ctx context.Context, widgets []widget.Widget, width, height int) (string, error) {
	htmlStr, err := r.buildDashboardHTML(ctx, widgets, width)
	if err != nil {
		return "", err
	}

	htmlPath := filepath.Join(r.outputDir, "render.html")
	if err := os.WriteFile(htmlPath, []byte(htmlStr), 0o644); err != nil {
		return "", fmt.Errorf("write html: %w", err)
	}

	absHTMLPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return "", fmt.Errorf("resolve html path: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chromiumExecPath))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx, chromedp.WithErrorf(logChromedpError))
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

// chromedp v0.10.0 (pinned to keep go.mod at go 1.24.1) ships a cdproto
// schema that predates the "Loopback" IPAddressSpace value this Pi's newer
// Chromium sends, so every render logs a harmless unmarshal error for it.
func logChromedpError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if strings.Contains(msg, "unknown IPAddressSpace value: Loopback") {
		return
	}
	slog.Error(msg)
}
```

Create `templates/dashboard.html` (the shell — carries the CSS overrides that used to live in `templates/weather.html`, now applied once for every widget fragment dropped into it):

```html
<!-- templates/dashboard.html -->
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <link rel="stylesheet" href="https://usetrmnl.com/css/latest/plugins.css">
  <style>
    html, body { margin: 0; padding: 0; width: 100%; height: 100%; background: #fff; }
    .screen, .view { width: 100% !important; height: 100% !important; }
    /* TRMNL's framework pins .layout to a fixed 800x480 box (via --screen-w/
       --screen-h) regardless of the .screen/.view overrides above. Left in
       place, it clips/overflows once fonts are scaled up for larger canvases. */
    .layout { height: 100%; }
    /* TRMNL's framework sizes fonts/gaps for its 800x480 baseline. Scale them
       to the actual rendered canvas so text stays readable at other sizes.
       This applies to every widget fragment dropped into this shell, not
       just weather. */
    :root { --folio-scale: calc({{.Width}} / 800); }
    .value--xxlarge { font-size: calc(96px * var(--folio-scale)); line-height: calc(108px * var(--folio-scale)); }
    .label { font-size: calc(16px * var(--folio-scale)); }
    .layout.gap--large { gap: calc(20px * var(--folio-scale)); }
    /* Shrink-wrap paired stats so they sit together instead of stretching
       across the full row and leaving a large empty gap between them. */
    .columns--stat { gap: calc(60px * var(--folio-scale)); }
    .columns--stat .column { flex: none; width: auto; }
    .dashboard-corner {
      position: absolute;
      top: calc(16px * var(--folio-scale));
      right: calc(16px * var(--folio-scale));
      text-align: right;
    }
  </style>
</head>
<body class="environment trmnl">
  <div class="screen">
    <div class="view view--full">
      {{.Main}}
      <div class="dashboard-corner">{{.Corner}}</div>
    </div>
  </div>
</body>
</html>
```

Delete the old template:

```bash
rm templates/weather.html
```

Update `internal/render/render_test.go` (the existing chromedp integration smoke test) to exercise `RenderDashboard` against the real widget templates:

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
	"github.com/Gautam-J/Folio/internal/widget"
)

type fakeWeatherFetcher struct{ data models.WeatherData }

func (f fakeWeatherFetcher) Get(ctx context.Context, lat, lon float64) (models.WeatherData, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filename, err := r.RenderDashboard(ctx, []widget.Widget{weatherWidget, dateTimeWidget}, 1072, 1448)
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/render/... -run TestBuildDashboardHTML -v`
Expected: PASS

Run: `go build ./...`
Expected: builds cleanly (confirms the integration-tagged file still compiles even though it isn't run by default)

- [ ] **Step 5: Commit**

```bash
git add templates/dashboard.html internal/render/render.go internal/render/render_dashboard_test.go internal/render/render_test.go
git rm templates/weather.html
git commit -m "feat: add RenderDashboard and the dashboard shell template"
```

---

### Task 4: Wire widgets into HandleDisplay and main.go

**Files:**
- Modify: `internal/handlers/handlers.go`
- Modify: `internal/handlers/handlers_test.go`
- Modify: `cmd/main.go`

**Interfaces:**
- Consumes: `widget.Widget`, `widget.NewWeatherWidget`, `widget.NewDateTimeWidget` (Tasks 1-2), `Renderer.RenderDashboard` (Task 3).
- Produces: `func NewServer(accessToken string, refreshRate int, widgets []widget.Widget, renderer DashboardRenderer, outputDir string) *Server` — replaces the old `NewServer` signature (which took `lat, lon float64, weather WeatherFetcher, renderer ImageRenderer`).

- [ ] **Step 1: Write the failing test**

Replace `internal/handlers/handlers_test.go`:

```go
// internal/handlers/handlers_test.go
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Gautam-J/Folio/internal/models"
	"github.com/Gautam-J/Folio/internal/widget"
)

type stubWidget struct {
	html template.HTML
	err  error
}

func (s stubWidget) Render(ctx context.Context) (template.HTML, error) {
	return s.html, s.err
}

type stubRenderer struct {
	filename string
	err      error
}

func (s stubRenderer) RenderDashboard(ctx context.Context, widgets []widget.Widget, width, height int) (string, error) {
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
	srv := NewServer("secret", 1800,
		[]widget.Widget{stubWidget{html: "<p>weather</p>"}},
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
	srv := NewServer("secret", 1800, nil, stubRenderer{}, t.TempDir())

	req := newTestRequest("wrong", "1072", "1448")
	rec := httptest.NewRecorder()

	srv.HandleDisplay(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHandleDisplay_MissingDimensions(t *testing.T) {
	srv := NewServer("secret", 1800, nil, stubRenderer{}, t.TempDir())

	req := newTestRequest("secret", "", "")
	rec := httptest.NewRecorder()

	srv.HandleDisplay(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleDisplay_RenderError(t *testing.T) {
	srv := NewServer("secret", 1800, nil, stubRenderer{err: errors.New("boom")}, t.TempDir())

	req := newTestRequest("secret", "1072", "1448")
	rec := httptest.NewRecorder()

	srv.HandleDisplay(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
```

Note: `TestHandleDisplay_WeatherError` is removed — per the Global Constraints ruling, `HandleDisplay` no longer fetches weather itself, so it can no longer distinguish a weather-specific failure from any other widget failure. That per-widget error-isolation behavior is already covered by `TestBuildDashboardHTML_ComposesWidgetsAndIsolatesErrors` in Task 3.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/handlers/... -v`
Expected: FAIL — compile error, `NewServer` signature mismatch (old `handlers.go` still expects `lat, lon float64, weather WeatherFetcher, renderer ImageRenderer`)

- [ ] **Step 3: Write minimal implementation**

Replace `internal/handlers/handlers.go`:

```go
// internal/handlers/handlers.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Gautam-J/Folio/internal/models"
	"github.com/Gautam-J/Folio/internal/widget"
)

type DashboardRenderer interface {
	RenderDashboard(ctx context.Context, widgets []widget.Widget, width, height int) (string, error)
}

type Server struct {
	accessToken string
	refreshRate int
	widgets     []widget.Widget
	renderer    DashboardRenderer
	outputDir   string
}

func NewServer(accessToken string, refreshRate int, widgets []widget.Widget, renderer DashboardRenderer, outputDir string) *Server {
	return &Server{
		accessToken: accessToken,
		refreshRate: refreshRate,
		widgets:     widgets,
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

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	filename, err := s.renderer.RenderDashboard(ctx, s.widgets, width, height)
	if err != nil {
		slog.Error("render failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.DisplayResponse{
			Status: http.StatusInternalServerError,
			Error:  "render failed",
		})
		return
	}

	// The plugin invalidates its local image cache purely by comparing this
	// field to the last response's — not by content or HTTP headers — so it
	// must change on every render even though the served file is always
	// overwritten in place at the same URL.
	cacheBustFilename := fmt.Sprintf("current-%d.png", time.Now().Unix())

	slog.Info("served display request", "filename", filename)
	writeJSON(w, http.StatusOK, models.DisplayResponse{
		Status:      0,
		ImageURL:    fmt.Sprintf("http://%s/images/%s", r.Host, filename),
		Filename:    cacheBustFilename,
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

Replace `cmd/main.go`:

```go
// cmd/main.go
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Gautam-J/Folio/internal/config"
	"github.com/Gautam-J/Folio/internal/handlers"
	"github.com/Gautam-J/Folio/internal/render"
	"github.com/Gautam-J/Folio/internal/weather"
	"github.com/Gautam-J/Folio/internal/widget"
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

	weatherWidget, err := widget.NewWeatherWidget(weatherClient, cfg.Latitude, cfg.Longitude, "templates/widgets/weather.html")
	if err != nil {
		slog.Error("failed to init weather widget", "error", err)
		os.Exit(1)
	}

	dateTimeWidget, err := widget.NewDateTimeWidget("templates/widgets/datetime.html", time.Now)
	if err != nil {
		slog.Error("failed to init date/time widget", "error", err)
		os.Exit(1)
	}

	widgets := []widget.Widget{weatherWidget, dateTimeWidget}

	renderer, err := render.NewRenderer("templates/dashboard.html", outputDir)
	if err != nil {
		slog.Error("failed to init renderer", "error", err)
		os.Exit(1)
	}

	server := handlers.NewServer(
		cfg.AccessToken,
		cfg.RefreshRateSeconds,
		widgets,
		renderer,
		outputDir,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/display", server.HandleDisplay)
	mux.Handle("GET /images/", server.ImagesHandler())

	addr := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("starting server", "addr", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS across all packages

Run: `go build ./...`
Expected: builds cleanly

Run: `gofmt -l .`
Expected: no output (nothing unformatted)

Run: `go vet ./...`
Expected: no output

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/handlers.go internal/handlers/handlers_test.go cmd/main.go
git commit -m "feat: wire dashboard widgets into HandleDisplay and main"
```

---

## Final verification (after Task 4)

- [ ] Run `make test` — all unit tests pass.
- [ ] Run `make test-integration` (requires `/usr/bin/chromium`) — confirms `RenderDashboard` produces a real PNG at the requested size with both widgets composed.
- [ ] Run `make build && ./bin/folio -config config.yaml` locally (or deploy per README) and trigger a real `/api/display` request to visually confirm the corner date/time panel's placement doesn't overlap or clip against the weather panel at the Kindle's actual resolution — the exact corner styling in `templates/dashboard.html` is a starting point and may need on-device tuning, same as the original weather font-scaling work.
