//go:build linux && mdlayher

package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mdlayher/wifi"
)

type WiFiScannerMDLayher struct {
	client    *wifi.Client
	ouiLookup *OUILookup
}

func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

	client, err := wifi.New()
	if err != nil {
		panic(fmt.Sprintf("failed to create wifi client: %v", err))
	}

	return &WiFiScannerMDLayher{
		client:    client,
		ouiLookup: ouiLookup,
	}
}

func (s *WiFiScannerMDLayher) GetInterfaces() ([]string, error) {
	interfaces, err := s.client.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	var ifaces []string
	for _, iface := range interfaces {
		ifaces = append(ifaces, iface.Name)
	}

	if len(ifaces) == 0 {
		return nil, fmt.Errorf("no WiFi interfaces found")
	}

	return ifaces, nil
}

func (s *WiFiScannerMDLayher) ScanNetworks(iface string) ([]AccessPoint, error) {
	interfaces, err := s.client.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	var targetIface *wifi.Interface
	for _, i := range interfaces {
		if i.Name == iface {
			targetIface = i
			break
		}
	}

	if targetIface == nil {
		return nil, fmt.Errorf("interface %s not found", iface)
	}

	bss, err := s.client.BSS(targetIface)
	if err != nil {
		return nil, fmt.Errorf("failed to scan BSS: %w", err)
	}

	if bss == nil {
		return []AccessPoint{}, nil
	}

	aps := s.convertBSSToAccessPoint(bss)
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
		aps[i].MaxTheoreticalSpeed = calculateMaxTheoreticalSpeed(&aps[i])
		aps[i].RealWorldSpeed = calculateRealWorldSpeed(aps[i].MaxTheoreticalSpeed)

		if aps[i].Noise != 0 {
			aps[i].SNR = aps[i].Signal - aps[i].Noise
		}

		hasHE := false
		for _, cap := range aps[i].Capabilities {
			if cap == "HE" {
				hasHE = true
				break
			}
		}
		aps[i].EstimatedRange = calculateEstimatedRange(aps[i].TxPower, aps[i].Band, hasHE)
	}

	return aps, nil
}

func (s *WiFiScannerMDLayher) convertBSSToAccessPoint(bss *wifi.BSS) []AccessPoint {
	ap := AccessPoint{
		SSID:         bss.BSSID.String(),
		Vendor:       "", // OUI lookup would go here
		LastSeen:     time.Now().Add(-bss.LastSeen),
		Capabilities: []string{},
		TxPower:      0, // Initialize to 0
	}

	ap.SSID = bss.SSID

	ap.Frequency = bss.Frequency
	ap.Channel = frequencyToChannel(ap.Frequency)
	ap.Signal = int(bss.Signal)
	ap.SignalQuality = signalToQuality(int(bss.Signal))
	ap.BeaconInt = int(bss.BeaconInterval.Seconds() / 0.1024)

	ap.ChannelWidth = 20
	ap.Band = "2.4GHz"
	if ap.Frequency > 5900 {
		ap.Band = "6GHz"
	} else if ap.Frequency > 5000 {
		ap.Band = "5GHz"
	}

	ap.MIMOStreams = 1

	ap.BSSLoadStations = -1
	ap.BSSLoadUtilization = -1

	if bss.Load.StationCount > 0 {
		ap.BSSLoadStations = int(bss.Load.StationCount)
	}
	if bss.Load.ChannelUtilization > 0 {
		ap.BSSLoadUtilization = int(bss.Load.ChannelUtilization)
	}

	return []AccessPoint{ap}
}

func (s *WiFiScannerMDLayher) GetLinkInfo(iface string) (map[string]string, error) {
	info := make(map[string]string)

	interfaces, err := s.client.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	var targetIface *wifi.Interface
	for _, i := range interfaces {
		if i.Name == iface {
			targetIface = i
			break
		}
	}

	if targetIface == nil || targetIface.Type != wifi.InterfaceTypeStation {
		info["connected"] = "false"
		return info, nil
	}

	stations, err := s.client.StationInfo(targetIface)
	if err != nil {
		info["connected"] = "false"
		return info, nil
	}

	if len(stations) == 0 {
		info["connected"] = "false"
		return info, nil
	}

	station := stations[0]
	info["connected"] = "true"
	info["bssid"] = station.HardwareAddr.String()
	info["signal"] = fmt.Sprintf("%d", station.Signal)
	info["signal_avg"] = fmt.Sprintf("%d", station.SignalAverage)
	info["rx_bytes"] = fmt.Sprintf("%d", station.ReceivedBytes)
	info["tx_bytes"] = fmt.Sprintf("%d", station.TransmittedBytes)
	info["rx_packets"] = fmt.Sprintf("%d", station.ReceivedPackets)
	info["tx_packets"] = fmt.Sprintf("%d", station.TransmittedPackets)
	info["rx_bitrate"] = fmt.Sprintf("%.1f", float64(station.ReceiveBitrate)/1e6)
	info["tx_bitrate"] = fmt.Sprintf("%.1f", float64(station.TransmitBitrate)/1e6)
	info["tx_retries"] = fmt.Sprintf("%d", station.TransmitRetries)
	info["tx_failed"] = fmt.Sprintf("%d", station.TransmitFailed)
	info["connected_time"] = fmt.Sprintf("%d", int(station.Connected.Seconds()))

	return info, nil
}

func (s *WiFiScannerMDLayher) GetStationStats(iface string) (map[string]string, error) {
	return s.GetLinkInfo(iface)
}

func (s *WiFiScannerMDLayher) Close() error {
	return s.client.Close()
}

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

func calculateMaxTheoreticalSpeed(ap *AccessPoint) int {
	var baseSpeed float64
	var hasHE, hasVHT, hasHT bool

	for _, cap := range ap.Capabilities {
		if cap == "HE" {
			hasHE = true
		} else if cap == "VHT" {
			hasVHT = true
		} else if cap == "HT" {
			hasHT = true
		}
	}

	if hasHE {
		baseSpeed = 286.8
	} else if hasVHT {
		baseSpeed = 433.3
	} else if hasHT {
		baseSpeed = 72.2
	} else {
		baseSpeed = 54
	}

	widthMultiplier := 1.0
	switch ap.ChannelWidth {
	case 40:
		widthMultiplier = 2.0
	case 80:
		widthMultiplier = 4.0
	case 160:
		widthMultiplier = 8.0
	case 320:
		widthMultiplier = 16.0
	}

	streamMultiplier := float64(ap.MIMOStreams)
	if streamMultiplier < 1 {
		streamMultiplier = 1
	}

	maxSpeed := baseSpeed * widthMultiplier * streamMultiplier
	return int(maxSpeed)
}

func calculateRealWorldSpeed(theoreticalSpeed int) int {
	return int(float64(theoreticalSpeed) * 0.65)
}

func calculateEstimatedRange(txPower int, band string, hasHE bool) float64 {
	var frequencyMHz float64
	if band == "2.4GHz" {
		frequencyMHz = 2437
	} else if band == "5GHz" {
		frequencyMHz = 5400
	} else {
		frequencyMHz = 2437
	}

	eirp := float64(txPower)
	minSignal := -82.0
	if hasHE {
		minSignal = -87.0
	}

	signalMargin := eirp - minSignal
	adjustment := 20.0 * math.Log10(frequencyMHz/2437.0)
	rangeMeters := math.Pow(10.0, (signalMargin-adjustment)/20.0)

	if rangeMeters < 10 {
		return 10.0
	}
	if rangeMeters > 500 {
		return 500.0
	}

	return rangeMeters
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
