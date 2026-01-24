package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type windowsScanner struct {
	currentInterface string
	ouiLookup        *OUILookup
}

func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

	return &windowsScanner{
		ouiLookup: ouiLookup,
	}
}

func (s *windowsScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
	cmd := exec.Command("netsh", "wlan", "show", "networks", "mode=bssid", "interface="+iface)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to scan networks: %w (output: %s)", err, string(output))
	}

	return s.parseNetshScanOutput(string(output))
}

func (s *windowsScanner) parseNetshScanOutput(output string) ([]AccessPoint, error) {
	lines := strings.Split(output, "\n")
	var aps []AccessPoint
	var currentAP *AccessPoint

	bssidRegex := regexp.MustCompile(`BSSID\s+([0-9a-f:]+)`)
	ssidRegex := regexp.MustCompile(`SSID\s+\d+\s*:\s+(.+)`)
	signalRegex := regexp.MustCompile(`Signal\s*:\s+(\d+)%`)
	channelRegex := regexp.MustCompile(`Channel\s*:\s+(\d+)`)
	authRegex := regexp.MustCompile(`Authentication\s*:\s+(.+)`)
	encryptionRegex := regexp.MustCompile(`Encryption\s*:\s+(.+)`)

	for _, line := range lines {
		if matches := ssidRegex.FindStringSubmatch(line); matches != nil {
			if currentAP != nil {
				aps = append(aps, *currentAP)
			}
			currentAP = &AccessPoint{
				Vendor:       s.ouiLookup.LookupVendor(line),
				SSID:         strings.TrimSpace(matches[1]),
				LastSeen:     getCurrentTime(),
				Capabilities: []string{},
			}
		}
		if matches := bssidRegex.FindStringSubmatch(line); matches != nil && currentAP != nil {
			currentAP.BSSID = matches[1]
		}
		if matches := signalRegex.FindStringSubmatch(line); matches != nil && currentAP != nil {
			if signal, err := strconv.Atoi(matches[1]); err == nil {
				currentAP.Signal = (signal - 100)
				currentAP.SignalQuality = signal
			}
		}
		if matches := channelRegex.FindStringSubmatch(line); matches != nil && currentAP != nil {
			if ch, err := strconv.Atoi(matches[1]); err == nil {
				currentAP.Channel = ch
			}
		}
		if matches := authRegex.FindStringSubmatch(line); matches != nil && currentAP != nil {
			auth := strings.TrimSpace(matches[1])
			if auth == "WPA2-Personal" || auth == "WPA2-Enterprise" {
				currentAP.Security = "WPA2"
			} else if auth == "WPA3-Personal" || auth == "WPA3-SAE" {
				currentAP.Security = "WPA3"
			} else if auth == "WPA" {
				currentAP.Security = "WPA"
			} else if auth == "Open" {
				currentAP.Security = "Open"
			} else {
				currentAP.Security = auth
			}
		}
		if matches := encryptionRegex.FindStringSubmatch(line); matches != nil && currentAP != nil && currentAP.Security == "" {
			enc := strings.TrimSpace(matches[1])
			if enc == "AES" || enc == "CCMP" {
				currentAP.Security = "WPA2"
			} else if enc == "TKIP" {
				currentAP.Security = "WPA"
			}
		}
	}

	if currentAP != nil {
		aps = append(aps, *currentAP)
	}

	// Post-process to set frequency, band, and default values
	for i := range aps {
		// Set frequency and band based on channel
		aps[i].Frequency = channelToFrequency(aps[i].Channel)
		if aps[i].Frequency > 5900 {
			aps[i].Band = "6GHz"
		} else if aps[i].Frequency > 5000 {
			aps[i].Band = "5GHz"
		} else if aps[i].Frequency > 2400 {
			aps[i].Band = "2.4GHz"
		}

		// Set defaults if not already set
		if aps[i].ChannelWidth == 0 {
			aps[i].ChannelWidth = 20
		}
		if aps[i].Security == "" {
			aps[i].Security = "Open"
		}
		// SignalQuality already set from signal parsing
		if aps[i].SignalQuality == 0 && aps[i].Signal != 0 {
			aps[i].SignalQuality = signalToQuality(aps[i].Signal)
		}
	}

	return aps, nil
}

