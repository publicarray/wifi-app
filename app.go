package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx         context.Context
	wifiService *WiFiService
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		wifiService: NewWiFiService(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	installWailsForwarding(ctx)
	a.wifiService.SetContext(ctx)
}

// shutdown is called when the app is exiting.
func (a *App) shutdown(ctx context.Context) {
	_ = a.wifiService.Close()
}

// GetAvailableInterfaces returns a list of available WiFi interfaces
func (a *App) GetAvailableInterfaces() ([]string, error) {
	return a.wifiService.scanner.GetInterfaces()
}

// StartScanning begins periodic WiFi scanning on the specified interface
func (a *App) StartScanning(iface string) error {
	return a.wifiService.StartScanning(iface)
}

// StopScanning stops the periodic WiFi scanning
func (a *App) StopScanning() {
	a.wifiService.StopScanning()
}

// GetNetworks returns the list of discovered WiFi networks
func (a *App) GetNetworks() []Network {
	return a.wifiService.GetNetworks()
}

// GetClientStats returns current client connection statistics
func (a *App) GetClientStats() ClientStats {
	return a.wifiService.GetClientStats()
}

// GetChannelAnalysis returns channel utilization information
func (a *App) GetChannelAnalysis() []ChannelInfo {
	return a.wifiService.GetChannelAnalysis()
}

func (a *App) IsScanning() bool {
	return a.wifiService.IsScanning()
}

func (a *App) GetRoamingAnalysis() RoamingQualityReport {
	return a.wifiService.AnalyzeRoamingQuality()
}

// GetConfig returns the current persisted configuration. Used by the
// Settings UI to populate inputs.
func (a *App) GetConfig() Config {
	return a.wifiService.GetConfig()
}

// SaveConfig validates, persists, and applies the new configuration. The
// scan loop reads the live config at the top of each iteration, so changes
// take effect on the next scan tick.
func (a *App) SaveConfig(cfg Config) error {
	return a.wifiService.UpdateConfig(cfg)
}

func (a *App) GetAPPlacementRecommendations() []string {
	return a.wifiService.GetAPPlacementRecommendations()
}

// GetLatency returns the current per-target latency summaries. The sampler
// also emits `latency:updated` events every second — this binding is the
// synchronous hydrate path the UI uses on tab switch.
func (a *App) GetLatency() []LatencyTargetSummary {
	return a.wifiService.GetLatencySummaries()
}

func (a *App) ExportNetworks(format string) (string, error) {
	networks := a.wifiService.GetNetworks()

	if format == "json" {
		return a.exportToJSON(networks)
	} else if format == "csv" {
		return a.exportToCSV(networks)
	}

	return "", fmt.Errorf("unsupported format: %s. Use 'json' or 'csv'", format)
}

func (a *App) exportToJSON(networks []Network) (string, error) {
	data, err := json.MarshalIndent(networks, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

// exportToCSV writes one row per access point with full per-AP fields.
// Previously this wrote one row per network using fmt.Sprintf, which broke on
// SSIDs containing double quotes. encoding/csv handles RFC 4180 quoting
// correctly and the per-AP layout brings CSV into parity with JSON.
func (a *App) exportToCSV(networks []Network) (string, error) {
	var builder strings.Builder
	w := csv.NewWriter(&builder)

	if err := w.Write([]string{
		"SSID", "BSSID", "Vendor", "Signal_dBm", "SignalQuality_pct",
		"Noise_dBm", "SNR_dB", "Channel", "ChannelWidth_MHz", "Frequency_MHz",
		"Band", "Security", "SecurityCiphers", "AuthMethods", "PMF", "DFS",
		"BSSLoadStations", "BSSLoadUtilization_pct", "MaxPhyRate_Mbps",
		"MIMOStreams", "Capabilities", "APCount", "NetworkHasIssues",
		"NetworkIssues",
	}); err != nil {
		return "", err
	}

	for _, network := range networks {
		issues := strings.Join(network.IssueMessages, "; ")
		hasIssues := "No"
		if network.HasIssues {
			hasIssues = "Yes"
		}
		for _, ap := range network.AccessPoints {
			if err := w.Write([]string{
				ap.SSID,
				ap.BSSID,
				ap.Vendor,
				strconv.Itoa(ap.Signal),
				strconv.Itoa(ap.SignalQuality),
				strconv.Itoa(ap.Noise),
				strconv.Itoa(ap.SNR),
				strconv.Itoa(ap.Channel),
				strconv.Itoa(ap.ChannelWidth),
				strconv.Itoa(ap.Frequency),
				ap.Band,
				ap.Security,
				strings.Join(ap.SecurityCiphers, "|"),
				strings.Join(ap.AuthMethods, "|"),
				ap.PMF,
				strconv.FormatBool(ap.DFS),
				optIntString(ap.BSSLoadStations),
				optIntString(ap.BSSLoadUtilization),
				strconv.Itoa(ap.MaxPhyRate),
				strconv.Itoa(ap.MIMOStreams),
				strings.Join(ap.Capabilities, "|"),
				strconv.Itoa(network.APCount),
				hasIssues,
				issues,
			}); err != nil {
				return "", err
			}
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return builder.String(), nil
}

// optIntString renders an optional *int field as a string, using an empty
// string to represent absence (IE not present in the beacon).
func optIntString(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func (a *App) ExportClientStats() (string, error) {
	stats := a.wifiService.GetClientStats()

	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}

// SaveReport opens a save dialog and writes the given content to disk.
// Returns the chosen file path or empty string if the user cancels.
func (a *App) SaveReport(filename string, content string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app context not initialized")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename:      filename,
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	if err := chownToSudoUser(path); err != nil {
		slog.Warn("failed to chown saved report", "path", path, "err", err)
	}
	return path, nil
}
