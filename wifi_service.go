package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// scanBackoffDelays are the delays between retry attempts when a scan fails.
// After exhausting these, the loop emits `scan:error` and waits out the next
// scan interval tick rather than thrashing the driver.
var scanBackoffDelays = []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}

// WiFiService manages WiFi scanning and data aggregation
type WiFiService struct {
	scanner          WiFiBackend
	ctx              context.Context
	mu               sync.RWMutex
	scanning         bool
	cancelFunc       context.CancelFunc
	currentInterface string
	scanInFlight     atomic.Bool

	// Live config — read at the top of each scan iteration so SaveConfig()
	// changes (e.g. scan_interval) take effect on the next tick without
	// restarting the scan loop.
	config *liveConfig

	// latencySampler runs independently of the scan loop at a fixed 1 Hz so
	// RTT/jitter/loss measurements aren't throttled by scan_interval. Started
	// on first SetContext and stopped from Close.
	latencySampler *LatencySampler
	samplerOnce    sync.Once
	samplerCancel  context.CancelFunc

	// Aggregated data
	networks       []Network
	channelInfo    []ChannelInfo
	clientStats    ClientStats
	lastScanResult *ScanResult

	// Signal history tracking
	signalHistory  []SignalDataPoint
	roamingHistory []RoamingEvent
	lastBSSID      string
	// lastBSSIDSeenAt is the wall-clock time of the last scan tick that
	// observed lastBSSID as the active (connected) BSSID. Used to estimate
	// RoamingEvent.DurationMs on the next BSSID transition. Stays frozen
	// across disconnects so a reconnect to a different BSSID naturally
	// reports duration-including-the-gap rather than duration-since-reconnect.
	lastBSSIDSeenAt time.Time
}

// NewWiFiService creates a new WiFi service
func NewWiFiService() *WiFiService {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	cacheFile := filepath.Join(cacheDir, "wifi-app", "oui.txt")

	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		cacheFile = filepath.Join(os.TempDir(), "oui.txt")
	}

	// Load config eagerly so the first scan respects user settings. A failed
	// load (corrupted file, etc.) falls back to defaults inside LoadConfig.
	cfg, err := LoadConfig()
	if err != nil {
		slog.Warn("config load failed, using defaults", "err", err)
	}

	ws := &WiFiService{
		scanner:        NewWiFiScanner(cacheFile),
		config:         newLiveConfig(cfg),
		networks:       []Network{},
		channelInfo:    []ChannelInfo{},
		signalHistory:  []SignalDataPoint{},
		roamingHistory: []RoamingEvent{},
	}
	ws.latencySampler = NewLatencySampler(ws.config)
	return ws
}

// GetConfig returns the current live config snapshot.
func (ws *WiFiService) GetConfig() Config {
	return ws.config.Get()
}

// UpdateConfig validates, persists, and applies a new config. The next scan
// loop iteration picks up the new values; ongoing work isn't interrupted.
func (ws *WiFiService) UpdateConfig(cfg Config) error {
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	ws.config.Set(cfg)
	return nil
}

// SetContext sets the Wails runtime context. First call boots the latency
// sampler — its lifecycle is tied to the Wails app context, not to the scan
// loop, so probes keep flowing even when the user pauses scanning.
func (ws *WiFiService) SetContext(ctx context.Context) {
	ws.ctx = ctx
	if ws.latencySampler != nil {
		ws.latencySampler.SetWailsContext(ctx)
		ws.samplerOnce.Do(func() {
			samplerCtx, cancel := context.WithCancel(ctx)
			ws.samplerCancel = cancel
			ws.latencySampler.Start(samplerCtx)
		})
	}
}

// StartScanning begins periodic WiFi scanning
func (ws *WiFiService) StartScanning(iface string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.scanning {
		return fmt.Errorf("scanning already in progress")
	}

	// Inherit from the Wails app context so app shutdown cancels scanning.
	// Fall back to Background when SetContext hasn't been called (tests).
	parent := ws.ctx
	if parent == nil {
		parent = context.Background()
	}
	scanCtx, cancel := context.WithCancel(parent)

	ws.currentInterface = iface
	ws.scanning = true
	ws.cancelFunc = cancel

	go ws.scanLoop(scanCtx, iface)

	return nil
}

