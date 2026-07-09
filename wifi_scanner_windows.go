//go:build windows

package main

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	WLAN_API_VERSION = 2

	wlanInterfaceStateNotReady           = 0
	wlanInterfaceStateConnected          = 1
	wlanInterfaceStateAdHocNetworkFormed = 2
	wlanInterfaceStateDisconnecting      = 3
	wlanInterfaceStateDisconnected       = 4
	wlanInterfaceStateAssociating        = 5
	wlanInterfaceStateDiscovering        = 6
	wlanInterfaceStateAuthenticating     = 7

	wlanIntfOpcodeCurrentConnection = 7
	wlanIntfOpcodeChannelNumber     = 8
	wlanIntfOpcodeRssi              = 0x10000102

	dot11BssTypeInfrastructure = 1
	dot11BssTypeIndependent    = 2
	dot11BssTypeAny            = 3

	dot11PhyTypeUnknown    = 0
	dot11PhyTypeFhss       = 1
	dot11PhyTypeDsss       = 2
	dot11PhyTypeIrBaseband = 3
	dot11PhyTypeOfdm       = 4
	dot11PhyTypeHrDsss     = 5
	dot11PhyTypeErp        = 6
	dot11PhyTypeHt         = 7
	dot11PhyTypeVht        = 8
	dot11PhyTypeDmg        = 9
	dot11PhyTypeHe         = 10
	dot11PhyTypeEht        = 11

	dot11AuthAlgo80211Open      = 1
	dot11AuthAlgo80211SharedKey = 2
	dot11AuthAlgoWPA            = 3
	dot11AuthAlgoWPAPSK         = 4
	dot11AuthAlgoWPANone        = 5
	dot11AuthAlgoRSNA           = 6
	dot11AuthAlgoRSNAPSK        = 7
	dot11AuthAlgoWPA3           = 8
	dot11AuthAlgoWPA3SAE        = 9
	dot11AuthAlgoOWE            = 10
	dot11AuthAlgoWPA3ENT        = 11

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

	capabilityESS     = 0x0001
	capabilityIBSS    = 0x0002
	capabilityPrivacy = 0x0010

	dot11SSIDMaxLength  = 32
	wlanMaxNameLength   = 256
	wlanMaxRateSetSize  = 126
	ifMaxStringSize     = 256
	ifMaxPhysAddressLen = 32
	ifTypeIEEE80211     = 71
)

type DOT11_SSID struct {
	SSIDLength uint32
	SSID       [dot11SSIDMaxLength]byte
}

type DOT11_MAC_ADDRESS [6]byte

type WLAN_INTERFACE_INFO struct {
	InterfaceGUID        windows.GUID
	InterfaceDescription [wlanMaxNameLength]uint16
	State                uint32
}

type WLAN_INTERFACE_INFO_LIST struct {
	NumberOfItems uint32
	Index         uint32
	InterfaceInfo [1]WLAN_INTERFACE_INFO
}

type WLAN_RATE_SET struct {
	RateSetLength uint32
	RateSet       [wlanMaxRateSetSize]uint16
}

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

type WLAN_BSS_LIST struct {
	TotalSize      uint32
	NumberOfItems  uint32
	WlanBssEntries [1]WLAN_BSS_ENTRY
}

type WLAN_ASSOCIATION_ATTRIBUTES struct {
	Dot11SSID         DOT11_SSID
	Dot11BSSType      uint32
	Dot11BSSID        DOT11_MAC_ADDRESS
	Dot11PhyType      uint32
	Dot11PhyIndex     uint32
	WlanSignalQuality uint32
	RxRate            uint32
	TxRate            uint32
}

type WLAN_SECURITY_ATTRIBUTES struct {
	SecurityEnabled      int32
	OneXEnabled          int32
	Dot11AuthAlgorithm   uint32
	Dot11CipherAlgorithm uint32
}

type WLAN_CONNECTION_ATTRIBUTES struct {
	State                 uint32
	ConnectionMode        uint32
	ProfileName           [wlanMaxNameLength]uint16
	AssociationAttributes WLAN_ASSOCIATION_ATTRIBUTES
	SecurityAttributes    WLAN_SECURITY_ATTRIBUTES
}

type NET_LUID struct {
	Value uint64
}

