<script>
    import { onMount, onDestroy } from "svelte";

    export let networks = [];
    export let clientStats = null;
    // Optional per-channel stats straight from the backend (utilization,
    // congestion level, overlap counts). Not yet consumed by the derivation
    // below but plumbed through so callers have a single source of truth.
    export let channelAnalysis = [];

    // Channel layouts per band; mirror screen-channels.jsx BAND_DEFS so we
    // share semantics (DFS, non-overlapping, default widths).
    const BAND_DEFS = {
        "2.4GHz": {
            title: "2.4 GHz",
            subtitle:
                "Crowded, longer range — only 1, 6, 11 are non-overlapping at 20 MHz.",
            channels: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14],
            stepMHz: 5,
            widthDefault: 20,
            nonOverlap: [1, 6, 11],
            dfs: [],
            freqOf: (ch) => 2.407 + ch * 0.005,
        },
        "5GHz": {
            title: "5 GHz",
            subtitle:
                "UNII bands — DFS channels may pause briefly for radar detection.",
            channels: [
                36, 40, 44, 48, 52, 56, 60, 64, 100, 104, 108, 112, 116, 120,
                124, 128, 132, 136, 140, 144, 149, 153, 157, 161, 165,
            ],
            stepMHz: 20,
            widthDefault: 40,
            nonOverlap: [],
            dfs: [
                52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124, 128, 132,
                136, 140, 144,
            ],
            freqOf: (ch) => 5.0 + ch * 0.005,
        },
        "6GHz": {
            title: "6 GHz",
            subtitle:
                "UNII-5..8 — WiFi 6E/7 only — clean spectrum, short range.",
            channels: Array.from({ length: 59 }, (_, i) => 1 + i * 4),
            stepMHz: 20,
            widthDefault: 80,
            nonOverlap: [],
            dfs: [],
            freqOf: (ch) => 5.95 + ch * 0.005,
        },
    };

    // UNII band labels drawn above the spectrum axis.
    const UNII_BANDS = {
        "5GHz": [
            { label: "UNII-1", startGHz: 5.15, endGHz: 5.25 },
            { label: "UNII-2A", startGHz: 5.25, endGHz: 5.33 },
            { label: "UNII-2C", startGHz: 5.49, endGHz: 5.73 },
            { label: "UNII-3", startGHz: 5.735, endGHz: 5.835 },
        ],
        "6GHz": [
            { label: "UNII-5", startGHz: 5.925, endGHz: 6.425 },
            { label: "UNII-6", startGHz: 6.425, endGHz: 6.525 },
            { label: "UNII-7", startGHz: 6.525, endGHz: 6.875 },
            { label: "UNII-8", startGHz: 6.875, endGHz: 7.125 },
        ],
        "2.4GHz": [],
    };

    // Map (primary channel, bandwidth) → actual center channel per 802.11.
    // Without secondary-channel offset info from the scanner we infer center
    // from the standard primary→group mappings.
    function actualCenterChannel(primary, widthMHz, band) {
        if (!widthMHz || widthMHz <= 20) return primary;
        if (band === "5GHz") {
            if (widthMHz === 40) {
                const pairs = [
                    [36, 40, 38], [44, 48, 46], [52, 56, 54], [60, 64, 62],
                    [100, 104, 102], [108, 112, 110], [116, 120, 118],
                    [124, 128, 126], [132, 136, 134], [140, 144, 142],
                    [149, 153, 151], [157, 161, 159],
                ];
                for (const [a, b, c] of pairs)
                    if (primary === a || primary === b) return c;
            }
            if (widthMHz === 80) {
                const groups = [
                    [[36, 40, 44, 48], 42], [[52, 56, 60, 64], 58],
                    [[100, 104, 108, 112], 106], [[116, 120, 124, 128], 122],
                    [[132, 136, 140, 144], 138], [[149, 153, 157, 161], 155],
                ];
                for (const [chs, c] of groups)
                    if (chs.includes(primary)) return c;
            }
            if (widthMHz === 160) {
                if (primary >= 36 && primary <= 64) return 50;
                if (primary >= 100 && primary <= 128) return 114;
                if (primary >= 149 && primary <= 177) return 163;
            }
        }
        if (band === "6GHz") {
            if (widthMHz === 40)
                return 3 + Math.floor((primary - 1) / 8) * 8;
            if (widthMHz === 80)
                return 7 + Math.floor((primary - 1) / 16) * 16;
            if (widthMHz === 160)
                return 15 + Math.floor((primary - 1) / 32) * 32;
            if (widthMHz === 320)
                return 31 + Math.floor((primary - 1) / 64) * 64;
        }
        if (band === "2.4GHz") {
            if (widthMHz === 40) return primary + 2;
        }
        return primary;
    }

    let spectrumBand = "5GHz";
    let selectedChannel = null;
    let userSelectedBand = null; // remembers band the user picked an explicit channel on

    $: bandDef = BAND_DEFS[spectrumBand];
    $: bandAPs = collectSpectrumAPs(networks, spectrumBand);
    $: connectedBSSID = (clientStats?.bssid || "").toLowerCase();
    $: connectedAP = findConnectedAP(networks, connectedBSSID);
    $: connectedOnBand = connectedAP && apBand(connectedAP) === spectrumBand;

    // Pick a sensible default channel when the band changes or APs first arrive
    $: pickDefaultChannel(spectrumBand, bandAPs, connectedAP);

    function pickDefaultChannel(band, aps, connected) {
        if (userSelectedBand === band && selectedChannel != null) return;
        if (connected && apBand(connected) === band) {
            selectedChannel = connected.channel;
            return;
        }
        const counts = channelCounts(aps);
        const busiest = Object.entries(counts).sort(
            (a, b) => b[1] - a[1],
        )[0];
        if (busiest) {
            selectedChannel = Number(busiest[0]);
            return;
        }
        selectedChannel = BAND_DEFS[band].channels[0];
    }

    function channelCounts(aps) {
        const c = {};
        for (const ap of aps) {
            c[ap.channel] = (c[ap.channel] || 0) + 1;
        }
        return c;
    }

    // Stats for the active band (drives the toolbar KPI chips)
    $: bandCounts = channelCounts(bandAPs);
    $: activeChannels = new Set(bandAPs.map((a) => a.channel)).size;
    $: busiestEntry = Object.entries(bandCounts).sort(
        (a, b) => b[1] - a[1],
    )[0];
    $: cleanestChannel = computeCleanest(spectrumBand, bandCounts);

    function computeCleanest(band, counts) {
        const candidates =
            band === "2.4GHz"
                ? [1, 6, 11]
                : BAND_DEFS[band].channels;
        return candidates
            .map((ch) => ({ ch, aps: counts[ch] || 0 }))
            .sort((a, b) => a.aps - b.aps)[0];
    }

    // APs on the selected channel + APs that overlap it (rough overlap test
    // mirrors screen-channels.jsx — combined half-widths against center freqs).
    $: apsOnSelected = bandAPs.filter((a) => a.channel === selectedChannel);
    $: apsOverlapSelected = computeOverlap(
        bandAPs,
        bandDef,
        selectedChannel,
    );

    function computeOverlap(aps, def, ch) {
        if (!def || ch == null) return [];
        const myCenter = def.freqOf(ch);
        return aps.filter((ap) => {
            if (ap.channel === ch) return false;
            const w = ap.channelWidth || def.widthDefault;
            const apCenterCh = actualCenterChannel(ap.channel, w, spectrumBand);
            const theirCenter = def.freqOf(apCenterCh);
            const combinedHalfWidth = (w / 2 + 20 / 2) / 1000;
            return Math.abs(myCenter - theirCenter) < combinedHalfWidth;
        });
    }

    function collectSpectrumAPs(allNetworks, band) {
        const out = [];
        for (const network of allNetworks || []) {
            for (const ap of network.accessPoints || []) {
                const apBandStr = apBand(ap);
                if (apBandStr !== band) continue;
                if (typeof ap.signal !== "number") continue;
                if (typeof ap.channel !== "number") continue;
                out.push({ ...ap, ssid: ap.ssid || network.ssid || "" });
            }
        }
        return out;
    }

    function findConnectedAP(allNetworks, bssidLower) {
        if (!bssidLower) return null;
        for (const network of allNetworks || []) {
            const ap = (network.accessPoints || []).find(
                (a) => a && a.bssid && a.bssid.toLowerCase() === bssidLower,
            );
            if (ap) return { ...ap, ssid: ap.ssid || network.ssid || "" };
        }
        return null;
    }

    function apBand(ap) {
        if (ap?.band) return ap.band;
        const f = ap?.frequency;
        if (!f) return "";
        if (f >= 5925) return "6GHz";
        if (f >= 4900) return "5GHz";
        if (f > 0) return "2.4GHz";
        return "";
    }

    // ── Spectrum SVG geometry ─────────────────────────────────
    let spectrumWidth = 900;
    let spectrumWrapper;
    const SPECTRUM_HEIGHT = 260;
    const SPECTRUM_PAD = { top: 32, right: 16, bottom: 30, left: 44 };
    const SPECTRUM_Y_TICKS = [-40, -50, -60, -70, -80];
    const SPECTRUM_Y_MIN = -90;
    const SPECTRUM_Y_MAX = -30;

    // Real-frequency x-axis: spans from first channel's center to last
    // channel's center, linearly mapped to the inner plot area in MHz.
    function spectrumRangeGHz() {
        const def = bandDef;
        if (!def) return [0, 1];
        const first = def.freqOf(def.channels[0]);
        const last = def.freqOf(def.channels[def.channels.length - 1]);
        // Pad half a channel width on each side so edge bells aren't clipped.
        const padGHz = (def.widthDefault / 2) / 1000;
        return [first - padGHz, last + padGHz];
    }

    function spectrumXForFreqGHz(freqGHz) {
        const innerW =
            spectrumWidth - SPECTRUM_PAD.left - SPECTRUM_PAD.right;
        const [minF, maxF] = spectrumRangeGHz();
        if (maxF === minF) return SPECTRUM_PAD.left;
        return (
            SPECTRUM_PAD.left +
            ((freqGHz - minF) / (maxF - minF)) * innerW
        );
    }

    function spectrumXForChannel(ch) {
        const def = bandDef;
        if (!def) return 0;
        return spectrumXForFreqGHz(def.freqOf(ch));
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
        const innerW =
            spectrumWidth - SPECTRUM_PAD.left - SPECTRUM_PAD.right;
        const [minF, maxF] = spectrumRangeGHz();
        const totalMHz = (maxF - minF) * 1000;
        if (totalMHz <= 0) return 0;
        return (channelWidthMHz / totalMHz) * innerW;
    }

    function bellPath(cx, cy, w, baseY) {
        const w2 = w / 2;
        return (
            `M${cx - w2},${baseY} ` +
            `C${cx - w2 * 0.35},${baseY} ${cx - w2 * 0.45},${cy} ${cx},${cy} ` +
            `C${cx + w2 * 0.45},${cy} ${cx + w2 * 0.35},${baseY} ${cx + w2},${baseY} Z`
        );
    }

    function colorForSignal(dBm, isConnected) {
        if (isConnected) return "var(--acc-1)";
        if (dBm >= -60) return "var(--ok)";
        if (dBm >= -72) return "var(--warn)";
        return "var(--bad)";
    }

    function selectChannel(ch) {
        selectedChannel = ch;
        userSelectedBand = spectrumBand;
    }

    function switchBand(band) {
        spectrumBand = band;
        userSelectedBand = null;
        selectedChannel = null;
    }

    function spectrumTrunc(text, n) {
        if (!text) return "";
        return text.length > n ? text.slice(0, n - 1) + "…" : text;
    }

    function showChannelLabel(ch, idx, totalCount, selected) {
        if (ch === selected) return true;
        if (totalCount <= 14) return true;
        return idx % Math.ceil(totalCount / 10) === 0;
    }

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
    onDestroy(() => spectrumRO?.disconnect());

    $: bandAPsSorted = [...bandAPs].sort((a, b) => a.signal - b.signal);

    // ── Channel detail panel computations ───────────────────
    $: detail = computeDetail(
        selectedChannel,
        spectrumBand,
        bandDef,
        apsOnSelected,
        apsOverlapSelected,
        connectedAP,
    );

    function computeDetail(ch, band, def, aps, overlap, connected) {
        if (ch == null || !def) return null;
        const totalAps = aps.length;
        const isDfs = def.dfs.includes(ch);
        const isNonOverlap = def.nonOverlap.includes(ch);
        const strongest = aps.length
            ? Math.max(...aps.map((a) => a.signal))
            : null;
        const avg = aps.length
            ? Math.round(
                  aps.reduce((s, a) => s + a.signal, 0) / aps.length,
              )
            : null;
        const isConnected =
            connected && apBand(connected) === band && connected.channel === ch;

        const effectiveCrowd = totalAps + overlap.length * 0.5;
        let quality, qualityTone, qualityExplain;
        if (effectiveCrowd === 0) {
            quality = "Excellent";
            qualityTone = "ok";
            qualityExplain = "Channel is empty. Ideal for a new network.";
        } else if (effectiveCrowd <= 1.5) {
            quality = "Good";
            qualityTone = "ok";
            qualityExplain =
                "Low contention. Throughput should be close to link rate.";
        } else if (effectiveCrowd <= 3) {
            quality = "Fair";
            qualityTone = "warn";
            qualityExplain =
                "Moderate contention. Expect some airtime sharing.";
        } else {
            quality = "Poor";
            qualityTone = "bad";
            qualityExplain =
                "High contention. Consider moving to a quieter channel.";
        }

        const utilization = Math.min(
            95,
            Math.round(
                effectiveCrowd * 22 +
                    (strongest != null ? (strongest + 90) / 2 : 0),
            ),
        );

        let advice;
        if (qualityTone === "ok") {
            advice = isConnected
                ? "You're on a clean channel. No action needed."
                : "Good candidate for a new network.";
        } else if (qualityTone === "warn") {
            advice = isConnected
                ? "Consider roaming to a quieter channel if performance degrades."
                : "Workable but not ideal. Prefer a cleaner channel.";
        } else {
            advice =
                "Avoid this channel. Crowded spectrum will hurt throughput.";
        }
        if (isDfs) advice += " · DFS: radar detection may cause periodic pauses.";
        if (band === "2.4GHz" && !isNonOverlap)
            advice += " · Non-20MHz-aligned; prefer 1, 6, or 11.";

        return {
            ch,
            band,
            centerFreq: def.freqOf(ch).toFixed(3),
            widthDefault: def.widthDefault,
            totalAps,
            overlapCount: overlap.length,
            strongest,
            avg,
            isConnected,
            isDfs,
            isNonOverlap,
            quality,
            qualityTone,
            qualityExplain,
            utilization,
            advice,
        };
    }

    function utilColor(util) {
        if (util < 30) return "var(--ok)";
        if (util < 60) return "var(--warn)";
        return "var(--bad)";
    }

    function signalText(dBm) {
        if (typeof dBm !== "number") return "—";
        return `${dBm} dBm`;
    }
