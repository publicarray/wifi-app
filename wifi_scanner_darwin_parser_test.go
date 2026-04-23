package main

import (
	"testing"
)

func newTestAirportParser() *airportParser {
	return &airportParser{
		ouiLookup: &OUILookup{
			ouiMap: map[string]string{
				"AA:BB:CC": "Acme Networks",
			},
			loaded: true,
		},
	}
}

func TestAirportParseScan(t *testing.T) {
	p := newTestAirportParser()
	aps, err := p.ParseScan(mustReadFixtureAirport(t, "airport-scan/typical.plist"))
	if err != nil {
		t.Fatalf("ParseScan: %v", err)
	}
	if len(aps) != 3 {
		t.Fatalf("want 3 APs, got %d", len(aps))
	}

	// AP 0: WPA2/WPA3 mixed-mode 5 GHz network with HT/VHT/HE caps.
	homewifi := aps[0]
	if homewifi.SSID != "HomeWiFi" || homewifi.BSSID != "aa:bb:cc:11:22:33" {
		t.Errorf("ap0 SSID/BSSID = %q / %q", homewifi.SSID, homewifi.BSSID)
	}
	if homewifi.Vendor != "Acme Networks" {
		t.Errorf("ap0 Vendor = %q (OUI lookup)", homewifi.Vendor)
	}
	if homewifi.Signal != -55 || homewifi.Noise != -92 {
		t.Errorf("ap0 signal/noise = %d / %d", homewifi.Signal, homewifi.Noise)
	}
	if homewifi.SNR != 37 {
		t.Errorf("ap0 SNR = %d, want 37 (signal - noise)", homewifi.SNR)
	}
	if homewifi.Channel != 36 || homewifi.Frequency != 5180 {
		t.Errorf("ap0 channel/freq = %d / %d", homewifi.Channel, homewifi.Frequency)
	}
	if homewifi.Band != "5GHz" {
		t.Errorf("ap0 Band = %q", homewifi.Band)
	}
	if homewifi.ChannelWidth != 80 {
		t.Errorf("ap0 ChannelWidth = %d", homewifi.ChannelWidth)
	}
	if homewifi.Security != "WPA3" {
		t.Errorf("ap0 Security = %q, want WPA3 (mixed mode advertises both, WPA3 wins)", homewifi.Security)
	}
	if homewifi.PMF != "Optional" {
		t.Errorf("ap0 PMF = %q, want Optional (MFP w/o required)", homewifi.PMF)
	}
	if homewifi.CountryCode != "AU" {
		t.Errorf("ap0 CountryCode = %q, want AU (uppercased)", homewifi.CountryCode)
	}
	if homewifi.DTIM != 2 {
		t.Errorf("ap0 DTIM = %d", homewifi.DTIM)
	}
	for _, want := range []string{"HT", "VHT", "HE"} {
		if !contains(homewifi.Capabilities, want) {
			t.Errorf("ap0 Capabilities missing %q (got %v)", want, homewifi.Capabilities)
		}
	}
	if !contains(homewifi.SecurityCiphers, "AES") {
		t.Errorf("ap0 SecurityCiphers = %v, want to contain AES", homewifi.SecurityCiphers)
	}
	for _, want := range []string{"PSK", "SAE"} {
		if !contains(homewifi.AuthMethods, want) {
			t.Errorf("ap0 AuthMethods missing %q (got %v)", want, homewifi.AuthMethods)
		}
	}

	// AP 1: open network with HT only.
	guest := aps[1]
	if guest.Security != "Open" {
		t.Errorf("ap1 Security = %q, want Open", guest.Security)
	}
	if guest.PMF != "Disabled" {
		t.Errorf("ap1 PMF = %q, want Disabled", guest.PMF)
	}
	if guest.ChannelWidth != 20 {
		t.Errorf("ap1 ChannelWidth = %d, want default 20", guest.ChannelWidth)
	}

	// AP 2: hidden SSID with MFP-required (PMF=Required).
	hidden := aps[2]
	if hidden.SSID != "" {
		t.Errorf("ap2 SSID = %q, want empty (hidden)", hidden.SSID)
	}
	if hidden.Security != "WPA2" {
		t.Errorf("ap2 Security = %q, want WPA2", hidden.Security)
	}
	if hidden.PMF != "Required" {
		t.Errorf("ap2 PMF = %q, want Required", hidden.PMF)
	}
}

func TestAirportParseScan_Empty(t *testing.T) {
	p := newTestAirportParser()
	aps, err := p.ParseScan([]byte("<?xml version=\"1.0\"?><plist><array></array></plist>"))
	if err != nil {
		t.Fatalf("ParseScan empty array: %v", err)
	}
	if len(aps) != 0 {
		t.Errorf("want 0 APs from empty plist array, got %d", len(aps))
	}
}

func TestAirportParseLink_Connected(t *testing.T) {
	p := newTestAirportParser()
	info, err := p.ParseLink(mustReadFixtureAirport(t, "airport-link/connected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info["connected"] != "true" {
		t.Errorf("connected = %q, want true", info["connected"])
	}
	if info["bssid"] != "aa:bb:cc:11:22:33" {
		t.Errorf("bssid = %q", info["bssid"])
	}
	if info["signal"] != "-55" || info["signal_avg"] != "-55" {
		t.Errorf("signal/avg = %s / %s", info["signal"], info["signal_avg"])
	}
	if info["channel"] != "36" {
		t.Errorf("channel = %q", info["channel"])
	}
	if info["channel_width"] != "80" {
		t.Errorf("channel_width = %q, want 80", info["channel_width"])
	}
}

func TestAirportParseLink_NotConnected(t *testing.T) {
	p := newTestAirportParser()
	info, err := p.ParseLink(mustReadFixtureAirport(t, "airport-link/not-connected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info["connected"] != "false" {
		t.Errorf("connected = %q, want false", info["connected"])
	}
}

func TestParseAirportSecurity(t *testing.T) {
	cases := []struct {
		in       string
		security string
		pmf      string
	}{
		{"", "Open", "Disabled"},
		{"OPEN", "Open", "Disabled"},
		{"WEP", "WEP", "Disabled"},
		{"WPA(PSK/TKIP/AES)", "WPA", "Disabled"},
		{"WPA2(PSK/AES)", "WPA2", "Disabled"},
		{"WPA2(PSK/AES) MFP-required", "WPA2", "Required"},
		{"WPA3(SAE/AES) MFP", "WPA3", "Optional"},
		{"WPA2(PSK/AES) WPA3(SAE/AES)", "WPA3", "Disabled"},
	}
	for _, c := range cases {
		sec, _, _, pmf := parseAirportSecurity(c.in)
		if sec != c.security || pmf != c.pmf {
			t.Errorf("parseAirportSecurity(%q) = (%q, %q); want (%q, %q)",
				c.in, sec, pmf, c.security, c.pmf)
		}
	}
}

