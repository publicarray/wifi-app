//go:build windows

package main

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows WLAN API constants
const (
	WLAN_API_VERSION = 2

	// WLAN_INTERFACE_STATE
	wlanInterfaceStateNotReady           = 0
	wlanInterfaceStateConnected          = 1
	wlanInterfaceStateAdHocNetworkFormed = 2
	wlanInterfaceStateDisconnecting      = 3
	wlanInterfaceStateDisconnected       = 4
	wlanInterfaceStateAssociating        = 5
	wlanInterfaceStateDiscovering        = 6
	wlanInterfaceStateAuthenticating     = 7

	// WLAN_INTF_OPCODE
	wlanIntfOpcodeCurrentConnection = 7
	wlanIntfOpcodeChannelNumber     = 8
	wlanIntfOpcodeStatistics        = 0x10000101
	wlanIntfOpcodeRssi              = 0x10000102

	// DOT11_BSS_TYPE
	dot11BssTypeInfrastructure = 1
	dot11BssTypeIndependent    = 2
	dot11BssTypeAny            = 3

	// DOT11_PHY_TYPE
	dot11PhyTypeUnknown    = 0
	dot11PhyTypeFhss       = 1
	dot11PhyTypeDsss       = 2
	dot11PhyTypeIrBaseband = 3
	dot11PhyTypeOfdm       = 4  // 802.11a
	dot11PhyTypeHrDsss     = 5  // 802.11b
	dot11PhyTypeErp        = 6  // 802.11g
	dot11PhyTypeHt         = 7  // 802.11n
	dot11PhyTypeVht        = 8  // 802.11ac
	dot11PhyTypeDmg        = 9  // 802.11ad
	dot11PhyTypeHe         = 10 // 802.11ax (WiFi 6)
	dot11PhyTypeEht        = 11 // 802.11be (WiFi 7)

	// DOT11_AUTH_ALGORITHM
	dot11AuthAlgo80211Open      = 1
	dot11AuthAlgo80211SharedKey = 2
	dot11AuthAlgoWPA            = 3
	dot11AuthAlgoWPAPSK         = 4
	dot11AuthAlgoWPANone        = 5
	dot11AuthAlgoRSNA           = 6 // WPA2-Enterprise
	dot11AuthAlgoRSNAPSK        = 7 // WPA2-Personal
	dot11AuthAlgoWPA3           = 8 // WPA3-Enterprise 192-bit
	dot11AuthAlgoWPA3SAE        = 9 // WPA3-Personal
	dot11AuthAlgoOWE            = 10
	dot11AuthAlgoWPA3ENT        = 11

	// DOT11_CIPHER_ALGORITHM
	dot11CipherAlgoNone    = 0x00
	dot11CipherAlgoWEP40   = 0x01
	dot11CipherAlgoTKIP    = 0x02
	dot11CipherAlgoCCMP    = 0x04
	dot11CipherAlgoWEP104  = 0x05
	dot11CipherAlgoBIP     = 0x06
	dot11CipherAlgoGCMP    = 0x08
	dot11CipherAlgoGCMP256 = 0x09
	dot11CipherAlgoCCMP256 = 0x0a
	dot11CipherAlgoWEP     = 0x101

	// Capability bits
	capabilityESS     = 0x0001
	capabilityIBSS    = 0x0002
	capabilityPrivacy = 0x0010

	// Constants
	dot11SSIDMaxLength = 32
	wlanMaxNameLength  = 256
	wlanMaxRateSetSize = 126
)

// Windows WLAN API structures

// DOT11_SSID represents an 802.11 SSID
type DOT11_SSID struct {
	SSIDLength uint32
	SSID       [dot11SSIDMaxLength]byte
}

// DOT11_MAC_ADDRESS is a 6-byte MAC address
type DOT11_MAC_ADDRESS [6]byte

// WLAN_INTERFACE_INFO contains information about a wireless interface
type WLAN_INTERFACE_INFO struct {
	InterfaceGUID        windows.GUID
	InterfaceDescription [wlanMaxNameLength]uint16
	State                uint32
}

