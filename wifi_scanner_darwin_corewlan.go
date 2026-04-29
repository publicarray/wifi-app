//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework CoreWLAN -framework CoreLocation -framework Foundation

#import <CoreWLAN/CoreWLAN.h>
#import <CoreLocation/CoreLocation.h>
#import <Foundation/Foundation.h>
#import <stdlib.h>

// CoreLocation glue.
//
// macOS Sonoma (14) made WiFi scanning gated by Location Services. Without an
// authorized CLLocationManager, CWInterface.scanForNetworksWithName: returns
// entries with empty SSID/BSSID strings, and CWInterface.bssid likewise comes
// back blank. Just declaring NSLocationWhenInUseUsageDescription in Info.plist
// is not enough — the OS only shows the prompt after the app actually calls
// requestWhenInUseAuthorization on a CLLocationManager. We retain the manager
// in a file-scope strong reference (ARC) so it outlives the C call that
// created it and the system can deliver authorization callbacks.
static CLLocationManager *g_cw_locationManager = nil;

static void cw_ensure_location_manager(void) {
	if (g_cw_locationManager == nil) {
		g_cw_locationManager = [[CLLocationManager alloc] init];
	}
}

// cw_location_authorization_status returns the raw CLAuthorizationStatus.
//   0 = NotDetermined, 1 = Restricted, 2 = Denied,
//   3 = AuthorizedAlways, 4 = AuthorizedWhenInUse.
static int cw_location_authorization_status(void) {
	cw_ensure_location_manager();
	if (@available(macOS 11.0, *)) {
		return (int)g_cw_locationManager.authorizationStatus;
	}
	return (int)[CLLocationManager authorizationStatus];
}

// cw_location_services_enabled mirrors the system-wide toggle. macOS deprecated
// calling this on the main thread (it can block); cgo invokes it from a Go
// goroutine, which is off-main, so this is safe.
static int cw_location_services_enabled(void) {
	return [CLLocationManager locationServicesEnabled] ? 1 : 0;
}

// cw_request_location_authorization triggers the system prompt. Must run on a
// thread with a runloop or CoreLocation silently no-ops, so we hop to the main
// queue. The call is fire-and-forget; the caller polls
// cw_location_authorization_status to observe the user's response.
static void cw_request_location_authorization(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		cw_ensure_location_manager();
		if ([g_cw_locationManager respondsToSelector:@selector(requestWhenInUseAuthorization)]) {
			[g_cw_locationManager requestWhenInUseAuthorization];
		}
	});
}

static char *cw_copy_interfaces_json() {
	@autoreleasepool {
		CWWiFiClient *client = [CWWiFiClient sharedWiFiClient];
		NSArray<CWInterface *> *ifaces = client.interfaces;
		NSMutableArray *names = [NSMutableArray array];
		for (CWInterface *iface in ifaces) {
			if (iface.interfaceName) {
				[names addObject: iface.interfaceName];
			}
		}
		NSData *data = [NSJSONSerialization dataWithJSONObject:names options:0 error:nil];
		if (!data) {
			return strdup("[]");
		}
		NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
		return strdup([json UTF8String]);
	}
}

static CWInterface *cw_select_interface(const char *name) {
	CWWiFiClient *client = [CWWiFiClient sharedWiFiClient];
	if (name && name[0] != '\0') {
		NSString *ifaceName = [NSString stringWithUTF8String:name];
		return [client interfaceWithName:ifaceName];
	}
	return client.interface;
}

// cw_detect_network_security maps CWNetwork's supportsSecurity: responses
// to our Security enum integers (Open=0, WEP=1, WPA=2, WPA2=3, WPA3=4,
// OWE=5). CoreWLAN does not expose a single security-type property on
// CWNetwork, so we probe in priority order (strongest first) and return the
// best-supported mode. OWE (Enhanced Open) is a distinct category that we
// detect first so it's not accidentally collapsed into "Open" or "WPA3".
static int cw_detect_network_security(CWNetwork *net) {
	if (!net) {
		return 0;
	}
	if ([net supportsSecurity:kCWSecurityOWE] ||
	    [net supportsSecurity:kCWSecurityOWETransition]) {
		return 5; // OWE / Enhanced Open
	}
	if ([net supportsSecurity:kCWSecurityWPA3Personal] ||
	    [net supportsSecurity:kCWSecurityWPA3Enterprise] ||
	    [net supportsSecurity:kCWSecurityWPA3Transition]) {
		return 4; // WPA3
	}
	if ([net supportsSecurity:kCWSecurityWPA2Personal] ||
	    [net supportsSecurity:kCWSecurityWPA2Enterprise]) {
		return 3; // WPA2
	}
	if ([net supportsSecurity:kCWSecurityWPAPersonal] ||
	    [net supportsSecurity:kCWSecurityWPAEnterprise] ||
	    [net supportsSecurity:kCWSecurityWPAPersonalMixed] ||
	    [net supportsSecurity:kCWSecurityWPAEnterpriseMixed]) {
		return 2; // WPA
	}
	if ([net supportsSecurity:kCWSecurityWEP]) {
		return 1; // WEP
	}
	return 0; // Open / unknown
}

