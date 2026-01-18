//go:build linux && !iw

package main

import (
	"context"
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
	var targetInterface *wifi.Interface
	for _, i := range interfaces {
		if i.Name == iface {
			targetInterface = i
			break
		}
	}
	if targetInterface == nil {
		return nil, fmt.Errorf("interface %s not found", iface)
	}

	// active scanning wifi networks
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err = s.client.Scan(ctx, targetInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate scan: %w", err)
	}

	bssList, err := s.client.AccessPoints(targetInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to scan BSS: %w", err)
	}
	if bssList == nil || len(bssList) == 0 {
		return []AccessPoint{}, nil
	}
	var accessPoints []AccessPoint
	for _, bss := range bssList {
		ap := s.convertBSSToAccessPoint(bss)
		if len(ap) > 0 {
			accessPoints = append(accessPoints, ap[0])
		}
	}
	noiseFloor := 0
	surveys, err := s.client.SurveyInfo(targetInterface)
	if err == nil && len(surveys) > 0 {
		for _, survey := range surveys {
			if survey.Noise != 0 {
				noiseFloor = survey.Noise
				break
			}
		}
	}
	for i := range accessPoints {
		if accessPoints[i].Security == "" {
			accessPoints[i].Security = "Open"
		}
		if accessPoints[i].ChannelWidth == 0 {
			accessPoints[i].ChannelWidth = 20
		}
		if accessPoints[i].DTIM == 0 {
			accessPoints[i].DTIM = 100
		}
		if accessPoints[i].PMF == "" {
			accessPoints[i].PMF = "Disabled"
		}
		if accessPoints[i].MIMOStreams == 0 {
			accessPoints[i].MIMOStreams = 1
		}
		if accessPoints[i].BSSLoadStations == 0 && accessPoints[i].BSSLoadUtilization == -1 {
			accessPoints[i].BSSLoadStations = -1
			accessPoints[i].BSSLoadUtilization = -1
		}
		if noiseFloor != 0 {
			accessPoints[i].Noise = noiseFloor
			accessPoints[i].SNR = accessPoints[i].Signal - noiseFloor
		}
		accessPoints[i].MaxTheoreticalSpeed = calculateMaxTheoreticalSpeed(&accessPoints[i])
		accessPoints[i].RealWorldSpeed = calculateRealWorldSpeed(accessPoints[i].MaxTheoreticalSpeed)
		hasHE := false
		for _, cap := range accessPoints[i].Capabilities {
			if cap == "HE" {
				hasHE = true
				break
			}
		}
		accessPoints[i].EstimatedRange = calculateEstimatedRange(accessPoints[i].TxPower, accessPoints[i].Band, hasHE)
	}
	return accessPoints, nil
}

func (s *WiFiScannerMDLayher) convertBSSToAccessPoint(bss *wifi.BSS) []AccessPoint {
	ap := AccessPoint{
		SSID:         bss.BSSID.String(),
		Vendor:       s.ouiLookup.LookupVendor(bss.BSSID.String()),
		LastSeen:     time.Now().Add(-bss.LastSeen),
		Capabilities: []string{},
	}

	ap.SSID = bss.SSID

	ap.Frequency = bss.Frequency
	ap.Channel = frequencyToChannel(ap.Frequency)
	ap.Signal = int(bss.Signal / 100)
	ap.SignalQuality = signalToQuality(int(bss.Signal / 100))
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

	if bss.RSN.IsInitialized() {
		s.parseSecurityFromRSN(bss.RSN, &ap)
	} else {
		if ap.Security == "" {
			ap.Security = "Open"
		}
	}

	s.parseCapabilitiesIEs(bss.InformationElements, &ap)

	return []AccessPoint{ap}
}