// WLAN_INTERFACE_INFO_LIST contains an array of interface info
type WLAN_INTERFACE_INFO_LIST struct {
	NumberOfItems uint32
	Index         uint32
	InterfaceInfo [1]WLAN_INTERFACE_INFO
}

// WLAN_RATE_SET contains supported data rates
type WLAN_RATE_SET struct {
	RateSetLength uint32
	RateSet       [wlanMaxRateSetSize]uint16
}

// WLAN_BSS_ENTRY contains information about a BSS (access point)
type WLAN_BSS_ENTRY struct {
	Dot11SSID             DOT11_SSID
	PhyID                 uint32
	Dot11BSSID            DOT11_MAC_ADDRESS
	Dot11BSSType          uint32
	Dot11BSSPhyType       uint32
	RSSI                  int32
	LinkQuality           uint32
	InRegDomain           uint8
	BeaconPeriod          uint16
	Timestamp             uint64
	HostTimestamp         uint64
	CapabilityInformation uint16
	ChCenterFrequency     uint32
	WlanRateSet           WLAN_RATE_SET
	IEOffset              uint32
	IESize                uint32
}

// WLAN_BSS_LIST contains a list of BSS entries
type WLAN_BSS_LIST struct {
	TotalSize      uint32
	NumberOfItems  uint32
	WlanBssEntries [1]WLAN_BSS_ENTRY
}

// WLAN_ASSOCIATION_ATTRIBUTES contains association attributes
type WLAN_ASSOCIATION_ATTRIBUTES struct {
	Dot11SSID         DOT11_SSID
	Dot11BSSType      uint32
	Dot11BSSID        DOT11_MAC_ADDRESS
	Dot11PhyType      uint32
	Dot11PhyIndex     uint32
	WlanSignalQuality uint32
	RxRate            uint32 // in Kbps
	TxRate            uint32 // in Kbps
}

// WLAN_SECURITY_ATTRIBUTES contains security attributes
type WLAN_SECURITY_ATTRIBUTES struct {
	SecurityEnabled      int32
	OneXEnabled          int32
	Dot11AuthAlgorithm   uint32
	Dot11CipherAlgorithm uint32
}

// WLAN_CONNECTION_ATTRIBUTES contains connection attributes
type WLAN_CONNECTION_ATTRIBUTES struct {
	State                 uint32
	ConnectionMode        uint32
	ProfileName           [wlanMaxNameLength]uint16
	AssociationAttributes WLAN_ASSOCIATION_ATTRIBUTES
	SecurityAttributes    WLAN_SECURITY_ATTRIBUTES
}

// WLAN_STATISTICS contains wireless LAN statistics
type WLAN_STATISTICS struct {
	FourWayHandshakeFailures   uint64
	TKIPCounterMeasuresInvoked uint64
	Reserved                   uint64
	// ... more fields exist but we only need basic ones
}

// wlanapi.dll function bindings
var (
	wlanAPI               = windows.NewLazySystemDLL("wlanapi.dll")
	wlanOpenHandle        = wlanAPI.NewProc("WlanOpenHandle")
	wlanCloseHandle       = wlanAPI.NewProc("WlanCloseHandle")
	wlanEnumInterfaces    = wlanAPI.NewProc("WlanEnumInterfaces")
	wlanQueryInterface    = wlanAPI.NewProc("WlanQueryInterface")
	wlanScan              = wlanAPI.NewProc("WlanScan")
	wlanGetNetworkBssList = wlanAPI.NewProc("WlanGetNetworkBssList")
	wlanFreeMemory        = wlanAPI.NewProc("WlanFreeMemory")
)

type windowsScanner struct {
	ouiLookup    *OUILookup
	clientHandle uintptr
	mu           sync.Mutex
}

func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

	scanner := &windowsScanner{
		ouiLookup: ouiLookup,
	}

	// Open WLAN handle
	if err := scanner.openHandle(); err != nil {
		// Handle will be opened on first use if this fails
		fmt.Printf("Warning: Failed to open WLAN handle on init: %v\n", err)
	}

	return scanner
}

