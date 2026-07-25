<script>
    // Full UniFi controller diagnostics tab — the AP-point-of-view view a WiFi
    // tech wants when a small card isn't enough room. Renders the same
    // UniFiStatus snapshot App.svelte already hydrates/keeps-live, but expands
    // every device into health (uptime/CPU/mem/load), uplink/mesh topology,
    // per-radio config + live TX-retry, firmware state, and its client roster.
    export let unifiStatus = null;

    $: devices = unifiStatus?.devices || [];
    $: aps = devices.filter((d) => d.isAccessPoint);
    $: infra = devices.filter((d) => !d.isAccessPoint);
    $: hasError = !!unifiStatus?.error;
    $: updatableCount = devices.filter((d) => d.firmwareUpdatable).length;

    // Which device rosters are expanded (by device id).
    let expanded = {};
    function toggle(id) {
        expanded = { ...expanded, [id]: !expanded[id] };
    }

    function stateClass(state) {
        const s = (state || "").toUpperCase();
        if (s === "ONLINE") return "ok";
        if (s === "OFFLINE") return "bad";
        return "warn";
    }

    function healthClass(pct) {
        if (pct == null) return "";
        if (pct >= 90) return "bad";
        if (pct >= 70) return "warn";
        return "ok";
    }

    function retryClass(pct) {
        if (pct == null) return "";
        if (pct >= 20) return "bad";
        if (pct >= 8) return "warn";
        return "ok";
    }

    function fmtUptime(sec) {
        if (!sec || sec <= 0) return "—";
        const d = Math.floor(sec / 86400);
        const h = Math.floor((sec % 86400) / 3600);
        const m = Math.floor((sec % 3600) / 60);
        if (d > 0) return `${d}d ${h}h`;
        if (h > 0) return `${h}h ${m}m`;
        return `${m}m`;
    }

    // Recently-rebooted APs are evidence in a diagnostic report.
    function recentReboot(sec) {
        return sec > 0 && sec < 3600;
    }

    function fmtRate(bps) {
        if (!bps || bps <= 0) return null;
        if (bps >= 1e9) return `${(bps / 1e9).toFixed(1)} Gbps`;
        if (bps >= 1e6) return `${(bps / 1e6).toFixed(0)} Mbps`;
        if (bps >= 1e3) return `${(bps / 1e3).toFixed(0)} Kbps`;
        return `${bps} bps`;
    }

    function fmtPct(v) {
        return v == null ? "—" : `${Math.round(v)}%`;
    }

    // Session duration from the controller's connectedAt timestamp. Per-client
    // RSSI/PHY-rate/channel aren't available from the integration API, so
    // session time is the main live signal we can show.
    function fmtSession(iso) {
        if (!iso) return "—";
        const t = Date.parse(iso);
        if (isNaN(t)) return "—";
        return fmtUptime(Math.floor((Date.now() - t) / 1000));
    }

    function fmtLinkSpeed(mbps) {
        if (!mbps || mbps <= 0) return "";
        if (mbps >= 1000) return `${(mbps / 1000).toFixed(mbps % 1000 ? 1 : 0)}G`;
        return `${mbps}M`;
    }

    function bandLabel(ghz) {
        if (ghz === 2.4) return "2.4";
        if (ghz === 5) return "5";
        if (ghz === 6) return "6";
        if (ghz === 60) return "60";
        return String(ghz);
    }

    function stdLabel(s) {
        // "802.11ax" → "ax"
        return (s || "").replace(/^802\.11/, "");
    }

    function lastUpdatedLabel(ts) {
        if (!ts) return "";
        const d = new Date(ts);
        if (isNaN(d.getTime()) || d.getFullYear() < 2000) return "";
        return d.toLocaleTimeString();
    }
</script>

