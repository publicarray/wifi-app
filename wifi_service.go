package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	maxSignalHistory = 600 // Keep 10 minutes at 1 second intervals
	scanInterval     = 3 * time.Second
)

// WiFiService manages WiFi scanning and data aggregation
type WiFiService struct {
	scanner          *WiFiScanner
	ctx              context.Context
	mu               sync.RWMutex
	scanning         bool
	cancelFunc       context.CancelFunc
	currentInterface string

	// Aggregated data
	networks       []Network
	channelInfo    []ChannelInfo
	clientStats    ClientStats
	lastScanResult *ScanResult

	// Signal history tracking
	signalHistory  []SignalDataPoint
	roamingHistory []RoamingEvent
	lastBSSID      string
}

// NewWiFiService creates a new WiFi service
func NewWiFiService() *WiFiService {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	cacheFile := filepath.Join(cacheDir, "wifi-app", "oui.txt")

	os.MkdirAll(filepath.Dir(cacheFile), 0755)

	return &WiFiService{
		scanner:        NewWiFiScanner(cacheFile),
		networks:       []Network{},
		channelInfo:    []ChannelInfo{},
		signalHistory:  []SignalDataPoint{},
		roamingHistory: []RoamingEvent{},
	}
}

// SetContext sets the Wails runtime context
func (ws *WiFiService) SetContext(ctx context.Context) {
	ws.ctx = ctx
}

// StartScanning begins periodic WiFi scanning
func (ws *WiFiService) StartScanning(iface string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.scanning {
		return fmt.Errorf("scanning already in progress")
	}

	ws.currentInterface = iface
	ws.scanning = true

	scanCtx, cancel := context.WithCancel(context.Background())
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

// scanLoop runs the periodic scanning loop
func (ws *WiFiService) scanLoop(ctx context.Context, iface string) {
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ws.performScan(iface)
		}
	}
}

// performScan executes a single scan operation
func (ws *WiFiService) performScan(iface string) {
	aps, err := ws.scanner.ScanNetworks(iface)
	if err != nil {
		runtime.EventsEmit(ws.ctx, "scan:error", err.Error())
		return
	}

	// Aggregate data
	result := ws.aggregateData(aps, iface)

	ws.mu.Lock()
	ws.lastScanResult = result
	ws.networks = result.Networks
	ws.channelInfo = result.Channels
	ws.mu.Unlock()

	ws.updateClientStats(iface)

	runtime.EventsEmit(ws.ctx, "networks:updated", ws.networks)
	runtime.EventsEmit(ws.ctx, "client:updated", ws.clientStats)
}

// aggregateData aggregates access point data into networks and channel info
func (ws *WiFiService) aggregateData(aps []AccessPoint, iface string) *ScanResult {
	networkMap := make(map[string]*Network)
	channelMap := make(map[int]*ChannelInfo)

	for _, ap := range aps {
		// Group by SSID
		if _, exists := networkMap[ap.SSID]; !exists {
			networkMap[ap.SSID] = &Network{
				SSID:          ap.SSID,
				AccessPoints:  []AccessPoint{},
				BestSignal:    -100,
				Security:      ap.Security,
				Channel:       ap.Channel,
				IssueMessages: []string{},
			}
		}

		network := networkMap[ap.SSID]
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

// updateClientStats updates client connection statistics
func (ws *WiFiService) updateClientStats(iface string) {
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

	ws.updateSignalHistory()
}

// updateSignalHistory updates the signal history and detects roaming events
func (ws *WiFiService) updateSignalHistory() {
	dataPoint := SignalDataPoint{
		Timestamp: time.Now(),
		Signal:    ws.clientStats.Signal,
		BSSID:     ws.clientStats.BSSID,
	}

	ws.signalHistory = append(ws.signalHistory, dataPoint)

	// Keep only recent history
	if len(ws.signalHistory) > maxSignalHistory {
		ws.signalHistory = ws.signalHistory[len(ws.signalHistory)-maxSignalHistory:]
	}

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

		roamingEvent := RoamingEvent{
			Timestamp:      time.Now(),
			PreviousBSSID:  ws.lastBSSID,
			NewBSSID:       ws.clientStats.BSSID,
			PreviousSignal: prevSignal,
			NewSignal:      ws.clientStats.Signal,
		}
		ws.roamingHistory = append(ws.roamingHistory, roamingEvent)

		runtime.EventsEmit(ws.ctx, "roaming:detected", roamingEvent)
	}

	ws.lastBSSID = ws.clientStats.BSSID
	ws.clientStats.SignalHistory = ws.signalHistory
	ws.clientStats.RoamingHistory = ws.roamingHistory
}

// GetNetworks returns the list of discovered WiFi networks
func (ws *WiFiService) GetNetworks() []Network {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.networks
}

// GetClientStats returns current client connection statistics
func (ws *WiFiService) GetClientStats() ClientStats {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.clientStats
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

// AnalyzeRoamingQuality analyzes roaming quality based on signal history
func (ws *WiFiService) AnalyzeRoamingQuality() map[string]interface{} {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	result := make(map[string]interface{})

	if len(ws.roamingHistory) == 0 {
		result["roaming_events"] = 0
		result["status"] = "no_data"
		return result
	}

	result["roaming_events"] = len(ws.roamingHistory)
	result["status"] = "active"

	// Calculate average signal change
	totalSignalChange := 0
	for _, event := range ws.roamingHistory {
		totalSignalChange += event.NewSignal - event.PreviousSignal
	}
	avgSignalChange := 0
	if len(ws.roamingHistory) > 0 {
		avgSignalChange = totalSignalChange / len(ws.roamingHistory)
	}
	result["avg_signal_change"] = avgSignalChange

	// Analyze roaming quality
	if avgSignalChange > 10 {
		result["quality"] = "excellent"
		result["description"] = "Roaming is improving signal quality significantly"
	} else if avgSignalChange > 0 {
		result["quality"] = "good"
		result["description"] = "Roaming is improving signal quality"
	} else if avgSignalChange > -10 {
		result["quality"] = "fair"
		result["description"] = "Roaming maintains similar signal quality"
	} else {
		result["quality"] = "poor"
		result["description"] = "Roaming is degrading signal quality"
	}

	return result
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