func (s *windowsScanner) openHandle() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.clientHandle != 0 {
		return nil
	}

	var negotiatedVersion uint32
	var clientHandle uintptr

	ret, _, _ := wlanOpenHandle.Call(
		uintptr(WLAN_API_VERSION),
		0,
		uintptr(unsafe.Pointer(&negotiatedVersion)),
		uintptr(unsafe.Pointer(&clientHandle)),
	)

	if ret != 0 {
		return fmt.Errorf("WlanOpenHandle failed with error: %d", ret)
	}

	s.clientHandle = clientHandle
	return nil
}

func (s *windowsScanner) ensureHandle() error {
	if s.clientHandle == 0 {
		return s.openHandle()
	}
	return nil
}

func (s *windowsScanner) GetInterfaces() ([]string, error) {
	if err := s.ensureHandle(); err != nil {
		return nil, err
	}

	var interfaceList *WLAN_INTERFACE_INFO_LIST

	ret, _, _ := wlanEnumInterfaces.Call(
		s.clientHandle,
		0,
		uintptr(unsafe.Pointer(&interfaceList)),
	)

	if ret != 0 {
		return nil, fmt.Errorf("WlanEnumInterfaces failed with error: %d", ret)
	}

	if interfaceList == nil {
		return nil, fmt.Errorf("no WiFi interfaces found")
	}

	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(interfaceList)))

	if interfaceList.NumberOfItems == 0 {
		return nil, fmt.Errorf("no WiFi interfaces found")
	}

	interfaces := make([]string, 0, interfaceList.NumberOfItems)

	// Calculate the size of each interface info entry
	infoSize := unsafe.Sizeof(WLAN_INTERFACE_INFO{})

	for i := uint32(0); i < interfaceList.NumberOfItems; i++ {
		// Get pointer to the i-th interface info
		infoPtr := unsafe.Add(unsafe.Pointer(&interfaceList.InterfaceInfo[0]), uintptr(i)*infoSize)
		info := (*WLAN_INTERFACE_INFO)(infoPtr)

		// Convert GUID to string for interface identification
		guidStr := guidToString(info.InterfaceGUID)
		interfaces = append(interfaces, guidStr)
	}

	return interfaces, nil
}

func (s *windowsScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
	if err := s.ensureHandle(); err != nil {
		return nil, err
	}

	// Parse interface GUID
	guid, err := stringToGUID(iface)
	if err != nil {
		// If not a GUID, try to find interface by description
		guid, err = s.findInterfaceGUID(iface)
		if err != nil {
			return nil, fmt.Errorf("invalid interface: %w", err)
		}
	}

	// Trigger a scan (async operation)
	ret, _, _ := wlanScan.Call(
		s.clientHandle,
		uintptr(unsafe.Pointer(&guid)),
		0, // pDot11Ssid - NULL for all networks
		0, // pIeData - NULL
		0, // pReserved
	)

	if ret != 0 {
		// Don't fail if scan trigger fails, we might still have cached results
		fmt.Printf("Warning: WlanScan failed with error: %d\n", ret)
	}

	// Wait a bit for scan to complete
	// In production, you'd want to use WlanRegisterNotification for async notification
	time.Sleep(500 * time.Millisecond)

	// Get BSS list
	var bssList *WLAN_BSS_LIST

	ret, _, _ = wlanGetNetworkBssList.Call(
		s.clientHandle,
		uintptr(unsafe.Pointer(&guid)),
		0, // pDot11Ssid - NULL for all SSIDs
		uintptr(dot11BssTypeAny),
		0, // bSecurityEnabled - FALSE to get all
		0, // pReserved
		uintptr(unsafe.Pointer(&bssList)),
	)

	if ret != 0 {
		return nil, fmt.Errorf("WlanGetNetworkBssList failed with error: %d", ret)
	}

	if bssList == nil {
		return []AccessPoint{}, nil
	}

	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(bssList)))

	aps := make([]AccessPoint, 0, bssList.NumberOfItems)

	// Calculate size of BSS entry for pointer arithmetic
	entrySize := unsafe.Sizeof(WLAN_BSS_ENTRY{})

	for i := uint32(0); i < bssList.NumberOfItems; i++ {
		// Get pointer to the i-th BSS entry
		entryPtr := unsafe.Add(unsafe.Pointer(&bssList.WlanBssEntries[0]), uintptr(i)*entrySize)
		entry := (*WLAN_BSS_ENTRY)(entryPtr)

		ap := s.bssEntryToAccessPoint(entry)
		aps = append(aps, ap)
	}

	return aps, nil
}

