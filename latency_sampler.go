package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// gatewayLabel is the magic string in config.LatencyTargets that resolves to
// the system's default gateway at runtime. Treated case-insensitively so a
// hand-edited TOML with "Gateway" still works.
const gatewayLabel = "gateway"

// probeInterval is the fixed per-target probe cadence. 1 Hz matches the
// P2.1 design goal of "per-second latency sample" and keeps raw history
// aligned to the signal chart's per-scan cadence when summarised.
const probeInterval = time.Second

// probeTimeout is the per-probe wait budget. Kept under probeInterval so a
// timed-out probe still lets the next tick fire on schedule.
const probeTimeout = 900 * time.Millisecond

// tcpFallbackPort is the port used when ICMP is unavailable (no socket
// permissions, unsupported platform). 443 is reachable at ~all corporate
// sites and gives a realistic handshake RTT.
const tcpFallbackPort = "443"

// probeTarget holds everything a tick needs to probe one configured endpoint.
// Transport is decided at resolution time: ICMP when the runtime permits a
// raw or datagram ICMP socket, TCP (handshake RTT to 443) otherwise. The
// "gateway" magic label re-resolves opportunistically when defaultGateway()
// previously returned an error — typical on boot before DHCP lands.
type probeTarget struct {
	label     string
	raw       string // original user-provided value ("gateway" / IP / hostname)
	ip        net.IP // nil when transport == "tcp" and host is a name
	host      string // TCP fallback host (may be a name or IP literal)
	transport string // "icmp" | "tcp" | "unavailable"
	statusErr string // human-readable reason when transport == "unavailable"
}

// LatencySampler runs a goroutine per active sampler, fires one probe per
// target per probeInterval tick, keeps a bounded raw history per target, and
// emits a `latency:updated` Wails event every tick with a per-target summary
// (rolling stats over 1 / 10 / 60 s windows plus the raw history for
// charting). Multiple Start calls are coalesced — Start is idempotent.
//
// The sampler does not fall back to shelling out to `ping` — everything runs
// through golang.org/x/net/icmp (ICMP) or net.DialTimeout (TCP handshake), so
// the binary has no runtime dependency on the platform's ping command.
type LatencySampler struct {
	mu sync.RWMutex

	cfg       *liveConfig
	wailsCtx  context.Context
	eventName string

	running    bool
	cancelLoop context.CancelFunc

	targets []*probeTarget
	history map[string][]LatencyProbe // keyed by target label

	icmpConn    *icmp.PacketConn
	icmpNetwork string // "udp4" (unprivileged) or "ip4:icmp" (raw)
	icmpErr     string // populated when the sampler gave up trying to open ICMP

	// pending tracks in-flight ICMP echoes. The reader goroutine routes an
	// incoming reply to the probe goroutine waiting for it by keying on
	// seq only — not (id, seq) — because Linux's unprivileged datagram ICMP
	// socket ("udp4") rewrites the identifier field on send and rewrites it
	// back on receive to match the kernel-assigned socket id. Our Echo.ID on
	// the wire is silently ignored on that path, so a reply can never carry
	// the id we registered with. Seq is preserved end-to-end on both
	// transports, so it's the only field we can trust to demux. Concurrent
	// probes get unique seqs from a process-wide atomic counter, so there's
	// no collision risk even under a full probe fan-out.
	pendingMu sync.Mutex
	pending   map[uint16]chan *icmp.Echo
	seqCtr    uint32 // atomic; lower 16 bits used as Echo.Seq
	readerWG  sync.WaitGroup
}

// NewLatencySampler constructs a sampler bound to a liveConfig source. The
// Wails context is set later via SetWailsContext so App.startup can pass it
// through the same path the rest of the service uses.
func NewLatencySampler(cfg *liveConfig) *LatencySampler {
	return &LatencySampler{
		cfg:       cfg,
		eventName: "latency:updated",
		history:   make(map[string][]LatencyProbe),
		pending:   make(map[uint16]chan *icmp.Echo),
	}
}

// SetWailsContext attaches the Wails runtime context used for EventsEmit.
// Called from App.startup after SetContext on the wifi service.
func (s *LatencySampler) SetWailsContext(ctx context.Context) {
	s.mu.Lock()
	s.wailsCtx = ctx
	s.mu.Unlock()
}

