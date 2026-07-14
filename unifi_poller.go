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

	// vendorFn resolves a MAC to a vendor name for client-roster enrichment.
	// Injected once at construction (before the loop starts) from the scanner's
	// OUI database; nil on platforms/backends that don't expose one.
	vendorFn func(string) string

	status         UniFiStatus
	devices        []UniFiDeviceInfo // MACs normalized; matching source for enrichment
	hiddenSSIDHint string            // controller name for hidden SSIDs, when unambiguous
}

// uniFiRosterCap bounds the per-device client roster carried in events so a
// dense site can't bloat every unifi:updated payload.
const uniFiRosterCap = 50

func NewUniFiPoller(cfg *liveConfig) *UniFiPoller {
	return &UniFiPoller{
		cfg:       cfg,
		eventName: "unifi:updated",
		poke:      make(chan struct{}, 1),
	}
}

// SetVendorLookup injects the OUI vendor resolver. Call before Start; it is
// read from the poll goroutine without locking on that assumption.
func (p *UniFiPoller) SetVendorLookup(fn func(string) string) {
	p.vendorFn = fn
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
		p.publish(UniFiStatus{Configured: false}, nil, "")
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
		p.publish(status, nil, "")
		return
	}
	site, ok := pickUniFiSite(sites, cfg.UniFiSite)
	if !ok {
		status.Error = "controller returned no matching site"
		p.publish(status, nil, "")
		return
	}
	status.SiteID = site.ID
	status.SiteName = site.Name

	devices, err := client.Devices(tickCtx, site.ID)
	if err != nil {
		status.Error = err.Error()
		slog.Warn("unifi poll failed", "event", "unifi_error", "stage", "devices", "err", err)
		p.publish(status, nil, "")
		return
	}

	// Clients are best-effort: a failure here still leaves the device list
	// usable, just without per-AP client counts/rosters.
	rosterByDevice := map[string][]UniFiClientInfo{}
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
					mac := normalizeMAC(c.MACAddress)
					vendor := ""
					if p.vendorFn != nil {
						vendor = p.vendorFn(mac)
					}
					rosterByDevice[c.UplinkDeviceID] = append(rosterByDevice[c.UplinkDeviceID], UniFiClientInfo{
						Name:        c.Name,
						MAC:         mac,
						IP:          c.IPAddress,
						Vendor:      vendor,
						Guest:       strings.EqualFold(c.Access.Type, "GUEST"),
						Randomized:  isRandomizedMAC(mac),
						ConnectedAt: c.ConnectedAt,
					})
				}
			case "WIRED":
				status.WiredClients++
			}
		}
	}

	// WLAN list is best-effort and only feeds hidden-SSID naming; many
	// controller versions don't expose it and that's fine.
	hiddenSSIDHint := ""
	if wlans, werr := client.WLANs(tickCtx, site.ID); werr == nil {
		hiddenSSIDHint = pickHiddenWLANName(wlans)
	}

	infos := make([]UniFiDeviceInfo, 0, len(devices))
	for _, d := range devices {
		roster := rosterByDevice[d.ID]
		sort.Slice(roster, func(i, j int) bool {
			if roster[i].Name != roster[j].Name {
				return roster[i].Name < roster[j].Name
			}
			return roster[i].MAC < roster[j].MAC
		})
		count := len(roster)
		if len(roster) > uniFiRosterCap {
			roster = roster[:uniFiRosterCap]
		}
		infos = append(infos, UniFiDeviceInfo{
			ID:                d.ID,
			Name:              d.Name,
			Model:             d.Model,
			MAC:               normalizeMAC(d.MACAddress),
			IP:                d.IPAddress,
			State:             d.State,
			FirmwareVersion:   d.FirmwareVersion,
			FirmwareUpdatable: d.FirmwareUpdatable,
			IsAccessPoint:     hasFeature(d.Features, "accessPoint"),
			ClientCount:       count,
			Clients:           roster,
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })

	// Deep diagnostics (uplink/mesh, radios, health) come from per-device
	// endpoints; best-effort so a failure just leaves those fields empty.
	// Sort is done first so the concurrent writers below never reorder the
	// slice out from under the goroutines (each writes only its own index).
	enrichDeviceDetails(tickCtx, client, site.ID, infos)
	resolveUplinks(infos)

	status.Connected = true
	status.Devices = infos
	status.ApplicationVersion = p.cachedAppVersion(tickCtx, client)

	p.publish(status, infos, hiddenSSIDHint)
}

