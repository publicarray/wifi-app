package main

import "strings"

// WiFi frequency ranges and channel constants
const (
	// 2.4GHz band channels
	Freq2412MHz        = 2412
	Freq2484MHz        = 2484
	Freq2407MHz        = 2407
	ChannelSpacing5MHz = 5

	// 5GHz band channels
	Freq5170MHz = 5170
	Freq5825MHz = 5825
	Freq5000MHz = 5000

	// 6GHz band channels
	Freq5935MHz = 5935
	Freq5955MHz = 5955
	Freq5965MHz = 5965
	Freq5985MHz = 5985
	Freq5950MHz = 5950
	Freq7115MHz = 7115

	// Channel constants
	Channel14 = 14
	Channel2  = 2
	Channel6  = 6

	// DFS (Dynamic Frequency Selection) channels - 5GHz band
	DFSChannel52  = 52
	DFSChannel56  = 56
	DFSChannel60  = 60
	DFSChannel64  = 64
	DFSChannel100 = 100
	DFSChannel104 = 104
	DFSChannel108 = 108
	DFSChannel112 = 112
	DFSChannel116 = 116
	DFSChannel120 = 120
	DFSChannel124 = 124
	DFSChannel128 = 128
	DFSChannel132 = 132
	DFSChannel136 = 136
	DFSChannel140 = 140
	DFSChannel144 = 144
)

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

func isDFSChannel(channel int) bool {
	switch channel {
	case DFSChannel52, DFSChannel56, DFSChannel60, DFSChannel64,
		DFSChannel100, DFSChannel104, DFSChannel108, DFSChannel112,
		DFSChannel116, DFSChannel120, DFSChannel124, DFSChannel128,
		DFSChannel132, DFSChannel136, DFSChannel140, DFSChannel144:
		return true
	default:
		return false
	}
}

// parseBitrateInfo extracts WiFi standard, channel width, and MIMO config from bitrate string
func parseBitrateInfo(bitrateInfo string) (wifiStandard, channelWidth, mimoConfig string) {
	wifiStandard = "802.11"
	channelWidth = "20"
	mimoConfig = "1x1"

	if strings.Contains(bitrateInfo, "HE") {
		wifiStandard = "WiFi 6 (802.11ax)"
	} else if strings.Contains(bitrateInfo, "VHT") {
		wifiStandard = "WiFi 5 (802.11ac)"
	} else if strings.Contains(bitrateInfo, "HT") {
		wifiStandard = "WiFi 4 (802.11n)"
	} else {
		wifiStandard = "Legacy (802.11a/b/g)"
	}

	if strings.Contains(bitrateInfo, "320MHz") {
		channelWidth = "320"
	} else if strings.Contains(bitrateInfo, "160MHz") || strings.Contains(bitrateInfo, "80+80") {
		channelWidth = "160"
	} else if strings.Contains(bitrateInfo, "80MHz") {
		channelWidth = "80"
	} else if strings.Contains(bitrateInfo, "40MHz") {
		channelWidth = "40"
	}

	if strings.Contains(bitrateInfo, "HE-NSS 4") || strings.Contains(bitrateInfo, "VHT-NSS 4") {
		mimoConfig = "4x4"
	} else if strings.Contains(bitrateInfo, "HE-NSS 3") || strings.Contains(bitrateInfo, "VHT-NSS 3") {
		mimoConfig = "3x3"
	} else if strings.Contains(bitrateInfo, "HE-NSS 2") || strings.Contains(bitrateInfo, "VHT-NSS 2") {
		mimoConfig = "2x2"
	} else if strings.Contains(bitrateInfo, "HE-NSS 1") || strings.Contains(bitrateInfo, "VHT-NSS 1") {
		mimoConfig = "1x1"
	}

	return
}

func frequencyToChannel(freq int) int {
	if freq >= Freq2412MHz && freq <= Freq2484MHz {
		if freq == Freq2484MHz {
			return Channel14
		}
		return (freq - Freq2407MHz) / ChannelSpacing5MHz
	}
	if freq >= Freq5170MHz && freq <= Freq5825MHz {
		return (freq - Freq5000MHz) / ChannelSpacing5MHz
	}
	if freq >= Freq5955MHz && freq <= Freq7115MHz {
		if freq == Freq5935MHz || freq == Freq5955MHz {
			return Channel2
		}
		if freq == Freq5965MHz || freq == Freq5985MHz {
			return Channel6
		}
		return (freq - Freq5950MHz) / ChannelSpacing5MHz
	}
	return 0
}
