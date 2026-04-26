<script>
    import {
        SaveReport,
        ExportNetworks,
        GetRoamingAnalysis,
    } from "../../wailsjs/go/main/App.js";
    import { createEventDispatcher } from "svelte";

    export let networks = null;
    export let clientStats = null;

    const dispatch = createEventDispatcher();

    // ── Modal state — section toggles + per-section format pickers
    // Mirrors the design's Reports modal: each row is a checkbox + a
    // segmented format picker, then a single "Export selected" button at
    // the bottom fires every enabled section.
    let sections = {
        networks: true,
        stats: true,
        report: false,
    };
    let formats = {
        networks: "csv",
        stats: "json",
        report: "html",
    };
    let exporting = false;
    let lastError = "";

    const FORMAT_OPTIONS = {
        networks: ["csv", "json"],
        stats: ["json"],
        report: ["html", "json"],
    };

    $: networksAvailable = !!(networks && networks.length > 0);
    $: statsAvailable = !!clientStats;
    $: anySelected =
        (sections.networks && networksAvailable) ||
        (sections.stats && statsAvailable) ||
        sections.report;

    function downloadFile(content, filename, type = "text/plain") {
        const blob = new Blob([content], { type: `${type};charset=utf-8;` });
        const link = document.createElement("a");

        if ("msSaveBlob" in navigator) {
            navigator.msSaveBlob(blob, filename);
        } else {
            const url = URL.createObjectURL(blob);
            link.href = url;
            link.download = filename;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            URL.revokeObjectURL(url);
        }
    }

    async function saveFile(content, filename, type = "text/plain") {
        try {
            if (typeof SaveReport === "function") {
                const path = await SaveReport(filename, content);
                if (path) return;
            }
        } catch (err) {
            console.warn("SaveReport failed, falling back to download:", err);
        }
        downloadFile(content, filename, type);
    }

    // exportNetworksCsv delegates to the backend so the CSV schema and
    // quoting rules stay consistent across UI and any other callers of
    // ExportNetworks. Falls back to the client-side builder if the Wails
    // runtime is unavailable (e.g. browser-only dev mode).
    async function exportNetworksCsv() {
        if (typeof ExportNetworks === "function") {
            try {
                const csv = await ExportNetworks("csv");
                if (csv) {
                    await saveFile(csv, "networks.csv", "text/csv");
                    return;
                }
            } catch (err) {
                console.warn(
                    "ExportNetworks(csv) failed, falling back to client-side CSV:",
                    err,
                );
            }
        }
        await saveFile(buildNetworkCsv(), "networks.csv", "text/csv");
    }

    async function exportNetworksJson() {
        if (typeof ExportNetworks === "function") {
            try {
                const json = await ExportNetworks("json");
                if (json) {
                    await saveFile(json, "networks.json", "application/json");
                    return;
                }
            } catch (err) {
                console.warn(
                    "ExportNetworks(json) failed, falling back to client-side JSON:",
                    err,
                );
            }
        }
        await saveFile(
            JSON.stringify(sanitizeNetworks(networks), null, 2),
            "networks.json",
            "application/json",
        );
    }

    function escapeHtml(value) {
        return String(value ?? "")
            .replaceAll("&", "&amp;")
            .replaceAll("<", "&lt;")
            .replaceAll(">", "&gt;")
            .replaceAll('"', "&quot;")
            .replaceAll("'", "&#39;");
    }

    function sanitizeAccessPoint(ap) {
        if (!ap) return ap;
        const { noise, txPower, ...rest } = ap;
        return rest;
    }

    function sanitizeNetworks(data) {
        if (!Array.isArray(data)) return data;
        return data.map((network) => {
            const accessPoints = (network.accessPoints || []).map(
                sanitizeAccessPoint,
            );
            return {
                ...network,
                accessPoints,
            };
        });
    }

    function sanitizeClientStats(stats) {
        if (!stats) return stats;
        const { noise, snr, ...rest } = stats;
        return {
            ...rest,
            snr: typeof snr === "number" && snr > 0 ? snr : null,
        };
    }

    function buildNetworkCsv() {
        if (!networks || networks.length === 0) return "";
        const headers = [
            "SSID",
            "BSSID",
            "Vendor",
            "Band",
            "FrequencyMHz",
            "Channel",
            "ChannelWidthMHz",
            "SignalDbm",
            "SignalQuality",
            "Security",
            "Capabilities",
            "DFS",
            "LastSeen",
            "BSSLoadStations",
            "BSSLoadUtilization",
            "MaxPhyRateMbps",
            "MIMOStreams",
            "QoSSupport",
            "PMF",
            "QAMSupport",
            "UAPSD",
            "BSSColor",
            "OBSSPD",
            "CountryCode",
            "SecurityCiphers",
            "AuthMethods",
            "WiFiStandard",
            "APCount",
            "BestSignal",
            "BestSignalAP",
        ];
        const rows = [];
        networks.forEach((network) => {
            const base = {
                ssid: network.ssid,
                apCount: network.apCount,
                bestSignal: network.bestSignal,
                bestSignalAP: network.bestSignalAP || "",
            };
            if (network.accessPoints && network.accessPoints.length > 0) {
                network.accessPoints.forEach((ap) => {
                    rows.push([
                        base.ssid,
                        ap.bssid,
                        ap.vendor,
                        ap.band,
                        ap.frequency,
                        ap.channel,
                        ap.channelWidth,
                        ap.signal,
                        ap.signalQuality,
                        ap.security,
                        (ap.capabilities || []).join("|"),
                        ap.dfs,
                        ap.lastSeen,
                        ap.bssLoadStations,
                        ap.bssLoadUtilization,
                        ap.maxPhyRate > 0 ? ap.maxPhyRate : "",
                        ap.mimoStreams,
                        ap.qosSupport,
                        ap.pmf,
                        ap.qamSupport,
                        ap.uapsd,
                        ap.bssColor,
                        ap.obssPD,
                        ap.countryCode,
                        (ap.securityCiphers || []).join("|"),
                        (ap.authMethods || []).join("|"),
                        ap.wifiStandard,
                        base.apCount,
                        base.bestSignal,
                        base.bestSignalAP,
                    ]);
                });
            } else {
                rows.push([
                    base.ssid,
                    "",
                    "",
                    "",
                    "",
                    network.channel,
                    "",
                    "",
                    "",
                    network.security,
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    base.apCount,
                    base.bestSignal,
                    base.bestSignalAP,
                ]);
            }
        });
        return [headers, ...rows]
            .map((row) =>
                row
                    .map(
                        (value) =>
                            `"${String(value ?? "").replaceAll('"', '""')}"`,
                    )
                    .join(","),
            )
            .join("\n");
    }

    function buildReportData() {
        const totalNetworks = networks?.length || 0;
        const totalAPs =
            networks?.reduce(
                (count, network) => count + (network.accessPoints?.length || 0),
                0,
            ) || 0;

        return {
            generatedAt: new Date().toISOString(),
            summary: {
                totalNetworks,
                totalAPs,
                connected: clientStats?.connected || false,
                connectedSSID: clientStats?.ssid || null,
                connectedBSSID: clientStats?.bssid || null,
            },
            clientStats: sanitizeClientStats(clientStats) || null,
            networks: sanitizeNetworks(networks) || [],
        };
    }

    // ── Helpers shared by HTML report builder ─────────────────
    function fmtDbm(v) {
        return typeof v === "number" && !Number.isNaN(v)
            ? `${Math.round(v)} dBm`
            : "—";
    }

    function fmtMbps(v) {
        return typeof v === "number" && !Number.isNaN(v) && v > 0
            ? `${v.toFixed(1)} Mbps`
            : "—";
    }

    function fmtPercent(v) {
        return typeof v === "number" && !Number.isNaN(v)
            ? `${v.toFixed(1)} %`
            : "—";
    }

    function fmtDurationSec(s) {
        if (typeof s !== "number" || Number.isNaN(s)) return "—";
        const h = Math.floor(s / 3600);
        const m = Math.floor((s % 3600) / 60);
        const sec = s % 60;
        if (h > 0) return `${h}h ${m}m ${sec}s`;
        if (m > 0) return `${m}m ${sec}s`;
        return `${sec}s`;
    }

    function fmtMs(v) {
        if (typeof v !== "number" || !Number.isFinite(v) || v <= 0) return "—";
        if (v < 1000) return `${Math.round(v)} ms`;
        return `${(v / 1000).toFixed(1)} s`;
    }

    function pad2(n) {
        return n < 10 ? "0" + n : "" + n;
    }

    function reportTimestamp() {
        const d = new Date();
        const yyyy = d.getFullYear();
        const mm = pad2(d.getMonth() + 1);
        const dd = pad2(d.getDate());
        const hh = pad2(d.getHours());
        const mi = pad2(d.getMinutes());
        const ss = pad2(d.getSeconds());
        return {
            iso: d.toISOString(),
            ymd: `${yyyy}-${mm}-${dd}`,
            hms: `${hh}:${mi}:${ss}`,
            full: `${yyyy}-${mm}-${dd} ${hh}:${mi}:${ss}`,
            id: `RPT-${yyyy}${mm}${dd}-${hh}${mi}`,
        };
    }

    function signalQualityBucket(dBm) {
        if (typeof dBm !== "number" || Number.isNaN(dBm)) return null;
        if (dBm >= -50) return "excellent";
        if (dBm >= -60) return "good";
        if (dBm >= -67) return "fair";
        if (dBm >= -75) return "weak";
        return "poor";
    }

    function signalToneClass(dBm) {
        const b = signalQualityBucket(dBm);
        if (b === "excellent" || b === "good") return "ok";
        if (b === "fair" || b === "weak") return "warn";
        if (b === "poor") return "bad";
        return "";
    }

    function sigBarsHTML(dBm) {
        if (typeof dBm !== "number" || Number.isNaN(dBm)) return "—";
        const tone = signalToneClass(dBm) || "bad";
        const bars =
            dBm >= -55 ? 4 : dBm >= -65 ? 3 : dBm >= -75 ? 2 : dBm >= -85 ? 1 : 0;
        const heights = [3, 5, 7, 9];
        let html = '<span class="sig-bar">';
        for (let i = 0; i < 4; i++) {
            html +=
                i < bars
                    ? `<span class="on ${tone}" style="height:${heights[i]}px"></span>`
                    : `<span style="height:${heights[i]}px"></span>`;
        }
        return html + "</span>";
    }

    function summarizeSignalHistory(history) {
        const samples = (history || [])
            .map((p) => p && p.signal)
            .filter((v) => typeof v === "number" && !Number.isNaN(v));
        if (samples.length === 0) {
            return {
                count: 0,
                min: null,
                max: null,
                avg: null,
                stddev: null,
                pctExcellent: 0,
                pctGood: 0,
                pctFair: 0,
                pctWeak: 0,
                pctPoor: 0,
            };
        }
        const min = Math.min(...samples);
        const max = Math.max(...samples);
        const avg = samples.reduce((a, b) => a + b, 0) / samples.length;
        const variance =
            samples.reduce((acc, v) => acc + (v - avg) * (v - avg), 0) /
            samples.length;
        const stddev = Math.sqrt(variance);
        const buckets = { excellent: 0, good: 0, fair: 0, weak: 0, poor: 0 };
        for (const v of samples) {
            const b = signalQualityBucket(v);
            if (b) buckets[b]++;
        }
        const pct = (k) => (buckets[k] / samples.length) * 100;
        return {
            count: samples.length,
            min,
            max,
            avg,
            stddev,
            pctExcellent: pct("excellent"),
            pctGood: pct("good"),
            pctFair: pct("fair"),
            pctWeak: pct("weak"),
            pctPoor: pct("poor"),
        };
    }

    function buildSignalSvg(history) {
        const samples = (history || [])
            .map((p) => p && p.signal)
            .filter((v) => typeof v === "number");
        const N = samples.length;
        const W = 700;
        const H = 180;
        const pad = { left: 40, right: 10, top: 10, bottom: 22 };
        const innerW = W - pad.left - pad.right;
        const innerH = H - pad.top - pad.bottom;
        const yMin = -90;
        const yMax = -30;
        const yScale = (v) =>
            pad.top + ((yMax - v) / (yMax - yMin)) * innerH;
        // Quality zones — match the design exactly
        const zonesSvg = `
        <rect x="${pad.left}" y="${yScale(-30)}" width="${innerW}" height="${yScale(-50) - yScale(-30)}" fill="#15803d" opacity="0.07"/>
        <rect x="${pad.left}" y="${yScale(-50)}" width="${innerW}" height="${yScale(-60) - yScale(-50)}" fill="#15803d" opacity="0.025"/>
        <rect x="${pad.left}" y="${yScale(-60)}" width="${innerW}" height="${yScale(-67) - yScale(-60)}" fill="#a16207" opacity="0.05"/>
        <rect x="${pad.left}" y="${yScale(-67)}" width="${innerW}" height="${yScale(-75) - yScale(-67)}" fill="#a16207" opacity="0.10"/>
        <rect x="${pad.left}" y="${yScale(-75)}" width="${innerW}" height="${yScale(-90) - yScale(-75)}" fill="#b91c1c" opacity="0.11"/>
        <line x1="${pad.left}" y1="${yScale(-67)}" x2="${W - pad.right}" y2="${yScale(-67)}" stroke="#a16207" stroke-width="0.8" stroke-dasharray="4,4" opacity="0.5"/>
        <line x1="${pad.left}" y1="${yScale(-75)}" x2="${W - pad.right}" y2="${yScale(-75)}" stroke="#b91c1c" stroke-width="0.8" stroke-dasharray="4,4" opacity="0.5"/>`;
        const yTicks = [-30, -40, -50, -60, -70, -80, -90];
        let yLabels = '<g font-family="JetBrains Mono" font-size="9" fill="#6b7383" text-anchor="end">';
        for (const t of yTicks) {
            yLabels += `<text x="${pad.left - 4}" y="${yScale(t) + 3}">${t}</text>`;
        }
        yLabels += "</g>";
        const zoneLabels = `
        <g font-family="JetBrains Mono" font-size="8" font-weight="600" text-anchor="end" opacity="0.75">
            <text x="${W - pad.right - 4}" y="${yScale(-40) + 3}" fill="#15803d">EXCELLENT</text>
            <text x="${W - pad.right - 4}" y="${yScale(-63.5) + 3}" fill="#a16207">FAIR</text>
            <text x="${W - pad.right - 4}" y="${yScale(-71) + 3}" fill="#a16207">WEAK</text>
            <text x="${W - pad.right - 4}" y="${yScale(-82) + 3}" fill="#b91c1c">POOR</text>
        </g>`;
        let pathSvg = "";
        if (N >= 2) {
            const xScale = (i) =>
                pad.left + (i / (N - 1)) * innerW;
            let d = "";
            for (let i = 0; i < N; i++) {
                d += `${i === 0 ? "M" : "L"}${xScale(i).toFixed(1)},${yScale(samples[i]).toFixed(1)} `;
            }
            const lastX = xScale(N - 1);
            const lastY = yScale(samples[N - 1]);
            pathSvg = `
            <path d="${d.trim()}" fill="none" stroke="#0b6e74" stroke-width="1.8" stroke-linejoin="round"/>
            <circle cx="${lastX}" cy="${lastY}" r="2.5" fill="#0b6e74"/>`;
        } else {
            pathSvg = `<text x="${W / 2}" y="${H / 2}" text-anchor="middle" fill="#9ca3af" font-family="Inter" font-size="11">Not enough samples to render — start a scan</text>`;
        }
        return `<svg width="100%" height="${H}" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" style="display:block">
        ${zonesSvg}
        ${yLabels}
        ${zoneLabels}
        ${pathSvg}
        </svg>`;
    }

    function buildSignalDistributionSvg(stats) {
        if (!stats || stats.count === 0) {
            return '<div class="mut" style="font-size:10.5px">No samples in current capture window.</div>';
        }
        const W = 700;
        const H = 34;
        const pct = [
            stats.pctExcellent,
            stats.pctGood,
            stats.pctFair,
            stats.pctWeak,
            stats.pctPoor,
        ];
        const colors = [
            ["#15803d", 0.8],
            ["#15803d", 0.4],
            ["#a16207", 0.7],
            ["#a16207", 0.95],
            ["#b91c1c", 0.7],
        ];
        const labels = ["Excellent", "Good", "Fair", "Weak", "Poor"];
        let x = 0;
        let rects = "";
        let textTags = "";
        const labelOffsets = [];
        for (let i = 0; i < pct.length; i++) {
            const w = (pct[i] / 100) * W;
            if (w > 0.5) {
                rects += `<rect x="${x.toFixed(1)}" y="0" width="${w.toFixed(1)}" height="24" fill="${colors[i][0]}" opacity="${colors[i][1]}"/>`;
                labelOffsets.push({ x, w, text: `${labels[i]} ${pct[i].toFixed(0)}%` });
            }
            x += w;
        }
        // Attach labels under blocks ≥ 8% wide
        for (const lbl of labelOffsets) {
            if (lbl.w >= 56) {
                textTags += `<text x="${lbl.x.toFixed(1)}" y="33">${escapeHtml(lbl.text)}</text>`;
            }
        }
        return `<svg width="100%" height="${H}" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" style="display:block">
            ${rects}
            <g font-family="JetBrains Mono" font-size="9" fill="#6b7383">${textTags}</g>
        </svg>`;
    }

    function findConnectedAP(allNetworks, bssid) {
        if (!bssid) return null;
        const target = bssid.toLowerCase();
        for (const n of allNetworks || []) {
            for (const ap of n.accessPoints || []) {
                if (ap?.bssid && ap.bssid.toLowerCase() === target) {
                    return { network: n, ap };
                }
            }
        }
        return null;
    }

    function summarizeBands(allNetworks) {
        const bands = { "2.4GHz": 0, "5GHz": 0, "6GHz": 0 };
        const bssids = new Set();
        for (const n of allNetworks || []) {
            for (const ap of n.accessPoints || []) {
                if (ap?.bssid) bssids.add(ap.bssid);
                if (ap?.band && bands[ap.band] != null) bands[ap.band]++;
            }
        }
        const distinctBands = Object.values(bands).filter((c) => c > 0).length;
        return {
            ssidCount: (allNetworks || []).length,
            bssidCount: bssids.size,
            distinctBands,
            bands,
        };
    }

    function deriveFindings(connectedAP, allNetworks, roamingReport) {
        const findings = [];
        let nextW = 1;
        let nextI = 1;
        let nextP = 1;
        const ap = connectedAP?.ap;

        if (ap) {
            // OBSS PD / Spatial Reuse — only call out for WiFi 6 capable APs.
            const isWifi6 =
                ap.qamSupport && ap.qamSupport >= 1024;
            if (isWifi6 && ap.obssPD === false) {
                findings.push({
                    severity: "warn",
                    code: `W-${pad2(nextW++)}`,
                    title: "OBSS PD (Spatial Reuse) not enabled on primary AP",
                    description:
                        `The connected AP (<span class="mono">${escapeHtml(ap.bssid)}</span>) advertises ` +
                        `WiFi&nbsp;6 with <span class="mono">BSS Color ${escapeHtml(ap.bssColor)}</span> but does ` +
                        `not enable OBSS PD. In dense environments this limits spatial reuse and ` +
                        `can reduce effective throughput when co-channel APs transmit.`,
                    action:
                        "Enable Spatial Reuse (OBSS PD) for 5&nbsp;GHz on this AP.",
                });
            }
            // 802.11k coverage across the deployment.
            const apsInDeployment = (allNetworks || [])
                .flatMap((n) => n.accessPoints || [])
                .filter((a) => a && (a.ssid || "").length > 0);
            const sameSSID = apsInDeployment.filter(
                (a) => a.ssid === connectedAP.network?.ssid,
            );
            if (sameSSID.length > 1) {
                const withK = sameSSID.filter((a) => a.neighborReport).length;
                if (withK < sameSSID.length) {
                    findings.push({
                        severity: "warn",
                        code: `W-${pad2(nextW++)}`,
                        title: "802.11k neighbor reports incomplete",
                        description:
                            `${withK} of ${sameSSID.length} access points serving ` +
                            `<span class="mono">${escapeHtml(connectedAP.network.ssid)}</span> advertise 802.11k ` +
                            `neighbor lists. Without them, clients fall back to full active scans before ` +
                            `roaming, adding 80–150 ms per transition.`,
                        action:
                            "Enable 802.11k neighbor reports on every AP in the SSID group.",
                    });
                }
            }
            // 2.4 GHz congestion.
            const ch24 = (allNetworks || [])
                .flatMap((n) => n.accessPoints || [])
                .filter((a) => a && a.band === "2.4GHz");
            if (ch24.length >= 4) {
                const counts = {};
                for (const a of ch24) {
                    counts[a.channel] = (counts[a.channel] || 0) + 1;
                }
                const busiest = Object.entries(counts).sort((a, b) => b[1] - a[1])[0];
                if (busiest && busiest[1] >= 3) {
                    findings.push({
                        severity: "info",
                        code: `I-${pad2(nextI++)}`,
                        title: "2.4 GHz band is congested",
                        description:
                            `${ch24.length} APs are visible on 2.4&nbsp;GHz; channel <span class="mono">${escapeHtml(busiest[0])}</span> ` +
                            `is the busiest with ${busiest[1]} APs.`,
                        action:
                            "If IoT or guest devices on 2.4&nbsp;GHz report dropouts, prefer ch&nbsp;6 or 11 (cleanest non-overlapping).",
                    });
                }
            }
            // Modern security — pass.
            const secOk =
                (ap.security || "").includes("WPA3") &&
                ap.pmf === "Required" &&
                !ap.wps;
            if (secOk) {
                findings.push({
                    severity: "ok",
                    code: `P-${pad2(nextP++)}`,
                    title: "Security posture is modern",
                    description:
                        `${escapeHtml(ap.security)} with PMF Required and ${(ap.securityCiphers || []).join(", ") || "CCMP"} cipher. ` +
                        `${ap.wps ? "WPS is enabled (consider disabling)." : "WPS disabled. No action required."}`,
                });
            } else if ((ap.security || "").toLowerCase() === "open") {
                findings.push({
                    severity: "bad",
                    code: `C-${pad2(nextI++)}`,
                    title: "Connected AP is open / unencrypted",
                    description:
                        "The connected SSID is open. All traffic is observable on the air; this is not safe for production use.",
                    action:
                        "Disconnect immediately; configure WPA2 (minimum) or WPA3 on the AP.",
                });
            }
        }

        // Roaming-derived findings.
        if (roamingReport) {
            if (roamingReport.slowRoamCount > 0) {
                findings.push({
                    severity: "warn",
                    code: `W-${pad2(nextW++)}`,
                    title: `${roamingReport.slowRoamCount} slow roam${roamingReport.slowRoamCount === 1 ? "" : "s"} (≥ 2 s) detected`,
                    description:
                        "Slow roams typically indicate authentication delay (full re-auth without 802.11r FT) or 802.1X RADIUS latency.",
                    action:
                        "Verify FT is enabled across SSID; check RADIUS server latency if 802.1X is in use.",
                });
            }
            if (roamingReport.excessiveRoaming) {
                findings.push({
                    severity: "warn",
                    code: `W-${pad2(nextW++)}`,
                    title: "Excessive roaming (more than 10/hr)",
                    description:
                        "The client is changing APs more often than expected. Often caused by overlapping cell coverage or aggressive roaming thresholds.",
                    action:
                        "Reduce AP transmit power or raise the client min-RSSI threshold.",
                });
            }
            if (roamingReport.stickyClient) {
                findings.push({
                    severity: "warn",
                    code: `W-${pad2(nextW++)}`,
                    title: "Sticky-client behavior detected",
                    description:
                        "The client is lingering on weak APs instead of roaming. Performance will suffer near the cell edge.",
                    action:
                        "Lower the min-RSSI threshold on the AP, or enable 802.11k/v BSS Transition Management.",
                });
            }
        }

        return findings;
    }

    function severityCounts(findings) {
        const out = { critical: 0, warning: 0, info: 0, pass: 0 };
        for (const f of findings) {
            if (f.severity === "bad") out.critical++;
            else if (f.severity === "warn") out.warning++;
            else if (f.severity === "info") out.info++;
            else if (f.severity === "ok") out.pass++;
        }
        return out;
    }

    function gradeFromFindings(findings, signalAvg) {
        const counts = severityCounts(findings);
        let score = 100;
        score -= counts.critical * 25;
        score -= counts.warning * 8;
        score -= counts.info * 2;
        if (typeof signalAvg === "number") {
            if (signalAvg < -75) score -= 20;
            else if (signalAvg < -67) score -= 10;
            else if (signalAvg < -60) score -= 4;
        }
        score = Math.max(0, Math.min(100, score));
        let grade;
        let tone = "ok";
        if (score >= 95) grade = "A+";
        else if (score >= 90) grade = "A";
        else if (score >= 85) grade = "A−";
        else if (score >= 80) {
            grade = "B+";
        } else if (score >= 75) {
            grade = "B";
            tone = "warn";
        } else if (score >= 70) {
            grade = "B−";
            tone = "warn";
        } else if (score >= 60) {
            grade = "C";
            tone = "warn";
        } else if (score >= 50) {
            grade = "D";
            tone = "bad";
        } else {
            grade = "F";
            tone = "bad";
        }
        const label =
            tone === "ok"
                ? "Healthy"
                : tone === "warn"
                  ? "Needs attention"
                  : "Action required";
        return { grade, score, tone, label };
    }

    async function fetchRoamingReport() {
        try {
            if (typeof GetRoamingAnalysis === "function") {
                return await GetRoamingAnalysis();
            }
        } catch (err) {
            console.warn("GetRoamingAnalysis failed:", err);
        }
        return null;
    }


    async function buildHtmlReport() {
        const ts = reportTimestamp();
        const roamingReport = await fetchRoamingReport();
        const stats = clientStats || {};
        const connected = !!stats.connected;
        const sig = summarizeSignalHistory(stats.signalHistory);
        const connectedAP = findConnectedAP(networks, stats.bssid);
        const ap = connectedAP?.ap || null;
        const survey = summarizeBands(networks);
        const findings = deriveFindings(connectedAP, networks, roamingReport);
        const sevCounts = severityCounts(findings);
        const grade = gradeFromFindings(findings, sig.avg ?? stats.signalAvg);
        const recentRoams = (stats.roamingHistory || [])
            .slice()
            .reverse()
            .slice(0, 8);

        const ssid = stats.ssid || "(not connected)";
        const bssid = stats.bssid || "—";
        const bandStr = stats.frequency
            ? stats.frequency >= 5925
                ? "6 GHz"
                : stats.frequency >= 4900
                  ? "5 GHz"
                  : "2.4 GHz"
            : "—";
        const channel = stats.channel != null ? stats.channel : "—";
        const channelWidth = stats.channelWidth || ap?.channelWidth || "—";
        const security = stats.security || ap?.security || "—";
        const vendor = ap?.vendor || "—";

        const titleSuffix = connected
            ? `${escapeHtml(ssid)} · ${bandStr} / ch ${escapeHtml(channel)}`
            : "No active connection";

        // ── Page 2: Networks table rows ───────────────────────
        const sortedNetworks = (networks || []).slice().sort((a, b) => {
            // Connected first, then by best signal desc.
            const aConn =
                a.bestSignalAP &&
                a.bestSignalAP.toLowerCase() === (bssid || "").toLowerCase();
            const bConn =
                b.bestSignalAP &&
                b.bestSignalAP.toLowerCase() === (bssid || "").toLowerCase();
            if (aConn !== bConn) return aConn ? -1 : 1;
            return (b.bestSignal || -100) - (a.bestSignal || -100);
        });

        const networkRows = sortedNetworks
            .slice(0, 24)
            .map((n) => {
                const bestAP =
                    (n.accessPoints || []).find(
                        (a) => a && a.bssid === n.bestSignalAP,
                    ) || (n.accessPoints || [])[0];
                if (!bestAP) return "";
                const isConn =
                    bssid &&
                    bestAP.bssid &&
                    bssid.toLowerCase() === bestAP.bssid.toLowerCase();
                const ssidLabel = n.ssid || "(hidden)";
                const securityLabel = bestAP.security || n.security || "—";
                const secChip =
                    securityLabel.includes("WPA3")
                        ? `<span class="chip ok">${escapeHtml(securityLabel)}</span>`
                        : (securityLabel || "").toLowerCase() === "open"
                          ? `<span class="chip bad">Open</span>`
                          : `<span class="chip">${escapeHtml(securityLabel)}</span>`;
                const statusCell = isConn
                    ? `<span class="chip acc">Connected</span>`
                    : (n.ssid || "").length === 0
                      ? "Hidden"
                      : "OK";
                return `<tr${isConn ? ' class="connected"' : ""}>
                    <td>
                        <div class="ssid-cell">
                            <strong>${escapeHtml(ssidLabel)}</strong>
                            <span class="bssid">${escapeHtml(bestAP.bssid || "")} · ${escapeHtml(bestAP.vendor || "—")}</span>
                        </div>
                    </td>
                    <td class="mono">${escapeHtml(bestAP.band || "—")}</td>
                    <td class="num">${escapeHtml(bestAP.channel ?? "—")}</td>
                    <td class="mono">${sigBarsHTML(bestAP.signal)} ${escapeHtml(fmtDbm(bestAP.signal))}</td>
                    <td class="mono">${escapeHtml(bestAP.channelWidth || "—")}</td>
                    <td>${secChip}</td>
                    <td>${statusCell}</td>
                </tr>`;
            })
            .join("");

        // ── Page 3: 5 GHz spectrum (simplified bell-curve overlay)
        const fiveGHzAPs = (networks || [])
            .flatMap((n) =>
                (n.accessPoints || []).map((a) => ({
                    ssid: a.ssid || n.ssid || "—",
                    bssid: a.bssid,
                    band: a.band,
                    signal: a.signal,
                    channel: a.channel,
                    channelWidth: a.channelWidth || 20,
                })),
            )
            .filter((a) => a.band === "5GHz" && typeof a.channel === "number")
            .sort((a, b) => a.signal - b.signal); // weakest first
        const ch5List = [
            36, 40, 44, 48, 52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124,
            128, 132, 136, 140, 144, 149, 153, 157, 161, 165,
        ];
        const sw = 700;
        const sh = 80;
        const sPad = { left: 0, right: 0, top: 8, bottom: 18 };
        const sInnerW = sw - sPad.left - sPad.right;
        const sInnerH = sh - sPad.top - sPad.bottom;
        const sStep = sInnerW / Math.max(1, ch5List.length - 1);
        const xCh = (ch) => {
            const i = ch5List.indexOf(ch);
            if (i < 0) return -1;
            return sPad.left + i * sStep;
        };
        const yDb = (dBm) => {
            const yMin = -90;
            const yMax = -30;
            return (
                sPad.top + ((yMax - dBm) / (yMax - yMin)) * sInnerH
            );
        };
        const baseY = sPad.top + sInnerH;
        const bellPath = (cx, cy, w) => {
            const w2 = w / 2;
            return (
                `M${cx - w2},${baseY} ` +
                `C${cx - w2 * 0.35},${baseY} ${cx - w2 * 0.45},${cy} ${cx},${cy} ` +
                `C${cx + w2 * 0.45},${cy} ${cx + w2 * 0.35},${baseY} ${cx + w2},${baseY} Z`
            );
        };
        let bellsSvg = "";
        for (const a of fiveGHzAPs) {
            const cx = xCh(a.channel);
            if (cx < 0) continue;
            const cy = yDb(a.signal);
            const w = sStep * (a.channelWidth / 20);
            const isConn =
                bssid &&
                a.bssid &&
                bssid.toLowerCase() === a.bssid.toLowerCase();
            const color = isConn
                ? "#0b6e74"
                : a.signal >= -60
                  ? "#15803d"
                  : a.signal >= -72
                    ? "#a16207"
                    : "#b91c1c";
            bellsSvg += `<path d="${bellPath(cx, cy, w)}" fill="${color}" opacity="${isConn ? 0.3 : 0.22}" stroke="${color}" stroke-width="${isConn ? 1.6 : 1.2}"/>`;
            bellsSvg += `<text x="${cx}" y="${cy - 4}" font-family="Inter" font-size="9" font-weight="${isConn ? 700 : 600}" fill="${color}" text-anchor="middle">${escapeHtml(a.ssid.length > 14 ? a.ssid.slice(0, 13) + "…" : a.ssid)} · ${escapeHtml(a.signal)}</text>`;
        }
        let chTicks = '<g font-family="JetBrains Mono" font-size="8.5" fill="#6b7383" text-anchor="middle">';
        const showCh = [36, 44, 52, 60, 100, 108, 116, 132, 149, 157, 165];
        const connCh = stats.channel;
        for (const ch of showCh) {
            const x = xCh(ch);
            if (x < 0) continue;
            const isConn = ch === connCh;
            chTicks += `<text x="${x}" y="${baseY + 14}" ${isConn ? 'fill="#0b6e74" font-weight="600"' : ""}>${ch}</text>`;
        }
        chTicks += "</g>";

        // ── Page 3: 2.4 GHz channel grid ──────────────────────
        const ch24Counts = {};
        for (const n of networks || []) {
            for (const a of n.accessPoints || []) {
                if (a.band === "2.4GHz" && typeof a.channel === "number") {
                    ch24Counts[a.channel] = (ch24Counts[a.channel] || 0) + 1;
                }
            }
        }
        const ch24Cells = Array.from({ length: 14 }, (_, i) => i + 1)
            .map((ch) => {
                const c = ch24Counts[ch] || 0;
                let style;
                if (c === 0) {
                    style =
                        "background:#fafbfc; border:1px solid #e4e7ec; color:#9ca3af;";
                } else if (c <= 2) {
                    style =
                        "background:#e8f4ec; border:1px solid #bfe0c9; color:#15803d; font-weight:600;";
                } else if (c <= 4) {
                    style =
                        "background:#faf3e2; border:1px solid #ecdab0; color:#a16207; font-weight:600;";
                } else {
                    style =
                        "background:#fbe9e9; border:1px solid #f1c4c4; color:#b91c1c; font-weight:600;";
                }
                return `<div style="${style} aspect-ratio:1; border-radius:2px; font-family:var(--mono); font-size:8.5px; display:flex; align-items:center; justify-content:center">${ch}</div>`;
            })
            .join("");
        const ch24Total = Object.values(ch24Counts).reduce((a, b) => a + b, 0);
        const cleanest = [1, 6, 11].reduce(
            (best, ch) =>
                (ch24Counts[ch] || 0) < (ch24Counts[best] || 0) ? ch : best,
            1,
        );

        // ── Page 3: Selected channel KV ───────────────────────
        const selFreq =
            stats.frequency &&
            !Number.isNaN(stats.frequency)
                ? `${(stats.frequency / 1000).toFixed(3)} GHz`
                : "—";
        const apsOnSel = (networks || [])
            .flatMap((n) => n.accessPoints || [])
            .filter((a) => a.channel === stats.channel && a.band === ap?.band);
        const strongest =
            apsOnSel.length > 0
                ? Math.max(...apsOnSel.map((a) => a.signal))
                : null;
        const avgSelDbm =
            apsOnSel.length > 0
                ? Math.round(
                      apsOnSel.reduce((a, b) => a + b.signal, 0) /
                          apsOnSel.length,
                  )
                : null;

        // ── Page 4: Roaming KPIs + timeline ───────────────────
        const totalRoams = roamingReport?.totalRoams ?? 0;
        const goodRoams = roamingReport?.goodRoams ?? 0;
        const badRoams = roamingReport?.badRoams ?? 0;
        const avgRoamMs = roamingReport?.avgRoamDurationMs ?? null;
        const slowRoams = roamingReport?.slowRoamCount ?? 0;
        const ftUsed = ap?.fastroaming;
        const neighborReport = ap?.neighborReport;
        const bssTransition = ap?.bsstransition;

        const roamRows = recentRoams
            .map((e) => {
                const delta = (e.newSignal ?? 0) - (e.previousSignal ?? 0);
                const deltaCol =
                    delta >= 6
                        ? "var(--ok)"
                        : delta >= 0
                          ? "var(--warn)"
                          : "var(--bad)";
                const verdictTone =
                    delta >= 6 ? "ok" : delta >= 0 ? "warn" : "bad";
                const verdictText =
                    delta >= 6
                        ? "Good"
                        : delta >= 0
                          ? "Marginal"
                          : "Regression";
                const ts = new Date(e.timestamp);
                const tStr = `${pad2(ts.getHours())}:${pad2(ts.getMinutes())}:${pad2(ts.getSeconds())}`;
                return `<tr>
                    <td class="mono">${tStr}</td>
                    <td class="mono">${escapeHtml(e.previousBssid || "—")}</td>
                    <td class="mono mut">→</td>
                    <td class="mono">${escapeHtml(e.newBssid || "—")}</td>
                    <td class="mono">${escapeHtml(e.previousSignal ?? "—")}</td>
                    <td class="mono">${escapeHtml(e.newSignal ?? "—")}</td>
                    <td class="mono" style="color:${deltaCol}">${delta >= 0 ? "+" : ""}${escapeHtml(delta)}</td>
                    <td class="mono">${fmtMs(e.durationMs)}</td>
                    <td><span class="chip ${verdictTone}">${verdictText}</span></td>
                </tr>`;
            })
            .join("");

        // ── Page 4: Findings ──────────────────────────────────
        const findingsHtml = findings
            .map((f) => {
                const sevName =
                    f.severity === "bad"
                        ? "Critical"
                        : f.severity === "warn"
                          ? "Warning"
                          : f.severity === "info"
                            ? "Info"
                            : "Pass";
                return `<div class="finding ${f.severity}">
                    <div class="finding-sev">
                        <div class="sev-label">${sevName}</div>
                        <div class="sev-code">${escapeHtml(f.code)}</div>
                    </div>
                    <div class="finding-body">
                        <div class="finding-title">${escapeHtml(f.title)}</div>
                        <div class="finding-desc">${f.description}</div>
                        ${f.action ? `<div class="finding-action"><span class="action-label">Action</span><span class="action-text">${f.action}</span></div>` : ""}
                    </div>
                </div>`;
            })
            .join("");

        // ── Page 5: Capabilities KV ──────────────────────────
        const yesNo = (v) =>
            v === true || v === "true"
                ? "Supported"
                : v === false || v === "false"
                  ? "Not supported"
                  : v == null
                    ? "—"
                    : escapeHtml(v);
        const capsKV = ap
            ? [
                  ["WiFi Standard", escapeHtml(ap.wifiStandard || "—")],
                  ["WiFi Generation", escapeHtml(ap.wifiGeneration || "—")],
                  ["BSS Transition (802.11v)", yesNo(ap.bsstransition)],
                  ["Fast BSS Transition (.11r)", yesNo(ap.fastroaming)],
                  ["Neighbor Report (802.11k)", yesNo(ap.neighborReport)],
                  ["UAPSD (U-APSD)", yesNo(ap.uapsd)],
                  ["PMF (802.11w)", escapeHtml(ap.pmf || "—")],
                  ["Auth methods", escapeHtml((ap.authMethods || []).join(", ") || "—")],
                  ["Encryption cipher", escapeHtml((ap.securityCiphers || []).join(", ") || "—")],
                  ["WPS", ap.wps ? "Enabled" : "Disabled"],
                  ["Max QAM", ap.qamSupport ? `${ap.qamSupport}-QAM` : "—"],
                  ["MU-MIMO", yesNo(ap.mumimo)],
                  ["Beamforming", yesNo(ap.beamforming)],
                  ["BSS Color", escapeHtml(ap.bssColor ?? "—")],
                  ["OBSS PD (Spatial Reuse)", yesNo(ap.obssPD)],
                  ["QoS (WMM)", yesNo(ap.qosSupport)],
                  ["DTIM interval", escapeHtml(ap.dtim ?? "—")],
                  ["MIMO streams", ap.mimoStreams ? `${ap.mimoStreams} × ${ap.mimoStreams}` : "—"],
                  ["Max PHY rate", ap.maxPhyRate > 0 ? `${ap.maxPhyRate} Mbps` : "—"],
                  ["BSS load utilization", ap.bssLoadUtilization != null ? `${ap.bssLoadUtilization} %` : "—"],
                  ["Connected stations", ap.bssLoadStations ?? "—"],
                  ["Country code", escapeHtml(ap.countryCode || "—")],
              ]
            : [];

        const css = `
        :root {
            --ink-1: #0f141b; --ink-2: #3b4352; --ink-3: #6b7383; --ink-4: #9ca3af;
            --rule: #e4e7ec; --rule-2: #cfd4dc;
            --bg: #fafbfc; --panel: #ffffff; --panel-2: #f4f6f9;
            --acc: #0b6e74; --acc-tint: #e5f4f5;
            --ok: #15803d; --ok-tint: #e8f4ec;
            --warn: #a16207; --warn-tint: #faf3e2;
            --bad: #b91c1c; --bad-tint: #fbe9e9;
            --ui: 'Inter', -apple-system, BlinkMacSystemFont, 'Helvetica Neue', sans-serif;
            --mono: 'JetBrains Mono', ui-monospace, 'SF Mono', Menlo, monospace;
        }
        * { box-sizing: border-box; }
        html, body { margin: 0; padding: 0; }
        body { background: var(--bg); color: var(--ink-1); font-family: var(--ui); font-size: 11px; line-height: 1.5; -webkit-font-smoothing: antialiased; padding: 32px 24px 40px; }
        .page { width: 820px; margin: 0 auto 32px; background: var(--panel); padding: 56px 56px 64px; box-shadow: 0 1px 3px rgba(0,0,0,0.05), 0 8px 30px rgba(0,0,0,0.06); position: relative; }
        .page-header { display: flex; justify-content: space-between; align-items: flex-start; padding-bottom: 18px; border-bottom: 2px solid var(--ink-1); margin-bottom: 28px; }
        .brand { display: flex; align-items: center; gap: 10px; }
        .brand-mark { width: 32px; height: 32px; background: var(--ink-1); color: #fff; display: flex; align-items: center; justify-content: center; border-radius: 2px; }
        .brand-name { font-size: 13px; font-weight: 700; letter-spacing: -0.01em; }
        .brand-sub { font-family: var(--mono); font-size: 10px; color: var(--ink-3); text-transform: uppercase; letter-spacing: 0.12em; margin-top: 1px; }
        .report-meta { text-align: right; font-family: var(--mono); font-size: 10px; color: var(--ink-3); line-height: 1.6; }
        .report-meta .k { display: inline-block; width: 82px; text-align: left; color: var(--ink-4); }
        .report-meta .v { color: var(--ink-1); }
        .report-title-block { margin-bottom: 32px; }
        .report-title { font-size: 26px; font-weight: 700; letter-spacing: -0.02em; line-height: 1.15; color: var(--ink-1); margin: 0 0 6px; }
        .report-kicker { font-family: var(--mono); font-size: 10px; text-transform: uppercase; letter-spacing: 0.18em; color: var(--acc); font-weight: 600; margin-bottom: 10px; }
        .report-subtitle { font-size: 13px; color: var(--ink-2); font-weight: 400; max-width: 56ch; }
        section { margin-bottom: 32px; page-break-inside: avoid; }
        .section-head { display: flex; align-items: baseline; gap: 12px; border-bottom: 1px solid var(--ink-1); padding-bottom: 6px; margin-bottom: 14px; }
        .section-num { font-family: var(--mono); font-size: 10px; color: var(--acc); font-weight: 600; letter-spacing: 0.1em; }
        .section-title { font-size: 14px; font-weight: 700; letter-spacing: -0.005em; margin: 0; flex: 1; }
        .section-head .aside { font-size: 10px; color: var(--ink-3); font-family: var(--mono); }
        .sub-head { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.1em; color: var(--ink-2); margin: 16px 0 8px; padding-bottom: 3px; border-bottom: 1px dashed var(--rule); }
        p { margin: 0 0 10px; color: var(--ink-2); }
        .verdict { display: grid; grid-template-columns: 140px 1fr; gap: 0; border: 1px solid var(--rule-2); border-radius: 3px; overflow: hidden; background: var(--panel); }
        .verdict-badge { padding: 22px 18px; display: flex; flex-direction: column; justify-content: center; align-items: center; text-align: center; border-right: 1px solid var(--rule-2); }
        .verdict-badge.ok { background: var(--ok-tint); }
        .verdict-badge.warn { background: var(--warn-tint); }
        .verdict-badge.bad { background: var(--bad-tint); }
        .verdict-badge .grade { font-size: 42px; font-weight: 700; letter-spacing: -0.04em; line-height: 1; }
        .verdict-badge.ok .grade, .verdict-badge.ok .grade-label { color: var(--ok); }
        .verdict-badge.warn .grade, .verdict-badge.warn .grade-label { color: var(--warn); }
        .verdict-badge.bad .grade, .verdict-badge.bad .grade-label { color: var(--bad); }
        .verdict-badge .grade-label { font-family: var(--mono); font-size: 9px; text-transform: uppercase; letter-spacing: 0.14em; font-weight: 600; margin-top: 6px; }
        .verdict-score { font-family: var(--mono); font-size: 10px; color: var(--ink-3); margin-top: 10px; }
        .verdict-body { padding: 16px 20px; }
        .verdict-headline { font-size: 13px; font-weight: 600; color: var(--ink-1); margin-bottom: 6px; }
        .verdict-summary { font-size: 11.5px; color: var(--ink-2); line-height: 1.55; }
        .kpi-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 0; border: 1px solid var(--rule-2); border-radius: 3px; overflow: hidden; margin-top: 12px; }
        .kpi { padding: 12px 14px; border-right: 1px solid var(--rule); background: var(--panel); }
        .kpi:last-child { border-right: none; }
        .kpi-label { font-family: var(--mono); font-size: 9px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.12em; color: var(--ink-3); margin-bottom: 4px; }
        .kpi-value { font-family: var(--mono); font-size: 18px; font-weight: 500; color: var(--ink-1); letter-spacing: -0.01em; }
        .kpi-sub { font-family: var(--mono); font-size: 9.5px; color: var(--ink-3); margin-top: 2px; }
        .kpi-value.ok { color: var(--ok); }
        .kpi-value.warn { color: var(--warn); }
        .kpi-value.bad { color: var(--bad); }
        table.data { width: 100%; border-collapse: collapse; font-size: 10.5px; }
        table.data thead th { text-align: left; font-family: var(--mono); font-size: 9px; text-transform: uppercase; letter-spacing: 0.1em; color: var(--ink-3); font-weight: 600; padding: 6px 10px 6px 0; border-bottom: 1.5px solid var(--ink-1); white-space: nowrap; }
        table.data tbody td { padding: 8px 10px 8px 0; border-bottom: 1px solid var(--rule); vertical-align: top; }
        table.data tbody tr.connected { background: var(--acc-tint); }
        table.data tbody tr.connected td:first-child { border-left: 2px solid var(--acc); padding-left: 8px; }
        table.data .num { text-align: right; font-family: var(--mono); font-variant-numeric: tabular-nums; }
        table.data .mono { font-family: var(--mono); font-variant-numeric: tabular-nums; }
        .ssid-cell { display: flex; flex-direction: column; gap: 2px; }
        .ssid-cell .bssid { font-family: var(--mono); font-size: 9.5px; color: var(--ink-3); }
        .chip { display: inline-flex; align-items: center; gap: 4px; padding: 1px 6px; border: 1px solid var(--rule-2); border-radius: 10px; font-size: 9.5px; font-family: var(--mono); color: var(--ink-2); background: var(--panel); white-space: nowrap; }
        .chip.ok { color: var(--ok); border-color: #bfe0c9; background: var(--ok-tint); }
        .chip.warn { color: var(--warn); border-color: #ecdab0; background: var(--warn-tint); }
        .chip.bad { color: var(--bad); border-color: #f1c4c4; background: var(--bad-tint); }
        .chip.acc { color: var(--acc); border-color: #b8dcde; background: var(--acc-tint); }
        .kv-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 0 28px; border-top: 1px solid var(--rule); border-bottom: 1px solid var(--rule); }
        .kv-row { display: flex; justify-content: space-between; padding: 6px 0; border-bottom: 1px dashed var(--rule); font-size: 11px; }
        .kv-row:last-child, .kv-row.no-border { border-bottom: none; }
        .kv-row .k { color: var(--ink-3); font-weight: 500; }
        .kv-row .v { color: var(--ink-1); font-family: var(--mono); }
        .findings { display: flex; flex-direction: column; gap: 10px; }
        .finding { display: grid; grid-template-columns: 90px 1fr; gap: 0; border: 1px solid var(--rule-2); border-radius: 3px; overflow: hidden; }
        .finding-sev { padding: 12px 10px; text-align: center; border-right: 1px solid var(--rule-2); display: flex; flex-direction: column; justify-content: center; gap: 3px; }
        .finding-sev .sev-label { font-family: var(--mono); font-size: 9px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.14em; }
        .finding-sev .sev-code { font-family: var(--mono); font-size: 9px; color: var(--ink-3); }
        .finding.info .finding-sev { background: var(--acc-tint); color: var(--acc); }
        .finding.ok .finding-sev { background: var(--ok-tint); color: var(--ok); }
        .finding.warn .finding-sev { background: var(--warn-tint); color: var(--warn); }
        .finding.bad .finding-sev { background: var(--bad-tint); color: var(--bad); }
        .finding-body { padding: 12px 14px; }
        .finding-title { font-size: 12px; font-weight: 600; color: var(--ink-1); margin-bottom: 4px; }
        .finding-desc { font-size: 10.5px; color: var(--ink-2); line-height: 1.55; margin-bottom: 8px; }
        .finding-action { font-size: 10.5px; padding-top: 6px; border-top: 1px dashed var(--rule); display: flex; gap: 8px; align-items: flex-start; }
        .finding-action .action-label { font-family: var(--mono); font-size: 9px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.12em; color: var(--ink-3); min-width: 72px; padding-top: 2px; }
        .finding-action .action-text { color: var(--ink-1); }
        .chart-frame { border: 1px solid var(--rule-2); border-radius: 3px; padding: 14px 14px 10px; background: var(--panel); margin-top: 8px; }
        .chart-title { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 8px; }
        .chart-title h4 { font-size: 11px; font-weight: 600; margin: 0; color: var(--ink-1); }
        .chart-title .legend { font-family: var(--mono); font-size: 9px; color: var(--ink-3); display: flex; gap: 12px; }
        .page-footer { margin-top: 36px; padding-top: 10px; border-top: 1px solid var(--rule); display: flex; justify-content: space-between; font-family: var(--mono); font-size: 9px; color: var(--ink-4); text-transform: uppercase; letter-spacing: 0.1em; }
        .raw-block { font-family: var(--mono); font-size: 9.5px; color: var(--ink-2); background: var(--panel-2); border: 1px solid var(--rule); padding: 10px 12px; border-radius: 2px; white-space: pre-wrap; line-height: 1.5; }
        .ch-legend { display: flex; gap: 14px; align-items: center; font-family: var(--mono); font-size: 9px; color: var(--ink-3); margin-bottom: 8px; }
        .ch-legend .sw { display: inline-block; width: 10px; height: 10px; margin-right: 4px; border: 1px solid var(--rule-2); vertical-align: middle; border-radius: 1px; }
        @media print { body { background: white; padding: 0; } .page { box-shadow: none; margin: 0; width: 100%; padding: 24mm 18mm; page-break-after: always; } .page:last-child { page-break-after: auto; } }
        @page { size: Letter; margin: 0; }
        .row { display: flex; gap: 16px; }
        .col { flex: 1; min-width: 0; }
        .mut { color: var(--ink-3); }
        .mono { font-family: var(--mono); }
        .sig-bar { display: inline-flex; align-items: flex-end; gap: 1px; height: 10px; vertical-align: -1px; }
        .sig-bar span { width: 2px; background: #cbd2dc; border-radius: 0.5px; }
        .sig-bar span.on.ok { background: var(--ok); }
        .sig-bar span.on.warn { background: var(--warn); }
        .sig-bar span.on.bad { background: var(--bad); }
        .confidential { display: inline-block; font-family: var(--mono); font-size: 8.5px; letter-spacing: 0.2em; color: var(--ink-3); border: 1px solid var(--rule-2); padding: 2px 6px; text-transform: uppercase; }
        .toc { border: 1px solid var(--rule-2); padding: 14px 18px; border-radius: 3px; background: var(--panel-2); }
        .toc-title { font-family: var(--mono); font-size: 9px; text-transform: uppercase; letter-spacing: 0.14em; color: var(--ink-3); margin-bottom: 8px; font-weight: 600; }
        .toc ol { margin: 0; padding: 0; list-style: none; counter-reset: toc; }
        .toc li { display: flex; align-items: baseline; font-size: 11px; padding: 3px 0; counter-increment: toc; }
        .toc li::before { content: counter(toc, decimal-leading-zero); font-family: var(--mono); font-size: 9px; color: var(--acc); font-weight: 600; min-width: 26px; letter-spacing: 0.1em; }
        .toc li .dots { flex: 1; border-bottom: 1px dotted var(--rule-2); margin: 0 8px; transform: translateY(-3px); }
        .toc li .pg { font-family: var(--mono); font-size: 10px; color: var(--ink-3); }
        `;

        const brandHeader = (page, total) => `
        <div class="page-header">
            <div class="brand">
                <div class="brand-mark">
                    <svg width="18" height="18" viewBox="0 0 16 16" fill="none">
                        <path d="M8 12v0M4.5 9a5 5 0 017 0M2 6.5a8 8 0 0112 0" stroke="#fff" stroke-width="1.5" stroke-linecap="round"/>
                        <circle cx="8" cy="12" r="1" fill="#fff"/>
                    </svg>
                </div>
                <div>
                    <div class="brand-name">WiFi Diagnostic</div>
                    <div class="brand-sub">${escapeHtml(ts.id)}</div>
                </div>
            </div>
            <div class="report-meta">
                <div><span class="k">Page</span><span class="v">${pad2(page)} / ${pad2(total)}</span></div>
            </div>
        </div>`;

        const pageFooter = (page, total) => `
        <div class="page-footer">
            <span>WiFi Diagnostic · ${escapeHtml(ts.id)}</span>
            <span class="confidential">Confidential · Page ${page} of ${total}</span>
        </div>`;

        const pages = 5;

        // ── Build pages ───────────────────────────────────────
        const page1 = `
        <div class="page">
            <div class="page-header">
                <div class="brand">
                    <div class="brand-mark">
                        <svg width="18" height="18" viewBox="0 0 16 16" fill="none">
                            <path d="M8 12v0M4.5 9a5 5 0 017 0M2 6.5a8 8 0 0112 0" stroke="#fff" stroke-width="1.5" stroke-linecap="round"/>
                            <circle cx="8" cy="12" r="1" fill="#fff"/>
                        </svg>
                    </div>
                    <div>
                        <div class="brand-name">WiFi Diagnostic</div>
                        <div class="brand-sub">diagnostic report</div>
                    </div>
                </div>
                <div class="report-meta">
                    <div><span class="k">Report ID</span><span class="v">${escapeHtml(ts.id)}</span></div>
                    <div><span class="k">Captured</span><span class="v">${escapeHtml(ts.full)}</span></div>
                    <div><span class="k">Duration</span><span class="v">${escapeHtml(fmtDurationSec(stats.connectedTime))}</span></div>
                    <div><span class="k">Interface</span><span class="v">${escapeHtml(stats.interface || "—")}</span></div>
                </div>
            </div>
            <div class="report-title-block">
                <div class="report-kicker">Diagnostic report</div>
                <h1 class="report-title">Wireless network assessment<br/>${titleSuffix}</h1>
                <p class="report-subtitle">
                    ${connected
                        ? `Captured on interface <span class="mono">${escapeHtml(stats.interface || "")}</span>. Includes environment survey, link metrics, roaming behavior, and prioritized recommendations.`
                        : `No active WiFi association at the time this report was generated. Network survey data is included for environment review.`}
                </p>
            </div>
            <section>
                <div class="section-head">
                    <span class="section-num">01</span>
                    <h2 class="section-title">Executive summary</h2>
                    <span class="aside">${sevCounts.warning + sevCounts.critical} recommendation${sevCounts.warning + sevCounts.critical === 1 ? "" : "s"} · ${sevCounts.critical} critical</span>
                </div>
                <div class="verdict">
                    <div class="verdict-badge ${grade.tone}">
                        <div class="grade">${escapeHtml(grade.grade)}</div>
                        <div class="grade-label">${escapeHtml(grade.label)}</div>
                        <div class="verdict-score">Score ${grade.score} / 100</div>
                    </div>
                    <div class="verdict-body">
                        <div class="verdict-headline">
                            ${connected
                                ? grade.tone === "ok"
                                    ? "Connection is strong and stable; review any non-critical findings."
                                    : "Connection has issues that warrant attention."
                                : "No active connection; only the environment survey applies."}
                        </div>
                        <div class="verdict-summary">
                            ${connected
                                ? `The client is associated to <span class="mono">${escapeHtml(ssid)}</span> on a ${escapeHtml(bandStr)} channel ` +
                                  `with ${sig.count > 0 ? `RSSI averaging <span class="mono">${fmtDbm(sig.avg)}</span> over <span class="mono">${sig.count}</span> samples` : "no signal history yet"}. ` +
                                  `Security: <span class="mono">${escapeHtml(security)}</span>. ` +
                                  `${findings.length > 0 ? `${findings.length} finding${findings.length === 1 ? "" : "s"} below.` : "No findings."}`
                                : "Connect to a network and run a scan to capture link metrics."}
                        </div>
                    </div>
                </div>
                <div class="kpi-row">
                    <div class="kpi">
                        <div class="kpi-label">Signal (avg)</div>
                        <div class="kpi-value ${signalToneClass(sig.avg ?? stats.signalAvg) || ""}">${fmtDbm(sig.avg ?? stats.signalAvg)}</div>
                        <div class="kpi-sub">${sig.count > 0 ? `worst ${fmtDbm(sig.min)} · best ${fmtDbm(sig.max)}` : "no samples"}</div>
                    </div>
                    <div class="kpi">
                        <div class="kpi-label">Link rate</div>
                        <div class="kpi-value">${fmtMbps(stats.txBitrate)}</div>
                        <div class="kpi-sub">TX${stats.wifiStandard ? ` · ${escapeHtml(stats.wifiStandard)}` : ""}</div>
                    </div>
                    <div class="kpi">
                        <div class="kpi-label">Retries</div>
                        <div class="kpi-value ${typeof stats.retryRate === "number" && stats.retryRate < 5 ? "ok" : stats.retryRate < 10 ? "warn" : "bad"}">${fmtPercent(stats.retryRate)}</div>
                        <div class="kpi-sub">last 60 s</div>
                    </div>
                    <div class="kpi">
                        <div class="kpi-label">Roaming</div>
                        <div class="kpi-value ${badRoams === 0 && totalRoams > 0 ? "ok" : badRoams > goodRoams ? "bad" : "warn"}">${goodRoams} / ${totalRoams}</div>
                        <div class="kpi-sub">${avgRoamMs ? `avg ${fmtMs(avgRoamMs)}` : "—"}</div>
                    </div>
                </div>
            </section>
            <section>
                <div class="section-head">
                    <span class="section-num">—</span>
                    <h2 class="section-title">Contents</h2>
                </div>
                <div class="toc">
                    <div class="toc-title">Sections</div>
                    <ol>
                        <li>Executive summary<span class="dots"></span><span class="pg">01</span></li>
                        <li>Environment &amp; connection<span class="dots"></span><span class="pg">02</span></li>
                        <li>Detected networks<span class="dots"></span><span class="pg">02</span></li>
                        <li>Signal quality timeline<span class="dots"></span><span class="pg">03</span></li>
                        <li>Channel analysis<span class="dots"></span><span class="pg">03</span></li>
                        <li>Roaming &amp; mobility<span class="dots"></span><span class="pg">04</span></li>
                        <li>Findings &amp; recommendations<span class="dots"></span><span class="pg">04</span></li>
                        <li>Appendix — raw data<span class="dots"></span><span class="pg">05</span></li>
                    </ol>
                </div>
            </section>
            ${pageFooter(1, pages)}
        </div>`;

        const page2 = `
        <div class="page">
            ${brandHeader(2, pages)}
            <section>
                <div class="section-head">
                    <span class="section-num">02</span>
                    <h2 class="section-title">Environment &amp; connection</h2>
                </div>
                <div class="kv-grid">
                    <div class="kv-row"><span class="k">Interface</span><span class="v">${escapeHtml(stats.interface || "—")}</span></div>
                    <div class="kv-row"><span class="k">Regulatory</span><span class="v">${escapeHtml(ap?.countryCode || "—")}</span></div>
                    <div class="kv-row"><span class="k">SSID</span><span class="v">${escapeHtml(ssid)}</span></div>
                    <div class="kv-row"><span class="k">BSSID</span><span class="v">${escapeHtml(bssid)}</span></div>
                    <div class="kv-row"><span class="k">Band / channel</span><span class="v">${escapeHtml(bandStr)} · ${escapeHtml(channel)} (${escapeHtml(channelWidth)} MHz)</span></div>
                    <div class="kv-row"><span class="k">Vendor (OUI)</span><span class="v">${escapeHtml(vendor)}</span></div>
                    <div class="kv-row"><span class="k">Security</span><span class="v">${escapeHtml(security)}${ap?.pmf ? ` · PMF ${escapeHtml(ap.pmf)}` : ""}</span></div>
                    <div class="kv-row"><span class="k">Session duration</span><span class="v">${escapeHtml(fmtDurationSec(stats.connectedTime))}</span></div>
                    <div class="kv-row"><span class="k">WiFi standard</span><span class="v">${escapeHtml(stats.wifiStandard || "—")}</span></div>
                    <div class="kv-row"><span class="k">MIMO</span><span class="v">${escapeHtml(stats.mimoConfig || (ap?.mimoStreams ? `${ap.mimoStreams}×${ap.mimoStreams}` : "—"))}</span></div>
                    <div class="kv-row"><span class="k">RX bytes</span><span class="v">${escapeHtml((stats.rxBytes || 0).toLocaleString())}</span></div>
                    <div class="kv-row"><span class="k">TX bytes</span><span class="v">${escapeHtml((stats.txBytes || 0).toLocaleString())}</span></div>
                </div>
            </section>
            <section>
                <div class="section-head">
                    <span class="section-num">03</span>
                    <h2 class="section-title">Detected networks</h2>
                    <span class="aside">${survey.ssidCount} SSIDs · ${survey.bssidCount} BSSIDs · ${survey.distinctBands} band${survey.distinctBands === 1 ? "" : "s"}</span>
                </div>
                <table class="data">
                    <thead>
                        <tr>
                            <th style="width:30%">SSID / BSSID</th>
                            <th>Band</th>
                            <th>Ch</th>
                            <th>Signal</th>
                            <th>Width</th>
                            <th>Security</th>
                            <th>Status</th>
                        </tr>
                    </thead>
                    <tbody>${networkRows || '<tr><td colspan="7" class="mut">No networks detected — start a scan first.</td></tr>'}</tbody>
                </table>
            </section>
            ${pageFooter(2, pages)}
        </div>`;

        const page3 = `
        <div class="page">
            ${brandHeader(3, pages)}
            <section>
                <div class="section-head">
                    <span class="section-num">04</span>
                    <h2 class="section-title">Signal quality timeline</h2>
                    <span class="aside">${sig.count} sample${sig.count === 1 ? "" : "s"}</span>
                </div>
                <div class="chart-frame">
                    <div class="chart-title">
                        <h4>RSSI — connected AP${connected ? ` (${escapeHtml(ssid)})` : ""}</h4>
                        <div class="legend">
                            <span>EXCELLENT ≥ −50</span>
                            <span>FAIR −60..−67</span>
                            <span>WEAK −67..−75</span>
                            <span>POOR &lt; −75</span>
                        </div>
                    </div>
                    ${buildSignalSvg(stats.signalHistory)}
                    <div style="display:flex; gap:24px; font-family:var(--mono); font-size:10px; color:var(--ink-3); margin-top:6px">
                        <span>MIN <strong style="color:var(--ink-1)">${fmtDbm(sig.min)}</strong></span>
                        <span>MAX <strong style="color:var(--ink-1)">${fmtDbm(sig.max)}</strong></span>
                        <span>AVG <strong style="color:var(--ink-1)">${fmtDbm(sig.avg)}</strong></span>
                        <span>σ <strong style="color:var(--ink-1)">${typeof sig.stddev === "number" ? sig.stddev.toFixed(1) + " dB" : "—"}</strong></span>
                        <span>SAMPLES <strong style="color:var(--ink-1)">${sig.count.toLocaleString()}</strong></span>
                    </div>
                </div>
                <div class="sub-head">Signal quality distribution</div>
                ${buildSignalDistributionSvg(sig)}
            </section>
            <section>
                <div class="section-head">
                    <span class="section-num">05</span>
                    <h2 class="section-title">Channel analysis</h2>
                    <span class="aside">${survey.distinctBands} band${survey.distinctBands === 1 ? "" : "s"} · ${survey.ssidCount} networks</span>
                </div>
                <div class="ch-legend">
                    <span><span class="sw" style="background:#fafbfc"></span>Empty</span>
                    <span><span class="sw" style="background:#e8f4ec; border-color:#bfe0c9"></span>Low (1–2 APs)</span>
                    <span><span class="sw" style="background:#faf3e2; border-color:#ecdab0"></span>Medium (3–4)</span>
                    <span><span class="sw" style="background:#fbe9e9; border-color:#f1c4c4"></span>High (5+)</span>
                    <span><span class="sw" style="background:#e5f4f5; border-color:#0b6e74"></span>Connected</span>
                </div>
                <div class="sub-head">5 GHz spectrum${connected && bandStr === "5 GHz" ? " — connected here" : ""}</div>
                <svg width="100%" height="${sh}" viewBox="0 0 ${sw} ${sh}" preserveAspectRatio="none" style="display:block; margin-bottom:6px">
                    <line x1="0" y1="${baseY}" x2="${sw}" y2="${baseY}" stroke="#cfd4dc"/>
                    ${bellsSvg || `<text x="${sw / 2}" y="${sh / 2}" text-anchor="middle" fill="#9ca3af" font-family="Inter" font-size="11">No 5 GHz APs detected.</text>`}
                    ${chTicks}
                </svg>
                <div class="row" style="gap:10px; margin-top:10px">
                    <div class="col">
                        <div class="sub-head" style="margin-top:0">2.4 GHz channels</div>
                        <div style="display:grid; grid-template-columns:repeat(14, 1fr); gap:3px;">${ch24Cells}</div>
                        <div class="mono" style="font-size:9.5px; color:var(--ink-3); margin-top:6px">
                            ${ch24Total} APs · busiest ch ${
                                Object.entries(ch24Counts).sort((a, b) => b[1] - a[1])[0]?.[0] ||
                                "—"
                            } · cleanest ch ${cleanest} (1/6/11)
                        </div>
                    </div>
                    <div class="col">
                        <div class="sub-head" style="margin-top:0">${connected ? `Selected channel: ${channel} (connected)` : "Selected channel"}</div>
                        ${connected
                            ? `<div class="kv-grid" style="border-top:none; border-bottom:none">
                                <div class="kv-row"><span class="k">Center frequency</span><span class="v">${escapeHtml(selFreq)}</span></div>
                                <div class="kv-row"><span class="k">Width</span><span class="v">${escapeHtml(channelWidth)} MHz</span></div>
                                <div class="kv-row"><span class="k">APs on channel</span><span class="v">${apsOnSel.length}</span></div>
                                <div class="kv-row"><span class="k">Strongest RSSI</span><span class="v">${fmtDbm(strongest)}</span></div>
                                <div class="kv-row"><span class="k">Average RSSI</span><span class="v">${fmtDbm(avgSelDbm)}</span></div>
                                <div class="kv-row"><span class="k">DFS</span><span class="v">${ap?.dfs ? "Yes" : "No"}</span></div>
                                <div class="kv-row"><span class="k">Channel utilization</span><span class="v">${ap?.bssLoadUtilization != null ? `${ap.bssLoadUtilization} %` : "—"}</span></div>
                                <div class="kv-row"><span class="k">Connected stations</span><span class="v">${ap?.bssLoadStations ?? "—"}</span></div>
                            </div>`
                            : `<div class="mut" style="font-size:11px">Not connected to a network.</div>`}
                    </div>
                </div>
            </section>
            ${pageFooter(3, pages)}
        </div>`;

        const page4 = `
        <div class="page">
            ${brandHeader(4, pages)}
            <section>
                <div class="section-head">
                    <span class="section-num">06</span>
                    <h2 class="section-title">Roaming &amp; mobility</h2>
                    <span class="aside">${totalRoams} event${totalRoams === 1 ? "" : "s"}</span>
                </div>
                <div class="kpi-row">
                    <div class="kpi"><div class="kpi-label">Total roams</div><div class="kpi-value">${totalRoams}</div><div class="kpi-sub">recent history</div></div>
                    <div class="kpi"><div class="kpi-label">Good roams</div><div class="kpi-value ok">${goodRoams}</div><div class="kpi-sub">improved ≥ 6 dBm</div></div>
                    <div class="kpi"><div class="kpi-label">Marginal</div><div class="kpi-value ${badRoams > 0 ? "warn" : ""}">${badRoams}</div><div class="kpi-sub">&lt; 6 dBm gain</div></div>
                    <div class="kpi"><div class="kpi-label">Avg transition</div><div class="kpi-value ${avgRoamMs != null && avgRoamMs >= 2000 ? "bad" : avgRoamMs != null && avgRoamMs >= 500 ? "warn" : "ok"}">${fmtMs(avgRoamMs)}</div><div class="kpi-sub">${ftUsed ? "FT enabled" : "FT not advertised"}</div></div>
                </div>
                <div class="sub-head">Roam timeline</div>
                ${recentRoams.length === 0
                    ? `<div class="mut" style="font-size:11px">No roam events captured during this session.</div>`
                    : `<table class="data">
                        <thead>
                            <tr>
                                <th>Time</th><th>From BSSID</th><th>→</th><th>To BSSID</th>
                                <th>Before</th><th>After</th><th>Δ</th><th>Duration</th><th>Verdict</th>
                            </tr>
                        </thead>
                        <tbody>${roamRows}</tbody>
                    </table>`}
                <div class="sub-head">Behavior</div>
                <div class="kv-grid">
                    <div class="kv-row"><span class="k">Excessive roaming</span><span class="v">${roamingReport?.excessiveRoaming ? '<span class="chip warn">Yes</span> — > 10/hr threshold' : '<span class="chip ok">No</span> — below threshold'}</span></div>
                    <div class="kv-row"><span class="k">Sticky client</span><span class="v">${roamingReport?.stickyClient ? '<span class="chip warn">Yes</span>' : '<span class="chip ok">No</span>'}</span></div>
                    <div class="kv-row"><span class="k">Slow roams (≥ 2 s)</span><span class="v">${slowRoams > 0 ? `<span class="chip warn">${slowRoams}</span>` : '<span class="chip ok">0</span>'}</span></div>
                    <div class="kv-row"><span class="k">802.11r (FT)</span><span class="v">${ftUsed ? '<span class="chip ok">Supported</span>' : '<span class="chip warn">Not advertised</span>'}</span></div>
                    <div class="kv-row"><span class="k">802.11v (BSS)</span><span class="v">${bssTransition ? '<span class="chip ok">Supported</span>' : '<span class="chip warn">Not advertised</span>'}</span></div>
                    <div class="kv-row"><span class="k">802.11k (Nbr)</span><span class="v">${neighborReport ? '<span class="chip ok">Supported</span>' : '<span class="chip warn">Not advertised</span>'}</span></div>
                </div>
            </section>
            <section>
                <div class="section-head">
                    <span class="section-num">07</span>
                    <h2 class="section-title">Findings &amp; recommendations</h2>
                    <span class="aside">${sevCounts.critical} critical · ${sevCounts.warning} warning · ${sevCounts.info} info · ${sevCounts.pass} pass</span>
                </div>
                <div class="findings">
                    ${findingsHtml || '<div class="mut" style="font-size:11px; padding:14px 0">No findings.</div>'}
                </div>
            </section>
            ${pageFooter(4, pages)}
        </div>`;

        const rawBlock = connected
            ? `Connected to ${escapeHtml(bssid)} (on ${escapeHtml(stats.interface || "—")})\n` +
              `  SSID: ${escapeHtml(ssid)}\n` +
              `  freq: ${escapeHtml(stats.frequency ?? "—")}\n` +
              `  RX: ${(stats.rxBytes || 0).toLocaleString()} bytes (${(stats.rxPackets || 0).toLocaleString()} packets)\n` +
              `  TX: ${(stats.txBytes || 0).toLocaleString()} bytes (${(stats.txPackets || 0).toLocaleString()} packets)\n` +
              `  signal: ${escapeHtml(stats.signal ?? "—")} dBm\n` +
              `  rx bitrate: ${escapeHtml(fmtMbps(stats.rxBitrate))}\n` +
              `  tx bitrate: ${escapeHtml(fmtMbps(stats.txBitrate))}\n` +
              (ap?.dtim ? `  dtim period: ${escapeHtml(ap.dtim)}\n` : "") +
              (ap?.beaconInt ? `  beacon int: ${escapeHtml(ap.beaconInt)}` : "")
            : "Not connected at the time of capture.";

        const page5 = `
        <div class="page">
            ${brandHeader(5, pages)}
            <section>
                <div class="section-head">
                    <span class="section-num">08</span>
                    <h2 class="section-title">Appendix — raw data</h2>
                </div>
                <div class="sub-head">Association details</div>
                <div class="raw-block">${rawBlock}</div>
                ${capsKV.length > 0
                    ? `<div class="sub-head">Advanced capabilities (negotiated)</div>
                        <div class="kv-grid">
                            ${capsKV.map(([k, v]) => `<div class="kv-row"><span class="k">${escapeHtml(k)}</span><span class="v">${v}</span></div>`).join("")}
                        </div>`
                    : ""}
                <div class="sub-head">Methodology</div>
                <p style="font-size:11px">
                    Data captured using the WiFi Diagnostic Tool over the current session.
                    Signal samples taken at the configured scan interval.
                    Roam events captured from kernel <span class="mono">nl80211</span> events;
                    BSS capabilities read from association beacons.
                    Report generated on-device; no data transmitted off-host.
                </p>
                <div class="sub-head">Terminology</div>
                <div class="kv-grid">
                    <div class="kv-row"><span class="k">RSSI</span><span class="v">Received Signal Strength Indicator (dBm)</span></div>
                    <div class="kv-row"><span class="k">PHY rate</span><span class="v">Physical-layer link rate (Mbps)</span></div>
                    <div class="kv-row"><span class="k">SNR</span><span class="v">Signal-to-Noise Ratio (dB)</span></div>
                    <div class="kv-row"><span class="k">DFS</span><span class="v">Dynamic Frequency Selection (radar avoidance)</span></div>
                    <div class="kv-row"><span class="k">FT</span><span class="v">Fast Transition (802.11r)</span></div>
                    <div class="kv-row"><span class="k">PMF</span><span class="v">Protected Management Frames (802.11w)</span></div>
                    <div class="kv-row"><span class="k">MCS</span><span class="v">Modulation and Coding Scheme</span></div>
                    <div class="kv-row"><span class="k">NSS</span><span class="v">Number of Spatial Streams</span></div>
                </div>
                <div style="margin-top:32px; display:flex; justify-content:space-between; align-items:flex-end">
                    <div>
                        <div style="font-size:10px; color:var(--ink-3); font-family:var(--mono); text-transform:uppercase; letter-spacing:0.12em">Prepared by</div>
                        <div style="font-size:12px; font-weight:600; margin-top:4px">WiFi Diagnostic Tool</div>
                        <div class="mono" style="font-size:10px; color:var(--ink-3); margin-top:2px">Automated report · no manual interpretation applied</div>
                    </div>
                    <div style="text-align:right">
                        <div style="font-size:10px; color:var(--ink-3); font-family:var(--mono); text-transform:uppercase; letter-spacing:0.12em">Signature</div>
                        <div style="width:180px; height:36px; border-bottom:1px solid var(--ink-2); margin-top:18px"></div>
                        <div class="mono" style="font-size:10px; color:var(--ink-3); margin-top:2px">Technician · date</div>
                    </div>
                </div>
            </section>
            <div class="page-footer">
                <span>WiFi Diagnostic · ${escapeHtml(ts.id)} · End of report</span>
                <span class="confidential">Confidential · Page 5 of 5</span>
            </div>
        </div>`;

        return `<!doctype html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>WiFi Diagnostic Report — ${escapeHtml(ts.ymd)}</title>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
<style>${css}</style>
</head>
<body>
${page1}
${page2}
${page3}
${page4}
${page5}
</body>
</html>`;
    }

    async function exportSelected() {
        if (exporting || !anySelected) return;
        exporting = true;
        lastError = "";
        try {
            if (sections.networks && networksAvailable) {
                if (formats.networks === "csv") {
                    await exportNetworksCsv();
                } else {
                    await exportNetworksJson();
                }
            }
            if (sections.stats && statsAvailable) {
                if (formats.stats === "json") {
                    await saveFile(
                        JSON.stringify(
                            sanitizeClientStats(clientStats),
                            null,
                            2,
                        ),
                        "client-stats.json",
                        "application/json",
                    );
                }
            }
            if (sections.report) {
                if (formats.report === "html") {
                    const html = await buildHtmlReport();
                    await saveFile(html, "wifi-report.html", "text/html");
                } else if (formats.report === "json") {
                    await saveFile(
                        JSON.stringify(buildReportData(), null, 2),
                        "wifi-report.json",
                        "application/json",
                    );
                }
            }
            dispatch("exported");
        } catch (err) {
            lastError = err?.message || String(err);
        } finally {
            exporting = false;
        }
    }