func (s *windowsScanner) GetInterfaces() ([]string, error) {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w (output: %s)", err, string(output))
	}

	lines := strings.Split(string(output), "\n")
	var interfaces []string
	nameRegex := regexp.MustCompile(`:\s+(\S+)`)

	for _, line := range lines {
		if strings.Contains(line, "Wi-Fi") {
			if matches := nameRegex.FindStringSubmatch(line); matches != nil {
				name := strings.TrimSpace(matches[1])
				if name != "" && !contains(interfaces, name) {
					interfaces = append(interfaces, name)
				}
			}
		}
	}

	if len(interfaces) == 0 {
		return nil, fmt.Errorf("no WiFi interfaces found")
	}

	return interfaces, nil
}

func (s *windowsScanner) GetConnectionInfo(iface string) (ConnectionInfo, error) {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces", fmt.Sprintf("interface=%s", iface))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("failed to get connection info: %w", err)
	}

	lines := strings.Split(string(output), "\n")

	stateRegex := regexp.MustCompile(`State\s*:\s*(\w+)`)
	ssidRegex := regexp.MustCompile(`SSID\s*:\s*(.+)`)
	bssidRegex := regexp.MustCompile(`BSSID\s*:\s*([0-9a-f:]+)`)
	channelRegex := regexp.MustCompile(`Channel\s*:\s*(\d+)`)
	receiveRateRegex := regexp.MustCompile(`Receive rate \(Mbps\)\s*:\s*([\d.]+)`)
	transmitRateRegex := regexp.MustCompile(`Transmit rate \(Mbps\)\s*:\s*([\d.]+)`)
	signalRegex := regexp.MustCompile(`Signal\s*:\s*(\d+)%`)

	connInfo := ConnectionInfo{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := stateRegex.FindStringSubmatch(line); matches != nil {
			connInfo.Connected = strings.Contains(matches[1], "connected")
		}
		if matches := ssidRegex.FindStringSubmatch(line); matches != nil {
			connInfo.SSID = strings.TrimSpace(matches[1])
		}
		if matches := bssidRegex.FindStringSubmatch(line); matches != nil {
			connInfo.BSSID = strings.TrimSpace(matches[1])
		}
		if matches := channelRegex.FindStringSubmatch(line); matches != nil {
			if ch, err := strconv.Atoi(matches[1]); err == nil {
				connInfo.Channel = ch
			}
		}
		if matches := receiveRateRegex.FindStringSubmatch(line); matches != nil {
			if rate, err := strconv.ParseFloat(matches[1], 64); err == nil {
				connInfo.RxBitrate = rate
			}
		}
		if matches := transmitRateRegex.FindStringSubmatch(line); matches != nil {
			if rate, err := strconv.ParseFloat(matches[1], 64); err == nil {
				connInfo.TxBitrate = rate
			}
		}
		if matches := signalRegex.FindStringSubmatch(line); matches != nil {
			if signal, err := strconv.Atoi(matches[1]); err == nil {
				connInfo.Signal = signal - 100
				connInfo.SignalAvg = signal - 100
			}
		}
	}

	if connInfo.Signal == 0 {
		connInfo.Signal = -70
	}
	if connInfo.SignalAvg == 0 {
		connInfo.SignalAvg = connInfo.Signal
	}
	connInfo.WiFiStandard = "802.11ac/n"
	if connInfo.ChannelWidth == 0 {
		connInfo.ChannelWidth = 20
	}
	if connInfo.MIMOConfig == "" {
		connInfo.MIMOConfig = "1x1"
	}

	return connInfo, nil
}

