# TRMNL BYOD/BYOS Server for Kindle Paperwhite — Weather Widget

**Date:** 2026-09-12
**Status:** Approved for implementation planning

## Goal

Serve a custom, self-hosted image endpoint for a Kindle Paperwhite running
KOReader + the [trmnl-koreader](https://github.com/usetrmnl/trmnl-koreader)
plugin, replacing TRMNL's hosted service entirely (BYOD + BYOS). First
milestone: a single weather widget, styled with TRMNL's open-source
[framework](https://trmnl.com/framework) CSS, running as a Go service on the
Raspberry Pi that already hosts `sysinfo-server`.

**Explicit non-goal for this milestone:** calendar and tasks widgets. The
long-term vision is a TRMNL-style dashboard mashup (weather + calendar +
tasks), but that's deliberately deferred to its own future brainstorming
round. This spec only covers weather, with an architecture that doesn't have
to be reworked to add more widgets later.

## Client protocol (as implemented by trmnl-koreader)

The KOReader plugin calls **only one endpoint** — no `/api/setup`, no
`/api/log`:

```
GET /api/display
Headers:
  access-token:    <user-configured API key>
  ID:              <device MAC address, header name configurable, default "ID">
  percent-charged: <battery percentage 0-100>
  png-width:       <screen width in px>
  png-height:      <screen height in px>
  rssi:            "0" (hardcoded by the plugin)
  User-Agent:      trmnl-display/0.1.0-koreader
```

Expected JSON response:

```json
{
  "status": 0,
  "image_url": "http://<server>/images/current.png",
  "filename": "current.png",
  "refresh_rate": 1800
}
```

The plugin then issues a separate `GET` to `image_url` to fetch the PNG,
decodes it, and displays it full-screen. Default poll interval is 1800s
(30 min), overridable by `refresh_rate` in the response if the user enables
"use server refresh interval" in the plugin settings.

## Architecture

```
KOReader (Kindle)
   │  GET /api/display  (headers above)
   ▼
Go server (LAN-only, systemd service on the Pi)
   ├─ 1. Validate access-token against config
   ├─ 2. Fetch weather from Open-Meteo (in-memory cache, ~15 min TTL)
   ├─ 3. Render HTML template (TRMNL framework CSS, weather widget)
   │      sized to the request's png-width × png-height
   ├─ 4. Screenshot via headless Chromium (chromedp) → PNG,
   │      overwrite generated/current.png
   ├─ 5. Respond {status, image_url, filename, refresh_rate}
   ▼
Kindle GETs image_url (served as a static file by the same Go server)
```

Single Go binary, no Docker. Chromium is already installed on the target Pi
(`/usr/bin/chromium`), used as a `chromedp` subprocess dependency.

## Components

Project root: `/home/gautam/Folio` (this directory).

```
Folio/
├── cmd/main.go              # wiring, config load, logging
├── internal/
│   ├── config/               # loads config.yaml
│   ├── weather/               # Open-Meteo client + in-memory cache
│   ├── render/                # HTML template fill + chromedp screenshot
│   ├── handlers/               # GET /api/display, GET /images/:filename
│   └── models/                 # WeatherData, DisplayResponse structs
├── templates/weather.html     # TRMNL-framework-based single-view template
└── config.yaml
```

- **`weather`**: `Get(lat, long float64) (WeatherData, error)`. Wraps the
  Open-Meteo HTTP API. Caches the last successful result in memory (no
  eviction on failure — see error handling).
- **`render`**: `Render(data WeatherData, width, height int) (pngPath string, error)`.
  Fills `templates/weather.html`, launches `chromedp` with viewport set to
  the exact requested device dimensions, screenshots to PNG, writes to
  `generated/current.png`.
- **`handlers`**: `/api/display` orchestrates
  auth → weather.Get → render.Render → JSON response. A static file server
  exposes `generated/` so the Kindle's follow-up `image_url` GET resolves.
  Both `weather` and `render` are consumed through small interfaces so
  handler tests can stub them out.

## Configuration

YAML file (`config.yaml`), loaded at startup:

```yaml
access_token: "<shared secret, must match the plugin's API Key setting>"
port: 8080
latitude: <configured by user>
longitude: <configured by user>
refresh_rate_seconds: 1800
```

Config load failure (missing file, missing required field) fails startup
immediately — no silent defaulting on required fields.

## Data flow & error handling

| Case | Behavior |
|---|---|
| Success | `200` `{"status": 0, "image_url": ..., "filename": "current.png", "refresh_rate": 1800}`. `current.png` overwritten. |
| Missing/wrong `access-token` | `401` `{"status": 401, "error": "invalid access token"}` |
| Open-Meteo unreachable, cache present | Serve cached weather (stale-but-available), log the failure server-side, still `200`. |
| Open-Meteo unreachable, no cache yet (cold start) | `502` `{"status": 502, "error": "weather unavailable"}` |
| Chromium/render failure | `5xx` `{"status": 500, "error": "render failed"}`; `current.png` is **not** overwritten, so the Kindle keeps displaying the last good image on its next poll. |

## Testing

- **`weather`**: table-driven tests against `httptest.Server` stubbing
  Open-Meteo — happy path, malformed JSON, timeout → verify cache fallback.
- **`handlers`**: `httptest` against `/api/display` with `weather`/`render`
  swapped for stub interfaces — verifies auth, JSON contract, error-status
  mapping. No real Chromium involved.
- **`render`**: one real-Chromium smoke test, tagged
  `//go:build integration`, run via `make test-integration` (kept out of
  the default fast `make test` — slow, requires Chromium present). Asserts
  a PNG is produced at the requested width × height.
- No tests for `cmd/main.go` wiring, matching the existing `PiBeacon`
  project's convention of no tests on the thin entrypoint.

## Deployment

Bare Go binary + systemd service, following the existing
`sysinfo-server.service` pattern already running on this Pi for `PiBeacon`:

```ini
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

LAN-only: no reverse proxy, no TLS, no tunnel. The Kindle only fetches while
on home WiFi. `access-token` is the only auth boundary; acceptable given the
network is not internet-exposed.

## Future work (explicitly out of scope now)

- Calendar widget, tasks widget, and a TRMNL-style mashup layout combining
  all three — separate brainstorming round once weather is working
  end-to-end on-device.
- Remote (away-from-home) access, if ever needed — would require revisiting
  the LAN-only assumption and adding a tunnel + stronger auth.
