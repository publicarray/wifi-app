package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
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
	Freq5950MHz = 5950
	Freq7115MHz = 7115

	// Channel constants
	Channel14 = 14
	Channel2  = 2

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

// WiFi signal quality constants
const (
	// ExcellentSignal represents the signal threshold (in dBm) above which signal is considered excellent
	// Signals at or better than -30 dBm are treated as maximum quality (100%)
	ExcellentSignal = -30

	// PoorSignal represents the signal threshold (in dBm) below which signal is considered poor
	// Signals at or worse than -100 dBm are treated as minimum quality (0%)
	PoorSignal = -100

	// SignalRangeSize is the total signal range in dBm used for quality calculation
	// This represents the range from PoorSignal (-100 dBm) to ExcellentSignal (-30 dBm)
	SignalRangeSize = 70
)

// signalToQuality converts a WiFi signal strength (in dBm) to a quality percentage (0-100)
//
// The conversion uses a linear mapping:
//
//	-30 dBm or better -> 100% (excellent)
//	-100 dBm or worse -> 0% (poor)
//	Values in between are linearly interpolated
//
// Examples:
//
//	-30 dBm -> 100%
//	-50 dBm -> ~71%
//	-70 dBm -> ~43%
//	-100 dBm -> 0%
//
// This function is shared across all platform-specific WiFi scanners to ensure
// consistent signal quality calculation.
func signalToQuality(signal int) int {
	if signal >= ExcellentSignal {
		return 100
	}
	if signal <= PoorSignal {
		return 0
	}
	// Linear interpolation: map [-100, -30] to [0, 100]
	return int((float64(signal-PoorSignal) / float64(SignalRangeSize)) * 100)
}

// appendUnique adds an item to a slice if it doesn't already exist
// This is useful for avoiding duplicate entries in capabilities arrays
// Returns the modified slice (or the original if item already exists)
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// appendCapped appends v to s and keeps only the last cap elements. The
// returned slice is always a freshly-allocated backing array when truncation
// occurs so concurrent readers of the previous slice header are never
// affected by the write.
func appendCapped[T any](s []T, v T, cap int) []T {
	s = append(s, v)
	if len(s) <= cap {
		return s
	}
	out := make([]T, cap)
	copy(out, s[len(s)-cap:])
	return out
}

// intPtr returns a pointer to a copy of v. Used to populate optional numeric
// fields whose absence should serialize as JSON null (e.g. BSSLoad IE fields).
func intPtr(v int) *int {
	return &v
}

// saturatingSubUint64 returns a-b, clamped at 0. Interface byte/packet
// counters can reset (driver restart, sleep/resume); a plain unsigned
// subtraction would wrap to ~1.8e19 and wreck the traffic display.
func saturatingSubUint64(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}

// normalizeMAC lowercases a colon- or dash-separated MAC address and
// zero-pads each octet. Legacy macOS tools (airport) print unpadded octets
// ("0:1b:63:4:5:6") which would otherwise fail OUI lookups and BSSID
// matching. Returns "" when the input is not a 6-octet MAC.
func normalizeMAC(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "-", ":")))
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return ""
	}
	var b strings.Builder
	for i, p := range parts {
		if len(p) == 1 {
			p = "0" + p
		}
		if len(p) != 2 || !isHexDigit(p[0]) || !isHexDigit(p[1]) {
			return ""
		}
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(p)
	}
	return b.String()
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}

// capsMatchAny reports whether any capability token equals (case-insensitive)
// one of the given keys. Exact token comparison — substring matching caused
// false positives (e.g. "EHT" containing "HT").
func capsMatchAny(capabilities []string, keys ...string) bool {
	for _, c := range capabilities {
		c = strings.TrimSpace(c)
		for _, k := range keys {
			if strings.EqualFold(c, k) {
				return true
			}
		}
	}
	return false
}