// StopScanning stops the periodic scanning
func (ws *WiFiService) StopScanning() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.cancelFunc != nil {
		ws.cancelFunc()
		ws.cancelFunc = nil
	}

	ws.scanning = false
}

// Close stops scanning and releases scanner resources.
func (ws *WiFiService) Close() error {
	ws.StopScanning()
	if ws.samplerCancel != nil {
		ws.samplerCancel()
	}
	if ws.latencySampler != nil {
		ws.latencySampler.Stop()
	}
	if ws.scanner != nil {
		return ws.scanner.Close()
	}
	return nil
}

// GetLatencySummaries returns the current per-target latency snapshot. Used
// by the GetLatency binding so the frontend can populate the panel synchronously
// on tab switch without waiting for the next sampler tick.
func (ws *WiFiService) GetLatencySummaries() []LatencyTargetSummary {
	if ws.latencySampler == nil {
		return []LatencyTargetSummary{}
	}
	return ws.latencySampler.SnapshotSummaries()
}

// scanLoop runs the periodic scanning loop
func (ws *WiFiService) scanLoop(ctx context.Context, iface string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		start := time.Now()
		ws.performScan(ctx, iface)
		elapsed := time.Since(start)
		// Read the interval fresh each loop so a SaveConfig-driven change
		// (e.g. user drops scan_interval from 4 s to 1 s in the Settings
		// tab) takes effect on the next tick rather than requiring a scan
		// stop/start.
		sleepFor := ws.config.Get().ScanInterval() - elapsed
		if sleepFor < 0 {
			sleepFor = 0
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(sleepFor):
		}
	}
}

// performScan executes a single scan operation. ctx is propagated into
// scanWithBackoff so a pending retry-backoff doesn't block app shutdown.
func (ws *WiFiService) performScan(ctx context.Context, iface string) {
	if !ws.scanInFlight.CompareAndSwap(false, true) {
		return
	}
	defer ws.scanInFlight.Store(false)

	aps, err := ws.scanWithBackoff(ctx, iface)
	if err != nil {
		slog.Error("scan failed", "event", "scan_error", "interface", iface, "err", err)
		runtime.EventsEmit(ws.ctx, "scan:error", err.Error())
		return
	}
	for i := range aps {
		NormalizeAccessPoint(&aps[i])
	}

	// Aggregate data (read-only — no shared state touched)
	result := ws.aggregateData(aps, iface)

	// Commit aggregated results + refresh client stats under a single write
	// lock, then snapshot what we're about to emit. The emit happens *after*
	// the lock is released so slow listeners can't block further scans.
	ws.mu.Lock()
	ws.lastScanResult = result
	ws.networks = result.Networks
	ws.channelInfo = result.Channels
	ws.updateClientStatsLocked(iface)
	networksSnapshot := ws.networks
	channelsSnapshot := ws.channelInfo
	clientSnapshot := ws.cloneClientStatsLocked()
	ws.mu.Unlock()

	runtime.EventsEmit(ws.ctx, "networks:updated", networksSnapshot)
	runtime.EventsEmit(ws.ctx, "channels:updated", channelsSnapshot)
	runtime.EventsEmit(ws.ctx, "client:updated", clientSnapshot)
}

// scanWithBackoff retries a failing scan with short exponential backoff so a
// single transient driver hiccup doesn't surface as a scan:error toast.
// The backoff wait is ctx-aware so app shutdown doesn't have to wait for the
// full backoff window before the scan goroutine exits.
func (ws *WiFiService) scanWithBackoff(ctx context.Context, iface string) ([]AccessPoint, error) {
	var lastErr error
	for attempt := 0; attempt <= len(scanBackoffDelays); attempt++ {
		aps, err := ws.scanner.ScanNetworks(iface)
		if err == nil {
			return aps, nil
		}
		lastErr = err
		if attempt == len(scanBackoffDelays) {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(scanBackoffDelays[attempt]):
		}
	}
	return nil, lastErr
}

