package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type darwinScanner struct {
	currentInterface  string
	ouiLookup         *OUILookup
	airportPath       string
	hasAirport        bool
	hasWdutil         bool
	hasSystemProfiler bool
	hasNetworksetup   bool
	hasCoreWLAN       bool
	airportParser     *airportParser
	systemParser      *systemProfilerParser
	baselineStats     map[string]trafficStats
	connectionStart   map[string]time.Time
	mu                sync.Mutex
	// macHelperPath is the absolute path to the bundled wifi-app-mac-helper
	// binary, set at construction by defaultMacHelperPath(). Empty when no
	// helper ships alongside the main executable, in which case scans fall
	// back to plain CoreWLAN data without the IE-derived advanced fields.
	macHelperPath string
}

type trafficStats struct {
	inOctets   uint64
	outOctets  uint64
	inPackets  uint64
	outPackets uint64
	timestamp  time.Time
}

func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()
	airportPath := findAirportPath()
	_, wdutilErr := exec.LookPath("wdutil")
	_, profilerErr := exec.LookPath("system_profiler")
	_, networksetupErr := exec.LookPath("networksetup")
	setCoreWLANLookup(ouiLookup)

	scanner := &darwinScanner{
		ouiLookup:         ouiLookup,
		airportPath:       airportPath,
		hasAirport:        airportPath != "",
		hasWdutil:         wdutilErr == nil,
		hasSystemProfiler: profilerErr == nil,
		hasNetworksetup:   networksetupErr == nil,
		hasCoreWLAN:       coreWLANAvailable(),
		airportParser:     &airportParser{ouiLookup: ouiLookup},
		systemParser:      &systemProfilerParser{ouiLookup: ouiLookup},
		baselineStats:     make(map[string]trafficStats),
		connectionStart:   make(map[string]time.Time),
		macHelperPath:     defaultMacHelperPath(),
	}
	// Trigger the macOS Location Services prompt at startup. CoreWLAN scans
	// return blank SSID/BSSID without a grant on macOS 14+, so we ask early
	// rather than waiting for the first scan tick.
	if scanner.hasCoreWLAN {
		coreWLANPrimeLocationAuthorization()
	}
	return scanner
}

func (s *darwinScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
	if s.hasCoreWLAN {
		aps, err := coreWLANScanNetworks(iface)
		if err == nil && len(aps) > 0 {
			s.augmentWithHelper(iface, aps)
			return aps, nil
		}
		// Surface a Location Services denial directly: the airport CLI and
		// system_profiler are subject to the same gate on macOS 14+, so the
		// fallback would just return an empty list without explanation.
		if errors.Is(err, ErrLocationDenied) {
			return nil, err
		}
	}
	if s.hasAirport {
		cmd := exec.Command(s.airportPath, "-s", "-x")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to scan networks with airport: %w (output: %s)", err, string(output))
		}
		return s.airportParser.ParseScan(output)
	}

	if s.hasSystemProfiler {
		cmd := exec.Command("system_profiler", "-json", "SPAirPortDataType")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to scan networks with system_profiler: %w (output: %s)", err, string(output))
		}
		return s.systemParser.ParseScan(output)
	}

	return nil, fmt.Errorf("no macOS WiFi scan command available (airport/system_profiler missing)")
}

func (s *darwinScanner) GetInterfaces() ([]string, error) {
	// Prefer CoreWLAN when available — it returns just the WiFi interfaces
	// and does not require shelling out. Fall back to networksetup if the
	// cgo build is not in use or CoreWLAN returned nothing.
	if s.hasCoreWLAN {
		if names, err := coreWLANInterfaces(); err == nil && len(names) > 0 {
			return names, nil
		}
	}

	cmd := exec.Command("/usr/sbin/networksetup", "-listallhardwareports")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var interfaces []string
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "Hardware Port:") &&
			strings.Contains(line, "Wi-Fi") {
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimSpace(lines[j])
				if strings.HasPrefix(next, "Device:") {
					interfaces = append(interfaces, strings.TrimSpace(strings.TrimPrefix(next, "Device:")))
					break
				}
				if next == "" {
					break
				}
			}
		}
	}

	if len(interfaces) > 0 {
		return interfaces, nil
	}

	return []string{}, fmt.Errorf("no WiFi interfaces found")
}

