//go:build linux && iw

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestIWParser() *iwParser {
	// Use a fully-loaded OUILookup with a tiny embedded vendor map so
	// LookupVendor returns deterministic results without touching disk or
	// the network.
	return &iwParser{
		ouiLookup: &OUILookup{
			ouiMap: map[string]string{
				"AA:BB:CC": "Acme Networks",
			},
			loaded: true,
		},
	}
}

func mustReadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", path))
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

func mustFindAP(t *testing.T, aps []AccessPoint, bssid string) AccessPoint {
	t.Helper()
	for _, ap := range aps {
		if ap.BSSID == bssid {
			return ap
		}
	}
	t.Fatalf("AP %s not found in %+v", bssid, aps)
	return AccessPoint{}
}

func TestIWParseScan_WPA2_2GHz(t *testing.T) {
	p := newTestIWParser()
	aps, err := p.ParseScan(mustReadFixture(t, "iw-scan/wpa2-2g.txt"))
	if err != nil {
		t.Fatalf("ParseScan: %v", err)
	}
	if len(aps) != 1 {
		t.Fatalf("want 1 AP, got %d", len(aps))
	}
	ap := aps[0]
	if ap.BSSID != "aa:bb:cc:11:22:33" {
		t.Errorf("BSSID = %q", ap.BSSID)
	}
	if ap.SSID != "MyNetwork" {
		t.Errorf("SSID = %q", ap.SSID)
	}
	if ap.Vendor != "Acme Networks" {
		t.Errorf("Vendor = %q (OUI lookup not wired)", ap.Vendor)
	}
	if ap.Frequency != 2462 || ap.Channel != 11 {
		t.Errorf("freq/channel = %d / %d", ap.Frequency, ap.Channel)
	}
	if ap.Band != "2.4GHz" {
		t.Errorf("Band = %q", ap.Band)
	}
	if ap.Signal != -45 {
		t.Errorf("Signal = %d", ap.Signal)
	}
	if ap.Security != "WPA2" {
		t.Errorf("Security = %q", ap.Security)
	}
	if ap.PMF != "Optional" {
		t.Errorf("PMF = %q (want Optional from MFP-capable)", ap.PMF)
	}
	if ap.DTIM != 2 {
		t.Errorf("DTIM = %d", ap.DTIM)
	}
	if !ap.WPS {
		t.Error("WPS = false, want true")
	}
	if !ap.QoSSupport {
		t.Error("QoSSupport = false, want true (WMM present)")
	}
	if ap.BSSLoadStations == nil || *ap.BSSLoadStations != 5 {
		t.Errorf("BSSLoadStations = %v, want 5", ap.BSSLoadStations)
	}
	// 153/255 * 100 = 60
	if ap.BSSLoadUtilization == nil || *ap.BSSLoadUtilization != 60 {
		t.Errorf("BSSLoadUtilization = %v, want 60", ap.BSSLoadUtilization)
	}
	if ap.MIMOStreams != 2 {
		t.Errorf("MIMOStreams = %d, want 2 (highest stream count seen)", ap.MIMOStreams)
	}
	if ap.ChannelWidth != 20 {
		t.Errorf("ChannelWidth = %d, want 20", ap.ChannelWidth)
	}
	if !contains(ap.SecurityCiphers, "CCMP") {
		t.Errorf("SecurityCiphers = %v, want to contain CCMP", ap.SecurityCiphers)
	}
	if !contains(ap.AuthMethods, "PSK") {
		t.Errorf("AuthMethods = %v, want to contain PSK", ap.AuthMethods)
	}
}