</script>

<div class="export-controls">
    <div class="section-list">
        <!-- Networks -->
        <div
            class="section-row"
            class:enabled={sections.networks && networksAvailable}
            class:disabled={!networksAvailable}
        >
            <input
                type="checkbox"
                bind:checked={sections.networks}
                disabled={!networksAvailable}
                aria-label="Include networks"
            />
            <div class="section-text">
                <div class="section-label">Networks</div>
                <div class="section-desc">
                    {networksAvailable
                        ? `SSID list + capabilities · ${networks.length} row${networks.length === 1 ? "" : "s"}`
                        : "No scan data yet"}
                </div>
            </div>
            <div class="section-formats">
                {#each FORMAT_OPTIONS.networks as fmt}
                    <button
                        type="button"
                        class:active={formats.networks === fmt}
                        on:click={() => (formats.networks = fmt)}
                        disabled={!networksAvailable}
                    >{fmt}</button>
                {/each}
            </div>
        </div>

        <!-- Client stats -->
        <div
            class="section-row"
            class:enabled={sections.stats && statsAvailable}
            class:disabled={!statsAvailable}
        >
            <input
                type="checkbox"
                bind:checked={sections.stats}
                disabled={!statsAvailable}
                aria-label="Include client stats"
            />
            <div class="section-text">
                <div class="section-label">Client stats</div>
                <div class="section-desc">
                    {statsAvailable
                        ? "Connection, signal quality, data rates"
                        : "Not connected to a network"}
                </div>
            </div>
            <div class="section-formats">
                {#each FORMAT_OPTIONS.stats as fmt}
                    <button
                        type="button"
                        class:active={formats.stats === fmt}
                        on:click={() => (formats.stats = fmt)}
                        disabled={!statsAvailable}
                    >{fmt}</button>
                {/each}
            </div>
        </div>

        <!-- Full diagnostic report -->
        <div class="section-row" class:enabled={sections.report}>
            <input
                type="checkbox"
                bind:checked={sections.report}
                aria-label="Include full diagnostic report"
            />
            <div class="section-text">
                <div class="section-label">Full diagnostic report</div>
                <div class="section-desc">
                    Networks + client stats bundled into one file
                </div>
            </div>
            <div class="section-formats">
                {#each FORMAT_OPTIONS.report as fmt}
                    <button
                        type="button"
                        class:active={formats.report === fmt}
                        on:click={() => (formats.report = fmt)}
                    >{fmt}</button>
                {/each}
            </div>
        </div>
    </div>

    {#if lastError}
        <div class="export-error">
            <span class="err-icon">⚠</span>
            <span>{lastError}</span>
        </div>
    {/if}

    <footer class="export-footer">
        <button
            type="button"
            class="footer-btn ghost"
            on:click={() => dispatch("close")}
        >Cancel</button>
        <button
            type="button"
            class="footer-btn primary"
            on:click={exportSelected}
            disabled={!anySelected || exporting}
        >
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path
                    d="M6 1.5v7M3 6l3 3 3-3M2 10.5h8"
                    stroke="currentColor"
                    stroke-width="1.4"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                />
            </svg>
            {exporting ? "Exporting…" : "Export selected"}
        </button>
    </footer>
</div>

<style>
    .export-controls {
        display: flex;
        flex-direction: column;
        font-family: var(--font-ui);
    }

    .section-list {
        display: flex;
        flex-direction: column;
        gap: 10px;
        padding: 18px 20px;
    }

    .section-row {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 10px 12px;
        border: 1px solid var(--line-1);
        border-radius: 6px;
        background: transparent;
        opacity: 0.6;
        transition:
            opacity 0.1s,
            background 0.1s;
    }

    .section-row.enabled {
        background: var(--bg-3);
        opacity: 1;
    }

    .section-row.disabled {
        opacity: 0.45;
    }

    .section-row input[type="checkbox"] {
        accent-color: var(--acc-1);
        margin: 0;
        cursor: pointer;
    }

    .section-text {
        flex: 1;
        min-width: 0;
    }

    .section-label {
        font-size: 12.5px;
        font-weight: 500;
        color: var(--fg-1);
    }

    .section-desc {
        font-size: 11px;
        color: var(--fg-3);
        margin-top: 2px;
    }

    .section-formats {
        display: inline-flex;
        background: var(--bg-3);
        border: 1px solid var(--line-2);
        border-radius: 6px;
        padding: 2px;
        gap: 2px;
    }

    .section-row.enabled .section-formats {
        background: var(--bg-2);
    }

    .section-formats button {
        background: transparent;
        border: none;
        color: var(--fg-2);
        font-size: 10.5px;
        font-family: var(--font-mono);
        text-transform: uppercase;
        padding: 4px 10px;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
        letter-spacing: 0.02em;
    }

    .section-formats button.active {
        background: var(--bg-1);
        color: var(--fg-1);
        box-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    }

    .section-formats button:disabled {
        opacity: 0.4;
        cursor: not-allowed;
    }

    .section-formats button:hover:not(.active):not(:disabled) {
        color: var(--fg-1);
    }

    .export-error {
        margin: 0 20px;
        padding: 8px 10px;
        background: var(--bad-bg);
        border: 1px solid var(--bad-line);
        color: var(--bad);
        border-radius: 6px;
        font-size: 11.5px;
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .err-icon {
        font-size: 13px;
    }

    .export-footer {
        display: flex;
        justify-content: flex-end;
        gap: 8px;
        padding: 12px 20px;
        border-top: 1px solid var(--line-1);
        background: var(--bg-2);
    }

    .footer-btn {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 6px 12px;
        border-radius: 5px;
        border: 1px solid var(--line-2);
        background: var(--bg-3);
        color: var(--fg-1);
        font-size: 12px;
        font-weight: 500;
        cursor: pointer;
        font-family: inherit;
    }

    .footer-btn.ghost {
        background: transparent;
    }

    .footer-btn.ghost:hover {
        background: var(--bg-3);
    }

    .footer-btn.primary {
        background: var(--acc-1);
        color: var(--text-on-accent);
        border-color: var(--acc-1);
        font-weight: 600;
    }

    .footer-btn.primary:hover:not(:disabled) {
        background: color-mix(in srgb, var(--acc-1) 80%, white);
    }

    .footer-btn.primary:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
</style>
