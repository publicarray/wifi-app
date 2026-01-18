package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type darwinScanner struct {
	currentInterface string
	ouiLookup        *OUILookup
}

func newDarwinScanner() IWiFiScanner {
	ouiLookup := NewOUILookup("")
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
	agrExtRSSIRegex := regexp.MustCompile(`agrExtRSSI:\s+(-?\d+)`)
	agrCtlNoiseRegex := regexp.MustCompile(`agrCtlNoise:\s+(-?\d+)`)
	agrExtNoiseRegex := regexp.MustCompile(`agrExtNoise:\s+(-?\d+)`)

	stateRegex := regexp.MustCompile(`state:\s+(\S+)`)
	lastTxRateRegex := regexp.MustCompile(`lastTxRate:\s+(\d+)`)
	maxRateRegex := regexp.MustCompile(`maxRate:\s+(\d+)`)
	lastAssocStatusRegex := regexp.MustCompile(`lastAssocStatus:\s+(\d+)`)
	macRegex := regexp.MustCompile(`\s+([0-9a-f:]{2}:[0-9a-f:]{2}:[0-9a-f:]{2}:[0-9a-f:]{2}:[0-9a-f:]{2}:[0-9a-f:]{2})`)

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

func (s *darwinScanner) GetStationStats(iface string) (StationStats, error) {
	cmd := exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-I")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return StationStats{}, fmt.Errorf("failed to get station stats: %w", err)
	}

	lines := strings.Split(string(output), "\n")

	rxBitrateRegex := regexp.MustCompile(`\s+lastRxRate:\s+(\d+)`)
	txBitrateRegex := regexp.MustCompile(`\s+lastTxRate:\s+(\d+)`)
	rssiRegex := regexp.MustCompile(`\s+agrCtlRSSI:\s+(-?\d+)`)
	agrCtlRSSIRegex := regexp.MustCompile(`\s+agrCtlRSSI:\s+(-?\d+)`)
	agrExtRSSIRegex := regexp.MustCompile(`\s+agrExtRSSI:\s+(-?\d+)`)
	noiseRegex := regexp.MustCompile(`\s+agrCtlNoise:\s+(-?\d+)`)

	stats := StationStats{Connected: true}

	for _, line := range lines {
		if matches := rxBitrateRegex.FindStringSubmatch(line); matches != nil {
			if rate, err := strconv.ParseFloat(matches[1], 64); err == nil {
				stats.RxBitrate = rate
			}
		}
		if matches := txBitrateRegex.FindStringSubmatch(line); matches != nil {
			if rate, err := strconv.ParseFloat(matches[1], 64); err == nil {
				stats.TxBitrate = rate
			}
		}
		if matches := rssiRegex.FindStringSubmatch(line); matches != nil {
			if rssi, err := strconv.Atoi(matches[1]); err == nil {
				stats.Signal = rssi
			}
		}
		if matches := agrCtlRSSIRegex.FindStringSubmatch(line); matches != nil {
			if rssi, err := strconv.Atoi(matches[1]); err == nil {
				stats.SignalAvg = rssi
			}
		}
		if matches := noiseRegex.FindStringSubmatch(line); matches != nil {
			if noise, err := strconv.Atoi(matches[1]); err == nil && stats.Signal != 0 {
				stats.Noise = noise
				stats.SNR = stats.Signal - noise
			}
		}
	}

	stats.WiFiStandard = "802.11ac/n"
	stats.ChannelWidth = 20
	stats.MIMOConfig = "1x1"

	return stats, nil
}

func (s *darwinScanner) Close() error {
	return nil
}