func TestIWParseScan_WPA3_5GHz_HE(t *testing.T) {
	p := newTestIWParser()
	aps, err := p.ParseScan(mustReadFixture(t, "iw-scan/wpa3-5g.txt"))
	if err != nil {
		t.Fatalf("ParseScan: %v", err)
	}
	if len(aps) != 1 {
		t.Fatalf("want 1 AP, got %d", len(aps))
	}
	ap := aps[0]
	if ap.SSID != "HomeWiFi-5G" {
		t.Errorf("SSID = %q", ap.SSID)
	}
	if ap.Frequency != 5180 || ap.Channel != 36 || ap.Band != "5GHz" {
		t.Errorf("freq/channel/band = %d / %d / %s", ap.Frequency, ap.Channel, ap.Band)
	}
	if ap.Security != "WPA3" {
		t.Errorf("Security = %q (want WPA3 from SAE auth suite)", ap.Security)
	}
	if ap.PMF != "Required" {
		t.Errorf("PMF = %q (want Required from MFP-required)", ap.PMF)
	}
	if ap.ChannelWidth != 80 {
		t.Errorf("ChannelWidth = %d, want 80 (VHT80)", ap.ChannelWidth)
	}
	if ap.MIMOStreams != 4 {
		t.Errorf("MIMOStreams = %d, want 4", ap.MIMOStreams)
	}
	if ap.BSSColor != 7 {
		t.Errorf("BSSColor = %d, want 7", ap.BSSColor)
	}
	if !ap.OBSSPD {
		t.Error("OBSSPD = false, want true")
	}
	if !ap.MUMIMO {
		t.Error("MUMIMO = false, want true")
	}
	if !ap.BSSTransition {
		t.Error("BSSTransition = false, want true")
	}
	if !ap.NeighborReport {
		t.Error("NeighborReport = false, want true")
	}
	if !ap.FastRoaming {
		t.Error("FastRoaming = false, want true (FT/SAE present)")
	}
	if ap.QAMSupport != 1024 {
		t.Errorf("QAMSupport = %d, want 1024", ap.QAMSupport)
	}
	if ap.MaxPhyRate <= 0 {
		t.Errorf("MaxPhyRate = %d, want >0 (HE-MCS rate calc)", ap.MaxPhyRate)
	}
	for _, want := range []string{"HT", "VHT", "HE"} {
		if !contains(ap.Capabilities, want) {
			t.Errorf("Capabilities missing %q (got %v)", want, ap.Capabilities)
		}
	}
}