// cw_supported_phy_modes returns an NSArray of CWPHYMode integers the
// network advertises. Used by the Go side to populate Capabilities
// (HT/VHT/HE/EHT). We probe newest-first; older modes are implied by newer
// ones in practice but we report each separately so the caller can decide.
static NSArray *cw_supported_phy_modes(CWNetwork *net) {
	NSMutableArray *modes = [NSMutableArray array];
	if (!net) {
		return modes;
	}
	if ([net supportsPHYMode:kCWPHYMode11ax]) {
		[modes addObject:@(kCWPHYMode11ax)];
	}
	if ([net supportsPHYMode:kCWPHYMode11ac]) {
		[modes addObject:@(kCWPHYMode11ac)];
	}
	if ([net supportsPHYMode:kCWPHYMode11n]) {
		[modes addObject:@(kCWPHYMode11n)];
	}
	if ([net supportsPHYMode:kCWPHYMode11g]) {
		[modes addObject:@(kCWPHYMode11g)];
	}
	if ([net supportsPHYMode:kCWPHYMode11a]) {
		[modes addObject:@(kCWPHYMode11a)];
	}
	if ([net supportsPHYMode:kCWPHYMode11b]) {
		[modes addObject:@(kCWPHYMode11b)];
	}
	return modes;
}

static NSDictionary *cw_network_to_dict(CWNetwork *net) {
	NSMutableDictionary *dict = [NSMutableDictionary dictionary];
	if (net.ssid) {
		dict[@"ssid"] = net.ssid;
	}
	if (net.bssid) {
		dict[@"bssid"] = net.bssid;
	}
	dict[@"rssi"] = @(net.rssiValue);
	dict[@"noise"] = @(net.noiseMeasurement);
	dict[@"beaconInterval"] = @(net.beaconInterval);
	if (net.countryCode) {
		dict[@"countryCode"] = net.countryCode;
	}
	if (net.wlanChannel) {
		dict[@"channel"] = @(net.wlanChannel.channelNumber);
		dict[@"channelWidth"] = @(net.wlanChannel.channelWidth);
		dict[@"channelBand"] = @(net.wlanChannel.channelBand);
	}
	dict[@"security"] = @(cw_detect_network_security(net));
	dict[@"phyModes"] = cw_supported_phy_modes(net);
	return dict;
}

static NSDictionary *cw_interface_current_dict(CWInterface *iface) {
	NSMutableDictionary *dict = [NSMutableDictionary dictionary];
	if (!iface) {
		return dict;
	}
	if (iface.ssid) {
		dict[@"ssid"] = iface.ssid;
	}
	if (iface.bssid) {
		dict[@"bssid"] = iface.bssid;
	}
	dict[@"rssi"] = @(iface.rssiValue);
	dict[@"noise"] = @(iface.noiseMeasurement);
	dict[@"txRate"] = @(iface.transmitRate);
	dict[@"txPower"] = @(iface.transmitPower);
	dict[@"phyMode"] = @(iface.activePHYMode);
	if (iface.countryCode) {
		dict[@"countryCode"] = iface.countryCode;
	}
	if (iface.wlanChannel) {
		dict[@"channel"] = @(iface.wlanChannel.channelNumber);
		dict[@"channelWidth"] = @(iface.wlanChannel.channelWidth);
		dict[@"channelBand"] = @(iface.wlanChannel.channelBand);
	}
	// CWInterface exposes the active security directly. kCWSecurityPersonal
	// and kCWSecurityEnterprise are "OS-picked" buckets that don't tell us
	// the actual mode in use; we conservatively report them as WPA2 so the
	// UI doesn't claim "Open" on a secured network.
	switch ((int)iface.security) {
		case kCWSecurityOWE:
		case kCWSecurityOWETransition:
			dict[@"security"] = @(5); // OWE
			break;
		case kCWSecurityWPA3Personal:
		case kCWSecurityWPA3Enterprise:
		case kCWSecurityWPA3Transition:
			dict[@"security"] = @(4);
			break;
		case kCWSecurityWPA2Personal:
		case kCWSecurityWPA2Enterprise:
		case kCWSecurityPersonal:
		case kCWSecurityEnterprise:
			dict[@"security"] = @(3);
			break;
		case kCWSecurityWPAPersonal:
		case kCWSecurityWPAEnterprise:
		case kCWSecurityWPAPersonalMixed:
		case kCWSecurityWPAEnterpriseMixed:
			dict[@"security"] = @(2);
			break;
		case kCWSecurityWEP:
			dict[@"security"] = @(1);
			break;
		default:
			dict[@"security"] = @(0);
			break;
	}
	return dict;
}

