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
	DFS           bool      `json:"dfs"`           // DFS (Dynamic Frequency Selection) channel
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
	WPS                bool    `json:"wps"`                // WPS (WiFi Protected Setup) status
	BSSLoadStations    *int    `json:"bssLoadStations"`    // Number of connected stations; nil when IE absent
	BSSLoadUtilization *int    `json:"bssLoadUtilization"` // Channel utilization percentage (0-100); nil when IE absent
	MaxPhyRate         int     `json:"maxPhyRate"`         // Max PHY rate in Mbps
	TWTSupport         bool    `json:"twtSupport"`         // Target Wake Time support (WiFi 6)
	NeighborReport     bool    `json:"neighborReport"`     // 802.11k Neighbor Report support
	MIMOStreams        int     `json:"mimoStreams"`        // Number of MIMO spatial streams (1-4)
	EstimatedRange     float64 `json:"estimatedRange"`     // Estimated range in meters based on TX power and signal
	SNR                int     `json:"snr"`                // Signal-to-noise ratio
	SurveyUtilization  int     `json:"surveyUtilization"`  // Channel busy percentage from survey info
	SurveyBusyMs       int     `json:"surveyBusyMs"`       // Channel busy time in ms
	SurveyExtBusyMs    int     `json:"surveyExtBusyMs"`    // External busy time in ms
	MaxTxPowerDbm      int     `json:"maxTxPowerDbm"`      // Max regulatory TX power in dBm
	// Security details
	SecurityCiphers []string `json:"securityCiphers"` // Encryption ciphers (CCMP, GCMP, TKIP, etc.)
	AuthMethods     []string `json:"authMethods"`     // Authentication methods (PSK, SAE, EAP, etc.)
	// WiFi 6/7 features
	BSSColor       int  `json:"bssColor"`       // BSS Color ID (WiFi 6)
	OBSSPD         bool `json:"obssPD"`         // OBSS PD (Spatial reuse) support
	QAMSupport     int  `json:"qamSupport"`     // Max QAM modulation (256, 1024, 4096)
	MUMIMO         bool `json:"mumimo"`         // MU-MIMO support

	// Derived fields
	WiFiGeneration string `json:"wifiGeneration"` // WiFi generation (4, 5, 6, 7)
	WiFiStandard   string `json:"wifiStandard"`   // Dominant WiFi standard (e.g. WiFi 6 (802.11ax))
	Beamforming    bool   `json:"beamforming"`    // Transmit beamforming support
	OFDMADownlink  bool `json:"ofdmaDownlink"`  // OFDMA downlink support (WiFi 6+, implicit when HE)
	OFDMAUplink    bool `json:"ofdmaUplink"`    // OFDMA uplink (HE MAC OFDMA RA Support bit)
	MLO            bool `json:"mlo"`            // Multi-Link Operation (WiFi 7) — Basic Multi-Link Element (ext-ID 107) present
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

// RoamingEvent represents a client roaming from one AP to another.
//
// DurationMs is an approximate measurement: wall-clock time between the last
// scan tick that confirmed the previous BSSID as active and the first scan
// tick that observed the new BSSID. It includes any intervening disconnect
// gap (lastBSSIDSeenAt is not updated while disconnected) and is naturally
// overestimated by up to one scan interval — the period between the real
// roam and our next opportunity to observe it. For sub-second resolution,
// drop `scan_interval_seconds` in the config; the default 4 s caps the
// useful precision of this metric.
//
// Severity thresholds used by the UI / report:
//   - < 500 ms:  healthy (802.11r/k/v typical)
//   - 500–2000 ms: slow (plain WPA2 re-association)
//   - ≥ 2000 ms:  bad (auth issues, 802.1X delays, AP co-op breakdown)
type RoamingEvent struct {
	Timestamp       time.Time `json:"timestamp"`
	PreviousBSSID   string    `json:"previousBssid"`
	NewBSSID        string    `json:"newBssid"`
	PreviousSignal  int       `json:"previousSignal"`
	NewSignal       int       `json:"newSignal"`
	PreviousChannel int       `json:"previousChannel"`
	NewChannel      int       `json:"newChannel"`
	DurationMs      int64     `json:"durationMs"` // 0 when we can't bound it (first-ever observed BSSID)
}

// SignalDataPoint represents a signal measurement at a specific time
type SignalDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Signal    int       `json:"signal"` // Signal in dBm
	BSSID     string    `json:"bssid"`  // Which AP the signal is from
}

// APSignalHistory is a per-BSSID time series of signal samples. The Signal
// tab uses this to keep the "Other APs" chart continuous across tab switches:
// the backend records every scan tick into ws.apSignalHistory regardless of
// whether the SignalChart component is currently mounted.
type APSignalHistory struct {
	BSSID  string            `json:"bssid"`
	SSID   string            `json:"ssid"`
	Points []SignalDataPoint `json:"points"`
}

