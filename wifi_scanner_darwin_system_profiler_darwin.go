package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type systemProfilerParser struct {
	ouiLookup *OUILookup
}

func (p *systemProfilerParser) ParseScan(output []byte) ([]AccessPoint, error) {
	root, err := parseSystemProfilerJSON(output)
	if err != nil {
		return nil, err
	}

	items := getSlice(root["SPAirPortDataType"])
	if len(items) == 0 {
		return nil, fmt.Errorf("system_profiler output missing SPAirPortDataType")
	}

	var aps []AccessPoint
	for _, item := range items {
		itemMap := getMap(item)
		if itemMap == nil {
			continue
		}
		interfaces := getSlice(itemMap["spairport_airport_interfaces"])
		for _, iface := range interfaces {
			ifaceMap := getMap(iface)
			if ifaceMap == nil {
				continue
			}
			entries := extractNetworksFromInterface(ifaceMap)
			for _, entry := range entries {
				ssid := getString(entry, "spairport_network_name", "_name", "SSID_STR", "SSID")
				bssid := extractBSSID(getString(entry, "spairport_network_bssid", "spairport_network_bssid_string", "BSSID"))
				if ssid == "" || bssid == "" {
					continue
				}

				signal := getIntAny(entry, "spairport_network_rssi", "spairport_network_signal", "RSSI")
				if signal == 0 {
					signal = parseFirstInt(getString(entry, "spairport_network_rssi", "spairport_network_signal"))
				}
				channel := getIntAny(entry, "spairport_network_channel", "CHANNEL")
				if channel == 0 {
					channel = parseFirstInt(getString(entry, "spairport_network_channel", "spairport_network_channel_string"))
				}
				channelWidth := getIntAny(entry, "spairport_network_channel_width", "CHANNEL_WIDTH")
				if channelWidth == 0 {
					channelWidth = parseChannelWidth(getString(entry, "spairport_network_channel", "spairport_network_channel_string"))
				}
				if channelWidth == 0 {
					channelWidth = 20
				}

				freq := channelToFrequency(channel)
				band := "2.4GHz"
				if freq > 5900 {
					band = "6GHz"
				} else if freq > 5000 {
					band = "5GHz"
				}

				securityField := firstNonEmpty(
					getJoinedString(entry, "spairport_network_security"),
					getJoinedString(entry, "spairport_network_security_type"),
					getJoinedString(entry, "SECURITY"),
				)
				security, ciphers, authMethods, pmf := parseAirportSecurity(securityField)

				ap := AccessPoint{
					SSID:            ssid,
					BSSID:           bssid,
					Channel:         channel,
					Frequency:       freq,
					Band:            band,
					Signal:          signal,
					SignalQuality:   signalToQuality(signal),
					Vendor:          p.ouiLookup.LookupVendor(bssid),
					LastSeen:        time.Now(),
					Capabilities:    []string{},
					ChannelWidth:    channelWidth,
					Security:        security,
					SecurityCiphers: ciphers,
					AuthMethods:     authMethods,
					PMF:             pmf,
					CountryCode:     getString(entry, "spairport_network_country_code", "spairport_network_country"),
					Noise:           getIntAny(entry, "spairport_network_noise", "NOISE"),
				}

				if ap.Noise != 0 {
					ap.SNR = ap.Signal - ap.Noise
				}
				ap.DFS = isDFSChannel(ap.Channel)
				if ap.Security == "" {
					ap.Security = "Open"
				}
				aps = append(aps, ap)
			}
		}
	}

	if len(aps) == 0 {
		return nil, fmt.Errorf("system_profiler did not return any networks")
	}

	return aps, nil
}

func (p *systemProfilerParser) ParseLink(output []byte) (map[string]string, error) {
	return nil, fmt.Errorf("system_profiler link parsing not supported")
}

func (p *systemProfilerParser) ParseStation(output []byte) (map[string]string, error) {
	return nil, fmt.Errorf("system_profiler station parsing not supported")
}

