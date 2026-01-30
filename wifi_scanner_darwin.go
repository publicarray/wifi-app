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
	parser           ScanParser
}

func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

	return &darwinScanner{
		ouiLookup: ouiLookup,
		parser:    &airportParser{ouiLookup: ouiLookup},
	}
}

func (s *darwinScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
	cmd := exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-s", "-x")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to scan networks: %w (output: %s)", err, string(output))
	}

	if s.parser == nil {
		return nil, fmt.Errorf("no scan parser configured")
	}
	return s.parser.ParseScan(output)
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
	if s.parser == nil {
		return map[string]string{"connected": "false"}, fmt.Errorf("no station parser configured")
	}
	return s.parser.ParseStation(output)
}

func (s *darwinScanner) GetLinkInfo(iface string) (map[string]string, error) {
	cmd := exec.Command("/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport", "-I")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]string{"connected": "false"}, fmt.Errorf("failed to get link info: %w", err)
	}
	if s.parser == nil {
		return map[string]string{"connected": "false"}, fmt.Errorf("no link parser configured")
	}
	return s.parser.ParseLink(output)
}

func (s *darwinScanner) Close() error {
	return nil
}