// NormalizeAccessPoint applies consistent defaults and derived values across platforms.
func NormalizeAccessPoint(ap *AccessPoint) {
	if ap == nil {
		return
	}

	if ap.Frequency > 0 && ap.Channel == 0 {
		ap.Channel = frequencyToChannel(ap.Frequency)
	}
	// 2. Infer Band if missing
	if ap.Band == "" && ap.Frequency > 0 {
		ap.Band = frequencyToBand(ap.Frequency)
	}
	if ap.Signal != 0 && ap.SignalQuality == 0 {
		ap.SignalQuality = signalToQuality(ap.Signal)
	}
	if ap.ChannelWidth == 0 {
		ap.ChannelWidth = 20
	}
	if ap.Security == "" {
		ap.Security = "Open"
	}
	if ap.PMF == "" {
		ap.PMF = "Disabled"
	}
	normalizeCapabilities(ap)
	if ap.CountryCode != "" {
		ap.CountryCode = strings.ToUpper(strings.TrimSpace(ap.CountryCode))
	}
	if len(ap.SecurityCiphers) > 0 {
		normalized := make([]string, 0, len(ap.SecurityCiphers))
		for _, cipher := range ap.SecurityCiphers {
			c := normalizeToken(cipher)
			switch strings.ToUpper(c) {
			case "CCMP-128", "CCMP":
				c = "CCMP"
			case "GCMP-128", "GCMP":
				c = "GCMP"
			}
			if c != "" {
				normalized = appendUnique(normalized, c)
			}
		}
		ap.SecurityCiphers = normalized
	}
	if len(ap.AuthMethods) > 0 {
		normalized := make([]string, 0, len(ap.AuthMethods))
		for _, method := range ap.AuthMethods {
			m := normalizeToken(method)
			if m != "" {
				normalized = appendUnique(normalized, m)
			}
		}
		ap.AuthMethods = normalized
	}
	if ap.MIMOStreams == 0 {
		ap.MIMOStreams = 1
	}
	if ap.MaxPhyRate == 0 {
		ap.MaxPhyRate = estimateMaxPhyRate(ap)
	}
	// Infer the modulation ceiling from the PHY generation when no IE-derived
	// value is present, so all platforms report a comparable QAMSupport.
	if ap.QAMSupport == 0 {
		switch {
		case capsMatchAny(ap.Capabilities, "WiFi7", "EHT"):
			ap.QAMSupport = 4096
		case capsMatchAny(ap.Capabilities, "HE", "WiFi6"):
			ap.QAMSupport = 1024
		case capsMatchAny(ap.Capabilities, "VHT", "WiFi5"):
			ap.QAMSupport = 256
		case capsMatchAny(ap.Capabilities, "HT", "WiFi4"):
			ap.QAMSupport = 64
		}
	}
	if ap.Noise != 0 {
		ap.SNR = ap.Signal - ap.Noise
	}
	ap.DFS = isDFSChannel(ap.Channel)
}

func normalizeToken(value string) string {
	replacer := strings.NewReplacer(
		"\u2010", "-",
		"\u2011", "-",
		"\u2012", "-",
		"\u2013", "-",
		"\u2212", "-",
	)
	return strings.TrimSpace(replacer.Replace(value))
}

func normalizeCapabilities(ap *AccessPoint) {
	if ap == nil {
		return
	}
	if len(ap.Capabilities) == 0 {
		return
	}

	norm := make([]string, 0, len(ap.Capabilities))
	has := func(key string) bool {
		for _, c := range norm {
			if strings.EqualFold(c, key) {
				return true
			}
		}
		return false
	}

	add := func(val string) {
		if val == "" {
			return
		}
		if !has(val) {
			norm = append(norm, val)
		}
	}

	for _, c := range ap.Capabilities {
		clean := strings.TrimSpace(c)
		switch strings.ToLower(clean) {
		case "802.11ax":
			add("HE")
			add("WiFi6")
		case "802.11ac":
			add("VHT")
			add("WiFi5")
		case "802.11n":
			add("HT")
			add("WiFi4")
		case "802.11be":
			add("WiFi7")
		case "802.11a", "802.11b", "802.11g":
			add("Legacy")
		default:
			if strings.HasPrefix(strings.ToLower(clean), "802.11") {
				continue
			}
			add(clean)
		}
	}

	ap.Capabilities = norm
}