func (s *darwinScanner) GetConnectionInfo(iface string) (ConnectionInfo, error) {
	if s.hasCoreWLAN {
		info, err := coreWLANConnectionInfo(iface)
		if err == nil {
			return info, nil
		}
	}
	if s.hasAirport {
		cmd := exec.Command(s.airportPath, "-I")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return ConnectionInfo{}, fmt.Errorf("failed to get connection info with airport: %w", err)
		}
		return parseAirportConnectionInfo(output), nil
	}

	if s.hasWdutil {
		cmd := exec.Command("wdutil", "info")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return ConnectionInfo{}, fmt.Errorf("failed to get connection info with wdutil: %w (output: %s)", err, string(output))
		}
		return parseWdutilConnectionInfo(output), nil
	}

	if s.hasSystemProfiler {
		cmd := exec.Command("system_profiler", "-json", "SPAirPortDataType")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return ConnectionInfo{}, fmt.Errorf("failed to get connection info with system_profiler: %w (output: %s)", err, string(output))
		}
		return parseSystemProfilerConnectionInfo(output), nil
	}

	if s.hasNetworksetup {
		return parseNetworksetupConnectionInfo(iface), nil
	}

	return ConnectionInfo{}, fmt.Errorf("no macOS WiFi info command available (airport/wdutil/system_profiler/networksetup missing)")
}

func (s *darwinScanner) GetStationStats(iface string) (map[string]string, error) {
	if s.hasCoreWLAN {
		stats, err := coreWLANStationInfo(iface)
		if err == nil {
			return stats, nil
		}
	}
	if s.hasAirport {
		cmd := exec.Command(s.airportPath, "-I")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]string{"connected": "false"}, fmt.Errorf("failed to get station stats with airport: %w", err)
		}
		return s.airportParser.ParseStation(output)
	}

	if s.hasWdutil {
		cmd := exec.Command("wdutil", "info")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]string{"connected": "false"}, fmt.Errorf("failed to get station stats with wdutil: %w (output: %s)", err, string(output))
		}
		return parseWdutilStationInfo(output), nil
	}

	if s.hasSystemProfiler {
		cmd := exec.Command("system_profiler", "-json", "SPAirPortDataType")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]string{"connected": "false"}, fmt.Errorf("failed to get station stats with system_profiler: %w (output: %s)", err, string(output))
		}
		return parseSystemProfilerStationInfo(output), nil
	}

	if s.hasNetworksetup {
		info := parseNetworksetupConnectionInfo(iface)
		stats := map[string]string{"connected": strconv.FormatBool(info.Connected)}
		if info.BSSID != "" {
			stats["bssid"] = info.BSSID
		}
		if info.Signal != 0 {
			stats["signal"] = strconv.Itoa(info.Signal)
			stats["signal_avg"] = strconv.Itoa(info.Signal)
		}
		return stats, nil
	}

	return map[string]string{"connected": "false"}, fmt.Errorf("no macOS WiFi info command available (airport/wdutil/system_profiler/networksetup missing)")
}

func (s *darwinScanner) GetLinkInfo(iface string) (map[string]string, error) {
	if s.hasCoreWLAN {
		link, err := coreWLANLinkInfo(iface)
		if err == nil {
			return link, nil
		}
	}
	if s.hasAirport {
		cmd := exec.Command(s.airportPath, "-I")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]string{"connected": "false"}, fmt.Errorf("failed to get link info with airport: %w", err)
		}
		result, _ := s.airportParser.ParseLink(output)
		return s.enrichWithTrafficStats(iface, result), nil
	}

	if s.hasWdutil {
		cmd := exec.Command("wdutil", "info")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]string{"connected": "false"}, fmt.Errorf("failed to get link info with wdutil: %w (output: %s)", err, string(output))
		}
		result := parseWdutilLinkInfo(output)
		return s.enrichWithTrafficStats(iface, result), nil
	}

	if s.hasSystemProfiler {
		cmd := exec.Command("system_profiler", "-json", "SPAirPortDataType")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]string{"connected": "false"}, fmt.Errorf("failed to get link info with system_profiler: %w", err)
		}
		result := parseSystemProfilerLinkInfo(output)
		return s.enrichWithTrafficStats(iface, result), nil
	}

	if s.hasNetworksetup {
		info := parseNetworksetupConnectionInfo(iface)
		link := map[string]string{"connected": strconv.FormatBool(info.Connected)}
		if info.SSID != "" {
			link["ssid"] = info.SSID
		}
		if info.BSSID != "" {
			link["bssid"] = info.BSSID
		}
		return s.enrichWithTrafficStats(iface, link), nil
	}

	return map[string]string{"connected": "false"}, fmt.Errorf("no macOS WiFi info command available (airport/wdutil/system_profiler/networksetup missing)")
}

