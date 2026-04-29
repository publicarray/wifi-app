package main

import (
	"fmt"
	"strings"
)

// 802.11 Information Element parsers.
//
// Each parser takes the IE *body* bytes (i.e. without the 1-byte Element ID
// and 1-byte Length prefix) plus a *AccessPoint to populate. They are pure
// functions: no I/O, no platform-specific types. Both the Linux nl80211
// scanner and the optional macOS helper feed beacon IE bytes through these.
//
// Element ID assignments follow IEEE 802.11-2024 / Linux kernel
// include/linux/ieee80211.h. Element ID Extension assignments live under
// Element ID 255.

// parseInformationElements walks a concatenated IE TLV stream (Element ID +
// Length + body, repeated) and dispatches each element to the matching
// per-IE parser. Truncated trailing elements are silently dropped.
//
// Used by the macOS helper (which receives raw IE bytes from Apple80211) and
// is also useful for unit tests that exercise the parser stack end-to-end.
func parseInformationElements(buf []byte, ap *AccessPoint) {
	for len(buf) >= 2 {
		id := buf[0]
		length := int(buf[1])
		if 2+length > len(buf) {
			return
		}
		body := buf[2 : 2+length]
		dispatchElement(id, body, ap)
		buf = buf[2+length:]
	}
}

// dispatchElement routes a single IE body to the matching parser. Mirrors the
// switch in the linux scanner's parseCapabilitiesIEs.
func dispatchElement(id byte, body []byte, ap *AccessPoint) {
	switch id {
	case 5:
		parseTIM(body, ap)
	case 7:
		parseCountryIE(body, ap)
	case 38:
		parseTPCReport(body, ap)
	case 45:
		parseHTCapabilities(body, ap)
	case 61:
		parseHTOperation(body, ap)
	case 70:
		parseRMCapabilities(body, ap)
	case 127:
		parseExtendedCapabilities(body, ap)
	case 191:
		parseVHTCapabilities(body, ap)
	case 192:
		parseVHTOperation(body, ap)
	case 221:
		parseVendorSpecificIE(body, ap)
	case 255:
		parseHECapabilities(body, ap)
	}
}

func parseHTCapabilities(data []byte, ap *AccessPoint) {
	// HT Capabilities IE (ID 45) per IEEE 802.11-2020 section 9.4.2.55
	// Structure: HT Capability Info (2) + A-MPDU Params (1) + Supported MCS Set (16) + ...
	if len(data) < 3 {
		return
	}

	ap.Capabilities = appendUnique(ap.Capabilities, "HT")

	capabilities := uint16(data[0]) | uint16(data[1])<<8

	// Bit 1: Supported Channel Width Set (0=20MHz only, 1=20MHz and 40MHz)
	if (capabilities & 0x02) != 0 {
		if ap.ChannelWidth < 40 {
			ap.ChannelWidth = 40
		}
	}

	// Supported MCS Set is at bytes 3-18 (16 bytes)
	// Bytes 3-12: RX MCS Bitmask (10 bytes = 80 bits for MCS 0-79)
	// Each spatial stream supports MCS 0-7, so 8 MCS indexes per stream.
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

	// Bits 2-3: Supported Channel Width Set
	chanWidthSet := (data[0] >> 2) & 0x03
	switch chanWidthSet {
	case 1:
		ap.ChannelWidth = 160
	case 2:
		ap.ChannelWidth = 160
	}

	// MU Beamformer indication.
	if (data[2] & 0x08) != 0 {
		ap.MUMIMO = true
	}

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

	rxHighest := int(uint16(data[6]) | uint16(data[7])<<8)
	txHighest := int(uint16(data[10]) | uint16(data[11])<<8)
	if ap.MaxPhyRate == 0 && rxHighest > 0 {
		ap.MaxPhyRate = rxHighest
	}
	if ap.MaxPhyRate == 0 && txHighest > 0 {
		ap.MaxPhyRate = txHighest
	}
}

func parseHECapabilities(data []byte, ap *AccessPoint) {
	// Element ID 255 carries an Extension ID in byte 0 that selects between
	// HE Capabilities (35), HE Operation (36), EHT Operation (106),
	// Multi-Link (107) and EHT Capabilities (108).
	if len(data) < 1 {
		return
	}
	switch data[0] {
	case 35:
		parseHECapabilitiesElement(data[1:], ap)
	case 36:
		parseHEOperation(data[1:], ap)
	case 106:
		parseEHTOperation(data[1:], ap)
	case 107:
		parseMultiLinkElement(data[1:], ap)
	case 108:
		parseEHTCapabilitiesElement(data[1:], ap)
	}
}

func parseHECapabilitiesElement(data []byte, ap *AccessPoint) {
	// HE Capabilities Element per IEEE 802.11ax-2021 section 9.4.2.242
	// Structure: HE MAC Caps (6) + HE PHY Caps (11) + Supported MCS / NSS Set + PPE Thresholds
	if len(data) < 17 {
		return
	}

	ap.Capabilities = appendUnique(ap.Capabilities, "HE")
	ap.Capabilities = appendUnique(ap.Capabilities, "WiFi6")

	// Byte 1, bit 3: TWT Responder Support.
	if (data[1] & 0x08) != 0 {
		ap.TWTSupport = true
	}

	// HE AP supports DL OFDMA by spec; mark true on HE Capabilities IE.
	ap.OFDMADownlink = true
	if len(data) >= 4 && (data[3]&0x04) != 0 {
		ap.OFDMAUplink = true
	}

	phyCap := data[6:17]
	if (phyCap[2] & 0x10) != 0 {
		ap.MUMIMO = true
	}

	if len(phyCap) >= 5 && ((phyCap[4]&0x08) != 0 || (phyCap[4]&0x10) != 0) {
		ap.QAMSupport = 1024
	}

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
		if rate := maxPhyRateFromHEMCS(ap.ChannelWidth, maxHEMCSFromMap(rxMcsMap), maxStream); rate > 0 {
			ap.MaxPhyRate = rate
		}
	}
}

