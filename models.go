package main

import "time"

// AccessPoint represents a single WiFi access point (BSSID)
type AccessPoint struct {
	BSSID         string    `json:"bssid"`         // MAC address of the AP
	SSID          string    `json:"ssid"`          // Network name
	Vendor        string    `json:"vendor"`        // Manufacturer/vendor name
	Frequency     int       `json:"frequency"`     // Frequency in MHz
	Channel       int       `json:"channel"`       // WiFi channel number
	ChannelWidth  int       `json:"channelWidth"`  // Channel width in MHz (20, 40, 80, 160)
	Signal        int       `json:"signal"`        // Signal strength in dBm
	SignalQuality int       `json:"signalQuality"` // Signal quality percentage (0-100)
	Noise         int       `json:"noise"`         // Noise level in dBm
	TxPower       int       `json:"txPower"`       // Transmit power in dBm
	Security      string    `json:"security"`      // Security type (WPA2, WPA3, etc.)
	Band          string    `json:"band"`          // 2.4GHz or 5GHz
	LastSeen      time.Time `json:"lastSeen"`      // Last time this AP was seen
	Capabilities  []string  `json:"capabilities"`  // AP capabilities (HT, VHT, HE, etc.)
	BeaconInt     int       `json:"beaconInt"`     // Beacon interval in TU
	// Advanced capabilities
	BSSTransition bool   `json:"bsstransition"` // BSS Transition Management support (802.11v)
	UAPSD         bool   `json:"uapsd"`         // Unsolicited Automatic Power Save Delivery
	FastRoaming   bool   `json:"fastroaming"`   // Fast BSS Transition (802.11r)
	DTIM          int    `json:"dtim"`          // DTIM (Delivery Traffic Indication Message) interval
	PMF           string `json:"pmf"`           // Protected Management Frames (Required, Optional, Disabled)
	// Additional advanced metrics
	WPS                 bool    `json:"wps"`                 // WPS (WiFi Protected Setup) status
	BSSLoadStations     int     `json:"bssLoadStations"`     // Number of connected stations
	BSSLoadUtilization  int     `json:"bssLoadUtilization"`  // Channel utilization percentage (0-255)
	MaxTheoreticalSpeed int     `json:"maxTheoreticalSpeed"` // Max theoretical throughput in Mbps
	TWTSupport          bool    `json:"twtSupport"`          // Target Wake Time support (WiFi 6)
	NeighborReport      bool    `json:"neighborReport"`      // 802.11k Neighbor Report support
	MIMOStreams         int     `json:"mimoStreams"`         // Number of MIMO spatial streams (1-4)
	RealWorldSpeed      int     `json:"realWorldSpeed"`      // Expected real-world throughput in Mbps (~60-70% of theoretical)
	EstimatedRange      float64 `json:"estimatedRange"`      // Estimated range in meters based on TX power and signal
	SNR                 int     `json:"snr"`                 // Signal-to-noise ratio
	// Security details
	SecurityCiphers []string `json:"securityCiphers"` // Encryption ciphers (CCMP, GCMP, TKIP, etc.)
	AuthMethods     []string `json:"authMethods"`     // Authentication methods (PSK, SAE, EAP, etc.)
	// WiFi 6/7 features
	BSSColor   int  `json:"bssColor"`   // BSS Color ID (WiFi 6)
	OBSSPD     bool `json:"obssPD"`     // OBSS PD (Spatial reuse) support
	QAMSupport int  `json:"qamSupport"` // Max QAM modulation (256, 1024, 4096)
	MUMIMO     bool `json:"mumimo"`     // MU-MIMO support
	// Network management
	QoSSupport  bool   `json:"qosSupport"`  // WMM/QoS support
	CountryCode string `json:"countryCode"` // Regulatory country code (US, EU, etc.)
	APName      string `json:"apName"`      // AP name/description if advertised
}