func parseSystemProfilerCurrentNetwork(output []byte) map[string]string {
	info := map[string]string{
		"connected": "false",
	}
	root, err := parseSystemProfilerJSON(output)
	if err != nil {
		return info
	}

	items := getSlice(root["SPAirPortDataType"])
	for _, item := range items {
		itemMap := getMap(item)
		if itemMap == nil {
			continue
		}
		interfaces := getSlice(itemMap["spairport_airport_interfaces"])
		for _, iface := range interfaces {
			ifaceMap := getMap(iface)
			if ifaceMap == nil {
				continue
			}
			current := getMap(ifaceMap["spairport_current_network_information"])
			if current == nil {
				current = getMap(ifaceMap["spairport_current_network"])
			}
			if current == nil {
				continue
			}

			ssid := getString(current, "spairport_network_name", "_name", "SSID")
			bssid := extractBSSID(getString(current, "spairport_network_bssid", "spairport_network_bssid_string", "BSSID"))
			if ssid == "" && bssid == "" {
				continue
			}

			info["connected"] = "true"
			info["ssid"] = ssid
			info["bssid"] = bssid
			if channel := getString(current, "spairport_network_channel", "spairport_network_channel_string"); channel != "" {
				info["channel"] = channel
				if width := parseChannelWidth(channel); width != 0 {
					info["channel_width"] = fmt.Sprintf("%d", width)
				}
			}
			if signal := getString(current, "spairport_network_rssi", "spairport_network_signal", "RSSI"); signal != "" {
				info["signal"] = signal
				info["signal_avg"] = signal
			}
			if noise := getString(current, "spairport_network_noise", "NOISE"); noise != "" {
				info["noise"] = noise
			}
			if rx := getString(current, "spairport_network_last_rx_rate", "spairport_network_rx_rate", "RxRate"); rx != "" {
				info["rx_bitrate"] = rx
			}
			if tx := getString(current, "spairport_network_last_tx_rate", "spairport_network_tx_rate", "TxRate"); tx != "" {
				info["tx_bitrate"] = tx
			}
			if standard := getString(current, "spairport_network_phy_mode", "PHYMode"); standard != "" {
				info["wifi_standard"] = normalizeWiFiStandard(standard)
			}
			return info
		}
	}
	return info
}

func parseSystemProfilerJSON(output []byte) (map[string]interface{}, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(output, &root); err != nil {
		return nil, fmt.Errorf("failed to parse system_profiler JSON: %w", err)
	}
	return root, nil
}

func extractNetworksFromInterface(iface map[string]interface{}) []map[string]interface{} {
	for _, key := range []string{
		"spairport_networks",
		"spairport_other_local_wireless_networks",
		"spairport_other_local_networks",
		"spairport_scan_results",
	} {
		if entries := mapSlice(getSlice(iface[key])); len(entries) > 0 {
			return entries
		}
	}

	for _, value := range iface {
		if entries := mapSlice(getSlice(value)); len(entries) > 0 {
			return entries
		}
	}
	return nil
}

func mapSlice(slice []interface{}) []map[string]interface{} {
	var result []map[string]interface{}
	for _, item := range slice {
		itemMap := getMap(item)
		if itemMap == nil {
			continue
		}
		if isNetworkMap(itemMap) {
			result = append(result, itemMap)
		}
	}
	return result
}

func isNetworkMap(entry map[string]interface{}) bool {
	for _, key := range []string{
		"spairport_network_bssid",
		"spairport_network_name",
		"spairport_network_channel",
		"spairport_network_rssi",
		"spairport_network_signal",
		"spairport_network_security",
		"_name",
		"SSID",
		"BSSID",
	} {
		if _, ok := entry[key]; ok {
			return true
		}
	}
	return false
}

func getSlice(value interface{}) []interface{} {
	if value == nil {
		return nil
	}
	if slice, ok := value.([]interface{}); ok {
		return slice
	}
	return nil
}

func getMap(value interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}
	if m, ok := value.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