func (s *windowsScanner) bssEntryToAccessPoint(entry *WLAN_BSS_ENTRY) AccessPoint {
	ssid := formatSSID(entry.Dot11SSID)
	bssid := formatMACAddress(entry.Dot11BSSID)

	// Frequency is in kHz, convert to MHz
	frequency := int(entry.ChCenterFrequency / 1000)
	channel := frequencyToChannel(frequency)

	band := "2.4GHz"
	if frequency > 5900 {
		band = "6GHz"
	} else if frequency > 5000 {
		band = "5GHz"
	}

	// Determine security from capability bits
	security := "Open"
	if entry.CapabilityInformation&capabilityPrivacy != 0 {
		security = "WEP" // At minimum, will be refined if we parse IEs
	}

	// WiFi standard from PHY type
	wifiStandard := phyTypeToStandard(entry.Dot11BSSPhyType)

	// Calculate signal quality percentage
	signalQuality := int(entry.LinkQuality)
	if signalQuality > 100 {
		signalQuality = 100
	}

	ap := AccessPoint{
		SSID:          ssid,
		BSSID:         bssid,
		Vendor:        s.ouiLookup.LookupVendor(bssid),
		Frequency:     frequency,
		Channel:       channel,
		ChannelWidth:  20, // Default, would need IE parsing for actual width
		Signal:        int(entry.RSSI),
		SignalQuality: signalQuality,
		Security:      security,
		Band:          band,
		LastSeen:      time.Now(),
		Capabilities:  []string{wifiStandard},
		BeaconInt:     int(entry.BeaconPeriod),
	}

	// Parse Information Elements for additional details if available
	if entry.IESize > 0 && entry.IEOffset > 0 {
		s.parseInformationElements(&ap, entry)
	}

	return ap
}

func (s *windowsScanner) parseInformationElements(ap *AccessPoint, entry *WLAN_BSS_ENTRY) {
	// IE data starts at offset IEOffset from the beginning of the WLAN_BSS_ENTRY
	// This is a simplified parser - full IE parsing is complex

	iePtr := unsafe.Add(unsafe.Pointer(entry), uintptr(entry.IEOffset))
	ieData := unsafe.Slice((*byte)(iePtr), entry.IESize)

	offset := uint32(0)
	for offset+2 <= entry.IESize {
		elementID := ieData[offset]
		length := ieData[offset+1]

		if offset+2+uint32(length) > entry.IESize {
			break
		}

		data := ieData[offset+2 : offset+2+uint32(length)]

		switch elementID {
		case 48: // RSN (WPA2/WPA3)
			ap.Security = parseRSNSecurity(data)
		case 221: // Vendor specific (WPA1, WMM, etc.)
			if length >= 4 {
				// Check for WPA OUI: 00:50:F2:01
				if data[0] == 0x00 && data[1] == 0x50 && data[2] == 0xF2 && data[3] == 0x01 {
					if ap.Security == "Open" || ap.Security == "WEP" {
						ap.Security = "WPA"
					}
				}
				// Check for WMM/QoS OUI: 00:50:F2:02
				if data[0] == 0x00 && data[1] == 0x50 && data[2] == 0xF2 && data[3] == 0x02 {
					ap.QoSSupport = true
				}
			}
		case 45: // HT Capabilities (802.11n)
			if length >= 2 {
				htCaps := uint16(data[0]) | uint16(data[1])<<8
				if htCaps&0x0002 != 0 { // Channel width 40MHz supported
					ap.ChannelWidth = 40
				}
			}
		case 191: // VHT Capabilities (802.11ac)
			if length >= 4 {
				vhtCaps := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
				// Check supported channel width
				switch (vhtCaps >> 2) & 0x03 {
				case 1:
					ap.ChannelWidth = 160
				case 2:
					ap.ChannelWidth = 160 // 80+80
				default:
					if ap.ChannelWidth < 80 {
						ap.ChannelWidth = 80
					}
				}
				// Check MU-MIMO
				if vhtCaps&(1<<19) != 0 {
					ap.MUMIMO = true
				}
			}
		case 255: // Extension element
			if length >= 1 {
				extID := data[0]
				switch extID {
				case 35: // HE Capabilities (802.11ax)
					ap.Capabilities = append(ap.Capabilities, "WiFi6")
				case 106: // EHT Capabilities (802.11be)
					ap.Capabilities = append(ap.Capabilities, "WiFi7")
				}
			}
		}

		offset += 2 + uint32(length)
	}
}

