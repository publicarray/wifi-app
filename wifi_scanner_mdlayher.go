//go:build linux && !iw

package main

import (
	"context"
	"fmt"
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
	}
	return accessPoints, nil
}

func (s *WiFiScannerMDLayher) convertBSSToAccessPoint(bss *wifi.BSS) []AccessPoint {
	ap := AccessPoint{
		SSID:         bss.SSID,
		BSSID:        bss.BSSID.String(),
		Vendor:       s.ouiLookup.LookupVendor(bss.BSSID.String()),
		LastSeen:     time.Now().Add(-bss.LastSeen),
		Capabilities: []string{},
	}

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

	// Lookup SSID and frequency from BSS scan results
	bssList, err := s.client.AccessPoints(targetIface)
	if err == nil && len(bssList) > 0 {
		for _, bss := range bssList {
			if bss.BSSID.String() == station.HardwareAddr.String() {
				info["ssid"] = bss.SSID
				info["frequency"] = fmt.Sprintf("%d", bss.Frequency)
				break
			}
		}
	}

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

	retryRate := 0.0
	if station.TransmittedPackets > 0 {
		retryRate = float64(station.TransmitRetries) / float64(station.TransmittedPackets) * 100.0
		if retryRate > 100.0 {
			retryRate = 100.0
		}
	}
	info["retry_rate"] = fmt.Sprintf("%.2f", retryRate)

	return info, nil
}

func (s *WiFiScannerMDLayher) GetStationStats(iface string) (map[string]string, error) {
	return s.GetLinkInfo(iface)
}

func (s *WiFiScannerMDLayher) Close() error {
	return s.client.Close()
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
	ap.Capabilities = appendUnique(ap.Capabilities, "WiFi5")

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
	switch extID {
	case 35:
		parseHECapabilitiesElement(data[1:], ap)
	case 36:
		parseHEOperation(data[1:], ap)
	case 106: // EHT Capabilities (WiFi 7)
		parseEHTCapabilitiesElement(data[1:], ap)
	case 107: // EHT Operation (WiFi 7)
		parseEHTOperation(data[1:], ap)
	}
}

func parseHECapabilitiesElement(data []byte, ap *AccessPoint) {
	// HE Capabilities Element per IEEE 802.11ax-2021 section 9.4.2.242
	// Structure: HE MAC Caps (6 bytes) + HE PHY Caps (11 bytes) + Supported MCS and NSS Set (variable) + PPE Thresholds
	if len(data) < 17 {
		return
	}

	ap.Capabilities = appendUnique(ap.Capabilities, "HE")
	ap.Capabilities = appendUnique(ap.Capabilities, "WiFi6")

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

func parseEHTCapabilitiesElement(data []byte, ap *AccessPoint) {
	ap.Capabilities = appendUnique(ap.Capabilities, "WiFi7")

	if ap.QAMSupport < 4096 {
		ap.QAMSupport = 4096
	}

	// EHT PHY Capabilities: byte 1, bit 1 indicates 320MHz support
	if len(data) >= 2 && (data[1]&0x02) != 0 && ap.ChannelWidth < 320 {
		ap.ChannelWidth = 320
	}
}

func parseEHTOperation(data []byte, ap *AccessPoint) {
	_ = data
	_ = ap
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