// Start boots the sampler goroutine. Safe to call multiple times — a second
// Start is a no-op while the first is still running. The parent context
// drives shutdown; cancelling it stops the goroutine cleanly.
func (s *LatencySampler) Start(parent context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.running = true
	s.cancelLoop = cancel
	// Resolve targets up front so the first tick has something to probe.
	s.targets = s.resolveTargetsLocked(s.cfg.Get().LatencyTargets)
	startedReader := s.icmpConn != nil
	if startedReader {
		s.readerWG.Add(1)
	}
	s.mu.Unlock()

	if startedReader {
		go s.icmpReader(ctx)
	}
	go s.loop(ctx)
}

// Stop cancels the sampler goroutine and closes the ICMP socket if one was
// opened. Blocks until the loop acknowledges cancellation implicitly via the
// context it received.
func (s *LatencySampler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	cancel := s.cancelLoop
	s.cancelLoop = nil
	s.running = false
	conn := s.icmpConn
	s.icmpConn = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
	s.readerWG.Wait()
}

// loop is the sampler goroutine body. Every probeInterval it re-reads the
// config (picking up SaveConfig-driven target edits), fires one probe per
// target in parallel, appends results, and emits the per-target summary.
func (s *LatencySampler) loop(ctx context.Context) {
	// Align to wall-clock seconds so probes land on predictable boundaries
	// in the chart. Not load-bearing; purely cosmetic.
	align := time.Until(time.Now().Truncate(probeInterval).Add(probeInterval))
	if align > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(align):
		}
	}

	ticker := time.NewTicker(probeInterval)
	defer ticker.Stop()

	// Fire an immediate tick so the UI has data on the first event rather
	// than waiting a full interval.
	s.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick runs one probe round across all active targets and emits the
// aggregated summary. Config-driven target changes are picked up here.
func (s *LatencySampler) tick(ctx context.Context) {
	cfg := s.cfg.Get()
	s.reconcileTargets(cfg.LatencyTargets)

	historyCap := cfg.SignalHistoryMinutes * 60
	if historyCap < 30 {
		historyCap = 30
	}

	s.mu.RLock()
	targetsCopy := make([]*probeTarget, len(s.targets))
	copy(targetsCopy, s.targets)
	s.mu.RUnlock()

	if len(targetsCopy) == 0 {
		s.emitSummary(historyCap)
		return
	}

	results := make([]LatencyProbe, len(targetsCopy))
	var wg sync.WaitGroup
	wg.Add(len(targetsCopy))
	for i, t := range targetsCopy {
		go func(i int, t *probeTarget) {
			defer wg.Done()
			results[i] = s.probeOne(ctx, t)
		}(i, t)
	}
	wg.Wait()

	s.mu.Lock()
	for _, p := range results {
		if p.Label == "" {
			continue
		}
		h := s.history[p.Label]
		h = appendCapped(h, p, historyCap)
		s.history[p.Label] = h
	}
	s.mu.Unlock()

	s.emitSummary(historyCap)
}

// probeOne runs a single probe against one target and returns the resulting
// probe record. Targets that failed to resolve at configuration time are
// recorded as lost probes so the frontend has a continuous timeline for the
// target card — "no gateway" still shows up as a flat loss line, not a gap.
func (s *LatencySampler) probeOne(ctx context.Context, t *probeTarget) LatencyProbe {
	now := time.Now()
	probe := LatencyProbe{
		Timestamp: now,
		Target:    resolvedAddrString(t),
		Label:     t.label,
		Transport: t.transport,
		Lost:      true,
	}

	switch t.transport {
	case "icmp":
		rtt, err := s.probeICMP(ctx, t)
		if err != nil {
			slog.Debug("latency icmp probe failed", "label", t.label, "target", t.ip.String(), "err", err)
			return probe
		}
		probe.RTTMs = rtt
		probe.Lost = false
	case "tcp":
		rtt, err := s.probeTCP(ctx, t)
		if err != nil {
			slog.Debug("latency tcp probe failed", "label", t.label, "target", t.host, "err", err)
			return probe
		}
		probe.RTTMs = rtt
		probe.Lost = false
	}
	return probe
}

