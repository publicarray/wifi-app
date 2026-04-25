<script>
    export let roamingMetrics = null;
    export let placementRecommendations = [];
    export let clientStats = null;
    // Networks lets us inspect the connected AP's capabilities (802.11r/k)
    // for the behavior checklist. Optional — when absent, the checklist
    // hides those rows rather than guessing.
    export let networks = [];

    // Recent roaming events live on clientStats.roamingHistory — the backend
    // populates it via cloneClientStatsLocked on every tick, so we just need
    // to read the tail. Oldest-first in the backend; we reverse here so the
    // newest roam is at the top of the list.
    $: recentRoams = clientStats?.roamingHistory
        ? [...clientStats.roamingHistory].reverse().slice(0, 15)
        : [];

    function getMetricsClass(value) {
        if (value === false) return "metric-good";
        if (value === true) return "metric-bad";
        if (value > 0) return "metric-good";
        if (value < 0) return "metric-bad";
        return "metric-neutral";
    }

    // Duration tiers match the plan's guidance + the DurationMs doc on the Go
    // side: <500 ms healthy, 500–2000 ms slow, >=2000 ms bad ("auth issues").
    function durationTier(ms) {
        if (!Number.isFinite(ms) || ms <= 0) return "unknown";
        if (ms < 500) return "good";
        if (ms < 2000) return "slow";
        return "bad";
    }

    function durationClass(ms) {
        const t = durationTier(ms);
        if (t === "good") return "metric-good";
        if (t === "slow") return "metric-neutral";
        if (t === "bad") return "metric-bad";
        return "";
    }

    function formatDuration(ms) {
        if (!Number.isFinite(ms) || ms <= 0) return "—";
        if (ms < 1000) return `${Math.round(ms)} ms`;
        return `${(ms / 1000).toFixed(1)} s`;
    }

    function formatTime(ts) {
        if (!ts) return "";
        const d = new Date(ts);
        return d.toLocaleTimeString();
    }

    function signalDeltaClass(delta) {
        if (delta > 0) return "metric-good";
        if (delta < 0) return "metric-bad";
        return "";
    }

    function signalDelta(event) {
        const d = (event.newSignal || 0) - (event.previousSignal || 0);
        return (d > 0 ? "+" : "") + d + " dBm";
    }

    function shortBssid(b) {
        if (!b) return "—";
        // Keep the last 5 chars (":AA:BB") so adjacent APs in the same
        // vendor space are still visually distinguishable without hogging
        // the row width.
        if (b.length <= 8) return b;
        return "…" + b.slice(-8);
    }

    // ── Behavior checklist derivations ────────────────────────
    // Connected AP — used to read 802.11r / 802.11k capability.
    $: connectedAP = (() => {
        const target = (clientStats?.bssid || "").toLowerCase();
        if (!target) return null;
        for (const n of networks || []) {
            for (const ap of n.accessPoints || []) {
                if (ap?.bssid && ap.bssid.toLowerCase() === target) return ap;
            }
        }
        return null;
    })();

    // Ping-pong: at least one A→B→A pattern within 90 s in recent history.
    $: pingPongDetected = (() => {
        const hist = clientStats?.roamingHistory || [];
        if (hist.length < 2) return false;
        for (let i = 1; i < hist.length; i++) {
            const prev = hist[i - 1];
            const curr = hist[i];
            // Same client returning to the previous BSSID it just left.
            if (
                (prev.previousBssid || "").toLowerCase() ===
                    (curr.newBssid || "").toLowerCase() &&
                (prev.newBssid || "").toLowerCase() ===
                    (curr.previousBssid || "").toLowerCase()
            ) {
                const t1 = new Date(prev.timestamp).getTime();
                const t2 = new Date(curr.timestamp).getTime();
                if (Math.abs(t2 - t1) <= 90_000) return true;
            }
        }
        return false;
    })();

    // Recent good vs poor roams — used by the KPI tiles.
    $: kpi = {
        total: roamingMetrics?.totalRoams || 0,
        good: roamingMetrics?.goodRoams || 0,
        bad: roamingMetrics?.badRoams || 0,
        avgDelta: roamingMetrics?.avgSignalChange ?? 0,
    };
    $: goodPct =
        kpi.total > 0 ? Math.round((kpi.good / kpi.total) * 100) : 0;
    $: badPct = kpi.total > 0 ? Math.round((kpi.bad / kpi.total) * 100) : 0;

    // Behavior checklist: tone is one of "ok" | "warn" | "bad" | "unknown".
    $: behaviorRows = (() => {
        const rows = [];
        rows.push({
            label: "Excessive roaming",
            tone: roamingMetrics?.excessiveRoaming ? "warn" : "ok",
            detail: roamingMetrics?.excessiveRoaming
                ? "More than 10 roams/hr — investigate AP coverage and roaming aggressiveness."
                : "Below 10 roams/hr threshold.",
        });
        rows.push({
            label: "Sticky client",
            tone: roamingMetrics?.stickyClient ? "warn" : "ok",
            detail: roamingMetrics?.stickyClient
                ? "Client lingers on weak APs instead of roaming. Lower the min-RSSI threshold or check 802.11k support."
                : "Client leaves APs near the expected RSSI threshold.",
        });
        rows.push({
            label: "Ping-pong between APs",
            tone: pingPongDetected ? "warn" : "ok",
            detail: pingPongDetected
                ? "A→B→A oscillation observed within 90 s — consider raising the roam threshold or AP transmit power."
                : "No oscillation detected in recent history.",
        });
        rows.push({
            label: "Slow roams (≥ 2 s)",
            tone:
                (roamingMetrics?.slowRoamCount || 0) > 0 ? "warn" : "ok",
            detail:
                (roamingMetrics?.slowRoamCount || 0) > 0
                    ? `${roamingMetrics.slowRoamCount} slow roam${roamingMetrics.slowRoamCount === 1 ? "" : "s"} — likely auth or 802.1X delays.`
                    : "All roams completed in under 2 seconds.",
        });
        if (connectedAP) {
            rows.push({
                label: "Fast Transition (802.11r)",
                tone: connectedAP.fastroaming ? "ok" : "warn",
                detail: connectedAP.fastroaming
                    ? "Connected AP advertises FT — speeds up roams."
                    : "Connected AP does not advertise FT — roams may include full re-auth.",
            });
            rows.push({
                label: "Neighbor reports (802.11k)",
                tone: connectedAP.neighborReport ? "ok" : "warn",
                detail: connectedAP.neighborReport
                    ? "Connected AP advertises a neighbor list."
                    : "Connected AP does not advertise neighbor reports — clients fall back to full scans.",
            });
        }
        return rows;
    })();

    $: anyBehaviorIssue = behaviorRows.some((r) => r.tone !== "ok");
