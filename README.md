# Folio

A self-hosted TRMNL-compatible server that renders a weather dashboard for a Kindle Paperwhite running [KOReader](https://koreader.rocks/) with the [trmnl-koreader](https://github.com/usetrmnl/trmnl-koreader) plugin.

Folio speaks TRMNL's [BYOD/BYOS](https://docs.trmnl.com/go/diy/byod-s) protocol: the Kindle polls a single HTTP endpoint on a timer, Folio renders a dashboard of widgets (weather, a date/time panel, a random quote, week/month/year progress bars, an on-this-day historical event, and GitHub stats, using [TRMNL's own framework CSS](https://trmnl.com/framework)) to a PNG via headless Chromium, and hands back a JSON response pointing at the image.

## How it works

```
Kindle (KOReader + trmnl-koreader plugin)
   │  GET /api/display   (access-token, png-width, png-height headers)
   ▼
Folio server
   │  1. each widget renders itself (weather fetches Open-Meteo and quote
   │     fetches ZenQuotes, both no API key needed; date/time and progress
   │     just read the clock)
   │  2. compose the widget fragments into templates/dashboard.html
   │  3. screenshot it with headless Chromium (chromedp) at the requested size
   │  4. write generated/current.png
   ▼
JSON response: { image_url, filename, refresh_rate, ... }
   │
   ▼
Kindle downloads generated/current.png and displays it
```

The plugin re-downloads the image whenever the response's `filename` field changes from the last one it saw — not based on HTTP caching headers — so Folio returns a timestamp-suffixed `filename` on every request even though the served file path never changes.

## Requirements

- Go 1.24.1+
- Chromium (`/usr/bin/chromium`) installed on the host that runs the server
- A Kindle (or other e-reader) running KOReader with the [trmnl-koreader](https://github.com/usetrmnl/trmnl-koreader) plugin, on the same LAN as the server

## Setup

```bash
git clone git@github.com:Gautam-J/folio.git
cd folio
cp config.example.yaml config.yaml
```

Edit `config.yaml`:

```yaml
access_token: "changeme"     # shared secret — must match the plugin's "API Key" field
port: 8080
latitude: 0.0                # your coordinates, for the weather lookup
longitude: 0.0
refresh_rate_seconds: 1800   # how often the Kindle should poll (30 min default)
github_username: "changeme"  # for the GitHub stats widget
github_token: "changeme"     # read-only PAT; GraphQL needs a token even for public data
```

Build and run:

```bash
make build
make run
```

## Configuring the KOReader plugin

In KOReader, open the TRMNL plugin's **Configure TRMNL** menu:

- **API Key** — the `access_token` from `config.yaml`
- **Base URL** — `http://<server-host>:<port>`
- **Refresh Interval** — only used if "Use server refresh interval" is off; otherwise Folio's `refresh_rate_seconds` wins

Then enable **Enable auto-refresh** so the plugin polls continuously (it fetches immediately, then repeats on the refresh interval). If auto-refresh ever gets stopped (e.g. by tapping the screen), **Start TRMNL (interactive)** resumes it.

Two KOReader device settings matter for reliability, since a suspended Kindle can't run the plugin's background refresh loop at all:

- **Settings → Screen → Autosuspend timeout** — set this off or longer than `refresh_rate_seconds`, or the Kindle will suspend itself between refreshes and stop polling.
- **More tools → Keep Alive** — enable this too; it's a separate OS-level wakelock that can be needed even with autosuspend disabled.

Trade-off: keeping the device always-awake trades battery life for reliable background updates.

## Deploying as a systemd service

```bash
make build
sudo cp deploy/folio.service /etc/systemd/system/folio.service
sudo systemctl daemon-reload
sudo systemctl enable --now folio
```

`deploy/folio.service` assumes the repo lives at `/home/gautam/Folio` and runs as user `gautam` — edit `WorkingDirectory`, `ExecStart`, and `User` to match your setup before installing.

## Development

```bash
make test              # unit tests
make test-integration  # includes the chromedp render test (requires Chromium)
make deps              # go mod tidy
make clean             # remove bin/ and generated/
```

## Project layout

- `cmd/main.go` — entry point, wiring, HTTP server
- `internal/config` — YAML config loading and validation
- `internal/widget` — the `Widget` interface and its implementations (`WeatherWidget`, `DateTimeWidget`, `QuoteWidget`, `ProgressWidget`, `OnThisDayWidget`, `GitHubStatsWidget`); each owns its own data fetch and template
- `internal/weather` — Open-Meteo client
- `internal/quote` — ZenQuotes client
- `internal/onthisday` — ZenQuotes on-this-day client
- `internal/github` — GitHub REST + GraphQL client (stars, contributions)
- `internal/render` — composes widget fragments into the dashboard and renders it to a PNG via chromedp
- `internal/handlers` — the `/api/display` and `/images/` HTTP handlers
- `templates/dashboard.html` — the dashboard shell template; `templates/widgets/` — the per-widget fragment templates
- `docs/superpowers/specs`, `docs/superpowers/plans` — the design specs and implementation plans this project was built from

## Scope

Current scope is a multi-widget dashboard (weather, date/time, a random quote, week/month/year progress bars, an on-this-day historical event, and GitHub stats), LAN-only, with a shared-token auth model. An RSS tech news widget is a potential future direction — see `docs/superpowers/specs/2026-09-12-dashboard-mashup-design.md`'s "Future work" section — not yet built. A ZenQuotes-backed inspirational image widget (`zenquotes.io/api/image`) is also on the list, along with an xkcd comic widget (`https://xkcd.com/<random_number>/info.0.json`, returns an image).

## License

MIT — see [LICENSE](LICENSE).