// probeICMP sends a single echo request and waits for the matching reply
// routed to us by the shared icmpReader goroutine. Demux is by Seq alone
// (see seqCtr docs on LatencySampler) so the same code path works for both
// the privileged raw socket and the unprivileged kernel-rewritten datagram
// socket. The process-wide atomic counter guarantees uniqueness across all
// in-flight probes.
func (s *LatencySampler) probeICMP(ctx context.Context, t *probeTarget) (float64, error) {
	s.mu.RLock()
	conn := s.icmpConn
	s.mu.RUnlock()
	if conn == nil {
		return 0, fmt.Errorf("icmp socket unavailable")
	}

	seq := uint16(atomic.AddUint32(&s.seqCtr, 1) & 0xFFFF)
	id := icmpIDFromLabel(t.label) // ignored on udp4 but harmless on ip4:icmp

	ch := make(chan *icmp.Echo, 1)
	s.pendingMu.Lock()
	s.pending[seq] = ch
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, seq)
		s.pendingMu.Unlock()
	}()

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  int(seq),
			Data: []byte("wifi-app-latency"),
		},
	}
	wire, err := msg.Marshal(nil)
	if err != nil {
		return 0, fmt.Errorf("marshal icmp: %w", err)
	}

	var dst net.Addr
	if s.icmpNetwork == "udp4" {
		dst = &net.UDPAddr{IP: t.ip}
	} else {
		dst = &net.IPAddr{IP: t.ip}
	}

	start := time.Now()
	if _, err := conn.WriteTo(wire, dst); err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}

	timer := time.NewTimer(probeTimeout)
	defer timer.Stop()
	select {
	case <-ch:
		return float64(time.Since(start).Microseconds()) / 1000.0, nil
	case <-timer.C:
		return 0, fmt.Errorf("timeout")
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// icmpReader loops on the shared ICMP socket, parses each reply, and routes
// it to the probe goroutine waiting on the (id, seq) key. Exits when the
// context is cancelled or the socket is closed.
//
// Uses a 500ms read deadline so cancellation is observed promptly without
// needing an external pipe / pipefd to unblock the Read.
func (s *LatencySampler) icmpReader(ctx context.Context) {
	defer s.readerWG.Done()

	s.mu.RLock()
	conn := s.icmpConn
	s.mu.RUnlock()
	if conn == nil {
		return
	}

	proto := ipv4.ICMPTypeEchoReply.Protocol()
	buf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			// Deadline hit OR the socket was closed during Stop — either
			// way loop back and check ctx.
			if isTimeout(err) {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			// Non-timeout error on a live socket: brief sleep to avoid
			// hammering the CPU if the kernel returns a persistent error.
			time.Sleep(50 * time.Millisecond)
			continue
		}
		parsed, err := icmp.ParseMessage(proto, buf[:n])
		if err != nil {
			continue
		}
		echo, ok := parsed.Body.(*icmp.Echo)
		if !ok {
			continue
		}
		seq := uint16(echo.Seq & 0xFFFF)
		s.pendingMu.Lock()
		ch, found := s.pending[seq]
		s.pendingMu.Unlock()
		if !found {
			continue
		}
		// Non-blocking send — probe may have already timed out and gone.
		select {
		case ch <- echo:
		default:
		}
	}
}

func isTimeout(err error) bool {
	type timeout interface{ Timeout() bool }
	if t, ok := err.(timeout); ok {
		return t.Timeout()
	}
	return false
}

// probeTCP measures TCP handshake RTT to tcpFallbackPort. This overestimates
// true layer-3 latency by ~one half RTT (SYN-ACK/ACK timing) and can't
// distinguish "server rejected connection" from "packet loss", but it works
// unprivileged on every platform we ship to.
func (s *LatencySampler) probeTCP(ctx context.Context, t *probeTarget) (float64, error) {
	host := t.host
	if host == "" {
		return 0, fmt.Errorf("tcp host empty")
	}
	addr := net.JoinHostPort(host, tcpFallbackPort)

	dialer := &net.Dialer{Timeout: probeTimeout}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	start := time.Now()
	conn, err := dialer.DialContext(probeCtx, "tcp4", addr)
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return float64(time.Since(start).Microseconds()) / 1000.0, nil
}

// reconcileTargets re-resolves the target list if it changed since the last
// tick, and re-resolves "gateway" every tick in case DHCP only just handed
// one over. Called from tick under no lock; takes the lock internally.
func (s *LatencySampler) reconcileTargets(configured []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !targetsDiffer(s.targets, configured) {
		// Still try to re-resolve "gateway" if it was unavailable — the
		// system may have gained a default route since last tick.
		for _, t := range s.targets {
			if !strings.EqualFold(t.raw, gatewayLabel) {
				continue
			}
			if t.transport != "unavailable" {
				continue
			}
			if ip, err := defaultGateway(); err == nil && ip != nil {
				t.ip = ip
				t.host = ip.String()
				t.statusErr = ""
				t.transport = s.chooseTransportForIP(ip)
			}
		}
		return
	}
	s.targets = s.resolveTargetsLocked(configured)
}

