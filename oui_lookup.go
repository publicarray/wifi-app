package main

import (
	"encoding/csv"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// OUILookup handles MAC address vendor lookups
type OUILookup struct {
	ouiMap map[string]string
	mu     sync.RWMutex
	loaded bool
}

// NewOUILookup creates a new OUI lookup service
func NewOUILookup() *OUILookup {
	return &OUILookup{
		ouiMap: make(map[string]string),
	}
}

// LoadOUIDatabase loads the OUI database from a local file or downloads it
func (o *OUILookup) LoadOUIDatabase() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Try to load from local cache first
	cacheFile := filepath.Join(os.TempDir(), "oui.txt")
	
	// Check if cache exists and is not empty
	if stat, err := os.Stat(cacheFile); err == nil && stat.Size() > 0 {
		if err := o.loadFromFile(cacheFile); err == nil {
			o.loaded = true
			return nil
		}
	}

	// Download if cache doesn't exist or failed to load
	if err := o.downloadOUIDatabase(cacheFile); err != nil {
		// If download fails, use embedded minimal database
		o.loadMinimalDatabase()
		o.loaded = true
		return nil
	}

	if err := o.loadFromFile(cacheFile); err != nil {
		o.loadMinimalDatabase()
	}

	o.loaded = true
	return nil
}

// downloadOUIDatabase downloads the IEEE OUI database
func (o *OUILookup) downloadOUIDatabase(filepath string) error {
	// IEEE OUI database URL
	url := "http://standards-oui.ieee.org/oui/oui.txt"
	
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// loadFromFile loads OUI data from a file
func (o *OUILookup) loadFromFile(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	lines := make([]string, 0)
	data, _ := io.ReadAll(file)
	lines = strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// OUI format: XX-XX-XX   (hex)		Vendor Name
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			mac := strings.Replace(parts[0], "-", ":", -1)
			mac = strings.ToUpper(mac)
			
			// Find the vendor name (after "(hex)")
			hexIndex := strings.Index(line, "(hex)")
			if hexIndex > 0 && len(line) > hexIndex+5 {
				vendor := strings.TrimSpace(line[hexIndex+5:])
				o.ouiMap[mac] = vendor
			}
		}
	}

	return nil
}

