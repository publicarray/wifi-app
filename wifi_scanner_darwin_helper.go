//go:build darwin

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// helperRecord is the per-BSS payload emitted by wifi-app-mac-helper. The
// helper writes a JSON document of shape {"records":[...]} to stdout.
type helperRecord struct {
	BSSID string `json:"bssid"`
	SSID  string `json:"ssid"`
	IEHex string `json:"ie_hex"`
}

type helperPayload struct {
	Records []helperRecord `json:"records"`
}

// augmentWithHelper invokes the optional Apple80211 helper subprocess and
// applies its IE-derived fields back onto the AP slice in place. Silent
// no-op when:
//   - no helper path is configured;
//   - the helper exits non-zero (e.g. private framework symbols not found);
//   - the helper times out;
//   - JSON parse fails or the BSSID has no match in `aps`.
//
// The helper is best-effort by design — every field it can populate stays at
// its CoreWLAN-derived value when augmentation fails. We log the failure at
// Info level so a misconfigured helper path is visible without spamming the
// scan log on every tick once the user disables it.
func (s *darwinScanner) augmentWithHelper(iface string, aps []AccessPoint) {
	s.mu.Lock()
	helperPath := s.macHelperPath
	s.mu.Unlock()
	if helperPath == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, helperPath, "-iface", iface)
	out, err := cmd.Output()
	if err != nil {
		// Capture stderr from ExitError so the log line is actionable.
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			slog.Info("mac helper failed", "err", err, "stderr", strings.TrimSpace(string(ee.Stderr)))
		} else {
			slog.Info("mac helper failed", "err", err)
		}
		return
	}

	var payload helperPayload
	if err := json.Unmarshal(out, &payload); err != nil {
		slog.Info("mac helper JSON parse failed", "err", err)
		return
	}
	if len(payload.Records) == 0 {
		return
	}

	// Map by lowercased BSSID for fast lookup; the helper already lowercases
	// but be defensive in case a future build changes that.
	byBSSID := make(map[string]helperRecord, len(payload.Records))
	for _, rec := range payload.Records {
		byBSSID[strings.ToLower(rec.BSSID)] = rec
	}

	matched := 0
	for i := range aps {
		rec, ok := byBSSID[strings.ToLower(aps[i].BSSID)]
		if !ok || rec.IEHex == "" {
			continue
		}
		raw, err := hex.DecodeString(rec.IEHex)
		if err != nil {
			continue
		}
		parseInformationElements(raw, &aps[i])
		matched++
	}
	if matched > 0 {
		slog.Debug("mac helper augmented APs", "matched", matched, "total_records", len(payload.Records))
	}
}
