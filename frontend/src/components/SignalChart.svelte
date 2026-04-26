<script context="module">
    // apHistory lives at module scope so the per-other-AP signal traces
    // survive switching away from the Signal tab and back. The connected
    // AP's history is read from clientStats.signalHistory (backend-managed),
    // so without a persistent module-scope store the connected line would
    // appear "complete" while the other-AP lines look like they reset on
    // every remount.
    //
    // Growth is bounded by the per-AP window+max-points filter and the
    // sliding-window cleanup pass in recordNetworkSignals, so the Map is
    // capped to "BSSIDs seen in the last HISTORY_WINDOW_MS".
    const moduleApHistory = new Map();
</script>

<script>
    import { onMount, onDestroy } from "svelte";
    import { Chart, registerables } from "chart.js";
    import "chartjs-adapter-date-fns";

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
    const HISTORY_WINDOW_MS = 60 * 60 * 1000;
    const HISTORY_MAX_POINTS = 1500;
    const STALE_HOLD_MS = 30000;

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

    onMount(() => {
        Chart.register(...registerables);
        initializeChart();

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

    // Update chart when clientStats, networks, or range change
    $: if (connectedChart && othersChart) {
        networks;
        clientStats;
        rangeMs;
        recordNetworkSignals();
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

    function recordNetworkSignals() {
        const now = Date.now();
        const seenBSSIDs = new Set();

        if (networks && networks.length > 0) {
            networks.forEach((network) => {
                if (!network?.accessPoints?.length) return;
                network.accessPoints.forEach((ap) => {
                    const bssid = ap?.bssid;
                    if (!bssid) return;
                    if (typeof ap.signal !== "number") return;
                    const ssid = ap?.ssid || network?.ssid || "Unknown";
                    const timestamp = now;
                    seenBSSIDs.add(bssid);

                    let entry = apHistory.get(bssid);
                    if (!entry) {
                        entry = { bssid, ssid, points: [] };
                        apHistory.set(bssid, entry);
                    }
                    entry.ssid = ssid;
                    const lastPoint = entry.points[entry.points.length - 1];
                    if (!lastPoint || lastPoint.x !== timestamp) {
                        entry.points.push({ x: timestamp, y: ap.signal });
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

        apHistory.forEach((entry, bssid) => {
            if (seenBSSIDs.has(bssid)) return;
            const lastPoint = entry.points?.[entry.points.length - 1];
            if (!lastPoint) return;
            if (now - lastPoint.x <= STALE_HOLD_MS) {
                if (lastPoint.x !== now) {
                    entry.points.push({ x: now, y: lastPoint.y });
                }
            }
        });

        apHistory.forEach((entry, bssid) => {
            if (!entry.points || entry.points.length === 0) {
                apHistory.delete(bssid);
                return;
            }
            const lastPoint = entry.points[entry.points.length - 1];
            if (lastPoint.x < now - HISTORY_WINDOW_MS) {
                apHistory.delete(bssid);
            }
        });

        historyAPs = apHistory.size;
        historyPoints = 0;
        apHistory.forEach((entry) => {
            historyPoints += entry.points.length;
        });
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

        entries.forEach(([bssid, data]) => {
            // Find the corresponding AP to get SSID
            const entry = apHistory.get(bssid);
            let label = entry?.ssid ? `${entry.ssid} (${bssid})` : bssid;
            const isConnected = connectedBSSID === bssid;
            if (connectedBSSID === bssid && clientStats) {
                label = `${clientStats.ssid || "Connected"} (${bssid})`;
            }
            const baseColor = isConnected
                ? theme.accentStrong || theme.accent
                : colors[colorIndex % colors.length];

            const dataset = {
                label: label,
                data: filterToRange(normalizePoints(data), cutoff),
                borderColor: baseColor,
                backgroundColor: withAlpha(baseColor, isConnected ? 0.2 : 0.06),
                borderWidth: isConnected ? 3 : 1.5,
                borderDash: isConnected ? [] : [4, 3],
                pointRadius: isConnected ? 3 : 2,
                pointHoverRadius: isConnected ? 6 : 4,
                pointBackgroundColor: baseColor,
                tension: 0.25,
                fill: isConnected ? "origin" : false,
                showLine: true,
                spanGaps: true,
                order: isConnected ? 10 : 5,
            };
            if (isConnected) {
                if (connectedHistoryWindowed.length <= 1) {
                    connectedDatasets.push(dataset);
                }
            } else otherDatasets.push(dataset);
            colorIndex++;
        });

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
