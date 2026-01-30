package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type darwinScanner struct {
	currentInterface string
	ouiLookup        *OUILookup
}

func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

	return &darwinScanner{
		ouiLookup: ouiLookup,
	}
}

func (s *darwinScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
	cmd := exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-s", "-x")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to scan networks: %w (output: %s)", err, string(output))
	}

	return s.parseAirportScanXML(output)
}

func (s *darwinScanner) parseAirportScanXML(output []byte) ([]AccessPoint, error) {
	entries, err := parsePlistArray(output)
	if err != nil {
		return nil, err
	}

	var aps []AccessPoint
	for _, entry := range entries {
		ssid := getString(entry, "SSID_STR", "SSID")
		bssid := getString(entry, "BSSID")
		if ssid == "" || bssid == "" {
			continue
		}

		signal := getInt(entry, "RSSI")
		channel := getInt(entry, "CHANNEL")
		channelWidth := getInt(entry, "CHANNEL_WIDTH")
		if channelWidth == 0 {
			channelWidth = 20
		}
		freq := channelToFrequency(channel)
		band := "2.4GHz"
		if freq > 5900 {
			band = "6GHz"
		} else if freq > 5000 {
			band = "5GHz"
		}

		securityField := getJoinedString(entry, "SECURITY")
		security, ciphers, authMethods, pmf := parseAirportSecurity(securityField)

		ap := AccessPoint{
			SSID:            ssid,
			BSSID:           bssid,
			Channel:         channel,
			Frequency:       freq,
			Band:            band,
			Signal:          signal,
			SignalQuality:   signalToQuality(signal),
			Vendor:          s.ouiLookup.LookupVendor(bssid),
			LastSeen:        s.parseAirportTime(),
			Capabilities:    []string{},
			ChannelWidth:    channelWidth,
			Security:        security,
			SecurityCiphers: ciphers,
			AuthMethods:     authMethods,
			PMF:             pmf,
			CountryCode:     strings.ToUpper(getString(entry, "COUNTRY_CODE", "CC")),
			Noise:           getInt(entry, "NOISE"),
		}

		if getBool(entry, "HT") {
			ap.Capabilities = appendUnique(ap.Capabilities, "HT")
		}
		if getBool(entry, "VHT") {
			ap.Capabilities = appendUnique(ap.Capabilities, "VHT")
		}
		if getBool(entry, "HE") {
			ap.Capabilities = appendUnique(ap.Capabilities, "HE")
		}

		if ap.Noise != 0 {
			ap.SNR = ap.Signal - ap.Noise
		}
		ap.DFS = isDFSChannel(ap.Channel)
		if ap.Security == "" {
			ap.Security = "Open"
		}
		aps = append(aps, ap)
	}

	return aps, nil
}