func parseHEOperation(data []byte, ap *AccessPoint) {
	// HE Operation Element per IEEE 802.11ax-2021 section 9.4.2.243.
	// Byte 3, bits 0-5 = BSS Color (0-63).
	if len(data) < 4 {
		return
	}
	ap.BSSColor = int(data[3] & 0x3F)
}

func parseEHTCapabilitiesElement(data []byte, ap *AccessPoint) {
	ap.Capabilities = appendUnique(ap.Capabilities, "WiFi7")

	if ap.QAMSupport < 4096 {
		ap.QAMSupport = 4096
	}

	if len(data) >= 2 && (data[1]&0x02) != 0 && ap.ChannelWidth < 320 {
		ap.ChannelWidth = 320
	}

	if maxMcs := parseEHTMaxMCS(data); maxMcs > 0 {
		streams := ap.MIMOStreams
		if streams <= 0 {
			streams = 1
		}
		if rate := maxPhyRateFromHEMCS(ap.ChannelWidth, maxMcs, streams); rate > 0 {
			ap.MaxPhyRate = rate
		}
	}
}

func parseEHTOperation(data []byte, ap *AccessPoint) {
	_ = data
	_ = ap
}

// parseMultiLinkElement reads a Multi-Link Element (Element ID Extension 107)
// per IEEE 802.11be / 802.11-2024 section 9.4.2.312. A Basic Multi-Link
// Element (Type 0) in a beacon advertises that the AP is part of an MLD
// (Multi-Link Device), i.e. MLO is supported. We don't decode the Common
// Info / Link Info bodies — presence alone is enough for the UI.
func parseMultiLinkElement(data []byte, ap *AccessPoint) {
	if len(data) < 2 {
		return
	}
	if (data[0] & 0x07) == 0 {
		ap.MLO = true
	}
}

func parseTIM(data []byte, ap *AccessPoint) {
	// DTIM count (0), DTIM period (1), bitmap control (2), partial virtual bitmap...
	if len(data) < 2 {
		return
	}
	ap.DTIM = int(data[1])
}

func parseEHTMaxMCS(data []byte) int {
	for i := 2; i+1 < len(data); i++ {
		mcsMap := uint16(data[i]) | uint16(data[i+1])<<8
		if max := maxHEMCSFromMap(mcsMap); max > 0 {
			return max
		}
	}
	return 0
}

func parseTPCReport(data []byte, ap *AccessPoint) {
	// TPC Report IE (ID 38): TX Power (1 byte) + Link Margin (1 byte).
	if len(data) < 2 {
		return
	}
	ap.TxPower = int(int8(data[0]))
}

func parseRMCapabilities(data []byte, ap *AccessPoint) {
	// RM Enabled Capabilities IE (ID 70). Byte 0, bit 1 = Neighbor Report (802.11k).
	if len(data) < 1 {
		return
	}
	if (data[0] & 0x02) != 0 {
		ap.NeighborReport = true
	}
}

func parseHTOperation(data []byte, ap *AccessPoint) {
	// HT Operation IE (ID 61). Byte 1 carries STA Channel Width.
	if len(data) < 2 {
		return
	}
	if (data[1]&0x04)>>2 == 1 {
		if ap.ChannelWidth < 40 {
			ap.ChannelWidth = 40
		}
	}
}

func parseVHTOperation(data []byte, ap *AccessPoint) {
	// VHT Operation IE (ID 192). Byte 0: Channel Width.
	if len(data) < 1 {
		return
	}
	switch data[0] {
	case 1:
		ap.ChannelWidth = 80
	case 2, 3:
		ap.ChannelWidth = 160
	}
}

func parseExtendedCapabilities(data []byte, ap *AccessPoint) {
	// Extended Capabilities IE (ID 127). Bit-indexed, LSB-first per byte.
	if len(data) >= 1 {
		// Bit 6: S-PSMP Support (U-APSD coexistence).
		if (data[0] & 0x40) != 0 {
			ap.UAPSD = true
		}
	}
	if len(data) >= 3 {
		// Bit 19: BSS Transition (802.11v).
		if (data[2] & 0x08) != 0 {
			ap.BSSTransition = true
		}
	}
}

func parseCountryIE(data []byte, ap *AccessPoint) {
	if len(data) < 3 {
		return
	}
	ap.CountryCode = strings.ToUpper(string(data[:3]))
}

func parseVendorSpecificIE(data []byte, ap *AccessPoint) {
	if len(data) < 4 {
		return
	}
	oui := fmt.Sprintf("%02X:%02X:%02X", data[0], data[1], data[2])
	switch oui {
	case "00:50:F2":
		switch data[3] {
		case 0x04:
			ap.WPS = true
		case 0x02:
			ap.QoSSupport = true
		case 0x01:
			if ap.Security == "" {
				ap.Security = "WPA"
			}
		}
	case "00:0F:AC":
		if len(data) >= 5 && data[4] == 0x13 {
			ap.APName = string(data[5:])
		}
	}
}
