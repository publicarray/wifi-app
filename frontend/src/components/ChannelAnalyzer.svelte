<script>
    export let networks = [];

    $: channels2_4GHz = analyze2_4GHzChannels(networks);
    $: channels5GHz = analyze5GHzChannels(networks);
    $: channelWidthMap = getChannelWidthMap(networks);
    $: stats2 = getBandStats(channels2_4GHz);
    $: stats5 = getBandStats(channels5GHz);
    $: overallBusiest = getBusiestChannel([...channels2_4GHz, ...channels5GHz]);
    $: totalAPs = stats2.totalAps + stats5.totalAps;
    $: totalActive = stats2.activeChannels + stats5.activeChannels;

    function analyze2_4GHzChannels(networksToAnalyze) {
        const channels = [];
        for (let i = 1; i <= 14; i++) {
            const channelNetworks = networksToAnalyze.filter(
                (network) =>
                    network.accessPoints &&
                    network.accessPoints.some((ap) => ap.channel === i),
            );

            const apsOnChannel = channelNetworks.flatMap((network) =>
                network.accessPoints.filter((ap) => ap.channel === i),
            );

            channels.push({
                number: i,
                frequency: 2407 + i * 5,
                networks: channelNetworks,
                aps: apsOnChannel,
                utilization: calculateUtilization(apsOnChannel),
                congestion: getCongestionLevel(apsOnChannel.length),
                overlapping: calculateOverlapping(i, networksToAnalyze),
            });
        }
        return channels;
    }

    function analyze5GHzChannels(networksToAnalyze) {
        const channels = [];
        const common5GHzChannels = [
            36, 40, 44, 48, 52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124,
            128, 132, 136, 140, 144, 149, 153, 157, 161, 165,
        ];

        common5GHzChannels.forEach((channel) => {
            const channelNetworks = networksToAnalyze.filter(
                (network) =>
                    network.accessPoints &&
                    network.accessPoints.some((ap) => ap.channel === channel),
            );

            const apsOnChannel = channelNetworks.flatMap((network) =>
                network.accessPoints.filter((ap) => ap.channel === channel),
            );

            channels.push({
                number: channel,
                frequency: 5000 + channel * 5,
                networks: channelNetworks,
                aps: apsOnChannel,
                utilization: calculateUtilization(apsOnChannel),
                congestion: getCongestionLevel(apsOnChannel.length),
                overlapping: 0, // 5GHz channels don't overlap
            });
        });
        return channels;
    }

    function calculateUtilization(aps) {
        if (aps.length === 0) return 0;
        // Rough utilization estimate based on AP count and activity
        return Math.min(aps.length * 20, 100);
    }

    function getCongestionLevel(apCount) {
        switch (true) {
            case apCount === 0:
                return "empty";
            case apCount <= 2:
                return "low";
            case apCount <= 4:
                return "medium";
            default:
                return "high";
        }
    }

    function getBandStats(channels) {
        const totalAps = channels.reduce(
            (sum, channel) => sum + channel.aps.length,
            0,
        );
        const activeChannels = channels.filter(
            (channel) => channel.aps.length > 0,
        ).length;
        const busiest = getBusiestChannel(channels);
        return { totalAps, activeChannels, busiest };
    }

    function getBusiestChannel(channels) {
        return channels.reduce(
            (max, channel) =>
                channel.aps.length > max.aps.length ? channel : max,
            { number: "N/A", aps: [] },
        );
    }

    function calculateOverlapping(channel, networksToAnalyze) {
        if (channel > 14) return 0;
        // 2.4GHz channels overlap by 4 channels
        let overlap = 0;
        for (let i = channel - 4; i <= channel + 4; i++) {
            if (i !== channel && i >= 1 && i <= 14) {
                const hasNetwork = networksToAnalyze.some(
                    (network) =>
                        network.accessPoints &&
                        network.accessPoints.some((ap) => ap.channel === i),
                );
                if (hasNetwork) overlap++;
            }
        }
        return overlap;
    }

    function getChannelWidthMap(networksToMap) {
        const widthMap = {};
        networksToMap.forEach((network) => {
            network.accessPoints.forEach((ap) => {
                const key = `${ap.channel}-${ap.band}`;
                widthMap[key] = Math.max(
                    widthMap[key] || 0,
                    ap.channelWidth || 20,
                );
            });
        });
        return widthMap;
    }

    function getCongestionColor(congestion) {
        switch (congestion) {
            case "empty":
                return "#2f3338";
            case "low":
                return "#22c55e";
            case "medium":
                return "#f59e0b";
            case "high":
                return "#ef4444";
            default:
                return "#3a3f46";
        }
    }

    function getUtilizationColor(utilization) {
        if (utilization < 30) return "#22c55e";
        if (utilization < 70) return "#f59e0b";
        return "#ef4444";
    }

    function formatFrequency(freq) {
        return (freq / 1000).toFixed(3) + " GHz";
    }

    function getChannelWidth(channel, band) {
        const key = `${channel}-${band}`;
        return channelWidthMap[key] || 20;
    }