// ClientStats represents the current client connection statistics
type ClientStats struct {
	Connected      bool              `json:"connected"`
	Interface      string            `json:"interface"`
	SSID           string            `json:"ssid"`
	BSSID          string            `json:"bssid"`
	LocalIP        string            `json:"localIp"` // Client IPv4 on the WiFi interface, "" when unavailable
	Gateway        string            `json:"gateway"` // Default gateway IPv4, "" when no default route
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

// RoamingQualityReport is the typed result of AnalyzeRoamingQuality.
// Previously this was returned as map[string]interface{}, which made the
// frontend type-cast every field.
//
// Duration aggregates (AvgRoamDurationMs, MaxRoamDurationMs, SlowRoamCount)
// are derived from RoamingEvent.DurationMs and inherit its "±1 scan
// interval" uncertainty — see RoamingEvent for details. SlowRoamCount uses
// the 2000 ms "auth issues" threshold from the plan; a small number of slow
// roams is a quality-of-service indicator, not a fault.
type RoamingQualityReport struct {
	TotalRoams        int    `json:"totalRoams"`
	GoodRoams         int    `json:"goodRoams"`
	BadRoams          int    `json:"badRoams"`
	AvgSignalChange   int    `json:"avgSignalChange"`
	ExcessiveRoaming  bool   `json:"excessiveRoaming"`
	StickyClient      bool   `json:"stickyClient"`
	TimeSinceLastRoam string `json:"timeSinceLastRoam,omitempty"`
	RoamingAdvice     string `json:"roamingAdvice"`
	AvgRoamDurationMs int64  `json:"avgRoamDurationMs"`
	MaxRoamDurationMs int64  `json:"maxRoamDurationMs"`
	SlowRoamCount     int    `json:"slowRoamCount"`
}

// LatencyProbe is a single RTT measurement toward a target. One probe per
// target per sampler tick. Lost probes have RTTMs == 0 and Lost == true so
// the frontend can plot them as gaps rather than zeros.
type LatencyProbe struct {
	Timestamp time.Time `json:"timestamp"`
	Target    string    `json:"target"`    // resolved IP or hostname
	Label     string    `json:"label"`     // user-facing label (e.g. "gateway", "1.1.1.1")
	RTTMs     float64   `json:"rttMs"`     // round-trip time in milliseconds; 0 if lost
	Lost      bool      `json:"lost"`      // true when the probe timed out or failed
	Transport string    `json:"transport"` // "icmp", "tcp", or "udp"
}

// LatencyStats rolls up probes for a target over a single window (1s / 10s /
// 60s). Loss percent is population, not moving average — "N lost / N total in
// window". A Samples count of 0 means the window hasn't accumulated any data
// yet (typical right after the sampler starts).
type LatencyStats struct {
	WindowSeconds int     `json:"windowSeconds"`
	Samples       int     `json:"samples"`
	MinMs         float64 `json:"minMs"`
	AvgMs         float64 `json:"avgMs"`
	MaxMs         float64 `json:"maxMs"`
	StddevMs      float64 `json:"stddevMs"`
	LossPercent   float64 `json:"lossPercent"`
}

// LatencyTargetSummary is everything the frontend needs to render a summary
// card + chart series for one target: the label, the resolved address, the
// latest probe, rolling stats across windows, and a bounded raw history.
type LatencyTargetSummary struct {
	Label      string         `json:"label"`
	Target     string         `json:"target"`
	Transport  string         `json:"transport"`
	Available  bool           `json:"available"` // false when the target can't currently be resolved (e.g. no gateway)
	LastProbe  *LatencyProbe  `json:"lastProbe,omitempty"`
	Windows    []LatencyStats `json:"windows"` // 1s / 10s / 60s windows
	History    []LatencyProbe `json:"history"` // bounded raw history for the chart
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

type ConnectionInfo struct {
	Connected    bool    `json:"connected"`
	SSID         string  `json:"ssid"`
	BSSID        string  `json:"bssid"`
	Channel      int     `json:"channel"`
	Frequency    int     `json:"frequency"`
	Signal       int     `json:"signal"`
	SignalAvg    int     `json:"signalAvg"`
	RxBitrate    float64 `json:"rxBitrate"`
	TxBitrate    float64 `json:"txBitrate"`
	WiFiStandard string  `json:"wifiStandard"`
	ChannelWidth int     `json:"channelWidth"`
	MIMOConfig   string  `json:"mimoConfig"`
}

// StationStats represents detailed WiFi station statistics
type StationStats struct {
	Connected    bool    `json:"connected"`
	Signal       int     `json:"signal"`
	SignalAvg    int     `json:"signalAvg"`
	Noise        int     `json:"noise"`
	SNR          int     `json:"snr"`
	RxBitrate    float64 `json:"rxBitrate"`
	TxBitrate    float64 `json:"txBitrate"`
	WiFiStandard string  `json:"wifiStandard"`
	ChannelWidth int     `json:"channelWidth"`
	MIMOConfig   string  `json:"mimoConfig"`
}
