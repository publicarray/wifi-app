<script context="module">
    // apHistory lives at module scope so the per-other-AP signal traces
    // survive switching away from the Signal tab and back. The recorder
    // is also called from App.svelte's networks:updated handler so the
    // store keeps growing while SignalChart is unmounted — without that,
    // the chart resets to an empty history every time the user reopens
    // the Signal tab. The backend (WiFiService.recordAPSignalHistoryLocked)
    // is the durable source of truth across app restarts; this map is the
    // in-memory hot path that drives Chart.js datasets.
    const moduleApHistory = new Map();

    const HISTORY_WINDOW_MS = 60 * 60 * 1000;
    const HISTORY_MAX_POINTS = 1500;
    const STALE_HOLD_MS = 30000;

    // recordSignalsFromNetworks is exported so App.svelte can run the
    // sample recorder from its `networks:updated` event handler. Reactive
    // blocks inside SignalChart only fire while the component is mounted,
    // so without a top-level call site the per-AP traces flatline on tab
    // switch.
    export function recordSignalsFromNetworks(networks) {
        const now = Date.now();
        const seenBSSIDs = new Set();

        if (networks && networks.length > 0) {
            networks.forEach((network) => {
                if (!network?.accessPoints?.length) return;
                network.accessPoints.forEach((ap) => {
                    const bssid = ap?.bssid;
                    if (!bssid) return;
                    if (typeof ap.signal !== "number") return;
                    const rawSsid = ap?.ssid || network?.ssid || "";
                    const hidden = !rawSsid;
                    const ssid = rawSsid || "Unknown";
                    seenBSSIDs.add(bssid);

                    let entry = moduleApHistory.get(bssid);
                    if (!entry) {
                        entry = { bssid, ssid, points: [], band: "", hidden };
                        moduleApHistory.set(bssid, entry);
                    }
                    entry.ssid = ssid;
                    entry.hidden = hidden;
                    // Cache band for the band filter so we don't have to
                    // re-derive it from frequency on every chart render.
                    if (ap.band) {
                        entry.band = ap.band;
                    } else if (typeof ap.frequency === "number") {
                        entry.band =
                            ap.frequency >= 5925
                                ? "6GHz"
                                : ap.frequency >= 4900
                                  ? "5GHz"
                                  : ap.frequency > 0
                                    ? "2.4GHz"
                                    : "";
                    }
                    const lastPoint = entry.points[entry.points.length - 1];
                    if (!lastPoint || lastPoint.x !== now) {
                        entry.points.push({ x: now, y: ap.signal });
                    }

                    const cutoff = now - HISTORY_WINDOW_MS;
                    entry.points = entry.points.filter(
                        (point) => point.x >= cutoff,
                    );
                    if (entry.points.length > HISTORY_MAX_POINTS) {
                        entry.points.splice(
                            0,
                            entry.points.length - HISTORY_MAX_POINTS,
                        );
                    }
                });
            });
        }

        // Hold last-known signal for STALE_HOLD_MS so a single missed scan
        // doesn't punch a gap in the chart line.
        moduleApHistory.forEach((entry) => {
            if (seenBSSIDs.has(entry.bssid)) return;
            const lastPoint = entry.points?.[entry.points.length - 1];
            if (!lastPoint) return;
            if (now - lastPoint.x <= STALE_HOLD_MS) {
                if (lastPoint.x !== now) {
                    entry.points.push({ x: now, y: lastPoint.y });
                }
            }
        });

        // GC: drop empty entries and any whose last point fell out of the
        // retention window.
        moduleApHistory.forEach((entry, bssid) => {
            if (!entry.points || entry.points.length === 0) {
                moduleApHistory.delete(bssid);
                return;
            }
            const lastPoint = entry.points[entry.points.length - 1];
            if (lastPoint.x < now - HISTORY_WINDOW_MS) {
                moduleApHistory.delete(bssid);
            }
        });
    }

    // seedSignalHistoryFromBackend hydrates the in-memory store from the
    // persistent backend history (WiFiService.GetAPSignalHistories). Called
    // once on app startup so the Signal tab is non-empty even on the first
    // visit after launch.
    export function seedSignalHistoryFromBackend(histories) {
        if (!Array.isArray(histories)) return;
        const now = Date.now();
        const cutoff = now - HISTORY_WINDOW_MS;
        for (const h of histories) {
            if (!h?.bssid) continue;
            const points = (h.points || [])
                .map((p) => {
                    const x =
                        typeof p?.timestamp === "number"
                            ? p.timestamp
                            : Date.parse(p?.timestamp);
                    if (!Number.isFinite(x)) return null;
                    if (typeof p?.signal !== "number") return null;
                    return { x, y: p.signal };
                })
                .filter((pt) => pt && pt.x >= cutoff)
                .sort((a, b) => a.x - b.x);
            if (points.length === 0) continue;
            const existing = moduleApHistory.get(h.bssid);
            if (existing && existing.points.length >= points.length) continue;
            const rawSsid = h.ssid || "";
            moduleApHistory.set(h.bssid, {
                bssid: h.bssid,
                ssid: rawSsid || existing?.ssid || "Unknown",
                band: existing?.band || "",
                hidden: rawSsid ? false : (existing?.hidden ?? true),
                points,
            });
        }
    }

    export function apSignalHistoryTotals() {
        let aps = 0;
        let points = 0;
        moduleApHistory.forEach((entry) => {
            if (entry?.points?.length) {
                aps++;
                points += entry.points.length;
            }
        });
        return { aps, points };
    }
