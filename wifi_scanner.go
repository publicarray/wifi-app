package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// WiFiScanner handles WiFi scanning operations on Linux using iw
type WiFiScanner struct {
	currentInterface string
	ouiLookup        *OUILookup
}

// NewWiFiScanner creates a new WiFi scanner instance
func NewWiFiScanner() *WiFiScanner {
	ouiLookup := NewOUILookup()
	go ouiLookup.LoadOUIDatabase()

	return &WiFiScanner{
		ouiLookup: ouiLookup,
	}
}

// GetInterfaces returns a list of available WiFi interfaces
func (s *WiFiScanner) GetInterfaces() ([]string, error) {
	cmd := exec.Command("iw", "dev")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w (output: %s)", err, string(output))
	}

	var interfaces []string
	lines := strings.Split(string(output), "\n")
	interfaceRegex := regexp.MustCompile(`^\s+Interface\s+(\S+)`)

	for _, line := range lines {
		if matches := interfaceRegex.FindStringSubmatch(line); matches != nil {
			interfaces = append(interfaces, matches[1])
		}
	}

	if len(interfaces) == 0 {
		return nil, fmt.Errorf("no WiFi interfaces found")
	}

	return interfaces, nil
}

// ScanNetworks scans for available WiFi networks on the specified interface
func (s *WiFiScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
	cmd := exec.Command("iw", iface, "scan")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "Operation not permitted") {
			return nil, fmt.Errorf("permission denied: WiFi scanning requires elevated privileges. Run with sudo or set capabilities: sudo setcap cap_net_admin+ep ./wifi-app")
		}
		return nil, fmt.Errorf("failed to scan networks: %w (output: %s)", err, string(output))
	}

	return s.parseScanOutput(string(output))
}

func (s *WiFiScanner) parseScanOutput(output string) ([]AccessPoint, error) {
	var aps []AccessPoint
	lines := strings.Split(output, "\n")

	if len(lines) == 0 {
		return []AccessPoint{}, nil
	}

	var currentAP *AccessPoint
	bssRegex := regexp.MustCompile(`^BSS\s+([0-9a-f:]+)`)
	freqRegex := regexp.MustCompile(`^\s+freq:\s+(\d+)`)
	signalRegex := regexp.MustCompile(`^\s+signal:\s+([-\d.]+)\s+dBm`)
	ssidRegex := regexp.MustCompile(`^\s+SSID:\s+(.*)$`)
	channelRegex := regexp.MustCompile(`^\s+\* primary channel:\s+(\d+)`)
	widthRegex := regexp.MustCompile(`HT20|HT40|VHT80|VHT160|HE80|HE160|320MHz`)

	for _, line := range lines {
		if matches := bssRegex.FindStringSubmatch(line); matches != nil {
			if currentAP != nil {
				aps = append(aps, *currentAP)
			}
			bssid := matches[1]
			currentAP = &AccessPoint{
				BSSID:        bssid,
				Vendor:       s.ouiLookup.LookupVendor(bssid),
				LastSeen:     time.Now(),
				Capabilities: []string{},
			}
		} else if currentAP != nil {
			if matches := freqRegex.FindStringSubmatch(line); matches != nil {
				freq, _ := strconv.Atoi(matches[1])
				currentAP.Frequency = freq
				currentAP.Channel = frequencyToChannel(freq)
				if freq > 5900 {
					currentAP.Band = "6GHz"
				} else if freq > 5000 {
					currentAP.Band = "5GHz"
				} else if freq > 2400 {
					currentAP.Band = "2.4GHz"
				}
			}

			if matches := signalRegex.FindStringSubmatch(line); matches != nil {
				signal, _ := strconv.ParseFloat(matches[1], 64)
				currentAP.Signal = int(signal)
				currentAP.SignalQuality = signalToQuality(int(signal))
			}

			if matches := ssidRegex.FindStringSubmatch(line); matches != nil {
				currentAP.SSID = strings.TrimSpace(matches[1])
			}

			if widthRegex.MatchString(line) {
				if strings.Contains(line, "HT20") {
					currentAP.ChannelWidth = 20
				} else if strings.Contains(line, "HT40") || strings.Contains(line, "VHT40") {
					currentAP.ChannelWidth = 40
				} else if strings.Contains(line, "VHT80") || strings.Contains(line, "HE80") {
					currentAP.ChannelWidth = 80
				} else if strings.Contains(line, "VHT160") || strings.Contains(line, "HE160") || strings.Contains(line, "320MHz") {
					currentAP.ChannelWidth = 160
				}
			}

			if strings.Contains(line, "HT") {
				currentAP.Capabilities = appendUnique(currentAP.Capabilities, "HT")
			}
			if strings.Contains(line, "VHT") {
				currentAP.Capabilities = appendUnique(currentAP.Capabilities, "VHT")
			}
			if strings.Contains(line, "HE") {
				currentAP.Capabilities = appendUnique(currentAP.Capabilities, "HE")
			}

			if strings.Contains(line, "WPA3") {
				currentAP.Security = "WPA3"
			} else if strings.Contains(line, "RSN") || strings.Contains(line, "WPA2") {
				if currentAP.Security == "" {
					currentAP.Security = "WPA2"
				}
			} else if strings.Contains(line, "WPA") {
				if currentAP.Security == "" {
					currentAP.Security = "WPA"
				}
			} else if strings.Contains(line, "capability:") && strings.Contains(line, "Privacy") {
				if currentAP.Security == "" {
					currentAP.Security = "WEP"
				}
			}

			if matches := channelRegex.FindStringSubmatch(line); matches != nil {
				ch, _ := strconv.Atoi(matches[1])
				currentAP.Channel = ch
			}
		}
	}

	if currentAP != nil {
		aps = append(aps, *currentAP)
	}

	for i := range aps {
		if aps[i].Security == "" {
			aps[i].Security = "Open"
		}
		if aps[i].ChannelWidth == 0 {
			aps[i].ChannelWidth = 20
		}
	}

	return aps, nil
}