func (s *darwinScanner) enrichWithTrafficStats(iface string, result map[string]string) map[string]string {
	if result["connected"] != "true" {
		s.mu.Lock()
		delete(s.baselineStats, iface)
		delete(s.connectionStart, iface)
		s.mu.Unlock()
		return result
	}

	traffic, err := s.getTrafficStats(iface)
	if err != nil || traffic["rx_bytes"] == "0" {
		result["rx_bytes"] = "0"
		result["tx_bytes"] = "0"
		result["rx_packets"] = "0"
		result["tx_packets"] = "0"
		result["tx_retries"] = "0"
		result["tx_failed"] = "0"
		result["connected_time"] = "0"
		result["retry_rate"] = "0.00"
		return result
	}

	rxBytesNow, _ := strconv.ParseUint(traffic["rx_bytes"], 10, 64)
	txBytesNow, _ := strconv.ParseUint(traffic["tx_bytes"], 10, 64)
	rxPktsNow, _ := strconv.ParseUint(traffic["rx_packets"], 10, 64)
	txPktsNow, _ := strconv.ParseUint(traffic["tx_packets"], 10, 64)

	s.mu.Lock()
	baseline, hasBaseline := s.baselineStats[iface]
	connStart, hasConnStart := s.connectionStart[iface]

	if !hasBaseline {
		baseline = trafficStats{
			inOctets:   rxBytesNow,
			outOctets:  txBytesNow,
			inPackets:  rxPktsNow,
			outPackets: txPktsNow,
			timestamp:  time.Now(),
		}
		s.baselineStats[iface] = baseline
	}
	if !hasConnStart {
		connStart = time.Now()
		s.connectionStart[iface] = connStart
	}
	s.mu.Unlock()

	rxDelta := saturatingSubUint64(rxBytesNow, baseline.inOctets)
	txDelta := saturatingSubUint64(txBytesNow, baseline.outOctets)
	rxPktsDelta := saturatingSubUint64(rxPktsNow, baseline.inPackets)
	txPktsDelta := saturatingSubUint64(txPktsNow, baseline.outPackets)
	connectedTime := int(time.Since(connStart).Seconds())

	result["rx_bytes"] = strconv.FormatUint(rxDelta, 10)
	result["tx_bytes"] = strconv.FormatUint(txDelta, 10)
	result["rx_packets"] = strconv.FormatUint(rxPktsDelta, 10)
	result["tx_packets"] = strconv.FormatUint(txPktsDelta, 10)
	result["connected_time"] = strconv.Itoa(connectedTime)

	retries, _ := strconv.ParseUint(traffic["tx_retries"], 10, 64)
	result["tx_retries"] = strconv.FormatUint(retries, 10)

	failed, _ := strconv.ParseUint(traffic["tx_failed"], 10, 64)
	result["tx_failed"] = strconv.FormatUint(failed, 10)

	retryRate := 0.0
	if txPktsDelta > 0 {
		retryRate = float64(retries) / float64(txPktsDelta) * 100.0
		if retryRate > 100.0 {
			retryRate = 100.0
		}
	}
	result["retry_rate"] = fmt.Sprintf("%.2f", retryRate)

	return result
}

func (s *darwinScanner) Close() error {
	return nil
}