// loadMinimalDatabase loads a minimal embedded OUI database for common vendors
func (o *OUILookup) loadMinimalDatabase() {
	// Common WiFi equipment manufacturers
	commonOUIs := map[string]string{
		// Ubiquiti (Unifi)
		"00:27:22": "Ubiquiti Networks",
		"24:5A:4C": "Ubiquiti Networks",
		"68:D7:9A": "Ubiquiti Networks",
		"70:A7:41": "Ubiquiti Networks",
		"74:83:C2": "Ubiquiti Networks",
		"78:8A:20": "Ubiquiti Networks",
		"80:2A:A8": "Ubiquiti Networks",
		"B4:FB:E4": "Ubiquiti Networks",
		"DC:9F:DB": "Ubiquiti Networks",
		"F0:9F:C2": "Ubiquiti Networks",
		"FC:EC:DA": "Ubiquiti Networks",
		"1E:6A:1B": "Ubiquiti Networks",
		
		// Cisco/Linksys
		"00:00:0C": "Cisco Systems",
		"00:01:42": "Cisco Systems",
		"00:01:43": "Cisco Systems",
		"00:01:96": "Cisco Systems",
		"00:01:97": "Cisco Systems",
		"00:01:C7": "Cisco Systems",
		"00:02:3D": "Cisco Systems",
		"00:02:4A": "Cisco Systems",
		"00:02:4B": "Cisco Systems",
		"00:02:B9": "Cisco Systems",
		"00:02:BA": "Cisco Systems",
		"00:02:FC": "Cisco Systems",
		"00:02:FD": "Cisco Systems",
		"00:03:6B": "Cisco Systems",
		"00:03:6C": "Cisco Systems",
		"00:03:9F": "Cisco Systems",
		"00:03:A0": "Cisco Systems",
		"00:03:E3": "Cisco Systems",
		"00:03:E4": "Cisco Systems",
		"00:03:FD": "Cisco Systems",
		"00:03:FE": "Cisco Systems",
		
		// TP-Link
		"00:27:19": "TP-Link",
		"10:FE:ED": "TP-Link",
		"14:CF:92": "TP-Link",
		"1C:3B:F3": "TP-Link",
		"50:C7:BF": "TP-Link",
		"54:A5:1B": "TP-Link",
		"60:E3:27": "TP-Link",
		"98:DE:D0": "TP-Link",
		"A0:F3:C1": "TP-Link",
		"AC:84:C6": "TP-Link",
		"C0:4A:00": "TP-Link",
		"E8:94:F6": "TP-Link",
		"EC:08:6B": "TP-Link",
		
		// Netgear
		"00:09:5B": "Netgear",
		"00:0F:B5": "Netgear",
		"00:14:6C": "Netgear",
		"00:18:4D": "Netgear",
		"00:1B:2F": "Netgear",
		"00:1E:2A": "Netgear",
		"00:22:3F": "Netgear",
		"00:24:B2": "Netgear",
		"00:26:F2": "Netgear",
		"20:E5:2A": "Netgear",
		"28:C6:8E": "Netgear",
		"30:46:9A": "Netgear",
		"A0:21:B7": "Netgear",
		"C0:3F:0E": "Netgear",
		"E0:46:9A": "Netgear",
		
		// Aruba Networks
		"00:0B:86": "Aruba Networks",
		"00:1A:1E": "Aruba Networks",
		"00:24:6C": "Aruba Networks",
		"20:4C:03": "Aruba Networks",
		"24:DE:C6": "Aruba Networks",
		"6C:F3:7F": "Aruba Networks",
		"70:3A:0E": "Aruba Networks",
		"94:B4:0F": "Aruba Networks",
		"D8:C7:C8": "Aruba Networks",
		
		// Ruckus Wireless
		"00:24:A8": "Ruckus Wireless",
		"24:C9:A1": "Ruckus Wireless",
		"2C:30:33": "Ruckus Wireless",
		"58:93:96": "Ruckus Wireless",
		"88:DC:96": "Ruckus Wireless",
		"C4:10:8A": "Ruckus Wireless",
		
		// Meraki (Cisco)
		"00:18:0A": "Cisco Meraki",
		"88:15:44": "Cisco Meraki",
		"E0:55:3D": "Cisco Meraki",
		"E0:CB:BC": "Cisco Meraki",
		
		// MikroTik
		"00:0C:42": "MikroTik",
		"4C:5E:0C": "MikroTik",
		"6C:3B:6B": "MikroTik",
		"D4:CA:6D": "MikroTik",
		"E6:8D:8C": "MikroTik",
		
		// Apple
		"00:03:93": "Apple",
		"00:0A:27": "Apple",
		"00:0A:95": "Apple",
		"00:0D:93": "Apple",
		"00:10:FA": "Apple",
		"00:11:24": "Apple",
		"00:14:51": "Apple",
		"00:16:CB": "Apple",
		"00:17:F2": "Apple",
		"00:19:E3": "Apple",
		"00:1B:63": "Apple",
		"00:1C:B3": "Apple",
		"00:1D:4F": "Apple",
		"00:1E:52": "Apple",
		"00:1E:C2": "Apple",
		"00:1F:5B": "Apple",
		"00:1F:F3": "Apple",
		"00:21:E9": "Apple",
		"00:22:41": "Apple",
		"00:23:12": "Apple",
		"00:23:32": "Apple",
		"00:23:6C": "Apple",
		"00:23:DF": "Apple",
		"00:24:36": "Apple",
		"00:25:00": "Apple",
		"00:25:4B": "Apple",
		"00:25:BC": "Apple",
		"00:26:08": "Apple",
		"00:26:4A": "Apple",
		"00:26:B0": "Apple",
		"00:26:BB": "Apple",
	}

	for mac, vendor := range commonOUIs {
		o.ouiMap[mac] = vendor
	}
}

// LookupVendor looks up the vendor for a given MAC address
func (o *OUILookup) LookupVendor(macAddress string) string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if !o.loaded {
		return "Unknown"
	}

	// Extract OUI (first 3 octets)
	macAddress = strings.ToUpper(macAddress)
	macAddress = strings.ReplaceAll(macAddress, "-", ":")
	
	parts := strings.Split(macAddress, ":")
	if len(parts) < 3 {
		return "Unknown"
	}

	oui := strings.Join(parts[:3], ":")

	if vendor, ok := o.ouiMap[oui]; ok {
		return vendor
	}

	return "Unknown"
}

// IsLoaded returns whether the OUI database has been loaded
func (o *OUILookup) IsLoaded() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.loaded
}
