//go:build darwin && !cgo

package main

import (
	"errors"
	"fmt"
)

// ErrLocationDenied is referenced from non-cgo darwin builds for type parity;
// it's never returned because the stub backend is not used as a primary
// source.
var ErrLocationDenied = errors.New("macOS Location Services authorization required to scan WiFi networks")

func coreWLANAvailable() bool {
	return false
}

func coreWLANEnsureLocationAuthorization() error { return nil }
func coreWLANPrimeLocationAuthorization()        {}

func setCoreWLANLookup(_ *OUILookup) {}

func coreWLANScanNetworks(_ string) ([]AccessPoint, error) {
	return nil, fmt.Errorf("corewlan unavailable (cgo disabled)")
}

func coreWLANConnectionInfo(_ string) (ConnectionInfo, error) {
	return ConnectionInfo{}, fmt.Errorf("corewlan unavailable (cgo disabled)")
}

func coreWLANLinkInfo(_ string) (map[string]string, error) {
	return map[string]string{"connected": "false"}, fmt.Errorf("corewlan unavailable (cgo disabled)")
}

func coreWLANStationInfo(_ string) (map[string]string, error) {
	return map[string]string{"connected": "false"}, fmt.Errorf("corewlan unavailable (cgo disabled)")
}

func coreWLANInterfaces() ([]string, error) {
	return nil, fmt.Errorf("corewlan unavailable (cgo disabled)")
}
