package main

// WiFiBackend defines the interface that all WiFi scanner backends must implement
type WiFiBackend interface {
	// GetInterfaces returns a list of available WiFi interfaces
	GetInterfaces() ([]string, error)

	// ScanNetworks scans for available WiFi networks on the specified interface
	ScanNetworks(iface string) ([]AccessPoint, error)

	// GetLinkInfo gets information about the current WiFi connection
	GetLinkInfo(iface string) (map[string]string, error)

	// GetStationStats gets detailed statistics about the connected station
	GetStationStats(iface string) (map[string]string, error)

	// Close performs any necessary cleanup
	Close() error
}
