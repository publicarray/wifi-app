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
	wlanIntfOpcodeStatistics        = 0x10000101
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
	}

	if err := scanner.openHandle(); err != nil {
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
	infoSize := unsafe.Sizeof(WLAN_INTERFACE_INFO{})

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
		fmt.Printf("Warning: WlanScan failed with error: %d\n", ret)
	}

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

	aps := make([]AccessPoint, 0, bssList.NumberOfItems)
	entrySize := unsafe.Sizeof(WLAN_BSS_ENTRY{})

	for i := uint32(0); i < bssList.NumberOfItems; i++ {
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

	frequency := int(entry.ChCenterFrequency / 1000)
	channel := frequencyToChannel(frequency)

	band := "2.4GHz"
	if frequency > 5900 {
		band = "6GHz"
	} else if frequency > 5000 {
		band = "5GHz"
	}

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
		Vendor:        s.ouiLookup.LookupVendor(bssid),
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

	if entry.IESize > 0 && entry.IEOffset > 0 {
		s.parseInformationElements(&ap, entry)
	}

	return ap
}

func (s *windowsScanner) parseInformationElements(ap *AccessPoint, entry *WLAN_BSS_ENTRY) {
	iePtr := unsafe.Add(unsafe.Pointer(entry), uintptr(entry.IEOffset))
	ieData := unsafe.Slice((*byte)(iePtr), entry.IESize)

	// Track parsed values for speed calculation
	var htMCSSet []byte
	var vhtMCSMap uint16
	var heMCSMap uint16
	var hasVHT, hasHE, hasEHT bool

	offset := uint32(0)
	for offset+2 <= entry.IESize {
		elementID := ieData[offset]
		length := ieData[offset+1]

		if offset+2+uint32(length) > entry.IESize {
			break
		}

		data := ieData[offset+2 : offset+2+uint32(length)]

		switch elementID {
		case 5: // TIM (Traffic Indication Map)
			if length >= 2 {
				ap.DTIM = int(data[1])
			}

		case 7: // Country Information
			if length >= 2 {
				// First 2 bytes are the country code (ASCII)
				ap.CountryCode = string(data[0:2])
			}

		case 11: // BSS Load
			if length >= 5 {
				// Byte 0-1: Station Count (little-endian)
				ap.BSSLoadStations = int(uint16(data[0]) | uint16(data[1])<<8)
				// Byte 2: Channel Utilization (0-255, convert to percentage)
				ap.BSSLoadUtilization = int(data[2])
			}

		case 45: // HT Capabilities
			if length >= 2 {
				htCaps := uint16(data[0]) | uint16(data[1])<<8
				if htCaps&0x0002 != 0 {
					ap.ChannelWidth = 40
				}
			}
			// Extract MIMO streams from HT MCS Set (bytes 3-6)
			if length >= 6 {
				htMCSSet = data[3:7]
				streams := countHTStreams(htMCSSet)
				if streams > ap.MIMOStreams {
					ap.MIMOStreams = streams
				}
			}

		case 48: // RSN
			s.parseRSNElement(ap, data)

		case 54: // Mobility Domain (802.11r)
			if length >= 2 {
				ap.FastRoaming = true
			}

		case 70: // RM Enabled Capabilities (802.11k)
			if length >= 1 {
				ap.NeighborReport = (data[0] & 0x02) != 0
				ap.BSSTransition = (data[0] & 0x08) != 0
			}

		case 127: // Extended Capabilities
			if length >= 3 {
				ap.BSSTransition = (data[2] & 0x08) != 0
			}
			if length >= 6 {
				ap.TWTSupport = (data[5] & 0x02) != 0
			}

		case 191: // VHT Capabilities
			hasVHT = true
			if length >= 4 {
				vhtCaps := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
				switch (vhtCaps >> 2) & 0x03 {
				case 1, 2:
					ap.ChannelWidth = 160
				default:
					if ap.ChannelWidth < 80 {
						ap.ChannelWidth = 80
					}
				}
				if vhtCaps&(1<<19) != 0 {
					ap.MUMIMO = true
				}
			}
			// VHT MCS Map at bytes 4-5 (RX) and 6-7 (TX) - we use RX
			if length >= 8 {
				vhtMCSMap = uint16(data[4]) | uint16(data[5])<<8
				streams := countVHTStreams(vhtMCSMap)
				if streams > ap.MIMOStreams {
					ap.MIMOStreams = streams
				}
				// Determine QAM from MCS map
				qam := getVHTQAM(vhtMCSMap)
				if qam > ap.QAMSupport {
					ap.QAMSupport = qam
				}
			}

		case 221: // Vendor Specific
			if length >= 4 {
				// Microsoft WPA OUI
				if data[0] == 0x00 && data[1] == 0x50 && data[2] == 0xF2 {
					switch data[3] {
					case 0x01: // WPA
						if ap.Security == "Open" || ap.Security == "WEP" {
							ap.Security = "WPA"
						}
					case 0x02: // WMM/WME
						ap.QoSSupport = true
						if length >= 8 {
							ap.UAPSD = (data[7] & 0x80) != 0
						}
					case 0x04: // WPS
						ap.WPS = true
					}
				}
			}

		case 255: // Extension Element
			if length >= 1 {
				extID := data[0]
				extData := data[1:]

				switch extID {
				case 35: // HE Capabilities
					hasHE = true
					ap.Capabilities = appendUnique(ap.Capabilities, "WiFi6")
					if len(extData) >= 6 {
						ap.TWTSupport = (extData[0] & 0x04) != 0
						ap.UAPSD = (extData[0] & 0x08) != 0
						ap.BSSColor = int(extData[4] & 0x3F)
						ap.OBSSPD = (extData[4] & 0x40) != 0
					}
					// HE MCS Map starts at byte 6 (after MAC caps and PHY caps)
					// Structure varies but typically at fixed offset
					if len(extData) >= 11 {
						heMCSMap = uint16(extData[9]) | uint16(extData[10])<<8
						streams := countHEStreams(heMCSMap)
						if streams > ap.MIMOStreams {
							ap.MIMOStreams = streams
						}
						// HE supports 1024-QAM
						if ap.QAMSupport < 1024 {
							ap.QAMSupport = 1024
						}
					}

				case 36: // HE Operation
					if len(extData) >= 5 {
						heOp := extData[0]
						if heOp&0x04 != 0 {
							ap.BSSColor = int(extData[4] & 0x3F)
						}
					}

				case 106: // EHT Capabilities (WiFi 7)
					hasEHT = true
					ap.Capabilities = appendUnique(ap.Capabilities, "WiFi7")
					if len(extData) >= 2 && ap.ChannelWidth < 320 {
						if extData[1]&0x02 != 0 {
							ap.ChannelWidth = 320
						}
					}
					// EHT supports 4096-QAM
					if ap.QAMSupport < 4096 {
						ap.QAMSupport = 4096
					}
				}
			}
		}

		offset += 2 + uint32(length)
	}

	// Set default MIMO if not detected
	if ap.MIMOStreams == 0 {
		ap.MIMOStreams = 1
	}

	// Set default QAM based on WiFi generation if not detected
	if ap.QAMSupport == 0 {
		if hasEHT {
			ap.QAMSupport = 4096
		} else if hasHE {
			ap.QAMSupport = 1024
		} else if hasVHT {
			ap.QAMSupport = 256
		} else {
			ap.QAMSupport = 64
		}
	}

	// Calculate theoretical and real-world speeds
	ap.MaxTheoreticalSpeed = calculateMaxSpeed(ap.ChannelWidth, ap.MIMOStreams, ap.QAMSupport, hasHE, hasEHT)
	ap.RealWorldSpeed = int(float64(ap.MaxTheoreticalSpeed) * 0.65) // ~65% of theoretical

	// Calculate SNR if noise is available (Windows doesn't provide noise, estimate from signal)
	if ap.Noise == 0 {
		ap.Noise = -95 // Typical noise floor
	}
	ap.SNR = ap.Signal - ap.Noise

	// Estimate range based on signal and typical path loss model
	ap.EstimatedRange = estimateRange(ap.Signal, ap.Frequency)
}