type MIB_IF_ROW2 struct {
	InterfaceLuid               NET_LUID
	InterfaceIndex              uint32
	InterfaceGuid               windows.GUID
	Alias                       [ifMaxStringSize + 1]uint16
	Description                 [ifMaxStringSize + 1]uint16
	PhysicalAddressLength       uint32
	PhysicalAddress             [ifMaxPhysAddressLen]byte
	PermanentPhysicalAddress    [ifMaxPhysAddressLen]byte
	Mtu                         uint32
	Type                        uint32
	TunnelType                  uint32
	MediaType                   uint32
	PhysicalMediumType          uint32
	AccessType                  uint32
	DirectionType               uint32
	InterfaceAndOperStatusFlags byte
	OperStatus                  uint32
	AdminStatus                 uint32
	MediaConnectState           uint32
	NetworkGuid                 windows.GUID
	ConnectionType              uint32
	TransmitLinkSpeed           uint64
	ReceiveLinkSpeed            uint64
	InOctets                    uint64
	InUcastPkts                 uint64
	InNUcastPkts                uint64
	InDiscards                  uint64
	InErrors                    uint64
	InUnknownProtos             uint64
	InUcastOctets               uint64
	InMulticastOctets           uint64
	InBroadcastOctets           uint64
	OutOctets                   uint64
	OutUcastPkts                uint64
	OutNUcastPkts               uint64
	OutDiscards                 uint64
	OutErrors                   uint64
	OutUcastOctets              uint64
	OutMulticastOctets          uint64
	OutBroadcastOctets          uint64
	OutQLen                     uint64
}

var (
	wlanAPI               = windows.NewLazySystemDLL("wlanapi.dll")
	wlanOpenHandle        = wlanAPI.NewProc("WlanOpenHandle")
	wlanCloseHandle       = wlanAPI.NewProc("WlanCloseHandle")
	wlanEnumInterfaces    = wlanAPI.NewProc("WlanEnumInterfaces")
	wlanQueryInterface    = wlanAPI.NewProc("WlanQueryInterface")
	wlanScan              = wlanAPI.NewProc("WlanScan")
	wlanGetNetworkBssList = wlanAPI.NewProc("WlanGetNetworkBssList")
	wlanFreeMemory        = wlanAPI.NewProc("WlanFreeMemory")

	iphlpapi                   = windows.NewLazySystemDLL("iphlpapi.dll")
	getIfEntry2                = iphlpapi.NewProc("GetIfEntry2")
	convertInterfaceGuidToLuid = iphlpapi.NewProc("ConvertInterfaceGuidToLuid")
)

type windowsScanner struct {
	ouiLookup       *OUILookup
	clientHandle    uintptr
	mu              sync.Mutex
	interfaceCache  map[string]interfaceCacheEntry
	baselineStats   map[string]trafficStats
	connectionStart map[string]time.Time
	parser          *windowsParser
}

type windowsParser struct {
	ouiLookup *OUILookup
}

type interfaceCacheEntry struct {
	guid        windows.GUID
	description string
}

type trafficStats struct {
	inOctets   uint64
	outOctets  uint64
	inPackets  uint64
	outPackets uint64
	timestamp  time.Time
}

