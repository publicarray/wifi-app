package main

import (
	"encoding/hex"
	"testing"
)

// buildIE produces a single Element ID + Length + body TLV stream.
func buildIE(id byte, body []byte) []byte {
	out := make([]byte, 0, 2+len(body))
	out = append(out, id, byte(len(body)))
	out = append(out, body...)
	return out
}

// concatIEs glues several TLVs back-to-back, the way Apple80211 returns the
// "IE" CFData blob.
func concatIEs(ies ...[]byte) []byte {
	var out []byte
	for _, ie := range ies {
		out = append(out, ie...)
	}
	return out
}

func TestParseInformationElements_HEOperationBSSColor(t *testing.T) {
	// Element ID 255 + Ext-ID 36 (HE Operation). Body bytes:
	//   [0..2] HE Operation Parameters, [3] BSS Color Information.
	// BSS Color value = 0x2A (42); upper bits unused for our purposes.
	heOp := buildIE(255, []byte{
		36, 0x00, 0x00, 0x00, 0x2A, 0x00, 0x00,
	})

	var ap AccessPoint
	parseInformationElements(heOp, &ap)
	if ap.BSSColor != 42 {
		t.Errorf("BSSColor = %d, want 42", ap.BSSColor)
	}
}

func TestParseInformationElements_ExtendedCaps_BSSTransition(t *testing.T) {
	// Extended Capabilities (ID 127). Bit 19 (BSS Transition, 802.11v) lives
	// in byte 2 bit 3 -> 0x08.
	ext := buildIE(127, []byte{0x00, 0x00, 0x08})

	var ap AccessPoint
	parseInformationElements(ext, &ap)
	if !ap.BSSTransition {
		t.Errorf("BSSTransition not detected from byte 2 bit 3")
	}
}

func TestParseInformationElements_RMCapabilities_NeighborReport(t *testing.T) {
	// RM Enabled Capabilities (ID 70). Byte 0 bit 1 -> 0x02 means Neighbor
	// Report (802.11k) is supported.
	rm := buildIE(70, []byte{0x02})

	var ap AccessPoint
	parseInformationElements(rm, &ap)
	if !ap.NeighborReport {
		t.Errorf("NeighborReport not detected")
	}
}

func TestParseInformationElements_TIM_DTIMPeriod(t *testing.T) {
	// TIM (ID 5). Byte 1 = DTIM Period.
	tim := buildIE(5, []byte{0x00, 0x03, 0x00})

	var ap AccessPoint
	parseInformationElements(tim, &ap)
	if ap.DTIM != 3 {
		t.Errorf("DTIM = %d, want 3", ap.DTIM)
	}
}

func TestParseInformationElements_VendorSpecific_WPS(t *testing.T) {
	// WPS lives in a Microsoft vendor-specific IE (OUI 00:50:F2, Type 0x04).
	wps := buildIE(221, []byte{0x00, 0x50, 0xF2, 0x04})

	var ap AccessPoint
	parseInformationElements(wps, &ap)
	if !ap.WPS {
		t.Errorf("WPS not detected from OUI/type tag")
	}
}

func TestParseInformationElements_StreamMixed(t *testing.T) {
	// Real-world beacons concatenate many IEs. Make sure the walker handles
	// a multi-element stream and populates several fields independently.
	stream := concatIEs(
		buildIE(7, []byte{'U', 'S', ' '}),                       // Country = "US "
		buildIE(70, []byte{0x02}),                               // RM: Neighbor Report
		buildIE(127, []byte{0x00, 0x00, 0x08}),                  // Extended Caps: BSS Transition
		buildIE(5, []byte{0x00, 0x02, 0x00}),                    // TIM: DTIM = 2
		buildIE(255, []byte{36, 0x00, 0x00, 0x00, 0x05, 0, 0}),  // HE Operation: BSSColor = 5
	)

	var ap AccessPoint
	parseInformationElements(stream, &ap)

	// CountryCode preserves all 3 IE bytes (the third is the operating-class
	// indicator, often " " or "X" / "I" / "O"). Frontend strips for display.
	if ap.CountryCode != "US " {
		t.Errorf("CountryCode = %q, want %q", ap.CountryCode, "US ")
	}
	if !ap.NeighborReport {
		t.Errorf("NeighborReport not set")
	}
	if !ap.BSSTransition {
		t.Errorf("BSSTransition not set")
	}
	if ap.DTIM != 2 {
		t.Errorf("DTIM = %d, want 2", ap.DTIM)
	}
	if ap.BSSColor != 5 {
		t.Errorf("BSSColor = %d, want 5", ap.BSSColor)
	}
}

func TestParseInformationElements_TruncatedTrailing(t *testing.T) {
	// A trailing element with a length longer than the remaining bytes must
	// be dropped silently — never panic, never partial-parse.
	rm := buildIE(70, []byte{0x02})
	truncated := append(rm, 127, 50, 0x00) // claims 50 bytes, has 1
	var ap AccessPoint
	parseInformationElements(truncated, &ap)
	if !ap.NeighborReport {
		t.Errorf("first IE must still parse before truncated tail")
	}
}

func TestParseInformationElements_BeaconHexFromHelper(t *testing.T) {
	// Sanity-check the helper's expected wire format: hex-decoded IE bytes
	// flow through parseInformationElements unchanged. Synthesises what the
	// macOS helper would emit as ie_hex for a tiny beacon.
	hexStr := "" +
		"0703555320" + // Country IE: ID=7, Len=3, body="US "
		"460102" + //    RM Enabled Capabilities (Neighbor Report)
		"7f03000008" + //Extended Capabilities (BSS Transition)
		"050300020a" //  TIM (DTIM=2)

	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var ap AccessPoint
	parseInformationElements(raw, &ap)
	if ap.CountryCode != "US " || !ap.NeighborReport || !ap.BSSTransition || ap.DTIM != 2 {
		t.Errorf("ap = %+v", ap)
	}
}
