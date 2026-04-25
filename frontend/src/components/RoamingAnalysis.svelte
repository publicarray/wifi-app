<script>
    export let roamingMetrics = null;
    export let placementRecommendations = [];
    export let clientStats = null;

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
        <div class="section">
            <h3>Roaming Summary</h3>
            <div class="metrics-grid">
                <div class="metric-card">
                    <div class="metric-label">Total Roams</div>
                    <div class="metric-value">
                        {roamingMetrics.totalRoams || 0}
                    </div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Good Roams</div>
                    <div class="metric-value metric-good">
                        {roamingMetrics.goodRoams || 0}
                    </div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Bad Roams</div>
                    <div
                        class="metric-value {roamingMetrics.badRoams > 0
                            ? 'metric-bad'
                            : 'metric-good'}"
                    >
                        {roamingMetrics.badRoams || 0}
                    </div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Avg Signal Change</div>
                    <div
                        class="metric-value {getMetricsClass(
                            roamingMetrics.avgSignalChange,
                        )}"
                    >
                        {roamingMetrics.avgSignalChange > 0
                            ? "+"
                            : ""}{roamingMetrics.avgSignalChange} dBm
                    </div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Avg Roam Duration</div>
                    <div
                        class="metric-value {durationClass(
                            roamingMetrics.avgRoamDurationMs,
                        )}"
                    >
                        {formatDuration(roamingMetrics.avgRoamDurationMs)}
                    </div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Max Roam Duration</div>
                    <div
                        class="metric-value {durationClass(
                            roamingMetrics.maxRoamDurationMs,
                        )}"
                    >
                        {formatDuration(roamingMetrics.maxRoamDurationMs)}
                    </div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Slow Roams (≥ 2 s)</div>
                    <div
                        class="metric-value {roamingMetrics.slowRoamCount > 0
                            ? 'metric-bad'
                            : 'metric-good'}"
                    >
                        {roamingMetrics.slowRoamCount || 0}
                    </div>
                </div>
            </div>
            <p class="metric-caveat">
                Duration resolution is capped by the scan interval (default
                4 s). Lower <code>scan_interval_seconds</code> in Settings for
                finer-grained roam timing.
            </p>
        </div>

        <div class="section">
            <h3>Roaming Behavior</h3>
            <div class="behavior-grid">
                <div class="behavior-item">
                    <span class="behavior-label">Excessive Roaming</span>
                    <span
                        class="behavior-value {roamingMetrics.excessiveRoaming
                            ? 'metric-bad'
                            : 'metric-good'}"
                    >
                        {roamingMetrics.excessiveRoaming ? "⚠️ Yes" : "✓ No"}
                    </span>
                </div>
                <div class="behavior-item">
                    <span class="behavior-label">Sticky Client</span>
                    <span
                        class="behavior-value {roamingMetrics.stickyClient
                            ? 'metric-bad'
                            : 'metric-good'}"
                    >
                        {roamingMetrics.stickyClient ? "⚠️ Yes" : "✓ No"}
                    </span>
                </div>
                {#if roamingMetrics.timeSinceLastRoam}
                    <div class="behavior-item">
                        <span class="behavior-label">Time Since Last Roam</span>
                        <span class="behavior-value">
                            {roamingMetrics.timeSinceLastRoam}
                        </span>
                    </div>
                {/if}
            </div>
        </div>

        {#if recentRoams.length > 0}
            <div class="section">
                <h3>Recent Roams</h3>
                <div class="roams-table">
                    <div class="roams-head">
                        <span>Time</span>
                        <span>Previous BSSID</span>
                        <span>New BSSID</span>
                        <span class="numeric">Signal Δ</span>
                        <span class="numeric">Duration</span>
                    </div>
                    {#each recentRoams as event}
                        <div class="roam-row">
                            <span class="time">{formatTime(event.timestamp)}</span>
                            <span class="bssid" title={event.previousBssid}>
                                {shortBssid(event.previousBssid)}
                                <small
                                    >{event.previousSignal} dBm</small
                                >
                            </span>
                            <span class="bssid" title={event.newBssid}>
                                {shortBssid(event.newBssid)}
                                <small>{event.newSignal} dBm</small>
                            </span>
                            <span
                                class="numeric {signalDeltaClass(
                                    event.newSignal - event.previousSignal,
                                )}"
                            >
                                {signalDelta(event)}
                            </span>
                            <span
                                class="numeric duration-badge {durationClass(
                                    event.durationMs,
                                )}"
                                title={durationTier(event.durationMs) === "bad"
                                    ? "Roam took over 2 s — likely auth delay"
                                    : ""}
                            >
                                {formatDuration(event.durationMs)}
                            </span>
                        </div>
                    {/each}
                </div>
            </div>
        {/if}

        <div class="section">
            <h3>Analysis & Advice</h3>
            <div
                class="advice-box {roamingMetrics.excessiveRoaming ||
                roamingMetrics.stickyClient ||
                roamingMetrics.slowRoamCount > 0
                    ? 'advice-warning'
                    : 'advice-good'}"
            >
                <span class="advice-icon">
                    {roamingMetrics.excessiveRoaming ||
                    roamingMetrics.stickyClient ||
                    roamingMetrics.slowRoamCount > 0
                        ? "⚠️"
                        : "✓"}
                </span>
                <span class="advice-text">{roamingMetrics.roamingAdvice}</span>
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
        gap: 20px;
        padding: 20px;
        padding-bottom: 72px;
    }

    .section {
        background: var(--panel-soft);
        border-radius: 6px;
        padding: 16px;
        border: 1px solid var(--border);
    }

    .section h3 {
        margin: 0 0 16px 0;
        font-size: 16px;
        font-weight: 600;
        color: var(--text);
    }

    .metrics-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
        gap: 12px;
    }

    .metric-card {
        background: var(--panel);
        padding: 16px;
        border-radius: 4px;
        display: flex;
        flex-direction: column;
        gap: 4px;
        border: 1px solid var(--border);
    }

    .metric-label {
        font-size: 12px;
        color: var(--muted-2);
        font-weight: 500;
    }

    .metric-value {
        font-size: 20px;
        font-weight: 600;
        color: var(--text);
    }

    .metric-caveat {
        margin: 12px 0 0;
        font-size: 12px;
        color: var(--muted-2);
    }

    .metric-caveat code {
        background: var(--panel);
        padding: 1px 5px;
        border-radius: 3px;
        font-size: 11px;
    }

    .metric-good {
        color: var(--success);
    }

    .metric-bad {
        color: var(--danger);
    }

    .metric-neutral {
        color: var(--warning);
    }

    .behavior-grid {
        display: flex;
        flex-direction: column;
        gap: 12px;
    }

    .behavior-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 12px;
        background: var(--panel);
        border-radius: 4px;
        border: 1px solid var(--border);
    }

    .behavior-label {
        font-size: 14px;
        color: var(--text);
        font-weight: 500;
    }

    .behavior-value {
        font-size: 14px;
        font-weight: 600;
    }

    .roams-table {
        display: flex;
        flex-direction: column;
        gap: 4px;
    }

    .roams-head,
    .roam-row {
        display: grid;
        grid-template-columns: 100px 1fr 1fr 110px 110px;
        gap: 12px;
        align-items: center;
        padding: 8px 12px;
    }

    .roams-head {
        color: var(--muted-2);
        font-size: 11px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        border-bottom: 1px solid var(--border);
        padding-bottom: 10px;
    }

    .roam-row {
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 4px;
        font-size: 13px;
        color: var(--text);
    }

    .roam-row .time {
        font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
        font-size: 12px;
        color: var(--muted);
    }

    .roam-row .bssid {
        font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
        font-size: 12px;
        display: flex;
        flex-direction: column;
    }

    .roam-row .bssid small {
        color: var(--muted-2);
        font-size: 11px;
        font-family: inherit;
    }

    .roam-row .numeric {
        text-align: right;
        font-weight: 600;
    }

    .duration-badge {
        font-variant-numeric: tabular-nums;
    }

    .advice-box {
        display: flex;
        align-items: flex-start;
        gap: 12px;
        padding: 16px;
        border-radius: 4px;
    }

    .advice-box.advice-good {
        background: color-mix(in srgb, var(--success) 15%, transparent);
        border: 1px solid var(--success);
    }

    .advice-box.advice-warning {
        background: color-mix(in srgb, var(--warning) 15%, transparent);
        border: 1px solid var(--warning);
    }

    .advice-icon {
        font-size: 20px;
        flex-shrink: 0;
    }

    .advice-text {
        font-size: 14px;
        color: var(--text);
        line-height: 1.5;
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

        .metric-value {
            font-size: 18px;
        }

        .roams-head,
        .roam-row {
            grid-template-columns: 1fr 1fr 80px 80px;
        }

        .roams-head span:nth-child(1),
        .roam-row .time {
            display: none;
        }
    }
</style>
