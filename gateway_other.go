//go:build !linux && !windows && !darwin

package main

import (
	"fmt"
	"log/slog"
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
	slog.Warn("gateway_other: defaultGateway called on unsupported platform")
	return nil, fmt.Errorf("default gateway resolution not implemented on this platform")
}
