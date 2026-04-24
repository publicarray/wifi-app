# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Install dependencies
go mod tidy && cd frontend && npm install

# Development (full app with live reload)
wails dev                        # starts Wails dev server + Vite HMR at http://localhost:34115

# Frontend only (no Go backend)
cd frontend && npm run dev

# Build
wails build                      # production binary
wails build -debug               # debug binary (then: sudo build/bin/wifi-app)

# Cross-compile
GOOS=windows GOARCH=amd64 wails build

# Verify Go code after edits
go build .

# Verify frontend after edits
cd frontend && npm run build
```

No test suite or linter is currently configured.

## Architecture

This is a **Wails v2** desktop app: Go backend + Svelte 3 frontend compiled into a single binary. The frontend is embedded via `//go:embed all:frontend/dist`.

### Data flow

1. `WiFiService` polls the active `WiFiBackend` every 4 seconds
2. Results are aggregated into `[]Network` (SSID-grouped APs) and `[]ChannelInfo`
3. `WiFiService` emits `networks:updated`, `client:updated`, and `channels:updated` events via `runtime.EventsEmit`
4. `App.svelte` listens with `EventsOn` and passes data down to components
5. Frontend can also call Go methods directly via Wails auto-generated bindings in `wailsjs/go/main/App.js`

### Platform backends (`WiFiBackend` interface)

All backends implement `wifi_scanner_interface.go`:

| File | Build tag | Platform |
|------|-----------|----------|
| `wifi_scanner_mdlayher.go` | `linux && !iw` | Linux via nl80211 netlink (default) |
| `wifi_scanner_iw.go` | `linux && iw` | Linux via `iw` shell exec (deprecated) |
| `wifi_scanner_darwin_corewlan.go` | `darwin && cgo` | macOS via CoreWLAN/cgo (experimental) |
| `wifi_scanner_darwin_corewlan_stub.go` | `darwin && !cgo` | empty stub so non-cgo darwin builds compile |
| `wifi_scanner_darwin.go` | `darwin` | macOS fallback |
| `wifi_scanner_windows.go` | `windows` | Windows native WiFi API |

### Key Go files

- `app.go` — `App` struct; every public method is auto-exposed as a Wails binding
- `wifi_service.go` — scanning loop, SSID aggregation, signal history (600 points), roaming detection
- `models.go` — all shared structs: `AccessPoint`, `Network`, `ClientStats`, `ChannelInfo`, `ScanResult`
- `oui_lookup.go` — MAC vendor lookup (cached in `~/.cache/wifi-app/oui.txt`)

### Frontend

Plain Svelte 3 (not SvelteKit). Charts use Chart.js with `chartjs-plugin-zoom`.

- `App.svelte` — top-level: tab routing, Wails event listeners, state
- `NetworkList.svelte` — largest component; AP details table with filtering. Delegates row rendering to `NetworkRow.svelte` and column headers to `NetworkListHeader.svelte`
- `SignalChart.svelte` — real-time Chart.js signal graph
- `ReportWindow.svelte` — separate window used by the JSON/CSV export flow (`SaveReport` in `app.go`)
- Wails bindings: `../wailsjs/go/main/App.js`
- Runtime events: `../wailsjs/runtime/runtime.js`
- Dark theme (`#1a1a1a` background)

## Important constraints

- **`vendor-patch/github.com/mdlayher/wifi`** is a local fork. The `go.mod` `replace` directive currently points to a published fork (`github.com/publicarray/wifi`); the local path is commented out. Check this before updating the `mdlayher/wifi` dependency.
- WiFi scanning requires elevated privileges (`sudo`) on most platforms.
- Do not use SvelteKit features — this is plain Svelte + Vite.
- Do not block the UI thread; all scanning is async via events.
- The `SaveReport` handler in `app.go` re-chowns saved files to `SUDO_UID`/`SUDO_GID` when run under sudo — preserve this when modifying file-save logic.
- Scan loop in `wifi_service.go` inherits its context from the Wails app context (via `ws.ctx`). Do not re-introduce `context.Background()` — it breaks shutdown.
- `WiFiService.updateClientStatsLocked` / `updateSignalHistoryLocked` assume the caller holds `ws.mu.Lock`. `GetClientStats` returns a deep-copied snapshot (fresh `SignalHistory`/`RoamingHistory` slices); never hand out the live backing slices.
- `BSSLoadStations` / `BSSLoadUtilization` on `AccessPoint` are `*int`. Nil means "BSS Load IE absent". `BSSLoadUtilization` is always normalised to 0-100 %. The previous `-1` sentinel convention is gone — don't reintroduce it.
- `AnalyzeRoamingQuality` returns the typed `RoamingQualityReport` struct (not `map[string]interface{}`).
- CSV export (`exportToCSV` in `app.go`) uses `encoding/csv` and writes one row per AP. The frontend `ExportControls` delegates to `ExportNetworks(format)` for server-side generation — any schema change must go in `app.go`.
- Scans retry with exponential backoff (500 ms / 1 s / 2 s) before emitting `scan:error`; the loop now also emits `channels:updated` on every successful scan.
- macOS (CoreWLAN) and Windows paths cross-compile clean but are not runtime-verified in this repo — keep changes there minimal and conservative.
- Logging uses `log/slog` via `slog.Default()` (set up in `logging.go`). Format auto-detects: text on a TTY, JSON otherwise; override with `WIFI_APP_LOG_FORMAT=text|json` and `WIFI_APP_LOG_LEVEL=debug|info|warn|error`. `App.startup` installs a forwarding handler that also sends Warn/Error records to `runtime.LogWarning`/`LogError` — do not reintroduce direct `runtime.LogXxx` or `log.Printf` calls.
- Latency sampler (`latency_sampler.go`) runs at a fixed 1 Hz independent of the scan loop, emits `latency:updated` events, and uses `golang.org/x/net/icmp` with a TCP-443 fallback — do not add a `ping` shell-out. ICMP is opened once per process at `LatencySampler.Start`; failures leave `icmpErr` populated and the sampler stays on TCP for the rest of the run. The `gateway` magic target resolves via `defaultGateway()` (Linux: `/proc/net/route`; other platforms: stub returning "not implemented"). Concurrent ICMP probes share one socket and are demuxed by `icmpReader` via `(id, seq)` — do not call `conn.ReadFrom` from anywhere else.