// aggregateData aggregates access point data into networks and channel info
func (ws *WiFiService) aggregateData(aps []AccessPoint, iface string) *ScanResult {
	networkMap := make(map[string]*Network)
	channelMap := make(map[int]*ChannelInfo)

	for _, ap := range aps {
		// Group by SSID. Hidden APs advertise an empty SSID; we key those
		// per-BSSID so two unrelated hidden networks don't collapse into one
		// row. The on-the-wire SSID stays empty so the UI can render "(hidden)".
		key := ap.SSID
		if key == "" {
			key = "<hidden:" + ap.BSSID + ">"
		}
		if _, exists := networkMap[key]; !exists {
			networkMap[key] = &Network{
				SSID:          ap.SSID,
				AccessPoints:  []AccessPoint{},
				BestSignal:    -100,
				Security:      ap.Security,
				Channel:       ap.Channel,
				IssueMessages: []string{},
			}
		}

		network := networkMap[key]
		network.AccessPoints = append(network.AccessPoints, ap)
		network.APCount = len(network.AccessPoints)

		if ap.Signal > network.BestSignal {
			network.BestSignal = ap.Signal
			network.BestSignalAP = ap.BSSID
			network.Channel = ap.Channel
		}

		// Group by channel
		if _, exists := channelMap[ap.Channel]; !exists {
			channelMap[ap.Channel] = &ChannelInfo{
				Channel:          ap.Channel,
				Frequency:        ap.Frequency,
				Band:             ap.Band,
				NetworkCount:     0,
				Networks:         []string{},
				Utilization:      0,
				CongestionLevel:  "low",
				OverlappingCount: 0,
			}
		}

		channel := channelMap[ap.Channel]
		channel.NetworkCount++
		channel.Networks = append(channel.Networks, ap.SSID)

		// Calculate utilization based on network count
		channel.Utilization = min(100, channel.NetworkCount*15)
		if channel.Utilization > 80 {
			channel.CongestionLevel = "high"
		} else if channel.Utilization > 50 {
			channel.CongestionLevel = "medium"
		}
	}

	// Convert maps to slices
	networks := make([]Network, 0, len(networkMap))
	for _, network := range networkMap {
		// Detect issues
		ws.detectIssues(network)
		networks = append(networks, *network)
	}

	// Sort networks by signal strength (descending)
	sort.Slice(networks, func(i, j int) bool {
		return networks[i].BestSignal > networks[j].BestSignal
	})

	channels := make([]ChannelInfo, 0, len(channelMap))
	for _, channel := range channelMap {
		// Calculate overlapping channels
		channel.OverlappingCount = ws.countOverlappingChannels(channel.Channel, channelMap)
		channels = append(channels, *channel)
	}

	// Sort channels by frequency
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Channel < channels[j].Channel
	})

	return &ScanResult{
		Timestamp:     time.Now(),
		Interface:     iface,
		Networks:      networks,
		Channels:      channels,
		TotalAPs:      len(aps),
		TotalNetworks: len(networks),
	}
}

// detectIssues checks for WiFi configuration issues
func (ws *WiFiService) detectIssues(network *Network) {
	network.HasIssues = false
	network.IssueMessages = []string{}

	// Check for duplicate SSIDs with different security types
	securityTypes := make(map[string]bool)
	for _, ap := range network.AccessPoints {
		if ap.Security != "" {
			securityTypes[ap.Security] = true
		}
	}
	if len(securityTypes) > 1 {
		network.HasIssues = true
		network.IssueMessages = append(network.IssueMessages,
			"Multiple security types detected for same SSID")
	}

	// Check for weak signal
	if network.BestSignal < -80 {
		network.HasIssues = true
		network.IssueMessages = append(network.IssueMessages,
			"Weak signal strength (below -80 dBm)")
	}

	// Check for channel overlap (2.4 GHz only)
	if network.Channel > 0 && network.Channel <= 14 {
		if network.Channel == 1 || network.Channel == 6 || network.Channel == 11 {
			// These are non-overlapping, so they're good
		} else {
			network.HasIssues = true
			network.IssueMessages = append(network.IssueMessages,
				fmt.Sprintf("Channel %d may overlap with adjacent channels", network.Channel))
		}
	}
}

