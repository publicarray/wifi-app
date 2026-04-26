//go:build windows

package main

import (
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

func defaultGateway() (net.IP, error) {
	bufLen := uint32(64)
	allocSize := unsafe.Sizeof(windows.MibIpForwardTable2{}) + uintptr(bufLen)*unsafe.Sizeof(windows.MibIpForwardRow2{})
	alloc := make([]byte, allocSize)

	rowBase := uintptr(unsafe.Pointer(&alloc[0])) + unsafe.Sizeof(windows.MibIpForwardTable2{})
	(*windows.MibIpForwardTable2)(unsafe.Pointer(&alloc[0])).NumEntries = 0
	*(*unsafe.Pointer)(unsafe.Pointer(uintptr(unsafe.Pointer(&alloc[0])) + 8)) = unsafe.Pointer(rowBase)

	err := windows.GetIpForwardTable2(windows.AF_INET, (**windows.MibIpForwardTable2)(unsafe.Pointer(&alloc[0])))
	if err != nil {
		return nil, fmt.Errorf("GetIpForwardTable2 failed: %d", err)
	}

	table := (*windows.MibIpForwardTable2)(unsafe.Pointer(&alloc[0]))
	for i := uint32(0); i < table.NumEntries; i++ {
		row := (*windows.MibIpForwardRow2)(unsafe.Pointer(rowBase + uintptr(i)*unsafe.Sizeof(windows.MibIpForwardRow2{})))
		if row.DestinationPrefix.PrefixLength == 0 {
			ip := extractIPFromRawSockaddrInet(&row.NextHop)
			if ip != nil && !ip.IsUnspecified() {
				return ip, nil
			}
		}
	}
	return nil, fmt.Errorf("no default route found")
}

func extractIPFromRawSockaddrInet(sa *windows.RawSockaddrInet) net.IP {
	if sa == nil || sa.Family != windows.AF_INET {
		return nil
	}
	b := (*[4]byte)(unsafe.Pointer(&sa.Data[0]))[:]
	return net.IP(b[:])
}