func (s *windowsScanner) parseRSNElement(ap *AccessPoint, data []byte) {
	if len(data) < 8 {
		ap.Security = "WPA2"
		return
	}

	groupCipher := ""
	if len(data) >= 6 {
		if data[2] == 0x00 && data[3] == 0x0F && data[4] == 0xAC {
			switch data[5] {
			case 2:
				groupCipher = "TKIP"
			case 4:
				groupCipher = "CCMP"
			case 8:
				groupCipher = "GCMP"
			case 9:
				groupCipher = "GCMP-256"
			}
		}
	}

	offset := 6
	if len(data) < offset+2 {
		ap.Security = "WPA2"
		return
	}

	pairwiseCount := uint16(data[offset]) | uint16(data[offset+1])<<8
	offset += 2

	pairwiseCiphers := []string{}
	for i := uint16(0); i < pairwiseCount && offset+4 <= len(data); i++ {
		if data[offset] == 0x00 && data[offset+1] == 0x0F && data[offset+2] == 0xAC {
			switch data[offset+3] {
			case 2:
				pairwiseCiphers = append(pairwiseCiphers, "TKIP")
			case 4:
				pairwiseCiphers = append(pairwiseCiphers, "CCMP")
			case 8:
				pairwiseCiphers = append(pairwiseCiphers, "GCMP")
			case 9:
				pairwiseCiphers = append(pairwiseCiphers, "GCMP-256")
			}
		}
		offset += 4
	}

	if len(data) < offset+2 {
		ap.Security = "WPA2"
		ap.SecurityCiphers = pairwiseCiphers
		return
	}

	akmCount := uint16(data[offset]) | uint16(data[offset+1])<<8
	offset += 2

	authMethods := []string{}
	securityType := "WPA2"

	for i := uint16(0); i < akmCount && offset+4 <= len(data); i++ {
		if data[offset] == 0x00 && data[offset+1] == 0x0F && data[offset+2] == 0xAC {
			switch data[offset+3] {
			case 1:
				authMethods = append(authMethods, "EAP")
			case 2:
				authMethods = append(authMethods, "PSK")
			case 5:
				authMethods = append(authMethods, "EAP-SHA256")
			case 6:
				authMethods = append(authMethods, "PSK-SHA256")
			case 8:
				authMethods = append(authMethods, "SAE")
				securityType = "WPA3"
			case 9:
				authMethods = append(authMethods, "FT-SAE")
				securityType = "WPA3"
				ap.FastRoaming = true
			case 12:
				authMethods = append(authMethods, "EAP-SUITE-B")
				securityType = "WPA3-Enterprise"
			case 13:
				authMethods = append(authMethods, "EAP-SUITE-B-192")
				securityType = "WPA3-Enterprise"
			case 18:
				authMethods = append(authMethods, "OWE")
				securityType = "OWE"
			}
		}
		offset += 4
	}

	if len(data) >= offset+2 {
		rsnCaps := uint16(data[offset]) | uint16(data[offset+1])<<8

		mfpCapable := (rsnCaps & 0x0080) != 0
		mfpRequired := (rsnCaps & 0x0040) != 0

		if mfpRequired {
			ap.PMF = "Required"
		} else if mfpCapable {
			ap.PMF = "Optional"
		} else {
			ap.PMF = "Disabled"
		}
	}

	ap.Security = securityType
	ap.SecurityCiphers = pairwiseCiphers
	ap.AuthMethods = authMethods
	if groupCipher != "" && len(ap.SecurityCiphers) == 0 {
		ap.SecurityCiphers = []string{groupCipher}
	}
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
		ChannelWidth: 20,
		MIMOConfig:   "1x1",
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

func (s *windowsScanner) GetLinkInfo(iface string) (map[string]string, error) {
	info, err := s.GetConnectionInfo(iface)
	if err != nil {
		return map[string]string{"connected": "false"}, err
	}

	if !info.Connected {
		s.mu.Lock()
		delete(s.baselineStats, iface)
		delete(s.connectionStart, iface)
		s.mu.Unlock()
		return map[string]string{"connected": "false"}, nil
	}

	guid, _ := s.resolveInterfaceGUID(iface)
	stats, err := s.getInterfaceStats(guid)

	result := map[string]string{
		"connected":  "true",
		"ssid":       info.SSID,
		"bssid":      info.BSSID,
		"channel":    fmt.Sprintf("%d", info.Channel),
		"signal":     fmt.Sprintf("%d", info.Signal),
		"signal_avg": fmt.Sprintf("%d", info.SignalAvg),
		"rx_bitrate": fmt.Sprintf("%.1f", info.RxBitrate),
		"tx_bitrate": fmt.Sprintf("%.1f", info.TxBitrate),
	}

	if err == nil && stats != nil {
		s.mu.Lock()
		baseline, hasBaseline := s.baselineStats[iface]
		connStart, hasConnStart := s.connectionStart[iface]

		if !hasBaseline {
			s.baselineStats[iface] = trafficStats{
				inOctets:   stats.InOctets,
				outOctets:  stats.OutOctets,
				inPackets:  stats.InUcastPkts + stats.InNUcastPkts,
				outPackets: stats.OutUcastPkts + stats.OutNUcastPkts,
				timestamp:  time.Now(),
			}
			baseline = s.baselineStats[iface]
		}

		if !hasConnStart {
			s.connectionStart[iface] = time.Now()
			connStart = s.connectionStart[iface]
		}
		s.mu.Unlock()

		rxBytes := stats.InOctets - baseline.inOctets
		txBytes := stats.OutOctets - baseline.outOctets
		rxPackets := (stats.InUcastPkts + stats.InNUcastPkts) - baseline.inPackets
		txPackets := (stats.OutUcastPkts + stats.OutNUcastPkts) - baseline.outPackets
		connectedTime := int(time.Since(connStart).Seconds())

		result["rx_bytes"] = fmt.Sprintf("%d", rxBytes)
		result["tx_bytes"] = fmt.Sprintf("%d", txBytes)
		result["rx_packets"] = fmt.Sprintf("%d", rxPackets)
		result["tx_packets"] = fmt.Sprintf("%d", txPackets)
		result["tx_retries"] = fmt.Sprintf("%d", stats.OutDiscards)
		result["tx_failed"] = fmt.Sprintf("%d", stats.OutErrors)
		result["connected_time"] = fmt.Sprintf("%d", connectedTime)
		result["retry_rate"] = fmt.Sprintf("%.2f", calculateRetryRate(stats.OutDiscards, txPackets))
		result["frequency"] = fmt.Sprintf("%d", info.Frequency)
	} else {
		result["rx_bytes"] = "0"
		result["tx_bytes"] = "0"
		result["rx_packets"] = "0"
		result["tx_packets"] = "0"
		result["tx_retries"] = "0"
		result["tx_failed"] = "0"
		result["connected_time"] = "0"
		result["retry_rate"] = "0.00"
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
		return map[string]string{"connected": "false"}, nil
	}

	guid, _ := s.resolveInterfaceGUID(iface)
	stats, err := s.getInterfaceStats(guid)

	result := map[string]string{
		"connected":  "true",
		"bssid":      info.BSSID,
		"signal":     fmt.Sprintf("%d", info.Signal),
		"signal_avg": fmt.Sprintf("%d", info.SignalAvg),
		"rx_bitrate": fmt.Sprintf("%.1f", info.RxBitrate),
		"tx_bitrate": fmt.Sprintf("%.1f", info.TxBitrate),
	}

	if err == nil && stats != nil {
		s.mu.Lock()
		baseline, hasBaseline := s.baselineStats[iface]
		connStart, hasConnStart := s.connectionStart[iface]

		if !hasBaseline {
			s.baselineStats[iface] = trafficStats{
				inOctets:   stats.InOctets,
				outOctets:  stats.OutOctets,
				inPackets:  stats.InUcastPkts + stats.InNUcastPkts,
				outPackets: stats.OutUcastPkts + stats.OutNUcastPkts,
				timestamp:  time.Now(),
			}
			baseline = s.baselineStats[iface]
		}

		if !hasConnStart {
			s.connectionStart[iface] = time.Now()
			connStart = s.connectionStart[iface]
		}
		s.mu.Unlock()

		rxBytes := stats.InOctets - baseline.inOctets
		txBytes := stats.OutOctets - baseline.outOctets
		rxPackets := (stats.InUcastPkts + stats.InNUcastPkts) - baseline.inPackets
		txPackets := (stats.OutUcastPkts + stats.OutNUcastPkts) - baseline.outPackets
		connectedTime := int(time.Since(connStart).Seconds())

		result["rx_bytes"] = fmt.Sprintf("%d", rxBytes)
		result["tx_bytes"] = fmt.Sprintf("%d", txBytes)
		result["rx_packets"] = fmt.Sprintf("%d", rxPackets)
		result["tx_packets"] = fmt.Sprintf("%d", txPackets)
		result["tx_retries"] = fmt.Sprintf("%d", stats.OutDiscards)
		result["tx_failed"] = fmt.Sprintf("%d", stats.OutErrors)
		result["connected_time"] = fmt.Sprintf("%d", connectedTime)
		result["last_ack_signal"] = fmt.Sprintf("%d", info.Signal)
		result["retry_rate"] = fmt.Sprintf("%.2f", calculateRetryRate(stats.OutDiscards, txPackets))
	} else {
		result["rx_bytes"] = "0"
		result["tx_bytes"] = "0"
		result["rx_packets"] = "0"
		result["tx_packets"] = "0"
		result["tx_retries"] = "0"
		result["tx_failed"] = "0"
		result["connected_time"] = "0"
		result["last_ack_signal"] = "0"
		result["retry_rate"] = "0.00"
	}

	return result, nil
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
	if cached, ok := s.interfaceCache[iface]; ok {
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

func guidToString(guid windows.GUID) string {
	return fmt.Sprintf("{%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X}",
		guid.Data1, guid.Data2, guid.Data3,
		guid.Data4[0], guid.Data4[1],
		guid.Data4[2], guid.Data4[3], guid.Data4[4], guid.Data4[5], guid.Data4[6], guid.Data4[7])
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

func countHTStreams(mcsSet []byte) int {
	if len(mcsSet) < 4 {
		return 1
	}
	streams := 0
	for i := 0; i < 4; i++ {
		if mcsSet[i] != 0 {
			streams = i + 1
		}
	}
	if streams == 0 {
		return 1
	}
	return streams
}

func countVHTStreams(mcsMap uint16) int {
	streams := 0
	for ss := 0; ss < 8; ss++ {
		mcs := (mcsMap >> (ss * 2)) & 0x03
		if mcs != 0x03 {
			streams = ss + 1
		}
	}
	if streams == 0 {
		return 1
	}
	return streams
}

func countHEStreams(mcsMap uint16) int {
	streams := 0
	for ss := 0; ss < 8; ss++ {
		mcs := (mcsMap >> (ss * 2)) & 0x03
		if mcs != 0x03 {
			streams = ss + 1
		}
	}
	if streams == 0 {
		return 1
	}
	return streams
}

func getVHTQAM(mcsMap uint16) int {
	for ss := 0; ss < 8; ss++ {
		mcs := (mcsMap >> (ss * 2)) & 0x03
		switch mcs {
		case 0:
			return 64
		case 1:
			return 256
		case 2:
			return 256
		}
	}
	return 64
}

func calculateMaxSpeed(channelWidth, streams, qam int, hasHE, hasEHT bool) int {
	if streams == 0 {
		streams = 1
	}
	if channelWidth == 0 {
		channelWidth = 20
	}

	var baseSpeed int
	switch channelWidth {
	case 20:
		if hasEHT {
			baseSpeed = 172
		} else if hasHE {
			baseSpeed = 143
		} else {
			baseSpeed = 86
		}
	case 40:
		if hasEHT {
			baseSpeed = 344
		} else if hasHE {
			baseSpeed = 287
		} else {
			baseSpeed = 200
		}
	case 80:
		if hasEHT {
			baseSpeed = 720
		} else if hasHE {
			baseSpeed = 600
		} else {
			baseSpeed = 433
		}
	case 160:
		if hasEHT {
			baseSpeed = 1441
		} else if hasHE {
			baseSpeed = 1201
		} else {
			baseSpeed = 867
		}
	case 320:
		baseSpeed = 2882
	default:
		baseSpeed = 86
	}

	speed := baseSpeed * streams

	if qam >= 4096 {
		speed = speed * 120 / 100
	} else if qam >= 1024 {
		speed = speed * 110 / 100
	}

	return speed
}

func estimateRange(signal, frequency int) float64 {
	if signal == 0 {
		return 0
	}

	pathLossExponent := 2.7
	if frequency > 5000 {
		pathLossExponent = 3.0
	}
	if frequency > 5900 {
		pathLossExponent = 3.5
	}

	freqMHz := float64(frequency)
	if freqMHz == 0 {
		freqMHz = 2437
	}

	txPower := 20.0
	fspl := txPower - float64(signal)

	freeSpaceConstant := 27.55
	log10Freq := 0.0
	switch {
	case freqMHz < 3000:
		log10Freq = 3.39
	case freqMHz < 6000:
		log10Freq = 3.72
	default:
		log10Freq = 3.78
	}

	distanceLog := (fspl - freeSpaceConstant - 20*log10Freq) / (10 * pathLossExponent)

	distance := 1.0
	for i := 0; i < int(distanceLog*10); i++ {
		distance *= 1.259
	}

	if distance < 1 {
		distance = 1
	}
	if distance > 500 {
		distance = 500
	}

	return distance
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