func NewWiFiScanner(cacheFile string) WiFiBackend {
	ouiLookup := NewOUILookup(cacheFile)
	ouiLookup.LoadOUIDatabase()

	scanner := &windowsScanner{
		ouiLookup:       ouiLookup,
		interfaceCache:  make(map[string]interfaceCacheEntry),
		baselineStats:   make(map[string]trafficStats),
		connectionStart: make(map[string]time.Time),
		parser:          &windowsParser{ouiLookup: ouiLookup},
	}

	if err := scanner.openHandle(); err != nil {
		slog.Error("failed to open WLAN handle on init", "err", err)
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
	infoSize := unsafe.Sizeof(WLAN_INTERFACE_INFO{})

	// interfaceCache is read concurrently by the scan-loop goroutine via
	// resolveInterfaceGUID while Wails bindings call GetInterfaces — guard it.
	s.mu.Lock()
	for i := uint32(0); i < interfaceList.NumberOfItems; i++ {
		infoPtr := unsafe.Add(unsafe.Pointer(&interfaceList.InterfaceInfo[0]), uintptr(i)*infoSize)
		info := (*WLAN_INTERFACE_INFO)(infoPtr)

		description := syscall.UTF16ToString(info.InterfaceDescription[:])
		if description == "" {
			description = "Wi-Fi"
		}

		s.interfaceCache[description] = interfaceCacheEntry{
			guid:        info.InterfaceGUID,
			description: description,
		}

		interfaces = append(interfaces, description)
	}
	s.mu.Unlock()

	return interfaces, nil
}

func (s *windowsScanner) ScanNetworks(iface string) ([]AccessPoint, error) {
	if err := s.ensureHandle(); err != nil {
		return nil, err
	}

	guid, err := s.resolveInterfaceGUID(iface)
	if err != nil {
		return nil, fmt.Errorf("invalid interface: %w", err)
	}

	ret, _, _ := wlanScan.Call(
		s.clientHandle,
		uintptr(unsafe.Pointer(&guid)),
		0,
		0,
		0,
	)

	if ret != 0 {
		slog.Warn("WlanScan failed", "ret", ret)
	}

	// WlanScan is asynchronous and typically takes ~4 s to sweep all channels;
	// the BSS list fetched below therefore reflects the *previous* sweep. At
	// the app's 4 s poll cadence that is one tick of staleness, which we accept
	// to avoid the complexity of WlanRegisterNotification scan-complete
	// callbacks. The short sleep just gives fast partial results a chance.
	time.Sleep(100 * time.Millisecond)

	var bssList *WLAN_BSS_LIST

	ret, _, _ = wlanGetNetworkBssList.Call(
		s.clientHandle,
		uintptr(unsafe.Pointer(&guid)),
		0,
		uintptr(dot11BssTypeAny),
		0,
		0,
		uintptr(unsafe.Pointer(&bssList)),
	)

	if ret != 0 {
		return nil, fmt.Errorf("WlanGetNetworkBssList failed with error: %d", ret)
	}

	if bssList == nil {
		return []AccessPoint{}, nil
	}

	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(bssList)))

	if s.parser == nil {
		return nil, fmt.Errorf("windows scanner: parser not initialized")
	}

	aps := make([]AccessPoint, 0, bssList.NumberOfItems)
	entrySize := unsafe.Sizeof(WLAN_BSS_ENTRY{})

	for i := uint32(0); i < bssList.NumberOfItems; i++ {
		entryPtr := unsafe.Add(unsafe.Pointer(&bssList.WlanBssEntries[0]), uintptr(i)*entrySize)
		entry := (*WLAN_BSS_ENTRY)(entryPtr)
		ap := s.parser.bssEntryToAccessPoint(entry)
		aps = append(aps, ap)
	}

	return aps, nil
}

func (p *windowsParser) bssEntryToAccessPoint(entry *WLAN_BSS_ENTRY) AccessPoint {
	ssid := formatSSID(entry.Dot11SSID)
	bssid := formatMACAddress(entry.Dot11BSSID)

	frequency := int(entry.ChCenterFrequency / 1000)
	channel := frequencyToChannel(frequency)

	band := frequencyToBand(frequency)

	security := "Open"
	if entry.CapabilityInformation&capabilityPrivacy != 0 {
		security = "WEP"
	}

	wifiStandard := phyTypeToStandard(entry.Dot11BSSPhyType)

	signalQuality := int(entry.LinkQuality)
	if signalQuality > 100 {
		signalQuality = 100
	}

	ap := AccessPoint{
		SSID:          ssid,
		BSSID:         bssid,
		Vendor:        p.ouiLookup.LookupVendor(bssid),
		Frequency:     frequency,
		Channel:       channel,
		ChannelWidth:  20,
		Signal:        int(entry.RSSI),
		SignalQuality: signalQuality,
		Security:      security,
		Band:          band,
		LastSeen:      time.Now(),
		Capabilities:  []string{wifiStandard},
		BeaconInt:     int(entry.BeaconPeriod),
		DFS:           isDFSChannel(channel),
	}

	// Feed the raw IE TLV stream through the shared parser (wifi_ie_parser.go)
	// — the same code path the Linux scanner and the macOS helper use — so
	// per-IE semantics can't drift between platforms. Defaults for fields the
	// IEs don't cover (MIMOStreams, QAMSupport, SNR) are applied later by
	// NormalizeAccessPoint.
	if entry.IESize > 0 && entry.IEOffset > 0 {
		iePtr := unsafe.Add(unsafe.Pointer(entry), uintptr(entry.IEOffset))
		ieData := unsafe.Slice((*byte)(iePtr), entry.IESize)
		parseInformationElements(ieData, &ap)
	}

	return ap
}

