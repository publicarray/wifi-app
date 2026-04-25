<script>
    import { onMount, onDestroy } from "svelte";

    export let networks = [];
    // Optional per-channel stats straight from the backend (utilization,
    // congestion level, overlap counts). Not yet consumed by the derivation
    // below but plumbed through so callers have a single source of truth.
    export let channelAnalysis = [];

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
        if (apCount === 0) return "empty";
        if (apCount <= 2) return "low";
        if (apCount <= 4) return "medium";
        return "high";
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
                return "var(--border-strong)";
            case "low":
                return "var(--success)";
            case "medium":
                return "var(--warning)";
            case "high":
                return "var(--danger)";
            default:
                return "var(--border)";
        }
    }

    function getUtilizationColor(utilization) {
        if (utilization < 30) return "var(--success)";
        if (utilization < 70) return "var(--warning)";
        return "var(--danger)";
    }

    function formatFrequency(freq) {
        return (freq / 1000).toFixed(3) + " GHz";
    }

    function getChannelWidth(channel, band) {
        const key = `${channel}-${band}`;
        return channelWidthMap[key] || 20;
    }

    // ── Spectrum bell-curve chart ─────────────────────────────
    // Mirrors the design's `screen-channels.jsx` SpectrumChart. Each AP is
    // drawn as a smooth bell at its channel center freq, with width scaled to
    // the AP's channel width and height to its signal level. Clicking a
    // channel column highlights it.
    let spectrumBand = "5GHz";
    let selectedSpectrumChannel = null;
    let spectrumWidth = 900;
    let spectrumWrapper;

    const SPECTRUM_HEIGHT = 240;
    const SPECTRUM_PAD = { top: 16, right: 16, bottom: 30, left: 44 };
    const SPECTRUM_Y_TICKS = [-40, -50, -60, -70, -80];
    const SPECTRUM_Y_MIN = -90;
    const SPECTRUM_Y_MAX = -30;

    const BAND_DEFS_SPEC = {
        "2.4GHz": {
            channels: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14],
            stepMHz: 5,
            widthDefault: 20,
            dfs: [],
        },
        "5GHz": {
            channels: [
                36, 40, 44, 48, 52, 56, 60, 64, 100, 104, 108, 112, 116, 120,
                124, 128, 132, 136, 140, 144, 149, 153, 157, 161, 165,
            ],
            stepMHz: 20,
            widthDefault: 40,
            dfs: [
                52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124, 128, 132,
                136, 140, 144,
            ],
        },
        "6GHz": {
            channels: [
                1, 5, 9, 13, 17, 21, 25, 29, 33, 37, 41, 45, 49, 53, 57, 61,
            ],
            stepMHz: 20,
            widthDefault: 80,
            dfs: [],
        },
    };

    $: spectrumDef = BAND_DEFS_SPEC[spectrumBand];
    $: spectrumAPs = collectSpectrumAPs(networks, spectrumBand);
    $: connectedAP = findConnectedAP(networks);

    function collectSpectrumAPs(allNetworks, band) {
        const out = [];
        for (const network of allNetworks || []) {
            for (const ap of network.accessPoints || []) {
                const apBand = ap.band || apBandFromFreq(ap.frequency);
                if (apBand !== band) continue;
                if (typeof ap.signal !== "number") continue;
                if (typeof ap.channel !== "number") continue;
                out.push({ ...ap, ssid: ap.ssid || network.ssid || "" });
            }
        }
        return out;
    }

    function findConnectedAP(allNetworks) {
        for (const network of allNetworks || []) {
            const ap = (network.accessPoints || []).find(
                (a) => a && a.bssid && a.bssid.toLowerCase() === (clientStatsBSSIDLower() || ""),
            );
            if (ap) return ap;
        }
        return null;
    }

    function clientStatsBSSIDLower() {
        // ChannelAnalyzer doesn't currently receive clientStats; fall back to
        // matching nothing. Connected highlight comes from the bestSignalAP /
        // network.connected attribute instead in the bell renderer.
        return "";
    }

    function apBandFromFreq(freq) {
        if (!freq) return "";
        if (freq >= 5925) return "6GHz";
        if (freq >= 4900) return "5GHz";
        if (freq > 0) return "2.4GHz";
        return "";
    }

    function spectrumXForChannel(ch) {
        const def = spectrumDef;
        if (!def) return 0;
        const i = def.channels.indexOf(ch);
        if (i < 0) {
            // Off-grid channel — position by interpolation to the nearest two.
            for (let j = 0; j < def.channels.length - 1; j++) {
                const a = def.channels[j];
                const b = def.channels[j + 1];
                if (ch > a && ch < b) {
                    const t = (ch - a) / (b - a);
                    const innerW =
                        spectrumWidth - SPECTRUM_PAD.left - SPECTRUM_PAD.right;
                    const stepPx = innerW / (def.channels.length - 1);
                    return SPECTRUM_PAD.left + (j + t) * stepPx;
                }
            }
            return SPECTRUM_PAD.left;
        }
        const innerW =
            spectrumWidth - SPECTRUM_PAD.left - SPECTRUM_PAD.right;
        return (
            SPECTRUM_PAD.left +
            (i / Math.max(1, def.channels.length - 1)) * innerW
        );
    }

    function spectrumYForSignal(dBm) {
        const innerH =
            SPECTRUM_HEIGHT - SPECTRUM_PAD.top - SPECTRUM_PAD.bottom;
        return (
            SPECTRUM_PAD.top +
            ((SPECTRUM_Y_MAX - dBm) / (SPECTRUM_Y_MAX - SPECTRUM_Y_MIN)) *
                innerH
        );
    }

    function spectrumWidthPx(channelWidthMHz) {
        const def = spectrumDef;
        if (!def) return 0;
        const innerW =
            spectrumWidth - SPECTRUM_PAD.left - SPECTRUM_PAD.right;
        const stepPx = innerW / Math.max(1, def.channels.length - 1);
        // Convert: each "step" between adjacent listed channels equals
        // stepMHz. So an AP with channelWidth W spans W / stepMHz steps.
        return stepPx * (channelWidthMHz / def.stepMHz);
    }

    function bellPath(cx, cy, w, baseY) {
        const w2 = w / 2;
        return (
            `M${cx - w2},${baseY} ` +
            `C${cx - w2 * 0.35},${baseY} ${cx - w2 * 0.45},${cy} ${cx},${cy} ` +
            `C${cx + w2 * 0.45},${cy} ${cx + w2 * 0.35},${baseY} ${cx + w2},${baseY} Z`
        );
    }

    function spectrumColorForSignal(dBm, isConnected) {
        if (isConnected) return "var(--acc-1)";
        if (dBm >= -60) return "var(--ok)";
        if (dBm >= -72) return "var(--warn)";
        return "var(--bad)";
    }

    function spectrumOnSelect(ch) {
        selectedSpectrumChannel =
            selectedSpectrumChannel === ch ? null : ch;
    }

    function spectrumTrunc(text, n) {
        if (!text) return "";
        return text.length > n ? text.slice(0, n - 1) + "…" : text;
    }

    function spectrumChannelLabel(ch, idx, totalCount, selected) {
        // Always label the selected channel; for crowded bands, label every Nth.
        if (ch === selected) return true;
        if (totalCount <= 14) return true;
        return idx % Math.ceil(totalCount / 10) === 0;
    }

    // ResizeObserver wires width to the SVG so the spectrum stays responsive.
    let spectrumRO = null;
    onMount(() => {
        if (!spectrumWrapper || typeof ResizeObserver === "undefined") return;
        spectrumRO = new ResizeObserver((entries) => {
            const r = entries[0]?.contentRect;
            if (r && r.width > 0) {
                spectrumWidth = Math.max(400, Math.round(r.width));
            }
        });
        spectrumRO.observe(spectrumWrapper);
    });
    onDestroy(() => {
        spectrumRO?.disconnect();
    });

    // Sort APs weakest-first so the strongest paint on top.
    $: spectrumAPsSorted = [...spectrumAPs].sort(
        (a, b) => a.signal - b.signal,
    );
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

    <!-- Spectrum bell-curve chart -->
    <div class="spectrum-panel">
        <div class="spectrum-header">
            <div>
                <div class="spectrum-title">{spectrumBand} spectrum</div>
                <div class="spectrum-sub">
                    {#if spectrumBand === "2.4GHz"}
                        Crowded, longer range — 1, 6, 11 are the only
                        non-overlapping channels at 20 MHz.
                    {:else if spectrumBand === "5GHz"}
                        UNII bands — DFS channels may pause briefly for
                        radar detection.
                    {:else}
                        UNII-5..8 — WiFi 6E/7 only — clean spectrum, short
                        range.
                    {/if}
                </div>
            </div>
            <div class="spectrum-spacer"></div>
            <div class="spectrum-band-tabs">
                {#each ["2.4GHz", "5GHz", "6GHz"] as band}
                    <button
                        type="button"
                        class:active={spectrumBand === band}
                        on:click={() => {
                            spectrumBand = band;
                            selectedSpectrumChannel = null;
                        }}
                    >{band}</button>
                {/each}
            </div>
            <div class="spectrum-legend">
                <span class="leg-dot" style="background: var(--ok)"></span>Strong
                <span class="leg-dot" style="background: var(--warn)"></span>Fair
                <span class="leg-dot" style="background: var(--bad)"></span>Weak
            </div>
        </div>

        <div class="spectrum-svg-wrapper" bind:this={spectrumWrapper}>
            {#if spectrumDef}
                {@const innerW =
                    spectrumWidth - SPECTRUM_PAD.left - SPECTRUM_PAD.right}
                {@const innerH =
                    SPECTRUM_HEIGHT - SPECTRUM_PAD.top - SPECTRUM_PAD.bottom}
                {@const stepPx =
                    innerW / Math.max(1, spectrumDef.channels.length - 1)}
                {@const baseY = SPECTRUM_PAD.top + innerH}
                <svg
                    class="spectrum-svg"
                    width={spectrumWidth}
                    height={SPECTRUM_HEIGHT}
                    viewBox={`0 0 ${spectrumWidth} ${SPECTRUM_HEIGHT}`}
                    preserveAspectRatio="none"
                >
                    <!-- Y grid + dBm labels -->
                    {#each SPECTRUM_Y_TICKS as tick}
                        <line
                            x1={SPECTRUM_PAD.left}
                            y1={spectrumYForSignal(tick)}
                            x2={spectrumWidth - SPECTRUM_PAD.right}
                            y2={spectrumYForSignal(tick)}
                            stroke="var(--line-1)"
                            stroke-dasharray="2,4"
                        />
                        <text
                            x={SPECTRUM_PAD.left - 8}
                            y={spectrumYForSignal(tick)}
                            fill="var(--fg-3)"
                            font-size="10"
                            font-family="var(--font-mono)"
                            text-anchor="end"
                            dominant-baseline="middle"
                        >{tick}</text>
                    {/each}

                    <!-- Selected channel column (translucent accent) -->
                    {#if selectedSpectrumChannel != null}
                        <rect
                            x={spectrumXForChannel(selectedSpectrumChannel) -
                                stepPx * 0.45}
                            y={SPECTRUM_PAD.top}
                            width={stepPx * 0.9}
                            height={innerH}
                            fill="var(--acc-1)"
                            opacity="0.06"
                        />
                        <line
                            x1={spectrumXForChannel(selectedSpectrumChannel)}
                            x2={spectrumXForChannel(selectedSpectrumChannel)}
                            y1={SPECTRUM_PAD.top}
                            y2={baseY}
                            stroke="var(--acc-1)"
                            stroke-dasharray="3,3"
                            stroke-width="1"
                            opacity="0.6"
                        />
                    {/if}

                    <!-- DFS markers -->
                    {#each spectrumDef.dfs as dfsCh}
                        <line
                            x1={spectrumXForChannel(dfsCh)}
                            x2={spectrumXForChannel(dfsCh)}
                            y1={baseY - 2}
                            y2={baseY + 4}
                            stroke="#a78bfa"
                            stroke-width="2"
                        />
                    {/each}

                    <!-- AP bells -->
                    {#each spectrumAPsSorted as ap (ap.bssid + ap.channel)}
                        {@const cx = spectrumXForChannel(ap.channel)}
                        {@const cy = spectrumYForSignal(ap.signal)}
                        {@const w = spectrumWidthPx(
                            ap.channelWidth || spectrumDef.widthDefault,
                        )}
                        {@const color = spectrumColorForSignal(ap.signal, false)}
                        <path
                            d={bellPath(cx, cy, w, baseY)}
                            fill={color}
                            fill-opacity="0.18"
                            stroke={color}
                            stroke-width="1.2"
                            stroke-linejoin="round"
                        />
                        <text
                            x={cx}
                            y={cy - 6}
                            fill={color}
                            font-size="10.5"
                            font-weight="600"
                            text-anchor="middle"
                            style="pointer-events: none"
                        >{spectrumTrunc(ap.ssid, 14)}</text>
                        <text
                            x={cx}
                            y={cy - 18}
                            fill="var(--fg-3)"
                            font-size="9"
                            font-family="var(--font-mono)"
                            text-anchor="middle"
                            style="pointer-events: none"
                        >{ap.signal}</text>
                    {/each}

                    <!-- X axis -->
                    <line
                        x1={SPECTRUM_PAD.left}
                        y1={baseY}
                        x2={spectrumWidth - SPECTRUM_PAD.right}
                        y2={baseY}
                        stroke="var(--line-2)"
                    />

                    <!-- Channel ticks + labels (clickable) -->
                    {#each spectrumDef.channels as ch, idx}
                        {@const x = spectrumXForChannel(ch)}
                        <!-- svelte-ignore a11y-click-events-have-key-events -->
                        <g
                            class="spectrum-tick"
                            on:click={() => spectrumOnSelect(ch)}
                            style="cursor: pointer"
                        >
                            <rect
                                x={x - stepPx * 0.45}
                                y={SPECTRUM_PAD.top}
                                width={stepPx * 0.9}
                                height={innerH + 18}
                                fill="transparent"
                            />
                            <line
                                x1={x}
                                y1={baseY}
                                x2={x}
                                y2={baseY + 3}
                                stroke="var(--line-3)"
                            />
                            {#if spectrumChannelLabel(ch, idx, spectrumDef.channels.length, selectedSpectrumChannel)}
                                <text
                                    x={x}
                                    y={baseY + 16}
                                    fill={ch === selectedSpectrumChannel
                                        ? "var(--acc-1)"
                                        : "var(--fg-2)"}
                                    font-size="10"
                                    font-family="var(--font-mono)"
                                    font-weight={ch === selectedSpectrumChannel
                                        ? 600
                                        : 400}
                                    text-anchor="middle"
                                >{ch}</text>
                            {/if}
                        </g>
                    {/each}

                    <!-- Axis labels -->
                    <text
                        x={SPECTRUM_PAD.left - 36}
                        y={SPECTRUM_PAD.top - 2}
                        fill="var(--fg-3)"
                        font-size="9"
                        font-family="var(--font-mono)"
                        transform={`rotate(-90 ${SPECTRUM_PAD.left - 36} ${SPECTRUM_PAD.top - 2})`}
                    >dBm</text>
                    <text
                        x={spectrumWidth / 2}
                        y={SPECTRUM_HEIGHT - 4}
                        fill="var(--fg-3)"
                        font-size="10"
                        text-anchor="middle"
                    >Channel</text>
                </svg>
            {/if}
            {#if spectrumAPs.length === 0}
                <div class="spectrum-empty">
                    No APs detected on the {spectrumBand} band yet — start a
                    scan or switch bands.
                </div>
            {/if}
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
    /* ── Spectrum bell-curve panel ───────────────────────────── */
    .spectrum-panel {
        background: var(--bg-2);
        border: 1px solid var(--line-1);
        border-radius: 8px;
        margin-bottom: 16px;
        overflow: hidden;
    }

    .spectrum-header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 16px;
        border-bottom: 1px solid var(--line-1);
        flex-wrap: wrap;
    }

    .spectrum-title {
        font-size: 12px;
        font-weight: 600;
        color: var(--fg-1);
        letter-spacing: 0.01em;
    }

    .spectrum-sub {
        font-size: 11px;
        color: var(--fg-3);
        margin-top: 2px;
        max-width: 60ch;
    }

    .spectrum-spacer {
        flex: 1;
    }

    .spectrum-band-tabs {
        display: inline-flex;
        background: var(--bg-3);
        border: 1px solid var(--line-2);
        border-radius: 6px;
        padding: 2px;
        gap: 2px;
    }

    .spectrum-band-tabs button {
        background: transparent;
        border: none;
        color: var(--fg-2);
        font-size: 12px;
        padding: 4px 12px;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
        font-family: inherit;
    }

    .spectrum-band-tabs button.active {
        background: var(--bg-1);
        color: var(--fg-1);
        box-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    }

    .spectrum-legend {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 11px;
        color: var(--fg-3);
        font-family: var(--font-mono);
    }

    .leg-dot {
        display: inline-block;
        width: 8px;
        height: 8px;
        border-radius: 2px;
        margin-right: 2px;
        opacity: 0.85;
    }

    .leg-dot + .leg-dot {
        margin-left: 6px;
    }

    .spectrum-svg-wrapper {
        padding: 14px;
        position: relative;
        min-height: 240px;
    }

    .spectrum-svg {
        display: block;
        width: 100%;
        height: 240px;
    }

    .spectrum-tick:hover rect {
        fill: var(--bg-3);
        opacity: 0.4;
    }

    .spectrum-empty {
        position: absolute;
        inset: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--fg-3);
        font-size: 12px;
        pointer-events: none;
    }

    .channel-analyzer-container {
        min-height: 100%;
        overflow: visible;
        padding: 20px;
        padding-bottom: 72px;
        background:
            radial-gradient(
                900px 500px at 5% -10%,
                var(--channel-bg-radial-1),
                transparent 60%
            ),
            radial-gradient(
                800px 400px at 100% 0%,
                var(--channel-bg-radial-2),
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
        color: var(--success);
        background: color-mix(in srgb, var(--success) 18%, transparent);
        border-color: color-mix(in srgb, var(--success) 45%, transparent);
    }

    .status-pill.idle {
        color: var(--accent-2);
        background: color-mix(in srgb, var(--accent-2) 18%, transparent);
        border-color: color-mix(in srgb, var(--accent-2) 40%, transparent);
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
        background: var(--channel-legend-bg);
        border: 1px solid var(--channel-legend-border);
    }

    .legend-color {
        width: 12px;
        height: 12px;
        border-radius: 4px;
    }

    .legend-color.empty {
        background: var(--border-strong);
    }
    .legend-color.low {
        background: var(--success);
    }
    .legend-color.medium {
        background: var(--warning);
    }
    .legend-color.high {
        background: var(--danger);
    }
    .legend-color.overlap {
        background: transparent;
        border: 1px dashed var(--warning);
    }

    .band-section {
        margin-bottom: 24px;
        padding: 16px;
        border-radius: 14px;
        border: 1px solid var(--border);
        background: linear-gradient(
            180deg,
            var(--panel-gradient-1),
            var(--panel-gradient-2)
        );
        box-shadow: var(--panel-shadow);
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
        background: color-mix(in srgb, var(--accent) 16%, transparent);
        color: var(--text);
        border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
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
                var(--channel-block-gloss-1),
                var(--channel-block-gloss-2)
            ),
            linear-gradient(
                180deg,
                var(--channel-block-shade-1),
                var(--channel-block-shade-2)
            );
        border: 1px solid var(--channel-block-border);
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
        border-color: var(--channel-hover-border);
        box-shadow: var(--channel-hover-shadow);
    }

    .channel-block.has-aps {
        border-color: var(--channel-active-border);
        box-shadow: var(--channel-active-shadow);
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
        background: var(--channel-meter-track);
        overflow: hidden;
    }

    .channel-meter span {
        display: block;
        height: 100%;
        border-radius: 999px;
        background: linear-gradient(
            90deg,
            var(--congestion-color),
            var(--channel-meter-highlight)
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
        color: var(--warning);
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    .band-overview {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
        gap: 16px;
        padding: 12px;
        background: var(--channel-overview-bg);
        border-radius: 10px;
        border: 1px solid var(--channel-overview-border);
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
        border-bottom: 1px solid var(--channel-header-border);
        flex-wrap: wrap;
    }

    .channel-id {
        font-weight: 600;
        color: var(--text);
        min-width: 48px;
        padding: 2px 8px;
        border-radius: 999px;
        background: var(--channel-id-bg);
        border: 1px solid var(--channel-id-border);
    }

    .channel-freq {
        color: var(--muted);
        font-size: 13px;
    }

    .channel-band {
        background: var(--channel-band-bg);
        padding: 2px 6px;
        border-radius: 6px;
        font-size: 11px;
        color: var(--muted);
    }

    .channel-width-badge {
        background: color-mix(in srgb, var(--accent-2) 18%, transparent);
        padding: 2px 6px;
        border-radius: 6px;
        font-size: 11px;
        color: var(--accent-2);
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
        background: var(--panel-strong);
        color: var(--muted-2);
    }
    .congestion-badge.low {
        background: var(--success);
        color: var(--text-on-accent);
    }
    .congestion-badge.medium {
        background: var(--warning);
        color: var(--text-on-accent);
    }
    .congestion-badge.high {
        background: var(--danger);
        color: var(--text-on-accent);
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
        background: var(--channel-meter-track);
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
        border-bottom: 1px solid var(--channel-divider);
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
        color: var(--accent-2);
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