// GetLinkInfo gets information about the current WiFi connection
func (s *WiFiScanner) GetLinkInfo(iface string) (map[string]string, error) {
	cmd := exec.Command("iw", iface, "link")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get link info: %w", err)
	}

	info := make(map[string]string)
	outputStr := string(output)

	if strings.Contains(outputStr, "Not connected") {
		info["connected"] = "false"
		return info, nil
	}

	info["connected"] = "true"
	lines := strings.Split(outputStr, "\n")

	bssidRegex := regexp.MustCompile(`Connected to ([0-9a-f:]+)`)
	ssidRegex := regexp.MustCompile(`SSID:\s+(.*)$`)
	freqRegex := regexp.MustCompile(`freq:\s+([\d.]+)`)
	signalRegex := regexp.MustCompile(`signal:\s+([-\d]+)\s+dBm`)
	rxBitrateRegex := regexp.MustCompile(`rx bitrate:\s+([\d.]+)\s+MBit/s(?:\s+([\dMHz\sHE-VHT]+))?`)
	txBitrateRegex := regexp.MustCompile(`tx bitrate:\s+([\d.]+)\s+MBit/s(?:\s+([\dMHz\sHE-VHT]+))?`)
	rxBytesRegex := regexp.MustCompile(`RX:\s+(\d+)\s+bytes`)
	txBytesRegex := regexp.MustCompile(`TX:\s+(\d+)\s+bytes`)

	for _, line := range lines {
		if matches := bssidRegex.FindStringSubmatch(line); matches != nil {
			info["bssid"] = matches[1]
		}
		if matches := ssidRegex.FindStringSubmatch(line); matches != nil {
			info["ssid"] = strings.TrimSpace(matches[1])
		}
		if matches := freqRegex.FindStringSubmatch(line); matches != nil {
			info["frequency"] = matches[1]
		}
		if matches := signalRegex.FindStringSubmatch(line); matches != nil {
			info["signal"] = matches[1]
		}
		if matches := rxBitrateRegex.FindStringSubmatch(line); matches != nil {
			info["rx_bitrate"] = matches[1]
			if len(matches) > 2 && matches[2] != "" {
				info["rx_bitrate_info"] = matches[2]
			}
		}
		if matches := txBitrateRegex.FindStringSubmatch(line); matches != nil {
			info["tx_bitrate"] = matches[1]
			if len(matches) > 2 && matches[2] != "" {
				info["tx_bitrate_info"] = matches[2]
			}
		}
		if matches := rxBytesRegex.FindStringSubmatch(line); matches != nil {
			info["rx_bytes"] = matches[1]
		}
		if matches := txBytesRegex.FindStringSubmatch(line); matches != nil {
			info["tx_bytes"] = matches[1]
		}
	}

	return info, nil
}