// Network represents a WiFi network (SSID) that may have multiple access points
type Network struct {
	SSID          string        `json:"ssid"`
	AccessPoints  []AccessPoint `json:"accessPoints"`
	BestSignal    int           `json:"bestSignal"`    // Strongest signal among all APs
	BestSignalAP  string        `json:"bestSignalAP"`  // BSSID of AP with best signal
	Channel       int           `json:"channel"`       // Primary channel (from best AP)
	Security      string        `json:"security"`      // Security type
	APCount       int           `json:"apCount"`       // Number of APs for this SSID
	HasIssues     bool          `json:"hasIssues"`     // True if misconfigurations detected
	IssueMessages []string      `json:"issueMessages"` // List of detected issues
}

// RoamingEvent represents a client roaming from one AP to another
type RoamingEvent struct {
	Timestamp       time.Time `json:"timestamp"`
	PreviousBSSID   string    `json:"previousBssid"`
	NewBSSID        string    `json:"newBssid"`
	PreviousSignal  int       `json:"previousSignal"`
	NewSignal       int       `json:"newSignal"`
	PreviousChannel int       `json:"previousChannel"`
	NewChannel      int       `json:"newChannel"`
}

// SignalDataPoint represents a signal measurement at a specific time
type SignalDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Signal    int       `json:"signal"` // Signal in dBm
	BSSID     string    `json:"bssid"`  // Which AP the signal is from
}

// ClientStats represents the current client connection statistics
type ClientStats struct {
	Connected      bool              `json:"connected"`
	Interface      string            `json:"interface"`
	SSID           string            `json:"ssid"`
	BSSID          string            `json:"bssid"`
	Frequency      float64           `json:"frequency"` // Frequency in MHz
	Channel        int               `json:"channel"`
	ChannelWidth   int               `json:"channelWidth"` // Channel width in MHz (20, 40, 80, 160, 320)
	WiFiStandard   string            `json:"wifiStandard"` // WiFi standard (802.11a/b/g/n/ac/ax/be)
	MIMOConfig     string            `json:"mimoConfig"`   // MIMO configuration (e.g., "2x2", "4x4")
	Signal         int               `json:"signal"`       // Current signal in dBm
	SignalAvg      int               `json:"signalAvg"`    // Average signal in dBm
	Noise          int               `json:"noise"`        // Noise floor in dBm (if available)
	SNR            int               `json:"snr"`          // Signal-to-noise ratio (Signal - Noise)
	TxBitrate      float64           `json:"txBitrate"`    // TX bitrate in Mbps
	RxBitrate      float64           `json:"rxBitrate"`    // RX bitrate in Mbps
	TxBytes        uint64            `json:"txBytes"`
	RxBytes        uint64            `json:"rxBytes"`
	TxPackets      uint64            `json:"txPackets"`
	RxPackets      uint64            `json:"rxPackets"`
	TxRetries      uint64            `json:"txRetries"`
	TxFailed       uint64            `json:"txFailed"`
	RetryRate      float64           `json:"retryRate"`      // Retry rate percentage
	ConnectedTime  int               `json:"connectedTime"`  // Connection duration in seconds
	LastAckSignal  int               `json:"lastAckSignal"`  // Last ACK signal in dBm
	SignalHistory  []SignalDataPoint `json:"signalHistory"`  // Signal over time
	RoamingHistory []RoamingEvent    `json:"roamingHistory"` // History of roaming events
}

// ChannelInfo represents information about a WiFi channel
type ChannelInfo struct {
	Channel          int      `json:"channel"`
	Frequency        int      `json:"frequency"`
	Band             string   `json:"band"`             // 2.4GHz or 5GHz
	NetworkCount     int      `json:"networkCount"`     // Number of networks on this channel
	Networks         []string `json:"networks"`         // SSIDs on this channel
	Utilization      int      `json:"utilization"`      // Channel utilization percentage
	CongestionLevel  string   `json:"congestionLevel"`  // "low", "medium", "high"
	OverlappingCount int      `json:"overlappingCount"` // Number of overlapping networks
}

// ScanResult represents the complete result of a WiFi scan
type ScanResult struct {
	Timestamp     time.Time     `json:"timestamp"`
	Interface     string        `json:"interface"`
	Networks      []Network     `json:"networks"`
	Channels      []ChannelInfo `json:"channels"`
	TotalAPs      int           `json:"totalAPs"`
	TotalNetworks int           `json:"totalNetworks"`
}