func (s *darwinScanner) getTrafficStats(iface string) (map[string]string, error) {
	result := make(map[string]string)

	cmd := exec.Command("netstat", "-i", "-b")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return result, err
	}

	var ifaceData struct {
		ibytes, obytes    uint64
		ipkts, opkts      uint64
		collisions, drops uint64
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "Name") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		// Use exact interface name - already validated as WiFi from GetInterfaces()
		if fields[0] != iface {
			continue
		}
		if ib, err := strconv.ParseUint(fields[4], 10, 64); err == nil {
			ifaceData.ibytes = ib
		}
		if ob, err := strconv.ParseUint(fields[6], 10, 64); err == nil {
			ifaceData.obytes = ob
		}
		if ip, err := strconv.ParseUint(fields[3], 10, 64); err == nil {
			ifaceData.ipkts = ip
		}
		if op, err := strconv.ParseUint(fields[5], 10, 64); err == nil {
			ifaceData.opkts = op
		}
		if len(fields) >= 9 {
			if coll, err := strconv.ParseUint(fields[8], 10, 64); err == nil {
				ifaceData.collisions = coll
			}
		}
		if len(fields) >= 10 {
			if drops, err := strconv.ParseUint(fields[9], 10, 64); err == nil {
				ifaceData.drops = drops
			}
		}
		break
	}

	result["rx_bytes"] = strconv.FormatUint(ifaceData.ibytes, 10)
	result["tx_bytes"] = strconv.FormatUint(ifaceData.obytes, 10)
	result["rx_packets"] = strconv.FormatUint(ifaceData.ipkts, 10)
	result["tx_packets"] = strconv.FormatUint(ifaceData.opkts, 10)
	result["tx_retries"] = strconv.FormatUint(ifaceData.collisions, 10)
	result["tx_failed"] = strconv.FormatUint(ifaceData.drops, 10)

	return result, nil
}

func saturatingSubUint64(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}

func (s *darwinScanner) updateTrafficBaseline(iface string, traffic map[string]string) {
	if traffic == nil || traffic["rx_bytes"] == "0" {
		return
	}

	rxBytes, _ := strconv.ParseUint(traffic["rx_bytes"], 10, 64)
	txBytes, _ := strconv.ParseUint(traffic["tx_bytes"], 10, 64)
	rxPkts, _ := strconv.ParseUint(traffic["rx_packets"], 10, 64)
	txPkts, _ := strconv.ParseUint(traffic["tx_packets"], 10, 64)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.baselineStats[iface] = trafficStats{
		inOctets:   rxBytes,
		outOctets:  txBytes,
		inPackets:  rxPkts,
		outPackets: txPkts,
		timestamp:  time.Now(),
	}

	if _, ok := s.connectionStart[iface]; !ok {
		s.connectionStart[iface] = time.Now()
	}
}