func (s *darwinScanner) parseAirportScanOutput(output string) ([]AccessPoint, error) {
	lines := strings.Split(output, "\n")
	var aps []AccessPoint
	bssidRegex := regexp.MustCompile(`([0-9a-fA-F:]{17})`)

	for i, line := range lines {
		if i == 0 {
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		loc := bssidRegex.FindStringIndex(line)
		if loc == nil {
			continue
		}

		ssid := strings.TrimSpace(line[:loc[0]])
		bssid := line[loc[0]:loc[1]]
		rest := strings.Fields(line[loc[1]:])
		if len(rest) < 3 {
			continue
		}

		signal := -100
		channel := -1
		channelWidth := 20
		htField := ""
		countryCode := ""
		securityField := ""
		if rssi, err := strconv.Atoi(rest[0]); err == nil {
			signal = rssi
		}
		channelField := rest[1]
		if comma := strings.Index(channelField, ","); comma > 0 {
			if width, err := strconv.Atoi(strings.TrimSpace(channelField[comma+1:])); err == nil {
				channelWidth = width
			}
			channelField = channelField[:comma]
		}
		if ch, err := strconv.Atoi(channelField); err == nil {
			channel = ch
		}
		if len(rest) >= 3 {
			htField = rest[2]
		}
		if len(rest) >= 4 {
			countryCode = rest[3]
		}
		if len(rest) >= 5 {
			securityField = strings.Join(rest[4:], " ")
		}

		if ssid != "" && channel > 0 {
			freq := channelToFrequency(channel)
			band := "2.4GHz"
			if freq > 5900 {
				band = "6GHz"
			} else if freq > 5000 {
				band = "5GHz"
			}

			security, ciphers, authMethods, pmf := parseAirportSecurity(securityField)
			ap := AccessPoint{
				SSID:            ssid,
				BSSID:           bssid,
				Channel:         channel,
				Frequency:       freq,
				Band:            band,
				Signal:          signal,
				SignalQuality:   signalToQuality(signal),
				Vendor:          s.ouiLookup.LookupVendor(bssid),
				LastSeen:        s.parseAirportTime(),
				Capabilities:    []string{},
				ChannelWidth:    channelWidth,
				Security:        security,
				SecurityCiphers: ciphers,
				AuthMethods:     authMethods,
				PMF:             pmf,
				CountryCode:     strings.ToUpper(countryCode),
			}
			if strings.Contains(strings.ToUpper(htField), "Y") {
				ap.Capabilities = appendUnique(ap.Capabilities, "HT")
			}
			if strings.Contains(strings.ToUpper(htField), "VHT") {
				ap.Capabilities = appendUnique(ap.Capabilities, "VHT")
			}
			if strings.Contains(strings.ToUpper(htField), "HE") {
				ap.Capabilities = appendUnique(ap.Capabilities, "HE")
			}
			ap.DFS = isDFSChannel(ap.Channel)
			if ap.Security == "" {
				ap.Security = "Open"
			}
			aps = append(aps, ap)
		}
	}

	return aps, nil
}

func (s *darwinScanner) parseAirportTime() time.Time {
	return time.Now()
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
	cmd := exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-I")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("failed to get connection info: %w", err)
	}

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
			if matches[1] == "running" {
				connInfo.Connected = true
			} else {
				connInfo.Connected = false
			}
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
				connInfo.Noise = noise
				if connInfo.Signal != 0 {
					connInfo.SNR = connInfo.Signal - noise
				}
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

	return connInfo, nil
}

func (s *darwinScanner) GetStationStats(iface string) (map[string]string, error) {
	cmd := exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-I")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]string{"connected": "false"}, fmt.Errorf("failed to get station stats: %w", err)
	}

	lines := strings.Split(string(output), "\n")

	stateRegex := regexp.MustCompile(`\s+state:\s+(\S+)`)
	bssidRegex := regexp.MustCompile(`\s+BSSID:\s+([0-9a-f:]+)`)
	rxBitrateRegex := regexp.MustCompile(`\s+lastRxRate:\s+(\d+)`)
	txBitrateRegex := regexp.MustCompile(`\s+lastTxRate:\s+(\d+)`)
	rssiRegex := regexp.MustCompile(`\s+agrCtlRSSI:\s+(-?\d+)`)
	agrCtlRSSIRegex := regexp.MustCompile(`\s+agrCtlRSSI:\s+(-?\d+)`)
	noiseRegex := regexp.MustCompile(`\s+agrCtlNoise:\s+(-?\d+)`)

	stats := make(map[string]string)
	connected := false

	for _, line := range lines {
		if matches := stateRegex.FindStringSubmatch(line); matches != nil {
			if matches[1] == "running" {
				connected = true
			}
		}
		if matches := bssidRegex.FindStringSubmatch(line); matches != nil {
			stats["bssid"] = matches[1]
		}
		if matches := rxBitrateRegex.FindStringSubmatch(line); matches != nil {
			stats["rx_bitrate"] = matches[1]
		}
		if matches := txBitrateRegex.FindStringSubmatch(line); matches != nil {
			stats["tx_bitrate"] = matches[1]
		}
		if matches := rssiRegex.FindStringSubmatch(line); matches != nil {
			stats["signal"] = matches[1]
		}
		if matches := agrCtlRSSIRegex.FindStringSubmatch(line); matches != nil {
			stats["signal_avg"] = matches[1]
		}
		if matches := noiseRegex.FindStringSubmatch(line); matches != nil {
			stats["noise"] = matches[1]
			if signal, exists := stats["signal"]; exists {
				if noise, err := strconv.Atoi(matches[1]); err == nil {
					if signalVal, err := strconv.Atoi(signal); err == nil {
						stats["snr"] = fmt.Sprintf("%d", signalVal-noise)
					}
				}
			}
		}
	}

	if !connected {
		stats["connected"] = "false"
		return stats, nil
	}

	stats["connected"] = "true"

	stats["rx_bytes"] = "0"
	stats["tx_bytes"] = "0"
	stats["rx_packets"] = "0"
	stats["tx_packets"] = "0"
	stats["tx_retries"] = "0"
	stats["tx_failed"] = "0"
	stats["connected_time"] = "0"
	stats["last_ack_signal"] = "0"

	return stats, nil
}