</script>

<div class="roaming-analysis-container">
    {#if !roamingMetrics}
        <div class="no-data">
            <span class="no-data-icon">📊</span>
            <p>No roaming data available</p>
            <p class="hint">
                Connect to a network and wait for roaming events to occur
            </p>
        </div>
    {:else}
        <!-- KPI strip — primary roam tally -->
        <div class="kpi-row primary">
            <div class="kpi-tile">
                <div class="kpi-label">Total roams</div>
                <div class="kpi-value mono">{kpi.total}</div>
                <div class="kpi-sub mono">past 24h</div>
            </div>
            <div class="kpi-tile">
                <div class="kpi-label">Good roams</div>
                <div class="kpi-value mono kpi-ok">{kpi.good}</div>
                <div class="kpi-sub mono">
                    {goodPct}% · improved ≥ 6 dBm
                </div>
            </div>
            <div class="kpi-tile">
                <div class="kpi-label">Poor roams</div>
                <div
                    class="kpi-value mono"
                    class:kpi-warn={kpi.bad > 0}
                >{kpi.bad}</div>
                <div class="kpi-sub mono">{badPct}% · marginal gain</div>
            </div>
            <div class="kpi-tile">
                <div class="kpi-label">Avg Δ signal</div>
                <div
                    class="kpi-value mono"
                    class:kpi-ok={kpi.avgDelta >= 0}
                    class:kpi-bad={kpi.avgDelta < 0}
                >
                    {kpi.avgDelta > 0 ? "+" : ""}{kpi.avgDelta} dBm
                </div>
                <div class="kpi-sub mono">post-roam vs. pre</div>
            </div>
        </div>

        <!-- Secondary KPI row — roam duration aggregates -->
        <div class="kpi-row secondary">
            <div class="kpi-tile">
                <div class="kpi-label">Avg duration</div>
                <div
                    class="kpi-value mono"
                    class:kpi-ok={durationTier(roamingMetrics.avgRoamDurationMs) === "good"}
                    class:kpi-warn={durationTier(roamingMetrics.avgRoamDurationMs) === "slow"}
                    class:kpi-bad={durationTier(roamingMetrics.avgRoamDurationMs) === "bad"}
                >
                    {formatDuration(roamingMetrics.avgRoamDurationMs)}
                </div>
                <div class="kpi-sub mono">
                    capped by scan interval
                </div>
            </div>
            <div class="kpi-tile">
                <div class="kpi-label">Max duration</div>
                <div
                    class="kpi-value mono"
                    class:kpi-ok={durationTier(roamingMetrics.maxRoamDurationMs) === "good"}
                    class:kpi-warn={durationTier(roamingMetrics.maxRoamDurationMs) === "slow"}
                    class:kpi-bad={durationTier(roamingMetrics.maxRoamDurationMs) === "bad"}
                >
                    {formatDuration(roamingMetrics.maxRoamDurationMs)}
                </div>
                <div class="kpi-sub mono">worst observed</div>
            </div>
            <div class="kpi-tile">
                <div class="kpi-label">Slow roams</div>
                <div
                    class="kpi-value mono"
                    class:kpi-warn={(roamingMetrics.slowRoamCount || 0) > 0}
                >
                    {roamingMetrics.slowRoamCount || 0}
                </div>
                <div class="kpi-sub mono">≥ 2 s · auth delay</div>
            </div>
        </div>

        <!-- Behavior checklist -->
        <div class="panel">
            <div class="panel-header">
                <div>
                    <div class="panel-title">Roaming behavior</div>
                    <div class="panel-sub">Compared against RRM guidelines</div>
                </div>
                {#if roamingMetrics.timeSinceLastRoam}
                    <div class="panel-spacer"></div>
                    <span class="chip">
                        Last roam {roamingMetrics.timeSinceLastRoam}
                    </span>
                {/if}
            </div>
            <div class="behavior-list">
                {#each behaviorRows as row}
                    <div class="behavior-row">
                        <div class="behavior-text">
                            <div class="behavior-title">{row.label}</div>
                            <div class="behavior-detail">{row.detail}</div>
                        </div>
                        <span class="chip pill-{row.tone}">
                            {#if row.tone === "ok"}
                                ✓ OK
                            {:else if row.tone === "warn"}
                                ⚠ Review
                            {:else if row.tone === "bad"}
                                ✕ Issue
                            {:else}
                                ? Unknown
                            {/if}
                        </span>
                    </div>
                {/each}
            </div>
        </div>

        {#if recentRoams.length > 0}
            <div class="panel">
                <div class="panel-header">
                    <div>
                        <div class="panel-title">Roam timeline</div>
                        <div class="panel-sub">
                            Most recent · {recentRoams.length} of last
                            {clientStats?.roamingHistory?.length || 0}
                        </div>
                    </div>
                </div>
                <div class="timeline">
                    {#each recentRoams as event}
                        <div class="timeline-row">
                            <span class="t-time mono">
                                {formatTime(event.timestamp)}
                            </span>
                            <div class="t-bssid mono">
                                <span title={event.previousBssid}>
                                    {shortBssid(event.previousBssid)}
                                </span>
                                <span class="t-arrow">→</span>
                                <span
                                    class="t-new"
                                    title={event.newBssid}
                                >
                                    {shortBssid(event.newBssid)}
                                </span>
                            </div>
                            <div class="t-delta mono">
                                <span class="t-pre">{event.previousSignal}</span>
                                <span class="t-arrow">→</span>
                                <span
                                    class="t-post"
                                    class:t-pre-bad={event.newSignal <
                                        event.previousSignal}
                                >{event.newSignal}</span>
                            </div>
                            <span
                                class="mono t-duration"
                                class:t-d-warn={durationTier(event.durationMs) ===
                                    "slow"}
                                class:t-d-bad={durationTier(event.durationMs) ===
                                    "bad"}
                                title={durationTier(event.durationMs) === "bad"
                                    ? "Roam took over 2 s — likely auth delay"
                                    : ""}
                            >
                                {formatDuration(event.durationMs)}
                            </span>
                            <span
                                class="chip pill-{event.newSignal -
                                    event.previousSignal >=
                                6
                                    ? 'ok'
                                    : event.newSignal - event.previousSignal >=
                                        0
                                      ? 'warn'
                                      : 'bad'}"
                            >
                                {event.newSignal - event.previousSignal >= 6
                                    ? "Good"
                                    : event.newSignal -
                                            event.previousSignal >=
                                        0
                                      ? "Marginal"
                                      : "Regression"}
                            </span>
                        </div>
                    {/each}
                </div>
            </div>
        {/if}

        <!-- Advice card -->
        <div
            class="advice-card"
            class:advice-warn={anyBehaviorIssue}
            class:advice-ok={!anyBehaviorIssue}
        >
            <div class="advice-icon-wrap">
                {#if anyBehaviorIssue}
                    ⚠
                {:else}
                    ✓
                {/if}
            </div>
            <div class="advice-body">
                <div class="advice-title">
                    {anyBehaviorIssue
                        ? "Roaming has a few issues to investigate"
                        : "Roaming looks healthy"}
                </div>
                <div class="advice-text">{roamingMetrics.roamingAdvice}</div>
                <div class="advice-caveat">
                    Duration resolution is capped by the scan interval
                    (default 4 s). Lower
                    <code>scan_interval_seconds</code> in Settings for
                    finer-grained timing.
                </div>
            </div>
        </div>
    {/if}

    {#if placementRecommendations && placementRecommendations.length > 0}
        <div class="section">
            <h3>AP Placement Recommendations</h3>
            <div class="recommendations-list">
                {#each placementRecommendations as recommendation}
                    <div class="recommendation-item">
                        <span class="rec-icon">💡</span>
                        <span class="rec-text">{recommendation}</span>
                    </div>
                {/each}
            </div>
        </div>
    {:else if placementRecommendations && placementRecommendations.length === 0}
        <div class="section">
            <h3>AP Placement Recommendations</h3>
            <div class="no-recommendations">
                <span class="no-rec-icon">✓</span>
                <p>No recommendations at this time</p>
                <p class="hint">Your current AP placement appears optimal</p>
            </div>
        </div>
    {/if}
</div>

<style>
    .roaming-analysis-container {
        min-height: 100%;
        overflow: visible;
        display: flex;
        flex-direction: column;
        gap: 12px;
        padding: 16px;
        padding-bottom: 72px;
        font-family: var(--font-ui);
    }

    .mono {
        font-family: var(--font-mono);
        font-variant-numeric: tabular-nums;
    }

    /* ── KPI tiles ──────────────────────────────────────────── */
    .kpi-row {
        display: grid;
        gap: 8px;
    }

    .kpi-row.primary {
        grid-template-columns: repeat(4, 1fr);
    }

    .kpi-row.secondary {
        grid-template-columns: repeat(3, 1fr);
    }

    .kpi-tile {
        padding: 12px 14px;
        border: 1px solid var(--line-1);
        border-radius: 6px;
        background: var(--bg-2);
        display: flex;
        flex-direction: column;
        gap: 4px;
        min-width: 0;
    }

    .kpi-label {
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.1em;
        color: var(--fg-3);
        font-weight: 600;
    }

    .kpi-value {
        font-size: 20px;
        font-weight: 500;
        color: var(--fg-1);
        line-height: 1.1;
        letter-spacing: -0.01em;
    }

    .kpi-value.kpi-ok {
        color: var(--ok);
    }

    .kpi-value.kpi-warn {
        color: var(--warn);
    }

    .kpi-value.kpi-bad {
        color: var(--bad);
    }

    .kpi-sub {
        font-size: 10.5px;
        color: var(--fg-3);
    }

    /* ── Panel (shared with timeline + behavior list) ───────── */
    .panel {
        background: var(--bg-2);
        border: 1px solid var(--line-1);
        border-radius: 8px;
        overflow: hidden;
    }

    .panel-header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 16px;
        border-bottom: 1px solid var(--line-1);
    }

    .panel-title {
        font-size: 12px;
        font-weight: 600;
        color: var(--fg-1);
        letter-spacing: 0.01em;
    }

    .panel-sub {
        font-size: 11px;
        color: var(--fg-3);
        margin-top: 2px;
    }

    .panel-spacer {
        flex: 1;
    }

    /* ── Chips / pills ──────────────────────────────────────── */
    .chip {
        display: inline-flex;
        align-items: center;
        gap: 5px;
        padding: 3px 8px;
        border-radius: 12px;
        font-size: 11px;
        font-weight: 500;
        font-family: var(--font-mono);
        border: 1px solid var(--line-2);
        color: var(--fg-2);
        background: var(--bg-3);
        white-space: nowrap;
    }

    .chip.pill-ok {
        color: var(--ok);
        border-color: var(--ok-line);
        background: var(--ok-bg);
    }

    .chip.pill-warn {
        color: var(--warn);
        border-color: var(--warn-line);
        background: var(--warn-bg);
    }

    .chip.pill-bad {
        color: var(--bad);
        border-color: var(--bad-line);
        background: var(--bad-bg);
    }

    .chip.pill-unknown {
        color: var(--fg-3);
    }

    /* ── Behavior checklist ─────────────────────────────────── */
    .behavior-list {
        display: flex;
        flex-direction: column;
    }

    .behavior-row {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 10px 16px;
        border-bottom: 1px solid var(--line-1);
    }

    .behavior-row:last-child {
        border-bottom: none;
    }

    .behavior-text {
        flex: 1;
        min-width: 0;
    }

    .behavior-title {
        font-size: 12.5px;
        font-weight: 500;
        color: var(--fg-1);
    }

    .behavior-detail {
        font-size: 11px;
        color: var(--fg-3);
        margin-top: 2px;
    }

    /* ── Timeline ───────────────────────────────────────────── */
    .timeline {
        display: flex;
        flex-direction: column;
        gap: 8px;
        padding: 14px;
    }

    .timeline-row {
        display: grid;
        grid-template-columns: 80px 1fr 110px 80px 100px;
        gap: 12px;
        align-items: center;
        padding: 10px 12px;
        background: var(--bg-3);
        border: 1px solid var(--line-1);
        border-radius: 6px;
    }

    .t-time {
        font-size: 11px;
        color: var(--fg-3);
    }

    .t-bssid {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 11px;
        color: var(--fg-2);
        min-width: 0;
    }

    .t-bssid .t-arrow {
        color: var(--fg-3);
    }

    .t-bssid .t-new {
        color: var(--acc-1);
    }

    .t-delta {
        text-align: right;
        font-size: 11px;
    }

    .t-delta .t-pre {
        color: var(--fg-3);
    }

    .t-delta .t-arrow {
        color: var(--fg-3);
        margin: 0 4px;
    }

    .t-delta .t-post {
        color: var(--ok);
    }

    .t-delta .t-post.t-pre-bad {
        color: var(--bad);
    }

    .t-duration {
        text-align: right;
        font-size: 11px;
        color: var(--fg-2);
    }

    .t-duration.t-d-warn {
        color: var(--warn);
    }

    .t-duration.t-d-bad {
        color: var(--bad);
    }

    /* ── Advice card ────────────────────────────────────────── */
    .advice-card {
        display: flex;
        align-items: flex-start;
        gap: 12px;
        padding: 14px;
        border-radius: 8px;
        border: 1px solid var(--line-1);
        background: var(--bg-2);
    }

    .advice-card.advice-ok {
        border-color: var(--ok-line);
        background: linear-gradient(
            180deg,
            rgba(74, 222, 128, 0.04),
            transparent 60%
        );
    }

    .advice-card.advice-warn {
        border-color: var(--warn-line);
        background: linear-gradient(
            180deg,
            rgba(251, 191, 36, 0.05),
            transparent 60%
        );
    }

    .advice-icon-wrap {
        width: 28px;
        height: 28px;
        border-radius: 6px;
        display: flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
        font-size: 13px;
        font-weight: 600;
        background: var(--bg-3);
        color: var(--fg-2);
    }

    .advice-card.advice-ok .advice-icon-wrap {
        background: var(--ok-bg);
        color: var(--ok);
    }

    .advice-card.advice-warn .advice-icon-wrap {
        background: var(--warn-bg);
        color: var(--warn);
    }

    .advice-body {
        flex: 1;
        min-width: 0;
    }

    .advice-title {
        font-size: 13px;
        font-weight: 600;
        color: var(--fg-1);
        margin-bottom: 4px;
    }

    .advice-text {
        font-size: 12px;
        color: var(--fg-2);
        line-height: 1.5;
    }

    .advice-caveat {
        font-size: 11px;
        color: var(--fg-3);
        margin-top: 8px;
        line-height: 1.5;
    }

    .advice-caveat code {
        background: var(--bg-3);
        padding: 1px 5px;
        border-radius: 3px;
        font-size: 10.5px;
        font-family: var(--font-mono);
    }

    /* Legacy classes still referenced by .no-data / recommendations
       block — minimal shim to keep that path looking native. */
    .section {
        background: var(--bg-2);
        border: 1px solid var(--line-1);
        border-radius: 8px;
        padding: 16px;
    }

    .section h3 {
        margin: 0 0 12px 0;
        font-size: 12px;
        font-weight: 600;
        color: var(--fg-1);
        letter-spacing: 0.01em;
        text-transform: uppercase;
        letter-spacing: 0.08em;
    }

    .recommendations-list {
        display: flex;
        flex-direction: column;
        gap: 12px;
    }

    .recommendation-item {
        display: flex;
        align-items: flex-start;
        gap: 10px;
        padding: 12px;
        background: var(--panel);
        border-radius: 4px;
        border-left: 3px solid var(--warning);
    }

    .rec-icon {
        font-size: 16px;
        flex-shrink: 0;
    }

    .rec-text {
        font-size: 14px;
        color: var(--text);
        line-height: 1.4;
    }

    .no-data,
    .no-recommendations {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 40px 20px;
        text-align: center;
        color: var(--muted-2);
    }

    .no-data-icon,
    .no-rec-icon {
        font-size: 48px;
        margin-bottom: 16px;
        opacity: 0.5;
    }

    .no-data p,
    .no-recommendations p {
        margin: 4px 0;
        font-size: 14px;
    }

    .hint {
        font-size: 12px;
        color: var(--muted-2);
        margin-top: 8px;
    }

    @media (max-width: 768px) {
        .roaming-analysis-container {
            padding: 12px;
        }

        .metrics-grid {
            grid-template-columns: 1fr 1fr;
        }

        .kpi-value {
            font-size: 18px;
        }

        .kpi-row.primary {
            grid-template-columns: repeat(2, 1fr);
        }

        .kpi-row.secondary {
            grid-template-columns: repeat(3, 1fr);
        }

        .timeline-row {
            grid-template-columns: 1fr 90px 80px;
            row-gap: 6px;
        }

        .t-time,
        .t-bssid {
            grid-column: 1 / -1;
        }
    }
</style>
