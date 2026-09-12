# Dashboard Mashup — Widget Framework (Weather + Date/Time)

**Date:** 2026-09-12
**Status:** Approved for implementation planning

## Goal

Turn Folio from a single-widget renderer into a multi-panel TRMNL-style
dashboard. This milestone builds the widget seam itself and proves it with
the two simplest panels: the existing weather widget (unchanged content,
relocated into the new structure) and a new date/time panel showing
current date, time, and last-updated timestamp.

This is the first of several planned sub-projects. The user has already
named a longer list of future panels — GitHub stats, daily quotes, mental
models, word of the day, RSS tech news — each to be added later as its own
small addition once this framework exists. Building the seam now (rather
than hand-wiring a second widget into the existing single-template
approach) avoids near-certain rework, since those follow-on widgets are
already planned, not hypothetical.

**Explicit non-goal for this milestone:** implementing any widget beyond
weather and date/time. GitHub/quotes/RSS/etc. are future sub-projects, each
to go through its own brainstorming round.

## Layout

Weather keeps the hero placement it has today (most of the screen). The
date/time panel is a small corner box (exact corner/sizing is an
implementation-level CSS decision, not fixed here) showing:
- Current date and time (server-computed at render time, same
  `timezone=auto` locality as weather's existing sunrise/sunset handling)
- A "last updated" timestamp

## Architecture

```
GET /api/display
  → handlers.Server.HandleDisplay
      → build []widget.Widget{weatherWidget, dateTimeWidget}
      → render.Renderer.RenderDashboard(ctx, widgets, w, h)
          → for each widget: widget.Render(ctx) → template.HTML fragment
          → compose fragments into templates/dashboard.html (grid shell)
          → chromedp screenshot (unchanged from today)
  → JSON { image_url, filename, refresh_rate, ... }
```

`internal/render.Render` (weather-specific) is replaced by
`RenderDashboard`, which is widget-agnostic — it doesn't know what weather
or date/time *are*, only that it has a list of things that each produce an
HTML fragment.

## Components

```
Folio/
├── internal/
│   ├── widget/                       # NEW
│   │   ├── widget.go                  # Widget interface
│   │   ├── weather_widget.go           # wraps weather.Client
│   │   └── datetime_widget.go          # no external dependency
│   ├── render/render.go               # RenderDashboard replaces Render
│   ├── weather/                       # unchanged
│   ├── handlers/                      # HandleDisplay updated to build widgets
│   └── models/                        # unchanged
├── templates/
│   ├── dashboard.html                 # NEW — grid shell, two named slots
│   └── widgets/
│       ├── weather.html                # moved from templates/weather.html, unchanged content
│       └── datetime.html               # NEW
```

- **`widget.Widget`** interface:
  ```go
  type Widget interface {
      Render(ctx context.Context) (template.HTML, error)
  }
  ```
  Each widget owns its own data fetch *and* its own template — fully
  self-contained. Nothing outside `internal/widget` needs to know a
  widget's internal data shape.
- **`WeatherWidget`**: wraps today's `weather.Client`, renders
  `templates/widgets/weather.html` with the same `templateData` shape
  (`models.WeatherData` + `Width`/`Height`) used today.
- **`DateTimeWidget`**: no fetch step — `Render` just computes `time.Now()`
  (local, matching the server's configured timezone) and the last dashboard
  render time, and executes `templates/widgets/datetime.html`.
- **`Renderer.RenderDashboard(ctx, widgets []widget.Widget, width, height int) (string, error)`**:
  calls each widget's `Render`, executes `templates/dashboard.html` with
  the resulting fragments (as `template.HTML`, so they're not re-escaped),
  writes `generated/render.html`, screenshots via chromedp exactly as
  today's `Render` does.

## Data flow & error handling

| Case | Behavior |
|---|---|
| All widgets render successfully | Dashboard composed and screenshotted normally. |
| One widget's `Render` returns an error | That slot gets an empty placeholder fragment; the rest of the dashboard still renders and the PNG is still produced. A broken panel never blanks the whole screen. |
| Weather fetch fails but a cached result exists | Unchanged from today — `weather.Client` serves the cache internally; `WeatherWidget.Render` never sees this as an error. |
| Chromium/render failure | Unchanged from today — `5xx`, `current.png` not overwritten, Kindle keeps its last good image. |

This error-isolation behavior is the pattern every future widget
(GitHub, RSS, etc.) is expected to follow: fetch failures should degrade
that widget's own panel, never the whole dashboard.

## Testing

- **`widget`**: unit tests per widget — `WeatherWidget.Render` (stub
  `weather.Client`, assert expected content appears in the returned
  fragment) and `DateTimeWidget.Render` (assert date/time content appears,
  using a fixed clock if needed for determinism).
- **`render`**: `RenderDashboard` test with two stub widgets (one erroring,
  one succeeding) asserting both the successful fragment and the
  error-placeholder appear in the composed HTML.
- **`render` integration test**: existing `//go:build integration` chromedp
  smoke test updated to call `RenderDashboard` instead of `Render`.
- **`handlers`**: existing `HandleDisplay` tests updated for the new widget
  list construction; stubbed widgets replace the stubbed `ImageRenderer`
  interface's weather-specific shape as needed.

## Future work (explicitly out of scope now)

- GitHub stats, daily quotes, mental models, word of the day, RSS tech
  news widgets — each a separate brainstorming round once this framework
  is proven end-to-end on-device.
- Remote (away-from-home) access — unrelated to this milestone, still
  deferred per the original weather spec.
