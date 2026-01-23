package main

// WiFi signal quality constants
const (
	// ExcellentSignal represents the signal threshold (in dBm) above which signal is considered excellent
	// Signals at or better than -30 dBm are treated as maximum quality (100%)
	ExcellentSignal = -30

	// PoorSignal represents the signal threshold (in dBm) below which signal is considered poor
	// Signals at or worse than -100 dBm are treated as minimum quality (0%)
	PoorSignal = -100

	// SignalRangeSize is the total signal range in dBm used for quality calculation
	// This represents the range from PoorSignal (-100 dBm) to ExcellentSignal (-30 dBm)
	SignalRangeSize = 70
)

// signalToQuality converts a WiFi signal strength (in dBm) to a quality percentage (0-100)
//
// The conversion uses a linear mapping:
//
//	-30 dBm or better -> 100% (excellent)
//	-100 dBm or worse -> 0% (poor)
//	Values in between are linearly interpolated
//
// Examples:
//
//	-30 dBm -> 100%
//	-50 dBm -> ~71%
//	-70 dBm -> ~43%
//	-100 dBm -> 0%
//
// This function is shared across all platform-specific WiFi scanners to ensure
// consistent signal quality calculation.
func signalToQuality(signal int) int {
	if signal >= ExcellentSignal {
		return 100
	}
	if signal <= PoorSignal {
		return 0
	}
	// Linear interpolation: map [-100, -30] to [0, 100]
	return int((float64(signal-PoorSignal) / float64(SignalRangeSize)) * 100)
}

// appendUnique adds an item to a slice if it doesn't already exist
// This is useful for avoiding duplicate entries in capabilities arrays
// Returns the modified slice (or the original if item already exists)
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func channelToFrequency(channel int) int {
	if channel >= 1 && channel <= 14 {
		if channel == 14 {
			return 2484
		}
		return 2407 + (channel * 5)
	}
	if channel >= 36 && channel <= 165 {
		return 5000 + (channel * 5)
	}
	if channel >= 1 && channel <= 233 {
		if channel == 2 || channel == 1 {
			return 5935
		}
		if channel == 5 || channel == 9 {
			return 5950 + ((channel - 5) * 20)
		}
		if channel >= 11 && channel <= 253 {
			return 5950 + (channel * 20)
		}
	}
	return 0
}