// pickHiddenWLANName returns the controller-configured name of the hidden
// SSID when the site has exactly one enabled hidden WLAN. With zero or
// several hidden WLANs there is no safe way to label a specific hidden BSSID,
// so "" (don't label) is returned instead of guessing.
func pickHiddenWLANName(wlans []uniFiWLAN) string {
	var names []string
	for _, w := range wlans {
		if w.Enabled != nil && !*w.Enabled {
			continue
		}
		hidden := w.Hidden || (w.HideSSID != nil && *w.HideSSID)
		if !hidden {
			continue
		}
		name := w.SSID
		if name == "" {
			name = w.Name
		}
		if name != "" {
			names = appendUnique(names, name)
		}
	}
	if len(names) == 1 {
		return names[0]
	}
	return ""
}

// uniFiDeepPollConcurrency bounds how many device detail/stats requests run
// at once so a dense site doesn't open dozens of parallel connections.
const uniFiDeepPollConcurrency = 6

// hasFeature reports whether a device's feature list contains name.
func hasFeature(features []string, name string) bool {
	for _, f := range features {
		if strings.EqualFold(f, name) {
			return true
		}
	}
	return false
}

// enrichDeviceDetails fills uplink/radio/health fields on infos by fetching the
// per-device detail + latest-statistics endpoints concurrently. Best-effort:
// any per-device error leaves that device's extended fields at their zero
// value. Each goroutine writes only its own slice index, so no lock is needed
// and the slice must not be reordered while this runs.
func enrichDeviceDetails(ctx context.Context, client *uniFiClient, siteID string, infos []UniFiDeviceInfo) {
	if len(infos) == 0 {
		return
	}
	sem := make(chan struct{}, uniFiDeepPollConcurrency)
	var wg sync.WaitGroup
	for i := range infos {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			mergeDeviceDetail(ctx, client, siteID, &infos[i])
		}(i)
	}
	wg.Wait()
}

// mergeDeviceDetail fetches and merges detail + stats for one device.
func mergeDeviceDetail(ctx context.Context, client *uniFiClient, siteID string, info *UniFiDeviceInfo) {
	detail, derr := client.DeviceDetail(ctx, siteID, info.ID)
	if derr == nil {
		info.UplinkDeviceID = detail.Uplink.DeviceID
		info.Radios = radiosFromDetail(detail.Interfaces.Radios)
		// Note: the integration API exposes no wired-vs-wireless(mesh) medium
		// on the device uplink — only a parent deviceId. Port state is not a
		// reliable proxy either (a PoE-injected mesh AP still links its port,
		// and "parent is an AP" covers both wired daisy-chains and mesh). So
		// medium is deliberately NOT inferred; we surface topology + the
		// uplink port's negotiated speed only.
		for _, p := range detail.Interfaces.Ports {
			if strings.EqualFold(p.State, "UP") && p.SpeedMbps > info.UplinkPortSpeedMbps {
				info.UplinkPortSpeedMbps = p.SpeedMbps
			}
		}
	} else {
		slog.Debug("unifi device detail failed", "event", "unifi_detail", "device", info.ID, "err", derr)
	}

	stats, serr := client.DeviceStats(ctx, siteID, info.ID)
	if serr != nil {
		slog.Debug("unifi device stats failed", "event", "unifi_stats", "device", info.ID, "err", serr)
		return
	}
	info.UptimeSec = stats.UptimeSec
	info.UplinkTxBps = stats.Uplink.TxRateBps
	info.UplinkRxBps = stats.Uplink.RxRateBps
	if stats.CPUUtilizationPct > 0 {
		info.CPUPct = floatPtr(stats.CPUUtilizationPct)
	}
	if stats.MemoryUtilizationPct > 0 {
		info.MemPct = floatPtr(stats.MemoryUtilizationPct)
	}
	if stats.LoadAverage1Min > 0 {
		info.LoadAvg1 = floatPtr(stats.LoadAverage1Min)
	}
	// Join live TX-retry rates onto the configured radios by band.
	for _, r := range stats.Interfaces.Radios {
		for i := range info.Radios {
			if info.Radios[i].Band == r.FrequencyGHz {
				info.Radios[i].TxRetriesPct = floatPtr(r.TxRetriesPct)
			}
		}
	}
}

