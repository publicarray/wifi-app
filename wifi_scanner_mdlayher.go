//go:build linux

package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mdlayher/wifi"
)

type WiFiScannerNL80211 struct {
	client    *wifi.Client
	ouiLookup *OUILookup
	parser    *mdlayherParser
	initErr   error
}

const activeScanTimeout = 20 * time.Second

func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

	client, err := wifi.New()
	if err != nil {
		return &WiFiScannerNL80211{
			client:    nil,
			ouiLookup: ouiLookup,
			parser:    &mdlayherParser{ouiLookup: ouiLookup},
			initErr:   fmt.Errorf("failed to create wifi client: %w", err),
		}
	}

	return &WiFiScannerNL80211{
		client:    client,
		ouiLookup: ouiLookup,
		parser:    &mdlayherParser{ouiLookup: ouiLookup},
		initErr:   nil,
	}
}

func (s *WiFiScannerNL80211) GetInterfaces() ([]string, error) {
	if s.initErr != nil {
		return nil, s.initErr
	}
	interfaces, err := s.client.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	// nl80211 returns every virtual interface attached to a radio, including
	// p2p-dev-* (WiFi Direct), AP-mode, monitor, etc. Put Station-mode
	// (managed client) interfaces first so the frontend's default-select-first
	// behaviour lands on something that can actually scan. Other modes are
	// kept as a fallback in case the user's setup has no Station interface.
	var stations, others []string
	for _, iface := range interfaces {
		if iface.Name == "" {
			continue
		}
		if iface.Type == wifi.InterfaceTypeStation {
			stations = append(stations, iface.Name)
		} else {
			others = append(others, iface.Name)
		}
	}

	ifaces := append(stations, others...)
	if len(ifaces) == 0 {
		return nil, fmt.Errorf("no WiFi interfaces found")
	}
	return ifaces, nil
}

