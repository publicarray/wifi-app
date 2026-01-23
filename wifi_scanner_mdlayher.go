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

// WiFi frequency ranges and channel constants
const (
	// 2.4GHz band channels
	Freq2412MHz        = 2412
	Freq2484MHz        = 2484
	Freq2407MHz        = 2407
	ChannelSpacing5MHz = 5

	// 5GHz band channels
	Freq5170MHz = 5170
	Freq5825MHz = 5825
	Freq5000MHz = 5000

	// 6GHz band channels
	Freq5935MHz = 5935
	Freq5955MHz = 5955
	Freq5965MHz = 5965
	Freq5985MHz = 5985
	Freq5950MHz = 5950
	Freq7115MHz = 7115

	// Channel constants
	Channel14 = 14
	Channel2  = 2
	Channel6  = 6

	// DFS (Dynamic Frequency Selection) channels - 5GHz band
	DFSChannel52  = 52
	DFSChannel56  = 56
	DFSChannel60  = 60
	DFSChannel64  = 64
	DFSChannel100 = 100
	DFSChannel104 = 104
	DFSChannel108 = 108
	DFSChannel112 = 112
	DFSChannel116 = 116
	DFSChannel120 = 120
	DFSChannel124 = 124
	DFSChannel128 = 128
	DFSChannel132 = 132
	DFSChannel136 = 136
	DFSChannel140 = 140
	DFSChannel144 = 144
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		accessPoints[i].DFS = isDFSChannel(accessPoints[i].Channel)
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

	pmfCapable := rsn.Capabilities&0x80 != 0  // Bit 7 = MFPC (Management Frame Protection Capable)
	pmfRequired := rsn.Capabilities&0x40 != 0 // Bit 6 = MFPR (Management Frame Protection Required)

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
	if freq >= Freq2412MHz && freq <= Freq2484MHz {
		if freq == Freq2484MHz {
			return Channel14
		}
		return (freq - Freq2407MHz) / ChannelSpacing5MHz
	}
	if freq >= Freq5170MHz && freq <= Freq5825MHz {
		return (freq - Freq5000MHz) / ChannelSpacing5MHz
	}
	if freq >= Freq5955MHz && freq <= Freq7115MHz {
		if freq == Freq5935MHz || freq == Freq5955MHz {
			return Channel2
		}
		if freq == Freq5965MHz || freq == Freq5985MHz {
			return Channel6
		}
		return (freq - Freq5950MHz) / ChannelSpacing5MHz
	}
	return 0
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
	const (
		CenterFreq24GHz        = 2437
		CenterFreq5GHz         = 5400
		DefaultSignalThreshold = -82.0
		HESignalThreshold      = -87.0
		MinRangeMeters         = 10.0
		MaxRangeMeters         = 500.0
		PathLossConstant       = 20.0
		ReferenceFrequency     = 2437.0
	)

	var frequencyMHz float64
	if band == "2.4GHz" {
		frequencyMHz = CenterFreq24GHz
	} else if band == "5GHz" {
		frequencyMHz = CenterFreq5GHz
	} else {
		frequencyMHz = CenterFreq24GHz
	}

	eirp := float64(txPower)
	minSignal := DefaultSignalThreshold
	if hasHE {
		minSignal = HESignalThreshold
	}

	signalMargin := eirp - minSignal
	adjustment := PathLossConstant * math.Log10(frequencyMHz/ReferenceFrequency)
	rangeMeters := math.Pow(10.0, (signalMargin-adjustment)/PathLossConstant)

	if rangeMeters < MinRangeMeters {
		return MinRangeMeters
	}
	if rangeMeters > MaxRangeMeters {
		return MaxRangeMeters
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
		case 61:
			parseHTOperation(ie.Data, ap)
		case 70:
			parseRMCapabilities(ie.Data, ap)
		case 191:
			parseVHTCapabilities(ie.Data, ap)
		case 192:
			parseVHTOperation(ie.Data, ap)
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
	// HT Capabilities IE (ID 45) per IEEE 802.11-2020 section 9.4.2.55
	// Structure: HT Capability Info (2) + A-MPDU Params (1) + Supported MCS Set (16) + ...
	if len(data) < 3 {
		return
	}

	ap.Capabilities = appendUnique(ap.Capabilities, "HT")

	// HT Capability Info (bytes 0-1), little-endian
	capabilities := uint16(data[0]) | uint16(data[1])<<8

	// Bit 1: Supported Channel Width Set (0=20MHz only, 1=20MHz and 40MHz)
	if (capabilities & 0x02) != 0 {
		if ap.ChannelWidth < 40 {
			ap.ChannelWidth = 40
		}
	}

	// Supported MCS Set is at bytes 3-18 (16 bytes)
	// Bytes 3-12: RX MCS Bitmask (10 bytes = 80 bits for MCS 0-79)
	// Each spatial stream supports MCS 0-7, so 8 MCS indexes per stream
	if len(data) >= 7 {
		rxMcsBitmask := data[3:7]
		maxStream := 0
		for ss := 0; ss < 4; ss++ {
			if rxMcsBitmask[ss] != 0 {
				maxStream = ss + 1
			}
		}
		if maxStream > ap.MIMOStreams {
			ap.MIMOStreams = maxStream
		}
	}
}

func parseVHTCapabilities(data []byte, ap *AccessPoint) {
	// VHT Capabilities IE (ID 191) per IEEE 802.11-2020 section 9.4.2.157
	// Structure: VHT Capabilities Info (4 bytes) + VHT Supported MCS Set (8 bytes) = 12 bytes
	if len(data) < 12 {
		return
	}

	ap.Capabilities = appendUnique(ap.Capabilities, "VHT")

	// VHT Capabilities Info (bytes 0-3)
	// Bits 2-3: Supported Channel Width Set
	chanWidthSet := (data[0] >> 2) & 0x03
	switch chanWidthSet {
	case 1:
		ap.ChannelWidth = 160
	case 2:
		ap.ChannelWidth = 160
	}

	// Bits 19: SU Beamformer, Bit 20: SU Beamformee, Bits 21-22: MU Beamformer
	if (data[2] & 0x08) != 0 {
		ap.MUMIMO = true
	}

	// VHT Supported MCS Set (bytes 4-11)
	// Bytes 4-5: RX MCS Map (16 bits, 2 bits per spatial stream)
	// 0b11 = not supported, 0b00-0b10 = supported
	rxMcsMap := uint16(data[4]) | uint16(data[5])<<8

	maxStream := 0
	for ss := 0; ss < 8; ss++ {
		mcsVal := (rxMcsMap >> (ss * 2)) & 0x03
		if mcsVal != 3 {
			maxStream = ss + 1
		}
	}

	if maxStream > ap.MIMOStreams {
		ap.MIMOStreams = maxStream
	}
}

func parseHECapabilities(data []byte, ap *AccessPoint) {
	// HE Capabilities IE is Element ID 255 with Extension ID 35
	// The first byte should be the extension ID
	if len(data) < 1 {
		return
	}

	extID := data[0]
	if extID == 35 {
		parseHECapabilitiesElement(data[1:], ap)
	} else if extID == 36 {
		parseHEOperation(data[1:], ap)
	}
}

func parseHECapabilitiesElement(data []byte, ap *AccessPoint) {
	// HE Capabilities Element per IEEE 802.11ax-2021 section 9.4.2.242
	// Structure: HE MAC Caps (6 bytes) + HE PHY Caps (11 bytes) + Supported MCS and NSS Set (variable) + PPE Thresholds
	if len(data) < 17 {
		return
	}

	ap.Capabilities = appendUnique(ap.Capabilities, "HE")

	// HE MAC Capabilities (bytes 0-5)
	// Bit 11: TWT Responder Support
	macCap0 := data[0]
	macCap1 := data[1]
	if (macCap1 & 0x08) != 0 {
		ap.TWTSupport = true
	}
	_ = macCap0

	// HE PHY Capabilities (bytes 6-16)
	phyCap := data[6:17]

	// Byte 8, Bits 3-4: MU Beamformer
	if (phyCap[2] & 0x10) != 0 {
		ap.MUMIMO = true
	}

	// TX 1024-QAM: Byte 10, Bit 3
	// RX 1024-QAM: Byte 10, Bit 4
	if len(phyCap) >= 5 && ((phyCap[4]&0x08) != 0 || (phyCap[4]&0x10) != 0) {
		ap.QAMSupport = 1024
	}

	// OBSS PD-based SR: Byte 9, Bit 4
	if len(phyCap) >= 4 && (phyCap[3]&0x10) != 0 {
		ap.OBSSPD = true
	}

	// Supported HE-MCS And NSS Set (bytes 17+)
	// RX HE-MCS Map: 2 bytes, 2 bits per spatial stream
	if len(data) >= 19 {
		rxMcsMap := uint16(data[17]) | uint16(data[18])<<8
		maxStream := 0
		for ss := 0; ss < 8; ss++ {
			mcsVal := (rxMcsMap >> (ss * 2)) & 0x03
			if mcsVal != 3 {
				maxStream = ss + 1
			}
		}
		if maxStream > ap.MIMOStreams {
			ap.MIMOStreams = maxStream
		}
	}
}

func parseHEOperation(data []byte, ap *AccessPoint) {
	// HE Operation Element per IEEE 802.11ax-2021 section 9.4.2.243
	// Structure: HE Operation Parameters (3) + BSS Color Information (1) + Basic HE-MCS and NSS Set (2) + ...
	if len(data) < 4 {
		return
	}
	// BSS Color Information is at byte 3
	// Bits 0-5: BSS Color (0-63)
	ap.BSSColor = int(data[3] & 0x3F)
}

func parseTPCReport(data []byte, ap *AccessPoint) {
	// TPC Report IE (ID 38) per IEEE 802.11-2020 section 9.4.2.16
	// Structure: TX Power (1 byte) + Link Margin (1 byte) = 2 bytes total
	if len(data) < 2 {
		return
	}
	ap.TxPower = int(int8(data[0]))
}

func parseRMCapabilities(data []byte, ap *AccessPoint) {
	// RM Enabled Capabilities IE (ID 70) per IEEE 802.11-2020 section 9.4.2.44
	// 5 bytes of capability flags
	if len(data) < 1 {
		return
	}
	// Byte 0, Bit 1: Neighbor Report capability (802.11k)
	if (data[0] & 0x02) != 0 {
		ap.NeighborReport = true
	}
}

func parseHTOperation(data []byte, ap *AccessPoint) {
	// HT Operation IE (ID 61) per IEEE 802.11-2020 section 9.4.2.56
	if len(data) < 2 {
		return
	}
	// Byte 1 contains STA Channel Width and secondary channel offset
	staChanWidth := (data[1] & 0x04) >> 2
	if staChanWidth == 1 {
		if ap.ChannelWidth < 40 {
			ap.ChannelWidth = 40
		}
	}
}

func parseVHTOperation(data []byte, ap *AccessPoint) {
	// VHT Operation IE (ID 192) per IEEE 802.11-2020 section 9.4.2.158
	if len(data) < 1 {
		return
	}
	// Byte 0: Channel Width field
	// 0 = 20 or 40 MHz, 1 = 80 MHz, 2 = 160 MHz, 3 = 80+80 MHz
	switch data[0] {
	case 1:
		ap.ChannelWidth = 80
	case 2:
		ap.ChannelWidth = 160
	case 3:
		ap.ChannelWidth = 160
	}
}

func parseExtendedCapabilities(data []byte, ap *AccessPoint) {
	// Extended Capabilities IE structure per IEEE 802.11-2020 section 9.4.2.26
	// Capabilities are bit-indexed, with byte 0 containing bits 0-7, byte 1 containing bits 8-15, etc.

	// Byte 0: bits 0-7 (20/40 BSS Coexistence, etc.)
	if len(data) >= 1 {
		// Bit 6: S-PSMP Support (U-APSD coexistence)
		if (data[0] & 0x40) != 0 {
			ap.UAPSD = true
		}
	}

	// Byte 2: bits 16-23
	// Bit 19: BSS Transition (802.11v) = byte 2, bit 3 (0x08)
	if len(data) >= 3 {
		if (data[2] & 0x08) != 0 {
			ap.BSSTransition = true
		}
	}

	// Byte 3: bits 24-31
	// Bit 31: Interworking = byte 3, bit 7 (0x80)
	// Bit 30: QoS Map = byte 3, bit 6 (0x40)

	// Note: Neighbor Report (802.11k) is NOT in Extended Capabilities IE.
	// It's in RM Enabled Capabilities IE (ID 70). See parseRMCapabilities().
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
		if len(data) >= 5 && data[4] == 0x13 {
			ap.APName = string(data[5:])
		}
	}
}

func isDFSChannel(channel int) bool {
	switch channel {
	case DFSChannel52, DFSChannel56, DFSChannel60, DFSChannel64,
		DFSChannel100, DFSChannel104, DFSChannel108, DFSChannel112,
		DFSChannel116, DFSChannel120, DFSChannel124, DFSChannel128,
		DFSChannel132, DFSChannel136, DFSChannel140, DFSChannel144:
		return true
	default:
		return false
	}
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