func maxPhyRateFromHEMCS(width int, maxMcs int, streams int) int {
	if maxMcs <= 0 || streams <= 0 {
		return 0
	}
	if width <= 0 {
		width = 20
	}
	base := heMcsRate20(maxMcs)
	if base == 0 {
		return 0
	}
	rate := base * (float64(width) / 20.0) * float64(streams)
	return int(math.Round(rate))
}

func heMcsRate20(maxMcs int) float64 {
	switch maxMcs {
	case 0:
		return 8.6
	case 1:
		return 17.2
	case 2:
		return 25.8
	case 3:
		return 34.4
	case 4:
		return 51.6
	case 5:
		return 68.8
	case 6:
		return 77.4
	case 7:
		return 86.0
	case 8:
		return 103.2
	case 9:
		return 120.1
	case 10:
		return 129.0
	case 11:
		return 143.4
	case 12:
		return 154.4
	case 13:
		return 172.1
	default:
		return 0
	}
}

// maxHEMCSFromMap derives the max HE-MCS index encoded in a 2-bits-per-stream
// HE TX/RX MCS map. Used by the windows and mdlayher (nl80211) scanners.
func maxHEMCSFromMap(mcsMap uint16) int {
	maxMcs := 0
	for ss := 0; ss < 8; ss++ {
		mcsVal := (mcsMap >> (ss * 2)) & 0x03
		if mcsVal == 3 {
			continue
		}
		mcs := 7
		switch mcsVal {
		case 1:
			mcs = 9
		case 2:
			mcs = 11
		}
		if mcs > maxMcs {
			maxMcs = mcs
		}
	}
	return maxMcs
}

func estimateMaxPhyRate(ap *AccessPoint) int {
	if ap == nil {
		return 0
	}

	streams := ap.MIMOStreams
	if streams <= 0 {
		streams = 1
	}

	width := ap.ChannelWidth
	if width == 0 {
		width = 20
	}

	var perStream int
	switch {
	case capsMatchAny(ap.Capabilities, "WiFi7", "EHT"):
		perStream = basePhyRateHE(width)
	case capsMatchAny(ap.Capabilities, "HE", "WiFi6"):
		perStream = basePhyRateHE(width)
	case capsMatchAny(ap.Capabilities, "VHT", "WiFi5"):
		perStream = basePhyRateVHT(width)
	case capsMatchAny(ap.Capabilities, "HT", "WiFi4"):
		perStream = basePhyRateHT(width)
	default:
		perStream = 0
	}

	if perStream == 0 {
		return 0
	}
	return perStream * streams
}

func basePhyRateHT(width int) int {
	switch width {
	case 40:
		return 150
	default:
		return 72
	}
}

func basePhyRateVHT(width int) int {
	switch width {
	case 160:
		return 867
	case 80:
		return 433
	case 40:
		return 200
	default:
		return 87
	}
}

func basePhyRateHE(width int) int {
	switch width {
	case 320:
		return 2402
	case 160:
		return 1201
	case 80:
		return 600
	case 40:
		return 287
	default:
		return 143
	}
}

// NormalizeClientStats applies consistent defaults across platforms.
func NormalizeClientStats(stats *ClientStats) {
	if stats == nil {
		return
	}
	if stats.SignalAvg == 0 && stats.Signal != 0 {
		stats.SignalAvg = stats.Signal
	}
	if stats.ChannelWidth == 0 {
		stats.ChannelWidth = 20
	}
	if stats.WiFiStandard == "" {
		stats.WiFiStandard = "Unknown"
	}
	if stats.MIMOConfig == "" {
		stats.MIMOConfig = "1x1"
	}
	if stats.SNR == 0 && stats.Noise != 0 {
		stats.SNR = stats.Signal - stats.Noise
	}
}

