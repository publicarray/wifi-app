//go:build darwin

package main

import (
	"fmt"
	"log/slog"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	rtmVersionDarwin = 0x2
	rtmGetDarwin     = 0x4
	rtaGatewayDarwin = 0x2
	rtaDstDarwin     = 0x1
	rtaNetmaskDarwin = 0x4
	rtfUpDarwin      = 0x1
	rtfGatewayDarwin = 0x2
)

func defaultGateway() (net.IP, error) {
	slog.Debug("gateway_darwin: starting gateway resolution")

	sock, err := unix.Socket(unix.AF_ROUTE, unix.SOCK_RAW, unix.AF_UNSPEC)
	if err != nil {
		slog.Error("gateway_darwin: socket af_route failed", "err", err)
		return nil, fmt.Errorf("socket af_route: %w", err)
	}
	defer unix.Close(sock)

	seq := int32(unix.Getpid())
	msgSize := int(unix.SizeofRtMsghdr) + int(unix.SizeofSockaddrInet4)*3
	buf := make([]byte, msgSize)

	hdr := (*unix.RtMsghdr)(unsafe.Pointer(&buf[0]))
	hdr.Msglen = uint16(msgSize)
	hdr.Version = uint8(rtmVersionDarwin)
	hdr.Type = uint8(rtmGetDarwin)
	hdr.Flags = int32(rtfUpDarwin | rtfGatewayDarwin)
	hdr.Addrs = int32(rtaDstDarwin | rtaGatewayDarwin | rtaNetmaskDarwin)
	hdr.Seq = seq
	hdr.Pid = seq

	dstPtr := unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + uintptr(unix.SizeofRtMsghdr))
	dst := (*unix.RawSockaddrInet4)(dstPtr)
	dst.Len = uint8(unix.SizeofSockaddrInet4)
	dst.Family = unix.AF_INET

	gwPtr := unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + uintptr(unix.SizeofRtMsghdr) + uintptr(unix.SizeofSockaddrInet4))
	gw := (*unix.RawSockaddrInet4)(gwPtr)
	gw.Len = uint8(unix.SizeofSockaddrInet4)
	gw.Family = unix.AF_INET

	maskPtr := unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + uintptr(unix.SizeofRtMsghdr) + uintptr(unix.SizeofSockaddrInet4)*2)
	mask := (*unix.RawSockaddrInet4)(maskPtr)
	mask.Len = uint8(unix.SizeofSockaddrInet4)
	mask.Family = unix.AF_INET

	err = unix.Sendto(sock, buf[:msgSize], 0, nil)
	if err != nil {
		slog.Error("gateway_darwin: send routing message failed", "err", err)
		return nil, fmt.Errorf("send routing message: %w", err)
	}

	for {
		n, _, err := unix.Recvfrom(sock, buf, 0)
		if err != nil {
			slog.Error("gateway_darwin: recv routing reply failed", "err", err)
			return nil, fmt.Errorf("recv routing reply: %w", err)
		}
		if n < unix.SizeofRtMsghdr {
			continue
		}
		if hdr.Type != uint8(rtmGetDarwin) {
			continue
		}
		if hdr.Seq != seq {
			continue
		}
		if hdr.Errno != 0 {
			slog.Error("gateway_darwin: routing error", "errno", hdr.Errno)
			return nil, fmt.Errorf("routing error: %d", hdr.Errno)
		}

		ip := extractGatewayIP(buf[:n], int32(hdr.Addrs))
		if ip != nil {
			slog.Info("gateway_darwin: found default gateway", "ip", ip.String())
			return ip, nil
		}
		slog.Warn("gateway_darwin: no gateway in routing reply")
		return nil, fmt.Errorf("no gateway in routing reply")
	}
}

func extractGatewayIP(data []byte, addrs int32) net.IP {
	offset := int(unix.SizeofRtMsghdr)

	for mask := int32(1); mask <= int32(0x100); mask <<= 1 {
		if addrs&mask == 0 {
			continue
		}
		saPtr := unsafe.Add(unsafe.Pointer(&data[0]), uintptr(offset))
		sa := (*unix.RawSockaddrInet4)(saPtr)
		if sa.Family == unix.AF_INET {
			if mask == rtaGatewayDarwin {
				return net.IP(sa.Addr[:])
			}
		}
		offset += int(unix.SizeofSockaddrInet4)
	}
	return nil
}
