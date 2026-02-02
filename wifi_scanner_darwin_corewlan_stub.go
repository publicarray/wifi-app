//go:build darwin && !cgo

package main

import "fmt"

func coreWLANAvailable() bool {
	return false
}

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