// channelToFrequency maps a bare channel number to a center frequency in MHz.
// Channel numbers alone are ambiguous: 1-14 exist in both 2.4 GHz and 6 GHz,
// and 36-165 exist in both 5 GHz and 6 GHz. Without band context we resolve
// to 2.4 GHz / 5 GHz respectively (what every current caller expects); only
// the unambiguous 6 GHz ranges (15-35, 166-233) map to the 6 GHz formula.
// Callers that know the band (e.g. CoreWLAN) must do their own mapping.
func channelToFrequency(channel int) int {
	switch {
	case channel >= 1 && channel <= 14:
		if channel == 14 {
			return 2484
		}
		return 2407 + (channel * 5)
	case channel >= 36 && channel <= 165:
		return 5000 + (channel * 5)
	case channel >= 15 && channel <= 233:
		// 6 GHz: center = 5950 + 5*channel (IEEE 802.11ax Annex E, Table E-4).
		return 5950 + (channel * 5)
	}
	return 0
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

// parseBitrateInfo extracts WiFi standard, channel width, and MIMO config
// from a kernel-style bitrate info string — e.g. "866.7 MBit/s VHT-MCS 9
// 80MHz short GI VHT-NSS 2", as synthesised by formatRateInfo on Linux.
//
// mimoConfig reports the *current frame's* spatial streams (NSS), shown as
// "NxN" because that's how the design notates negotiated MIMO. For HT
// (WiFi 4) NSS is encoded in the MCS index (MCS 0-7 = 1ss, 8-15 = 2ss, …).
// VHT/HE/EHT/UHR carry an explicit "{prefix}-NSS X" token.
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
	} else if strings.Contains(bitrateInfo, "HT") || htMCS(bitrateInfo) >= 0 {
		// Some drivers emit bare "MCS X" with no HT prefix; treat that as HT.
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

	if streams := extractNSS(bitrateInfo); streams > 0 {
		mimoConfig = fmt.Sprintf("%dx%d", streams, streams)
	}

	return
}

// extractNSS returns the number of spatial streams encoded in the bitrate
// info, or 0 if it can't be determined. Handles all formats:
//   - VHT/HE/EHT/UHR: explicit "{prefix}-NSS X" token (1..8)
//   - HT (WiFi 4): MCS index encodes streams as (MCS/8)+1 (so MCS 0-7=1ss,
//     8-15=2ss, 16-23=3ss, 24-31=4ss; MCS 32 is a special "1ss only" mode)
func extractNSS(bitrateInfo string) int {
	for _, prefix := range []string{"EHT-NSS ", "UHR-NSS ", "HE-NSS ", "VHT-NSS "} {
		if v := nssAfter(bitrateInfo, prefix); v > 0 {
			return v
		}
	}

	// HT: parse the MCS index. The format is either "HT-MCS X" or bare "MCS
	// X" when the rate flag is HT — try both. Bare "MCS " must not match
	// inside "VHT-MCS"/"HE-MCS"/"EHT-MCS"/"UHR-MCS"; we guard by rejecting
	// any preceding alnum or '-' character.
	if mcs := htMCS(bitrateInfo); mcs >= 0 {
		if mcs == 32 {
			return 1
		}
		streams := mcs/8 + 1
		if streams < 1 {
			streams = 1
		}
		if streams > 4 {
			streams = 4
		}
		return streams
	}

	return 0
}

func htMCS(s string) int {
	if idx := strings.Index(s, "HT-MCS "); idx >= 0 {
		// Reject "VHT-MCS"/"EHT-MCS" matching at the H of HT-MCS.
		if idx == 0 || !isAlnum(s[idx-1]) {
			return readInt(s[idx+len("HT-MCS "):])
		}
	}
	prefix := "MCS "
	from := 0
	for {
		rel := strings.Index(s[from:], prefix)
		if rel < 0 {
			return -1
		}
		idx := from + rel
		// Bare MCS only — preceding char must not be a letter, digit, or '-'.
		if idx == 0 || (!isAlnum(s[idx-1]) && s[idx-1] != '-') {
			return readInt(s[idx+len(prefix):])
		}
		from = idx + 1
	}
}

func isAlnum(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func nssAfter(s, prefix string) int {
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return 0
	}
	v := readInt(s[idx+len(prefix):])
	if v <= 0 || v > 8 {
		return 0
	}
	return v
}

