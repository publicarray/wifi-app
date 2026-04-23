package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the user-tunable runtime configuration. Persisted as TOML at
// configPath() (XDG-aware on Linux, Application Support on macOS, AppData on
// Windows). Missing or unreadable files fall back to DefaultConfig() — the
// app must always start cleanly even with no config.
//
// Field-level notes:
//   - ScanIntervalSeconds: how often the backend triggers a fresh scan.
//     Lower = more current data + more battery drain + more chance of EBUSY
//     contention with NetworkManager.
//   - SignalHistoryMinutes: how much per-AP signal history the connected
//     chart and report generator can draw on.
//   - RoamingHistorySize: hard cap on retained roaming events.
//   - DefaultInterface: pre-selects the dropdown on launch when present.
//   - LatencyTargets: hosts the future latency sampler (P2.1) will probe.
//     "gateway" is a magic value resolved at runtime to the default route.
//   - ReportTemplatePath: future override for the MSP report template
//     (P3.3); empty means use the embedded default.
type Config struct {
	ScanIntervalSeconds  int      `toml:"scan_interval_seconds" json:"scanIntervalSeconds"`
	SignalHistoryMinutes int      `toml:"signal_history_minutes" json:"signalHistoryMinutes"`
	RoamingHistorySize   int      `toml:"roaming_history_size" json:"roamingHistorySize"`
	DefaultInterface     string   `toml:"default_interface" json:"defaultInterface"`
	LatencyTargets       []string `toml:"latency_targets" json:"latencyTargets"`
	ReportTemplatePath   string   `toml:"report_template_path" json:"reportTemplatePath"`
}

// DefaultConfig returns the values used when no config file exists or fields
// are missing. Keep these aligned with the legacy package-constant defaults
// so behaviour doesn't change for users who never touch the config file.
func DefaultConfig() Config {
	return Config{
		ScanIntervalSeconds:  4,
		SignalHistoryMinutes: 10,
		RoamingHistorySize:   100,
		DefaultInterface:     "",
		LatencyTargets:       []string{"gateway", "1.1.1.1"},
		ReportTemplatePath:   "",
	}
}

// validate clamps out-of-range values so a hand-edited file with a 0 or
// negative scan interval can't lock up the scan loop or starve memory.
// Returns the clamped config plus a (possibly nil) wrapping error describing
// the adjustments — callers can surface that to the user.
func (c Config) validate() (Config, error) {
	defaults := DefaultConfig()
	var notes []string

	if c.ScanIntervalSeconds < 1 {
		notes = append(notes, fmt.Sprintf("scan_interval_seconds=%d clamped to %d", c.ScanIntervalSeconds, defaults.ScanIntervalSeconds))
		c.ScanIntervalSeconds = defaults.ScanIntervalSeconds
	}
	if c.ScanIntervalSeconds > 600 {
		// 10 minutes is plenty even for ultra-slow polling.
		notes = append(notes, fmt.Sprintf("scan_interval_seconds=%d clamped to 600", c.ScanIntervalSeconds))
		c.ScanIntervalSeconds = 600
	}
	if c.SignalHistoryMinutes < 1 {
		c.SignalHistoryMinutes = defaults.SignalHistoryMinutes
	}
	if c.SignalHistoryMinutes > 240 {
		notes = append(notes, fmt.Sprintf("signal_history_minutes=%d clamped to 240", c.SignalHistoryMinutes))
		c.SignalHistoryMinutes = 240
	}
	if c.RoamingHistorySize < 1 {
		c.RoamingHistorySize = defaults.RoamingHistorySize
	}
	if c.RoamingHistorySize > 10000 {
		notes = append(notes, fmt.Sprintf("roaming_history_size=%d clamped to 10000", c.RoamingHistorySize))
		c.RoamingHistorySize = 10000
	}
	if c.LatencyTargets == nil {
		c.LatencyTargets = defaults.LatencyTargets
	}

	if len(notes) == 0 {
		return c, nil
	}
	return c, errors.New("config adjusted: " + joinComma(notes))
}

func joinComma(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	out := items[0]
	for _, s := range items[1:] {
		out += ", " + s
	}
	return out
}

// Derived helpers — computed on demand so a config update via SaveConfig is
// visible to the next caller without a restart.

