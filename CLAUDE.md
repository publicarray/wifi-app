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
| `wifi_scanner_mdlayher.go` | `linux` | Linux via nl80211 netlink |
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
- `WiFiService.updateClientStatsLocked` / `updateSignalHistoryLocked` assume the caller holds `ws.mu.Lock`, but they no longer perform I/O: backend calls (`GetLinkInfo`/`GetStationStats`), gateway resolution, and local-IP lookup happen in `performScan` *before* the lock is taken and are passed in as arguments — do not reintroduce blocking calls under `ws.mu`. All Wails events (including `roaming:detected`, which the locked helpers return as a `*RoamingEvent`) are emitted after the lock is released. `GetClientStats` returns a deep-copied snapshot (fresh `SignalHistory`/`RoamingHistory` slices); never hand out the live backing slices.
- `BSSLoadStations` / `BSSLoadUtilization` on `AccessPoint` are `*int`. Nil means "BSS Load IE absent". `BSSLoadUtilization` is always normalised to 0-100 %. The previous `-1` sentinel convention is gone — don't reintroduce it.
- `AccessPoint.BeaconInt` is in TUs (typically 100) on every backend; the UI labels it "TU". Raw 802.11 IE parsing is centralised in `wifi_ie_parser.go` (`parseInformationElements`/`dispatchElement`) and shared by the Windows scanner and the macOS helper — extend it there, never as a per-platform switch. RSN (IE 48) is parsed in `parseInformationElements` only, NOT in `dispatchElement`, because Linux derives security from nl80211's typed RSN data.
- `AnalyzeRoamingQuality` returns the typed `RoamingQualityReport` struct (not `map[string]interface{}`).
- CSV export (`exportToCSV` in `app.go`) uses `encoding/csv` and writes one row per AP. The frontend `ExportControls` delegates to `ExportNetworks(format)` for server-side generation — any schema change must go in `app.go`.
- Scans retry with exponential backoff (500 ms / 1 s / 2 s) before emitting `scan:error`; the loop now also emits `channels:updated` on every successful scan.
- macOS (CoreWLAN) and Windows paths cross-compile clean but are not runtime-verified in this repo — keep changes there minimal and conservative.
- Logging uses `log/slog` via `slog.Default()` (set up in `logging.go`). Format auto-detects: text on a TTY, JSON otherwise; override with `WIFI_APP_LOG_FORMAT=text|json` and `WIFI_APP_LOG_LEVEL=debug|info|warn|error`. `App.startup` installs a forwarding handler that also sends Warn/Error records to `runtime.LogWarning`/`LogError` — do not reintroduce direct `runtime.LogXxx` or `log.Printf` calls.
- Latency sampler (`latency_sampler.go`) runs at a fixed 1 Hz independent of the scan loop, emits `latency:updated` events, and uses `golang.org/x/net/icmp` with a TCP-443 fallback — do not add a `ping` shell-out. On Windows, ICMP goes through iphlpapi's `IcmpSendEcho` (`latency_icmp_windows.go`) instead of a socket — raw sockets there need admin. Elsewhere ICMP is opened once per process (lazily via `ensureICMP`); failures leave `icmpErr` populated and the sampler stays on TCP for the rest of the run. The `gateway` magic target resolves via `defaultGateway()` (Linux: `/proc/net/route`; macOS: `AF_ROUTE` routing socket; Windows: `GetIpForwardTable`). Target resolution (DNS, gateway) runs off-lock in `reconcileTargets`/`resolveTargets` — don't move it back under `s.mu`. Concurrent ICMP probes share one socket and are demuxed by `icmpReader` via seq — do not call `conn.ReadFrom` from anywhere else.
- UniFi integration (`unifi_client.go` / `unifi_poller.go`) is optional and driven by `unifi_*` config fields. The poller mirrors the LatencySampler lifecycle (started once from `SetContext`, config re-read per tick, emits `unifi:updated` off-lock, `Snapshot()` for the `GetUniFiStatus` binding) and no-ops until a controller URL + API key are set. `EnrichAccessPoints` joins controller devices onto scan results by BSSID using exact-then-heuristic MAC matching (middle-4-octets + nearest last octet; ambiguous ties are skipped — keep it that way). TLS verification is skipped only when `unifi_allow_insecure_tls` is set; the API key lives in config.toml, which SaveConfig writes 0600.
- `RoamingEvent.DurationMs` is a scan-tick-resolution estimate of roam duration: `now - lastBSSIDSeenAt` measured in `updateSignalHistoryLocked`. `lastBSSIDSeenAt` is frozen across disconnects (the function only runs when connected), so a reconnect to a different BSSID naturally measures the full outage. Always overestimates by up to one `scan_interval_seconds`; precision is bounded by the scan interval. Aggregates (`AvgRoamDurationMs`, `MaxRoamDurationMs`, `SlowRoamCount`) live on `RoamingQualityReport`; `SlowRoamCount` uses a 2000 ms threshold matching the plan's "auth issues" tier.