func parseRSNSecurity(data []byte) string {
	if len(data) < 8 {
		return "WPA2"
	}

	// Skip version (2 bytes) and group cipher suite (4 bytes)
	offset := 6

	if len(data) < offset+2 {
		return "WPA2"
	}

	// Pairwise cipher count
	pairwiseCount := uint16(data[offset]) | uint16(data[offset+1])<<8
	offset += 2

	// Skip pairwise cipher suites
	offset += int(pairwiseCount) * 4

	if len(data) < offset+2 {
		return "WPA2"
	}

	// AKM count
	akmCount := uint16(data[offset]) | uint16(data[offset+1])<<8
	offset += 2

	// Check AKM suites for WPA3
	for i := uint16(0); i < akmCount && offset+4 <= len(data); i++ {
		akmSuite := data[offset : offset+4]
		// OUI 00:0F:AC (IEEE)
		if akmSuite[0] == 0x00 && akmSuite[1] == 0x0F && akmSuite[2] == 0xAC {
			switch akmSuite[3] {
			case 8: // SAE (WPA3-Personal)
				return "WPA3"
			case 12, 13: // WPA3-Enterprise
				return "WPA3-Enterprise"
			case 18: // OWE
				return "OWE"
			}
		}
		offset += 4
	}

	return "WPA2"
}

func (s *windowsScanner) GetConnectionInfo(iface string) (ConnectionInfo, error) {
	if err := s.ensureHandle(); err != nil {
		return ConnectionInfo{}, err
	}

	guid, err := s.resolveInterfaceGUID(iface)
	if err != nil {
		return ConnectionInfo{}, err
	}

	// Query current connection
	var dataSize uint32
	var connAttr *WLAN_CONNECTION_ATTRIBUTES

	ret, _, _ := wlanQueryInterface.Call(
		s.clientHandle,
		uintptr(unsafe.Pointer(&guid)),
		uintptr(wlanIntfOpcodeCurrentConnection),
		0,
		uintptr(unsafe.Pointer(&dataSize)),
		uintptr(unsafe.Pointer(&connAttr)),
		0,
	)

	if ret != 0 {
		// Not connected or error
		return ConnectionInfo{Connected: false}, nil
	}

	if connAttr == nil {
		return ConnectionInfo{Connected: false}, nil
	}

	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(connAttr)))

	// Check if actually connected
	if connAttr.State != wlanInterfaceStateConnected {
		return ConnectionInfo{Connected: false}, nil
	}

	assoc := &connAttr.AssociationAttributes

	// Get channel
	channel, _ := s.queryChannel(guid)

	// Get RSSI
	rssi, _ := s.queryRSSI(guid)
	if rssi == 0 {
		// Estimate from signal quality
		rssi = int32(-100 + int32(assoc.WlanSignalQuality)/2)
	}

	info := ConnectionInfo{
		Connected:    true,
		SSID:         formatSSID(assoc.Dot11SSID),
		BSSID:        formatMACAddress(assoc.Dot11BSSID),
		Channel:      channel,
		Signal:       int(rssi),
		SignalAvg:    int(rssi),
		RxBitrate:    float64(assoc.RxRate) / 1000.0, // Convert Kbps to Mbps
		TxBitrate:    float64(assoc.TxRate) / 1000.0,
		WiFiStandard: phyTypeToStandard(assoc.Dot11PhyType),
		ChannelWidth: 20,    // Would need deeper query for actual width
		MIMOConfig:   "1x1", // Would need driver-specific query
	}

	return info, nil
}

