<script>
    /** @typedef {import('../../wailsjs/go/models').main.ClientStats} ClientStats */
    /** @type {ClientStats | null} */
    export let clientStats = null;

    function getSignalClass(signal) {
        if (signal > -60) return "signal-good";
        if (signal > -75) return "signal-medium";
        return "signal-poor";
    }

    function getSignalQuality(signal) {
        if (signal > -60) return { text: "Excellent", color: "var(--success)" };
        if (signal > -70) return { text: "Good", color: "var(--success)" };
        if (signal > -80) return { text: "Fair", color: "var(--warning)" };
        return { text: "Poor", color: "var(--danger)" };
    }

    function formatBytes(bytes) {
        if (bytes === 0) return "0 B";
        if (!isNumber(bytes)) return "N/A";
        const k = 1024;
        const sizes = ["B", "KB", "MB", "GB"];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
    }

    function formatDuration(seconds) {
        if (!isNumber(seconds)) return "N/A";
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        const secs = seconds % 60;

        if (hours > 0) {
            return `${hours}h ${minutes}m ${secs}s`;
        } else if (minutes > 0) {
            return `${minutes}m ${secs}s`;
        }
        return `${secs}s`;
    }

    function isNumber(value) {
        return typeof value === "number" && !Number.isNaN(value);
    }

    function formatDbm(value) {
        return isNumber(value) ? `${value} dBm` : "N/A";
    }

    function formatMbps(value) {
        return isNumber(value) ? `${value.toFixed(1)} Mbps` : "N/A";
    }

    function formatFrequency(value) {
        return isNumber(value) ? `${(value / 1000).toFixed(3)} GHz` : "N/A";
    }

    function getRetryRateClass(retryRate) {
        if (retryRate < 5) return "rate-good";
        if (retryRate < 10) return "rate-medium";
        return "rate-poor";
    }

    // ── KPI tiles at the top of the screen ────────────────────
    // Derived from clientStats so the strip stays in sync with the live
    // backend feed. Tones map onto the design's {ok, warn, bad} palette.
    function signalTone(dBm) {
        if (!isNumber(dBm)) return "muted";
        if (dBm >= -60) return "ok";
        if (dBm >= -72) return "warn";
        return "bad";
    }

    function snrTone(snr) {
        if (!isNumber(snr) || snr <= 0) return "muted";
        if (snr >= 25) return "ok";
        if (snr >= 15) return "warn";
        return "bad";
    }

    function retryTone(retry) {
        if (!isNumber(retry)) return "muted";
        if (retry < 5) return "ok";
        if (retry < 10) return "warn";
        return "bad";
    }

    $: kpiSignal = clientStats?.signal;
    $: kpiSNR = clientStats?.snr;
    $: kpiTx = clientStats?.txBitrate;
    $: kpiRx = clientStats?.rxBitrate;
    $: kpiUptime = clientStats?.connectedTime;
    $: kpiRetry = clientStats?.retryRate;
</script>