static char *cw_copy_scan_json(const char *ifaceName) {
	@autoreleasepool {
		CWInterface *iface = cw_select_interface(ifaceName);
		if (!iface) {
			return strdup("[]");
		}
		NSError *error = nil;
		NSSet<CWNetwork *> *nets = [iface scanForNetworksWithName:nil error:&error];
		if (!nets) {
			return strdup("[]");
		}
		NSMutableArray *items = [NSMutableArray array];
		for (CWNetwork *net in nets) {
			[items addObject: cw_network_to_dict(net)];
		}
		NSData *data = [NSJSONSerialization dataWithJSONObject:items options:0 error:nil];
		if (!data) {
			return strdup("[]");
		}
		NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
		return strdup([json UTF8String]);
	}
}

static char *cw_copy_current_json(const char *ifaceName) {
	@autoreleasepool {
		CWInterface *iface = cw_select_interface(ifaceName);
		NSDictionary *dict = cw_interface_current_dict(iface);
		NSData *data = [NSJSONSerialization dataWithJSONObject:dict options:0 error:nil];
		if (!data) {
			return strdup("{}");
		}
		NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
		return strdup([json UTF8String]);
	}
}

static void cw_free(char *ptr) {
	if (ptr) {
		free(ptr);
	}
}
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type coreWLANNetwork struct {
	SSID           string `json:"ssid"`
	BSSID          string `json:"bssid"`
	RSSI           int    `json:"rssi"`
	Noise          int    `json:"noise"`
	BeaconInterval int    `json:"beaconInterval"`
	CountryCode    string `json:"countryCode"`
	Channel        int    `json:"channel"`
	ChannelWidth   int    `json:"channelWidth"`
	ChannelBand    int    `json:"channelBand"`
	Security       int    `json:"security"`
	PhyModes       []int  `json:"phyModes"`
}

type coreWLANCurrent struct {
	SSID         string  `json:"ssid"`
	BSSID        string  `json:"bssid"`
	RSSI         int     `json:"rssi"`
	Noise        int     `json:"noise"`
	TxRate       float64 `json:"txRate"`
	TxPower      int     `json:"txPower"`
	PhyMode      int     `json:"phyMode"`
	CountryCode  string  `json:"countryCode"`
	Channel      int     `json:"channel"`
	ChannelWidth int     `json:"channelWidth"`
	ChannelBand  int     `json:"channelBand"`
	Security     int     `json:"security"`
}

func coreWLANAvailable() bool {
	return true
}

// CLAuthorizationStatus values (mirrors Apple's enum).
const (
	locationAuthNotDetermined = 0
	locationAuthRestricted    = 1
	locationAuthDenied        = 2
	locationAuthAlways        = 3
	locationAuthWhenInUse     = 4
)

// ErrLocationDenied is returned when CoreWLAN cannot scan because macOS
// Location Services are disabled or the user has not authorized the app.
// Callers should surface this to the UI so the user can enable the toggle in
// System Settings → Privacy & Security → Location Services rather than seeing
// an empty network list.
var ErrLocationDenied = errors.New("macOS Location Services authorization required to scan WiFi networks")

// coreWLANEnsureLocationAuthorization returns nil only when a Location Services
// grant exists. It triggers the prompt the first time it sees an
// "undetermined" status. Subsequent calls poll the (cached) status, so the
// fast path is one C call.
func coreWLANEnsureLocationAuthorization() error {
	if int(C.cw_location_services_enabled()) == 0 {
		return ErrLocationDenied
	}
	switch int(C.cw_location_authorization_status()) {
	case locationAuthAlways, locationAuthWhenInUse:
		return nil
	case locationAuthNotDetermined:
		C.cw_request_location_authorization()
		return ErrLocationDenied
	default:
		return ErrLocationDenied
	}
}