// resolveTargetsLocked turns the user's string list into probe targets. ICMP
// is preferred; TCP is the fallback when ICMP setup failed globally or when a
// host can't be resolved to an IPv4 address. Caller must hold s.mu.
func (s *LatencySampler) resolveTargetsLocked(configured []string) []*probeTarget {
	// Lazy-init ICMP once per sampler lifetime.
	if s.icmpConn == nil && s.icmpErr == "" {
		s.tryOpenICMP()
	}

	// De-duplicate by trimmed label so a config with "1.1.1.1" twice only
	// probes once. Empty entries are silently dropped.
	seen := make(map[string]bool)
	out := make([]*probeTarget, 0, len(configured))
	for _, raw := range configured {
		label := strings.TrimSpace(raw)
		if label == "" {
			continue
		}
		if seen[strings.ToLower(label)] {
			continue
		}
		seen[strings.ToLower(label)] = true

		t := &probeTarget{label: label, raw: label}
		if strings.EqualFold(label, gatewayLabel) {
			ip, err := defaultGateway()
			if err != nil || ip == nil {
				t.transport = "unavailable"
				t.statusErr = errString(err, "no default gateway")
			} else {
				t.ip = ip
				t.host = ip.String()
				t.transport = s.chooseTransportForIP(ip)
			}
		} else if ip := net.ParseIP(label); ip != nil && ip.To4() != nil {
			t.ip = ip.To4()
			t.host = label
			t.transport = s.chooseTransportForIP(t.ip)
		} else {
			// Hostname — resolve to IPv4 for ICMP, or leave as a name for
			// the TCP fallback (net.Dialer handles resolution with the
			// system resolver).
			if ips, err := net.LookupIP(label); err == nil {
				for _, ip := range ips {
					if v4 := ip.To4(); v4 != nil {
						t.ip = v4
						break
					}
				}
			}
			t.host = label
			t.transport = s.chooseTransportForIP(t.ip)
		}
		out = append(out, t)
	}
	return out
}

// chooseTransportForIP picks ICMP when the sampler has a working socket and
// an IPv4 address is available; otherwise falls back to TCP. Called on every
// target resolve; cheap.
func (s *LatencySampler) chooseTransportForIP(ip net.IP) string {
	if s.icmpConn != nil && ip != nil {
		return "icmp"
	}
	return "tcp"
}

// tryOpenICMP attempts an unprivileged ICMP socket first (Linux
// ping_group_range + macOS are happy; Windows is not), then the privileged
// raw ip4:icmp socket. On failure, s.icmpErr is set and the sampler falls
// back to TCP probes. Caller must hold s.mu.
func (s *LatencySampler) tryOpenICMP() {
	// Windows needs special handling we don't have today — skip straight to TCP.
	if goruntime.GOOS == "windows" {
		s.icmpErr = "icmp not supported on windows path yet"
		return
	}

	attempts := []string{"udp4", "ip4:icmp"}
	var lastErr error
	for _, network := range attempts {
		conn, err := icmp.ListenPacket(network, "0.0.0.0")
		if err != nil {
			lastErr = err
			continue
		}
		s.icmpConn = conn
		s.icmpNetwork = network
		return
	}
	s.icmpErr = errString(lastErr, "icmp socket open failed")
	slog.Info("latency sampler: icmp unavailable, falling back to tcp",
		"event", "latency_icmp_unavailable", "err", s.icmpErr)
}

// emitSummary builds one LatencyTargetSummary per active target and emits it
// over the Wails event bus. Reads history under lock and copies out so
// downstream Svelte listeners never alias the sampler's live slices.
func (s *LatencySampler) emitSummary(historyCap int) {
	s.mu.RLock()
	summaries := make([]LatencyTargetSummary, 0, len(s.targets))
	for _, t := range s.targets {
		hist := s.history[t.label]
		histCopy := make([]LatencyProbe, len(hist))
		copy(histCopy, hist)

		var last *LatencyProbe
		if n := len(histCopy); n > 0 {
			p := histCopy[n-1]
			last = &p
		}

		summary := LatencyTargetSummary{
			Label:     t.label,
			Target:    resolvedAddrString(t),
			Transport: t.transport,
			Available: t.transport != "unavailable",
			LastProbe: last,
			History:   histCopy,
			Windows:   computeWindows(histCopy),
		}
		summaries = append(summaries, summary)
	}
	ctx := s.wailsCtx
	s.mu.RUnlock()

	if ctx == nil {
		return
	}
	wailsruntime.EventsEmit(ctx, s.eventName, summaries)
}