func TestIWParseScan_LiveMultiBSS(t *testing.T) {
	p := newTestIWParser()
	aps, err := p.ParseScan(mustReadFixture(t, "iw-scan/live-multi-bss.txt"))
	if err != nil {
		t.Fatalf("ParseScan: %v", err)
	}
	if len(aps) != 3 {
		t.Fatalf("want 3 APs, got %d", len(aps))
	}

	wpa3 := mustFindAP(t, aps, "02:11:22:33:44:55")
	if wpa3.SSID != "ExampleNet-5G" {
		t.Errorf("SSID = %q", wpa3.SSID)
	}
	if wpa3.Frequency != 5220 || wpa3.Channel != 44 || wpa3.Band != "5GHz" {
		t.Errorf("freq/channel/band = %d / %d / %s", wpa3.Frequency, wpa3.Channel, wpa3.Band)
	}
	if wpa3.Security != "WPA3" {
		t.Errorf("Security = %q (want WPA3 from SAE auth suite)", wpa3.Security)
	}
	if wpa3.PMF != "Required" {
		t.Errorf("PMF = %q", wpa3.PMF)
	}
	if wpa3.CountryCode != "ZZ" {
		t.Errorf("CountryCode = %q", wpa3.CountryCode)
	}
	if wpa3.TxPower != 22 {
		t.Errorf("TxPower = %d", wpa3.TxPower)
	}
	if wpa3.BSSLoadStations == nil || *wpa3.BSSLoadStations != 4 {
		t.Errorf("BSSLoadStations = %v, want 4", wpa3.BSSLoadStations)
	}
	if wpa3.BSSLoadUtilization == nil || *wpa3.BSSLoadUtilization != 8 {
		t.Errorf("BSSLoadUtilization = %v, want 8", wpa3.BSSLoadUtilization)
	}
	if wpa3.ChannelWidth != 80 {
		t.Errorf("ChannelWidth = %d, want 80 from HE80 capability", wpa3.ChannelWidth)
	}
	if wpa3.MIMOStreams != 2 {
		t.Errorf("MIMOStreams = %d, want 2", wpa3.MIMOStreams)
	}
	if wpa3.BSSColor != 24 {
		t.Errorf("BSSColor = %d, want 24", wpa3.BSSColor)
	}
	if !wpa3.QoSSupport || !wpa3.UAPSD || !wpa3.BSSTransition || !wpa3.NeighborReport || !wpa3.MUMIMO || !wpa3.TWTSupport || !wpa3.FastRoaming {
		t.Errorf("missing live flags: QoS=%t UAPSD=%t BTM=%t NR=%t MU-MIMO=%t TWT=%t FT=%t",
			wpa3.QoSSupport, wpa3.UAPSD, wpa3.BSSTransition, wpa3.NeighborReport, wpa3.MUMIMO, wpa3.TWTSupport, wpa3.FastRoaming)
	}
	if wpa3.QAMSupport != 1024 {
		t.Errorf("QAMSupport = %d, want 1024", wpa3.QAMSupport)
	}
	if wpa3.MaxPhyRate <= 0 {
		t.Errorf("MaxPhyRate = %d, want >0", wpa3.MaxPhyRate)
	}
	if !contains(wpa3.SecurityCiphers, "CCMP") {
		t.Errorf("SecurityCiphers = %v, want CCMP", wpa3.SecurityCiphers)
	}
	if !contains(wpa3.AuthMethods, "SAE FT/SAE") {
		t.Errorf("AuthMethods = %v, want SAE FT/SAE", wpa3.AuthMethods)
	}
	for _, want := range []string{"HT", "VHT", "HE"} {
		if !contains(wpa3.Capabilities, want) {
			t.Errorf("Capabilities missing %q (got %v)", want, wpa3.Capabilities)
		}
	}

	twoFour := mustFindAP(t, aps, "02:11:22:33:44:66")
	if twoFour.SSID != "" {
		t.Errorf("SSID = %q, want empty", twoFour.SSID)
	}
	if twoFour.Frequency != 2437 || twoFour.Channel != 6 || twoFour.Band != "2.4GHz" {
		t.Errorf("freq/channel/band = %d / %d / %s", twoFour.Frequency, twoFour.Channel, twoFour.Band)
	}
	if twoFour.Security != "WPA2" {
		t.Errorf("Security = %q", twoFour.Security)
	}
	if twoFour.PMF != "Optional" {
		t.Errorf("PMF = %q", twoFour.PMF)
	}
	if twoFour.CountryCode != "" {
		t.Errorf("CountryCode = %q, want empty", twoFour.CountryCode)
	}
	if twoFour.DTIM != 1 {
		t.Errorf("DTIM = %d, want 1", twoFour.DTIM)
	}
	if twoFour.BSSLoadStations == nil || *twoFour.BSSLoadStations != 0 {
		t.Errorf("BSSLoadStations = %v, want 0", twoFour.BSSLoadStations)
	}
	if twoFour.BSSLoadUtilization == nil || *twoFour.BSSLoadUtilization != 13 {
		t.Errorf("BSSLoadUtilization = %v, want 13", twoFour.BSSLoadUtilization)
	}
	if twoFour.ChannelWidth != 40 {
		t.Errorf("ChannelWidth = %d, want 40 from HT40 capability", twoFour.ChannelWidth)
	}
	if twoFour.BSSColor != 45 {
		t.Errorf("BSSColor = %d, want 45", twoFour.BSSColor)
	}
	if !twoFour.BSSTransition || !twoFour.NeighborReport || !twoFour.MUMIMO || !twoFour.TWTSupport {
		t.Errorf("missing 2.4GHz flags: BTM=%t NR=%t MU-MIMO=%t TWT=%t",
			twoFour.BSSTransition, twoFour.NeighborReport, twoFour.MUMIMO, twoFour.TWTSupport)
	}

	secondaryFive := mustFindAP(t, aps, "02:11:22:33:44:77")
	if secondaryFive.SSID != "ExampleNet-Mesh-5G" {
		t.Errorf("SSID = %q", secondaryFive.SSID)
	}
	if secondaryFive.PMF != "Disabled" {
		t.Errorf("PMF = %q, want Disabled", secondaryFive.PMF)
	}
	if secondaryFive.CountryCode != "ZZ" {
		t.Errorf("CountryCode = %q", secondaryFive.CountryCode)
	}
	if secondaryFive.BSSColor != 29 {
		t.Errorf("BSSColor = %d, want 29", secondaryFive.BSSColor)
	}
}