// countOverlappingChannels counts channels that overlap with the given channel
func (ws *WiFiService) countOverlappingChannels(channel int, channelMap map[int]*ChannelInfo) int {
	if channel > 14 {
		return 0 // 5 GHz channels don't overlap in the same way
	}

	count := 0
	for ch := range channelMap {
		if ch <= 14 && ch != channel {
			// 2.4 GHz channels overlap within 5 channel width
			if abs(ch-channel) <= 4 {
				count++
			}
		}
	}
	return count
}

// updateClientStatsLocked updates client connection statistics.
// The caller MUST hold ws.mu.Lock for the duration of the call; the function
// reads and writes ws.clientStats, ws.signalHistory, ws.roamingHistory, and
// ws.lastBSSID without acquiring the mutex itself.
func (ws *WiFiService) updateClientStatsLocked(iface string) {
	linkInfo, err := ws.scanner.GetLinkInfo(iface)
	if err != nil {
		ws.clientStats.Connected = false
		return
	}

	if linkInfo["connected"] == "false" {
		ws.clientStats.Connected = false
		return
	}

	ws.clientStats.Connected = true
	ws.clientStats.Interface = iface
	ws.clientStats.SSID = linkInfo["ssid"]
	ws.clientStats.BSSID = linkInfo["bssid"]
	ws.clientStats.LocalIP = ifaceIPv4(iface)
	ws.clientStats.Gateway = defaultGatewayString()

	if freq, err := strconv.ParseFloat(linkInfo["frequency"], 64); err == nil {
		ws.clientStats.Frequency = freq
		ws.clientStats.Channel = frequencyToChannel(int(freq))
	}

	if signal, err := strconv.Atoi(linkInfo["signal"]); err == nil {
		ws.clientStats.Signal = signal
	}

	stationStats, err := ws.scanner.GetStationStats(iface)
	if err == nil {
		if signalAvg, err := strconv.Atoi(stationStats["signal_avg"]); err == nil {
			ws.clientStats.SignalAvg = signalAvg
		} else {
			ws.clientStats.SignalAvg = ws.clientStats.Signal
		}

		if txBitrate, err := strconv.ParseFloat(stationStats["tx_bitrate"], 64); err == nil {
			ws.clientStats.TxBitrate = txBitrate
		}
		if rxBitrate, err := strconv.ParseFloat(stationStats["rx_bitrate"], 64); err == nil {
			ws.clientStats.RxBitrate = rxBitrate
		}

		wifiStandard, channelWidth, mimoConfig := parseBitrateInfo(stationStats["tx_bitrate_info"])
		ws.clientStats.WiFiStandard = wifiStandard
		ws.clientStats.ChannelWidth, _ = strconv.Atoi(channelWidth)
		ws.clientStats.MIMOConfig = mimoConfig

		if txBytes, err := strconv.ParseUint(stationStats["tx_bytes"], 10, 64); err == nil {
			ws.clientStats.TxBytes = txBytes
		}
		if rxBytes, err := strconv.ParseUint(stationStats["rx_bytes"], 10, 64); err == nil {
			ws.clientStats.RxBytes = rxBytes
		}
		if txPackets, err := strconv.ParseUint(stationStats["tx_packets"], 10, 64); err == nil {
			ws.clientStats.TxPackets = txPackets
		}
		if rxPackets, err := strconv.ParseUint(stationStats["rx_packets"], 10, 64); err == nil {
			ws.clientStats.RxPackets = rxPackets
		}

		if txRetries, err := strconv.ParseUint(stationStats["tx_retries"], 10, 64); err == nil {
			ws.clientStats.TxRetries = txRetries
		}
		if txFailed, err := strconv.ParseUint(stationStats["tx_failed"], 10, 64); err == nil {
			ws.clientStats.TxFailed = txFailed
		}

		if ws.clientStats.TxPackets > 0 {
			ws.clientStats.RetryRate = (float64(ws.clientStats.TxRetries) / float64(ws.clientStats.TxPackets)) * 100
		}

		if connectedTime, err := strconv.Atoi(stationStats["connected_time"]); err == nil {
			ws.clientStats.ConnectedTime = connectedTime
		}

		if lastAckSignal, err := strconv.Atoi(stationStats["last_ack_signal"]); err == nil {
			ws.clientStats.LastAckSignal = lastAckSignal
		}
	}

	ws.updateSignalHistoryLocked()
	NormalizeClientStats(&ws.clientStats)
}

