package main

import "testing"

func TestMatchUniFiDevice(t *testing.T) {
	devices := []UniFiDeviceInfo{
		{Name: "Office", MAC: "68:d7:9a:11:22:30"},
		{Name: "Garage", MAC: "68:d7:9a:11:22:60"},
		{Name: "Attic", MAC: "f0:9f:c2:aa:bb:cc"},
	}

	cases := []struct {
		name    string
		bssid   string
		want    string
		matched bool
	}{
		{"exact match", "68:d7:9a:11:22:30", "Office", true},
		{"first octet flipped (virtual BSSID)", "6a:d7:9a:11:22:30", "Office", true},
		{"last octet offset within range", "6a:d7:9a:11:22:33", "Office", true},
		{"nearest device wins", "68:d7:9a:11:22:5e", "Garage", true},
		{"offset too large", "68:d7:9a:11:22:45", "", false},
		{"different middle octets", "68:d7:9a:ff:22:30", "", false},
		{"unrelated vendor", "aa:bb:cc:dd:ee:ff", "", false},
		{"virtual bssid, flipped first octet and +1 last", "6a:d7:9a:11:22:31", "Office", true},
		{"invalid bssid", "not-a-mac", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := matchUniFiDevice(devices, c.bssid)
			if ok != c.matched {
				t.Fatalf("matched=%v, want %v (got %+v)", ok, c.matched, got)
			}
			if ok && got.Name != c.want {
				t.Errorf("matched %q, want %q", got.Name, c.want)
			}
		})
	}
}

func TestMatchUniFiDevice_FourthOctetStride(t *testing.T) {
	// Real-world U6+ example: device base MAC 1c:6a:1b:94:08:e9 broadcasts
	// virtual BSSIDs with a flipped first octet and the 4th octet stepped in
	// 0x10 strides per SSID (94 → a4 → b4).
	devices := []UniFiDeviceInfo{
		{Name: "Studio", MAC: "1c:6a:1b:94:08:e9"},
	}

	for _, bssid := range []string{
		"1e:6a:1b:94:08:e9", // first-octet variant of the base MAC
		"1e:6a:1b:a4:08:e9", // +0x10 stride, second SSID
		"1e:6a:1b:b4:08:e9", // +0x20 stride, third SSID
	} {
		got, ok := matchUniFiDevice(devices, bssid)
		if !ok || got.Name != "Studio" {
			t.Errorf("bssid %s: matched=%v got=%q, want Studio", bssid, ok, got.Name)
		}
	}

	// Low nibble differs — not a 0x10 stride, must not match.
	if got, ok := matchUniFiDevice(devices, "1e:6a:1b:a5:08:e9"); ok {
		t.Errorf("non-stride 4th octet matched %+v", got)
	}
	// Stride beyond 7 SSIDs out (delta 0x80) — must not match.
	if got, ok := matchUniFiDevice(devices, "1e:6a:1b:14:08:e9"); ok {
		t.Errorf("out-of-range stride matched %+v", got)
	}
}

func TestMatchUniFiDevice_LastOctetPreferredOverStride(t *testing.T) {
	// A BSSID one-off the last octet of device A is a closer derivation than
	// a 0x10 4th-octet stride from device B — A must win, not tie.
	devices := []UniFiDeviceInfo{
		{Name: "A", MAC: "1c:6a:1b:94:08:e8"},
		{Name: "B", MAC: "1c:6a:1b:84:08:e9"},
	}
	got, ok := matchUniFiDevice(devices, "1e:6a:1b:94:08:e9")
	if !ok || got.Name != "A" {
		t.Fatalf("got %q ok=%v, want A", got.Name, ok)
	}
}

func TestMatchUniFiDevice_AmbiguousTieSkipped(t *testing.T) {
	// Two sequential units whose last octets are equidistant from the BSSID —
	// must be treated as ambiguous, not guessed.
	devices := []UniFiDeviceInfo{
		{Name: "A", MAC: "68:d7:9a:11:22:10"},
		{Name: "B", MAC: "68:d7:9a:11:22:14"},
	}
	if got, ok := matchUniFiDevice(devices, "68:d7:9a:11:22:12"); ok {
		t.Fatalf("expected ambiguous tie to be skipped, matched %+v", got)
	}
}

func TestPickUniFiSite(t *testing.T) {
	sites := []uniFiSite{
		{ID: "abc", Name: "Default"},
		{ID: "def", Name: "Warehouse"},
	}
	if s, ok := pickUniFiSite(sites, ""); !ok || s.ID != "abc" {
		t.Errorf("empty preference should pick first site, got %+v ok=%v", s, ok)
	}
	if s, ok := pickUniFiSite(sites, "warehouse"); !ok || s.ID != "def" {
		t.Errorf("name match failed, got %+v ok=%v", s, ok)
	}
	if s, ok := pickUniFiSite(sites, "def"); !ok || s.ID != "def" {
		t.Errorf("id match failed, got %+v ok=%v", s, ok)
	}
	if _, ok := pickUniFiSite(sites, "nope"); ok {
		t.Errorf("unknown preference should not match")
	}
	if _, ok := pickUniFiSite(nil, ""); ok {
		t.Errorf("no sites should not match")
	}
}