</script>

<div class="channel-analyzer-container">
    <!-- Toolbar: band tabs + KPI chips -->
    <div class="channel-toolbar">
        <div class="segmented" role="tablist" aria-label="Frequency band">
            {#each ["2.4GHz", "5GHz", "6GHz"] as band}
                <button
                    type="button"
                    role="tab"
                    aria-selected={spectrumBand === band}
                    class:active={spectrumBand === band}
                    on:click={() => switchBand(band)}
                >{band}</button>
            {/each}
        </div>
        <div class="toolbar-spacer"></div>
        <span class="kpi-chip">
            <span class="kpi-label">APs</span>
            <span class="kpi-value">{bandAPs.length}</span>
        </span>
        <span class="kpi-chip">
            <span class="kpi-label">Active</span>
            <span class="kpi-value"
                >{activeChannels}/{bandDef?.channels.length ?? 0}</span
            >
        </span>
        {#if busiestEntry}
            <span class="kpi-chip warn">
                Busiest ch {busiestEntry[0]} · {busiestEntry[1]}
            </span>
        {/if}
        {#if cleanestChannel}
            <span class="kpi-chip ok">
                Cleanest ch {cleanestChannel.ch}{cleanestChannel.aps
                    ? ` · ${cleanestChannel.aps}`
                    : ""}
            </span>
        {/if}
    </div>

    <!-- Spectrum bell-curve chart -->
    <div class="spectrum-panel">
        <div class="spectrum-header">
            <div>
                <div class="spectrum-title">{bandDef?.title} spectrum</div>
                <div class="spectrum-sub">{bandDef?.subtitle}</div>
            </div>
            <div class="spectrum-spacer"></div>
            <div class="spectrum-legend">
                <span class="leg-dot" style="background: var(--acc-1)"></span
                >Connected
                <span class="leg-dot" style="background: var(--ok)"></span
                >Strong
                <span class="leg-dot" style="background: var(--warn)"></span
                >Fair
                <span class="leg-dot" style="background: var(--bad)"></span
                >Weak
            </div>
        </div>

        <div class="spectrum-svg-wrapper" bind:this={spectrumWrapper}>
            {#if bandDef}
                {@const innerW =
                    spectrumWidth - SPECTRUM_PAD.left - SPECTRUM_PAD.right}
                {@const innerH =
                    SPECTRUM_HEIGHT - SPECTRUM_PAD.top - SPECTRUM_PAD.bottom}
                {@const tickW = spectrumWidthPx(20)}
                {@const primaryW = spectrumWidthPx(20)}
                {@const baseY = SPECTRUM_PAD.top + innerH}
                <svg
                    class="spectrum-svg"
                    width={spectrumWidth}
                    height={SPECTRUM_HEIGHT}
                    viewBox={`0 0 ${spectrumWidth} ${SPECTRUM_HEIGHT}`}
                    preserveAspectRatio="none"
                >
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

                    {#each UNII_BANDS[spectrumBand] || [] as uband}
                        {@const ubx1 = Math.max(
                            SPECTRUM_PAD.left,
                            spectrumXForFreqGHz(uband.startGHz),
                        )}
                        {@const ubx2 = Math.min(
                            spectrumWidth - SPECTRUM_PAD.right,
                            spectrumXForFreqGHz(uband.endGHz),
                        )}
                        {#if ubx2 > ubx1 + 4}
                            <line
                                x1={ubx1 + 2}
                                x2={ubx2 - 2}
                                y1={SPECTRUM_PAD.top - 14}
                                y2={SPECTRUM_PAD.top - 14}
                                stroke="var(--line-2)"
                                stroke-width="1"
                            />
                            <line
                                x1={ubx1 + 2}
                                x2={ubx1 + 2}
                                y1={SPECTRUM_PAD.top - 17}
                                y2={SPECTRUM_PAD.top - 11}
                                stroke="var(--line-2)"
                                stroke-width="1"
                            />
                            <line
                                x1={ubx2 - 2}
                                x2={ubx2 - 2}
                                y1={SPECTRUM_PAD.top - 17}
                                y2={SPECTRUM_PAD.top - 11}
                                stroke="var(--line-2)"
                                stroke-width="1"
                            />
                            <text
                                x={(ubx1 + ubx2) / 2}
                                y={SPECTRUM_PAD.top - 18}
                                fill="var(--fg-3)"
                                font-size="10"
                                font-family="var(--font-mono)"
                                text-anchor="middle"
                            >{uband.label}</text>
                        {/if}
                    {/each}

                    {#if selectedChannel != null}
                        {@const selX = spectrumXForChannel(selectedChannel)}
                        <rect
                            x={selX - tickW / 2}
                            y={SPECTRUM_PAD.top}
                            width={tickW}
                            height={innerH}
                            fill="var(--acc-1)"
                            opacity="0.08"
                        />
                        <line
                            x1={selX}
                            x2={selX}
                            y1={SPECTRUM_PAD.top}
                            y2={baseY}
                            stroke="var(--acc-1)"
                            stroke-dasharray="3,3"
                            stroke-width="1"
                            opacity="0.6"
                        />
                    {/if}

                    {#each bandDef.dfs as dfsCh}
                        <line
                            x1={spectrumXForChannel(dfsCh)}
                            x2={spectrumXForChannel(dfsCh)}
                            y1={baseY - 2}
                            y2={baseY + 4}
                            stroke="#a78bfa"
                            stroke-width="2"
                        />
                    {/each}

                    {#each bandAPsSorted as ap (ap.bssid + ap.channel)}
                        {@const apW = ap.channelWidth || bandDef.widthDefault}
                        {@const centerCh = actualCenterChannel(
                            ap.channel,
                            apW,
                            spectrumBand,
                        )}
                        {@const cx = spectrumXForChannel(centerCh)}
                        {@const cy = spectrumYForSignal(ap.signal)}
                        {@const wPx = spectrumWidthPx(apW)}
                        {@const primaryX = spectrumXForChannel(ap.channel)}
                        {@const isConn =
                            connectedBSSID &&
                            ap.bssid &&
                            ap.bssid.toLowerCase() === connectedBSSID}
                        {@const color = colorForSignal(ap.signal, isConn)}
                        {@const rectH = Math.max(2, baseY - cy)}
                        <rect
                            x={cx - wPx / 2}
                            y={cy}
                            width={wPx}
                            height={rectH}
                            rx="2"
                            ry="2"
                            fill={color}
                            fill-opacity={isConn ? 0.18 : 0.10}
                            stroke={color}
                            stroke-width={isConn ? 1.5 : 1}
                            stroke-opacity={isConn ? 0.85 : 0.55}
                        />
                        <rect
                            x={primaryX - primaryW / 2}
                            y={cy}
                            width={primaryW}
                            height={rectH}
                            fill={color}
                            fill-opacity={isConn ? 0.45 : 0.32}
                            stroke="none"
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

                    <line
                        x1={SPECTRUM_PAD.left}
                        y1={baseY}
                        x2={spectrumWidth - SPECTRUM_PAD.right}
                        y2={baseY}
                        stroke="var(--line-2)"
                    />

                    {#each bandDef.channels as ch, idx}
                        {@const x = spectrumXForChannel(ch)}
                        <!-- svelte-ignore a11y-click-events-have-key-events -->
                        <g
                            class="spectrum-tick"
                            on:click={() => selectChannel(ch)}
                            style="cursor: pointer"
                        >
                            <rect
                                x={x - tickW / 2}
                                y={SPECTRUM_PAD.top}
                                width={tickW}
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
                            {#if showChannelLabel(ch, idx, bandDef.channels.length, selectedChannel)}
                                <text
                                    x={x}
                                    y={baseY + 16}
                                    fill={ch === selectedChannel
                                        ? "var(--acc-1)"
                                        : "var(--fg-2)"}
                                    font-size="10"
                                    font-family="var(--font-mono)"
                                    font-weight={ch === selectedChannel
                                        ? 600
                                        : 400}
                                    text-anchor="middle"
                                >{ch}</text>
                            {/if}
                        </g>
                    {/each}

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
            {#if bandAPs.length === 0}
                <div class="spectrum-empty">
                    No APs detected on the {spectrumBand} band yet — start a
                    scan or switch bands.
                </div>
            {/if}
        </div>
    </div>

    <!-- Detail + AP list for selected channel -->
    {#if detail}
        <div class="detail-grid">
            <div class="panel detail-panel">
                <div class="panel-header">
                    <div class="ch-badge-block">
                        <div
                            class="ch-badge"
                            class:connected={detail.isConnected}
                        >
                            {detail.ch}
                        </div>
                        <div>
                            <div class="ch-title">
                                Channel {detail.ch}
                                {#if detail.isConnected}
                                    <span class="chip acc">Connected</span>
                                {/if}
                            </div>
                            <div class="ch-sub">
                                {detail.band} · {detail.centerFreq} GHz · {detail.widthDefault}
                                MHz default
                            </div>
                        </div>
                    </div>
                    <div class="panel-spacer"></div>
                    <div class="ch-flags">
                        {#if detail.isDfs}
                            <span class="chip warn">DFS</span>
                        {/if}
                        {#if detail.isNonOverlap}
                            <span class="chip ok">Non-overlapping</span>
                        {/if}
                    </div>
                </div>
                <div class="panel-body">
                    <div class="quality-card">
                        <div class="quality-row">
                            <span class="metric-label">Channel quality</span>
                            <span class="chip {detail.qualityTone}"
                                >{detail.quality}</span
                            >
                        </div>
                        <div class="quality-explain">
                            {detail.qualityExplain}
                        </div>
                        <div class="util-row">
                            <span class="util-caption">Estimated airtime</span>
                            <span
                                class="util-value mono"
                                style="color: {utilColor(detail.utilization)}"
                                >{detail.utilization}%</span
                            >
                        </div>
                        <div class="util-track">
                            <span
                                style="width: {detail.utilization}%; background: {utilColor(
                                    detail.utilization,
                                )};"
                            ></span>
                        </div>
                    </div>

                    <div class="stat-grid">
                        <div class="mini-stat">
                            <div class="metric-label">APs on channel</div>
                            <div class="mini-value mono">{detail.totalAps}</div>
                        </div>
                        <div class="mini-stat">
                            <div class="metric-label">Overlapping APs</div>
                            <div
                                class="mini-value mono"
                                class:warn={detail.overlapCount > 2}
                            >{detail.overlapCount}</div>
                        </div>
                        <div class="mini-stat">
                            <div class="metric-label">Strongest</div>
                            <div class="mini-value mono">
                                {signalText(detail.strongest)}
                            </div>
                        </div>
                        <div class="mini-stat">
                            <div class="metric-label">Average</div>
                            <div class="mini-value mono">
                                {signalText(detail.avg)}
                            </div>
                        </div>
                    </div>

                    <div class="advice {detail.qualityTone}">
                        <div class="advice-label">Advice</div>
                        <div class="advice-body">{detail.advice}</div>
                    </div>
                </div>
            </div>

            <div class="panel ap-list-panel">
                <div class="panel-header">
                    <div>
                        <div class="panel-title">Access points</div>
                        <div class="ch-sub">
                            {apsOnSelected.length} on ch {detail.ch}{#if apsOverlapSelected.length}
                                · {apsOverlapSelected.length} overlapping
                            {/if}
                        </div>
                    </div>
                </div>
                <div class="panel-body ap-table-wrap">
                    <table class="ap-table">
                        <thead>
                            <tr>
                                <th>SSID</th>
                                <th class="num">Signal</th>
                                <th class="num">Width</th>
                                <th>Security</th>
                            </tr>
                        </thead>
                        <tbody>
                            {#if apsOnSelected.length === 0 && apsOverlapSelected.length === 0}
                                <tr
                                    ><td colspan="4" class="empty"
                                        >No APs detected on this channel</td
                                    ></tr
                                >
                            {/if}
                            {#if apsOnSelected.length > 0}
                                <tr class="section-row"
                                    ><td colspan="4">On this channel</td></tr
                                >
                                {#each apsOnSelected as ap (ap.bssid)}
                                    <tr>
                                        <td>
                                            <div class="ap-ssid-row">
                                                <span class="ap-ssid"
                                                    >{ap.ssid || "<Hidden>"}</span
                                                >
                                                {#if connectedBSSID && ap.bssid && ap.bssid.toLowerCase() === connectedBSSID}
                                                    <span class="chip acc"
                                                        >Connected</span
                                                    >
                                                {/if}
                                            </div>
                                            <div class="ap-bssid mono">
                                                {ap.bssid}
                                            </div>
                                        </td>
                                        <td class="num mono">{ap.signal} dBm</td>
                                        <td class="num mono"
                                            >{ap.channelWidth ||
                                                bandDef.widthDefault} MHz</td
                                        >
                                        <td class="mono"
                                            >{ap.security || "—"}</td
                                        >
                                    </tr>
                                {/each}
                            {/if}
                            {#if apsOverlapSelected.length > 0}
                                <tr class="section-row warn"
                                    ><td colspan="4"
                                        >Overlapping interference</td
                                    ></tr
                                >
                                {#each apsOverlapSelected as ap (ap.bssid)}
                                    <tr class="dim">
                                        <td>
                                            <div class="ap-ssid-row">
                                                <span class="ap-ssid"
                                                    >{ap.ssid || "<Hidden>"}</span
                                                >
                                            </div>
                                            <div class="ap-bssid mono">
                                                {ap.bssid} · ch {ap.channel}
                                            </div>
                                        </td>
                                        <td class="num mono">{ap.signal} dBm</td>
                                        <td class="num mono"
                                            >{ap.channelWidth ||
                                                bandDef.widthDefault} MHz</td
                                        >
                                        <td class="mono"
                                            >{ap.security || "—"}</td
                                        >
                                    </tr>
                                {/each}
                            {/if}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    {/if}
</div>

<style>
    .channel-analyzer-container {
        min-height: 100%;
        padding: 16px;
        padding-bottom: 72px;
        display: flex;
        flex-direction: column;
        gap: 12px;
        background: linear-gradient(180deg, var(--bg-0) 0%, var(--bg-1) 100%);
        color: var(--text);
    }

    .channel-toolbar {
        display: flex;
        align-items: center;
        gap: 10px;
        flex-wrap: wrap;
    }

    .toolbar-spacer {
        flex: 1;
    }

    .segmented {
        display: inline-flex;
        background: var(--bg-3);
        border: 1px solid var(--border-strong, var(--border));
        border-radius: 6px;
        padding: 2px;
        gap: 2px;
    }

    .segmented button {
        background: transparent;
        border: none;
        color: var(--muted);
        font-size: 12px;
        padding: 5px 14px;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
        font-family: inherit;
    }

    .segmented button.active {
        background: var(--bg-1);
        color: var(--text);
        box-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    }

    .segmented button:hover:not(.active) {
        color: var(--text);
    }

    .kpi-chip {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 4px 10px;
        border-radius: 999px;
        font-size: 11px;
        background: var(--bg-3);
        color: var(--muted);
        border: 1px solid var(--border);
    }

    .kpi-label {
        text-transform: uppercase;
        letter-spacing: 0.06em;
        font-size: 10px;
    }

    .kpi-value {
        color: var(--text);
        font-weight: 600;
    }

    .kpi-chip.warn {
        color: var(--warn);
        border-color: var(--warn-line);
        background: var(--warn-bg);
    }

    .kpi-chip.ok {
        color: var(--ok);
        border-color: var(--ok-line);
        background: var(--ok-bg);
    }

    .panel {
        background: var(--bg-2);
        border: 1px solid var(--line-1, var(--border));
        border-radius: 8px;
        overflow: hidden;
        display: flex;
        flex-direction: column;
        min-height: 0;
    }

    .panel-header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 14px;
        border-bottom: 1px solid var(--line-1, var(--border));
        flex-wrap: wrap;
    }

    .panel-spacer {
        flex: 1;
    }

    .panel-title {
        font-size: 12px;
        font-weight: 600;
        color: var(--fg-1, var(--text));
    }

    .panel-body {
        padding: 14px;
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    /* ── Spectrum panel ─────────────────────────────────────── */
    .spectrum-panel {
        background: var(--bg-2);
        border: 1px solid var(--line-1, var(--border));
        border-radius: 8px;
        overflow: hidden;
    }

    .spectrum-header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 16px;
        border-bottom: 1px solid var(--line-1, var(--border));
        flex-wrap: wrap;
    }

    .spectrum-title {
        font-size: 12px;
        font-weight: 600;
        color: var(--fg-1, var(--text));
    }

    .spectrum-sub {
        font-size: 11px;
        color: var(--fg-3, var(--muted-2));
        margin-top: 2px;
        max-width: 60ch;
    }

    .spectrum-spacer {
        flex: 1;
    }

    .spectrum-legend {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 11px;
        color: var(--fg-3, var(--muted-2));
        font-family: var(--font-mono, ui-monospace, monospace);
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
        color: var(--fg-3, var(--muted-2));
        font-size: 12px;
        pointer-events: none;
    }

    /* ── Detail + AP list grid ──────────────────────────────── */
    .detail-grid {
        display: grid;
        grid-template-columns: 1fr 1.2fr;
        gap: 12px;
        min-height: 0;
    }

    .ch-badge-block {
        display: flex;
        align-items: center;
        gap: 10px;
    }

    .ch-badge {
        width: 38px;
        height: 38px;
        border-radius: 8px;
        background: var(--bg-3);
        border: 1px solid var(--line-2, var(--border-strong, var(--border)));
        color: var(--text);
        font-family: var(--font-mono, ui-monospace, monospace);
        font-size: 14px;
        font-weight: 600;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .ch-badge.connected {
        background: var(--acc-1-bg);
        border-color: var(--acc-1-line);
        color: var(--acc-1);
    }

    .ch-title {
        font-size: 13px;
        font-weight: 600;
        color: var(--text);
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .ch-sub {
        font-size: 11px;
        color: var(--fg-3, var(--muted-2));
        font-family: var(--font-mono, ui-monospace, monospace);
        margin-top: 2px;
    }

    .ch-flags {
        display: flex;
        gap: 6px;
    }

    .chip {
        padding: 2px 8px;
        border-radius: 999px;
        font-size: 10px;
        letter-spacing: 0.06em;
        text-transform: uppercase;
        font-weight: 600;
        background: var(--bg-3);
        color: var(--muted);
        border: 1px solid var(--border);
    }

    .chip.acc {
        background: var(--acc-1-bg);
        color: var(--acc-1);
        border-color: var(--acc-1-line);
    }

    .chip.ok {
        background: var(--ok-bg);
        color: var(--ok);
        border-color: var(--ok-line);
    }

    .chip.warn {
        background: var(--warn-bg);
        color: var(--warn);
        border-color: var(--warn-line);
    }

    .chip.bad {
        background: var(--bad-bg);
        color: var(--bad);
        border-color: var(--bad-line);
    }

    .quality-card {
        padding: 12px;
        border-radius: 6px;
        background: var(--bg-3);
        border: 1px solid var(--line-1, var(--border));
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .quality-row {
        display: flex;
        align-items: baseline;
        gap: 8px;
    }

    .metric-label {
        font-size: 10px;
        color: var(--fg-3, var(--muted-2));
        text-transform: uppercase;
        letter-spacing: 0.1em;
        font-weight: 600;
    }

    .quality-explain {
        font-size: 12px;
        color: var(--fg-2, var(--muted));
        line-height: 1.5;
    }

    .util-row {
        display: flex;
        justify-content: space-between;
        align-items: baseline;
        font-size: 11px;
    }

    .util-caption {
        color: var(--fg-3, var(--muted-2));
        text-transform: uppercase;
        letter-spacing: 0.1em;
        font-weight: 600;
        font-size: 10px;
    }

    .util-value {
        font-weight: 500;
    }

    .util-track {
        height: 6px;
        background: var(--bg-4, var(--panel-strong));
        border-radius: 3px;
        overflow: hidden;
    }

    .util-track span {
        display: block;
        height: 100%;
    }

    .stat-grid {
        display: grid;
        grid-template-columns: repeat(2, 1fr);
        gap: 8px;
    }

    .mini-stat {
        padding: 8px 12px;
        border-radius: 5px;
        background: var(--bg-3);
        border: 1px solid var(--line-1, var(--border));
    }

    .mini-value {
        font-size: 15px;
        font-weight: 500;
        color: var(--text);
        margin-top: 2px;
    }

    .mini-value.warn {
        color: var(--warn);
    }

    .advice {
        padding: 10px;
        border-radius: 6px;
        display: flex;
        gap: 10px;
        align-items: flex-start;
    }

    .advice.ok {
        background: var(--ok-bg);
        border: 1px solid var(--ok-line);
    }

    .advice.warn {
        background: var(--warn-bg);
        border: 1px solid var(--warn-line);
    }

    .advice.bad {
        background: var(--bad-bg);
        border: 1px solid var(--bad-line);
    }

    .advice-label {
        font-size: 11px;
        text-transform: uppercase;
        letter-spacing: 0.1em;
        font-weight: 600;
        min-width: 64px;
    }

    .advice.ok .advice-label {
        color: var(--ok);
    }

    .advice.warn .advice-label {
        color: var(--warn);
    }

    .advice.bad .advice-label {
        color: var(--bad);
    }

    .advice-body {
        font-size: 11.5px;
        color: var(--text);
        line-height: 1.5;
    }

    /* ── AP list table ──────────────────────────────────────── */
    .ap-table-wrap {
        padding: 0;
        overflow: auto;
        max-height: 480px;
    }

    .ap-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 12px;
    }

    .ap-table th,
    .ap-table td {
        padding: 8px 12px;
        text-align: left;
        border-bottom: 1px solid var(--line-1, var(--border));
    }

    .ap-table th {
        position: sticky;
        top: 0;
        background: var(--bg-2);
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.08em;
        color: var(--muted);
        font-weight: 600;
        z-index: 1;
    }

    .ap-table td.num,
    .ap-table th.num {
        text-align: right;
        white-space: nowrap;
    }

    .ap-table tr.dim {
        opacity: 0.75;
    }

    .ap-table tr.section-row td {
        padding: 4px 12px;
        font-size: 9.5px;
        font-weight: 600;
        color: var(--fg-3, var(--muted-2));
        text-transform: uppercase;
        letter-spacing: 0.12em;
        background: var(--bg-1, var(--panel));
    }

    .ap-table tr.section-row.warn td {
        color: var(--warn);
    }

    .ap-table td.empty {
        text-align: center;
        color: var(--muted);
        padding: 18px 12px;
    }

    .ap-ssid-row {
        display: flex;
        align-items: center;
        gap: 6px;
    }

    .ap-ssid {
        font-weight: 500;
        color: var(--text);
    }

    .ap-bssid {
        font-size: 10px;
        color: var(--fg-3, var(--muted-2));
        margin-top: 2px;
    }

    .mono {
        font-family: var(--font-mono, ui-monospace, monospace);
    }

    @media (max-width: 1100px) {
        .detail-grid {
            grid-template-columns: 1fr;
        }
    }

    @media (max-width: 768px) {
        .channel-analyzer-container {
            padding: 12px;
        }
    }
</style>
