//go:build !linux

package main

import (
	"fmt"
	"net"
)

// defaultGateway is a stub for non-Linux platforms. The latency sampler treats
// the "gateway" magic target as unavailable when this returns an error and
// reports it as such to the frontend — no fallback is attempted.
//
// A native darwin (routing socket) / windows (GetBestRoute2) implementation
// can land later; for now the user can configure an explicit IP literal in
// `config.latency_targets` to get coverage on those platforms.
func defaultGateway() (net.IP, error) {
	return nil, fmt.Errorf("default gateway resolution not implemented on this platform")
}