func findAirportPath() string {
	candidates := []string{
		"/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport",
		"/System/Library/PrivateFrameworks/Apple80211.framework/Resources/airport",
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	if path, err := exec.LookPath("airport"); err == nil {
		return path
	}
	return ""
}

func parseAirportConnectionInfo(output []byte) ConnectionInfo {
	lines := strings.Split(string(output), "\n")

	stateRegex := regexp.MustCompile(`\s+state:\s+(\S+)`)
	ssidRegex := regexp.MustCompile(`\s+SSID:\s+(.+)`)
	bssidRegex := regexp.MustCompile(`\s+BSSID:\s+([0-9a-f:]+)`)
	channelRegex := regexp.MustCompile(`\s+channel:\s+(\d+)(?:,\s*(\d+))?`)
	rssiRegex := regexp.MustCompile(`\s+agrCtlRSSI:\s+(-?\d+)`)
	rxMcsRegex := regexp.MustCompile(`\s+lastRxRate:\s+(\d+)`)
	txMcsRegex := regexp.MustCompile(`\s+lastTxRate:\s+(\d+)`)
	phyTypeRegex := regexp.MustCompile(`\s+phy mode:\s+(\S+)`)

	connInfo := ConnectionInfo{}

	for _, line := range lines {
		if matches := stateRegex.FindStringSubmatch(line); matches != nil {
			connInfo.Connected = matches[1] == "running"
		}
		if matches := ssidRegex.FindStringSubmatch(line); matches != nil {
			connInfo.SSID = matches[1]
		}
		if matches := bssidRegex.FindStringSubmatch(line); matches != nil {
			connInfo.BSSID = matches[1]
		}
		if matches := channelRegex.FindStringSubmatch(line); matches != nil {
			if ch, err := strconv.Atoi(matches[1]); err == nil {
				connInfo.Channel = ch
				connInfo.Frequency = channelToFrequency(ch)
			}
			if len(matches) > 2 && matches[2] != "" {
				if width, err := strconv.Atoi(matches[2]); err == nil {
					connInfo.ChannelWidth = width
				}
			}
		}
		if matches := rssiRegex.FindStringSubmatch(line); matches != nil {
			if rssi, err := strconv.Atoi(matches[1]); err == nil {
				connInfo.Signal = rssi
				connInfo.SignalAvg = rssi
			}
		}
		if matches := rxMcsRegex.FindStringSubmatch(line); matches != nil {
			if rate, err := strconv.ParseFloat(matches[1], 64); err == nil {
				connInfo.RxBitrate = rate
			}
		}
		if matches := txMcsRegex.FindStringSubmatch(line); matches != nil {
			if rate, err := strconv.ParseFloat(matches[1], 64); err == nil {
				connInfo.TxBitrate = rate
			}
		}
		if matches := phyTypeRegex.FindStringSubmatch(line); matches != nil {
			connInfo.WiFiStandard = normalizeWiFiStandard(matches[1])
		}
	}

	if connInfo.ChannelWidth == 0 {
		connInfo.ChannelWidth = 20
	}
	if connInfo.MIMOConfig == "" {
		connInfo.MIMOConfig = "1x1"
	}

	return connInfo
}

func parseWdutilConnectionInfo(output []byte) ConnectionInfo {
	values := parseWdutilKeyValues(output)
	connInfo := ConnectionInfo{}
	connInfo.SSID = firstValue(values, "ssid")
	connInfo.BSSID = extractBSSID(firstValue(values, "bssid", "ap bssid", "current bssid"))
	connInfo.Connected = isConnectedState(firstValue(values, "state", "status", "link status")) || connInfo.SSID != "" || connInfo.BSSID != ""

	if channel := firstValue(values, "channel", "primary channel"); channel != "" {
		if ch := parseFirstInt(channel); ch != 0 {
			connInfo.Channel = ch
			connInfo.Frequency = channelToFrequency(ch)
		}
		if width := parseChannelWidth(channel); width != 0 {
			connInfo.ChannelWidth = width
		}
		if connInfo.Channel == 0 {
			if ch, width := parseWdutilChannel(channel); ch != 0 {
				connInfo.Channel = ch
				connInfo.Frequency = channelToFrequency(ch)
				if width != 0 {
					connInfo.ChannelWidth = width
				}
			}
		}
	}
	if width := parseChannelWidth(firstValue(values, "channel width", "chan width")); width != 0 {
		connInfo.ChannelWidth = width
	}
	if rssi := parseFirstInt(firstValue(values, "rssi", "signal", "agrctlrssi")); rssi != 0 {
		connInfo.Signal = rssi
		connInfo.SignalAvg = rssi
	}
	if rx := parseFirstFloat(firstValue(values, "rx rate", "last rx rate", "rx bitrate")); rx != 0 {
		connInfo.RxBitrate = rx
	}
	if tx := parseFirstFloat(firstValue(values, "tx rate", "last tx rate", "tx bitrate")); tx != 0 {
		connInfo.TxBitrate = tx
	}
	if standard := firstValue(values, "phy mode", "phy", "protocol"); standard != "" {
		connInfo.WiFiStandard = normalizeWiFiStandard(standard)
	}
	if connInfo.ChannelWidth == 0 {
		connInfo.ChannelWidth = 20
	}
	if connInfo.MIMOConfig == "" {
		connInfo.MIMOConfig = "1x1"
	}
	return connInfo
}

func parseWdutilLinkInfo(output []byte) map[string]string {
	values := parseWdutilKeyValues(output)
	info := make(map[string]string)
	connected := isConnectedState(firstValue(values, "state", "status", "link status")) ||
		firstValue(values, "ssid") != "" || firstValue(values, "bssid") != ""
	info["connected"] = strconv.FormatBool(connected)
	if !connected {
		return info
	}
	if bssid := extractBSSID(firstValue(values, "bssid", "ap bssid", "current bssid")); bssid != "" {
		info["bssid"] = bssid
	}
	if signal := parseFirstInt(firstValue(values, "rssi", "signal", "agrctlrssi")); signal != 0 {
		info["signal"] = strconv.Itoa(signal)
		info["signal_avg"] = strconv.Itoa(signal)
	}
	if rx := parseFirstFloat(firstValue(values, "rx rate", "last rx rate", "rx bitrate")); rx != 0 {
		info["rx_bitrate"] = strconv.FormatFloat(rx, 'f', -1, 64)
	}
	if tx := parseFirstFloat(firstValue(values, "tx rate", "last tx rate", "tx bitrate")); tx != 0 {
		info["tx_bitrate"] = strconv.FormatFloat(tx, 'f', -1, 64)
	}
	if channel := parseFirstInt(firstValue(values, "channel", "primary channel")); channel != 0 {
		info["channel"] = strconv.Itoa(channel)
	}
	if width := parseChannelWidth(firstValue(values, "channel width", "chan width", "channel")); width != 0 {
		info["channel_width"] = strconv.Itoa(width)
	}
	if info["channel"] == "" {
		if ch, width := parseWdutilChannel(firstValue(values, "channel", "primary channel")); ch != 0 {
			info["channel"] = strconv.Itoa(ch)
			if info["channel_width"] == "" && width != 0 {
				info["channel_width"] = strconv.Itoa(width)
			}
		}
	}
	info["rx_bytes"] = "0"
	info["tx_bytes"] = "0"
	info["rx_packets"] = "0"
	info["tx_packets"] = "0"
	info["tx_retries"] = "0"
	info["tx_failed"] = "0"
	info["connected_time"] = "0"
	return info
}

func parseWdutilStationInfo(output []byte) map[string]string {
	values := parseWdutilKeyValues(output)
	stats := make(map[string]string)
	connected := isConnectedState(firstValue(values, "state", "status", "link status")) ||
		firstValue(values, "ssid") != "" || firstValue(values, "bssid") != ""
	stats["connected"] = strconv.FormatBool(connected)
	if !connected {
		return stats
	}
	if bssid := extractBSSID(firstValue(values, "bssid", "ap bssid", "current bssid")); bssid != "" {
		stats["bssid"] = bssid
	}
	if signal := parseFirstInt(firstValue(values, "rssi", "signal", "agrctlrssi")); signal != 0 {
		stats["signal"] = strconv.Itoa(signal)
		stats["signal_avg"] = strconv.Itoa(signal)
	}
	if noise := parseFirstInt(firstValue(values, "noise")); noise != 0 {
		stats["noise"] = strconv.Itoa(noise)
		if signal := parseFirstInt(firstValue(values, "rssi", "signal", "agrctlrssi")); signal != 0 {
			stats["snr"] = strconv.Itoa(signal - noise)
		}
	}
	if rx := parseFirstFloat(firstValue(values, "rx rate", "last rx rate", "rx bitrate")); rx != 0 {
		stats["rx_bitrate"] = strconv.FormatFloat(rx, 'f', -1, 64)
	}
	if tx := parseFirstFloat(firstValue(values, "tx rate", "last tx rate", "tx bitrate")); tx != 0 {
		stats["tx_bitrate"] = strconv.FormatFloat(tx, 'f', -1, 64)
	}
	stats["rx_bytes"] = "0"
	stats["tx_bytes"] = "0"
	stats["rx_packets"] = "0"
	stats["tx_packets"] = "0"
	stats["tx_retries"] = "0"
	stats["tx_failed"] = "0"
	stats["connected_time"] = "0"
	stats["last_ack_signal"] = "0"
	return stats
}

func parseSystemProfilerConnectionInfo(output []byte) ConnectionInfo {
	info := parseSystemProfilerCurrentNetwork(output)
	connInfo := ConnectionInfo{}
	connInfo.Connected = info["connected"] == "true"
	connInfo.SSID = info["ssid"]
	connInfo.BSSID = info["bssid"]
	if ch := parseFirstInt(info["channel"]); ch != 0 {
		connInfo.Channel = ch
		connInfo.Frequency = channelToFrequency(ch)
	}
	if width := parseChannelWidth(info["channel_width"]); width != 0 {
		connInfo.ChannelWidth = width
	}
	if signal := parseFirstInt(info["signal"]); signal != 0 {
		connInfo.Signal = signal
		connInfo.SignalAvg = signal
	}
	if rx := parseFirstFloat(info["rx_bitrate"]); rx != 0 {
		connInfo.RxBitrate = rx
	}
	if tx := parseFirstFloat(info["tx_bitrate"]); tx != 0 {
		connInfo.TxBitrate = tx
	}
	if standard := info["wifi_standard"]; standard != "" {
		connInfo.WiFiStandard = standard
	}
	if connInfo.ChannelWidth == 0 {
		connInfo.ChannelWidth = 20
	}
	if connInfo.MIMOConfig == "" {
		connInfo.MIMOConfig = "1x1"
	}
	return connInfo
}

func parseSystemProfilerLinkInfo(output []byte) map[string]string {
	info := parseSystemProfilerCurrentNetwork(output)
	if info["connected"] == "" {
		info["connected"] = "false"
	}
	return info
}

func parseSystemProfilerStationInfo(output []byte) map[string]string {
	info := parseSystemProfilerCurrentNetwork(output)
	if info["connected"] == "" {
		info["connected"] = "false"
	}
	return info
}

func parseNetworksetupConnectionInfo(iface string) ConnectionInfo {
	connInfo := ConnectionInfo{}
	if iface == "" {
		return connInfo
	}
	cmd := exec.Command("/usr/sbin/networksetup", "-getairportnetwork", iface)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return connInfo
	}
	line := strings.TrimSpace(string(output))
	if strings.Contains(strings.ToLower(line), "not associated") || strings.Contains(line, "Off") {
		connInfo.Connected = false
		return connInfo
	}
	const prefix = "Current Wi-Fi Network:"
	if strings.HasPrefix(line, prefix) {
		connInfo.SSID = strings.TrimSpace(strings.TrimPrefix(line, prefix))
	}
	if connInfo.SSID != "" {
		connInfo.Connected = true
	}
	return connInfo
}

