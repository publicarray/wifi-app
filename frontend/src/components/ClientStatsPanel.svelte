<script>
    /** @typedef {import('../../wailsjs/go/models').main.ClientStats} ClientStats */
    /** @type {ClientStats | null} */
    export let clientStats = null;
    export let networks = [];

    import { isNumber, formatBytes, formatCount, signalTone } from "../utils.js";

    function formatDuration(seconds) {
        if (!isNumber(seconds)) return "—";
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        const secs = seconds % 60;
        if (hours > 0) return `${hours}h ${minutes}m ${secs}s`;
        if (minutes > 0) return `${minutes}m ${secs}s`;
        return `${secs}s`;
    }

    function formatFrequency(mhz) {
        return isNumber(mhz) ? `${(mhz / 1000).toFixed(3)} GHz` : "—";
    }

    function bandFromFreq(mhz) {
        if (!isNumber(mhz)) return "";
        if (mhz >= 5925) return "6 GHz";
        if (mhz >= 4900) return "5 GHz";
        if (mhz > 0) return "2.4 GHz";
        return "";
    }

    // ── Quality scoring ─────────────────────────────────────
    // Map a signal in dBm onto a 0-100 fill (so the meter bar reads roughly
    // "how close to the top of the usable range").
    function signalPct(dBm) {
        if (!isNumber(dBm)) return 0;
        const clamped = Math.max(-90, Math.min(-30, dBm));
        return Math.round(((clamped + 90) / 60) * 100);
    }



    function snrPct(snr) {
        if (!isNumber(snr) || snr <= 0) return 0;
        return Math.round(Math.min(100, (snr / 50) * 100));
    }

    function snrTone(snr) {
        if (!isNumber(snr) || snr <= 0) return "muted";
        if (snr >= 25) return "ok";
        if (snr >= 15) return "warn";
        return "bad";
    }

    // Retry rate is "lower is better" — invert the bar so green = healthy.
    function retryPct(rate) {
        if (!isNumber(rate)) return 0;
        return Math.round(Math.max(0, Math.min(100, 100 - rate)));
    }

    function retryTone(rate) {
        if (!isNumber(rate)) return "muted";
        if (rate < 5) return "ok";
        if (rate < 10) return "warn";
        return "bad";
    }

    // Worst signal in the last 5 minutes from signalHistory.
    function worstSignal(history) {
        if (!history || history.length === 0) return null;
        const cutoff = Date.now() - 5 * 60 * 1000;
        let worst = null;
        for (const p of history) {
            const t =
                typeof p.timestamp === "number"
                    ? p.timestamp
                    : Date.parse(p.timestamp || "");
            if (Number.isNaN(t) || t < cutoff) continue;
            const v = typeof p.signal === "number" ? p.signal : null;
            if (v == null) continue;
            if (worst == null || v < worst) worst = v;
        }
        return worst;
    }

    function lastAckAge(history) {
        if (!history || history.length === 0) return null;
        const last = history[history.length - 1];
        const t =
            typeof last.timestamp === "number"
                ? last.timestamp
                : Date.parse(last.timestamp || "");
        if (Number.isNaN(t)) return null;
        const sec = Math.max(0, Math.round((Date.now() - t) / 1000));
        if (sec < 60) return `${sec}s ago`;
        if (sec < 3600) return `${Math.round(sec / 60)}m ago`;
        return `${Math.round(sec / 3600)}h ago`;
    }

    $: connected = !!(clientStats && clientStats.connected);

    $: bandLabel = bandFromFreq(clientStats?.frequency);

    $: worst5m = worstSignal(clientStats?.signalHistory);
    $: ackAge = lastAckAge(clientStats?.signalHistory);

    // Section visibility — only render Roaming if we have events.
    $: hasRoaming =
        clientStats?.roamingHistory && clientStats.roamingHistory.length > 0;

    // ── Connected AP lookup ─────────────────────────────────
    // The Advanced capabilities panel pulls from the AccessPoint that matches
    // the current BSSID. ClientStats only has client-side fields; AP-side
    // capabilities (BSS Color, FT/k/v, MU-MIMO, etc.) live on AccessPoint.
    function findConnectedAP(allNetworks, bssid) {
        if (!bssid) return null;
        const target = bssid.toLowerCase();
        for (const network of allNetworks || []) {
            for (const ap of network.accessPoints || []) {
                if (ap?.bssid && ap.bssid.toLowerCase() === target) {
                    return ap;
                }
            }
        }
        return null;
    }

    $: connectedAP = findConnectedAP(networks, clientStats?.bssid);

    // Tone helpers for capability rows: each row maps {Supported / Not
    // supported / value} into ok/bad/neutral chips so the panel scans quickly.
    function supportChip(flag) {
        if (flag === true) return { tone: "ok", text: "Supported" };
        if (flag === false) return { tone: "bad", text: "Not supported" };
        return null;
    }

    function formatPhyRate(mbps) {
        if (!isNumber(mbps) || mbps <= 0) return "—";
        return `${mbps} Mbps`;
    }

    // formatMimo prefers the AP's negotiated stream count (parsed from HT/VHT
    // /HE IEs) over the per-frame mimoConfig string. The latter is the NSS of
    // the most recent TX rate — it ticks down to 1×1 for short management
    // frames even on a 4×4 link, so it's misleading as a "negotiated MIMO"
    // display. The IE-derived count is the negotiated upper bound and is what
    // the user expects to see in an Advanced Capabilities panel.
    function formatMimo(streams, mimoConfig) {
        const apStreams =
            isNumber(streams) && streams > 0 ? streams : null;
        const liveStreams = parseStreams(mimoConfig);
        const max = apStreams ?? liveStreams;
        if (max == null) return "—";
        if (
            liveStreams != null &&
            apStreams != null &&
            liveStreams !== apStreams
        ) {
            return `${max}×${max} · live ${liveStreams}ss`;
        }
        return `${max}×${max} · ${max} streams`;
    }

    // mimoConfig arrives as "1x1" / "2x2" / "4x4" — extract the leading number.
    function parseStreams(mimoConfig) {
        if (!mimoConfig) return null;
        const m = /^(\d+)/.exec(mimoConfig);
        if (!m) return null;
        const v = parseInt(m[1], 10);
        if (!Number.isFinite(v) || v <= 0 || v > 8) return null;
        return v;
    }

    function formatQam(qam) {
        if (!isNumber(qam) || qam <= 0) return null;
        return `${qam}-QAM`;
    }

    function formatUtilization(util) {
        if (!isNumber(util) || util < 0) return null;
        return `${util}%`;
    }

    // PMF status maps to a tone: Required = ok, Optional = warn, Disabled = bad.
    function pmfTone(pmf) {
        if (!pmf) return null;
        const v = String(pmf).toLowerCase();
        if (v.includes("required")) return "ok";
        if (v.includes("optional") || v.includes("capable")) return "warn";
        if (v.includes("disabled") || v.includes("none")) return "bad";
        return null;
    }

    function joinCiphers(list) {
        if (!Array.isArray(list) || list.length === 0) return "";
        return list.join(" · ");
    }