func (s *WiFiScannerMDLayher) parseSecurityFromRSN(rsn wifi.RSNInfo, ap *AccessPoint) {
	for _, cipher := range rsn.PairwiseCiphers {
		ap.SecurityCiphers = append(ap.SecurityCiphers, cipher.String())
	}

	for _, akm := range rsn.AKMs {
		ap.AuthMethods = append(ap.AuthMethods, akm.String())
	}

	hasSAE := false
	hasWPA3 := false
	hasWPA2 := false
	for _, akm := range rsn.AKMs {
		if akm == wifi.RSNAkmSAE || akm == wifi.RSNAkmFTSAE {
			hasSAE = true
		}
		if akm == wifi.RSNAkm8021XSuiteB || akm == wifi.RSNAkm8021XCNSA ||
			akm == wifi.RSNAkmSAE || akm == wifi.RSNAkmFTSAE ||
			rsn.GroupMgmtCipher == wifi.RSNCipherGCMP128 ||
			rsn.GroupMgmtCipher == wifi.RSNCipherGCMP256 ||
			rsn.GroupMgmtCipher == wifi.RSNCipherBIPGMAC128 ||
			rsn.GroupMgmtCipher == wifi.RSNCipherBIPGMAC256 ||
			rsn.GroupMgmtCipher == wifi.RSNCipherBIPCMAC256 {
			hasWPA3 = true
		}
		if akm == wifi.RSNAkmPSK || akm == wifi.RSNAkmFTPSK ||
			akm == wifi.RSNAkm8021X || akm == wifi.RSNAkmFT8021X ||
			akm == wifi.RSNAkm8021XSHA256 {
			hasWPA2 = true
		}

		if akm == wifi.RSNAkmFTSAE || akm == wifi.RSNAkmFTPSK ||
			akm == wifi.RSNAkmFT8021X || akm == wifi.RSNAkmFT8021XSHA384 ||
			akm == wifi.RSNAkmFTFILSSHA256 || akm == wifi.RSNAkmFTFILSSHA384 ||
			akm == wifi.RSNAkmFTPSKSHA384 {
			ap.FastRoaming = true
		}
	}

	if hasSAE {
		ap.Security = "WPA3"
	} else if hasWPA3 {
		ap.Security = "WPA3-Enterprise"
	} else if hasWPA2 {
		ap.Security = "WPA2"
	}

	pmfCapable := rsn.Capabilities&0x01 != 0
	pmfRequired := rsn.Capabilities&0x02 != 0

	if pmfRequired {
		ap.PMF = "Required"
	} else if pmfCapable {
		ap.PMF = "Optional"
	} else {
		ap.PMF = "Disabled"
	}
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

func parseIEs(b []byte) []wifi.IE {
	var ies []wifi.IE
	for len(b) >= 2 {
		id := b[0]
		length := int(b[1])
		b = b[2:]

		if length > len(b) {
			break
		}

		ies = append(ies, wifi.IE{
			ID:   id,
			Data: b[:length],
		})

		b = b[length:]
	}

	return ies
}

func (s *WiFiScannerMDLayher) parseCapabilitiesIEs(ies []wifi.IE, ap *AccessPoint) {
	for _, ie := range ies {
		switch ie.ID {
		case 45:
			parseHTCapabilities(ie.Data, ap)
		case 191:
			parseVHTCapabilities(ie.Data, ap)
		case 255:
			parseHECapabilities(ie.Data, ap)
		case 38:
			parseTPCReport(ie.Data, ap)
		case 7:
			parseCountryIE(ie.Data, ap)
		case 127:
			parseExtendedCapabilities(ie.Data, ap)
		case 221:
			parseVendorSpecificIE(ie.Data, ap)
		}
	}
}

func parseHTCapabilities(data []byte, ap *AccessPoint) {
	if len(data) < 2 {
		return
	}

	capabilities := uint16(data[0])<<8 | uint16(data[1])

	if (capabilities & 0x0200) != 0 {
		ap.Capabilities = append(ap.Capabilities, "HT")
	}

	channelWidth := (capabilities >> 2) & 0x3
	switch channelWidth {
	case 1:
		ap.ChannelWidth = 40
	case 2:
		ap.ChannelWidth = 40
	case 3:
		ap.ChannelWidth = 20
	}

	if len(data) >= 2 {
		supportedMCS := data[1]
		rxMCS := supportedMCS & 0x7F
		txMCS := (supportedMCS >> 7) & 0x7F

		maxStream := 0
		if rxMCS&0x01 != 0 || txMCS&0x01 != 0 {
			maxStream = 1
		}
		if rxMCS&0x02 != 0 || txMCS&0x02 != 0 {
			maxStream = 2
		}
		if rxMCS&0x04 != 0 || txMCS&0x04 != 0 {
			maxStream = 3
		}
		if rxMCS&0x08 != 0 || txMCS&0x08 != 0 {
			maxStream = 4
		}

		if maxStream > ap.MIMOStreams {
			ap.MIMOStreams = maxStream
		}
	}
}

func parseVHTCapabilities(data []byte, ap *AccessPoint) {
	if len(data) < 2 {
		return
	}

	ap.Capabilities = appendUnique(ap.Capabilities, "VHT")

	if len(data) < 12 {
		return
	}

	channelWidth := data[0] & 0x03
	switch channelWidth {
	case 0:
		ap.ChannelWidth = 20
	case 1:
		ap.ChannelWidth = 40
	case 2:
		ap.ChannelWidth = 80
	case 3:
		ap.ChannelWidth = 160
	}

	maxMCS := data[2]
	txMCS := maxMCS & 0x03
	rxMCS := (maxMCS >> 2) & 0x03

	maxStream := 0
	if txMCS == 3 || rxMCS == 3 {
		maxStream = 4
	} else if txMCS == 2 || rxMCS == 2 {
		maxStream = 3
	} else if txMCS == 1 || rxMCS == 1 {
		maxStream = 2
	} else {
		maxStream = 1
	}

	if maxStream > ap.MIMOStreams {
		ap.MIMOStreams = maxStream
	}

	muMIMO := (data[1] & 0x18) >> 3
	if muMIMO != 0 {
		ap.MUMIMO = true
	}
}

func parseHECapabilities(data []byte, ap *AccessPoint) {
	if len(data) < 4 {
		return
	}

	ap.Capabilities = appendUnique(ap.Capabilities, "HE")

	macCap := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24

	if (macCap & 0x00000800) != 0 {
		ap.TWTSupport = true
	}

	if (macCap & 0x00000008) != 0 {
		ap.BSSTransition = true
	}

	if len(data) >= 8 {
		phyCap := uint32(data[4]) | uint32(data[5])<<8 | uint32(data[6])<<16 | uint32(data[7])<<24
		muMIMO := (phyCap & 0x00000003)
		if muMIMO != 0 {
			ap.MUMIMO = true
		}

		qamSupport := (phyCap >> 2) & 0x03
		if qamSupport == 1 {
			ap.QAMSupport = 1024
		} else if qamSupport == 2 {
			ap.QAMSupport = 4096
		} else if qamSupport == 0 {
			ap.QAMSupport = 256
		}

		if (phyCap & 0x00000008) != 0 {
			ap.OBSSPD = true
		}
	}
}

func parseTPCReport(data []byte, ap *AccessPoint) {
	if len(data) < 9 {
		return
	}

	ap.TxPower = int(int8(data[8]))
}

func parseExtendedCapabilities(data []byte, ap *AccessPoint) {
	if len(data) < 8 {
		return
	}

	if (data[0] & 0x40) != 0 {
		ap.UAPSD = true
	}

	if (data[1] & 0x01) != 0 {
		ap.BSSTransition = true
	}

	if (data[1] & 0x04) != 0 {
		ap.NeighborReport = true
	}

	if (data[7] & 0x80) != 0 {
		ap.PMF = "Required"
	} else if (data[7] & 0x40) != 0 {
		ap.PMF = "Optional"
	} else {
		ap.PMF = "Disabled"
	}
}

func parseCountryIE(data []byte, ap *AccessPoint) {
	if len(data) < 3 {
		return
	}

	country := string(data[:3])
	ap.CountryCode = strings.ToUpper(country)
}

func parseVendorSpecificIE(data []byte, ap *AccessPoint) {
	if len(data) < 4 {
		return
	}

	oui := data[0:3]

	ouiString := fmt.Sprintf("%02X:%02X:%02X", oui[0], oui[1], oui[2])
	wpaOUI := "00:50:F2"
	msOUI := "00:0F:AC"

	if ouiString == wpaOUI {
		if len(data) >= 4 && data[3] == 0x04 {
			ap.WPS = true
		} else if len(data) >= 4 && data[3] == 0x02 {
			ap.QoSSupport = true
		} else if len(data) >= 4 && data[3] == 0x01 {
			if ap.Security == "" {
				ap.Security = "WPA"
			}
		}
	} else if ouiString == msOUI {
		if len(data) >= 5 {
			ieType := data[4]
			if ieType == 0x4A && len(data) >= 6 {
				ap.BSSColor = int(data[5])
			} else if ieType == 0x13 {
				ap.APName = string(data[5:])
			}
		}
	}
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
