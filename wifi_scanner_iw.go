//go:build linux && iw

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
func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

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
	beaconRegex := regexp.MustCompile(`beacon interval:\s+(\d+)\s+TUs`)
	txPowerRegex := regexp.MustCompile(`TPC report: TX power:\s+([\d.]+)`)
	widthRegex := regexp.MustCompile(`HT20|HT40|VHT80|VHT160|HE80|HE160|320MHz`)
	wpsRegex := regexp.MustCompile(`WPS:.*Version:\s+([\d.]+)`)
	bssLoadRegex := regexp.MustCompile(`BSS Load:\s*`)
	bssStationCountRegex := regexp.MustCompile(`station count:\s+(\d+)`)
	bssUtilizationRegex := regexp.MustCompile(`channel utilisation:\s+(\d+)/255`)
	mimoStreamsRegex := regexp.MustCompile(`(\d+) streams:\s+MCS`)
	twtRegex := regexp.MustCompile(`TWT|BSR`)
	neighborReportRegex := regexp.MustCompile(`Neighbor Report`)
	cipherRegex := regexp.MustCompile(`Pairwise ciphers: (.+)|Group cipher: (.+)`)
	authRegex := regexp.MustCompile(`Authentication suites: (.+)`)
	bssColorRegex := regexp.MustCompile(`BSS Color: (\d+)`)
	obssPDRegex := regexp.MustCompile(`OBSS PD`)
	muMimoRegex := regexp.MustCompile(`MU Beamformer|MU Beamformee`)
	qosRegex := regexp.MustCompile(`WMM:`)
	qamRegex := regexp.MustCompile(`1024-QAM|4096-QAM`)
	countryRegex := regexp.MustCompile(`Country:\\s+(\\w+)`)
	apNameRegex := regexp.MustCompile(`AP name:\\s*(.+)`)

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

			if matches := beaconRegex.FindStringSubmatch(line); matches != nil {
				beaconInt, _ := strconv.Atoi(matches[1])
				currentAP.BeaconInt = beaconInt
				currentAP.DTIM = beaconInt
			}

			if matches := txPowerRegex.FindStringSubmatch(line); matches != nil {
				txPower, _ := strconv.ParseFloat(matches[1], 64)
				currentAP.TxPower = int(txPower)
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

			if strings.Contains(line, "BSS Transition") {
				currentAP.BSSTransition = true
			}

			if strings.Contains(line, "u-APSD") || strings.Contains(line, "u-apsd") {
				currentAP.UAPSD = true
			}

			if strings.Contains(line, "FT/SAE") || strings.Contains(line, "FT-PSK") {
				currentAP.FastRoaming = true
			}

			if strings.Contains(line, "MFP-required") {
				currentAP.PMF = "Required"
			} else if strings.Contains(line, "MFP-capable") {
				currentAP.PMF = "Optional"
			}

			if matches := wpsRegex.FindStringSubmatch(line); matches != nil {
				currentAP.WPS = true
			}

			if bssLoadRegex.MatchString(line) {
				currentAP.BSSLoadStations = -1
				currentAP.BSSLoadUtilization = -1
			}

			if matches := bssStationCountRegex.FindStringSubmatch(line); matches != nil {
				stations, _ := strconv.Atoi(matches[1])
				currentAP.BSSLoadStations = stations
			}

			if matches := bssUtilizationRegex.FindStringSubmatch(line); matches != nil {
				utilization, _ := strconv.Atoi(matches[1])
				currentAP.BSSLoadUtilization = utilization
			}

			if matches := mimoStreamsRegex.FindStringSubmatch(line); matches != nil {
				streams, _ := strconv.Atoi(matches[1])
				if streams > currentAP.MIMOStreams {
					currentAP.MIMOStreams = streams
				}
			}

			if twtRegex.MatchString(line) {
				currentAP.TWTSupport = true
			}

			if neighborReportRegex.MatchString(line) {
				currentAP.NeighborReport = true
			}

			if matches := cipherRegex.FindStringSubmatch(line); matches != nil {
				cipherText := strings.TrimSpace(matches[1])
				ciphers := strings.Split(cipherText, ",")
				for _, c := range ciphers {
					cipher := strings.TrimSpace(c)
					if cipher != "" {
						currentAP.SecurityCiphers = append(currentAP.SecurityCiphers, cipher)
					}
				}
			}

			if matches := authRegex.FindStringSubmatch(line); matches != nil {
				authText := strings.TrimSpace(matches[1])
				authMethods := strings.Split(authText, ",")
				for _, a := range authMethods {
					auth := strings.TrimSpace(a)
					if auth != "" {
						currentAP.AuthMethods = append(currentAP.AuthMethods, auth)
					}
				}
			}

			if matches := bssColorRegex.FindStringSubmatch(line); matches != nil {
				bssColor, _ := strconv.Atoi(matches[1])
				currentAP.BSSColor = bssColor
			}

			if obssPDRegex.MatchString(line) {
				currentAP.OBSSPD = true
			}

			if muMimoRegex.MatchString(line) {
				currentAP.MUMIMO = true
			}

			if qosRegex.MatchString(line) {
				currentAP.QoSSupport = true
			}

			if qamRegex.MatchString(line) {
				if strings.Contains(line, "1024-QAM") {
					currentAP.QAMSupport = 1024
				} else if strings.Contains(line, "4096-QAM") {
					currentAP.QAMSupport = 4096
				} else {
					currentAP.QAMSupport = 256
				}
			}

			if matches := countryRegex.FindStringSubmatch(line); matches != nil {
				currentAP.CountryCode = strings.ToUpper(matches[1])
			}

			if matches := apNameRegex.FindStringSubmatch(line); matches != nil {
				currentAP.APName = strings.TrimSpace(matches[1])
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
		if aps[i].DTIM == 0 {
			aps[i].DTIM = 100
		}
		if aps[i].PMF == "" {
			aps[i].PMF = "Disabled"
		}
		if aps[i].MIMOStreams == 0 {
			aps[i].MIMOStreams = 1
		}
		if aps[i].BSSLoadStations == 0 && aps[i].BSSLoadUtilization == -1 {
			aps[i].BSSLoadStations = -1
			aps[i].BSSLoadUtilization = -1
		}

		if aps[i].Noise != 0 {
			aps[i].SNR = aps[i].Signal - aps[i].Noise
		}

		aps[i].DFS = isDFSChannel(aps[i].Channel)
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

	if retries, ok := stats["tx_retries"]; ok {
		if packets, ok := stats["tx_packets"]; ok {
			retriesInt, _ := strconv.ParseUint(retries, 10, 64)
			packetsInt, _ := strconv.ParseUint(packets, 10, 64)
			retryRate := 0.0
			if packetsInt > 0 {
				retryRate = float64(retriesInt) / float64(packetsInt) * 100.0
				if retryRate > 100.0 {
					retryRate = 100.0
				}
			}
			stats["retry_rate"] = fmt.Sprintf("%.2f", retryRate)
		}
	}

	return stats, nil
}

func (s *WiFiScanner) Close() error {
	return nil
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

// parseBitrateInfo extracts WiFi standard, channel width, and MIMO config from bitrate string
func parseBitrateInfo(bitrateInfo string) (wifiStandard, channelWidth, mimoConfig string) {
	wifiStandard = "802.11"
	channelWidth = "20"
	mimoConfig = "1x1"

	if strings.Contains(bitrateInfo, "UHR") {
		wifiStandard = "WiFi 8 (802.11bn)"
	} else if strings.Contains(bitrateInfo, "EHT") {
		wifiStandard = "WiFi 7 (802.11be)"
	} else if strings.Contains(bitrateInfo, "HE") {
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
