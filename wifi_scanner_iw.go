//go:build linux && iw

package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// WiFiScanner handles WiFi scanning operations on Linux using iw
type WiFiScanner struct {
	currentInterface string
	ouiLookup        *OUILookup
	parser           ScanParser
}

// NewWiFiScanner creates a new WiFi scanner instance
func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

	return &WiFiScanner{
		ouiLookup: ouiLookup,
		parser:    &iwParser{ouiLookup: ouiLookup},
	}
}

// GetInterfaces returns a list of available WiFi interfaces
func (s *WiFiScanner) GetInterfaces() ([]string, error) {
	cmd := exec.Command("iw", "dev")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w (output: %s)", err, string(output))
	}

	var interfaces []string
	lines := strings.Split(string(output), "\n")
	interfaceRegex := regexp.MustCompile(`^\s+Interface\s+(\S+)`)

	for _, line := range lines {
		if matches := interfaceRegex.FindStringSubmatch(line); matches != nil {
			interfaces = append(interfaces, matches[1])
		}
	}

	if len(interfaces) == 0 {
		return nil, fmt.Errorf("no WiFi interfaces found (iw dev output had no Interface entries)")
	}

	return interfaces, nil
}

// ScanNetworks scans for available WiFi networks on the specified interface
func (s *WiFiScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
	cmd := exec.Command("iw", iface, "scan")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "Operation not permitted") {
			return nil, fmt.Errorf("permission denied: WiFi scanning requires elevated privileges. Run with sudo or set capabilities: sudo setcap cap_net_admin+ep ./wifi-app")
		}
		return nil, fmt.Errorf("failed to scan networks: %w (output: %s)", err, string(output))
	}

	if s.parser == nil {
		return nil, fmt.Errorf("no scan parser configured")
	}
	return s.parser.ParseScan(output)
}

// GetLinkInfo gets information about the current WiFi connection
func (s *WiFiScanner) GetLinkInfo(iface string) (map[string]string, error) {
	cmd := exec.Command("iw", iface, "link")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get link info: %w", err)
	}

	if s.parser == nil {
		return nil, fmt.Errorf("no link parser configured")
	}
	return s.parser.ParseLink(output)
}

// GetStationStats gets detailed statistics about the connected station
func (s *WiFiScanner) GetStationStats(iface string) (map[string]string, error) {
	cmd := exec.Command("iw", iface, "station", "dump")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get station stats: %w", err)
	}
	if s.parser == nil {
		return nil, fmt.Errorf("no station parser configured")
	}
	return s.parser.ParseStation(output)
}

func (s *WiFiScanner) Close() error {
	return nil
}