func (s *darwinScanner) GetLinkInfo(iface string) (map[string]string, error) {
	cmd := exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-I")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]string{"connected": "false"}, fmt.Errorf("failed to get link info: %w", err)
	}

	lines := strings.Split(string(output), "\n")

	stateRegex := regexp.MustCompile(`\s+state:\s+(\S+)`)
	bssidRegex := regexp.MustCompile(`\s+BSSID:\s+([0-9a-f:]+)`)
	rssiRegex := regexp.MustCompile(`\s+agrCtlRSSI:\s+(-?\d+)`)
	agrCtlRSSIRegex := regexp.MustCompile(`\s+agrCtlRSSI:\s+(-?\d+)`)
	rxMcsRegex := regexp.MustCompile(`\s+lastRxRate:\s+(\d+)`)
	txMcsRegex := regexp.MustCompile(`\s+lastTxRate:\s+(\d+)`)
	channelRegex := regexp.MustCompile(`\s+channel:\s+(\d+)(?:,\s*(\d+))?`)

	info := make(map[string]string)
	connected := false

	for _, line := range lines {
		if matches := stateRegex.FindStringSubmatch(line); matches != nil {
			if matches[1] == "running" {
				connected = true
			}
		}
		if matches := bssidRegex.FindStringSubmatch(line); matches != nil {
			info["bssid"] = matches[1]
		}
		if matches := rssiRegex.FindStringSubmatch(line); matches != nil {
			if rssi, err := strconv.Atoi(matches[1]); err == nil {
				info["signal"] = fmt.Sprintf("%d", rssi)
				info["signal_avg"] = fmt.Sprintf("%d", rssi)
			}
		}
		if matches := agrCtlRSSIRegex.FindStringSubmatch(line); matches != nil {
			if rssi, err := strconv.Atoi(matches[1]); err == nil {
				info["signal"] = fmt.Sprintf("%d", rssi)
				info["signal_avg"] = fmt.Sprintf("%d", rssi)
			}
		}
		if matches := rxMcsRegex.FindStringSubmatch(line); matches != nil {
			if rate, err := strconv.ParseFloat(matches[1], 64); err == nil {
				info["rx_bitrate"] = fmt.Sprintf("%.1f", rate)
			}
		}
		if matches := txMcsRegex.FindStringSubmatch(line); matches != nil {
			if rate, err := strconv.ParseFloat(matches[1], 64); err == nil {
				info["tx_bitrate"] = fmt.Sprintf("%.1f", rate)
			}
		}
		if matches := channelRegex.FindStringSubmatch(line); matches != nil {
			info["channel"] = matches[1]
			if len(matches) > 2 && matches[2] != "" {
				info["channel_width"] = matches[2]
			}
		}
	}

	if !connected {
		info["connected"] = "false"
		return info, nil
	}

	info["connected"] = "true"

	info["rx_bytes"] = "0"
	info["tx_bytes"] = "0"
	info["rx_packets"] = "0"
	info["tx_packets"] = "0"
	info["tx_retries"] = "0"
	info["tx_failed"] = "0"
	info["connected_time"] = "0"

	return info, nil
}