// coreWLANPrimeLocationAuthorization is called once at scanner startup so the
// system prompt is triggered as early as possible (rather than waiting for the
// first scan tick). Errors are not actionable here; just kick the request.
func coreWLANPrimeLocationAuthorization() {
	_ = coreWLANEnsureLocationAuthorization()
}

// coreWLANInterfaces returns the list of WiFi interface names visible to
// CoreWLAN. Used as a first-choice source on macOS before falling back to
// `networksetup -listallhardwareports`.
func coreWLANInterfaces() ([]string, error) {
	jsonStr := C.cw_copy_interfaces_json()
	defer C.cw_free(jsonStr)

	raw := C.GoString(jsonStr)
	if raw == "" || raw == "[]" {
		return nil, errors.New("corewlan returned no interfaces")
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return nil, fmt.Errorf("corewlan interfaces json parse failed: %w", err)
	}
	return names, nil
}

func coreWLANScanNetworks(iface string) ([]AccessPoint, error) {
	if err := coreWLANEnsureLocationAuthorization(); err != nil {
		return nil, err
	}
	cIface := C.CString(iface)
	defer C.free(unsafe.Pointer(cIface))
	jsonStr := C.cw_copy_scan_json(cIface)
	defer C.cw_free(jsonStr)

	raw := C.GoString(jsonStr)
	if raw == "" || raw == "[]" {
		return nil, errors.New("corewlan scan returned empty result")
	}

	var nets []coreWLANNetwork
	if err := json.Unmarshal([]byte(raw), &nets); err != nil {
		return nil, fmt.Errorf("corewlan scan json parse failed: %w", err)
	}

	if len(nets) == 0 {
		return nil, errors.New("corewlan scan returned no networks")
	}

	var aps []AccessPoint
	for _, net := range nets {
		// BSSID is the unique key; an empty SSID is just a hidden network and
		// must be retained (matches airportParser.ParseScan behaviour).
		if net.BSSID == "" {
			continue
		}
		channelWidth := mapCWChannelWidth(net.ChannelWidth)
		if channelWidth == 0 {
			channelWidth = 20
		}
		band, freq := cwBandAndFrequency(net.Channel, net.ChannelBand)

		securityField := mapCWSecurity(net.Security)
		security, ciphers, authMethods, pmf := parseAirportSecurity(securityField)

		caps := make([]string, 0, len(net.PhyModes))
		for _, m := range net.PhyModes {
			if tag := cwPhyModeCapability(m); tag != "" {
				caps = appendUnique(caps, tag)
			}
		}

		ap := AccessPoint{
			SSID:            net.SSID,
			BSSID:           strings.ToLower(net.BSSID),
			Channel:         net.Channel,
			Frequency:       freq,
			Band:            band,
			Signal:          net.RSSI,
			SignalQuality:   signalToQuality(net.RSSI),
			Vendor:          "",
			LastSeen:        time.Now(),
			Capabilities:    caps,
			ChannelWidth:    channelWidth,
			BeaconInt:       net.BeaconInterval,
			CountryCode:     net.CountryCode,
			Security:        security,
			SecurityCiphers: ciphers,
			AuthMethods:     authMethods,
			PMF:             pmf,
		}
		if net.Noise < 0 {
			ap.Noise = net.Noise
			ap.SNR = net.RSSI - net.Noise
		}
		if ap.Security == "" {
			ap.Security = "Open"
		}
		if defaultOUILookup != nil {
			ap.Vendor = defaultOUILookup.LookupVendor(ap.BSSID)
		}
		ap.DFS = isDFSChannel(ap.Channel)
		aps = append(aps, ap)
	}

	if len(aps) == 0 {
		return nil, errors.New("corewlan scan yielded no usable networks")
	}
	return aps, nil
}

func coreWLANConnectionInfo(iface string) (ConnectionInfo, error) {
	current, err := coreWLANCurrentInfo(iface)
	if err != nil {
		return ConnectionInfo{}, err
	}

	_, freq := cwBandAndFrequency(current.Channel, current.ChannelBand)
	standard := cwPhyModeStandard(current.PhyMode)
	conn := ConnectionInfo{
		Connected:    current.SSID != "" || current.BSSID != "",
		SSID:         current.SSID,
		BSSID:        strings.ToLower(current.BSSID),
		Channel:      current.Channel,
		Frequency:    freq,
		Signal:       current.RSSI,
		SignalAvg:    current.RSSI,
		TxBitrate:    current.TxRate,
		RxBitrate:    0,
		WiFiStandard: standard,
		ChannelWidth: mapCWChannelWidth(current.ChannelWidth),
		MIMOConfig:   "1x1",
	}
	if conn.ChannelWidth == 0 {
		conn.ChannelWidth = 20
	}
	return conn, nil
}

