package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type darwinScanner struct {
	currentInterface string
	ouiLookup        *OUILookup
}

func newDarwinScanner() WiFiBackend {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	cacheFile := filepath.Join(cacheDir, "wifi-app", "oui.txt")

	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		cacheFile = filepath.Join(os.TempDir(), "oui.txt")
	}

	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

	return &darwinScanner{
		ouiLookup: ouiLookup,
	}
}

func (s *darwinScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
	cmd := exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-s")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to scan networks: %w (output: %s)", err, string(output))
	}

	return s.parseAirportScanOutput(string(output))
}

func (s *darwinScanner) parseAirportScanOutput(output string) ([]AccessPoint, error) {
	lines := strings.Split(output, "\n")
	var aps []AccessPoint
	var currentAP *AccessPoint

	agrCtlRSSIRegex := regexp.MustCompile(`agrCtlRSSI:\s+(-?\d+)`)

	for _, line := range lines {
		if matches := agrCtlRSSIRegex.FindStringSubmatch(line); matches != nil {
			if rssi, err := strconv.Atoi(matches[1]); err == nil {
				if currentAP != nil {
					aps = append(aps, *currentAP)
				}
				currentAP = &AccessPoint{
					Vendor:        s.ouiLookup.LookupVendor(line),
					Signal:        rssi,
					SignalQuality: signalToQuality(rssi),
					LastSeen:      s.parseAirportTime(),
					Capabilities:  []string{},
				}
			}
		}
	}

	if currentAP != nil {
		aps = append(aps, *currentAP)
	}

	for i := range aps {
		aps[i].ChannelWidth = 20
		if aps[i].Band == "" {
			aps[i].Band = "2.4GHz/5GHz"
		}
	}

	return aps, nil
}

func (s *darwinScanner) parseAirportTime() time.Time {
	return time.Now()
}

func (s *darwinScanner) GetInterfaces() ([]string, error) {
	cmd := exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-I")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	macRegex := regexp.MustCompile(`\s+([0-9a-f:]{2}:[0-9a-f:]{2}:[0-9a-f:]{2}:[0-9a-f:]{2}:[0-9a-f:]{2}:[0-9a-f:]{2})`)

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if matches := macRegex.FindStringSubmatch(line); matches != nil {
			return []string{matches[1]}, nil
		}
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
	channelRegex := regexp.MustCompile(`\s+channel:\s+(\d+),\s*(\d+)\s+MHz`)
	rssiRegex := regexp.MustCompile(`\s+agrCtlRSSI:\s+(-?\d+)`)
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
	}

	connInfo.WiFiStandard = "802.11ac/n"
	connInfo.ChannelWidth = 20
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

func (s *darwinScanner) Close() error {
	return nil
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
