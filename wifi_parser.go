package main

// ScanParser converts platform-specific command output into normalized data.
type ScanParser interface {
	ParseScan(output []byte) ([]AccessPoint, error)
	ParseLink(output []byte) (map[string]string, error)
	ParseStation(output []byte) (map[string]string, error)
}
