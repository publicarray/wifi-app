# WiFi Diagnostic & Troubleshooting App

A professional WiFi diagnostic tool for technicians working with WiFi deployments (especially Unifi APs). Built with Wails (Go + Svelte) for Linux (with future Windows support planned).

## Features

- **Real-time WiFi Network Scanning**: Discover all nearby WiFi networks with detailed information
- **Access Point Detection**: View individual APs for each SSID (network)
- **Signal Strength Monitoring**: Real-time signal tracking with color-coded indicators
- **Client Statistics**: Detailed connection stats including TX/RX rates, retry rates, signal quality
- **Misconfiguration Detection**: Automatically identify common WiFi issues:
  - Channel overlap (especially 2.4GHz)
  - Weak signals
  - Security issues (Open/WEP networks)
  - Narrow channel widths in 5GHz
  - Multiple APs on same channel
- **Roaming Detection**: Track when your device switches between APs
- **Channel Analysis**: Visualize channel utilization and congestion

## Requirements

### Linux
- Go 1.16+
- Node.js 15+
- Wails v2
- `iw` tool (usually pre-installed on most Linux distributions)
- **Elevated privileges**: WiFi scanning requires either:
  - Running with `sudo`, OR
  - Setting capabilities: `sudo setcap cap_net_admin+ep ./wifi-app`

## Building

### Development Build
```bash
wails dev
```

### Production Build
```bash
wails build
```

The binary will be created in `build/bin/wifi-app`

## Running

### Option 1: Run with sudo (easiest)
```bash
sudo ./build/bin/wifi-app
```

### Option 2: Set capabilities (recommended)
```bash
# Set capabilities once
sudo setcap cap_net_admin+ep ./build/bin/wifi-app

# Run without sudo
./build/bin/wifi-app
```

### Option 3: Development mode with sudo
```bash
sudo wails dev
```

## Usage

1. **Select WiFi Interface**: Choose your WiFi interface from the dropdown (e.g., wlp1s0)
2. **Start Scanning**: Click "Start Scanning" to begin real-time WiFi scanning
3. **View Networks**: Browse discovered networks with signal strength, channel, and security info
4. **Expand Networks**: Click on a network to see individual access points (BSSIDs)
5. **Monitor Connection**: If connected to WiFi, view detailed client statistics
6. **Identify Issues**: Networks with detected issues are highlighted in orange

## Project Structure

```
wifi-app/
├── app.go              # Main Wails app with service bindings
├── models.go           # Data structures (Network, AccessPoint, ClientStats, etc.)
├── wifi_scanner.go     # Linux WiFi scanner using iw commands
├── wifi_service.go     # WiFi service with polling, aggregation, and roaming detection
├── main.go            # Application entry point
├── frontend/
│   └── src/
│       └── App.svelte # Main UI component
└── README.md
```

## Roadmap

- [ ] Signal strength chart (real-time graph)
- [ ] Channel analyzer visualization
- [ ] Roaming history view
- [ ] Export scan results (CSV/JSON)
- [ ] Filtering and sorting options
- [ ] Column customization
- [ ] AP placement recommendations
- [ ] Windows support
- [ ] Diagnostic reports

## Troubleshooting

### "Operation not permitted" error
This means the app doesn't have permission to scan WiFi networks. Use one of the running methods above with elevated privileges.

### No WiFi interface found
Make sure your WiFi adapter is enabled and the `iw` tool can detect it:
```bash
iw dev
```

### Scanning not working
Verify that `iw` can scan manually:
```bash
sudo iw <interface> scan
```

## License

MIT