// updateSignalHistoryLocked appends the current signal reading and detects
// roaming events. Caller must hold ws.mu.Lock.
//
// This function only runs on scan ticks where the client is connected —
// updateClientStatsLocked early-returns on a disconnected interface. That
// means ws.lastBSSIDSeenAt stays frozen across disconnect gaps, so the
// DurationMs computation below naturally covers "old BSSID last observed"
// through "new BSSID first observed", including any mid-roam outage.
//
// Duration resolution is capped by the scan interval. With the default 4 s
// cadence a 200 ms real roam and a 1500 ms real roam will both report
// somewhere in the 0–4000 ms range depending on tick phase. Tighten
// `scan_interval_seconds` in config for finer granularity.
func (ws *WiFiService) updateSignalHistoryLocked() {
	now := time.Now()
	dataPoint := SignalDataPoint{
		Timestamp: now,
		Signal:    ws.clientStats.Signal,
		BSSID:     ws.clientStats.BSSID,
	}

	// appendCapped allocates a fresh backing array when truncating, so prior
	// slice headers handed out via GetClientStats remain valid and immutable.
	cfg := ws.config.Get()
	ws.signalHistory = appendCapped(ws.signalHistory, dataPoint, cfg.SignalHistorySize())

	// Detect roaming events
	if ws.lastBSSID != "" && ws.lastBSSID != ws.clientStats.BSSID {
		// Find previous signal
		var prevSignal int
		for i := len(ws.signalHistory) - 2; i >= 0; i-- {
			if ws.signalHistory[i].BSSID == ws.lastBSSID {
				prevSignal = ws.signalHistory[i].Signal
				break
			}
		}

		var durationMs int64
		if !ws.lastBSSIDSeenAt.IsZero() {
			durationMs = now.Sub(ws.lastBSSIDSeenAt).Milliseconds()
		}

		roamingEvent := RoamingEvent{
			Timestamp:      now,
			PreviousBSSID:  ws.lastBSSID,
			NewBSSID:       ws.clientStats.BSSID,
			PreviousSignal: prevSignal,
			NewSignal:      ws.clientStats.Signal,
			DurationMs:     durationMs,
		}
		ws.roamingHistory = appendCapped(ws.roamingHistory, roamingEvent, cfg.RoamingHistorySize)

		runtime.EventsEmit(ws.ctx, "roaming:detected", roamingEvent)
	}

	ws.lastBSSID = ws.clientStats.BSSID
	ws.lastBSSIDSeenAt = now
	// Note: SignalHistory/RoamingHistory on ClientStats are populated lazily
	// via cloneClientStatsLocked when a caller asks for a snapshot; we never
	// hand out the live backing slices anymore.
}

// cloneClientStatsLocked returns a deep copy of ws.clientStats with fresh
// SignalHistory/RoamingHistory slices. Caller must hold the lock.
func (ws *WiFiService) cloneClientStatsLocked() ClientStats {
	out := ws.clientStats
	if n := len(ws.signalHistory); n > 0 {
		out.SignalHistory = make([]SignalDataPoint, n)
		copy(out.SignalHistory, ws.signalHistory)
	} else {
		out.SignalHistory = nil
	}
	if n := len(ws.roamingHistory); n > 0 {
		out.RoamingHistory = make([]RoamingEvent, n)
		copy(out.RoamingHistory, ws.roamingHistory)
	} else {
		out.RoamingHistory = nil
	}
	return out
}