func (s *windowsScanner) queryChannel(guid windows.GUID) (int, error) {
	var dataSize uint32
	var channel *uint32

	ret, _, _ := wlanQueryInterface.Call(
		s.clientHandle,
		uintptr(unsafe.Pointer(&guid)),
		uintptr(wlanIntfOpcodeChannelNumber),
		0,
		uintptr(unsafe.Pointer(&dataSize)),
		uintptr(unsafe.Pointer(&channel)),
		0,
	)

	if ret != 0 || channel == nil {
		return 0, fmt.Errorf("failed to query channel")
	}

	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(channel)))
	return int(*channel), nil
}

func (s *windowsScanner) queryRSSI(guid windows.GUID) (int32, error) {
	var dataSize uint32
	var rssi *int32

	ret, _, _ := wlanQueryInterface.Call(
		s.clientHandle,
		uintptr(unsafe.Pointer(&guid)),
		uintptr(wlanIntfOpcodeRssi),
		0,
		uintptr(unsafe.Pointer(&dataSize)),
		uintptr(unsafe.Pointer(&rssi)),
		0,
	)

	if ret != 0 || rssi == nil {
		return 0, fmt.Errorf("failed to query RSSI")
	}

	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(rssi)))
	return *rssi, nil
}

func (s *windowsScanner) GetLinkInfo(iface string) (map[string]string, error) {
	info, err := s.GetConnectionInfo(iface)
	if err != nil {
		return map[string]string{"connected": "false"}, err
	}

	if !info.Connected {
		return map[string]string{"connected": "false"}, nil
	}

	return map[string]string{
		"connected":      "true",
		"ssid":           info.SSID,
		"bssid":          info.BSSID,
		"channel":        fmt.Sprintf("%d", info.Channel),
		"signal":         fmt.Sprintf("%d", info.Signal),
		"signal_avg":     fmt.Sprintf("%d", info.SignalAvg),
		"rx_bitrate":     fmt.Sprintf("%.1f", info.RxBitrate),
		"tx_bitrate":     fmt.Sprintf("%.1f", info.TxBitrate),
		"rx_bytes":       "0", // Would need performance counters
		"tx_bytes":       "0",
		"rx_packets":     "0",
		"tx_packets":     "0",
		"tx_retries":     "0",
		"tx_failed":      "0",
		"connected_time": "0",
	}, nil
}

func (s *windowsScanner) GetStationStats(iface string) (map[string]string, error) {
	info, err := s.GetConnectionInfo(iface)
	if err != nil {
		return map[string]string{"connected": "false"}, err
	}

	if !info.Connected {
		return map[string]string{"connected": "false"}, nil
	}

	return map[string]string{
		"connected":       "true",
		"bssid":           info.BSSID,
		"signal":          fmt.Sprintf("%d", info.Signal),
		"signal_avg":      fmt.Sprintf("%d", info.SignalAvg),
		"rx_bitrate":      fmt.Sprintf("%.1f", info.RxBitrate),
		"tx_bitrate":      fmt.Sprintf("%.1f", info.TxBitrate),
		"rx_bytes":        "0",
		"tx_bytes":        "0",
		"rx_packets":      "0",
		"tx_packets":      "0",
		"tx_retries":      "0",
		"tx_failed":       "0",
		"connected_time":  "0",
		"last_ack_signal": "0",
	}, nil
}

func (s *windowsScanner) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.clientHandle != 0 {
		wlanCloseHandle.Call(s.clientHandle, 0)
		s.clientHandle = 0
	}
	return nil
}

// Helper functions

