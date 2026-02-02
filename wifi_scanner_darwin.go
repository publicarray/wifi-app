package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type darwinScanner struct {
	currentInterface  string
	ouiLookup         *OUILookup
	airportPath       string
	hasAirport        bool
	hasWdutil         bool
	hasSystemProfiler bool
	hasNetworksetup   bool
	airportParser     *airportParser
	systemParser      *systemProfilerParser
}

func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()
	airportPath := findAirportPath()
	_, wdutilErr := exec.LookPath("wdutil")
	_, profilerErr := exec.LookPath("system_profiler")
	_, networksetupErr := exec.LookPath("networksetup")

	return &darwinScanner{
		ouiLookup:         ouiLookup,
		airportPath:       airportPath,
		hasAirport:        airportPath != "",
		hasWdutil:         wdutilErr == nil,
		hasSystemProfiler: profilerErr == nil,
		hasNetworksetup:   networksetupErr == nil,
		airportParser:     &airportParser{ouiLookup: ouiLookup},
		systemParser:      &systemProfilerParser{ouiLookup: ouiLookup},
	}
}

func (s *darwinScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
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
	if s.hasAirport {
		cmd := exec.Command(s.airportPath, "-I")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]string{"connected": "false"}, fmt.Errorf("failed to get link info with airport: %w", err)
		}
		return s.airportParser.ParseLink(output)
	}

	if s.hasWdutil {
		cmd := exec.Command("wdutil", "info")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]string{"connected": "false"}, fmt.Errorf("failed to get link info with wdutil: %w (output: %s)", err, string(output))
		}
		return parseWdutilLinkInfo(output), nil
	}

	if s.hasSystemProfiler {
		cmd := exec.Command("system_profiler", "-json", "SPAirPortDataType")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]string{"connected": "false"}, fmt.Errorf("failed to get link info with system_profiler: %w (output: %s)", err, string(output))
		}
		return parseSystemProfilerLinkInfo(output), nil
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
		return link, nil
	}

	return map[string]string{"connected": "false"}, fmt.Errorf("no macOS WiFi info command available (airport/wdutil/system_profiler/networksetup missing)")
}

func (s *darwinScanner) Close() error {
	return nil
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
	noiseRegex := regexp.MustCompile(`\s+agrCtlNoise:\s+(-?\d+)`)
	rxMcsRegex := regexp.MustCompile(`\s+lastRxRate:\s+(\d+)`)
	txMcsRegex := regexp.MustCompile(`\s+lastTxRate:\s+(\d+)`)

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
		if matches := noiseRegex.FindStringSubmatch(line); matches != nil {
			if noise, err := strconv.Atoi(matches[1]); err == nil {
				_ = noise
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
	}

	connInfo.WiFiStandard = "802.11ac/n"
	if connInfo.ChannelWidth == 0 {
		connInfo.ChannelWidth = 20
	}
	connInfo.MIMOConfig = "1x1"

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
	} else {
		connInfo.WiFiStandard = "802.11ac/n"
	}
	if connInfo.ChannelWidth == 0 {
		connInfo.ChannelWidth = 20
	}
	connInfo.MIMOConfig = "1x1"
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
	} else {
		connInfo.WiFiStandard = "802.11ac/n"
	}
	if connInfo.ChannelWidth == 0 {
		connInfo.ChannelWidth = 20
	}
	connInfo.MIMOConfig = "1x1"
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