func (s *WiFiScannerNL80211) ScanNetworks(iface string) ([]AccessPoint, error) {
	if s.initErr != nil {
		return nil, s.initErr
	}
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

	ctx, cancel := context.WithTimeout(context.Background(), activeScanTimeout)
	defer cancel()
	if err := s.client.Scan(ctx, targetInterface); err != nil {
		// EBUSY on the trigger means someone else (NetworkManager,
		// wpa_supplicant, our own previous scan) has an active scan in flight
		// on this radio. The kernel still has the last scan results cached
		// and AccessPoints() can dump them — slightly stale, but real, and
		// the next tick will usually trigger successfully.
		if !isTransientScanError(err) {
			return nil, fmt.Errorf("failed to initiate scan: %w", err)
		}
		slog.Info("scan trigger transient, using cached BSS dump",
			"event", "scan_ebusy", "interface", iface, "err", err)
	}

	bssList, err := s.client.AccessPoints(targetInterface)
	if err != nil && isTransientScanError(err) {
		time.Sleep(2 * time.Second)
		bssList, err = s.client.AccessPoints(targetInterface)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan BSS: %w", err)
	}
	if len(bssList) == 0 {
		return []AccessPoint{}, nil
	}
	var accessPoints []AccessPoint
	for _, bss := range bssList {
		if s.parser == nil {
			return nil, fmt.Errorf("no parser configured for nl80211 scanner")
		}
		ap := s.parser.convertBSSToAccessPoint(bss)
		if len(ap) > 0 {
			accessPoints = append(accessPoints, ap[0])
		}
	}
	noiseByFreq := map[int]int{}
	busyByFreq := map[int]int{}
	extBusyByFreq := map[int]int{}
	utilByFreq := map[int]int{}
	maxTxPowerByFreq := map[int]int{}
	surveys, err := s.client.SurveyInfo(targetInterface)
	if err == nil && len(surveys) > 0 {
		for _, survey := range surveys {
			if survey.Noise != 0 && survey.Frequency != 0 {
				noiseByFreq[survey.Frequency] = survey.Noise
			}
			if survey.Frequency != 0 {
				if survey.ChannelTime > 0 && survey.ChannelTimeBusy > 0 {
					util := int(float64(survey.ChannelTimeBusy) / float64(survey.ChannelTime) * 100)
					if util > 100 {
						util = 100
					}
					utilByFreq[survey.Frequency] = util
				}
				if survey.ChannelTimeBusy > 0 {
					busyByFreq[survey.Frequency] = int(survey.ChannelTimeBusy / time.Millisecond)
				}
				if survey.ChannelTimeExtBusy > 0 {
					extBusyByFreq[survey.Frequency] = int(survey.ChannelTimeExtBusy / time.Millisecond)
				}
				if survey.MaxTXPower != 0 {
					maxTxPowerByFreq[survey.Frequency] = int(survey.MaxTXPower / 100)
				}
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
		if accessPoints[i].PMF == "" {
			accessPoints[i].PMF = "Disabled"
		}
		if accessPoints[i].MIMOStreams == 0 {
			accessPoints[i].MIMOStreams = 1
		}
		if noise, ok := noiseByFreq[accessPoints[i].Frequency]; ok && noise != 0 {
			accessPoints[i].Noise = noise
			accessPoints[i].SNR = accessPoints[i].Signal - noise
		}
		if util, ok := utilByFreq[accessPoints[i].Frequency]; ok && util > 0 {
			accessPoints[i].SurveyUtilization = util
		}
		if busy, ok := busyByFreq[accessPoints[i].Frequency]; ok && busy > 0 {
			accessPoints[i].SurveyBusyMs = busy
		}
		if extBusy, ok := extBusyByFreq[accessPoints[i].Frequency]; ok && extBusy > 0 {
			accessPoints[i].SurveyExtBusyMs = extBusy
		}
		if maxTx, ok := maxTxPowerByFreq[accessPoints[i].Frequency]; ok && maxTx != 0 {
			accessPoints[i].MaxTxPowerDbm = maxTx
		}
		accessPoints[i].DFS = isDFSChannel(accessPoints[i].Channel)
	}
	return accessPoints, nil
}

func isTransientScanError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "device or resource busy") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "timeout")
}

type mdlayherParser struct {
	ouiLookup *OUILookup
}

func (p *mdlayherParser) convertBSSToAccessPoint(bss *wifi.BSS) []AccessPoint {
	ap := AccessPoint{
		SSID:         bss.SSID,
		BSSID:        bss.BSSID.String(),
		Vendor:       p.ouiLookup.LookupVendor(bss.BSSID.String()),
		LastSeen:     time.Now().Add(-bss.LastSeen),
		Capabilities: []string{},
	}

	ap.Frequency = bss.Frequency
	ap.Channel = frequencyToChannel(ap.Frequency)
	if bss.Signal != 0 {
		ap.Signal = int(bss.Signal / 100)
		ap.SignalQuality = signalToQuality(ap.Signal)
	} else if bss.SignalUnspecified > 0 {
		ap.SignalQuality = int(bss.SignalUnspecified)
	}
	ap.BeaconInt = int(bss.BeaconInterval.Seconds() / 0.1024)

	ap.ChannelWidth = 20
	ap.Band = frequencyToBand(ap.Frequency)

	ap.MIMOStreams = 1

	if bss.Load.StationCount > 0 {
		ap.BSSLoadStations = intPtr(int(bss.Load.StationCount))
	}
	if bss.Load.ChannelUtilization > 0 {
		// Channel utilization IE carries a raw byte (0-255) that maps to 0-100%.
		ap.BSSLoadUtilization = intPtr(int(bss.Load.ChannelUtilization) * 100 / 255)
	}

	if bss.RSN.IsInitialized() {
		p.parseSecurityFromRSN(bss.RSN, &ap)
		if ap.Security == "" {
			ap.Security = "WPA2"
		}
	} else {
		if ap.Security == "" {
			ap.Security = "Open"
		}
	}

	p.parseCapabilitiesIEs(bss.InformationElements, &ap)

	return []AccessPoint{ap}
}