func parseAirportSecurity(security string) (string, []string, []string, string) {
	sec := strings.TrimSpace(security)
	if sec == "" {
		return "Open", nil, nil, "Disabled"
	}
	upper := strings.ToUpper(sec)
	securityType := ""
	switch {
	case strings.Contains(upper, "WPA3") || strings.Contains(upper, "SAE"):
		securityType = "WPA3"
	case strings.Contains(upper, "WPA2"):
		securityType = "WPA2"
	case strings.Contains(upper, "WPA"):
		securityType = "WPA"
	case strings.Contains(upper, "WEP"):
		securityType = "WEP"
	case strings.Contains(upper, "OPEN"):
		securityType = "Open"
	default:
		securityType = "Open"
	}

	var ciphers []string
	var authMethods []string
	for _, token := range []string{"AES", "CCMP", "TKIP", "GCMP"} {
		if strings.Contains(upper, token) {
			ciphers = appendUnique(ciphers, token)
		}
	}
	for _, token := range []string{"PSK", "SAE", "EAP", "8021X"} {
		if strings.Contains(upper, token) {
			authMethods = appendUnique(authMethods, token)
		}
	}

	pmf := "Disabled"
	if strings.Contains(upper, "MFP") || strings.Contains(upper, "PMF") {
		if strings.Contains(upper, "REQUIRED") {
			pmf = "Required"
		} else {
			pmf = "Optional"
		}
	}

	return securityType, ciphers, authMethods, pmf
}

func parsePlistArray(data []byte) ([]map[string]interface{}, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var (
		inArray    bool
		currentKey string
		currentMap map[string]interface{}
		result     []map[string]interface{}
	)

	for {
		token, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
		switch elem := token.(type) {
		case xml.StartElement:
			switch elem.Name.Local {
			case "array":
				inArray = true
			case "dict":
				if inArray {
					currentMap = make(map[string]interface{})
				}
			case "key":
				currentKey, _ = readCharData(decoder)
			case "string", "integer", "real", "date", "data":
				valueText, _ := readCharData(decoder)
				if currentMap != nil && currentKey != "" {
					currentMap[currentKey] = castPlistValue(elem.Name.Local, valueText)
				}
			case "true":
				if currentMap != nil && currentKey != "" {
					currentMap[currentKey] = true
				}
			case "false":
				if currentMap != nil && currentKey != "" {
					currentMap[currentKey] = false
				}
			}
		case xml.EndElement:
			switch elem.Name.Local {
			case "dict":
				if currentMap != nil {
					result = append(result, currentMap)
					currentMap = nil
				}
			case "array":
				inArray = false
			}
		}
	}

	return result, nil
}

func readCharData(decoder *xml.Decoder) (string, error) {
	var value strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return value.String(), err
		}
		switch t := token.(type) {
		case xml.CharData:
			value.Write([]byte(t))
		case xml.EndElement:
			return strings.TrimSpace(value.String()), nil
		}
	}
}

func castPlistValue(kind string, value string) interface{} {
	switch kind {
	case "integer":
		if i, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return i
		}
	case "real":
		if f, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return f
		}
	case "date":
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(value)); err == nil {
			return t
		}
	}
	return strings.TrimSpace(value)
}

func getString(entry map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := entry[key]; ok {
			switch v := value.(type) {
			case string:
				return v
			case fmt.Stringer:
				return v.String()
			}
		}
	}
	return ""
}

func getJoinedString(entry map[string]interface{}, key string) string {
	if value, ok := entry[key]; ok {
		switch v := value.(type) {
		case string:
			return v
		case []interface{}:
			parts := make([]string, 0, len(v))
			for _, item := range v {
				parts = append(parts, fmt.Sprint(item))
			}
			return strings.Join(parts, " ")
		}
	}
	return ""
}

func getInt(entry map[string]interface{}, key string) int {
	if value, ok := entry[key]; ok {
		switch v := value.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return i
			}
		}
	}
	return 0
}

func getBool(entry map[string]interface{}, key string) bool {
	if value, ok := entry[key]; ok {
		switch v := value.(type) {
		case bool:
			return v
		case string:
			return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
		case int:
			return v != 0
		}
	}
	return false
}

func (s *darwinScanner) Close() error {
	return nil
}
