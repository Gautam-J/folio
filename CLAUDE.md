# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Folio is a self-hosted TRMNL-compatible (BYOD/BYOS) server that renders a weather dashboard for a Kindle Paperwhite running KOReader + the `trmnl-koreader` plugin. It serves one endpoint, `GET /api/display`, that the Kindle polls on a timer: the server fetches weather, renders an HTML template to a PNG via headless Chromium, and returns JSON pointing at the image.

## Commands

```bash
make build              # go build -o bin/folio cmd/main.go
make run                # build + run (reads config.yaml from cwd)
make test               # go test ./... (unit tests only)
make test-integration   # go test -tags=integration ./... (includes chromedp render test, needs Chromium)
make deps               # go mod tidy
make clean              # rm -rf bin generated
```

Run a single test: `go test ./internal/handlers/ -run TestHandleDisplay_Success -v`

The integration test in `internal/render/render_test.go` is gated behind `//go:build integration` and requires `/usr/bin/chromium` on the host — it will fail or hang in environments without it, so prefer `make test` for routine work.

There's no linter configured beyond `go vet` / standard `gofmt`.

## Architecture

Request flow through `cmd/main.go`'s wiring:

```
GET /api/display (access-token, png-width, png-height headers)
  → handlers.Server.HandleDisplay
      → build []widget.Widget{weatherWidget, dateTimeWidget, quoteWidget, progressWidget}  — constructed once at startup in cmd/main.go
      → render.Renderer.RenderDashboard(ctx, widgets, w, h)   — internal/render
          → each widget's own Render(ctx) (template.HTML, error) (weather's and quote's call their respective client's Get internally)
          → composes the fragments into templates/dashboard.html's Main/Corner/Strip/Bottom slots
  → JSON { image_url, filename, refresh_rate, ... }
GET /images/* → handlers.Server.ImagesHandler (serves generated/ as a static dir)
```

- **`internal/handlers`** — the only HTTP-facing package. `Server` depends on `[]widget.Widget` and a `DashboardRenderer` interface (`RenderDashboard(ctx, widgets, width, height) (string, error)`), so tests stub both (see `handlers_test.go`) instead of hitting the network or a real browser.
- **`internal/widget`** — the `Widget` interface (`Render(ctx context.Context) (template.HTML, error)`) plus `WeatherWidget`, `DateTimeWidget`, `QuoteWidget`, and `ProgressWidget` (week/month/year progress bars), its four implementations. Each widget owns both its own data fetch and its own template, so `internal/render` and `internal/handlers` don't need to know a widget's internal data shape — adding a future widget means adding a type here, not touching the render/handler plumbing. The date/time widget also renders an "Updated HH:MM" line — this used to be a TRMNL `title_bar` on the weather widget, moved here since it's really a render-time value shared across all widgets, not weather-specific data.
- **`internal/weather`** — Open-Meteo client (no API key). Caches the last successful result in memory and serves it on fetch failure rather than erroring, so a transient network blip doesn't break a render.
- **`internal/quote`** — ZenQuotes `/api/random` client (no API key), same cache-last-good-on-failure shape as `internal/weather`. Fetches a fresh random quote on every render (no daily/date-based caching).
- **`internal/render`** — `RenderDashboard` calls each widget's `Render` to get an HTML fragment, slots them positionally into four fixed regions (widgets[0] → Main, widgets[1] → Corner, widgets[2] → Strip, widgets[3] → Bottom), executes `templates/dashboard.html` as a Go template with those fragments, writes it to `generated/render.html`, then drives headless Chromium via `chromedp` (pinned to v0.10.0 — see Gotchas) to screenshot it at the requested `png-width`/`png-height` into `generated/current.png`.
- **`internal/config`** — loads and validates `config.yaml` (real file is gitignored; copy from `config.example.yaml`). Fails fast if `access_token`, `port`, or `refresh_rate_seconds` are missing/invalid.
- **`internal/models`** — shared structs (`WeatherData`, `DisplayResponse`) with the JSON tags the TRMNL protocol expects.

### Critical protocol detail: cache-busting filename

The `trmnl-koreader` plugin decides whether to re-download the image by comparing the JSON response's `filename` field to the last one it saw — **not** HTTP caching headers, not content hashing. Because of this, `HandleDisplay` always returns a timestamp-suffixed `filename` (`current-<unix>.png`) on every request even though `image_url` and the actual file on disk (`generated/current.png`) never change path. Do not "simplify" this back to a fixed filename — it was a real bug (Kindle showing stale images) fixed exactly this way.

### Gotchas

- **Go version is pinned to 1.24.1** in `go.mod` to match the toolchain actually installed on the target Raspberry Pi. `chromedp` is pinned to v0.10.0 for the same reason — newer chromedp releases require a newer Go than 1.24.1. Don't let `go get`/`go mod tidy` bump either without checking the Pi's installed Go version first.
- chromedp v0.10.0's cdproto schema predates a Chromium `IPAddressSpace` value ("Loopback") that the Pi's newer Chromium emits; `internal/render/render.go` filters that specific benign log line out via a custom `chromedp.WithErrorf` handler. It's intentional, not dead error-handling.
- `templates/dashboard.html` pulls in TRMNL's framework CSS and includes inline `<style>` overrides forcing `.screen`/`.view` to 100% width/height — without them the render comes out mostly solid gray, since the framework defaults those containers to a fixed 800×480.
- `.view` is a flex column with `.dashboard-main` (Main+Corner) and `.dashboard-footer` (Strip+Bottom) as its two items, specifically so they stack without overlapping regardless of how tall the widgets' content is. `.dashboard-main` needs `flex: 1 1 auto` for this to work — without a definite height there, the weather widget's `.layout { height: 100% }` resolves against an indefinite parent and collapses to zero.

## Deployment

Deployed as a systemd service on a Raspberry Pi (`deploy/folio.service`) — see `README.md` for the full setup, KOReader plugin configuration, and required KOReader power-management settings (Autosuspend timeout, Keep Alive) that a background-refresh setup depends on.

## Scope

Current scope is a multi-widget dashboard (weather, date/time, quote, progress) via the `internal/widget` framework, LAN-only, single shared-token auth. `docs/superpowers/specs/2026-09-12-dashboard-mashup-design.md` and the matching plan in `docs/superpowers/plans/` are the design/implementation docs this framework was built from; its "Future work" section listed daily quotes among what was deferred — that's now built (as a random, not daily, quote per the user's direction). Still deferred: GitHub stats, mental models, word of the day, and RSS tech news widgets, plus two more ZenQuotes-backed widgets — "On this day" (`today.zenquotes.io/api`) and an inspirational image (`zenquotes.io/api/image`).
