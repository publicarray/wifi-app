package main

import (
	"testing"
)

func TestSignalToQuality(t *testing.T) {
	cases := []struct {
		signal int
		want   int
	}{
		{-30, 100},
		{-29, 100}, // above ExcellentSignal still clamped
		{0, 100},   // far above clamped
		{-100, 0},
		{-101, 0},  // below PoorSignal clamped
		{-200, 0},  // far below clamped
		{-65, 50},  // midpoint of [-100,-30] => 50
		{-50, 71},  // (-50 - -100)/70 * 100 = 50/70*100 ≈ 71
		{-80, 28},  // 20/70*100 ≈ 28
		{-95, 7},   // 5/70*100 ≈ 7
	}
	for _, c := range cases {
		got := signalToQuality(c.signal)
		if got != c.want {
			t.Errorf("signalToQuality(%d) = %d, want %d", c.signal, got, c.want)
		}
	}
}

// TestFrequencyChannelRoundTrip verifies the channel<->frequency mapping is
// internally consistent for the bands the app actually displays. Unknown or
// out-of-band inputs return 0 by convention.
func TestFrequencyChannelRoundTrip(t *testing.T) {
	cases := []struct {
		freq    int
		channel int
	}{
		// 2.4 GHz
		{2412, 1},
		{2437, 6},
		{2462, 11},
		{2484, 14},
		// 5 GHz UNII bands
		{5180, 36},
		{5240, 48},
		{5500, 100},
		{5825, 165},
	}
	for _, c := range cases {
		if got := frequencyToChannel(c.freq); got != c.channel {
			t.Errorf("frequencyToChannel(%d) = %d, want %d", c.freq, got, c.channel)
		}
		if got := channelToFrequency(c.channel); got != c.freq {
			t.Errorf("channelToFrequency(%d) = %d, want %d", c.channel, got, c.freq)
		}
	}

	// Out-of-band returns 0 sentinel.
	if got := frequencyToChannel(1000); got != 0 {
		t.Errorf("frequencyToChannel(1000) = %d, want 0 (out of band)", got)
	}
}

func TestParseBitrateInfo(t *testing.T) {
	cases := []struct {
		in           string
		wantStandard string
		wantWidth    string
		wantMimo     string
	}{
		{"", "Legacy (802.11a/b/g)", "20", "1x1"},
		{"144.4 MBit/s VHT-MCS 9 80MHz VHT-NSS 2", "WiFi 5 (802.11ac)", "80", "2x2"},
		{"600.0 MBit/s HE-MCS 11 160MHz HE-NSS 4 HE-GI 0.8us HE-DCM 0", "WiFi 6 (802.11ax)", "160", "4x4"},
		// EHT carries an explicit EHT-NSS token; verify we extract it.
		{"4800.0 MBit/s EHT-MCS 13 320MHz EHT-NSS 2", "WiFi 7 (802.11be)", "320", "2x2"},
		// EHT without NSS token (older drivers) → fall back to 1x1 default.
		{"4800.0 MBit/s EHT-MCS 13 320MHz", "WiFi 7 (802.11be)", "320", "1x1"},
		// UHR (WiFi 8) follows the same {prefix}-NSS pattern.
		{"UHR-MCS 11 UHR-NSS 2", "WiFi 8 (802.11bn)", "20", "2x2"},
		{"UHR-MCS 11", "WiFi 8 (802.11bn)", "20", "1x1"},
		// HT MCS index encodes streams: 0-7 = 1ss, 8-15 = 2ss, 16-23 = 3ss.
		{"144.4 MBit/s 40MHz HT-MCS 7", "WiFi 4 (802.11n)", "40", "1x1"},
		{"300.0 MBit/s 40MHz HT-MCS 15", "WiFi 4 (802.11n)", "40", "2x2"},
		{"450.0 MBit/s 40MHz MCS 23", "WiFi 4 (802.11n)", "40", "3x3"},
		// Bare "MCS" inside "VHT-MCS"/"HE-MCS" must not be misread as HT.
		{"866.7 MBit/s VHT-MCS 9 80MHz VHT-NSS 2", "WiFi 5 (802.11ac)", "80", "2x2"},
		{"54.0 MBit/s", "Legacy (802.11a/b/g)", "20", "1x1"},
	}
	for _, c := range cases {
		standard, width, mimo := parseBitrateInfo(c.in)
		if standard != c.wantStandard || width != c.wantWidth || mimo != c.wantMimo {
			t.Errorf("parseBitrateInfo(%q) = (%q, %q, %q); want (%q, %q, %q)",
				c.in, standard, width, mimo, c.wantStandard, c.wantWidth, c.wantMimo)
		}
	}
}