func (s *windowsScanner) GetLinkInfo(iface string) (map[string]string, error) {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces", fmt.Sprintf("interface=%s", iface))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]string{"connected": "false"}, fmt.Errorf("failed to get link info: %w", err)
	}

	lines := strings.Split(string(output), "\n")

	stateRegex := regexp.MustCompile(`State\s*:\s*(\w+)`)
	ssidRegex := regexp.MustCompile(`SSID\s*:\s*(.+)`)
	bssidRegex := regexp.MustCompile(`BSSID\s*:\s*([0-9a-f:]+)`)
	channelRegex := regexp.MustCompile(`Channel\s*:\s*(\d+)`)
	receiveRateRegex := regexp.MustCompile(`Receive rate \(Mbps\)\s*:\s*([\d.]+)`)
	transmitRateRegex := regexp.MustCompile(`Transmit rate \(Mbps\)\s*:\s*([\d.]+)`)
	signalRegex := regexp.MustCompile(`Signal\s*:\s*(\d+)%`)

	info := make(map[string]string)
	connected := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := stateRegex.FindStringSubmatch(line); matches != nil {
			connected = strings.Contains(matches[1], "connected")
		}
		if matches := ssidRegex.FindStringSubmatch(line); matches != nil {
			info["ssid"] = strings.TrimSpace(matches[1])
		}
		if matches := bssidRegex.FindStringSubmatch(line); matches != nil {
			info["bssid"] = strings.TrimSpace(matches[1])
		}
		if matches := channelRegex.FindStringSubmatch(line); matches != nil {
			info["channel"] = matches[1]
		}
		if matches := receiveRateRegex.FindStringSubmatch(line); matches != nil {
			if rate, err := strconv.ParseFloat(matches[1], 64); err == nil {
				info["rx_bitrate"] = fmt.Sprintf("%.1f", rate)
			}
		}
		if matches := transmitRateRegex.FindStringSubmatch(line); matches != nil {
			if rate, err := strconv.ParseFloat(matches[1], 64); err == nil {
				info["tx_bitrate"] = fmt.Sprintf("%.1f", rate)
			}
		}
		if matches := signalRegex.FindStringSubmatch(line); matches != nil {
			if signal, err := strconv.Atoi(matches[1]); err == nil {
				signalDbm := signal - 100
				info["signal"] = fmt.Sprintf("%d", signalDbm)
				info["signal_avg"] = fmt.Sprintf("%d", signalDbm)
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

func (s *windowsScanner) GetStationStats(iface string) (map[string]string, error) {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces", fmt.Sprintf("interface=%s", iface))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]string{"connected": "false"}, fmt.Errorf("failed to get station stats: %w", err)
	}

	lines := strings.Split(string(output), "\n")

	stateRegex := regexp.MustCompile(`State\s*:\s*(\w+)`)
	bssidRegex := regexp.MustCompile(`BSSID\s*:\s*([0-9a-f:]+)`)
	receiveRateRegex := regexp.MustCompile(`Receive rate \(Mbps\)\s*:\s*([\d.]+)`)
	transmitRateRegex := regexp.MustCompile(`Transmit rate \(Mbps\)\s*:\s*([\d.]+)`)
	signalRegex := regexp.MustCompile(`Signal\s*:\s*(\d+)%`)

	stats := make(map[string]string)
	connected := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := stateRegex.FindStringSubmatch(line); matches != nil {
			connected = strings.Contains(matches[1], "connected")
		}
		if matches := bssidRegex.FindStringSubmatch(line); matches != nil {
			stats["bssid"] = strings.TrimSpace(matches[1])
		}
		if matches := receiveRateRegex.FindStringSubmatch(line); matches != nil {
			stats["rx_bitrate"] = matches[1]
		}
		if matches := transmitRateRegex.FindStringSubmatch(line); matches != nil {
			stats["tx_bitrate"] = matches[1]
		}
		if matches := signalRegex.FindStringSubmatch(line); matches != nil {
			if signal, err := strconv.Atoi(matches[1]); err == nil {
				signalDbm := signal - 100
				stats["signal"] = fmt.Sprintf("%d", signalDbm)
				stats["signal_avg"] = fmt.Sprintf("%d", signalDbm)
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

func (s *windowsScanner) Close() error {
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getCurrentTime() time.Time {
	return time.Now()
}
