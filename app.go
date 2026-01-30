package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	a.wifiService.SetContext(ctx)
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

func (a *App) GetRoamingAnalysis() map[string]interface{} {
	return a.wifiService.AnalyzeRoamingQuality()
}

func (a *App) GetAPPlacementRecommendations() []string {
	return a.wifiService.GetAPPlacementRecommendations()
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

func (a *App) exportToCSV(networks []Network) (string, error) {
	var builder strings.Builder

	builder.WriteString("SSID,AP Count,Best Signal,Channel,Security,Has Issues\n")

	for _, network := range networks {
		issues := "No"
		if network.HasIssues {
			issues = "Yes"
		}
		builder.WriteString(fmt.Sprintf("\"%s\",%d,%d,%d,\"%s\",%s\n",
			network.SSID,
			network.APCount,
			network.BestSignal,
			network.Channel,
			network.Security,
			issues,
		))
	}

	return builder.String(), nil
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
	if sudoUID := os.Getenv("SUDO_UID"); sudoUID != "" {
		if sudoGID := os.Getenv("SUDO_GID"); sudoGID != "" {
			uid, uidErr := strconv.Atoi(sudoUID)
			gid, gidErr := strconv.Atoi(sudoGID)
			if uidErr == nil && gidErr == nil {
				_ = os.Chown(path, uid, gid)
			}
		}
	}
	return path, nil
}