{#if !unifiStatus || !unifiStatus.configured}
    <div class="empty">
        <div class="empty-title">UniFi controller not configured</div>
        <div class="empty-sub">
            Add a controller URL and API key under
            <strong>Preferences → UniFi controller</strong> to pull per-AP
            firmware, health, uplink/mesh and radio diagnostics here.
        </div>
    </div>
{:else}
    <!-- Controller summary bar -->
    <div class="summary">
        <span class="dot {unifiStatus.connected && !hasError ? 'ok' : hasError ? 'bad' : 'warn'}"></span>
        <div class="sum-main">
            <div class="sum-title">
                {unifiStatus.siteName || "UniFi site"}
                {#if unifiStatus.applicationVersion}
                    <span class="sum-ver">Network {unifiStatus.applicationVersion}</span>
                {/if}
            </div>
            <div class="sum-url mono">{unifiStatus.controllerUrl}</div>
        </div>

        <div class="sum-stats">
            <div class="stat">
                <div class="stat-val">{aps.length}</div>
                <div class="stat-lbl">APs</div>
            </div>
            <div class="stat">
                <div class="stat-val">{unifiStatus.wirelessClients ?? 0}</div>
                <div class="stat-lbl">wireless</div>
            </div>
            <div class="stat">
                <div class="stat-val">{unifiStatus.wiredClients ?? 0}</div>
                <div class="stat-lbl">wired</div>
            </div>
            {#if updatableCount > 0}
                <div class="stat warn-stat">
                    <div class="stat-val">{updatableCount}</div>
                    <div class="stat-lbl">fw update</div>
                </div>
            {/if}
        </div>

        {#if lastUpdatedLabel(unifiStatus.lastUpdated)}
            <span class="sum-updated">updated {lastUpdatedLabel(unifiStatus.lastUpdated)}</span>
        {/if}
    </div>

    {#if hasError}
        <div class="err-bar">{unifiStatus.error}</div>
    {/if}

    {#if devices.length === 0 && !hasError}
        <div class="empty-sub pad">No devices reported by the controller yet.</div>
    {/if}

    <!-- Access points: the diagnostic focus -->
    {#each [...aps, ...infra] as d (d.id)}
        <div class="dev" class:offline={stateClass(d.state) === "bad"}>
            <div class="dev-head">
                <span class="state-pill {stateClass(d.state)}">{(d.state || "?").toLowerCase()}</span>
                <span class="dev-name">{d.name || d.mac}</span>
                <span class="dev-model mono">{d.model}</span>
                {#if !d.isAccessPoint}
                    <span class="tag muted-tag">infra</span>
                {/if}

                <span class="dev-spacer"></span>

                <!-- Uplink topology. The integration API does not report the
                     wired/wireless medium, so we show the parent + uplink port
                     speed only, never a wired/mesh claim. -->
                {#if d.uplinkName || d.uplinkPortSpeedMbps > 0}
                    <span class="tag uplink" title="Uplink topology (parent device and port link speed)">
                        ↑ {d.uplinkName || "?"}{#if d.uplinkPortSpeedMbps > 0} · {fmtLinkSpeed(d.uplinkPortSpeedMbps)} port{/if}
                    </span>
                {/if}

                <!-- Firmware -->
                {#if d.firmwareUpdatable}
                    <span class="tag fw-update" title="Controller reports a newer firmware. Mismatched firmware across APs is a common roaming culprit.">
                        fw update · {d.firmwareVersion || "?"}
                    </span>
                {:else if d.firmwareVersion}
                    <span class="tag fw-ok" title="Firmware version">fw {d.firmwareVersion}</span>
                {/if}
            </div>

            <div class="dev-body">
                <!-- Health metrics -->
                <div class="metrics">
                    <div class="metric">
                        <span class="m-lbl">uptime</span>
                        <span class="m-val" class:warn={recentReboot(d.uptimeSec)}>
                            {fmtUptime(d.uptimeSec)}
                            {#if recentReboot(d.uptimeSec)}<span class="reboot">recent reboot</span>{/if}
                        </span>
                    </div>
                    <div class="metric">
                        <span class="m-lbl">CPU</span>
                        <span class="m-bar"><span class="m-fill {healthClass(d.cpuPct)}" style="width:{Math.min(d.cpuPct ?? 0, 100)}%"></span></span>
                        <span class="m-num">{fmtPct(d.cpuPct)}</span>
                    </div>
                    <div class="metric">
                        <span class="m-lbl">mem</span>
                        <span class="m-bar"><span class="m-fill {healthClass(d.memPct)}" style="width:{Math.min(d.memPct ?? 0, 100)}%"></span></span>
                        <span class="m-num">{fmtPct(d.memPct)}</span>
                    </div>
                    {#if d.loadAvg1 != null}
                        <div class="metric">
                            <span class="m-lbl">load</span>
                            <span class="m-val mono">{d.loadAvg1.toFixed(2)}</span>
                        </div>
                    {/if}
                    {#if fmtRate(d.uplinkTxBps) || fmtRate(d.uplinkRxBps)}
                        <div class="metric">
                            <span class="m-lbl">uplink</span>
                            <span class="m-val mono">↑{fmtRate(d.uplinkTxBps) || "0"} ↓{fmtRate(d.uplinkRxBps) || "0"}</span>
                        </div>
                    {/if}
                    <div class="metric">
                        <span class="m-lbl">IP</span>
                        <span class="m-val mono">{d.ip || "—"}</span>
                    </div>
                </div>

                <!-- Radios -->
                {#if d.radios && d.radios.length > 0}
                    <div class="radios">
                        {#each d.radios as r}
                            <div class="radio">
                                <span class="band band-{bandLabel(r.band)}">{bandLabel(r.band)}G</span>
                                <span class="r-detail mono">ch {r.channel || "?"}{#if r.widthMhz}/{r.widthMhz}<span class="unit">MHz</span>{/if}</span>
                                {#if r.standard}<span class="r-std">{stdLabel(r.standard)}</span>{/if}
                                {#if r.txRetriesPct != null}
                                    <span class="r-retry {retryClass(r.txRetriesPct)}" title="TX retry rate — high values indicate congestion or weak clients">
                                        {r.txRetriesPct.toFixed(1)}% retry
                                    </span>
                                {/if}
                            </div>
                        {/each}
                    </div>
                {/if}

                <!-- Client roster -->
                {#if d.clientCount > 0}
                    <button class="roster-toggle" on:click={() => toggle(d.id)}>
                        <span class="chev" class:open={expanded[d.id]}>▸</span>
                        {d.clientCount} client{d.clientCount === 1 ? "" : "s"}
                        {#if d.clients && d.clients.length < d.clientCount}
                            <span class="cap-note">(showing {d.clients.length})</span>
                        {/if}
                    </button>
                    {#if expanded[d.id] && d.clients && d.clients.length > 0}
                        <table class="roster">
                            <thead>
                                <tr>
                                    <th>Client</th>
                                    <th>Vendor</th>
                                    <th>MAC</th>
                                    <th>IP</th>
                                    <th>Session</th>
                                </tr>
                            </thead>
                            <tbody>
                                {#each d.clients as c (c.mac)}
                                    <tr>
                                        <td>
                                            {c.name || "—"}
                                            {#if c.guest}<span class="cflag guest" title="Guest-network client">guest</span>{/if}
                                        </td>
                                        <td class="muted-cell">
                                            {c.vendor || "—"}
                                            {#if c.randomized}<span class="cflag rnd" title="Locally-administered (randomized/private) MAC — rotates, won't map to a real vendor">rnd</span>{/if}
                                        </td>
                                        <td class="mono muted-cell">{c.mac}</td>
                                        <td class="mono muted-cell">{c.ip || "—"}</td>
                                        <td class="mono muted-cell">{fmtSession(c.connectedAt)}</td>
                                    </tr>
                                {/each}
                            </tbody>
                        </table>
                    {/if}
                {:else}
                    <div class="no-clients">no wireless clients</div>
                {/if}
            </div>
        </div>
    {/each}
{/if}

<style>
    /* ── Summary bar ─────────────────────────────────────────── */
    .summary {
        display: flex;
        align-items: center;
        gap: 16px;
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 10px;
        padding: 12px 16px;
        margin-bottom: 14px;
        flex-wrap: wrap;
    }
    .sum-main { min-width: 0; }
    .sum-title {
        font-weight: 600;
        color: var(--text);
        font-size: 14px;
        display: flex;
        align-items: baseline;
        gap: 8px;
    }
    .sum-ver { font-size: 11px; color: var(--muted); font-weight: 400; }
    .sum-url { font-size: 11px; color: var(--muted); }
    .sum-stats { display: flex; gap: 18px; margin-left: auto; }
    .stat { text-align: center; }
    .stat-val { font-size: 18px; font-weight: 600; color: var(--text); font-variant-numeric: tabular-nums; }
    .stat-lbl { font-size: 10px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.04em; }
    .warn-stat .stat-val { color: var(--warn); }
    .sum-updated { font-size: 11px; color: var(--muted); opacity: 0.75; }

    .err-bar {
        background: rgba(248, 113, 113, 0.1);
        border: 1px solid rgba(248, 113, 113, 0.3);
        color: var(--bad);
        border-radius: 8px;
        padding: 8px 12px;
        font-size: 12px;
        margin-bottom: 14px;
    }

    /* ── Device card ─────────────────────────────────────────── */
    .dev {
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 10px;
        margin-bottom: 10px;
        overflow: hidden;
    }
    .dev.offline { opacity: 0.6; }

    .dev-head {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 10px 14px;
        border-bottom: 1px solid var(--border);
        flex-wrap: wrap;
    }
    .dev-name { font-weight: 600; color: var(--text); font-size: 13px; }
    .dev-model { font-size: 11px; color: var(--muted); }
    .dev-spacer { flex: 1; }

    .dev-body { padding: 12px 14px; }

    /* ── Health metrics row ──────────────────────────────────── */
    .metrics {
        display: flex;
        flex-wrap: wrap;
        gap: 10px 22px;
        align-items: center;
    }
    .metric { display: flex; align-items: center; gap: 7px; }
    .m-lbl { font-size: 10px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.04em; }
    .m-val { font-size: 12px; color: var(--text); }
    .m-val.warn { color: var(--warn); }
    .m-num { font-size: 11px; color: var(--text); font-variant-numeric: tabular-nums; min-width: 32px; }
    .m-bar {
        width: 56px; height: 6px;
        background: var(--bg-4);
        border-radius: 3px;
        overflow: hidden;
        display: inline-block;
    }
    .m-fill { display: block; height: 100%; border-radius: 3px; background: var(--acc-1); }
    .m-fill.ok { background: var(--ok); }
    .m-fill.warn { background: var(--warn); }
    .m-fill.bad { background: var(--bad); }
    .reboot {
        font-size: 10px;
        color: var(--warn);
        background: rgba(251, 191, 36, 0.12);
        padding: 0 5px;
        border-radius: 4px;
        margin-left: 4px;
    }

    /* ── Radios ──────────────────────────────────────────────── */
    .radios { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 12px; }
    .radio {
        display: flex;
        align-items: center;
        gap: 8px;
        background: var(--bg-3);
        border: 1px solid var(--border);
        border-radius: 8px;
        padding: 5px 10px;
        font-size: 12px;
    }
    .band {
        font-weight: 700;
        font-size: 11px;
        padding: 1px 6px;
        border-radius: 5px;
        color: #0a0d12;
    }
    .band-2\.4 { background: #7dd3fc; }
    .band-5 { background: #5ce1e6; }
    .band-6 { background: #c4b5fd; }
    .band-60 { background: #fca5a5; }
    .r-detail { color: var(--text); }
    .r-detail .unit { color: var(--muted); font-size: 10px; }
    .r-std {
        font-size: 10px;
        color: var(--muted);
        text-transform: uppercase;
        letter-spacing: 0.03em;
    }
    .r-retry { font-size: 11px; }
    .r-retry.ok { color: var(--ok); }
    .r-retry.warn { color: var(--warn); }
    .r-retry.bad { color: var(--bad); }

    /* ── Tags ────────────────────────────────────────────────── */
    .tag {
        font-size: 11px;
        padding: 2px 8px;
        border-radius: 6px;
        white-space: nowrap;
    }
    .muted-tag { color: var(--muted); background: var(--bg-3); }
    .uplink { color: var(--muted); background: var(--bg-3); }
    .fw-update { color: var(--warn); background: rgba(251, 191, 36, 0.12); }
    .fw-ok { color: var(--muted); background: var(--bg-3); }

    .state-pill {
        display: inline-block;
        padding: 1px 8px;
        border-radius: 999px;
        font-size: 11px;
    }
    .state-pill.ok { color: var(--ok); background: rgba(74, 222, 128, 0.12); }
    .state-pill.warn { color: var(--warn); background: rgba(251, 191, 36, 0.12); }
    .state-pill.bad { color: var(--bad); background: rgba(248, 113, 113, 0.12); }

    /* ── Client roster ───────────────────────────────────────── */
    .roster-toggle {
        margin-top: 12px;
        background: none;
        border: none;
        color: var(--acc-1);
        cursor: pointer;
        font-size: 12px;
        padding: 2px 0;
        display: flex;
        align-items: center;
        gap: 6px;
    }
    .chev { transition: transform 0.12s; display: inline-block; }
    .chev.open { transform: rotate(90deg); }
    .cap-note { color: var(--muted); }
    .no-clients { margin-top: 12px; font-size: 11px; color: var(--muted); opacity: 0.7; }

    .roster {
        margin-top: 8px;
        width: 100%;
        border-collapse: collapse;
        font-size: 12px;
    }
    .roster th {
        text-align: left;
        padding: 3px 12px 5px 0;
        color: var(--muted);
        font-weight: 500;
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.04em;
        border-bottom: 1px solid var(--border);
    }
    .roster td {
        padding: 3px 12px 3px 0;
        color: var(--text);
        border-bottom: 1px solid var(--border);
    }
    .roster tbody tr:last-child td { border-bottom: none; }
    .muted-cell { color: var(--muted); }

    .cflag {
        display: inline-block;
        margin-left: 5px;
        padding: 0 5px;
        border-radius: 4px;
        font-size: 9px;
        text-transform: uppercase;
        letter-spacing: 0.03em;
        vertical-align: middle;
    }
    .cflag.guest { color: var(--acc-1); background: var(--acc-1-bg); }
    .cflag.rnd { color: var(--muted); background: var(--bg-4); }

    /* ── Misc ────────────────────────────────────────────────── */
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    .dot { width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0; }
    .dot.ok { background: var(--ok); }
    .dot.warn { background: var(--warn); }
    .dot.bad { background: var(--bad); }

    .empty { text-align: center; padding: 60px 20px; }
    .empty-title { font-size: 15px; color: var(--text); font-weight: 600; margin-bottom: 8px; }
    .empty-sub { font-size: 13px; color: var(--muted); max-width: 440px; margin: 0 auto; line-height: 1.5; }
    .empty-sub.pad { padding: 20px; }
</style>
