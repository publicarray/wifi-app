//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	icmpCreateFile  = iphlpapi.NewProc("IcmpCreateFile")
	icmpCloseHandle = iphlpapi.NewProc("IcmpCloseHandle")
	icmpSendEcho    = iphlpapi.NewProc("IcmpSendEcho")
)

// nativeICMPAvailable reports whether iphlpapi's ICMP echo API is present.
// It works unprivileged on Windows, unlike raw ICMP sockets — without it the
// sampler would fall back to TCP :443, which most home gateways don't answer,
// making the "gateway" latency target read as 100 % loss.
func nativeICMPAvailable() bool {
	return icmpCreateFile.Find() == nil && icmpSendEcho.Find() == nil && icmpCloseHandle.Find() == nil
}

// nativeICMPProbe sends a single ICMP echo via IcmpSendEcho and returns the
// RTT in milliseconds. IcmpSendEcho blocks in-kernel for up to the given
// timeout; the context is only consulted before sending because the API has
// no cancellation handle. The reply's Status/RoundTripTime fields are read
// by byte offset (Status at 4, RoundTripTime at 8 per ICMP_ECHO_REPLY) so we
// don't have to mirror the pointer-bearing struct layout.
func nativeICMPProbe(ctx context.Context, ip net.IP, timeout time.Duration) (float64, error) {
	v4 := ip.To4()
	if v4 == nil {
		return 0, fmt.Errorf("native icmp requires an IPv4 target")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	handle, _, callErr := icmpCreateFile.Call()
	if handle == 0 || handle == uintptr(windows.InvalidHandle) {
		return 0, fmt.Errorf("IcmpCreateFile failed: %v", callErr)
	}
	defer icmpCloseHandle.Call(handle)

	payload := []byte("wifi-app-latency")
	// Docs require sizeof(ICMP_ECHO_REPLY) + payload + 8 spare bytes; 512 is
	// comfortably above that for our 16-byte payload.
	reply := make([]byte, 512)

	timeoutMs := uint32(timeout / time.Millisecond)
	if timeoutMs == 0 {
		timeoutMs = 1
	}

	start := time.Now()
	ret, _, callErr := icmpSendEcho.Call(
		handle,
		// IPAddr: IPv4 address in network byte order packed into a ULONG.
		uintptr(binary.LittleEndian.Uint32(v4)),
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		0, // no IP options
		uintptr(unsafe.Pointer(&reply[0])),
		uintptr(len(reply)),
		uintptr(timeoutMs),
	)
	elapsed := time.Since(start)

	if ret == 0 {
		return 0, fmt.Errorf("IcmpSendEcho: %v", callErr)
	}

	status := binary.LittleEndian.Uint32(reply[4:8])
	if status != 0 { // IP_SUCCESS
		return 0, fmt.Errorf("icmp echo status %d", status)
	}

	rtt := float64(binary.LittleEndian.Uint32(reply[8:12]))
	if rtt == 0 {
		// RoundTripTime has 1 ms resolution, so sub-millisecond LAN replies
		// report 0. Use the measured wall time instead of charting zero.
		rtt = float64(elapsed.Microseconds()) / 1000.0
	}
	return rtt, nil
}