func TestIWParseScan_MultiAP(t *testing.T) {
	p := newTestIWParser()
	aps, err := p.ParseScan(mustReadFixture(t, "iw-scan/multi-ap.txt"))
	if err != nil {
		t.Fatalf("ParseScan: %v", err)
	}
	if len(aps) != 2 {
		t.Fatalf("want 2 APs, got %d", len(aps))
	}
	if aps[0].SSID != "GuestWiFi" || aps[0].Security != "Open" {
		t.Errorf("AP0 SSID/Security = %q / %q (want GuestWiFi / Open)", aps[0].SSID, aps[0].Security)
	}
	if aps[1].SSID != "" || aps[1].Security != "WPA2" {
		t.Errorf("AP1 SSID/Security = %q / %q (want empty hidden / WPA2)", aps[1].SSID, aps[1].Security)
	}
	if aps[1].Channel != 48 || aps[1].Band != "5GHz" {
		t.Errorf("AP1 channel/band = %d / %s", aps[1].Channel, aps[1].Band)
	}
}

func TestIWParseScan_Empty(t *testing.T) {
	p := newTestIWParser()
	aps, err := p.ParseScan([]byte(""))
	if err != nil {
		t.Fatalf("ParseScan empty: %v", err)
	}
	if len(aps) != 0 {
		t.Errorf("want 0 APs from empty input, got %d", len(aps))
	}
}

