//go:build !windows

package main

import (
	"context"
	"errors"
	"net"
	"time"
)

var errNativeICMPUnsupported = errors.New("native icmp not supported on this platform")

// nativeICMPAvailable reports whether the platform provides an ICMP echo
// facility outside the golang.org/x/net/icmp socket path. Only Windows does
// (iphlpapi IcmpSendEcho); everywhere else the sampler uses its shared
// unprivileged/raw ICMP socket.
func nativeICMPAvailable() bool { return false }

func nativeICMPProbe(_ context.Context, _ net.IP, _ time.Duration) (float64, error) {
	return 0, errNativeICMPUnsupported
}
