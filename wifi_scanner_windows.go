package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type windowsScanner struct {
	currentInterface string
	ouiLookup        *OUILookup
}

func newWindowsScanner() IWiFiScanner {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	cacheFile := filepath.Join(cacheDir, "wifi-app", "oui.txt")

	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		cacheFile = filepath.Join(os.TempDir(), "oui.txt")
	}

	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

	return &windowsScanner{
		ouiLookup: ouiLookup,
	}
}

func (s *windowsScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
	cmd := exec.Command("netsh", "wlan", "show", "networks", "mode=bssid", "interface="+iface)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to scan networks: %w (output: %s)", err, string(output))
	}

	return s.parseNetshScanOutput(string(output))
}

func (s *windowsScanner) parseNetshScanOutput(output string) ([]AccessPoint, error) {
	lines := strings.Split(output, "\n")
	var aps []AccessPoint
	var currentAP *AccessPoint

	bssidRegex := regexp.MustCompile(`BSSID\s+([0-9a-f:]+)`)
	ssidRegex := regexp.MustCompile(`SSID\s+\d+\s*:\s+(.+)`)
	signalRegex := regexp.MustCompile(`Signal\s*:\s+(\d+)%`)
	channelRegex := regexp.MustCompile(`Channel\s*:\s+(\d+)`)
	authRegex := regexp.MustCompile(`Authentication\s*:\s+(.+)`)
	encryptionRegex := regexp.MustCompile(`Encryption\s*:\s+(.+)`)

	for _, line := range lines {
		if matches := ssidRegex.FindStringSubmatch(line); matches != nil {
			if currentAP != nil {
				aps = append(aps, *currentAP)
			}
			currentAP = &AccessPoint{
				Vendor:        s.ouiLookup.LookupVendor(line),
				SSID:          strings.TrimSpace(matches[1]),
				SignalQuality: 50,
				LastSeen:      s.getCurrentTime(),
				Capabilities:  []string{},
			}
		}
		if matches := bssidRegex.FindStringSubmatch(line); matches != nil && currentAP != nil {
			currentAP.BSSID = matches[1]
		}
		if matches := signalRegex.FindStringSubmatch(line); matches != nil && currentAP != nil {
			if signal, err := strconv.Atoi(matches[1]); err != nil {
				currentAP.Signal = (signal - 100)
				currentAP.SignalQuality = signal
			}
		}
		if matches := channelRegex.FindStringSubmatch(line); matches != nil && currentAP != nil {
			if ch, err := strconv.Atoi(matches[1]); err != nil {
				currentAP.Channel = ch
			}
		}
		if matches := authRegex.FindStringSubmatch(line); matches != nil && currentAP != nil {
			auth := strings.TrimSpace(matches[1])
			if auth == "WPA2-Personal" || auth == "WPA2-Enterprise" {
				currentAP.Security = "WPA2"
			} else if auth == "WPA3-Personal" || auth == "WPA3-SAE" {
				currentAP.Security = "WPA3"
			} else if auth == "WPA" {
				currentAP.Security = "WPA"
			} else if auth == "Open" {
				currentAP.Security = "Open"
			} else {
				currentAP.Security = auth
			}
		}
		if matches := encryptionRegex.FindStringSubmatch(line); matches != nil && currentAP != nil && currentAP.Security == "" {
			enc := strings.TrimSpace(matches[1])
			if enc == "AES" || enc == "CCMP" {
				currentAP.Security = "WPA2"
			} else if enc == "TKIP" {
				currentAP.Security = "WPA"
			}
		}
	}

	if currentAP != nil {
		aps = append(aps, *currentAP)
	}

	for i := range aps {
		aps[i].ChannelWidth = 20
		aps[i].Frequency = 2400
		aps[i].Band = "2.4GHz"
		if aps[i].Security == "" {
			aps[i].Security = "Open"
		}
	}

	return aps, nil
}

func (s *windowsScanner) GetInterfaces() ([]string, error) {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w (output: %s)", err, string(output))
	}

	lines := strings.Split(string(output), "\n")
	var interfaces []string
	nameRegex := regexp.MustCompile(`:\s+(\S+)`)

	for _, line := range lines {
		if strings.Contains(line, "Wi-Fi") {
			if matches := nameRegex.FindStringSubmatch(line); matches != nil {
				name := strings.TrimSpace(matches[1])
				if name != "" && !contains(interfaces, name) {
					interfaces = append(interfaces, name)
				}
			}
		}
	}

	if len(interfaces) == 0 {
		return nil, fmt.Errorf("no WiFi interfaces found")
	}

	return interfaces, nil
}

func (s *windowsScanner) GetConnectionInfo(iface string) (ConnectionInfo, error) {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces", fmt.Sprintf("interface=%s", iface))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("failed to get connection info: %w", err)
	}

	return ConnectionInfo{}, fmt.Errorf("connection info not implemented for Windows")
}

func (s *windowsScanner) GetStationStats(iface string) (StationStats, error) {
	return StationStats{}, fmt.Errorf("station stats not implemented for Windows")
}

func (s *windowsScanner) Close() error {
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getCurrentTime() time.Time {
	return time.Now()
}

func signalToQuality(signal int) int {
	if signal >= -30 {
		return 100
	}
	if signal <= -100 {
		return 0
	}
	return int((float64(signal+100) / 70.0) * 100)
}