// GetNetworks returns the list of discovered WiFi networks
func (ws *WiFiService) GetNetworks() []Network {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.networks
}

// GetClientStats returns a snapshot of current client connection statistics.
// The returned value owns its SignalHistory/RoamingHistory slices — callers
// may inspect or mutate them without affecting the service's live state.
func (ws *WiFiService) GetClientStats() ClientStats {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.cloneClientStatsLocked()
}

// GetChannelAnalysis returns channel utilization information
func (ws *WiFiService) GetChannelAnalysis() []ChannelInfo {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.channelInfo
}

// IsScanning returns whether scanning is currently active
func (ws *WiFiService) IsScanning() bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.scanning
}

// AnalyzeRoamingQuality analyzes roaming quality based on signal history.
func (ws *WiFiService) AnalyzeRoamingQuality() RoamingQualityReport {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	if len(ws.roamingHistory) == 0 {
		return RoamingQualityReport{
			RoamingAdvice: "No roaming data available yet. Connect to a network with multiple APs to see roaming analysis.",
		}
	}

	totalRoams := len(ws.roamingHistory)
	goodRoams := 0
	badRoams := 0
	totalSignalChange := 0

	// Duration aggregates. We skip DurationMs==0 entries when computing the
	// average so a single "unbounded" first roam doesn't pull the mean down.
	// Max is similarly skipped. SlowRoamCount uses the 2000 ms threshold as
	// per the plan — anything over that is the "auth issues" tier, not just
	// "a bit slower than 802.11r would manage".
	var totalDurationMs int64
	var maxDurationMs int64
	durationSamples := 0
	slowRoamCount := 0

	for _, event := range ws.roamingHistory {
		signalChange := event.NewSignal - event.PreviousSignal
		totalSignalChange += signalChange
		if signalChange >= 0 {
			goodRoams++
		} else {
			badRoams++
		}
		if event.DurationMs > 0 {
			totalDurationMs += event.DurationMs
			durationSamples++
			if event.DurationMs > maxDurationMs {
				maxDurationMs = event.DurationMs
			}
			if event.DurationMs >= 2000 {
				slowRoamCount++
			}
		}
	}

	avgSignalChange := 0
	if totalRoams > 0 {
		avgSignalChange = totalSignalChange / totalRoams
	}

	var avgDurationMs int64
	if durationSamples > 0 {
		avgDurationMs = totalDurationMs / int64(durationSamples)
	}

	excessiveRoaming := totalRoams > 10 && len(ws.signalHistory) > 0 &&
		float64(totalRoams)/float64(len(ws.signalHistory))*100 > 5

	stickyClient := false
	if ws.clientStats.Connected && ws.clientStats.Signal < -75 && totalRoams == 0 {
		for _, network := range ws.networks {
			if network.SSID == ws.clientStats.SSID && len(network.AccessPoints) > 1 {
				for _, ap := range network.AccessPoints {
					if ap.BSSID != ws.clientStats.BSSID && ap.Signal > ws.clientStats.Signal+10 {
						stickyClient = true
						break
					}
				}
			}
		}
	}

	var timeSinceLastRoam string
	lastRoam := ws.roamingHistory[len(ws.roamingHistory)-1]
	timeSince := time.Since(lastRoam.Timestamp)
	switch {
	case timeSince < time.Minute:
		timeSinceLastRoam = fmt.Sprintf("%ds ago", int(timeSince.Seconds()))
	case timeSince < time.Hour:
		timeSinceLastRoam = fmt.Sprintf("%dm ago", int(timeSince.Minutes()))
	default:
		timeSinceLastRoam = fmt.Sprintf("%dh %dm ago", int(timeSince.Hours()), int(timeSince.Minutes())%60)
	}

	var advice string
	switch {
	case slowRoamCount > 0 && slowRoamCount*2 >= durationSamples && durationSamples > 0:
		// Majority of roams over the 2 s "auth issues" threshold — promote
		// this over the generic excessive/sticky messaging because a slow
		// roam is the diagnostic pattern that points at 802.1X / radius /
		// AKM-mismatch problems the tech can actually fix.
		advice = "Multiple roams exceeded 2 s. Common causes: 802.1X / RADIUS latency, missing 802.11r/k/v support, or AKM mismatches between APs. Check the authentication path first."
	case excessiveRoaming:
		advice = "Your device is roaming excessively. This may indicate overlapping AP coverage or unstable connections. Consider adjusting AP placement or roaming aggressiveness settings."
	case stickyClient:
		advice = "Your device appears to be a 'sticky client' - it's staying connected to a weak AP when better options are available. Consider enabling 802.11k/v/r on your network or adjusting client roaming settings."
	case avgSignalChange > 5:
		advice = "Roaming is working well! Your device is successfully moving to stronger access points."
	case avgSignalChange < -5:
		advice = "Roaming decisions may not be optimal. Your device sometimes roams to weaker APs. This could indicate AP coverage overlap issues."
	default:
		advice = "Roaming behavior appears normal. Signal quality is maintained during transitions."
	}

	return RoamingQualityReport{
		TotalRoams:        totalRoams,
		GoodRoams:         goodRoams,
		BadRoams:          badRoams,
		AvgSignalChange:   avgSignalChange,
		ExcessiveRoaming:  excessiveRoaming,
		StickyClient:      stickyClient,
		TimeSinceLastRoam: timeSinceLastRoam,
		RoamingAdvice:     advice,
		AvgRoamDurationMs: avgDurationMs,
		MaxRoamDurationMs: maxDurationMs,
		SlowRoamCount:     slowRoamCount,
	}
}