// SnapshotSummaries returns the same per-target view the Wails event emits,
// for synchronous API calls (GetLatency binding).
func (s *LatencySampler) SnapshotSummaries() []LatencyTargetSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]LatencyTargetSummary, 0, len(s.targets))
	for _, t := range s.targets {
		hist := s.history[t.label]
		histCopy := make([]LatencyProbe, len(hist))
		copy(histCopy, hist)

		var last *LatencyProbe
		if n := len(histCopy); n > 0 {
			p := histCopy[n-1]
			last = &p
		}

		out = append(out, LatencyTargetSummary{
			Label:     t.label,
			Target:    resolvedAddrString(t),
			Transport: t.transport,
			Available: t.transport != "unavailable",
			LastProbe: last,
			History:   histCopy,
			Windows:   computeWindows(histCopy),
		})
	}
	return out
}

// Helpers --------------------------------------------------------------------

func targetsDiffer(existing []*probeTarget, configured []string) bool {
	if len(existing) != filteredLen(configured) {
		return true
	}
	j := 0
	for _, raw := range configured {
		label := strings.TrimSpace(raw)
		if label == "" {
			continue
		}
		if j >= len(existing) {
			return true
		}
		if !strings.EqualFold(existing[j].label, label) {
			return true
		}
		j++
	}
	return false
}

func filteredLen(items []string) int {
	n := 0
	for _, s := range items {
		if strings.TrimSpace(s) != "" {
			n++
		}
	}
	return n
}

// computeWindows rolls up a probe slice into 1s / 10s / 60s stats windows
// relative to now. Windows with zero samples still appear in the output so
// the frontend can render "—" without branching on length.
func computeWindows(history []LatencyProbe) []LatencyStats {
	windows := []int{1, 10, 60}
	out := make([]LatencyStats, 0, len(windows))
	now := time.Now()
	for _, secs := range windows {
		cutoff := now.Add(-time.Duration(secs) * time.Second)
		out = append(out, statsForWindow(history, cutoff, secs))
	}
	return out
}

func statsForWindow(history []LatencyProbe, cutoff time.Time, windowSecs int) LatencyStats {
	stats := LatencyStats{WindowSeconds: windowSecs, MinMs: math.Inf(1)}
	var rtts []float64
	lost := 0
	for _, p := range history {
		if p.Timestamp.Before(cutoff) {
			continue
		}
		stats.Samples++
		if p.Lost {
			lost++
			continue
		}
		rtts = append(rtts, p.RTTMs)
		if p.RTTMs < stats.MinMs {
			stats.MinMs = p.RTTMs
		}
		if p.RTTMs > stats.MaxMs {
			stats.MaxMs = p.RTTMs
		}
	}
	if len(rtts) == 0 {
		stats.MinMs = 0
	} else {
		var sum float64
		for _, v := range rtts {
			sum += v
		}
		stats.AvgMs = sum / float64(len(rtts))
		var sqSum float64
		for _, v := range rtts {
			d := v - stats.AvgMs
			sqSum += d * d
		}
		stats.StddevMs = math.Sqrt(sqSum / float64(len(rtts)))
	}
	if stats.Samples > 0 {
		stats.LossPercent = float64(lost) / float64(stats.Samples) * 100
	}
	return stats
}

// resolvedAddrString formats the target's resolved address for display. Picks
// the IP when available, the hostname otherwise, or a placeholder when the
// target is unavailable (e.g. no gateway on the host).
func resolvedAddrString(t *probeTarget) string {
	if t.ip != nil {
		return t.ip.String()
	}
	if t.host != "" {
		return t.host
	}
	if t.statusErr != "" {
		return t.statusErr
	}
	return ""
}

// icmpIDFromLabel maps a target label to a stable 16-bit identifier so the
// sampler can correlate echo replies back to the originating target even
// when multiple probes share one ICMP socket.
func icmpIDFromLabel(label string) int {
	// Seed with the PID so cross-process ICMP replies don't collide.
	h := os.Getpid() & 0xFFFF
	for i := 0; i < len(label); i++ {
		h = (h*31 + int(label[i])) & 0xFFFF
	}
	if h == 0 {
		h = 1
	}
	return h
}

func errString(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	return err.Error()
}