</script>

<div class="client-stats-panel">
    {#if connected}
        <!-- Top: connection summary + signal quality side-by-side -->
        <div class="top-grid">
            <div class="panel">
                <div class="panel-header">
                    <div class="panel-title">Connection</div>
                    <div class="panel-spacer"></div>
                    {#if isNumber(clientStats.connectedTime)}
                        <span class="chip ok"
                            >Up {formatDuration(clientStats.connectedTime)}</span
                        >
                    {/if}
                </div>
                <div class="panel-body">
                    <div class="ssid-row">
                        <span class="ssid">{clientStats.ssid || "—"}</span>
                        {#if clientStats.wifiStandard}
                            <span class="chip acc"
                                >{clientStats.wifiStandard}</span
                            >
                        {/if}
                    </div>
                    <div class="stat-list">
                        <div class="stat-row">
                            <span class="k">BSSID</span>
                            <span class="v mono"
                                >{clientStats.bssid || "—"}</span
                            >
                        </div>
                        <div class="stat-row">
                            <span class="k">Interface</span>
                            <span class="v mono"
                                >{clientStats.interface || "—"}</span
                            >
                        </div>
                        <div class="stat-row">
                            <span class="k">Band</span>
                            <span class="v mono">
                                {#if bandLabel && isNumber(clientStats.channel)}
                                    {bandLabel} · ch {clientStats.channel}
                                    {#if isNumber(clientStats.channelWidth)}
                                        ({clientStats.channelWidth} MHz)
                                    {/if}
                                {:else}
                                    —
                                {/if}
                            </span>
                        </div>
                        <div class="stat-row">
                            <span class="k">Frequency</span>
                            <span class="v mono"
                                >{formatFrequency(clientStats.frequency)}</span
                            >
                        </div>
                        {#if connectedAP?.mimoStreams || clientStats.mimoConfig}
                            <div class="stat-row">
                                <span class="k">MIMO</span>
                                <span class="v mono"
                                    >{formatMimo(
                                        connectedAP?.mimoStreams,
                                        clientStats.mimoConfig,
                                    )}</span
                                >
                            </div>
                        {/if}
                        <div class="stat-row">
                            <span class="k">Security</span>
                            <span class="v mono">
                                {#if connectedAP?.security}
                                    {connectedAP.security}{#if joinCiphers(connectedAP.securityCiphers)}
                                        · {joinCiphers(
                                            connectedAP.securityCiphers,
                                        )}
                                    {/if}{#if connectedAP.pmf}
                                        · PMF {connectedAP.pmf}
                                    {/if}
                                {:else}
                                    —
                                {/if}
                            </span>
                        </div>
                        <div class="stat-row">
                            <span class="k">Vendor</span>
                            <span class="v mono"
                                >{connectedAP?.vendor || "—"}</span
                            >
                        </div>
                        <div class="stat-row">
                            <span class="k">IP / Gateway</span>
                            <span class="v mono">
                                {clientStats.localIp || "—"} /
                                {clientStats.gateway || "—"}
                            </span>
                        </div>
                    </div>
                </div>
            </div>

            <div class="panel">
                <div class="panel-header">
                    <div>
                        <div class="panel-title">Signal quality</div>
                        <div class="panel-sub">Live · sampled per scan</div>
                    </div>
                </div>
                <div class="panel-body">
                    <div class="meter-grid">
                        <div class="meter">
                            <div class="metric-label">Current</div>
                            <div class="meter-value-row">
                                <span
                                    class="meter-value mono tone-{signalTone(
                                        clientStats.signal,
                                    )}"
                                    >{isNumber(clientStats.signal)
                                        ? `${clientStats.signal} dBm`
                                        : "—"}</span
                                >
                            </div>
                            <div class="bar-track">
                                <div
                                    class="bar-fill tone-{signalTone(
                                        clientStats.signal,
                                    )}"
                                    style="width: {signalPct(
                                        clientStats.signal,
                                    )}%"
                                ></div>
                            </div>
                        </div>

                        <div class="meter">
                            <div class="metric-label">Average</div>
                            <div class="meter-value-row">
                                <span
                                    class="meter-value mono tone-{signalTone(
                                        clientStats.signalAvg,
                                    )}"
                                    >{isNumber(clientStats.signalAvg) &&
                                    clientStats.signalAvg !== 0
                                        ? `${clientStats.signalAvg} dBm`
                                        : "—"}</span
                                >
                            </div>
                            <div class="bar-track">
                                <div
                                    class="bar-fill tone-{signalTone(
                                        clientStats.signalAvg,
                                    )}"
                                    style="width: {signalPct(
                                        clientStats.signalAvg,
                                    )}%"
                                ></div>
                            </div>
                        </div>

                        <div class="meter">
                            <div class="metric-label">Worst</div>
                            <div class="meter-value-row">
                                <span
                                    class="meter-value mono tone-{signalTone(
                                        worst5m,
                                    )}"
                                    >{worst5m != null
                                        ? `${worst5m} dBm`
                                        : "—"}</span
                                >
                                <span class="meter-sub mono">5m</span>
                            </div>
                            <div class="bar-track">
                                <div
                                    class="bar-fill tone-{signalTone(worst5m)}"
                                    style="width: {signalPct(worst5m)}%"
                                ></div>
                            </div>
                        </div>

                        <div class="meter">
                            <div class="metric-label">SNR</div>
                            <div class="meter-value-row">
                                <span
                                    class="meter-value mono tone-{snrTone(
                                        clientStats.snr,
                                    )}"
                                    >{isNumber(clientStats.snr) &&
                                    clientStats.snr > 0
                                        ? `${clientStats.snr} dB`
                                        : "—"}</span
                                >
                                <span class="meter-sub mono"
                                    >{isNumber(clientStats.noise) &&
                                    clientStats.noise < 0
                                        ? `noise ${clientStats.noise} dBm`
                                        : "noise n/a"}</span
                                >
                            </div>
                            <div class="bar-track">
                                <div
                                    class="bar-fill tone-{snrTone(
                                        clientStats.snr,
                                    )}"
                                    style="width: {snrPct(clientStats.snr)}%"
                                ></div>
                            </div>
                        </div>

                        <div class="meter">
                            <div class="metric-label">Last ACK</div>
                            <div class="meter-value-row">
                                <span
                                    class="meter-value mono tone-{signalTone(
                                        clientStats.lastAckSignal,
                                    )}"
                                    >{isNumber(clientStats.lastAckSignal)
                                        ? `${clientStats.lastAckSignal} dBm`
                                        : "—"}</span
                                >
                                {#if ackAge}
                                    <span class="meter-sub mono">{ackAge}</span>
                                {/if}
                            </div>
                            <div class="bar-track">
                                <div
                                    class="bar-fill tone-{signalTone(
                                        clientStats.lastAckSignal,
                                    )}"
                                    style="width: {signalPct(
                                        clientStats.lastAckSignal,
                                    )}%"
                                ></div>
                            </div>
                        </div>

                        <div class="meter">
                            <div class="metric-label">Retries</div>
                            <div class="meter-value-row">
                                <span
                                    class="meter-value mono tone-{retryTone(
                                        clientStats.retryRate,
                                    )}"
                                    >{isNumber(clientStats.retryRate)
                                        ? `${clientStats.retryRate.toFixed(1)}%`
                                        : "—"}</span
                                >
                                <span class="meter-sub mono">cumulative</span>
                            </div>
                            <div class="bar-track">
                                <div
                                    class="bar-fill tone-{retryTone(
                                        clientStats.retryRate,
                                    )}"
                                    style="width: {retryPct(
                                        clientStats.retryRate,
                                    )}%"
                                ></div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Data rates: large numbers, prominent -->
        <div class="panel">
            <div class="panel-header">
                <div class="panel-title">Data rates</div>
                <div class="panel-sub">Negotiated PHY rates</div>
            </div>
            <div class="panel-body">
                <div class="rate-grid">
                    <div class="rate-tile">
                        <div class="metric-label">TX rate</div>
                        <div class="rate-value-row">
                            <span class="rate-value mono accent"
                                >{isNumber(clientStats.txBitrate)
                                    ? clientStats.txBitrate.toFixed(1)
                                    : "—"}</span
                            >
                            <span class="rate-unit mono">Mbps</span>
                        </div>
                    </div>
                    <div class="rate-tile">
                        <div class="metric-label">RX rate</div>
                        <div class="rate-value-row">
                            <span class="rate-value mono ok"
                                >{isNumber(clientStats.rxBitrate)
                                    ? clientStats.rxBitrate.toFixed(1)
                                    : "—"}</span
                            >
                            <span class="rate-unit mono">Mbps</span>
                        </div>
                    </div>
                    <div class="rate-tile">
                        <div class="metric-label">Channel width</div>
                        <div class="rate-value-row">
                            <span class="rate-value mono violet"
                                >{isNumber(clientStats.channelWidth)
                                    ? clientStats.channelWidth
                                    : "—"}</span
                            >
                            <span class="rate-unit mono">MHz</span>
                        </div>
                        <div class="rate-sub mono">
                            {#if clientStats.mimoConfig}
                                {clientStats.mimoConfig}
                            {/if}
                        </div>
                    </div>
                    <div class="rate-tile">
                        <div class="metric-label">Standard</div>
                        <div class="rate-value-row">
                            <span class="rate-value mono amber"
                                >{clientStats.wifiStandard || "—"}</span
                            >
                        </div>
                        <div class="rate-sub mono">
                            {bandLabel || ""}
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Traffic + Errors side-by-side -->
        <div class="bottom-grid">
            <div class="panel">
                <div class="panel-header">
                    <div class="panel-title">Traffic</div>
                    <div class="panel-sub">Cumulative since associate</div>
                </div>
                <div class="panel-body">
                    <div class="stat-grid">
                        <div class="mini-stat">
                            <div class="metric-label">TX bytes</div>
                            <div class="mini-value mono">
                                {formatBytes(clientStats.txBytes)}
                            </div>
                        </div>
                        <div class="mini-stat">
                            <div class="metric-label">RX bytes</div>
                            <div class="mini-value mono">
                                {formatBytes(clientStats.rxBytes)}
                            </div>
                        </div>
                        <div class="mini-stat">
                            <div class="metric-label">TX packets</div>
                            <div class="mini-value mono">
                                {formatCount(clientStats.txPackets)}
                            </div>
                        </div>
                        <div class="mini-stat">
                            <div class="metric-label">RX packets</div>
                            <div class="mini-value mono">
                                {formatCount(clientStats.rxPackets)}
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <div class="panel">
                <div class="panel-header">
                    <div class="panel-title">Errors</div>
                    <div class="panel-sub">Retransmits and failures</div>
                </div>
                <div class="panel-body">
                    <div class="error-card">
                        <div class="quality-row">
                            <span class="metric-label">Retry rate</span>
                            <span
                                class="meter-value mono tone-{retryTone(
                                    clientStats.retryRate,
                                )}"
                                >{isNumber(clientStats.retryRate)
                                    ? `${clientStats.retryRate.toFixed(1)}%`
                                    : "—"}</span
                            >
                        </div>
                        <div class="bar-track">
                            <div
                                class="bar-fill tone-{retryTone(
                                    clientStats.retryRate,
                                )}"
                                style="width: {Math.min(
                                    100,
                                    isNumber(clientStats.retryRate)
                                        ? clientStats.retryRate
                                        : 0,
                                )}%"
                            ></div>
                        </div>
                    </div>
                    <div class="stat-grid two">
                        <div class="mini-stat">
                            <div class="metric-label">TX retries</div>
                            <div class="mini-value mono">
                                {formatCount(clientStats.txRetries)}
                            </div>
                        </div>
                        <div class="mini-stat">
                            <div class="metric-label">TX failed</div>
                            <div
                                class="mini-value mono"
                                class:warn={isNumber(clientStats.txFailed) &&
                                    clientStats.txFailed > 0}
                            >
                                {formatCount(clientStats.txFailed)}
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        {#if connectedAP}
            <div class="panel">
                <div class="panel-header">
                    <div class="panel-title">Advanced capabilities</div>
                    <div class="panel-sub">
                        Negotiated with AP on association
                    </div>
                </div>
                <div class="panel-body capability-body">
                    <div class="capability-grid">
                        <div class="cap-section">
                            <div class="cap-title">Performance</div>
                            <div class="cap-row">
                                <span class="cap-k">Channel util.</span>
                                <span class="cap-v">
                                    {#if formatUtilization(connectedAP.bssLoadUtilization)}
                                        <span class="chip acc"
                                            >{formatUtilization(
                                                connectedAP.bssLoadUtilization,
                                            )}</span
                                        >
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">Max PHY</span>
                                <span class="cap-v mono"
                                    >{formatPhyRate(
                                        connectedAP.maxPhyRate,
                                    )}</span
                                >
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">MIMO</span>
                                <span class="cap-v mono">
                                    {formatMimo(
                                        connectedAP.mimoStreams,
                                        clientStats.mimoConfig,
                                    )}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">QAM</span>
                                <span class="cap-v">
                                    {#if formatQam(connectedAP.qamSupport)}
                                        <span class="chip acc"
                                            >{formatQam(
                                                connectedAP.qamSupport,
                                            )}</span
                                        >
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                        </div>

                        <div class="cap-section">
                            <div class="cap-title">Roaming</div>
                            <div class="cap-row">
                                <span class="cap-k">802.11v (BSS)</span>
                                <span class="cap-v">
                                    {#if supportChip(connectedAP.bsstransition)}
                                        <span
                                            class="chip {supportChip(
                                                connectedAP.bsstransition,
                                            ).tone}"
                                            >{supportChip(
                                                connectedAP.bsstransition,
                                            ).text}</span
                                        >
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">802.11r (FT)</span>
                                <span class="cap-v">
                                    {#if supportChip(connectedAP.fastroaming)}
                                        <span
                                            class="chip {supportChip(
                                                connectedAP.fastroaming,
                                            ).tone}"
                                            >{supportChip(
                                                connectedAP.fastroaming,
                                            ).text}</span
                                        >
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">802.11k (Nbr)</span>
                                <span class="cap-v">
                                    {#if supportChip(connectedAP.neighborReport)}
                                        <span
                                            class="chip {supportChip(
                                                connectedAP.neighborReport,
                                            ).tone}"
                                            >{supportChip(
                                                connectedAP.neighborReport,
                                            ).text}</span
                                        >
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">UAPSD</span>
                                <span class="cap-v">
                                    {#if supportChip(connectedAP.uapsd)}
                                        <span
                                            class="chip {supportChip(
                                                connectedAP.uapsd,
                                            ).tone}"
                                            >{supportChip(connectedAP.uapsd)
                                                .text}</span
                                        >
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                        </div>

                        <div class="cap-section">
                            <div class="cap-title">WiFi 6 / 7</div>
                            <div class="cap-row">
                                <span class="cap-k">MU-MIMO</span>
                                <span class="cap-v">
                                    {#if supportChip(connectedAP.mumimo)}
                                        <span
                                            class="chip {supportChip(
                                                connectedAP.mumimo,
                                            ).tone}"
                                            >{supportChip(connectedAP.mumimo)
                                                .text}</span
                                        >
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">BSS Color</span>
                                <span class="cap-v mono">
                                    {isNumber(connectedAP.bssColor) &&
                                    connectedAP.bssColor > 0
                                        ? connectedAP.bssColor
                                        : "—"}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">OBSS PD</span>
                                <span class="cap-v">
                                    {#if supportChip(connectedAP.obssPD)}
                                        <span
                                            class="chip {supportChip(
                                                connectedAP.obssPD,
                                            ).tone}"
                                            >{supportChip(connectedAP.obssPD)
                                                .text}</span
                                        >
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">Target Wake</span>
                                <span class="cap-v">
                                    {#if supportChip(connectedAP.twtSupport)}
                                        <span
                                            class="chip {supportChip(
                                                connectedAP.twtSupport,
                                            ).tone}"
                                            >{supportChip(
                                                connectedAP.twtSupport,
                                            ).text}</span
                                        >
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                        </div>

                        <div class="cap-section last">
                            <div class="cap-title">QoS &amp; Regulatory</div>
                            <div class="cap-row">
                                <span class="cap-k">WMM / QoS</span>
                                <span class="cap-v">
                                    {#if supportChip(connectedAP.qosSupport)}
                                        <span
                                            class="chip {supportChip(
                                                connectedAP.qosSupport,
                                            ).tone}"
                                            >{supportChip(
                                                connectedAP.qosSupport,
                                            ).text}</span
                                        >
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">DTIM</span>
                                <span class="cap-v mono">
                                    {isNumber(connectedAP.dtim) &&
                                    connectedAP.dtim > 0
                                        ? connectedAP.dtim
                                        : "—"}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">PMF</span>
                                <span class="cap-v">
                                    {#if connectedAP.pmf}
                                        {#if pmfTone(connectedAP.pmf)}
                                            <span
                                                class="chip {pmfTone(
                                                    connectedAP.pmf,
                                                )}">{connectedAP.pmf}</span
                                            >
                                        {:else}
                                            <span class="mono"
                                                >{connectedAP.pmf}</span
                                            >
                                        {/if}
                                    {:else}
                                        <span class="mono">—</span>
                                    {/if}
                                </span>
                            </div>
                            <div class="cap-row">
                                <span class="cap-k">Country</span>
                                <span class="cap-v mono"
                                    >{connectedAP.countryCode || "—"}</span
                                >
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        {/if}

        {#if hasRoaming}
            <div class="panel">
                <div class="panel-header">
                    <div class="panel-title">Roaming history</div>
                    <div class="panel-sub">
                        {clientStats.roamingHistory.length} event{clientStats
                            .roamingHistory.length === 1
                            ? ""
                            : "s"}
                    </div>
                </div>
                <div class="panel-body">
                    <div class="roaming-list">
                        {#each clientStats.roamingHistory.slice().reverse() as event}
                            <div class="roaming-event">
                                <div class="roaming-time mono">
                                    {new Date(
                                        event.timestamp,
                                    ).toLocaleTimeString()}
                                </div>
                                <div class="roaming-path mono">
                                    <span class="bssid-from"
                                        >{(event.previousBssid || "").slice(
                                            -8,
                                        )}</span
                                    >
                                    <span class="arrow">→</span>
                                    <span class="bssid-to"
                                        >{(event.newBssid || "").slice(-8)}</span
                                    >
                                </div>
                                <div class="roaming-signals mono">
                                    {event.previousSignal} → {event.newSignal} dBm
                                    · ch {event.previousChannel} → {event.newChannel}
                                </div>
                            </div>
                        {/each}
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
        display: flex;
        flex-direction: column;
        gap: 12px;
        padding: 16px;
        font-family: var(--font-ui, inherit);
    }

    .mono {
        font-family: var(--font-mono, ui-monospace, monospace);
        font-variant-numeric: tabular-nums;
    }

    /* ── Layout grids ──────────────────────────────────────── */
    .top-grid {
        display: grid;
        grid-template-columns: 2fr 3fr;
        gap: 12px;
    }

    .bottom-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 12px;
    }

    /* ── Panel scaffolding ─────────────────────────────────── */
    .panel {
        background: var(--bg-2);
        border: 1px solid var(--line-1, var(--border));
        border-radius: 8px;
        overflow: hidden;
        display: flex;
        flex-direction: column;
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

    .panel-sub {
        font-size: 11px;
        color: var(--fg-3, var(--muted-2));
        margin-top: 2px;
    }

    .panel-body {
        padding: 14px;
        display: flex;
        flex-direction: column;
        gap: 14px;
    }

    /* ── Chips ─────────────────────────────────────────────── */
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

    .chip.ok {
        background: var(--ok-bg);
        color: var(--ok);
        border-color: var(--ok-line);
    }

    .chip.acc {
        background: var(--acc-1-bg);
        color: var(--acc-1);
        border-color: var(--acc-1-line);
    }

    /* ── Connection panel ──────────────────────────────────── */
    .ssid-row {
        display: flex;
        align-items: center;
        gap: 10px;
    }

    .ssid {
        font-size: 18px;
        font-weight: 600;
        color: var(--text);
    }

    .stat-list {
        display: flex;
        flex-direction: column;
    }

    .stat-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 6px 0;
        font-size: 12px;
        border-bottom: 1px dashed var(--line-1, var(--border));
        gap: 8px;
    }

    .stat-row:last-child {
        border-bottom: none;
    }

    .stat-row .k {
        color: var(--fg-3, var(--muted-2));
        text-transform: uppercase;
        letter-spacing: 0.06em;
        font-size: 10.5px;
        font-weight: 500;
    }

    .stat-row .v {
        color: var(--fg-1, var(--text));
        text-align: right;
        min-width: 0;
        word-break: break-word;
    }

    /* ── Quality meters ────────────────────────────────────── */
    .meter-grid {
        display: grid;
        grid-template-columns: 1fr 1fr 1fr;
        gap: 8px;
    }

    .meter {
        padding: 12px;
        background: var(--bg-3);
        border-radius: 6px;
        border: 1px solid var(--line-1, var(--border));
        display: flex;
        flex-direction: column;
        gap: 6px;
        min-width: 0;
    }

    .metric-label {
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.1em;
        color: var(--fg-3, var(--muted-2));
        font-weight: 600;
    }

    .meter-value-row {
        display: flex;
        align-items: baseline;
        gap: 8px;
    }

    .meter-value {
        font-size: 16px;
        font-weight: 500;
        color: var(--text);
    }

    .meter-sub {
        font-size: 10px;
        color: var(--fg-3, var(--muted-2));
    }

    .meter-value.tone-ok,
    .mini-value.tone-ok {
        color: var(--ok);
    }

    .meter-value.tone-warn,
    .mini-value.tone-warn {
        color: var(--warn);
    }

    .meter-value.tone-bad,
    .mini-value.tone-bad {
        color: var(--bad);
    }

    .meter-value.tone-muted {
        color: var(--fg-2, var(--muted));
    }

    .bar-track {
        height: 4px;
        background: var(--bg-4, var(--panel-strong));
        border-radius: 2px;
        overflow: hidden;
    }

    .bar-fill {
        height: 100%;
        border-radius: 2px;
        background: var(--muted);
        transition: width 0.3s ease;
    }

    .bar-fill.tone-ok {
        background: var(--ok);
    }

    .bar-fill.tone-warn {
        background: var(--warn);
    }

    .bar-fill.tone-bad {
        background: var(--bad);
    }

    .bar-fill.tone-muted {
        background: var(--fg-3, var(--muted-2));
    }

    /* ── Rate tiles ────────────────────────────────────────── */
    .rate-grid {
        display: grid;
        grid-template-columns: repeat(4, 1fr);
        gap: 10px;
    }

    .rate-tile {
        padding: 12px;
        background: var(--bg-3);
        border-radius: 6px;
        border: 1px solid var(--line-1, var(--border));
        display: flex;
        flex-direction: column;
        gap: 6px;
        min-width: 0;
    }

    .rate-value-row {
        display: flex;
        align-items: baseline;
        gap: 5px;
    }

    .rate-value {
        font-size: 22px;
        font-weight: 500;
        color: var(--text);
    }

    .rate-value.accent {
        color: var(--acc-1);
    }

    .rate-value.ok {
        color: var(--ok);
    }

    .rate-value.violet {
        color: #a78bfa;
    }

    .rate-value.amber {
        color: #fbbf24;
    }

    .rate-unit {
        font-size: 11px;
        color: var(--fg-3, var(--muted-2));
    }

    .rate-sub {
        font-size: 10.5px;
        color: var(--fg-3, var(--muted-2));
    }

    /* ── Mini stats (traffic / errors) ─────────────────────── */
    .stat-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 8px;
    }

    .stat-grid.two {
        grid-template-columns: 1fr 1fr;
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

    .error-card {
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
        justify-content: space-between;
        align-items: baseline;
        gap: 8px;
    }

    /* ── Capability grid (Advanced capabilities panel) ─────── */
    .capability-body {
        padding: 0;
    }

    .capability-grid {
        display: grid;
        grid-template-columns: repeat(4, 1fr);
        gap: 0;
    }

    .cap-section {
        padding: 14px 16px;
        border-right: 1px solid var(--line-1, var(--border));
        min-width: 0;
    }

    .cap-section.last,
    .cap-section:last-child {
        border-right: none;
    }

    .cap-title {
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.12em;
        color: var(--fg-3, var(--muted-2));
        font-weight: 600;
        margin-bottom: 10px;
    }

    .cap-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 5px 0;
        font-size: 12px;
        border-bottom: 1px dashed var(--line-1, var(--border));
        gap: 8px;
        flex-wrap: wrap;
        min-width: 0;
    }

    .cap-row:last-child {
        border-bottom: none;
    }

    .cap-k {
        color: var(--fg-3, var(--muted-2));
        flex-shrink: 0;
    }

    .cap-v {
        color: var(--fg-1, var(--text));
        font-family: var(--font-mono, ui-monospace, monospace);
        font-variant-numeric: tabular-nums;
        text-align: right;
        min-width: 0;
        word-break: break-word;
    }

    .cap-v .chip {
        white-space: nowrap;
    }

    .chip.bad {
        background: var(--bad-bg);
        color: var(--bad);
        border-color: var(--bad-line);
    }

    .chip.warn {
        background: var(--warn-bg);
        color: var(--warn);
        border-color: var(--warn-line);
    }

    /* ── Roaming history ───────────────────────────────────── */
    .roaming-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
        max-height: 320px;
        overflow-y: auto;
    }

    .roaming-event {
        padding: 10px 12px;
        background: var(--bg-3);
        border: 1px solid var(--line-1, var(--border));
        border-radius: 6px;
        display: grid;
        grid-template-columns: auto 1fr auto;
        gap: 12px;
        align-items: center;
    }

    .roaming-time {
        font-size: 11px;
        color: var(--fg-3, var(--muted-2));
    }

    .roaming-path {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 12px;
    }

    .bssid-from {
        color: var(--warn);
    }

    .bssid-to {
        color: var(--ok);
    }

    .arrow {
        color: var(--fg-3, var(--muted-2));
    }

    .roaming-signals {
        font-size: 11px;
        color: var(--fg-2, var(--muted));
        text-align: right;
    }

    /* ── Empty state ───────────────────────────────────────── */
    .not-connected {
        flex: 1;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 40px 20px;
        text-align: center;
        color: var(--fg-3, var(--muted-2));
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
        margin-top: 8px;
    }

    /* ── Responsive ────────────────────────────────────────── */
    @media (max-width: 1100px) {
        .top-grid,
        .bottom-grid {
            grid-template-columns: 1fr;
        }

        .meter-grid {
            grid-template-columns: 1fr 1fr;
        }

        .rate-grid {
            grid-template-columns: 1fr 1fr;
        }

        .capability-grid {
            grid-template-columns: 1fr 1fr;
        }

        .cap-section {
            border-right: none;
            border-bottom: 1px solid var(--line-1, var(--border));
        }

        .cap-section:nth-last-child(-n + 2) {
            border-bottom: none;
        }
    }

    @media (max-width: 640px) {
        .meter-grid,
        .rate-grid,
        .stat-grid,
        .capability-grid {
            grid-template-columns: 1fr;
        }

        .cap-section {
            border-right: none;
            border-bottom: 1px solid var(--line-1, var(--border));
        }

        .cap-section:last-child {
            border-bottom: none;
        }

        .roaming-event {
            grid-template-columns: 1fr;
            gap: 4px;
        }

        .roaming-signals {
            text-align: left;
        }
    }
</style>
