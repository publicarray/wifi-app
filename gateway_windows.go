//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getIpForwardTable = iphlpapi.NewProc("GetIpForwardTable")

// mibIPForwardRow mirrors MIB_IPFORWARDROW (iprtrmib.h): 14 DWORDs with fixed
// layout. IPv4 addresses are stored in network byte order.
type mibIPForwardRow struct {
	Dest      uint32
	Mask      uint32
	Policy    uint32
	NextHop   uint32
	IfIndex   uint32
	Type      uint32
	Proto     uint32
	Age       uint32
	NextHopAS uint32
	Metric1   uint32
	Metric2   uint32
	Metric3   uint32
	Metric4   uint32
	Metric5   uint32
}

// defaultGateway returns the IPv4 default gateway by walking the system
// routing table via GetIpForwardTable. Among multiple default routes the one
// with the lowest metric wins. This replaces parsing `route print` output,
// which depended on the CLI's exact section layout and spawned a process per
// call.
func defaultGateway() (net.IP, error) {
	// Size query first: a nil buffer returns ERROR_INSUFFICIENT_BUFFER with
	// the required byte count in size.
	var size uint32
	ret, _, _ := getIpForwardTable.Call(0, uintptr(unsafe.Pointer(&size)), 0)
	if ret != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) && ret != 0 {
		return nil, fmt.Errorf("GetIpForwardTable size query failed: %d", ret)
	}
	if size < 4 {
		return nil, fmt.Errorf("empty routing table")
	}

	buf := make([]byte, size)
	ret, _, _ = getIpForwardTable.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	if ret != 0 {
		return nil, fmt.Errorf("GetIpForwardTable failed: %d", ret)
	}

	// MIB_IPFORWARDTABLE: DWORD dwNumEntries followed by the row array.
	numEntries := uintptr(binary.LittleEndian.Uint32(buf[0:4]))
	rowSize := unsafe.Sizeof(mibIPForwardRow{})

	var bestNextHop uint32
	bestMetric := ^uint32(0)
	for i := uintptr(0); i < numEntries; i++ {
		off := 4 + i*rowSize
		if off+rowSize > uintptr(len(buf)) {
			break
		}
		row := (*mibIPForwardRow)(unsafe.Pointer(&buf[off]))
		// Default route: destination 0.0.0.0/0 with a real next hop.
		if row.Dest != 0 || row.Mask != 0 || row.NextHop == 0 {
			continue
		}
		if row.Metric1 < bestMetric {
			bestMetric = row.Metric1
			bestNextHop = row.NextHop
		}
	}

	if bestNextHop == 0 {
		slog.Warn("gateway_windows: no default gateway found")
		return nil, fmt.Errorf("no default route")
	}

	// The DWORD holds the address in network byte order; writing it back
	// little-endian restores the original in-memory byte sequence.
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], bestNextHop)
	ip := net.IPv4(b[0], b[1], b[2], b[3]).To4()

	slog.Debug("gateway_windows: found gateway", "ip", ip.String())
	return ip, nil
}
