# Problem Statement
Build a WiFi diagnostic and troubleshooting application for technicians working with WiFi deployments (especially Unifi APs). The app needs to provide comprehensive network scanning, real-time signal monitoring, channel analysis, client roaming tracking, and misconfiguration detection - similar to WiFi Explorer Pro and UniFi WiFiman but with enhanced technical features.
# Current State
The project is a fresh Wails v2 + Svelte application with:
* Basic Wails scaffold with a sample Greet function (app.go:25-27)
* Svelte frontend with Vite build system
* No WiFi scanning or monitoring functionality yet
* Linux environment with `iw` and `nmcli` available for WiFi operations
* WiFi interface detected: wlp1s0 with connection statistics available via `iw` commands
Key Linux WiFi capabilities available:
* `iw dev` - list WiFi interfaces
* `iw <interface> scan` - scan for networks (requires elevated privileges)
* `iw <interface> link` - current connection details (signal, bitrate, channel)
* `iw <interface> station dump` - detailed client statistics (retries, signal avg, roaming data)
* `iw phy` - hardware capabilities
# Proposed Changes
## Architecture Overview
**Backend (Go):**
* WiFi scanner service (privileged operations handler)
* Data polling/streaming system for real-time updates
* Cross-platform abstraction layer (Linux priority, Windows stub for future)
* Data models for networks, APs, channels, client stats
**Frontend (Svelte):**
* Split-pane layout: network list (top) + signal/channel charts (bottom)
* Component-based architecture with stores for reactive state
* Sortable/filterable data tables with expandable rows
* Real-time charts using a charting library
* Color-coded health indicators
## Phase 1: Backend WiFi Data Collection
### 1.1 Go Data Models
Create structured types in Go for:
* `AccessPoint` - BSSID, SSID, signal, noise, channel, width, frequency, TX power, security, capabilities
* `Network` - SSID with multiple APs (one per BSSID)
* `ClientStats` - connected AP, signal history, roaming events, TX/RX rates, retries, packet loss
* `ChannelInfo` - channel number, frequency, utilization, overlapping networks
### 1.2 WiFi Scanner Service (wifi_scanner.go)
Implement Linux WiFi operations:
* `GetInterfaces()` - detect WiFi interfaces using `iw dev`
* `ScanNetworks(interface)` - parse `iw <interface> scan` output (requires sudo/capabilities)
* `GetLinkInfo(interface)` - parse `iw <interface> link` for current connection
* `GetStationStats(interface)` - parse `iw <interface> station dump` for detailed client info
* Parse key metrics: signal strength (dBm), noise, channel, width (20/40/80/160MHz), TX/RX bitrate, BSSID, SSID, frequency, security (WPA2/WPA3), TX retries, failed packets
### 1.3 Permission Handling
Linux requires elevated privileges for WiFi scanning:
* Document requirement to run with `sudo` or set capabilities: `sudo setcap cap_net_admin+ep ./wifi-app`
* Detect permission errors and provide user-friendly error messages
* Gracefully handle permission denial
### 1.4 Polling & Data Aggregation
* Implement periodic scanning (configurable interval, default 2-3 seconds)
* Aggregate APs by SSID to create network groups
* Track signal history over time (circular buffer, last 5-10 minutes)
* Detect roaming events (BSSID changes for connected network)
* Calculate channel utilization and detect congestion
## Phase 2: Backend API Binding
### 2.1 Wails Service Binding
Expose WiFi operations to frontend via Wails bindings:
* `GetAvailableInterfaces() []string`
* `StartScanning(interface string) error` - begin periodic scanning
* `StopScanning()`
* `GetNetworks() []Network` - return all discovered networks with APs
* `GetClientStats() ClientStats` - return current client connection stats
* `GetChannelAnalysis() []ChannelInfo` - return channel utilization data
### 2.2 Real-time Events
Use Wails runtime events to push updates to frontend:
* Emit "networks:updated" event on each scan completion
* Emit "client:updated" event when client stats change
* Emit "roaming:detected" event when client switches APs
## Phase 3: Frontend UI Components
### 3.1 Project Setup
Install frontend dependencies:
* Chart library: Chart.js with chartjs-adapter-date-fns for time-series
* UI components: Consider a lightweight component library or custom components
* State management: Svelte stores for reactive data
### 3.2 Layout Structure
Replace App.svelte with split-pane layout:
* Top pane (60%): Network list with AP details
* Bottom pane (40%): Tabbed view for signal chart and channel analyzer
* Toolbar: interface selector, scan controls, column customization
### 3.3 Network List Component
Implement sortable/filterable table:
* Columns: SSID, # of APs, Channel, Signal (strongest AP), Width, Security, Status
* Expandable rows showing individual APs per SSID (BSSID, signal, channel, TX power, TX/RX rate)
* Color coding: Green (good signal >-60dBm), Orange (moderate -60 to -75dBm), Red (poor <-75dBm)
* Sort by: SSID, signal, channel, security
* Filter by: SSID text, channel, signal threshold, security type
* User-configurable columns (show/hide TX power, noise, width, etc.)
### 3.4 Signal Strength Chart
Real-time line chart showing:
* Signal strength over time for connected network
* Signal per AP if multiple visible (different colored lines)
* Roaming event markers (vertical lines/annotations when switching APs)
* Time window: last 5-10 minutes with auto-scroll
### 3.5 Channel Analyzer
Visualize channel utilization:
* Bar chart or spectrum view showing 2.4GHz and 5GHz bands
* Networks plotted by channel with width visualization (20/40/80/160MHz)
* Overlapping channel detection highlighted
* Color coding for congestion: Green (1-2 networks), Orange (3-4), Red (5+)
### 3.6 Client Stats Panel
Show current client connection details:
* Connected SSID and BSSID
* Current signal, noise, SNR
* TX/RX bitrate and packet counts
* Retries and failed packet percentage
* Connection duration
* Roaming history table: timestamp, previous AP, new AP, signal at switch
## Phase 4: Advanced Features
### 4.1 Misconfiguration Detection
Analyze and highlight issues:
* **Channel overlap**: APs on same/overlapping channels (especially 2.4GHz)
* **Weak signal from connected AP**: Client connected to distant AP while closer one available (sticky client detection)
* **High retry rate**: >10% retries indicates poor link quality
* **Narrow channel width**: AP advertising 20MHz in 5GHz when 40/80MHz possible
* **Security issues**: WPA2/TKIP or open networks
* **Band steering problems**: Same SSID on both bands but only 2.4GHz visible
* Display warnings in network list with orange/red indicators and tooltips explaining issues
### 4.2 Roaming Analysis
Track and visualize client roaming behavior:
* Log all roaming events with timestamp, signal levels, and APs involved
* Calculate roaming thresholds (at what signal strength does client roam)
* Detect sticky client behavior (not roaming to better AP)
* Show roaming quality metrics (signal before/after, downtime during roam)
### 4.3 AP Placement Recommendations
Based on collected data:
* Identify coverage gaps (areas with weak signal)
* Suggest optimal AP placement based on signal strength patterns
* Recommend channel assignments to minimize interference
### 4.4 Export & Reporting
* Export scan results to CSV/JSON
* Generate diagnostic reports with screenshots and detected issues
* Save signal history and roaming data for later analysis
## Phase 5: Cross-platform Support (Future)
### 5.1 Windows Backend
Implement Windows WiFi APIs:
* Use Windows Native WiFi API via CGo or third-party Go library
* Map Windows data structures to existing models
* Handle Windows-specific permissions (admin rights)
### 5.2 Platform Abstraction
Refactor scanner to use interface:
* Define `WiFiScanner` interface with common methods
* Implement `LinuxScanner` and `WindowsScanner`
* Auto-detect platform and instantiate correct scanner
## Technical Considerations
### Privileges
* Linux scanning requires `CAP_NET_ADMIN` capability or sudo
* Option 1: Run entire app with sudo (document in README)
* Option 2: Set capabilities on binary: `sudo setcap cap_net_admin+ep ./wifi-app`
* Option 3: Use a privileged helper binary called by main app
### Performance
* WiFi scanning can be CPU intensive, limit scan frequency (2-5 second intervals)
* Use goroutines for non-blocking scans
* Limit signal history retention (circular buffer)
* Debounce frontend updates to avoid excessive re-renders
### Data Parsing
* `iw` output format may vary across kernel/driver versions
* Implement robust regex-based parsing with error handling
* Test with different WiFi chipsets and drivers
### UI Responsiveness
* Use Svelte stores for reactive state management
* Implement virtual scrolling if network list becomes large (>100 networks)
* Throttle chart updates to maintain 60fps
## Implementation Order
1. Backend: Data models and WiFi scanner (Phase 1)
2. Backend: Wails bindings and events (Phase 2)
3. Frontend: Basic layout and network list (Phase 3.1-3.3)
4. Frontend: Charts (Phase 3.4-3.5)
5. Frontend: Client stats panel (Phase 3.6)
6. Advanced: Misconfiguration detection (Phase 4.1-4.2)
7. Advanced: Roaming analysis and recommendations (Phase 4.3-4.4)
8. Future: Windows support (Phase 5)