func readInt(s string) int {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return -1
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return -1
	}
	return n
}

func frequencyToChannel(freq int) int {
	switch {
	case freq >= Freq2412MHz && freq <= Freq2484MHz:
		if freq == Freq2484MHz {
			return Channel14
		}
		return (freq - Freq2407MHz) / ChannelSpacing5MHz
	case freq >= Freq5170MHz && freq <= Freq5825MHz:
		return (freq - Freq5000MHz) / ChannelSpacing5MHz
	case freq >= Freq5935MHz && freq <= Freq7115MHz:
		// 6 GHz. Channel 2 sits at 5935 MHz, below the 5950 MHz anchor used
		// by every other 6 GHz channel (center = 5950 + 5*channel).
		if freq == Freq5935MHz {
			return Channel2
		}
		ch := (freq - Freq5950MHz) / ChannelSpacing5MHz
		if ch < 1 {
			return 0
		}
		return ch
	default:
		return 0
	}
}

// frequencyToBand converts a frequency in MHz to the corresponding WiFi band.
// Note: 5925 MHz is the correct start of the 6 GHz band (U-NII-5). The previous
// 5900 MHz checks were approximations, so we standardized on the exact boundary.
func frequencyToBand(freq int) string {
	switch {
	case freq >= 5925:
		return "6GHz"
	case freq >= 4900:
		return "5GHz"
	case freq > 0:
		return "2.4GHz"
	default:
		return ""
	}
}

// deriveWiFiGeneration returns the WiFi generation number (4, 5, 6, 7) or
// "Unknown" if it cannot be determined from the capabilities array.
func deriveWiFiGeneration(capabilities []string) string {
	switch {
	case capsMatchAny(capabilities, "WiFi7", "802.11be", "EHT"):
		return "7"
	case capsMatchAny(capabilities, "WiFi6", "HE", "802.11ax"):
		return "6"
	case capsMatchAny(capabilities, "WiFi5", "VHT", "802.11ac"):
		return "5"
	case capsMatchAny(capabilities, "WiFi4", "HT", "802.11n"):
		return "4"
	}
	return "Unknown"
}

// getDominantWiFiStandard returns the dominant WiFi standard (e.g. "WiFi 6E (802.11ax)")
// based on the capabilities array. Returns "Unknown" if it cannot be determined.
func getDominantWiFiStandard(capabilities []string, band string) string {
	switch {
	case capsMatchAny(capabilities, "EHT", "WiFi7", "802.11be"):
		return "WiFi 7 (802.11be)"
	case capsMatchAny(capabilities, "HE", "WiFi6", "802.11ax"):
		if band == "6GHz" {
			return "WiFi 6E (802.11ax)"
		}
		return "WiFi 6 (802.11ax)"
	case capsMatchAny(capabilities, "VHT", "WiFi5", "802.11ac"):
		return "WiFi 5 (802.11ac)"
	case capsMatchAny(capabilities, "HT", "WiFi4", "802.11n"):
		return "WiFi 4 (802.11n)"
	case capsMatchAny(capabilities, "Legacy", "802.11a", "802.11b", "802.11g"):
		return "Legacy (802.11a/b/g)"
	}
	return "Unknown"
}

// hasBeamformingSupport checks if the AP supports beamforming based on
// its capabilities array. Returns true if any beamforming indicator is found.
func hasBeamformingSupport(capabilities []string, muMIMO bool) bool {
	if muMIMO {
		return true
	}
	if len(capabilities) == 0 {
		return false
	}

	for _, cap := range capabilities {
		lower := strings.ToLower(strings.TrimSpace(cap))
		if strings.Contains(lower, "txbf") || strings.Contains(lower, "beamform") {
			return true
		}
	}

	// SU beamforming is mandatory-capable from VHT (WiFi 5) onward.
	return capsMatchAny(capabilities,
		"VHT", "HE", "EHT", "WiFi5", "WiFi6", "WiFi7", "802.11ac", "802.11ax", "802.11be")
}
