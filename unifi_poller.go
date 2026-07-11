package main

// UniFiPoller periodically pulls devices + connected clients from a UniFi
// Network controller (see unifi_client.go) and publishes a UniFiStatus
// snapshot over the `unifi:updated` Wails event. It also serves as the join
// point for scan enrichment: EnrichAccessPoints matches scanned BSSIDs to
// controller devices so AP rows can show the controller-assigned name/model.
//
// Lifecycle mirrors LatencySampler: bound to the shared liveConfig, started
// once from WiFiService.SetContext, config re-read every tick so Settings
// changes apply without a restart, and all Wails emits happen off-lock.

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// uniFiPollInterval is the controller poll cadence. Controller data changes
// slowly (device names, client counts), so 30 s keeps load negligible;
// SaveConfig pokes the loop for an immediate re-poll when settings change.
const uniFiPollInterval = 30 * time.Second

// uniFiTickBudget bounds one full poll round (sites + devices + clients).
const uniFiTickBudget = 25 * time.Second

type UniFiPoller struct {
	mu sync.RWMutex

	cfg       *liveConfig
	wailsCtx  context.Context
	eventName string

	running    bool
	cancelLoop context.CancelFunc
	poke       chan struct{}

	client     *uniFiClient
	clientKey  string // url\x00key\x00insecure — client is rebuilt when it changes
	appVersion string // cached /info result per client

	status  UniFiStatus
	devices []UniFiDeviceInfo // MACs normalized; matching source for enrichment
}

func NewUniFiPoller(cfg *liveConfig) *UniFiPoller {
	return &UniFiPoller{
		cfg:       cfg,
		eventName: "unifi:updated",
		poke:      make(chan struct{}, 1),
	}
}

// SetWailsContext attaches the Wails runtime context used for EventsEmit.
func (p *UniFiPoller) SetWailsContext(ctx context.Context) {
	p.mu.Lock()
	p.wailsCtx = ctx
	p.mu.Unlock()
}

// Start boots the poll loop. Idempotent while running.
func (p *UniFiPoller) Start(parent context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	p.running = true
	p.cancelLoop = cancel
	p.mu.Unlock()

	go p.loop(ctx)
}

// Stop cancels the poll loop.
func (p *UniFiPoller) Stop() {
	p.mu.Lock()
	cancel := p.cancelLoop
	p.cancelLoop = nil
	p.running = false
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Poke asks the loop to re-poll now — called after SaveConfig so a freshly
// entered URL/API key takes effect immediately instead of after the next
// 30 s tick. Non-blocking; coalesces bursts.
func (p *UniFiPoller) Poke() {
	select {
	case p.poke <- struct{}{}:
	default:
	}
}

func (p *UniFiPoller) loop(ctx context.Context) {
	ticker := time.NewTicker(uniFiPollInterval)
	defer ticker.Stop()

	p.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick(ctx)
		case <-p.poke:
			p.tick(ctx)
		}
	}
}