func (s *windowsScanner) resolveInterfaceGUID(iface string) (windows.GUID, error) {
	guid, err := stringToGUID(iface)
	if err == nil {
		return guid, nil
	}
	return s.findInterfaceGUID(iface)
}

func (s *windowsScanner) findInterfaceGUID(name string) (windows.GUID, error) {
	var interfaceList *WLAN_INTERFACE_INFO_LIST

	ret, _, _ := wlanEnumInterfaces.Call(
		s.clientHandle,
		0,
		uintptr(unsafe.Pointer(&interfaceList)),
	)

	if ret != 0 || interfaceList == nil {
		return windows.GUID{}, fmt.Errorf("failed to enumerate interfaces")
	}

	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(interfaceList)))

	// If name is empty or "Wi-Fi", return the first interface
	if name == "" || name == "Wi-Fi" || name == "WiFi" {
		if interfaceList.NumberOfItems > 0 {
			return interfaceList.InterfaceInfo[0].InterfaceGUID, nil
		}
	}

	// Search by description
	infoSize := unsafe.Sizeof(WLAN_INTERFACE_INFO{})
	for i := uint32(0); i < interfaceList.NumberOfItems; i++ {
		infoPtr := unsafe.Add(unsafe.Pointer(&interfaceList.InterfaceInfo[0]), uintptr(i)*infoSize)
		info := (*WLAN_INTERFACE_INFO)(infoPtr)

		desc := syscall.UTF16ToString(info.InterfaceDescription[:])
		if desc == name {
			return info.InterfaceGUID, nil
		}
	}

	// Return first interface if no match
	if interfaceList.NumberOfItems > 0 {
		return interfaceList.InterfaceInfo[0].InterfaceGUID, nil
	}

	return windows.GUID{}, fmt.Errorf("interface not found: %s", name)
}

func guidToString(guid windows.GUID) string {
	return fmt.Sprintf("{%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X}",
		guid.Data1, guid.Data2, guid.Data3,
		guid.Data4[0], guid.Data4[1],
		guid.Data4[2], guid.Data4[3], guid.Data4[4], guid.Data4[5], guid.Data4[6], guid.Data4[7])
}

func stringToGUID(s string) (windows.GUID, error) {
	var guid windows.GUID

	// Try parsing with braces
	n, err := fmt.Sscanf(s, "{%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X}",
		&guid.Data1, &guid.Data2, &guid.Data3,
		&guid.Data4[0], &guid.Data4[1],
		&guid.Data4[2], &guid.Data4[3], &guid.Data4[4], &guid.Data4[5], &guid.Data4[6], &guid.Data4[7])

	if err == nil && n == 11 {
		return guid, nil
	}

	// Try without braces
	n, err = fmt.Sscanf(s, "%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X",
		&guid.Data1, &guid.Data2, &guid.Data3,
		&guid.Data4[0], &guid.Data4[1],
		&guid.Data4[2], &guid.Data4[3], &guid.Data4[4], &guid.Data4[5], &guid.Data4[6], &guid.Data4[7])

	if err == nil && n == 11 {
		return guid, nil
	}

	return windows.GUID{}, fmt.Errorf("invalid GUID format: %s", s)
}

func formatSSID(ssid DOT11_SSID) string {
	if ssid.SSIDLength == 0 {
		return ""
	}
	length := ssid.SSIDLength
	if length > dot11SSIDMaxLength {
		length = dot11SSIDMaxLength
	}
	return string(ssid.SSID[:length])
}

func formatMACAddress(mac DOT11_MAC_ADDRESS) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

func phyTypeToStandard(phyType uint32) string {
	switch phyType {
	case dot11PhyTypeOfdm:
		return "802.11a"
	case dot11PhyTypeHrDsss:
		return "802.11b"
	case dot11PhyTypeErp:
		return "802.11g"
	case dot11PhyTypeHt:
		return "802.11n"
	case dot11PhyTypeVht:
		return "802.11ac"
	case dot11PhyTypeDmg:
		return "802.11ad"
	case dot11PhyTypeHe:
		return "802.11ax"
	case dot11PhyTypeEht:
		return "802.11be"
	default:
		return "802.11"
	}
}