// GetAPPlacementRecommendations provides recommendations for AP placement
func (ws *WiFiService) GetAPPlacementRecommendations() []string {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	var recommendations []string

	// Check channel congestion
	for _, channel := range ws.channelInfo {
		if channel.CongestionLevel == "high" {
			recommendations = append(recommendations,
				fmt.Sprintf("Consider switching from channel %d to a less congested channel",
					channel.Channel))
		}
	}

	// Check network coverage
	for _, network := range ws.networks {
		if network.BestSignal < -70 && len(network.AccessPoints) == 1 {
			recommendations = append(recommendations,
				fmt.Sprintf("Network '%s' has weak signal coverage. Consider adding additional access points",
					network.SSID))
		}
	}

	// Check for overlapping channels
	if ws.hasOverlappingChannels() {
		recommendations = append(recommendations,
			"Detected overlapping 2.4GHz channels. Use channels 1, 6, or 11 for optimal performance")
	}

	// General recommendations
	if len(recommendations) == 0 {
		recommendations = append(recommendations,
			"No immediate issues detected. Current configuration appears optimal")
	}

	return recommendations
}

// hasOverlappingChannels checks for overlapping 2.4GHz channels
func (ws *WiFiService) hasOverlappingChannels() bool {
	channels := make(map[int]bool)
	for _, channel := range ws.channelInfo {
		if channel.Channel > 0 && channel.Channel <= 14 {
			channels[channel.Channel] = true
		}
	}

	// Check if we're using non-standard channels (1, 6, 11 are standard)
	for ch := range channels {
		if ch != 1 && ch != 6 && ch != 11 {
			return true
		}
	}

	return false
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ifaceIPv4 returns the first non-loopback IPv4 address bound to the named
// interface, or "" if the interface is missing or has no IPv4 lease yet.
func ifaceIPv4(name string) string {
	if name == "" {
		return ""
	}
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP == nil {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil || ip.IsLoopback() {
			continue
		}
		return ip.String()
	}
	return ""
}

// defaultGatewayString wraps defaultGateway() to return a string suitable for
// the JSON payload, swallowing errors (e.g. no default route on a freshly
// associated interface still negotiating DHCP).
func defaultGatewayString() string {
	ip, err := defaultGateway()
	if err != nil || ip == nil {
		return ""
	}
	return ip.String()
}
