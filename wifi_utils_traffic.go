//go:build darwin || windows

package main

// saturatingSubUint64 returns a-b, clamped at 0. Interface byte/packet
// counters can reset (driver restart, sleep/resume); a plain unsigned
// subtraction would wrap to ~1.8e19 and wreck the traffic display.
//
// Only the darwin and windows backends compute counter deltas themselves —
// on Linux nl80211 reports per-connection counters directly — hence the
// build constraint (staticcheck flags the helper as unused on linux
// otherwise).
func saturatingSubUint64(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}
