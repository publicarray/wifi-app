# WIFI-APP KNOWLEDGE BASE

**Generated:** 2026-01-24T11:35:42Z  
**Commit:** 0c1339d  
**Branch:** master

## OVERVIEW

WiFi diagnostic desktop app. Go backend (Wails v2) + Svelte frontend. Multi-platform WiFi scanning with real-time visualization.

## STRUCTURE

```
wifi-app/
├── main.go                    # Wails entry point
├── app.go                     # App struct, Wails bindings (exports to frontend)
├── wifi_service.go            # Core service: polling, aggregation, events
├── wifi_scanner_interface.go  # WiFiBackend interface (all platforms implement)
├── wifi_scanner_*.go          # Platform implementations (_iw, _mdlayher, _darwin, _windows)
├── models.go                  # Data structures (AccessPoint, Network, ClientStats, etc.)
├── oui_lookup.go              # MAC vendor lookup
├── wifi_utils.go              # Shared utilities
├── vendor-patch/              # Forked github.com/mdlayher/wifi (Linux netlink)
├── frontend/
│   ├── src/App.svelte         # Main app, tab routing, Wails events
│   └── src/components/        # UI components (NetworkList, SignalChart, etc.)
└── build/                     # Platform build assets (icons, installers)
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Add Wails binding | `app.go` | Method on App struct → auto-exposed |
| WiFi data models | `models.go` | AccessPoint, Network, ClientStats, ChannelInfo |
| Scan logic (Linux) | `wifi_scanner_iw.go` or `wifi_scanner_mdlayher.go` | iw = exec, mdlayher = netlink |
| Scan logic (macOS) | `wifi_scanner_darwin.go` | CoreWLAN via cgo |
| Scan logic (Windows) | `wifi_scanner_windows.go` | Native WiFi API |
| Real-time events | `wifi_service.go` | runtime.EventsEmit for networks:updated, client:updated |
| Frontend event handling | `frontend/src/App.svelte` | EventsOn listeners |
| Add UI component | `frontend/src/components/` | Create .svelte, import in App.svelte |

## CODE MAP

### Go Backend (Key Symbols)

| Symbol | Type | File | Role |
|--------|------|------|------|
| `WiFiBackend` | interface | wifi_scanner_interface.go | Platform abstraction |
| `WiFiService` | struct | wifi_service.go | Scanning orchestration, state |
| `App` | struct | app.go | Wails bindings, frontend API |
| `AccessPoint` | struct | models.go | Single AP data (BSSID, signal, channel) |
| `Network` | struct | models.go | SSID + multiple APs |
| `ClientStats` | struct | models.go | Connected client info |

### Frontend (Svelte Components)

| Component | Purpose |
|-----------|---------|
| `NetworkList.svelte` | Main network table (64KB - largest, has AP details) |
| `SignalChart.svelte` | Real-time signal graph (Chart.js) |
| `ChannelAnalyzer.svelte` | Channel utilization view |
| `ClientStatsPanel.svelte` | Connection stats display |
| `RoamingAnalysis.svelte` | Roaming quality metrics |
| `Toolbar.svelte` | Interface selector, scan controls |
| `ExportControls.svelte` | JSON/CSV export |

## CONVENTIONS

### Go
- Platform-specific files: `*_darwin.go`, `*_windows.go`, `*_iw.go` (Linux exec), `*_mdlayher.go` (Linux netlink)
- All WiFi backends implement `WiFiBackend` interface
- Wails bindings: public methods on `App` struct auto-exposed
- Events: `runtime.EventsEmit(ctx, "event:name", data)`

### Frontend
- Svelte 3 (not SvelteKit)
- Wails bindings: import from `../wailsjs/go/main/App.js`
- Runtime events: import from `../wailsjs/runtime/runtime.js`
- Dark theme (#1a1a1a background)

## ANTI-PATTERNS

- **DO NOT** add new Go dependencies without considering `vendor-patch` impact
- **DO NOT** use SvelteKit features (this is plain Svelte + Vite)
- **DO NOT** block UI thread - scanning is async via Wails events

## UNIQUE STYLES

- `mdlayher/wifi` is FORKED in `vendor-patch/` - local modifications, not upstream
- Linux has TWO scanner backends: `iw` (shell exec) and `mdlayher` (netlink) - mdlayher preferred
- WiFi standard detection via capability parsing, not explicit API

## COMMANDS

```bash
# Development
wails dev                    # Live reload dev server

# Build
wails build                  # Production build

# Frontend only
cd frontend && npm run dev   # Vite dev server (no Go)
cd frontend && npm run build # Build frontend dist
```

## CI/CD

GitHub Actions: `.github/workflows/build.yml`
- Builds on push/PR
- Matrix: linux/amd64, windows/amd64, darwin/universal
- Uses `snider/build@main` action with `wails2` stack

## NOTES

- WiFi scanning requires elevated privileges on most platforms
- `vendor-patch/github.com/mdlayher/wifi` - local fork with fixes, check before updating mdlayher/wifi
- Large component: `NetworkList.svelte` (64KB) - handles complex AP display with filtering
- Signal history tracked in `WiFiService.signalHistory` for charting