</script>

<script>
    import { onMount, onDestroy } from "svelte";
    import { Chart, registerables } from "chart.js";
    import "chartjs-adapter-date-fns";
    import { GetAPSignalHistory } from "../../wailsjs/go/main/App.js";

    export let clientStats = null;
    export let networks = [];

    let connectedChartElement;
    let othersChartElement;
    let connectedChart = null;
    let othersChart = null;
    let themeMedia = null;
    let apHistory = moduleApHistory;
    let historyPoints = 0;
    let historyAPs = 0;

    const RANGE_OPTIONS = [
        { id: "1m", label: "1m", ms: 60 * 1000 },
        { id: "5m", label: "5m", ms: 5 * 60 * 1000 },
        { id: "15m", label: "15m", ms: 15 * 60 * 1000 },
        { id: "1h", label: "1h", ms: 60 * 60 * 1000 },
        { id: "24h", label: "24h", ms: 24 * 60 * 60 * 1000 },
    ];
    let range = "5m";
    $: rangeMs =
        RANGE_OPTIONS.find((r) => r.id === range)?.ms ?? 5 * 60 * 1000;

    // ── Other-APs filters ────────────────────────────────────────
    // Default tuned for an MSP technician walking a site: 8 strongest
    // BSSIDs is enough to see overlap without legend clutter.
    const TOPN_OPTIONS = [
        { id: 5, label: "5" },
        { id: 8, label: "8" },
        { id: 10, label: "10" },
        { id: 20, label: "20" },
        { id: 0, label: "All" },
    ];
    const BAND_OPTIONS = [
        { id: "all", label: "All" },
        { id: "2.4GHz", label: "2.4" },
        { id: "5GHz", label: "5" },
        { id: "6GHz", label: "6" },
    ];
    let topN = 8;
    let ssidFilter = "";
    let bandFilter = "all";
    let showHidden = false;
    let othersTotal = 0;
    let othersVisible = 0;

    // Inline Chart.js plugin — paints horizontal RSSI quality zones
    // (Excellent/Good/Fair/Weak/Poor) behind the data series, with right-edge
    // band labels. Mirrors the design's reference SVG chart in
    // screenshots/design/screen-signal.jsx (see ll. 147–193). Inline (not
    // registered globally) so it only affects this component's charts.
    const qualityZonesPlugin = {
        id: "qualityZones",
        beforeDatasetsDraw(chart) {
            const { ctx, chartArea, scales } = chart;
            if (!chartArea || !scales || !scales.y) return;
            const yScale = scales.y;
            const left = chartArea.left;
            const width = chartArea.right - chartArea.left;

            const zones = [
                { from: -30, to: -50, fill: "rgba(74, 222, 128, 0.07)" },
                { from: -50, to: -60, fill: "rgba(74, 222, 128, 0.03)" },
                { from: -60, to: -67, fill: "rgba(251, 191, 36, 0.05)" },
                { from: -67, to: -75, fill: "rgba(251, 191, 36, 0.11)" },
                { from: -75, to: -100, fill: "rgba(248, 113, 113, 0.13)" },
            ];
            ctx.save();
            for (const z of zones) {
                const yA = yScale.getPixelForValue(z.from);
                const yB = yScale.getPixelForValue(z.to);
                const top = Math.min(yA, yB);
                const height = Math.abs(yB - yA);
                if (height <= 0) continue;
                ctx.fillStyle = z.fill;
                ctx.fillRect(left, top, width, height);
            }

            // Dashed dividers at the Fair→Weak (-67) and Weak→Poor (-75)
            // boundaries. Helps identify "below this line, expect issues".
            ctx.setLineDash([4, 4]);
            ctx.lineWidth = 1;

            ctx.strokeStyle = "rgba(251, 191, 36, 0.4)";
            const y67 = yScale.getPixelForValue(-67);
            ctx.beginPath();
            ctx.moveTo(left, y67);
            ctx.lineTo(chartArea.right, y67);
            ctx.stroke();

            ctx.strokeStyle = "rgba(248, 113, 113, 0.4)";
            const y75 = yScale.getPixelForValue(-75);
            ctx.beginPath();
            ctx.moveTo(left, y75);
            ctx.lineTo(chartArea.right, y75);
            ctx.stroke();

            ctx.setLineDash([]);
            ctx.restore();

            // Right-edge band labels
            ctx.save();
            ctx.font =
                "600 9px 'JetBrains Mono', ui-monospace, 'SF Mono', Menlo, monospace";
            ctx.textAlign = "right";
            ctx.textBaseline = "middle";
            const labels = [
                { y: -40, text: "EXCELLENT", color: "rgba(74, 222, 128, 0.75)" },
                { y: -63.5, text: "FAIR", color: "rgba(251, 191, 36, 0.75)" },
                { y: -71, text: "WEAK", color: "rgba(251, 191, 36, 0.85)" },
                { y: -82, text: "POOR", color: "rgba(248, 113, 113, 0.85)" },
            ];
            for (const l of labels) {
                const y = yScale.getPixelForValue(l.y);
                if (y < chartArea.top || y > chartArea.bottom) continue;
                ctx.fillStyle = l.color;
                ctx.fillText(l.text, chartArea.right - 4, y);
            }
            ctx.restore();
        },
    };

    onMount(async () => {
        Chart.register(...registerables);
        initializeChart();

        // Hydrate from the backend store so the chart is populated on the
        // first visit after app start. App.svelte also seeds at boot, but a
        // direct fetch here covers the case where SignalChart mounts before
        // the boot fetch resolves.
        try {
            const histories = await GetAPSignalHistory();
            seedSignalHistoryFromBackend(histories);
            const totals = apSignalHistoryTotals();
            historyAPs = totals.aps;
            historyPoints = totals.points;
            updateChart();
        } catch (err) {
            // Non-fatal — live recording will fill the chart on the next tick.
        }

        themeMedia = window.matchMedia("(prefers-color-scheme: dark)");
        const handleThemeChange = () => applyChartTheme();
        themeMedia.addEventListener("change", handleThemeChange);

        return () => {
            themeMedia?.removeEventListener("change", handleThemeChange);
        };
    });

    onDestroy(() => {
        connectedChart?.destroy();
        othersChart?.destroy();
    });

    function buildChart(ctx, titleText) {
        const theme = getThemeColors();
        return new Chart(ctx, {
            type: "line",
            data: {
                datasets: [],
            },
            plugins: [qualityZonesPlugin],
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: "index",
                    intersect: false,
                },
                layout: {
                    padding: {
                        top: 6,
                        right: 12,
                        bottom: 6,
                        left: 8,
                    },
                },
                plugins: {
                    title: {
                        display: true,
                        text: titleText,
                        color: theme.text,
                        padding: {
                            top: 0,
                            right: 0,
                            bottom: 0,
                            left: 0,
                        },
                        font: {
                            size: 16,
                            weight: "600",
                        },
                    },
                    legend: {
                        display: true,
                        position: "top",
                        labels: {
                            color: theme.text,
                            usePointStyle: true,
                            boxWidth: 8,
                            boxHeight: 8,
                            padding: 20,
                        },
                    },
                    tooltip: {
                        backgroundColor: theme.tooltipBg,
                        titleColor: theme.text,
                        bodyColor: theme.text,
                        borderColor: theme.border,
                        borderWidth: 1,
                        titleMarginBottom: 6,
                        bodySpacing: 6,
                        boxPadding: 6,
                        padding: 12,
                        displayColors: true,
                        callbacks: {
                            title: function (context) {
                                return new Date(
                                    context[0].parsed.x,
                                ).toLocaleTimeString();
                            },
                            label: function (context) {
                                return `${context.dataset.label}: ${context.parsed.y} dBm`;
                            },
                        },
                    },
                },
                scales: {
                    x: {
                        type: "time",
                        time: {
                            displayFormats: {
                                second: "HH:mm:ss",
                                minute: "HH:mm",
                                hour: "HH:mm",
                            },
                        },
                        title: {
                            display: true,
                            text: "Time",
                            color: theme.muted,
                        },
                        ticks: {
                            color: theme.muted,
                            maxRotation: 0,
                            autoSkip: true,
                        },
                        grid: {
                            color: theme.grid,
                            borderColor: theme.borderStrong,
                        },
                    },
                    y: {
                        title: {
                            display: true,
                            text: "Signal Strength (dBm)",
                            color: theme.muted,
                        },
                        ticks: {
                            color: theme.muted,
                            callback: function (value) {
                                return value + " dBm";
                            },
                        },
                        grid: {
                            color: theme.grid,
                            borderColor: theme.borderStrong,
                        },
                        min: -100,
                        max: -30,
                        reverse: false, // Higher values (less negative) are better signals
                    },
                },
            },
        });
    }

    function initializeChart() {
        const connectedCtx = connectedChartElement.getContext("2d");
        const othersCtx = othersChartElement.getContext("2d");
        connectedChart = buildChart(connectedCtx, "Connected AP Signal");
        othersChart = buildChart(othersCtx, "Other APs in Range");
    }

    // Update chart when clientStats, networks, range, or any "Other APs"
    // filter changes. Recording itself happens in App.svelte's
    // networks:updated handler so the store keeps growing while this tab
    // is unmounted; here we just refresh the toolbar totals and re-render
    // Chart.js datasets.
    $: if (connectedChart && othersChart) {
        networks;
        clientStats;
        rangeMs;
        topN;
        ssidFilter;
        bandFilter;
        showHidden;
        const totals = apSignalHistoryTotals();
        historyAPs = totals.aps;
        historyPoints = totals.points;
        updateChart();
    }

    function getThemeColors() {
        const styles = getComputedStyle(document.documentElement);
        const series = [];
        for (let i = 1; i <= 10; i++) {
            const value = styles.getPropertyValue(`--series-${i}`).trim();
            if (value) series.push(value);
        }
        return {
            text: styles.getPropertyValue("--text").trim() || "#e0e0e0",
            muted: styles.getPropertyValue("--muted").trim() || "#aaa",
            border: styles.getPropertyValue("--border").trim() || "#333",
            borderStrong:
                styles.getPropertyValue("--border-strong").trim() || "#444",
            grid:
                styles.getPropertyValue("--chart-grid").trim() ||
                "rgba(255,255,255,0.08)",
            tooltipBg:
                styles.getPropertyValue("--tooltip-bg").trim() ||
                "rgba(42,42,42,0.9)",
            warning: styles.getPropertyValue("--warning").trim() || "#ff9800",
            accent: styles.getPropertyValue("--accent").trim() || "#3b82f6",
            accentStrong:
                styles.getPropertyValue("--accent-strong").trim() || "#2563eb",
            series: series.length
                ? series
                : [
                      "#0066cc",
                      "#4caf50",
                      "#ff9800",
                      "#f44336",
                      "#9c27b0",
                      "#00bcd4",
                      "#8bc34a",
                      "#ffc107",
                      "#795548",
                      "#607d8b",
                  ],
        };
    }

    function applyChartTheme() {
        if (!connectedChart || !othersChart) return;
        const theme = getThemeColors();
        [connectedChart, othersChart].forEach((chart) => {
            chart.options.plugins.title.color = theme.text;
            chart.options.plugins.legend.labels.color = theme.text;
            chart.options.plugins.tooltip.backgroundColor = theme.tooltipBg;
            chart.options.plugins.tooltip.titleColor = theme.text;
            chart.options.plugins.tooltip.bodyColor = theme.text;
            chart.options.plugins.tooltip.borderColor = theme.border;
            chart.options.scales.x.title.color = theme.muted;
            chart.options.scales.x.ticks.color = theme.muted;
            chart.options.scales.x.grid.color = theme.grid;
            chart.options.scales.x.grid.borderColor = theme.borderStrong;
            chart.options.scales.y.title.color = theme.muted;
            chart.options.scales.y.ticks.color = theme.muted;
            chart.options.scales.y.grid.color = theme.grid;
            chart.options.scales.y.grid.borderColor = theme.borderStrong;
        });
        if (apHistory.size > 0) updateChart();
        else {
            connectedChart.update("none");
            othersChart.update("none");
        }
    }

    function withAlpha(color, alpha) {
        if (!color) return `rgba(0,0,0,${alpha})`;
        if (color.startsWith("#")) {
            let hex = color.slice(1);
            if (hex.length === 3) {
                hex = hex
                    .split("")
                    .map((c) => c + c)
                    .join("");
            }
            const num = parseInt(hex, 16);
            const r = (num >> 16) & 255;
            const g = (num >> 8) & 255;
            const b = num & 255;
            return `rgba(${r}, ${g}, ${b}, ${alpha})`;
        }
        if (color.startsWith("rgb(")) {
            return color.replace("rgb(", "rgba(").replace(")", `, ${alpha})`);
        }
        if (color.startsWith("rgba(")) {
            return color.replace(/rgba\\(([^)]+)\\)/, `rgba($1, ${alpha})`);
        }
        return color;
    }

    function normalizePoints(points) {
        if (!Array.isArray(points)) return [];
        return points
            .map((point) => {
                const x =
                    typeof point.timestamp === "number"
                        ? point.timestamp
                        : point.x;
                const parsed =
                    typeof x === "number"
                        ? x
                        : Date.parse(point.timestamp || point.x);
                const ts = Number.isNaN(parsed) ? Date.now() : parsed;
                const y =
                    typeof point.signal === "number" ? point.signal : point.y;
                if (typeof y !== "number") return null;
                return { x: ts, y };
            })
            .filter(Boolean)
            .sort((a, b) => a.x - b.x);
    }

    function filterToRange(points, cutoff) {
        if (!Array.isArray(points)) return [];
        const inRange = points.filter((p) => p && p.x >= cutoff);
        if (inRange.length === points.length) return inRange;
        // Carry the most recent pre-cutoff sample forward to anchor the line
        // at the left edge instead of starting mid-chart.
        const lastBefore = [...points]
            .reverse()
            .find((p) => p && p.x < cutoff);
        if (lastBefore) {
            return [{ x: cutoff, y: lastBefore.y }, ...inRange];
        }
        return inRange;
    }

    function applyRangeBounds(chart, now, cutoff) {
        chart.options.scales.x.min = cutoff;
        chart.options.scales.x.max = now;
        const span = now - cutoff;
        let unit = "minute";
        if (span <= 2 * 60 * 1000) unit = "second";
        else if (span <= 60 * 60 * 1000) unit = "minute";
        else unit = "hour";
        chart.options.scales.x.time.unit = unit;
    }

    function updateChart() {
        const now = Date.now();
        const cutoff = now - rangeMs;

        if (apHistory.size === 0) {
            applyRangeBounds(connectedChart, now, cutoff);
            applyRangeBounds(othersChart, now, cutoff);
            connectedChart.data.datasets = [];
            othersChart.data.datasets = [];
            connectedChart.update();
            othersChart.update();
            return;
        }

        const theme = getThemeColors();

        // Group signal data by BSSID to show multiple APs
        const signalDataByBSSID = {};
        apHistory.forEach((entry, bssid) => {
            if (!entry.points || entry.points.length === 0) return;
            signalDataByBSSID[bssid] = entry.points;
        });

        // Create datasets for each BSSID
        const connectedDatasets = [];
        const otherDatasets = [];
        const colors = theme.series;
        let colorIndex = 0;

        const connectedBSSID = clientStats?.bssid;
        const entries = Object.entries(signalDataByBSSID).sort(([a], [b]) => {
            if (a === connectedBSSID) return -1;
            if (b === connectedBSSID) return 1;
            return a.localeCompare(b);
        });

        const connectedHistory = normalizePoints(
            clientStats?.signalHistory || [],
        );
        const connectedHistoryWindowed = filterToRange(
            connectedHistory,
            cutoff,
        );
        if (connectedHistoryWindowed.length > 1 && connectedBSSID) {
            const label = `${clientStats.ssid || "Connected"} (${connectedBSSID})`;
            const baseColor = theme.accentStrong || theme.accent;
            connectedDatasets.push({
                label,
                data: connectedHistoryWindowed,
                borderColor: baseColor,
                backgroundColor: withAlpha(baseColor, 0.2),
                borderWidth: 3,
                borderDash: [],
                pointRadius: 2,
                pointHoverRadius: 6,
                pointBackgroundColor: baseColor,
                tension: 0.25,
                fill: "origin",
                showLine: true,
                spanGaps: true,
                order: 10,
            });
        }

        // Build the connected-AP dataset (always shown when connected) and
        // a candidate list of other APs that we'll filter + rank below.
        const ssidQuery = (ssidFilter || "").trim().toLowerCase();
        const otherCandidates = [];

        entries.forEach(([bssid, data]) => {
            const entry = apHistory.get(bssid);
            const isConnected = connectedBSSID === bssid;

            const windowed = filterToRange(normalizePoints(data), cutoff);

            if (isConnected) {
                if (connectedHistoryWindowed.length <= 1) {
                    const baseColor = theme.accentStrong || theme.accent;
                    const label = clientStats
                        ? `${clientStats.ssid || "Connected"} (${bssid})`
                        : entry?.ssid
                          ? `${entry.ssid} (${bssid})`
                          : bssid;
                    connectedDatasets.push({
                        label,
                        data: windowed,
                        borderColor: baseColor,
                        backgroundColor: withAlpha(baseColor, 0.2),
                        borderWidth: 3,
                        borderDash: [],
                        pointRadius: 3,
                        pointHoverRadius: 6,
                        pointBackgroundColor: baseColor,
                        tension: 0.25,
                        fill: "origin",
                        showLine: true,
                        spanGaps: true,
                        order: 10,
                    });
                }
                return;
            }

            if (windowed.length === 0) return;

            // Band filter — entry.band is cached on record from networks.
            if (bandFilter !== "all" && entry?.band !== bandFilter) return;

            // Hidden filter — drop hidden-SSID APs unless user opts in.
            if (!showHidden && entry?.hidden) return;

            // SSID search — case-insensitive substring; also matches BSSID
            // so a user can paste a MAC fragment to pin a specific AP.
            if (ssidQuery) {
                const haystack = `${entry?.ssid || ""} ${bssid}`.toLowerCase();
                if (!haystack.includes(ssidQuery)) return;
            }

            // Track max signal in window for Top-N ranking.
            let maxSig = -Infinity;
            for (const p of windowed) if (p.y > maxSig) maxSig = p.y;

            otherCandidates.push({ bssid, entry, windowed, maxSig });
        });

        // Top-N: keep strongest. topN === 0 means "All".
        otherCandidates.sort((a, b) => b.maxSig - a.maxSig);
        const visible =
            topN > 0 ? otherCandidates.slice(0, topN) : otherCandidates;

        othersTotal = otherCandidates.length;
        othersVisible = visible.length;

        for (const cand of visible) {
            const { bssid, entry, windowed } = cand;
            const baseColor = colors[colorIndex % colors.length];
            const label = entry?.ssid
                ? `${entry.ssid} (${bssid})`
                : bssid;
            otherDatasets.push({
                label,
                data: windowed,
                borderColor: baseColor,
                backgroundColor: withAlpha(baseColor, 0.06),
                borderWidth: 1.5,
                borderDash: [4, 3],
                pointRadius: 2,
                pointHoverRadius: 4,
                pointBackgroundColor: baseColor,
                tension: 0.25,
                fill: false,
                showLine: true,
                spanGaps: true,
                order: 5,
            });
            colorIndex++;
        }

        // Add roaming events as vertical lines (only those within range)
        if (
            clientStats?.roamingHistory &&
            clientStats.roamingHistory.length > 0
        ) {
            clientStats.roamingHistory.forEach((roamEvent) => {
                if (roamEvent.timestamp < cutoff) return;
                connectedDatasets.push({
                    label: `Roaming: ${(roamEvent.previousBssid || "").slice(-6)} → ${(roamEvent.newBssid || "").slice(-6)}`,
                    data: [
                        { x: roamEvent.timestamp, y: -100 },
                        { x: roamEvent.timestamp, y: -30 },
                    ],
                    borderColor: theme.warning,
                    borderWidth: 2,
                    borderDash: [5, 5],
                    pointRadius: 0,
                    fill: false,
                    showLine: true,
                });
            });
        }

        applyRangeBounds(connectedChart, now, cutoff);
        applyRangeBounds(othersChart, now, cutoff);
        connectedChart.data.datasets = connectedDatasets;
        othersChart.data.datasets = otherDatasets;
        connectedChart.update();
        othersChart.update();
    }
