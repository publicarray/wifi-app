# WiFi App

Cross-platform WiFi diagnostic desktop app built with a Go backend (Wails v2)
and a Svelte frontend. It scans nearby networks, aggregates AP data, and
visualizes signal, channels, and roaming in real time.

## Features

- Multi-platform WiFi scanning (Linux, macOS, Windows)
- Real-time network list with AP details and health indicators
- Signal strength over time with roaming markers
- Channel analyzer with congestion and overlap visualization
- Client stats panel (SNR, bitrate, retries, etc.)
- Roaming analysis and AP placement recommendations
- Export controls (JSON/CSV)

## Project Structure (high level)

- `main.go` / `app.go`: Wails entry point + bindings
- `wifi_service.go`: scanning orchestration + events
- `wifi_scanner_*.go`: platform-specific scan backends
- `models.go`: shared data structures
- `frontend/`: Svelte UI (Vite)

## Requirements

- Go (Wails v2 toolchain)
- Node.js + npm (frontend build)
- Platform WiFi tools/APIs (see notes below)

## Development

### Full app (Go + Svelte)

```bash
wails dev
```

This starts the Wails dev server and Vite HMR. You can also open
`http://localhost:34115` for browser development with Go bindings.

### Frontend only

```bash
cd frontend
npm install
npm run dev
```

## Building

### Default build

```bash
wails build
```

### Debug run

```bash
wails build -debug
sudo build/bin/wifi-app
```

### Cross-compile example (Windows)

```bash
GOOS=windows GOARCH=amd64 wails build
```

## Platform Notes

- WiFi scanning typically requires elevated privileges.
- Linux has two backends:
  - `iw` (shell exec)
  - `nl80211` (netlink, preferred)
- macOS uses CoreWLAN via cgo.
- Windows uses the native WiFi API.

## Events and UI

- Backend emits runtime events (e.g. `networks:updated`, `client:updated`)
- Frontend listens in `frontend/src/App.svelte`

## CI

GitHub Actions: `.github/workflows/build.yml` builds Linux/Windows/macOS.