func parseWdutilKeyValues(output []byte) map[string]string {
	values := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			continue
		}
		if isEmptyWdutilValue(value) {
			continue
		}
		values[key] = value
	}
	return values
}

func firstValue(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[strings.ToLower(key)]; ok && value != "" {
			return value
		}
	}
	return ""
}

func parseFirstInt(value string) int {
	re := regexp.MustCompile(`-?\d+`)
	match := re.FindString(value)
	if match == "" {
		return 0
	}
	if i, err := strconv.Atoi(match); err == nil {
		return i
	}
	return 0
}

func parseFirstFloat(value string) float64 {
	re := regexp.MustCompile(`-?\d+(?:\.\d+)?`)
	match := re.FindString(value)
	if match == "" {
		return 0
	}
	if f, err := strconv.ParseFloat(match, 64); err == nil {
		return f
	}
	return 0
}

func parseChannelWidth(value string) int {
	re := regexp.MustCompile(`(\d+)\s*MHz`)
	matches := re.FindStringSubmatch(value)
	if len(matches) < 2 {
		return 0
	}
	if width, err := strconv.Atoi(matches[1]); err == nil {
		return width
	}
	return 0
}

func parseWdutilChannel(value string) (int, int) {
	re := regexp.MustCompile(`(?i)(?:2g|5g|6g)?\s*(\d+)\s*/\s*(\d+)`)
	matches := re.FindStringSubmatch(value)
	if len(matches) < 3 {
		return 0, 0
	}
	channel, _ := strconv.Atoi(matches[1])
	width, _ := strconv.Atoi(matches[2])
	return channel, width
}

func isEmptyWdutilValue(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return trimmed == "none" || trimmed == "n/a" || trimmed == "null"
}

func extractBSSID(value string) string {
	re := regexp.MustCompile(`([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}`)
	match := re.FindString(value)
	return strings.ToLower(match)
}

func isConnectedState(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "running" || value == "connected" || value == "up" || value == "active"
}

func normalizeWiFiStandard(value string) string {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "802.11") {
		return value
	}
	if strings.Contains(strings.ToLower(value), "ax") {
		return "802.11ax"
	}
	if strings.Contains(strings.ToLower(value), "ac") {
		return "802.11ac"
	}
	if strings.Contains(strings.ToLower(value), "n") {
		return "802.11n"
	}
	return value
}