// GetStationStats gets detailed statistics about the connected station
func (s *WiFiScanner) GetStationStats(iface string) (map[string]string, error) {
	cmd := exec.Command("iw", iface, "station", "dump")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get station stats: %w", err)
	}

	stats := make(map[string]string)
	outputStr := string(output)

	if strings.TrimSpace(outputStr) == "" {
		return stats, nil
	}

	lines := strings.Split(outputStr, "\n")

	stationRegex := regexp.MustCompile(`^Station ([0-9a-f:]+)`)
	signalRegex := regexp.MustCompile(`signal:\s+([-\d]+)(?:\s+\[([-\d,\s]+)\])?\s+dBm`)
	signalAvgRegex := regexp.MustCompile(`signal avg:\s+([-\d]+)(?:\s+\[([-\d,\s]+)\])?\s+dBm`)
	txBitrateRegex := regexp.MustCompile(`tx bitrate:\s+([\d.]+)\s+MBit/s\s+(\S+)`)
	rxBitrateRegex := regexp.MustCompile(`rx bitrate:\s+([\d.]+)\s+MBit/s\s+(\S+)`)
	txRetriesRegex := regexp.MustCompile(`tx retries:\s+(\d+)`)
	txFailedRegex := regexp.MustCompile(`tx failed:\s+(\d+)`)
	rxBytesRegex := regexp.MustCompile(`rx bytes:\s+(\d+)`)
	txBytesRegex := regexp.MustCompile(`tx bytes:\s+(\d+)`)
	rxPacketsRegex := regexp.MustCompile(`rx packets:\s+(\d+)`)
	txPacketsRegex := regexp.MustCompile(`tx packets:\s+(\d+)`)
	connectedTimeRegex := regexp.MustCompile(`connected time:\s+(\d+)\s+seconds`)
	lastAckSignalRegex := regexp.MustCompile(`last ack signal:\s*([-\d]+)\s+dBm`)

	for _, line := range lines {
		if matches := stationRegex.FindStringSubmatch(line); matches != nil {
			stats["bssid"] = matches[1]
		}
		if matches := signalRegex.FindStringSubmatch(line); matches != nil {
			stats["signal"] = matches[1]
		}
		if matches := signalAvgRegex.FindStringSubmatch(line); matches != nil {
			stats["signal_avg"] = matches[1]
		}
		if matches := txBitrateRegex.FindStringSubmatch(line); matches != nil {
			stats["tx_bitrate"] = matches[1]
			stats["tx_bitrate_info"] = matches[2]
		}
		if matches := rxBitrateRegex.FindStringSubmatch(line); matches != nil {
			stats["rx_bitrate"] = matches[1]
			stats["rx_bitrate_info"] = matches[2]
		}
		if matches := txRetriesRegex.FindStringSubmatch(line); matches != nil {
			stats["tx_retries"] = matches[1]
		}
		if matches := txFailedRegex.FindStringSubmatch(line); matches != nil {
			stats["tx_failed"] = matches[1]
		}
		if matches := rxBytesRegex.FindStringSubmatch(line); matches != nil {
			stats["rx_bytes"] = matches[1]
		}
		if matches := txBytesRegex.FindStringSubmatch(line); matches != nil {
			stats["tx_bytes"] = matches[1]
		}
		if matches := rxPacketsRegex.FindStringSubmatch(line); matches != nil {
			stats["rx_packets"] = matches[1]
		}
		if matches := txPacketsRegex.FindStringSubmatch(line); matches != nil {
			stats["tx_packets"] = matches[1]
		}
		if matches := connectedTimeRegex.FindStringSubmatch(line); matches != nil {
			stats["connected_time"] = matches[1]
		}
		if matches := lastAckSignalRegex.FindStringSubmatch(line); matches != nil {
			stats["last_ack_signal"] = matches[1]
		}
	}

	return stats, nil
}

// Helper functions

func frequencyToChannel(freq int) int {
	if freq >= 2412 && freq <= 2484 {
		if freq == 2484 {
			return 14
		}
		return (freq - 2407) / 5
	}
	if freq >= 5170 && freq <= 5825 {
		return (freq - 5000) / 5
	}
	if freq >= 5955 && freq <= 7115 {
		if freq == 5935 || freq == 5955 {
			return 2
		}
		if freq == 5965 || freq == 5985 {
			return 6
		}
		return (freq - 5950) / 5
	}
	return 0
}

func signalToQuality(signal int) int {
	if signal >= -30 {
		return 100
	}
	if signal <= -100 {
		return 0
	}
	return int((float64(signal+100) / 70.0) * 100)
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// parseBitrateInfo extracts WiFi standard, channel width, and MIMO config from bitrate string
func parseBitrateInfo(bitrateInfo string) (wifiStandard, channelWidth, mimoConfig string) {
	wifiStandard = "802.11"
	channelWidth = "20"
	mimoConfig = "1x1"

	if strings.Contains(bitrateInfo, "HE") {
		wifiStandard = "WiFi 6 (802.11ax)"
	} else if strings.Contains(bitrateInfo, "VHT") {
		wifiStandard = "WiFi 5 (802.11ac)"
	} else if strings.Contains(bitrateInfo, "HT") {
		wifiStandard = "WiFi 4 (802.11n)"
	} else {
		wifiStandard = "Legacy (802.11a/b/g)"
	}

	if strings.Contains(bitrateInfo, "320MHz") {
		channelWidth = "320"
	} else if strings.Contains(bitrateInfo, "160MHz") || strings.Contains(bitrateInfo, "80+80") {
		channelWidth = "160"
	} else if strings.Contains(bitrateInfo, "80MHz") {
		channelWidth = "80"
	} else if strings.Contains(bitrateInfo, "40MHz") {
		channelWidth = "40"
	}

	if strings.Contains(bitrateInfo, "HE-NSS 4") || strings.Contains(bitrateInfo, "VHT-NSS 4") {
		mimoConfig = "4x4"
	} else if strings.Contains(bitrateInfo, "HE-NSS 3") || strings.Contains(bitrateInfo, "VHT-NSS 3") {
		mimoConfig = "3x3"
	} else if strings.Contains(bitrateInfo, "HE-NSS 2") || strings.Contains(bitrateInfo, "VHT-NSS 2") {
		mimoConfig = "2x2"
	} else if strings.Contains(bitrateInfo, "HE-NSS 1") || strings.Contains(bitrateInfo, "VHT-NSS 1") {
		mimoConfig = "1x1"
	}

	return
}