func coreWLANLinkInfo(iface string) (map[string]string, error) {
	current, err := coreWLANCurrentInfo(iface)
	if err != nil {
		return map[string]string{"connected": "false"}, err
	}
	info := map[string]string{
		"connected": strconv.FormatBool(current.SSID != "" || current.BSSID != ""),
	}
	if info["connected"] == "false" {
		return info, nil
	}
	if current.BSSID != "" {
		info["bssid"] = strings.ToLower(current.BSSID)
	}
	if current.RSSI != 0 {
		info["signal"] = strconv.Itoa(current.RSSI)
		info["signal_avg"] = strconv.Itoa(current.RSSI)
	}
	if current.Noise < 0 {
		info["noise"] = strconv.Itoa(current.Noise)
		if current.RSSI != 0 {
			info["snr"] = strconv.Itoa(current.RSSI - current.Noise)
		}
	}
	if current.Channel != 0 {
		info["channel"] = strconv.Itoa(current.Channel)
	}
	width := mapCWChannelWidth(current.ChannelWidth)
	if width != 0 {
		info["channel_width"] = strconv.Itoa(width)
	}
	if current.TxRate != 0 {
		info["tx_bitrate"] = strconv.FormatFloat(current.TxRate, 'f', -1, 64)
	}
	if rateInfo := cwBitrateInfoString(current.PhyMode, width, current.TxRate); rateInfo != "" {
		info["tx_bitrate_info"] = rateInfo
		info["rx_bitrate_info"] = rateInfo
	}
	if std := cwPhyModeStandard(current.PhyMode); std != "" {
		info["wifi_standard"] = std
	}
	info["rx_bytes"] = "0"
	info["tx_bytes"] = "0"
	info["rx_packets"] = "0"
	info["tx_packets"] = "0"
	info["tx_retries"] = "0"
	info["tx_failed"] = "0"
	info["connected_time"] = "0"
	return info, nil
}

func coreWLANStationInfo(iface string) (map[string]string, error) {
	current, err := coreWLANCurrentInfo(iface)
	if err != nil {
		return map[string]string{"connected": "false"}, err
	}
	stats := map[string]string{
		"connected": strconv.FormatBool(current.SSID != "" || current.BSSID != ""),
	}
	if stats["connected"] == "false" {
		return stats, nil
	}
	if current.BSSID != "" {
		stats["bssid"] = strings.ToLower(current.BSSID)
	}
	if current.RSSI != 0 {
		stats["signal"] = strconv.Itoa(current.RSSI)
		stats["signal_avg"] = strconv.Itoa(current.RSSI)
	}
	if current.Noise < 0 {
		stats["noise"] = strconv.Itoa(current.Noise)
		if current.RSSI != 0 {
			stats["snr"] = strconv.Itoa(current.RSSI - current.Noise)
		}
	}
	if current.TxRate != 0 {
		stats["tx_bitrate"] = strconv.FormatFloat(current.TxRate, 'f', -1, 64)
	}
	width := mapCWChannelWidth(current.ChannelWidth)
	if rateInfo := cwBitrateInfoString(current.PhyMode, width, current.TxRate); rateInfo != "" {
		stats["tx_bitrate_info"] = rateInfo
		stats["rx_bitrate_info"] = rateInfo
	}
	if std := cwPhyModeStandard(current.PhyMode); std != "" {
		stats["wifi_standard"] = std
	}
	stats["rx_bytes"] = "0"
	stats["tx_bytes"] = "0"
	stats["rx_packets"] = "0"
	stats["tx_packets"] = "0"
	stats["tx_retries"] = "0"
	stats["tx_failed"] = "0"
	stats["connected_time"] = "0"
	stats["last_ack_signal"] = "0"
	return stats, nil
}

func coreWLANCurrentInfo(iface string) (coreWLANCurrent, error) {
	cIface := C.CString(iface)
	defer C.free(unsafe.Pointer(cIface))
	jsonStr := C.cw_copy_current_json(cIface)
	defer C.cw_free(jsonStr)

	raw := C.GoString(jsonStr)
	if raw == "" || raw == "{}" {
		return coreWLANCurrent{}, errors.New("corewlan current info empty")
	}

	var current coreWLANCurrent
	if err := json.Unmarshal([]byte(raw), &current); err != nil {
		return coreWLANCurrent{}, fmt.Errorf("corewlan current json parse failed: %w", err)
	}
	return current, nil
}

