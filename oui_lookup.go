package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// OUILookup handles MAC address vendor lookups.
//
// The lookup starts in "minimal" mode — a small embedded vendor table is
// seeded synchronously so Vendor columns are never empty. A richer database
// is downloaded or loaded from cache in the background; when that finishes
// the map is swapped under a write lock and onReady is invoked (if set) so
// the UI can refresh.
type OUILookup struct {
	mu        sync.RWMutex
	ouiMap    map[string]string
	loaded    bool
	cacheFile string
	onReady   func()
}

// NewOUILookup creates a new OUI lookup service
func NewOUILookup(cacheFile string) *OUILookup {
	return &OUILookup{
		ouiMap:    make(map[string]string),
		cacheFile: cacheFile,
	}
}

// SetReadyCallback registers a callback invoked once the full OUI database
// has loaded in the background. Useful for emitting a Wails event so the
// frontend can re-render Vendor columns.
func (o *OUILookup) SetReadyCallback(cb func()) {
	o.mu.Lock()
	o.onReady = cb
	o.mu.Unlock()
}

// LoadOUIDatabase seeds the minimal embedded OUI table synchronously, then
// kicks off the full database load (from cache or network) in the background.
// Returns immediately so it's safe to call from constructors.
func (o *OUILookup) LoadOUIDatabase() error {
	o.mu.Lock()
	o.loadMinimalDatabaseLocked()
	o.loaded = true
	o.mu.Unlock()

	go o.loadFullDatabaseAsync()
	return nil
}

// loadFullDatabaseAsync resolves the full database, either from a reasonably
// fresh on-disk cache or by downloading a new copy with bounded timeout +
// retries. On success it atomically replaces the lookup map under the write
// lock and fires onReady.
func (o *OUILookup) loadFullDatabaseAsync() {
	const cacheMaxAge = 30 * 24 * time.Hour

	useCache := false
	if stat, err := os.Stat(o.cacheFile); err == nil && stat.Size() > 0 && time.Since(stat.ModTime()) < cacheMaxAge {
		useCache = true
	}

	if !useCache {
		if err := o.downloadWithRetry(o.cacheFile); err != nil {
			slog.Warn("oui database download failed", "err", err)
		}
	}

	full, err := loadOUIMapFromFile(o.cacheFile)
	if err != nil || len(full) == 0 {
		// Cache missing or unreadable — try a download one more time.
		if err := o.downloadWithRetry(o.cacheFile); err == nil {
			full, _ = loadOUIMapFromFile(o.cacheFile)
		}
	}
	if len(full) == 0 {
		// Stick with the minimal database already loaded by LoadOUIDatabase.
		slog.Warn("oui database unavailable, vendor lookups will use minimal embedded table",
			"cache", o.cacheFile)
		return
	}

	// Merge the minimal seed so well-known WiFi vendors survive even if the
	// upstream database is missing them.
	for mac, vendor := range minimalOUIs {
		if _, exists := full[mac]; !exists {
			full[mac] = vendor
		}
	}

	o.mu.Lock()
	o.ouiMap = full
	cb := o.onReady
	o.mu.Unlock()

	if cb != nil {
		cb()
	}
}

// downloadWithRetry downloads the OUI CSV with a 10s per-attempt timeout and
// up to 3 attempts separated by short backoff. Blocking the startup path on
// a 30s timeout was the old behaviour; this keeps the background load bounded.
func (o *OUILookup) downloadWithRetry(path string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := o.downloadOUIDatabase(path); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(time.Duration(1<<attempt) * time.Second)
	}
	return lastErr
}

// downloadOUIDatabase downloads the OUI database from maclookup.app
func (o *OUILookup) downloadOUIDatabase(filepath string) error {
	url := "https://maclookup.app/downloads/csv-database/get-db"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/plain, text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	tmpPath := filepath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(file, resp.Body)
	file.Close()

	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, filepath)
}

// loadOUIMapFromFile parses the OUI CSV and returns a fresh map without
// touching any OUILookup instance. This lets the async loader build the full
// database off-lock, then hand the finished map to the lookup in one swap.
func loadOUIMapFromFile(filepath string) (map[string]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	out := make(map[string]string)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// CSV format: Mac Prefix,Vendor Name,Private,Block Type,Last Updated
		if len(record) >= 2 {
			macPrefix := strings.TrimSpace(record[0])
			vendorName := strings.TrimSpace(record[1])

			if macPrefix != "" && vendorName != "" {
				if normalized := normalizeOUIPrefix(macPrefix); normalized != "" {
					out[normalized] = vendorName
				}
			}
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no valid OUI entries found in file")
	}

	return out, nil
}

func normalizeOUIPrefix(prefix string) string {
	if prefix == "" {
		return ""
	}
	p := strings.ToUpper(strings.TrimSpace(prefix))
	p = strings.ReplaceAll(p, "-", "")
	p = strings.ReplaceAll(p, ":", "")
	if len(p) < 6 {
		return ""
	}
	p = p[:6]
	// Reject anything that isn't 6 hex digits — guards against header rows
	// in CSV input (e.g. "Mac Prefix") or other junk being silently inserted
	// as a key the lookup will never hit.
	for i := 0; i < 6; i++ {
		c := p[i]
		isHex := (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')
		if !isHex {
			return ""
		}
	}
	return fmt.Sprintf("%s:%s:%s", p[0:2], p[2:4], p[4:6])
}

// minimalOUIs is the single source of truth for the embedded vendor table.
// It is merged with any full database loaded from disk/network so well-known
// WiFi-equipment OUIs always resolve even if the upstream CSV is incomplete.
var minimalOUIs = map[string]string{
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

// loadMinimalDatabaseLocked seeds the lookup with the embedded minimal table.
// Caller must hold the write lock.
func (o *OUILookup) loadMinimalDatabaseLocked() {
	for mac, vendor := range minimalOUIs {
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
