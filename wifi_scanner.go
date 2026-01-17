package main

// type IWiFiScanner interface {
// 	NewWiFiScanner() IWiFiScanner
// 	ScanNetworks(iface string) ([]AccessPoint, error)
// 	GetInterfaces() ([]string, error)
// 	GetConnectionInfo(iface string) (ConnectionInfo, error)
// 	GetStationStats(iface string) (StationStats, error)
// 	Close() error
// }

// type ConnectionInfo struct {
// 	Connected    bool
// 	SSID         string
// 	BSSID        string
// 	Frequency    float64
// 	Channel      int
// 	ChannelWidth int
// 	WiFiStandard string
// 	MIMOConfig   string
// 	Signal       int
// 	TxBitrate    float64
// 	RxBitrate    float64
// 	TxBytes      uint64
// 	RxBytes      uint64
// }

// type StationStats struct {
// 	Connected     bool
// 	Interface     string
// 	SSID          string
// 	BSSID         string
// 	Frequency     float64
// 	Channel       int
// 	ChannelWidth  int
// 	WiFiStandard  string
// 	MIMOConfig    string
// 	Signal        int
// 	SignalAvg     int
// 	TxBitrate     float64
// 	RxBitrate     float64
// 	TxBytes       uint64
// 	RxBytes       uint64
// 	TxPackets     uint64
// 	RxPackets     uint64
// 	TxRetries     uint64
// 	TxFailed      uint64
// 	RetryRate     float64
// 	ConnectedTime int
// 	LastAckSignal int
// 	Noise         int
// 	SNR           int
// }

// func NewWiFiScanner() IWiFiScanner {
// 	return newLinuxScanner()
// }