// mapCWChannelWidth maps Apple's CWChannelWidth enum to MHz.
// Apple's enum: kCWChannelWidthUnknown=0, 20MHz=1, 40MHz=2, 80MHz=3, 160MHz=4,
// 320MHz=5 (added in macOS 15 for 802.11be / WiFi 7).
// Returns 0 for unknown so the caller can apply its own default.
func mapCWChannelWidth(width int) int {
	switch width {
	case 1:
		return 20
	case 2:
		return 40
	case 3:
		return 80
	case 4:
		return 160
	case 5:
		return 320
	default:
		return 0
	}
}

// cwPhyModeStandard maps Apple's CWPHYMode enum to the WiFi standard string
// the rest of the app uses. kCWPHYMode11be=7 was added in macOS 15.
func cwPhyModeStandard(mode int) string {
	switch mode {
	case 1:
		return "Legacy (802.11a/b/g)"
	case 2:
		return "Legacy (802.11a/b/g)"
	case 3:
		return "Legacy (802.11a/b/g)"
	case 4:
		return "WiFi 4 (802.11n)"
	case 5:
		return "WiFi 5 (802.11ac)"
	case 6:
		return "WiFi 6 (802.11ax)"
	case 7:
		return "WiFi 7 (802.11be)"
	default:
		return ""
	}
}

// cwPhyModeCapability returns the capability tag (HT/VHT/HE/EHT) for a
// CWPHYMode value, or "" for legacy / unknown modes.
func cwPhyModeCapability(mode int) string {
	switch mode {
	case 4:
		return "HT"
	case 5:
		return "VHT"
	case 6:
		return "HE"
	case 7:
		return "EHT"
	default:
		return ""
	}
}

// cwBitrateInfoString synthesises a bitrate-info string in the format
// parseBitrateInfo (wifi_utils.go) understands, so the existing service code
// can derive WiFiStandard / ChannelWidth / MIMOConfig from CoreWLAN data.
// CoreWLAN does not expose spatial-stream count, so MIMO ends up as the
// default 1x1.
func cwBitrateInfoString(phyMode, channelWidthMHz int, txRate float64) string {
	parts := make([]string, 0, 3)
	if txRate > 0 {
		parts = append(parts, fmt.Sprintf("%.1f MBit/s", txRate))
	}
	if cap := cwPhyModeCapability(phyMode); cap != "" {
		parts = append(parts, cap)
	}
	if channelWidthMHz > 0 {
		parts = append(parts, fmt.Sprintf("%dMHz", channelWidthMHz))
	}
	return strings.Join(parts, " ")
}

// cwBandAndFrequency converts a CoreWLAN (channel, channelBand) pair into our
// band string and a center frequency in MHz. CWChannelBand disambiguates the
// 2.4 / 5 / 6 GHz bands, which is required because 6 GHz channel numbers
// (1, 5, 9, ...) collide with 2.4 GHz. When the band is unknown, fall back to
// channelToFrequency, which guesses 2.4 GHz for low channel numbers.
//
// Apple enum: kCWChannelBandUnknown=0, 2GHz=1, 5GHz=2, 6GHz=3.
func cwBandAndFrequency(channel, band int) (string, int) {
	switch band {
	case 1:
		if channel == 14 {
			return "2.4GHz", 2484
		}
		if channel >= 1 && channel <= 13 {
			return "2.4GHz", 2407 + channel*5
		}
	case 2:
		return "5GHz", 5000 + channel*5
	case 3:
		return "6GHz", 5950 + channel*5
	}
	freq := channelToFrequency(channel)
	switch {
	default:
		return frequencyToBand(freq), freq
	}
}

func mapCWSecurity(sec int) string {
	switch sec {
	case 0:
		return "Open"
	case 1:
		return "WEP"
	case 2:
		return "WPA"
	case 3:
		return "WPA2"
	case 4:
		return "WPA3"
	case 5:
		return "OWE"
	default:
		return ""
	}
}

// defaultOUILookup is set by NewWiFiScanner after initialization.
var defaultOUILookup *OUILookup

func setCoreWLANLookup(lookup *OUILookup) {
	defaultOUILookup = lookup
}