// ScanInterval returns the configured scan interval as a time.Duration.
func (c Config) ScanInterval() time.Duration {
	return time.Duration(c.ScanIntervalSeconds) * time.Second
}

// SignalHistorySize returns the per-AP signal history capacity in points,
// derived from the minutes-of-history setting + scan interval. The default
// (10 minutes / 4 s) yields 150 points; the legacy hard-coded value was 600
// (10 minutes at the assumed 1-second cadence). This derivation keeps the
// chart's time window stable as the user changes the scan interval.
func (c Config) SignalHistorySize() int {
	n := c.SignalHistoryMinutes * 60 / max1(c.ScanIntervalSeconds)
	if n < 1 {
		return 1
	}
	return n
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

// configPath returns the XDG-aware path of the user's config file. Used by
// LoadConfig and SaveConfig so they always agree on the location.
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "wifi-app", "config.toml"), nil
}

// LoadConfig reads the config file. A missing file returns DefaultConfig()
// and a nil error — that's the normal first-run path. A present-but-invalid
// file returns DefaultConfig() and a wrapping error so the app can surface a
// warning without refusing to start.
func LoadConfig() (Config, error) {
	defaults := DefaultConfig()
	path, err := configPath()
	if err != nil {
		return defaults, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults, nil
		}
		return defaults, fmt.Errorf("read %s: %w", path, err)
	}

	// Start from defaults so missing keys don't zero out the struct.
	cfg := defaults
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return defaults, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg, _ = cfg.validate()
	return cfg, nil
}

// SaveConfig writes the config atomically (tmp file + rename) so a crash
// mid-write can never leave a half-written TOML file that prevents next-run
// startup. Creates the parent directory on demand.
//
// Honours SUDO_UID/SUDO_GID like SaveReport so a config written under sudo is
// still owned by the invoking user.
func SaveConfig(cfg Config) error {
	cfg, _ = cfg.validate()

	path, err := configPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	var buf saveBuffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = "  "
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encode toml: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}

	// Best-effort chown; SaveConfig is called from the UI so we don't have a
	// natural place to surface a warning. The user will notice next launch
	// if the file is unreadable.
	_ = chownToSudoUser(path)
	_ = chownToSudoUser(dir)
	return nil
}

// saveBuffer is a tiny io.Writer + Bytes() shim so we can encode TOML into a
// byte slice without pulling in bytes.Buffer's full surface (it's fine, just
// keeps the imports tight).
type saveBuffer struct{ b []byte }

func (s *saveBuffer) Write(p []byte) (int, error) { s.b = append(s.b, p...); return len(p), nil }
func (s *saveBuffer) Bytes() []byte                { return s.b }

// chownToSudoUser re-chowns path back to the SUDO_UID/SUDO_GID user when the
// process is running under sudo. Without this, files written by the app
// (config, reports, session DB) would be owned by root and unreadable to the
// invoking user.
//
// Returns nil when no chown was needed (not running under sudo) OR when the
// chown succeeded; returns an error only when SUDO_UID/GID looked like a
// real ask but the chown itself failed. Callers that want to surface a
// warning can do so on a non-nil error.
func chownToSudoUser(path string) error {
	uidStr := os.Getenv("SUDO_UID")
	gidStr := os.Getenv("SUDO_GID")
	if uidStr == "" || gidStr == "" {
		return nil
	}
	uid, uidErr := strconv.Atoi(uidStr)
	gid, gidErr := strconv.Atoi(gidStr)
	if uidErr != nil || gidErr != nil {
		return fmt.Errorf("invalid SUDO_UID/SUDO_GID env (%q/%q)", uidStr, gidStr)
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("chown %s to %d:%d: %w", path, uid, gid, err)
	}
	return nil
}

// liveConfig is a lock-protected wrapper exposing the current Config to the
// scan loop and other consumers. Updates via UpdateConfig are visible to the
// next read; nothing in-flight is interrupted.
type liveConfig struct {
	mu  sync.RWMutex
	cur Config
}

func newLiveConfig(initial Config) *liveConfig {
	return &liveConfig{cur: initial}
}

func (l *liveConfig) Get() Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.cur
}

func (l *liveConfig) Set(c Config) {
	c, _ = c.validate()
	l.mu.Lock()
	l.cur = c
	l.mu.Unlock()
}