func (s *windowsScanner) GetConnectionInfo(iface string) (ConnectionInfo, error) {
	if err := s.ensureHandle(); err != nil {
		return ConnectionInfo{}, err
	}

	guid, err := s.resolveInterfaceGUID(iface)
	if err != nil {
		return ConnectionInfo{}, err
	}

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
		return ConnectionInfo{Connected: false}, nil
	}

	if connAttr == nil {
		return ConnectionInfo{Connected: false}, nil
	}

	defer wlanFreeMemory.Call(uintptr(unsafe.Pointer(connAttr)))

	if connAttr.State != wlanInterfaceStateConnected {
		return ConnectionInfo{Connected: false}, nil
	}

	assoc := &connAttr.AssociationAttributes

	channel, _ := s.queryChannel(guid)
	rssi, _ := s.queryRSSI(guid)
	if rssi == 0 {
		rssi = int32(-100 + int32(assoc.WlanSignalQuality)/2)
	}

	frequency := channelToFrequency(channel)

	// Derive channel width and MIMO from rates since Windows WLAN API
	// doesn't expose these directly in association attributes.
	channelWidth := 20
	mimoConfig := "1x1"
	if assoc.RxRate > 0 || assoc.TxRate > 0 {
		maxRate := assoc.RxRate
		if assoc.TxRate > maxRate {
			maxRate = assoc.TxRate
		}
		maxRateKbps := float64(maxRate) * 500

		// Derive channel width from rate using known rate tables.
		switch assoc.Dot11PhyType {
		case dot11PhyTypeHe, dot11PhyTypeEht:
			// WiFi 6/7: 160MHz if rate > 600 Mbps, else 80MHz if > 287 Mbps
			if maxRateKbps >= 600*1000 {
				channelWidth = 160
			} else if maxRateKbps >= 287*1000 {
				channelWidth = 80
			}
		case dot11PhyTypeVht:
			// WiFi 5: 160MHz if rate > 866 Mbps, 80MHz if > 433 Mbps
			if maxRateKbps >= 866*1000 {
				channelWidth = 160
			} else if maxRateKbps >= 433*1000 {
				channelWidth = 80
			}
		case dot11PhyTypeHt:
			// WiFi 4: 40MHz if rate > 150 Mbps
			if maxRateKbps >= 150*1000 {
				channelWidth = 40
			}
		}

		// Derive MIMO config (NSS) from rates using rate tables.
		wifiStd := phyTypeToStandard(assoc.Dot11PhyType)
		_, nss := rateToMCSNSS(maxRateKbps, wifiStd)
		if nss > 0 {
			mimoConfig = fmt.Sprintf("%dx%d", nss, nss)
		}
	}

	info := ConnectionInfo{
		Connected:    true,
		SSID:         formatSSID(assoc.Dot11SSID),
		BSSID:        formatMACAddress(assoc.Dot11BSSID),
		Channel:      channel,
		Frequency:    frequency,
		Signal:       int(rssi),
		SignalAvg:    int(rssi),
		RxBitrate:    float64(assoc.RxRate) / 1000.0,
		TxBitrate:    float64(assoc.TxRate) / 1000.0,
		WiFiStandard: phyTypeToStandard(assoc.Dot11PhyType),
		ChannelWidth: channelWidth,
		MIMOConfig:   mimoConfig,
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

func (s *windowsScanner) getInterfaceStats(guid windows.GUID) (*MIB_IF_ROW2, error) {
	var row MIB_IF_ROW2

	ret, _, _ := convertInterfaceGuidToLuid.Call(
		uintptr(unsafe.Pointer(&guid)),
		uintptr(unsafe.Pointer(&row.InterfaceLuid)),
	)

	if ret != 0 {
		return nil, fmt.Errorf("ConvertInterfaceGuidToLuid failed: %d", ret)
	}

	ret, _, _ = getIfEntry2.Call(uintptr(unsafe.Pointer(&row)))
	if ret != 0 {
		return nil, fmt.Errorf("GetIfEntry2 failed: %d", ret)
	}

	return &row, nil
}

// applyTrafficStats fills the traffic-derived keys on result from the
// interface counters, establishing per-interface baselines on first sight.
// Deltas are saturating: adapter counters can reset (driver restart,
// sleep/resume) and must not wrap to huge values. Writes zero values when
// stats is nil so consumers always see the keys.
func (s *windowsScanner) applyTrafficStats(iface string, result map[string]string, stats *MIB_IF_ROW2) {
	if stats == nil {
		result["rx_bytes"] = "0"
		result["tx_bytes"] = "0"
		result["rx_packets"] = "0"
		result["tx_packets"] = "0"
		result["tx_retries"] = "0"
		result["tx_failed"] = "0"
		result["connected_time"] = "0"
		result["retry_rate"] = "0.00"
		return
	}

	s.mu.Lock()
	baseline, hasBaseline := s.baselineStats[iface]
	connStart, hasConnStart := s.connectionStart[iface]

	if !hasBaseline {
		baseline = trafficStats{
			inOctets:   stats.InOctets,
			outOctets:  stats.OutOctets,
			inPackets:  stats.InUcastPkts + stats.InNUcastPkts,
			outPackets: stats.OutUcastPkts + stats.OutNUcastPkts,
			timestamp:  time.Now(),
		}
		s.baselineStats[iface] = baseline
	}

	if !hasConnStart {
		connStart = time.Now()
		s.connectionStart[iface] = connStart
	}
	s.mu.Unlock()

	rxBytes := saturatingSubUint64(stats.InOctets, baseline.inOctets)
	txBytes := saturatingSubUint64(stats.OutOctets, baseline.outOctets)
	rxPackets := saturatingSubUint64(stats.InUcastPkts+stats.InNUcastPkts, baseline.inPackets)
	txPackets := saturatingSubUint64(stats.OutUcastPkts+stats.OutNUcastPkts, baseline.outPackets)
	connectedTime := int(time.Since(connStart).Seconds())

	result["rx_bytes"] = fmt.Sprintf("%d", rxBytes)
	result["tx_bytes"] = fmt.Sprintf("%d", txBytes)
	result["rx_packets"] = fmt.Sprintf("%d", rxPackets)
	result["tx_packets"] = fmt.Sprintf("%d", txPackets)
	result["tx_retries"] = fmt.Sprintf("%d", stats.OutDiscards)
	result["tx_failed"] = fmt.Sprintf("%d", stats.OutErrors)
	result["connected_time"] = fmt.Sprintf("%d", connectedTime)
	result["retry_rate"] = fmt.Sprintf("%.2f", calculateRetryRate(stats.OutDiscards, txPackets))
}

// clearBaselines drops the traffic baseline and connection-start markers for
// a disconnected interface so the next association starts fresh.
func (s *windowsScanner) clearBaselines(iface string) {
	s.mu.Lock()
	delete(s.baselineStats, iface)
	delete(s.connectionStart, iface)
	s.mu.Unlock()
}

func (s *windowsScanner) GetLinkInfo(iface string) (map[string]string, error) {
	info, err := s.GetConnectionInfo(iface)
	if err != nil {
		return map[string]string{"connected": "false"}, err
	}

	if !info.Connected {
		s.clearBaselines(iface)
		return map[string]string{"connected": "false"}, nil
	}

	guid, _ := s.resolveInterfaceGUID(iface)
	stats, err := s.getInterfaceStats(guid)
	if err != nil {
		stats = nil
	}

	result := map[string]string{
		"connected":       "true",
		"ssid":            info.SSID,
		"bssid":           info.BSSID,
		"channel":         fmt.Sprintf("%d", info.Channel),
		"signal":          fmt.Sprintf("%d", info.Signal),
		"signal_avg":      fmt.Sprintf("%d", info.SignalAvg),
		"rx_bitrate":      fmt.Sprintf("%.1f", info.RxBitrate),
		"tx_bitrate":      fmt.Sprintf("%.1f", info.TxBitrate),
		"wifi_standard":   info.WiFiStandard,
		"tx_bitrate_info": formatRateInfoFromAssoc(info),
		"rx_bitrate_info": formatRateInfoFromAssoc(info),
	}

	s.applyTrafficStats(iface, result, stats)
	if stats != nil {
		result["frequency"] = fmt.Sprintf("%d", info.Frequency)
		// The WLAN API identifies interfaces by adapter *description*, which
		// net.InterfaceByName can't resolve (it matches the friendly name,
		// e.g. "Wi-Fi"). Resolve the local IP via the interface index instead
		// so the service layer doesn't have to guess.
		if ip := ipv4ForInterfaceIndex(int(stats.InterfaceIndex)); ip != "" {
			result["local_ip"] = ip
		}
	} else {
		result["frequency"] = "0"
	}

	return result, nil
}

func (s *windowsScanner) GetStationStats(iface string) (map[string]string, error) {
	info, err := s.GetConnectionInfo(iface)
	if err != nil {
		return map[string]string{"connected": "false"}, err
	}

	if !info.Connected {
		s.clearBaselines(iface)
		return map[string]string{"connected": "false"}, nil
	}

	guid, _ := s.resolveInterfaceGUID(iface)
	stats, err := s.getInterfaceStats(guid)
	if err != nil {
		stats = nil
	}

	result := map[string]string{
		"connected":       "true",
		"bssid":           info.BSSID,
		"signal":          fmt.Sprintf("%d", info.Signal),
		"signal_avg":      fmt.Sprintf("%d", info.SignalAvg),
		"rx_bitrate":      fmt.Sprintf("%.1f", info.RxBitrate),
		"tx_bitrate":      fmt.Sprintf("%.1f", info.TxBitrate),
		"tx_bitrate_info": formatRateInfoFromAssoc(info),
		"rx_bitrate_info": formatRateInfoFromAssoc(info),
	}

	s.applyTrafficStats(iface, result, stats)
	if stats != nil {
		result["last_ack_signal"] = fmt.Sprintf("%d", info.Signal)
	} else {
		result["last_ack_signal"] = "0"
	}

	return result, nil
}

// ipv4ForInterfaceIndex returns the first non-loopback IPv4 address bound to
// the interface with the given OS index, or "" when unavailable.
func ipv4ForInterfaceIndex(index int) string {
	if index <= 0 {
		return ""
	}
	iface, err := net.InterfaceByIndex(index)
	if err != nil {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP == nil {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil || ip.IsLoopback() {
			continue
		}
		return ip.String()
	}
	return ""
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

func (s *windowsScanner) resolveInterfaceGUID(iface string) (windows.GUID, error) {
	s.mu.Lock()
	cached, ok := s.interfaceCache[iface]
	s.mu.Unlock()
	if ok {
		return cached.guid, nil
	}

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

	if name == "" || name == "Wi-Fi" || name == "WiFi" {
		if interfaceList.NumberOfItems > 0 {
			return interfaceList.InterfaceInfo[0].InterfaceGUID, nil
		}
	}

	infoSize := unsafe.Sizeof(WLAN_INTERFACE_INFO{})
	for i := uint32(0); i < interfaceList.NumberOfItems; i++ {
		infoPtr := unsafe.Add(unsafe.Pointer(&interfaceList.InterfaceInfo[0]), uintptr(i)*infoSize)
		info := (*WLAN_INTERFACE_INFO)(infoPtr)

		desc := syscall.UTF16ToString(info.InterfaceDescription[:])
		if desc == name {
			return info.InterfaceGUID, nil
		}
	}

	if interfaceList.NumberOfItems > 0 {
		return interfaceList.InterfaceInfo[0].InterfaceGUID, nil
	}

	return windows.GUID{}, fmt.Errorf("interface not found: %s", name)
}

func stringToGUID(s string) (windows.GUID, error) {
	var guid windows.GUID

	n, err := fmt.Sscanf(s, "{%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X}",
		&guid.Data1, &guid.Data2, &guid.Data3,
		&guid.Data4[0], &guid.Data4[1],
		&guid.Data4[2], &guid.Data4[3], &guid.Data4[4], &guid.Data4[5], &guid.Data4[6], &guid.Data4[7])

	if err == nil && n == 11 {
		return guid, nil
	}

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

func calculateRetryRate(retries, totalPackets uint64) float64 {
	if totalPackets == 0 {
		return 0.0
	}
	rate := float64(retries) / float64(totalPackets) * 100.0
	if rate > 100.0 {
		rate = 100.0
	}
	return rate
}

// formatRateInfoFromAssoc synthesizes a rate info string compatible with
// parseBitrateInfo (wifi_utils.go). The Windows WLAN API exposes the phy type
// and negotiated rate but not MCS index or spatial stream count directly, so
// we derive them from the rate value.
func formatRateInfoFromAssoc(info ConnectionInfo) string {
	slog.Debug("wifi_scanner_windows: formatRateInfoFromAssoc",
		"wifiStandard", info.WiFiStandard,
		"rxBitrate", info.RxBitrate,
		"txBitrate", info.TxBitrate,
		"channelWidth", info.ChannelWidth)

	var prefix string
	switch {
	case info.WiFiStandard == "802.11ax":
		prefix = "HE"
	case info.WiFiStandard == "802.11ac":
		prefix = "VHT"
	case info.WiFiStandard == "802.11n":
		prefix = "HT"
	case info.WiFiStandard == "802.11a", info.WiFiStandard == "802.11g", info.WiFiStandard == "802.11b":
		prefix = ""
	default:
		prefix = ""
		slog.Warn("wifi_scanner_windows: unknown WiFi standard", "standard", info.WiFiStandard)
	}

	var mcsIndex, nss int
	if info.RxBitrate > 0 {
		mcsIndex, nss = rateToMCSNSS(info.RxBitrate*1000, info.WiFiStandard)
	}

	var parts []string
	if info.ChannelWidth > 20 {
		parts = append(parts, fmt.Sprintf("%dMHz", info.ChannelWidth))
	}
	if prefix != "" && mcsIndex >= 0 {
		parts = append(parts, fmt.Sprintf("%s-MCS %d", prefix, mcsIndex))
		if nss > 0 && prefix != "HT" {
			parts = append(parts, fmt.Sprintf("%s-NSS %d", prefix, nss))
		}
	}

	result := strings.Join(parts, " ")
	slog.Debug("wifi_scanner_windows: formatRateInfoFromAssoc result", "result", result)
	return result
}

// rateToMCSNSS derives approximate MCS index and NSS from a rate in Kbps.
// The Windows WLAN API reports rates in 500 Kbps units. For a given WiFi
// standard and rate, we can estimate the MCS and NSS.
func rateToMCSNSS(rateKbps float64, wifiStandard string) (mcsIndex, nss int) {
	switch wifiStandard {
	case "802.11ax":
		baseRate := 287.0
		if rateKbps >= 1201*1000 {
			baseRate = 1201.0
		}
		if rateKbps >= 600*1000 && rateKbps < 1201*1000 {
			baseRate = 600.0
		}
		nss = 1
		for rateKbps >= baseRate*float64(nss)*1000 && nss < 8 {
			nss++
		}
		if nss > 1 {
			nss--
		}
		ratePerStream := baseRate * float64(nss) * 1000
		mcsIndex = int((rateKbps / ratePerStream))
		if mcsIndex > 11 {
			mcsIndex = 11
		}
		if mcsIndex < 0 {
			mcsIndex = 0
		}
	case "802.11ac":
		baseRate := 433.5
		if rateKbps >= 867*1000 {
			baseRate = 867.0
		}
		nss = 1
		for rateKbps >= baseRate*float64(nss)*1000 && nss < 8 {
			nss++
		}
		if nss > 1 {
			nss--
		}
		ratePerStream := baseRate * float64(nss) * 1000
		mcsIndex = int((rateKbps / ratePerStream))
		if mcsIndex > 9 {
			mcsIndex = 9
		}
		if mcsIndex < 0 {
			mcsIndex = 0
		}
	case "802.11n":
		baseRate := 72.0
		if rateKbps >= 150*1000 {
			baseRate = 150.0
		}
		nss = 1
		for rateKbps >= baseRate*float64(nss)*1000 && nss < 4 {
			nss++
		}
		if nss > 1 {
			nss--
		}
		ratePerStream := baseRate * float64(nss) * 1000
		mcsIndex = int((rateKbps / ratePerStream))
		if mcsIndex > 7 {
			mcsIndex = 7
		}
		if mcsIndex < 0 {
			mcsIndex = 0
		}
	}
	return mcsIndex, nss
}