func TestIWParseLink_Connected(t *testing.T) {
	p := newTestIWParser()
	info, err := p.ParseLink(mustReadFixture(t, "iw-link/connected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info["connected"] != "true" {
		t.Errorf("connected = %q, want true", info["connected"])
	}
	if info["bssid"] != "aa:bb:cc:11:22:33" {
		t.Errorf("bssid = %q", info["bssid"])
	}
	if info["ssid"] != "MyNetwork" {
		t.Errorf("ssid = %q", info["ssid"])
	}
	if info["frequency"] != "2462" {
		t.Errorf("frequency = %q", info["frequency"])
	}
	if info["signal"] != "-45" {
		t.Errorf("signal = %q", info["signal"])
	}
	if info["rx_bytes"] != "9876543" || info["tx_bytes"] != "1234567" {
		t.Errorf("byte counters = %s rx / %s tx", info["rx_bytes"], info["tx_bytes"])
	}
}

func TestIWParseLink_ConnectedHE(t *testing.T) {
	p := newTestIWParser()
	info, err := p.ParseLink(mustReadFixture(t, "iw-link/connected-he.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info["connected"] != "true" {
		t.Errorf("connected = %q, want true", info["connected"])
	}
	if info["bssid"] != "02:11:22:33:44:55" {
		t.Errorf("bssid = %q", info["bssid"])
	}
	if info["ssid"] != "ExampleNet-5G" {
		t.Errorf("ssid = %q", info["ssid"])
	}
	if info["frequency"] != "5220.0" {
		t.Errorf("frequency = %q", info["frequency"])
	}
	if info["signal"] != "-53" {
		t.Errorf("signal = %q", info["signal"])
	}
	if info["rx_bytes"] != "123456789" || info["tx_bytes"] != "98765432" {
		t.Errorf("byte counters = %s rx / %s tx", info["rx_bytes"], info["tx_bytes"])
	}
	if info["rx_bitrate"] != "413.0" || info["tx_bitrate"] != "458.8" {
		t.Errorf("bitrates = %s rx / %s tx", info["rx_bitrate"], info["tx_bitrate"])
	}
	if info["rx_bitrate_info"] != "40MHz HE-MCS 8 HE-NSS 2 HE-GI 0 HE-DCM 0" {
		t.Errorf("rx_bitrate_info = %q", info["rx_bitrate_info"])
	}
	if info["tx_bitrate_info"] != "40MHz HE-MCS 9 HE-NSS 2 HE-GI 0 HE-DCM 0" {
		t.Errorf("tx_bitrate_info = %q", info["tx_bitrate_info"])
	}
}

func TestIWParseLink_NotConnected(t *testing.T) {
	p := newTestIWParser()
	info, err := p.ParseLink(mustReadFixture(t, "iw-link/not-connected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info["connected"] != "false" {
		t.Errorf("connected = %q, want false", info["connected"])
	}
}

func TestIWParseStation(t *testing.T) {
	p := newTestIWParser()
	stats, err := p.ParseStation(mustReadFixture(t, "iw-station/connected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if stats["connected"] != "true" {
		t.Errorf("connected = %q", stats["connected"])
	}
	if stats["bssid"] != "aa:bb:cc:11:22:33" {
		t.Errorf("bssid = %q", stats["bssid"])
	}
	if stats["signal"] != "-45" || stats["signal_avg"] != "-46" {
		t.Errorf("signal/avg = %s / %s", stats["signal"], stats["signal_avg"])
	}
	if stats["tx_retries"] != "123" || stats["tx_failed"] != "5" {
		t.Errorf("retries/failed = %s / %s", stats["tx_retries"], stats["tx_failed"])
	}
	if stats["last_ack_signal"] != "-44" {
		t.Errorf("last_ack_signal = %q", stats["last_ack_signal"])
	}
	if stats["connected_time"] != "3600" {
		t.Errorf("connected_time = %q", stats["connected_time"])
	}
	// retry_rate = 123/5678 * 100 ≈ 2.17
	if stats["retry_rate"] != "2.17" {
		t.Errorf("retry_rate = %q, want 2.17", stats["retry_rate"])
	}
}

func TestIWParseStation_HE(t *testing.T) {
	p := newTestIWParser()
	stats, err := p.ParseStation(mustReadFixture(t, "iw-station/connected-he.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if stats["connected"] != "true" {
		t.Errorf("connected = %q", stats["connected"])
	}
	if stats["bssid"] != "02:11:22:33:44:55" {
		t.Errorf("bssid = %q", stats["bssid"])
	}
	if stats["signal"] != "-51" || stats["signal_avg"] != "-52" {
		t.Errorf("signal/avg = %s / %s", stats["signal"], stats["signal_avg"])
	}
	if stats["rx_bytes"] != "123457120" || stats["tx_bytes"] != "98765560" {
		t.Errorf("byte counters = %s rx / %s tx", stats["rx_bytes"], stats["tx_bytes"])
	}
	if stats["tx_retries"] != "50000" || stats["tx_failed"] != "12" {
		t.Errorf("retries/failed = %s / %s", stats["tx_retries"], stats["tx_failed"])
	}
	if stats["tx_bitrate"] != "458.8" || stats["rx_bitrate"] != "137.6" {
		t.Errorf("bitrates = %s tx / %s rx", stats["tx_bitrate"], stats["rx_bitrate"])
	}
	if stats["tx_bitrate_info"] != "40MHz HE-MCS 9 HE-NSS 2 HE-GI 0 HE-DCM 0" {
		t.Errorf("tx_bitrate_info = %q", stats["tx_bitrate_info"])
	}
	if stats["rx_bitrate_info"] != "40MHz HE-MCS 3 HE-NSS 2 HE-GI 0 HE-DCM 0" {
		t.Errorf("rx_bitrate_info = %q", stats["rx_bitrate_info"])
	}
	if stats["last_ack_signal"] != "-50" {
		t.Errorf("last_ack_signal = %q", stats["last_ack_signal"])
	}
	if stats["connected_time"] != "7200" {
		t.Errorf("connected_time = %q", stats["connected_time"])
	}
	if stats["retry_rate"] != "12.50" {
		t.Errorf("retry_rate = %q, want 12.50", stats["retry_rate"])
	}
}