<div class="client-stats-panel">
    {#if clientStats && clientStats.connected}
        <!-- KPI strip — primary live metrics -->
        <div class="kpi-strip">
            <div class="kpi-tile">
                <div class="kpi-label">Signal</div>
                <div class="kpi-value mono kpi-{signalTone(kpiSignal)}">
                    {isNumber(kpiSignal) ? `${kpiSignal} dBm` : "—"}
                </div>
                <div class="kpi-sub mono">
                    {isNumber(kpiSignal)
                        ? getSignalQuality(kpiSignal).text
                        : "—"}
                </div>
            </div>
            <div class="kpi-tile">
                <div class="kpi-label">SNR</div>
                <div class="kpi-value mono kpi-{snrTone(kpiSNR)}">
                    {isNumber(kpiSNR) && kpiSNR > 0 ? `${kpiSNR} dB` : "—"}
                </div>
                <div class="kpi-sub mono">
                    {isNumber(clientStats?.noise) && clientStats.noise < 0
                        ? `noise ${clientStats.noise} dBm`
                        : "noise n/a"}
                </div>
            </div>
            <div class="kpi-tile">
                <div class="kpi-label">TX rate</div>
                <div class="kpi-value mono kpi-accent">
                    {isNumber(kpiTx) ? kpiTx.toFixed(1) : "—"}
                </div>
                <div class="kpi-sub mono">Mbps</div>
            </div>
            <div class="kpi-tile">
                <div class="kpi-label">RX rate</div>
                <div class="kpi-value mono kpi-ok">
                    {isNumber(kpiRx) ? kpiRx.toFixed(1) : "—"}
                </div>
                <div class="kpi-sub mono">Mbps</div>
            </div>
            <div class="kpi-tile">
                <div class="kpi-label">Retries</div>
                <div class="kpi-value mono kpi-{retryTone(kpiRetry)}">
                    {isNumber(kpiRetry) ? `${kpiRetry.toFixed(1)}%` : "—"}
                </div>
                <div class="kpi-sub mono">last 60s</div>
            </div>
            <div class="kpi-tile">
                <div class="kpi-label">Uptime</div>
                <div class="kpi-value mono">
                    {isNumber(kpiUptime) ? formatDuration(kpiUptime) : "—"}
                </div>
                <div class="kpi-sub mono">since associate</div>
            </div>
        </div>
    {/if}

    {#if clientStats && clientStats.connected}
        <div class="section">
            <h4>Connection</h4>
            <div class="info-grid">
                <div class="info-item">
                    <span
                        class="label"
                        title="Service Set Identifier (Network Name)">SSID</span
                    >
                    <span class="value ssid"
                        >{clientStats.ssid || "Unknown"}</span
                    >
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Basic Service Set Identifier (MAC Address)"
                        >BSSID</span
                    >
                    <span class="value bssid"
                        >{clientStats.bssid || "Unknown"}</span
                    >
                </div>
                <div class="info-item">
                    <span class="label" title="Network Interface Name"
                        >Interface</span
                    >
                    <span class="value">{clientStats.interface || "N/A"}</span>
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Time since connection established">Duration</span
                    >
                    <span class="value"
                        >{formatDuration(clientStats.connectedTime)}</span
                    >
                </div>
            </div>
        </div>

        <div class="section">
            <h4>Signal Quality</h4>
            <div class="info-grid">
                <div class="info-item full-width">
                    <span class="label" title="Current signal strength in dBm"
                        >Current Signal</span
                    >
                    {#if isNumber(clientStats.signal)}
                        <span
                            class="value {getSignalClass(clientStats.signal)}"
                        >
                            {formatDbm(clientStats.signal)}
                        </span>
                        <span
                            class="quality-badge"
                            style="background: {getSignalQuality(
                                clientStats.signal,
                            ).color}"
                            title="Signal quality rating"
                        >
                            {getSignalQuality(clientStats.signal).text}
                        </span>
                    {:else}
                        <span class="value value-na">N/A</span>
                    {/if}
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Average signal strength over time"
                        >Average Signal</span
                    >
                    {#if isNumber(clientStats.signalAvg) || isNumber(clientStats.signal)}
                        <span
                            class="value {getSignalClass(
                                clientStats.signalAvg ?? clientStats.signal,
                            )}"
                        >
                            {formatDbm(
                                clientStats.signalAvg ?? clientStats.signal,
                            )}
                        </span>
                    {:else}
                        <span class="value value-na">N/A</span>
                    {/if}
                </div>
                <div class="info-item full-width">
                    <span
                        class="label"
                        title="Signal-to-Noise Ratio. Higher is better."
                        >SNR</span
                    >
                    {#if isNumber(clientStats.snr) && clientStats.snr > 0}
                        <span class="value">{clientStats.snr} dB</span>
                        <div class="snr-bar">
                            <div
                                class="snr-fill"
                                style="width: {Math.min(
                                    Math.max(clientStats.snr, 0),
                                    50,
                                ) * 2}%"
                            ></div>
                        </div>
                    {:else}
                        <span class="value value-na">N/A</span>
                    {/if}
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Signal strength of last acknowledgement packet"
                        >Last ACK Signal</span
                    >
                    {#if isNumber(clientStats.lastAckSignal)}
                        <span
                            class="value {getSignalClass(
                                clientStats.lastAckSignal,
                            )}"
                        >
                            {formatDbm(clientStats.lastAckSignal)}
                        </span>
                    {:else}
                        <span class="value value-na">N/A</span>
                    {/if}
                </div>
            </div>
        </div>

        <div class="section">
            <h4>Data Rates</h4>
            <div class="info-grid">
                <div class="info-item">
                    <span class="label" title="Current transmission rate"
                        >TX Rate</span
                    >
                    <span
                        class="value {isNumber(clientStats.txBitrate)
                            ? 'rate'
                            : 'value-na'}"
                        >{formatMbps(clientStats.txBitrate)}</span
                    >
                </div>
                <div class="info-item">
                    <span class="label" title="Current reception rate"
                        >RX Rate</span
                    >
                    <span
                        class="value {isNumber(clientStats.rxBitrate)
                            ? 'rate'
                            : 'value-na'}"
                        >{formatMbps(clientStats.rxBitrate)}</span
                    >
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Current operating channel and width"
                        >Channel</span
                    >
                    <span class="value">
                        {#if isNumber(clientStats.channel) && isNumber(clientStats.channelWidth)}
                            {clientStats.channel} ({clientStats.channelWidth}MHz)
                        {:else}
                            N/A
                        {/if}
                    </span>
                </div>
                <div class="info-item">
                    <span class="label" title="Current operating frequency"
                        >Frequency</span
                    >
                    <span class="value"
                        >{formatFrequency(clientStats.frequency)}</span
                    >
                </div>
            </div>
        </div>

        <div class="section">
            <h4>Traffic Statistics</h4>
            <div class="info-grid">
                <div class="info-item">
                    <span class="label" title="Total bytes transmitted"
                        >TX Bytes</span
                    >
                    <span class="value">{formatBytes(clientStats.txBytes)}</span
                    >
                </div>
                <div class="info-item">
                    <span class="label" title="Total bytes received"
                        >RX Bytes</span
                    >
                    <span class="value">{formatBytes(clientStats.rxBytes)}</span
                    >
                </div>
                <div class="info-item">
                    <span class="label" title="Total packets transmitted"
                        >TX Packets</span
                    >
                    <span class="value">
                        {#if isNumber(clientStats.txPackets)}
                            {clientStats.txPackets.toLocaleString()}
                        {:else}
                            N/A
                        {/if}
                    </span>
                </div>
                <div class="info-item">
                    <span class="label" title="Total packets received"
                        >RX Packets</span
                    >
                    <span class="value">
                        {#if isNumber(clientStats.rxPackets)}
                            {clientStats.rxPackets.toLocaleString()}
                        {:else}
                            N/A
                        {/if}
                    </span>
                </div>
            </div>
        </div>

        <div class="section">
            <h4>Error Statistics</h4>
            <div class="info-grid">
                <div class="info-item full-width">
                    <span
                        class="label"
                        title="Percentage of packets requiring retransmission"
                        >Retry Rate</span
                    >
                    {#if isNumber(clientStats.retryRate)}
                        <span
                            class="value {getRetryRateClass(
                                clientStats.retryRate,
                            )}"
                        >
                            {clientStats.retryRate.toFixed(1)}%
                        </span>
                        <div class="retry-bar">
                            <div
                                class="retry-fill {getRetryRateClass(
                                    clientStats.retryRate,
                                )}"
                                style="width: {Math.min(
                                    clientStats.retryRate,
                                    100,
                                )}%"
                            ></div>
                        </div>
                    {:else}
                        <span class="value value-na">N/A</span>
                    {/if}
                </div>
                <div class="info-item">
                    <span class="label" title="Number of packets retransmitted"
                        >TX Retries</span
                    >
                    <span class="value">
                        {#if isNumber(clientStats.txRetries)}
                            {clientStats.txRetries.toLocaleString()}
                        {:else}
                            N/A
                        {/if}
                    </span>
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Number of packets failed to transmit"
                        >TX Failed</span
                    >
                    <span class="value">
                        {#if isNumber(clientStats.txFailed)}
                            {clientStats.txFailed.toLocaleString()}
                        {:else}
                            N/A
                        {/if}
                    </span>
                </div>
            </div>
        </div>

        {#if clientStats.roamingHistory && clientStats.roamingHistory.length > 0}
            <div class="section">
                <h4>Roaming History</h4>
                <div class="roaming-list">
                    {#each clientStats.roamingHistory
                        .slice()
                        .reverse() as event}
                        <div class="roaming-event">
                            <div class="roaming-time">
                                {new Date(event.timestamp).toLocaleTimeString()}
                            </div>
                            <div class="roaming-details">
                                <div class="roaming-path">
                                    <span class="bssid-from"
                                        >{event.previousBssid.slice(-6)}</span
                                    >
                                    <span class="arrow">→</span>
                                    <span class="bssid-to"
                                        >{event.newBssid.slice(-6)}</span
                                    >
                                </div>
                                <div class="roaming-signals">
                                    <span class="signal-change">
                                        {event.previousSignal} dBm → {event.newSignal}
                                        dBm
                                    </span>
                                    <span class="channel-change">
                                        Ch {event.previousChannel} → {event.newChannel}
                                    </span>
                                </div>
                            </div>
                        </div>
                    {/each}
                </div>
            </div>
        {/if}

        {#if clientStats.signalHistory && clientStats.signalHistory.length > 0}
            <div class="section">
                <h4>Signal History</h4>
                <div class="history-stats">
                    <div class="history-item">
                        <span
                            class="label"
                            title="Number of signal samples collected"
                            >Data Points</span
                        >
                        <span class="value"
                            >{clientStats.signalHistory.length}</span
                        >
                    </div>
                    <div class="history-item">
                        <span
                            class="label"
                            title="Count of recorded roam events"
                            >Roaming Events</span
                        >
                        <span class="value"
                            >{(clientStats.roamingHistory || []).length}</span
                        >
                    </div>
                </div>
            </div>
        {/if}
    {:else}
        <div class="not-connected">
            <div class="not-connected-icon">📡</div>
            <p>Not connected to any WiFi network</p>
            <p class="hint">
                Start scanning and connect to see detailed statistics
            </p>
        </div>
    {/if}
</div>

<style>
    .client-stats-panel {
        height: 100%;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 12px;
        padding: 16px;
        font-family: var(--font-ui);
    }

    .mono {
        font-family: var(--font-mono);
        font-variant-numeric: tabular-nums;
    }

    /* ── KPI strip ───────────────────────────────────────────── */
    .kpi-strip {
        display: grid;
        grid-template-columns: repeat(6, 1fr);
        gap: 8px;
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
        font-size: 18px;
        font-weight: 500;
        color: var(--fg-1);
        line-height: 1.1;
        letter-spacing: -0.01em;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
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

    .kpi-value.kpi-accent {
        color: var(--acc-1);
    }

    .kpi-value.kpi-muted {
        color: var(--fg-2);
    }

    .kpi-sub {
        font-size: 10.5px;
        color: var(--fg-3);
    }

    /* ── Section panels ──────────────────────────────────────── */
    .section {
        padding: 14px 16px;
        border: 1px solid var(--line-1);
        border-radius: 8px;
        background: var(--bg-2);
    }

    .section h4 {
        margin: 0 0 12px 0;
        font-size: 10px;
        font-weight: 600;
        color: var(--fg-3);
        text-transform: uppercase;
        letter-spacing: 0.12em;
    }

    .info-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 10px 16px;
    }

    .info-item {
        display: flex;
        flex-direction: column;
        gap: 3px;
        padding: 5px 0;
        border-bottom: 1px dashed var(--line-1);
    }

    .info-item:last-child {
        border-bottom: none;
    }

    .info-item.full-width {
        grid-column: 1 / -1;
    }

    .label {
        font-size: 10.5px;
        color: var(--fg-3);
        font-weight: 500;
        text-transform: uppercase;
        letter-spacing: 0.06em;
    }

    .value {
        font-size: 12.5px;
        color: var(--fg-1);
        font-weight: 500;
        font-family: var(--font-mono);
        font-variant-numeric: tabular-nums;
    }

    .value.ssid {
        font-weight: 600;
        color: var(--accent-2);
    }

    .value.bssid {
        font-family: monospace;
        font-size: 12px;
        color: var(--muted);
    }

    .value.rate {
        font-weight: 600;
        color: var(--success);
    }

    .value.value-na {
        color: var(--muted-2);
    }

    /* Signal quality indicators */
    .signal-good {
        color: var(--success);
        font-weight: 600;
    }

    .signal-medium {
        color: var(--warning);
        font-weight: 600;
    }

    .signal-poor {
        color: var(--danger);
        font-weight: 600;
    }

    .quality-badge {
        display: inline-block;
        padding: 2px 8px;
        border-radius: 3px;
        font-size: 11px;
        font-weight: 600;
        color: white;
        margin-left: 8px;
    }

    /* SNR Bar */
    .snr-bar {
        width: 100%;
        height: 4px;
        background: var(--border);
        border-radius: 2px;
        margin-top: 4px;
        overflow: hidden;
    }

    .snr-fill {
        height: 100%;
        background: linear-gradient(
            90deg,
            var(--danger),
            var(--warning),
            var(--success)
        );
        transition: width 0.3s ease;
    }

    /* Retry Rate */
    .rate-good {
        color: var(--success);
    }

    .rate-medium {
        color: var(--warning);
    }

    .rate-poor {
        color: var(--danger);
    }

    .retry-bar {
        width: 100%;
        height: 4px;
        background: var(--border);
        border-radius: 2px;
        margin-top: 4px;
        overflow: hidden;
    }

    .retry-fill {
        height: 100%;
        transition: width 0.3s ease;
    }

    .retry-fill.rate-good {
        background: var(--success);
    }

    .retry-fill.rate-medium {
        background: var(--warning);
    }

    .retry-fill.rate-poor {
        background: var(--danger);
    }

    /* Roaming History */
    .roaming-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .roaming-event {
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 4px;
        padding: 12px;
    }

    .roaming-time {
        font-size: 12px;
        color: var(--muted-2);
        margin-bottom: 6px;
    }

    .roaming-details {
        display: flex;
        flex-direction: column;
        gap: 4px;
    }

    .roaming-path {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 13px;
    }

    .bssid-from,
    .bssid-to {
        font-family: monospace;
        font-weight: 500;
    }

    .bssid-from {
        color: var(--warning);
    }

    .bssid-to {
        color: var(--success);
    }

    .arrow {
        color: var(--muted-2);
    }

    .roaming-signals {
        display: flex;
        gap: 12px;
        font-size: 12px;
        color: var(--muted);
    }

    .signal-change {
        color: var(--accent-2);
    }

    .channel-change {
        color: var(--success);
    }

    /* History Stats */
    .history-stats {
        display: flex;
        gap: 16px;
    }

    .history-item {
        display: flex;
        flex-direction: column;
        gap: 2px;
    }

    .history-item .value {
        font-size: 16px;
        font-weight: 600;
        color: var(--accent-strong);
    }

    /* Not Connected State */
    .not-connected {
        flex: 1;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 40px 20px;
        text-align: center;
        color: var(--muted-2);
    }

    .not-connected-icon {
        font-size: 48px;
        margin-bottom: 16px;
        opacity: 0.5;
    }

    .not-connected p {
        margin: 4px 0;
        font-size: 14px;
    }

    .not-connected .hint {
        font-size: 12px;
        color: var(--muted-2);
        margin-top: 8px;
    }

    /* Responsive adjustments */
    @media (max-width: 1200px) {
        .info-grid {
            grid-template-columns: 1fr;
        }

        .section {
            padding: 12px;
        }
    }
</style>