// radiosFromDetail converts the detail radio entries into UI radio infos.
func radiosFromDetail(radios []uniFiRadio) []UniFiRadioInfo {
	if len(radios) == 0 {
		return nil
	}
	out := make([]UniFiRadioInfo, 0, len(radios))
	for _, r := range radios {
		out = append(out, UniFiRadioInfo{
			Band:     r.FrequencyGHz,
			Channel:  r.Channel,
			WidthMHz: r.ChannelWidthMHz,
			Standard: r.WLANStandard,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Band < out[j].Band })
	return out
}

// resolveUplinks turns each device's uplink device id into a parent name for
// topology display. The wired/wireless medium is decided from port state in
// mergeDeviceDetail — NOT from the parent device type. Runs after
// enrichDeviceDetails so every UplinkDeviceID is populated.
func resolveUplinks(infos []UniFiDeviceInfo) {
	byID := make(map[string]*UniFiDeviceInfo, len(infos))
	for i := range infos {
		byID[infos[i].ID] = &infos[i]
	}
	for i := range infos {
		if infos[i].UplinkDeviceID == "" {
			continue
		}
		if parent, ok := byID[infos[i].UplinkDeviceID]; ok {
			infos[i].UplinkName = parent.Name
		}
	}
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
func (p *UniFiPoller) publish(status UniFiStatus, devices []UniFiDeviceInfo, hiddenSSIDHint string) {
	p.mu.Lock()
	p.status = status
	p.devices = devices
	p.hiddenSSIDHint = hiddenSSIDHint
	ctx := p.wailsCtx
	p.mu.Unlock()

	if ctx != nil {
		wailsruntime.EventsEmit(ctx, p.eventName, status)
	}
}

// ResolveAPNames maps arbitrary BSSIDs (e.g. from roaming history, whose APs
// may no longer be in scan range) to controller device names using the same
// matching heuristic as scan enrichment. Unmatched BSSIDs are omitted.
func (p *UniFiPoller) ResolveAPNames(bssids []string) map[string]string {
	p.mu.RLock()
	devices := p.devices
	p.mu.RUnlock()

	out := make(map[string]string)
	if len(devices) == 0 {
		return out
	}
	for _, bssid := range bssids {
		if bssid == "" {
			continue
		}
		if d, ok := matchUniFiDevice(devices, bssid); ok && d.Name != "" {
			out[bssid] = d.Name
		}
	}
	return out
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
	hiddenSSIDHint := p.hiddenSSIDHint
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
		aps[i].UniFiDeviceID = d.ID
		// Label hidden SSIDs on our own APs when the controller has exactly
		// one enabled hidden WLAN (ambiguous sites get no label).
		if aps[i].SSID == "" && hiddenSSIDHint != "" {
			aps[i].UniFiHiddenSSID = hiddenSSIDHint
		}
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

// isRandomizedMAC reports whether a MAC is locally administered (the
// second-least-significant bit of the first octet is set) — the hallmark of an
// OS-generated privacy/randomized MAC. Useful context for a tech: such clients
// won't OUI-resolve to a real vendor and rotate their address.
func isRandomizedMAC(mac string) bool {
	oct, ok := macOctets(mac)
	if !ok {
		return false
	}
	return oct[0]&0x02 != 0
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
