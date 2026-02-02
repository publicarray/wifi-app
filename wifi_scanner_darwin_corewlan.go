//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework CoreWLAN -framework Foundation

#import <CoreWLAN/CoreWLAN.h>
#import <Foundation/Foundation.h>
#import <stdlib.h>

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

static NSDictionary *cw_network_to_dict(CWNetwork *net) {
	NSMutableDictionary *dict = [NSMutableDictionary dictionary];
	if (net.ssid) {
		dict[@"ssid"] = net.ssid;
	}
	if (net.bssid) {
		dict[@"bssid"] = net.bssid;
	}
	dict[@"rssi"] = @(net.rssiValue);
	if (net.wlanChannel) {
		dict[@"channel"] = @(net.wlanChannel.channelNumber);
		dict[@"channelWidth"] = @(net.wlanChannel.channelWidth);
	}
	if ([net respondsToSelector:@selector(securityMode)]) {
		dict[@"security"] = @([net securityMode]);
	} else if ([net respondsToSelector:@selector(security)]) {
		dict[@"security"] = @([net security]);
	}
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
	dict[@"txRate"] = @(iface.transmitRate);
	if (iface.wlanChannel) {
		dict[@"channel"] = @(iface.wlanChannel.channelNumber);
		dict[@"channelWidth"] = @(iface.wlanChannel.channelWidth);
	}
	if ([iface respondsToSelector:@selector(securityMode)]) {
		dict[@"security"] = @([iface securityMode]);
	} else if ([iface respondsToSelector:@selector(security)]) {
		dict[@"security"] = @([iface security]);
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
	SSID         string `json:"ssid"`
	BSSID        string `json:"bssid"`
	RSSI         int    `json:"rssi"`
	Channel      int    `json:"channel"`
	ChannelWidth int    `json:"channelWidth"`
	Security     int    `json:"security"`
}

type coreWLANCurrent struct {
	SSID         string  `json:"ssid"`
	BSSID        string  `json:"bssid"`
	RSSI         int     `json:"rssi"`
	TxRate       float64 `json:"txRate"`
	Channel      int     `json:"channel"`
	ChannelWidth int     `json:"channelWidth"`
	Security     int     `json:"security"`
}

func coreWLANAvailable() bool {
	return true
}

func coreWLANScanNetworks(iface string) ([]AccessPoint, error) {
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
		if net.SSID == "" || net.BSSID == "" {
			continue
		}
		channelWidth := mapCWChannelWidth(net.ChannelWidth)
		if channelWidth == 0 {
			channelWidth = 20
		}
		freq := channelToFrequency(net.Channel)
		band := "2.4GHz"
		if freq > 5900 {
			band = "6GHz"
		} else if freq > 5000 {
			band = "5GHz"
		}

		securityField := mapCWSecurity(net.Security)
		security, ciphers, authMethods, pmf := parseAirportSecurity(securityField)

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
			Capabilities:    []string{},
			ChannelWidth:    channelWidth,
			Security:        security,
			SecurityCiphers: ciphers,
			AuthMethods:     authMethods,
			PMF:             pmf,
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

	conn := ConnectionInfo{
		Connected:    current.SSID != "" || current.BSSID != "",
		SSID:         current.SSID,
		BSSID:        strings.ToLower(current.BSSID),
		Channel:      current.Channel,
		Frequency:    channelToFrequency(current.Channel),
		Signal:       current.RSSI,
		SignalAvg:    current.RSSI,
		TxBitrate:    current.TxRate,
		RxBitrate:    0,
		WiFiStandard: "802.11ac/n",
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
	if current.Channel != 0 {
		info["channel"] = strconv.Itoa(current.Channel)
	}
	if width := mapCWChannelWidth(current.ChannelWidth); width != 0 {
		info["channel_width"] = strconv.Itoa(width)
	}
	if current.TxRate != 0 {
		info["tx_bitrate"] = strconv.FormatFloat(current.TxRate, 'f', -1, 64)
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
	if current.TxRate != 0 {
		stats["tx_bitrate"] = strconv.FormatFloat(current.TxRate, 'f', -1, 64)
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

func mapCWChannelWidth(width int) int {
	switch width {
	case 0:
		return 20
	case 1:
		return 40
	case 2:
		return 80
	case 3:
		return 160
	case 4:
		return 320
	default:
		return 0
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
	default:
		return ""
	}
}

// defaultOUILookup is set by NewWiFiScanner after initialization.
var defaultOUILookup *OUILookup

func setCoreWLANLookup(lookup *OUILookup) {
	defaultOUILookup = lookup
}