</script>

<div class="signal-chart-container">
    <div class="signal-toolbar">
        <div class="segmented" role="tablist" aria-label="Time range">
            {#each RANGE_OPTIONS as opt}
                <button
                    type="button"
                    role="tab"
                    aria-selected={range === opt.id}
                    class:active={range === opt.id}
                    on:click={() => (range = opt.id)}
                >
                    {opt.label}
                </button>
            {/each}
        </div>
        {#if historyAPs > 0}
            <div class="toolbar-meta">
                <span>{historyAPs} APs · {historyPoints} samples</span>
                {#if clientStats && clientStats.roamingHistory && clientStats.roamingHistory.length > 0}
                    <span
                        >· {clientStats.roamingHistory.length} roams</span
                    >
                {/if}
            </div>
        {/if}
    </div>

    {#if !(clientStats && clientStats.connected)}
        <div class="chart-header">
            <h3>Signal Strength</h3>
            <p class="no-connection">Not connected to any WiFi network</p>
        </div>
    {/if}

    <div class="chart-wrapper">
        <canvas bind:this={connectedChartElement}></canvas>
    </div>

    <div class="others-toolbar">
        <div class="others-filters">
            <label class="filter-block">
                <span class="filter-label">Top</span>
                <div class="segmented small" role="tablist" aria-label="Top-N APs">
                    {#each TOPN_OPTIONS as opt}
                        <button
                            type="button"
                            aria-selected={topN === opt.id}
                            class:active={topN === opt.id}
                            on:click={() => (topN = opt.id)}
                        >{opt.label}</button>
                    {/each}
                </div>
            </label>
            <label class="filter-block">
                <span class="filter-label">Band</span>
                <div class="segmented small" role="tablist" aria-label="Band">
                    {#each BAND_OPTIONS as opt}
                        <button
                            type="button"
                            aria-selected={bandFilter === opt.id}
                            class:active={bandFilter === opt.id}
                            on:click={() => (bandFilter = opt.id)}
                        >{opt.label}</button>
                    {/each}
                </div>
            </label>
            <label class="filter-block grow">
                <span class="filter-label">Search</span>
                <input
                    type="text"
                    class="signal-search"
                    placeholder="SSID or BSSID…"
                    bind:value={ssidFilter}
                />
            </label>
            <label class="filter-block checkbox">
                <input type="checkbox" bind:checked={showHidden} />
                <span class="filter-label">Show hidden</span>
            </label>
        </div>
        <div class="others-meta mono">
            {othersVisible} / {othersTotal}
        </div>
    </div>

    <div class="chart-wrapper secondary">
        <canvas bind:this={othersChartElement}></canvas>
    </div>
</div>

<style>
    .signal-chart-container {
        height: 100%;
        min-height: 0;
        display: flex;
        flex-direction: column;
        gap: 12px;
        background: linear-gradient(180deg, var(--bg-0), var(--bg-1));
        padding: 16px;
        box-sizing: border-box;
    }

    .chart-header h3 {
        margin: 0 0 8px 0;
        font-size: 18px;
        font-weight: 600;
        color: var(--text);
    }

    .no-connection {
        color: var(--muted-2);
        font-size: 14px;
        margin: 0;
    }

    .chart-wrapper {
        flex: 1 1 0;
        position: relative;
        min-height: 0;
        padding: 6px;
        border-radius: 16px;
        background: linear-gradient(180deg, var(--panel), var(--panel-strong));
        border: 1px solid var(--border);
        box-shadow: var(--panel-shadow);
    }

    .chart-wrapper.secondary {
        min-height: 0;
    }

    .signal-toolbar {
        display: flex;
        align-items: center;
        gap: 12px;
        flex-wrap: wrap;
    }

    .toolbar-meta {
        margin-left: auto;
        display: flex;
        gap: 8px;
        font-size: 12px;
        color: var(--muted-2);
        font-family: var(--font-mono, ui-monospace, monospace);
    }

    .segmented {
        display: inline-flex;
        background: var(--bg-3, var(--panel-strong));
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
        padding: 4px 12px;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
        font-family: inherit;
    }

    .segmented button.active {
        background: var(--bg-1, var(--panel));
        color: var(--text);
        box-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    }

    .segmented button:hover:not(.active) {
        color: var(--text);
    }

    .segmented.small button {
        padding: 3px 8px;
        font-size: 11px;
    }

    .others-toolbar {
        display: flex;
        align-items: center;
        gap: 12px;
        flex-wrap: wrap;
    }

    .others-filters {
        display: flex;
        align-items: center;
        gap: 12px;
        flex-wrap: wrap;
        flex: 1;
    }

    .filter-block {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        font-size: 11px;
        color: var(--muted-2);
    }

    .filter-block.grow {
        flex: 1 1 200px;
        min-width: 160px;
    }

    .filter-label {
        text-transform: uppercase;
        letter-spacing: 0.08em;
        font-size: 10px;
        font-weight: 600;
        color: var(--muted);
    }

    .filter-value {
        min-width: 56px;
        text-align: right;
        color: var(--text);
        font-size: 11px;
    }

    .signal-range {
        flex: 1;
        accent-color: var(--acc-1, var(--accent));
    }

    .signal-search {
        flex: 1;
        background: var(--bg-3, var(--panel-strong));
        border: 1px solid var(--border-strong, var(--border));
        border-radius: 6px;
        padding: 4px 8px;
        color: var(--text);
        font-size: 12px;
        font-family: inherit;
        min-width: 0;
    }

    .signal-search:focus {
        outline: none;
        border-color: var(--acc-1, var(--accent));
    }

    .others-meta {
        font-size: 11px;
        color: var(--muted-2);
        white-space: nowrap;
    }

    .mono {
        font-family: var(--font-mono, ui-monospace, monospace);
    }

    /* Responsive adjustments */
    @media (max-width: 768px) {
        .signal-chart-container {
            padding: 12px;
        }

        .chart-header h3 {
            font-size: 16px;
        }
    }
</style>