func TestEstimateMaxPhyRate(t *testing.T) {
	cases := []struct {
		name string
		ap   AccessPoint
		want int // exact for now — change to range-check if rates drift
	}{
		{
			name: "no capabilities returns 0",
			ap:   AccessPoint{ChannelWidth: 80, MIMOStreams: 2},
			want: 0,
		},
		{
			name: "HT 1x1 20MHz",
			ap:   AccessPoint{ChannelWidth: 20, MIMOStreams: 1, Capabilities: []string{"HT"}},
			want: 72,
		},
		{
			name: "VHT 2x2 80MHz",
			ap:   AccessPoint{ChannelWidth: 80, MIMOStreams: 2, Capabilities: []string{"VHT"}},
			want: 866, // 433 * 2
		},
		{
			name: "HE 4x4 160MHz",
			ap:   AccessPoint{ChannelWidth: 160, MIMOStreams: 4, Capabilities: []string{"HE"}},
			want: 4804, // 1201 * 4
		},
		{
			name: "EHT/WiFi7 falls through to HE rate table",
			ap:   AccessPoint{ChannelWidth: 320, MIMOStreams: 2, Capabilities: []string{"WiFi7"}},
			want: 4804, // 2402 * 2
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := estimateMaxPhyRate(&c.ap)
			if got != c.want {
				t.Errorf("estimateMaxPhyRate(%+v) = %d, want %d", c.ap, got, c.want)
			}
		})
	}
}

func TestNormalizeCapabilities(t *testing.T) {
	ap := &AccessPoint{
		Capabilities: []string{
			"802.11ax",      // -> HE + WiFi6
			"802.11ac",      // -> VHT + WiFi5
			"802.11n",       // -> HT + WiFi4
			"802.11g",       // -> Legacy
			"802.11be",      // -> WiFi7
			"802.11other",   // -> dropped (any 802.11* not in switch)
			"WPS",           // passthrough
			"WPS",           // dedup
			"802.11ax",      // dedup -> HE/WiFi6 already present
		},
	}
	normalizeCapabilities(ap)

	want := map[string]bool{
		"HE":     true,
		"WiFi6":  true,
		"VHT":    true,
		"WiFi5":  true,
		"HT":     true,
		"WiFi4":  true,
		"Legacy": true,
		"WiFi7":  true,
		"WPS":    true,
	}
	for _, c := range ap.Capabilities {
		if !want[c] {
			t.Errorf("unexpected capability %q after normalize", c)
		}
		delete(want, c)
	}
	if len(want) > 0 {
		t.Errorf("missing capabilities after normalize: %v", want)
	}
}

func TestAppendCapped(t *testing.T) {
	t.Run("under cap appends in place", func(t *testing.T) {
		s := []int{1, 2}
		got := appendCapped(s, 3, 5)
		if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
			t.Errorf("appendCapped under cap = %v, want [1 2 3]", got)
		}
	})

	t.Run("at cap returns last cap", func(t *testing.T) {
		got := appendCapped([]int{1, 2, 3}, 4, 3)
		if len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 4 {
			t.Errorf("appendCapped at cap = %v, want [2 3 4]", got)
		}
	})

	t.Run("over cap returns fresh backing array", func(t *testing.T) {
		original := []int{1, 2, 3, 4, 5}
		appended := appendCapped(original, 6, 3) // -> [4 5 6]

		// Mutating the truncated result must not touch the original slice.
		appended[0] = 99
		if original[3] != 4 {
			t.Errorf("appendCapped did not return fresh backing array; original[3]=%d, want 4", original[3])
		}
	})
}

func TestIsDFSChannel(t *testing.T) {
	dfs := []int{52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124, 128, 132, 136, 140, 144}
	for _, ch := range dfs {
		if !isDFSChannel(ch) {
			t.Errorf("isDFSChannel(%d) = false, want true", ch)
		}
	}
	notDFS := []int{1, 6, 11, 36, 40, 44, 48, 149, 153, 157, 161, 165}
	for _, ch := range notDFS {
		if isDFSChannel(ch) {
			t.Errorf("isDFSChannel(%d) = true, want false", ch)
		}
	}
}

func TestIntPtr(t *testing.T) {
	p := intPtr(42)
	if p == nil || *p != 42 {
		t.Fatalf("intPtr(42) returned %v", p)
	}
	// Ensure mutating the returned pointer does not affect a fresh call.
	*p = 99
	q := intPtr(42)
	if *q != 42 {
		t.Errorf("intPtr does not return a fresh address; got *q=%d", *q)
	}
}

func TestAppendUnique(t *testing.T) {
	got := appendUnique(nil, "a")
	got = appendUnique(got, "b")
	got = appendUnique(got, "a") // dup, should be no-op
	got = appendUnique(got, "c")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("appendUnique sequence = %v, want [a b c]", got)
	}
}

func TestNormalizeToken(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  CCMP  ", "CCMP"},
		{"WPA‐2", "WPA-2"}, // hyphen unicode variants normalised
		{"WPA–PSK", "WPA-PSK"},
	}
	for _, c := range cases {
		if got := normalizeToken(c.in); got != c.want {
			t.Errorf("normalizeToken(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
