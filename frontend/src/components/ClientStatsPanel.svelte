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
        const k = 1024;
        const sizes = ["B", "KB", "MB", "GB"];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
    }

    function formatDuration(seconds) {
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

    function getRetryRateClass(retryRate) {
        if (retryRate < 5) return "rate-good";
        if (retryRate < 10) return "rate-medium";
        return "rate-poor";
    }
</script>

<div class="client-stats-panel">
    <div class="panel-header">
        <h3>Client Connection Stats</h3>
    </div>

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
                    <span class="value"
                        >{clientStats.interface || "Unknown"}</span
                    >
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Time since connection established">Duration</span
                    >
                    <span class="value"
                        >{formatDuration(clientStats.connectedTime || 0)}</span
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
                    <span class="value {getSignalClass(clientStats.signal)}">
                        {clientStats.signal} dBm
                    </span>
                    <span
                        class="quality-badge"
                        style="background: {getSignalQuality(clientStats.signal)
                            .color}"
                        title="Signal quality rating"
                    >
                        {getSignalQuality(clientStats.signal).text}
                    </span>
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Average signal strength over time"
                        >Average Signal</span
                    >
                    <span
                        class="value {getSignalClass(
                            clientStats.signalAvg || clientStats.signal,
                        )}"
                    >
                        {clientStats.signalAvg || clientStats.signal} dBm
                    </span>
                </div>
                <div class="info-item">
                    <span class="label" title="Background noise level in dBm"
                        >Noise</span
                    >
                    <span class="value">{clientStats.noise} dBm</span>
                </div>
                <div class="info-item full-width">
                    <span
                        class="label"
                        title="Signal-to-Noise Ratio. Higher is better."
                        >SNR</span
                    >
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
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Signal strength of last acknowledgement packet"
                        >Last ACK Signal</span
                    >
                    <span
                        class="value {getSignalClass(
                            clientStats.lastAckSignal,
                        )}"
                    >
                        {clientStats.lastAckSignal} dBm
                    </span>
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
                    <span class="value rate"
                        >{clientStats.txBitrate.toFixed(1)} Mbps</span
                    >
                </div>
                <div class="info-item">
                    <span class="label" title="Current reception rate"
                        >RX Rate</span
                    >
                    <span class="value rate"
                        >{clientStats.rxBitrate.toFixed(1)} Mbps</span
                    >
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Current operating channel and width"
                        >Channel</span
                    >
                    <span class="value"
                        >{clientStats.channel} ({clientStats.channelWidth}MHz)</span
                    >
                </div>
                <div class="info-item">
                    <span class="label" title="Current operating frequency"
                        >Frequency</span
                    >
                    <span class="value"
                        >{(clientStats.frequency / 1000).toFixed(3)} GHz</span
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
                    <span class="value"
                        >{clientStats.txPackets.toLocaleString()}</span
                    >
                </div>
                <div class="info-item">
                    <span class="label" title="Total packets received"
                        >RX Packets</span
                    >
                    <span class="value"
                        >{clientStats.rxPackets.toLocaleString()}</span
                    >
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
                    <span
                        class="value {getRetryRateClass(clientStats.retryRate)}"
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
                </div>
                <div class="info-item">
                    <span class="label" title="Number of packets retransmitted"
                        >TX Retries</span
                    >
                    <span class="value"
                        >{clientStats.txRetries.toLocaleString()}</span
                    >
                </div>
                <div class="info-item">
                    <span
                        class="label"
                        title="Number of packets failed to transmit"
                        >TX Failed</span
                    >
                    <span class="value"
                        >{clientStats.txFailed.toLocaleString()}</span
                    >
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
    }

    .panel-header {
        padding: 16px;
        background: var(--panel-soft);
        border-bottom: 1px solid var(--border);
    }

    .panel-header h3 {
        margin: 0;
        font-size: 16px;
        font-weight: 600;
        color: var(--text);
    }

    .section {
        padding: 16px;
        border-bottom: 1px solid var(--border);
    }

    .section h4 {
        margin: 0 0 12px 0;
        font-size: 14px;
        font-weight: 600;
        color: var(--muted);
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .info-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 12px;
    }

    .info-item {
        display: flex;
        flex-direction: column;
        gap: 4px;
    }

    .info-item.full-width {
        grid-column: 1 / -1;
    }

    .label {
        font-size: 12px;
        color: var(--muted-2);
        font-weight: 500;
    }

    .value {
        font-size: 14px;
        color: var(--text);
        font-weight: 500;
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