func (p *mdlayherParser) parseSecurityFromRSN(rsn wifi.RSNInfo, ap *AccessPoint) {
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

func (s *WiFiScannerNL80211) GetLinkInfo(iface string) (map[string]string, error) {
	info := make(map[string]string)
	if s.initErr != nil {
		return info, s.initErr
	}

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
	// rx/tx_bitrate_info feeds parseBitrateInfo (wifi_utils.go) so the WiFi
	// standard, channel width, and spatial-stream count surface in
	// ClientStats. We synthesise the iw-style string from the typed RateInfo
	// because the parser is shaped around that text format.
	info["rx_bitrate_info"] = formatRateInfo(station.ReceiveRateInfo)
	info["tx_bitrate_info"] = formatRateInfo(station.TransmitRateInfo)
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

func (s *WiFiScannerNL80211) GetStationStats(iface string) (map[string]string, error) {
	if s.initErr != nil {
		return map[string]string{}, s.initErr
	}
	return s.GetLinkInfo(iface)
}

func (s *WiFiScannerNL80211) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (p *mdlayherParser) parseCapabilitiesIEs(ies []wifi.IE, ap *AccessPoint) {
	for _, ie := range ies {
		dispatchElement(ie.ID, ie.Data, ap)
	}
}


// formatRateInfo renders a RateInfo into a string compatible with
// parseBitrateInfo (wifi_utils.go) — e.g. "80MHz VHT-MCS 9 VHT-NSS 2".
//
// Returns "" when the rate info is empty (e.g. a freshly associated station
// before the first frame). The parser falls back to defaults in that case.
func formatRateInfo(r wifi.RateInfo) string {
	if r.Bitrate == 0 && r.SpatialStreams == 0 && r.MCS == 0 && r.Bandwidth == 0 {
		return ""
	}

	var parts []string

	bw := r.Bandwidth
	if bw == 0 {
		// Fallback: HT40 flag implies 40 MHz; otherwise leave width blank so
		// the parser uses its 20 MHz default.
		if r.Flags&wifi.RateInfoFlagsHT40 != 0 {
			bw = 40
		}
	}
	if bw > 0 {
		parts = append(parts, fmt.Sprintf("%dMHz", bw))
	}

	var prefix string
	switch r.Format {
	case wifi.RateFormatEHT:
		prefix = "EHT"
	case wifi.RateFormatHE:
		prefix = "HE"
	case wifi.RateFormatVHT:
		prefix = "VHT"
	case wifi.RateFormatHT:
		prefix = "HT"
	}
	// Format may be unset on older kernels — fall back to flag bits.
	if prefix == "" {
		switch {
		case r.Flags&wifi.RateInfoFlagsEHT != 0:
			prefix = "EHT"
		case r.Flags&wifi.RateInfoFlagsHE != 0:
			prefix = "HE"
		case r.Flags&wifi.RateInfoFlagsVHT != 0:
			prefix = "VHT"
		case r.Flags&wifi.RateInfoFlagsMCS != 0:
			prefix = "HT"
		}
	}

	if prefix != "" {
		parts = append(parts, fmt.Sprintf("%s-MCS %d", prefix, r.MCS))
		// HT encodes streams in the MCS index (MCS 0-7 = 1ss, 8-15 = 2ss, …)
		// and never advertises an explicit "HT-NSS" token. For VHT/HE/EHT the
		// kernel reports the live spatial-stream count separately.
		if r.SpatialStreams > 0 && prefix != "HT" {
			parts = append(parts, fmt.Sprintf("%s-NSS %d", prefix, r.SpatialStreams))
		}
	}

	return strings.Join(parts, " ")
}