func (p *UniFiPoller) tick(ctx context.Context) {
	cfg := p.cfg.Get()

	if cfg.UniFiControllerURL == "" || cfg.UniFiAPIKey == "" {
		p.publish(UniFiStatus{Configured: false}, nil)
		return
	}

	client := p.ensureClient(cfg)

	tickCtx, cancel := context.WithTimeout(ctx, uniFiTickBudget)
	defer cancel()

	status := UniFiStatus{
		Configured:    true,
		ControllerURL: cfg.UniFiControllerURL,
		LastUpdated:   time.Now(),
	}

	sites, err := client.Sites(tickCtx)
	if err != nil {
		status.Error = err.Error()
		slog.Warn("unifi poll failed", "event", "unifi_error", "stage", "sites", "err", err)
		p.publish(status, nil)
		return
	}
	site, ok := pickUniFiSite(sites, cfg.UniFiSite)
	if !ok {
		status.Error = "controller returned no matching site"
		p.publish(status, nil)
		return
	}
	status.SiteID = site.ID
	status.SiteName = site.Name

	devices, err := client.Devices(tickCtx, site.ID)
	if err != nil {
		status.Error = err.Error()
		slog.Warn("unifi poll failed", "event", "unifi_error", "stage", "devices", "err", err)
		p.publish(status, nil)
		return
	}

	// Clients are best-effort: a failure here still leaves the device list
	// usable, just without per-AP client counts.
	wirelessByDevice := map[string]int{}
	clients, err := client.Clients(tickCtx, site.ID)
	if err != nil {
		status.Error = "client list unavailable: " + err.Error()
		slog.Warn("unifi poll degraded", "event", "unifi_error", "stage", "clients", "err", err)
	} else {
		for _, c := range clients {
			switch strings.ToUpper(c.Type) {
			case "WIRELESS":
				status.WirelessClients++
				if c.UplinkDeviceID != "" {
					wirelessByDevice[c.UplinkDeviceID]++
				}
			case "WIRED":
				status.WiredClients++
			}
		}
	}

	infos := make([]UniFiDeviceInfo, 0, len(devices))
	for _, d := range devices {
		infos = append(infos, UniFiDeviceInfo{
			ID:              d.ID,
			Name:            d.Name,
			Model:           d.Model,
			MAC:             normalizeMAC(d.MACAddress),
			IP:              d.IPAddress,
			State:           d.State,
			FirmwareVersion: d.FirmwareVersion,
			ClientCount:     wirelessByDevice[d.ID],
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })

	status.Connected = true
	status.Devices = infos
	status.ApplicationVersion = p.cachedAppVersion(tickCtx, client)

	p.publish(status, infos)
}

// ensureClient returns the HTTP client, rebuilding it when the controller
// URL / API key / TLS setting changed since the last tick.
func (p *UniFiPoller) ensureClient(cfg Config) *uniFiClient {
	key := cfg.UniFiControllerURL + "\x00" + cfg.UniFiAPIKey + "\x00" + strconv.FormatBool(cfg.UniFiAllowInsecureTLS)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil || p.clientKey != key {
		p.client = newUniFiClient(cfg.UniFiControllerURL, cfg.UniFiAPIKey, cfg.UniFiAllowInsecureTLS)
		p.clientKey = key
		p.appVersion = ""
	}
	return p.client
}

// cachedAppVersion fetches /info once per client lifetime (best-effort).
func (p *UniFiPoller) cachedAppVersion(ctx context.Context, client *uniFiClient) string {
	p.mu.RLock()
	v := p.appVersion
	p.mu.RUnlock()
	if v != "" {
		return v
	}
	info, err := client.Info(ctx)
	if err != nil || info.ApplicationVersion == "" {
		return ""
	}
	p.mu.Lock()
	p.appVersion = info.ApplicationVersion
	p.mu.Unlock()
	return info.ApplicationVersion
}

// publish stores the snapshot under lock and emits it after releasing —
// same emit-off-lock rule as the rest of the app.
func (p *UniFiPoller) publish(status UniFiStatus, devices []UniFiDeviceInfo) {
	p.mu.Lock()
	p.status = status
	p.devices = devices
	ctx := p.wailsCtx
	p.mu.Unlock()

	if ctx != nil {
		wailsruntime.EventsEmit(ctx, p.eventName, status)
	}
}

// Snapshot returns the last published status for the synchronous
// GetUniFiStatus binding (UI hydration on mount).
func (p *UniFiPoller) Snapshot() UniFiStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := p.status
	if n := len(p.status.Devices); n > 0 {
		out.Devices = make([]UniFiDeviceInfo, n)
		copy(out.Devices, p.status.Devices)
	}
	return out
}

// EnrichAccessPoints joins controller device records onto freshly scanned
// APs by BSSID. Runs in the scan path before aggregation; cheap map/slice
// work only, no I/O.
func (p *UniFiPoller) EnrichAccessPoints(aps []AccessPoint) {
	p.mu.RLock()
	devices := p.devices
	p.mu.RUnlock()
	if len(devices) == 0 {
		return
	}
	for i := range aps {
		d, ok := matchUniFiDevice(devices, aps[i].BSSID)
		if !ok {
			continue
		}
		aps[i].UniFiName = d.Name
		aps[i].UniFiModel = d.Model
		aps[i].UniFiIP = d.IP
		aps[i].UniFiState = d.State
		aps[i].UniFiClientCount = intPtr(d.ClientCount)
	}
}