</script>

<div class="channel-analyzer-container">
    <div class="analyzer-header">
        <div class="title-block">
            <div class="title-row">
                <h3>Channel Analysis</h3>
                <span class="status-pill {totalActive ? 'live' : 'idle'}">
                    {totalActive ? "Live" : "Idle"}
                </span>
            </div>
            <p class="subtitle">
                Congestion map by band, channel width, and overlap.
            </p>
        </div>
        <div class="header-stats">
            <div class="stat-pill">
                <span class="stat-label">Active</span>
                <span class="stat-value">{totalActive}</span>
            </div>
            <div class="stat-pill">
                <span class="stat-label">Total APs</span>
                <span class="stat-value">{totalAPs}</span>
            </div>
            <div class="stat-pill">
                <span class="stat-label">Busiest</span>
                <span class="stat-value">
                    {overallBusiest.number === "N/A"
                        ? "N/A"
                        : `Ch ${overallBusiest.number}`}
                </span>
            </div>
        </div>
    </div>

    <div class="legend">
        <div class="legend-item">
            <div class="legend-color empty"></div>
            <span>Empty</span>
        </div>
        <div class="legend-item">
            <div class="legend-color low"></div>
            <span>Low (1-2 APs)</span>
        </div>
        <div class="legend-item">
            <div class="legend-color medium"></div>
            <span>Medium (3-4 APs)</span>
        </div>
        <div class="legend-item">
            <div class="legend-color high"></div>
            <span>High (5+ APs)</span>
        </div>
        <div class="legend-item">
            <div class="legend-color overlap"></div>
            <span>Overlap (2.4GHz)</span>
        </div>
    </div>

    <!-- 2.4GHz Band -->
    <div class="band-section">
        <div class="band-header">
            <div>
                <h4>2.4GHz Band</h4>
                <p class="band-subtitle">
                    Overlapping channels amplify interference.
                </p>
            </div>
            <div class="band-chips">
                <span class="chip">Active {stats2.activeChannels}/14</span>
                <span class="chip">APs {stats2.totalAps}</span>
                <span class="chip">Busiest Ch {stats2.busiest.number}</span>
            </div>
        </div>
        <div class="channel-grid">
            {#each channels2_4GHz as channel, index}
                <div
                    class="channel-block"
                    class:has-aps={channel.aps.length > 0}
                    style="--congestion-color: {getCongestionColor(
                        channel.congestion,
                    )}; --i: {index}"
                    title="Channel {channel.number} ({formatFrequency(
                        channel.frequency,
                    )}) - {channel.aps
                        .length} APs - {channel.overlapping} overlapping - {getChannelWidth(
                        channel.number,
                        '2.4GHz',
                    )}MHz"
                >
                    <div class="channel-top">
                        <div class="channel-number">{channel.number}</div>
                        <div class="channel-width">
                            {getChannelWidth(channel.number, "2.4GHz")}MHz
                        </div>
                    </div>
                    <div class="channel-meter">
                        <span style="width: {channel.utilization}%"></span>
                    </div>
                    <div class="channel-info">
                        <div class="ap-count">{channel.aps.length} APs</div>
                        {#if channel.overlapping > 0}
                            <div class="overlap-indicator">
                                +{channel.overlapping} overlap
                            </div>
                        {/if}
                    </div>
                </div>
            {/each}
        </div>
        <div class="band-overview">
            <div class="overview-stat">
                <span class="stat-label">Active Channels:</span>
                <span class="stat-value"
                    >{channels2_4GHz.filter((c) => c.aps.length > 0)
                        .length}/14</span
                >
            </div>
            <div class="overview-stat">
                <span class="stat-label">Total APs:</span>
                <span class="stat-value"
                    >{channels2_4GHz.reduce(
                        (sum, c) => sum + c.aps.length,
                        0,
                    )}</span
                >
            </div>
            <div class="overview-stat">
                <span class="stat-label">Most Congested:</span>
                <span class="stat-value">
                    {channels2_4GHz.reduce(
                        (max, c) => (c.aps.length > max.aps.length ? c : max),
                        { number: "N/A", aps: [] },
                    ).number}
                </span>
            </div>
        </div>
    </div>

    <!-- 5GHz Band -->
    <div class="band-section">
        <div class="band-header">
            <div>
                <h4>5GHz Band</h4>
                <p class="band-subtitle">Wider channels, lower overlap.</p>
            </div>
            <div class="band-chips">
                <span class="chip"
                    >Active {stats5.activeChannels}/{channels5GHz.length}</span
                >
                <span class="chip">APs {stats5.totalAps}</span>
                <span class="chip">Busiest Ch {stats5.busiest.number}</span>
            </div>
        </div>
        <div class="channel-grid fiveghz">
            {#each channels5GHz as channel, index}
                <div
                    class="channel-block"
                    class:has-aps={channel.aps.length > 0}
                    style="--congestion-color: {getCongestionColor(
                        channel.congestion,
                    )}; --i: {index}"
                    title="Channel {channel.number} ({formatFrequency(
                        channel.frequency,
                    )}) - {channel.aps.length} APs - {getChannelWidth(
                        channel.number,
                        '5GHz',
                    )}MHz"
                >
                    <div class="channel-top">
                        <div class="channel-number">{channel.number}</div>
                        <div class="channel-width">
                            {getChannelWidth(channel.number, "5GHz")}MHz
                        </div>
                    </div>
                    <div class="channel-meter">
                        <span style="width: {channel.utilization}%"></span>
                    </div>
                    <div class="channel-info">
                        <div class="ap-count">{channel.aps.length} APs</div>
                    </div>
                </div>
            {/each}
        </div>
        <div class="band-overview">
            <div class="overview-stat">
                <span class="stat-label">Active Channels:</span>
                <span class="stat-value"
                    >{channels5GHz.filter((c) => c.aps.length > 0)
                        .length}/{channels5GHz.length}</span
                >
            </div>
            <div class="overview-stat">
                <span class="stat-label">Total APs:</span>
                <span class="stat-value"
                    >{channels5GHz.reduce(
                        (sum, c) => sum + c.aps.length,
                        0,
                    )}</span
                >
            </div>
            <div class="overview-stat">
                <span class="stat-label">Most Congested:</span>
                <span class="stat-value">
                    {channels5GHz.reduce(
                        (max, c) => (c.aps.length > max.aps.length ? c : max),
                        { number: "N/A", aps: [] },
                    ).number}
                </span>
            </div>
        </div>
    </div>

    <!-- Detailed Channel List -->
    <div class="channel-details">
        <h4>Channel Details</h4>
        <div class="channel-list">
            {#each [...channels2_4GHz, ...channels5GHz].filter((c) => c.aps.length > 0) as channel}
                <div class="channel-detail-item">
                    <div class="channel-header">
                        <span class="channel-id">Ch {channel.number}</span>
                        <span class="channel-freq"
                            >{formatFrequency(channel.frequency)}</span
                        >
                        <span class="channel-band"
                            >{channel.number <= 14 ? "2.4GHz" : "5GHz"}</span
                        >
                        <span class="channel-width-badge">
                            {getChannelWidth(
                                channel.number,
                                channel.number <= 14 ? "2.4GHz" : "5GHz",
                            )}MHz
                        </span>
                        <div class="channel-metrics">
                            <span class="congestion-badge {channel.congestion}"
                                >{channel.congestion}</span
                            >
                            <div class="utilization">
                                <div class="utilization-track">
                                    <span
                                        class="utilization-bar"
                                        style="width: {channel.utilization}%; background: {getUtilizationColor(
                                            channel.utilization,
                                        )}"
                                    ></span>
                                </div>
                                <span class="utilization-text"
                                    >{channel.utilization}%</span
                                >
                            </div>
                        </div>
                    </div>
                    <div class="channel-networks">
                        {#each channel.aps.slice(0, 3) as ap}
                            <div class="ap-item">
                                <span class="ap-ssid"
                                    >{ap.ssid || "<Hidden>"}</span
                                >
                                <span class="ap-signal">{ap.signal} dBm</span>
                                <span class="ap-width"
                                    >{ap.channelWidth || 20}MHz</span
                                >
                            </div>
                        {/each}
                        {#if channel.aps.length > 3}
                            <div class="more-aps">
                                +{channel.aps.length - 3} more APs
                            </div>
                        {/if}
                    </div>
                </div>
            {/each}
        </div>
    </div>
</div>

<style>
    .channel-analyzer-container {
        --bg-0: #101216;
        --bg-1: #14171c;
        --panel: #20252b;
        --panel-strong: #1a1f24;
        --panel-soft: #242a31;
        --text: #e6e8eb;
        --muted: #9aa3ad;
        --accent: #4fd1c5;
        --accent-2: #7dd3fc;
        --border: rgba(255, 255, 255, 0.08);

        min-height: 100%;
        overflow: visible;
        padding: 20px;
        padding-bottom: 72px;
        background:
            radial-gradient(
                900px 500px at 5% -10%,
                rgba(79, 209, 197, 0.18),
                transparent 60%
            ),
            radial-gradient(
                800px 400px at 100% 0%,
                rgba(59, 130, 246, 0.14),
                transparent 60%
            ),
            linear-gradient(180deg, var(--bg-0) 0%, var(--bg-1) 100%);
        color: var(--text);
        font-family:
            "Space Grotesk", "Sora", "Avenir Next", "Segoe UI", sans-serif;
    }

    .analyzer-header {
        display: flex;
        justify-content: space-between;
        align-items: flex-end;
        gap: 16px;
        margin-bottom: 12px;
    }

    .title-row {
        display: flex;
        align-items: center;
        gap: 10px;
    }

    .title-block {
        display: flex;
        flex-direction: column;
    }

    .analyzer-header h3 {
        margin: 0;
        font-size: 20px;
        font-weight: 600;
        letter-spacing: 0.4px;
    }

    .status-pill {
        padding: 3px 8px;
        border-radius: 999px;
        font-size: 11px;
        text-transform: uppercase;
        letter-spacing: 0.06em;
        border: 1px solid transparent;
    }

    .status-pill.live {
        color: #b5f3df;
        background: rgba(79, 209, 197, 0.15);
        border-color: rgba(79, 209, 197, 0.4);
    }

    .status-pill.idle {
        color: #cbd5f5;
        background: rgba(125, 211, 252, 0.12);
        border-color: rgba(125, 211, 252, 0.35);
    }

    .subtitle {
        margin: 6px 0 0;
        font-size: 13px;
        color: var(--muted);
    }

    .header-stats {
        display: flex;
        gap: 10px;
        flex-wrap: wrap;
    }

    .stat-pill {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 6px 10px;
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 999px;
    }

    .stat-label {
        font-size: 10px;
        color: var(--muted);
        letter-spacing: 0.08em;
        text-transform: uppercase;
    }

    .stat-value {
        font-size: 13px;
        font-weight: 600;
        color: var(--text);
    }

    .legend {
        display: flex;
        gap: 12px;
        flex-wrap: wrap;
        margin-bottom: 18px;
    }

    .legend-item {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 11px;
        color: var(--muted);
        padding: 4px 8px;
        border-radius: 999px;
        background: rgba(255, 255, 255, 0.04);
        border: 1px solid rgba(255, 255, 255, 0.06);
    }

    .legend-color {
        width: 12px;
        height: 12px;
        border-radius: 4px;
    }

    .legend-color.empty {
        background: #333;
    }
    .legend-color.low {
        background: #22c55e;
    }
    .legend-color.medium {
        background: #f59e0b;
    }
    .legend-color.high {
        background: #ef4444;
    }
    .legend-color.overlap {
        background: transparent;
        border: 1px dashed #f59e0b;
    }

    .band-section {
        margin-bottom: 24px;
        padding: 16px;
        border-radius: 14px;
        border: 1px solid var(--border);
        background: linear-gradient(
            180deg,
            rgba(32, 37, 43, 0.8),
            rgba(24, 28, 34, 0.8)
        );
        box-shadow: 0 18px 30px rgba(0, 0, 0, 0.28);
    }

    .band-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
        margin-bottom: 12px;
    }

    .band-section h4 {
        margin: 0;
        font-size: 17px;
        font-weight: 600;
    }

    .band-subtitle {
        margin: 4px 0 0;
        font-size: 12px;
        color: var(--muted);
    }

    .band-chips {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
        justify-content: flex-end;
    }

    .chip {
        padding: 4px 10px;
        border-radius: 999px;
        font-size: 10px;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        background: rgba(79, 209, 197, 0.12);
        color: #c7f9f2;
        border: 1px solid rgba(79, 209, 197, 0.28);
    }

    .channel-grid {
        display: grid;
        grid-template-columns: repeat(14, minmax(0, 1fr));
        gap: 8px;
        margin-bottom: 14px;
    }

    .channel-grid.fiveghz {
        grid-template-columns: repeat(8, minmax(0, 1fr));
    }

    .channel-block {
        position: relative;
        aspect-ratio: 1;
        min-height: 78px;
        background:
            linear-gradient(
                160deg,
                rgba(255, 255, 255, 0.04),
                rgba(255, 255, 255, 0)
            ),
            linear-gradient(180deg, rgba(0, 0, 0, 0.2), rgba(0, 0, 0, 0.65));
        border: 1px solid rgba(255, 255, 255, 0.08);
        border-radius: 12px;
        display: flex;
        flex-direction: column;
        align-items: stretch;
        justify-content: space-between;
        gap: 6px;
        padding: 8px;
        cursor: pointer;
        overflow: hidden;
        transition:
            transform 0.2s ease,
            box-shadow 0.2s ease,
            border-color 0.2s ease;
        animation: riseIn 420ms ease both;
        animation-delay: calc(var(--i) * 18ms);
    }

    .channel-block::after {
        content: "";
        position: absolute;
        inset: 0;
        background: radial-gradient(
            120px 80px at 10% 10%,
            var(--congestion-color),
            transparent 60%
        );
        opacity: 0.18;
        pointer-events: none;
    }

    .channel-block:hover {
        transform: translateY(-2px);
        border-color: rgba(255, 255, 255, 0.2);
        box-shadow: 0 12px 24px rgba(0, 0, 0, 0.35);
    }

    .channel-block.has-aps {
        border-color: rgba(79, 209, 197, 0.5);
        box-shadow: inset 0 0 0 1px rgba(79, 209, 197, 0.2);
    }

    .channel-top {
        display: flex;
        align-items: baseline;
        justify-content: space-between;
        gap: 6px;
    }

    .channel-number {
        font-size: 14px;
        font-weight: 600;
        color: var(--text);
    }

    .channel-width {
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.06em;
        color: var(--muted);
    }

    .channel-meter {
        height: 6px;
        border-radius: 999px;
        background: rgba(255, 255, 255, 0.1);
        overflow: hidden;
    }

    .channel-meter span {
        display: block;
        height: 100%;
        border-radius: 999px;
        background: linear-gradient(
            90deg,
            var(--congestion-color),
            rgba(255, 255, 255, 0.75)
        );
    }

    .channel-info {
        display: flex;
        flex-direction: column;
        gap: 4px;
        font-size: 11px;
    }

    .ap-count {
        color: var(--accent-2);
        font-weight: 600;
    }

    .overlap-indicator {
        color: #f59e0b;
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    .band-overview {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
        gap: 16px;
        padding: 12px;
        background: rgba(0, 0, 0, 0.35);
        border-radius: 10px;
        border: 1px solid rgba(255, 255, 255, 0.06);
    }

    .overview-stat {
        display: flex;
        flex-direction: column;
        gap: 2px;
    }

    .stat-label {
        font-size: 12px;
        color: var(--muted);
    }

    .stat-value {
        font-size: 14px;
        font-weight: 600;
        color: var(--text);
    }

    .channel-details h4 {
        margin: 20px 0 12px 0;
        font-size: 16px;
        font-weight: 600;
    }

    .channel-list {
        display: flex;
        flex-direction: column;
        gap: 10px;
    }

    .channel-detail-item {
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 12px;
        overflow: hidden;
    }

    .channel-header {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 12px 16px;
        background: var(--panel-strong);
        border-bottom: 1px solid rgba(255, 255, 255, 0.06);
        flex-wrap: wrap;
    }

    .channel-id {
        font-weight: 600;
        color: var(--text);
        min-width: 48px;
        padding: 2px 8px;
        border-radius: 999px;
        background: rgba(79, 209, 197, 0.12);
        border: 1px solid rgba(79, 209, 197, 0.25);
    }

    .channel-freq {
        color: var(--muted);
        font-size: 13px;
    }

    .channel-band {
        background: rgba(255, 255, 255, 0.08);
        padding: 2px 6px;
        border-radius: 6px;
        font-size: 11px;
        color: var(--muted);
    }

    .channel-width-badge {
        background: rgba(125, 211, 252, 0.12);
        padding: 2px 6px;
        border-radius: 6px;
        font-size: 11px;
        color: #cfe9ff;
    }

    .channel-metrics {
        margin-left: auto;
        display: flex;
        align-items: center;
        gap: 8px;
        flex-wrap: wrap;
    }

    .congestion-badge {
        padding: 2px 8px;
        border-radius: 6px;
        font-size: 11px;
        font-weight: 500;
        text-transform: uppercase;
    }

    .congestion-badge.empty {
        background: #2f3338;
        color: #9aa0a6;
    }
    .congestion-badge.low {
        background: #22c55e;
        color: #0f172a;
    }
    .congestion-badge.medium {
        background: #f59e0b;
        color: #0f172a;
    }
    .congestion-badge.high {
        background: #ef4444;
        color: #0f172a;
    }

    .utilization {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .utilization-track {
        width: 90px;
        height: 8px;
        border-radius: 999px;
        background: rgba(255, 255, 255, 0.1);
        overflow: hidden;
    }

    .utilization-bar {
        display: block;
        height: 100%;
        border-radius: 999px;
    }

    .utilization-text {
        font-size: 12px;
        color: var(--muted);
        min-width: 30px;
    }

    .channel-networks {
        padding: 8px 16px;
    }

    .ap-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 6px 0;
        font-size: 13px;
        border-bottom: 1px solid rgba(255, 255, 255, 0.06);
    }

    .ap-ssid {
        color: var(--text);
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        padding-right: 8px;
    }

    .ap-signal {
        color: #7dd3fc;
        font-weight: 500;
        min-width: 50px;
        text-align: right;
    }

    .ap-width {
        color: var(--muted);
        min-width: 40px;
        text-align: right;
    }

    .more-aps {
        color: var(--muted);
        font-style: italic;
        font-size: 12px;
        padding: 4px 0;
    }

    .channel-networks .ap-item:last-child {
        border-bottom: none;
    }

    @keyframes riseIn {
        from {
            opacity: 0;
            transform: translateY(6px);
        }
        to {
            opacity: 1;
            transform: translateY(0);
        }
    }

    @media (max-width: 1360px) {
        .channel-grid {
            grid-template-columns: repeat(12, minmax(0, 1fr));
        }

        .channel-grid.fiveghz {
            grid-template-columns: repeat(7, minmax(0, 1fr));
        }
    }

    @media (max-width: 1200px) {
        .channel-grid {
            grid-template-columns: repeat(10, minmax(0, 1fr));
        }

        .channel-grid.fiveghz {
            grid-template-columns: repeat(6, minmax(0, 1fr));
        }
    }

    @media (max-width: 1100px) {
        .channel-block {
            padding: 6px;
            min-height: 70px;
        }

        .channel-width {
            display: none;
        }

        .channel-info {
            flex-direction: row;
            justify-content: space-between;
        }

        .overlap-indicator {
            display: none;
        }
    }

    @media (max-width: 980px) {
        .channel-grid {
            grid-template-columns: repeat(7, minmax(0, 1fr));
        }

        .channel-grid.fiveghz {
            grid-template-columns: repeat(4, minmax(0, 1fr));
        }
    }

    /* Responsive adjustments */
    @media (max-width: 768px) {
        .channel-analyzer-container {
            padding: 12px;
        }

        .analyzer-header {
            flex-direction: column;
            align-items: flex-start;
            gap: 10px;
        }

        .channel-grid {
            grid-template-columns: repeat(5, minmax(0, 1fr));
        }

        .channel-grid.fiveghz {
            grid-template-columns: repeat(3, minmax(0, 1fr));
        }

        .band-header {
            flex-direction: column;
            align-items: flex-start;
        }

        .band-chips {
            justify-content: flex-start;
        }

        .band-overview {
            grid-template-columns: 1fr;
            gap: 8px;
        }

        .channel-header {
            flex-wrap: wrap;
            gap: 8px;
        }

        .channel-metrics {
            width: 100%;
            margin-left: 0;
            justify-content: space-between;
        }

        .channel-width {
            display: none;
        }

        .channel-info {
            flex-direction: row;
            justify-content: space-between;
        }

        .overlap-indicator {
            display: none;
        }
    }
</style>