// pickUniFiSite selects the configured site by id or name (case-insensitive);
// an empty preference means the first site — the common single-site setup.
func pickUniFiSite(sites []uniFiSite, preferred string) (uniFiSite, bool) {
	if len(sites) == 0 {
		return uniFiSite{}, false
	}
	if preferred == "" {
		return sites[0], true
	}
	for _, s := range sites {
		if strings.EqualFold(s.ID, preferred) || strings.EqualFold(s.Name, preferred) {
			return s, true
		}
	}
	return uniFiSite{}, false
}

// matchUniFiDevice finds the controller device a scanned BSSID belongs to.
//
// UniFi APs derive per-SSID/per-radio BSSIDs from the device base MAC using
// (at least) three schemes seen in the field:
//   - first octet mutated into locally-administered variants (68→6a→6e→…);
//   - last octet incremented per additional SSID (older UAP models);
//   - 4th octet stepped in 0x10 strides per SSID (U6/U7 era: 94→a4→b4→…).
//
// The heuristic: exact match first; otherwise ignore the first octet, anchor
// on octets 2-3 (OUI tail), and accept either a small last-octet delta or a
// 0x10-stride 4th-octet delta with the remaining octets identical. Candidates
// are scored (smaller = closer); ties across devices are treated as ambiguous
// and skipped rather than mis-labelled.
func matchUniFiDevice(devices []UniFiDeviceInfo, bssid string) (UniFiDeviceInfo, bool) {
	b := normalizeMAC(bssid)
	if b == "" {
		return UniFiDeviceInfo{}, false
	}

	for _, d := range devices {
		if d.MAC == b {
			return d, true
		}
	}

	bOct, ok := macOctets(b)
	if !ok {
		return UniFiDeviceInfo{}, false
	}

	bestScore := -1
	bestCount := 0
	var best UniFiDeviceInfo
	for _, d := range devices {
		dOct, ok := macOctets(d.MAC)
		if !ok {
			continue
		}
		score := uniFiBSSIDScore(dOct, bOct)
		if score < 0 {
			continue
		}
		switch {
		case bestScore < 0 || score < bestScore:
			bestScore = score
			bestCount = 1
			best = d
		case score == bestScore:
			bestCount++
		}
	}
	if bestScore >= 0 && bestCount == 1 {
		return best, true
	}
	return UniFiDeviceInfo{}, false
}

// macOctets parses a normalized MAC into six byte values.
func macOctets(mac string) ([6]int, bool) {
	var out [6]int
	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		return out, false
	}
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return out, false
		}
		out[i] = int(v)
	}
	return out, true
}

// uniFiBSSIDScore rates how plausibly bssid derives from a device base MAC.
// Returns -1 for no match; lower non-negative scores are closer matches.
// The first octet is ignored entirely (locally-administered variants).
func uniFiBSSIDScore(dev, bssid [6]int) int {
	if dev[1] != bssid[1] || dev[2] != bssid[2] {
		return -1
	}
	d3 := absInt(dev[3] - bssid[3])
	d4 := absInt(dev[4] - bssid[4])
	d5 := absInt(dev[5] - bssid[5])

	// Last-octet increment scheme: octets 4-5 identical, small final delta.
	const maxLastOctetDelta = 15
	if d3 == 0 && d4 == 0 && d5 <= maxLastOctetDelta {
		return d5
	}
	// 4th-octet stride scheme: octets 5-6 identical, 4th octet stepped in
	// 0x10 increments (low nibble preserved), up to 7 SSIDs out.
	if d4 == 0 && d5 == 0 && d3 != 0 && d3 <= 0x70 && d3%0x10 == 0 {
		return 16 + d3/0x10
	}
	return -1
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
