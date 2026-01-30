<script>
    import { onMount, onDestroy } from "svelte";
    import { Chart, registerables } from "chart.js";
    import "chartjs-adapter-date-fns";
    import zoomPlugin from "chartjs-plugin-zoom";

    export let clientStats = null;
    export let networks = [];

    let connectedChartElement;
    let othersChartElement;
    let connectedChart = null;
    let othersChart = null;
    let themeMedia = null;
    let apHistory = new Map();
    let historyPoints = 0;
    let historyAPs = 0;
    const HISTORY_WINDOW_MS = 30 * 60 * 1000;
    const HISTORY_MAX_POINTS = 300;

    onMount(() => {
        Chart.register(...registerables, zoomPlugin);
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
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: "index",
                    intersect: false,
                },
                layout: {
                    padding: {
                        top: 8,
                        right: 12,
                        bottom: 8,
                        left: 8,
                    },
                },
                plugins: {
                    title: {
                        display: true,
                        text: titleText,
                        color: theme.text,
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
                    zoom: {
                        pan: {
                            enabled: true,
                            mode: "x",
                        },
                        zoom: {
                            wheel: {
                                enabled: true,
                            },
                            pinch: {
                                enabled: true,
                            },
                            mode: "x",
                        },
                        limits: {
                            x: {
                                min: "original",
                                max: "original",
                            },
                        },
                    },
                },
                scales: {
                    x: {
                        type: "time",
                        time: {
                            unit: "minute",
                            displayFormats: {
                                minute: "HH:mm",
                            },
                        },
                        title: {
                            display: true,
                            text: "Time",
                            color: theme.muted,
                        },
                        ticks: {
                            color: theme.muted,
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

    // Update chart when clientStats or networks change
    $: if (connectedChart && othersChart) {
        networks;
        clientStats;
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

        if (networks && networks.length > 0) {
            networks.forEach((network) => {
                if (!network?.accessPoints?.length) return;
                network.accessPoints.forEach((ap) => {
                    const bssid = ap?.bssid;
                    if (!bssid) return;
                    if (typeof ap.signal !== "number") return;
                    const ssid = ap?.ssid || network?.ssid || "Unknown";
                    const timestamp = now;

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

    function updateChart() {
        if (apHistory.size === 0) {
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
        if (connectedHistory.length > 1 && connectedBSSID) {
            const label = `${clientStats.ssid || "Connected"} (${connectedBSSID})`;
            const baseColor = theme.accentStrong || theme.accent;
            connectedDatasets.push({
                label,
                data: connectedHistory,
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
                data: normalizePoints(data),
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
                if (connectedHistory.length <= 1) {
                    connectedDatasets.push(dataset);
                }
            } else otherDatasets.push(dataset);
            colorIndex++;
        });

        // Add roaming events as vertical lines
        if (
            clientStats?.roamingHistory &&
            clientStats.roamingHistory.length > 0
        ) {
            clientStats.roamingHistory.forEach((roamEvent) => {
                connectedDatasets.push({
                    label: `Roaming: ${roamEvent.previousBSSID.slice(-6)} → ${roamEvent.newBSSID.slice(-6)}`,
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

        connectedChart.data.datasets = connectedDatasets;
        othersChart.data.datasets = otherDatasets;
        connectedChart.update();
        othersChart.update();
    }

    function resetZoom() {
        connectedChart?.resetZoom();
        othersChart?.resetZoom();
    }

    function getSignalQuality(signal) {
        if (signal > -60) return { text: "Excellent", color: "var(--success)" };
        if (signal > -70) return { text: "Good", color: "var(--success)" };
        if (signal > -80) return { text: "Fair", color: "var(--warning)" };
        return { text: "Poor", color: "var(--danger)" };
    }
</script>

<div class="signal-chart-container">
    {#if clientStats && clientStats.connected}
        <div class="chart-header">
            <div class="connection-info">
                <h3>Connected: {clientStats.ssid}</h3>
                <div class="signal-summary">
                    <div class="current-signal">
                        <span class="signal-label">Current:</span>
                        <span
                            class="signal-value"
                            class:signal-good={clientStats.signal > -60}
                            class:signal-medium={clientStats.signal > -75 &&
                                clientStats.signal <= -60}
                            class:signal-poor={clientStats.signal <= -75}
                        >
                            {clientStats.signal} dBm
                        </span>
                        <span
                            class="signal-quality"
                            style="color: {getSignalQuality(clientStats.signal)
                                .color}"
                        >
                            ({getSignalQuality(clientStats.signal).text})
                        </span>
                    </div>
                    <div class="signal-stats">
                        <span
                            >Avg: {clientStats.signalAvg || clientStats.signal} dBm</span
                        >
                        <span>SNR: {clientStats.snr} dB</span>
                    </div>
                </div>
            </div>
        </div>
    {:else}
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

    {#if historyAPs > 0}
        <div class="chart-footer">
            <div class="history-info">
                <span>APs tracked: {historyAPs}</span>
                <span>History: {historyPoints} data points</span>
                {#if clientStats && clientStats.roamingHistory && clientStats.roamingHistory.length > 0}
                    <span
                        >Roaming events: {clientStats.roamingHistory
                            .length}</span
                    >
                {/if}
                <span class="zoom-hint">Scroll to zoom, drag to pan</span>
            </div>
            <button class="reset-zoom" type="button" on:click={resetZoom}>
                Reset zoom
            </button>
        </div>
    {/if}
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

    .chart-header {
        display: flex;
        align-items: flex-end;
        justify-content: space-between;
        gap: 12px;
    }

    .chart-header h3 {
        margin: 0 0 8px 0;
        font-size: 18px;
        font-weight: 600;
        color: var(--text);
    }

    .connection-info {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .signal-summary {
        display: flex;
        flex-direction: column;
        gap: 4px;
        padding: 10px 12px;
        border-radius: 12px;
        background: var(--panel);
        border: 1px solid var(--border);
        box-shadow: var(--panel-shadow);
    }

    .current-signal {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .signal-label {
        color: var(--muted);
        font-size: 14px;
    }

    .signal-value {
        font-weight: 600;
        font-size: 16px;
        padding: 2px 8px;
        border-radius: 999px;
        border: 1px solid color-mix(in srgb, currentColor 35%, transparent);
        background: color-mix(in srgb, currentColor 14%, transparent);
    }

    .signal-good {
        color: var(--success);
    }

    .signal-medium {
        color: var(--warning);
    }

    .signal-poor {
        color: var(--danger);
    }

    .signal-quality {
        font-size: 14px;
        font-weight: 500;
    }

    .signal-stats {
        display: flex;
        gap: 16px;
        font-size: 13px;
        color: var(--muted);
    }

    .signal-stats span {
        padding: 2px 8px;
        border-radius: 999px;
        background: var(--panel-strong);
        border: 1px solid var(--border);
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
        padding: 12px;
        border-radius: 16px;
        background: linear-gradient(180deg, var(--panel), var(--panel-strong));
        border: 1px solid var(--border);
        box-shadow: var(--panel-shadow);
    }

    .chart-wrapper.secondary {
        min-height: 0;
    }

    .chart-footer {
        padding-top: 4px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
        flex-wrap: wrap;
    }

    .history-info {
        display: flex;
        gap: 16px;
        font-size: 12px;
        color: var(--muted-2);
    }

    .history-info span {
        padding: 3px 8px;
        border-radius: 999px;
        background: var(--panel-strong);
        border: 1px solid var(--border);
    }

    .zoom-hint {
        color: var(--muted);
    }

    .reset-zoom {
        padding: 6px 10px;
        border-radius: 999px;
        border: 1px solid color-mix(in srgb, var(--accent) 45%, transparent);
        background: color-mix(in srgb, var(--accent) 18%, transparent);
        color: var(--text);
        font-size: 12px;
        font-weight: 600;
        letter-spacing: 0.02em;
        cursor: pointer;
    }

    .reset-zoom:hover {
        border-color: var(--accent);
        color: var(--accent);
    }

    /* Responsive adjustments */
    @media (max-width: 768px) {
        .signal-chart-container {
            padding: 12px;
        }

        .chart-header {
            flex-direction: column;
            align-items: flex-start;
        }

        .chart-header h3 {
            font-size: 16px;
        }

        .current-signal {
            flex-direction: column;
            align-items: flex-start;
            gap: 4px;
        }

        .signal-stats {
            flex-direction: column;
            gap: 2px;
        }

        .history-info {
            flex-direction: column;
            gap: 2px;
        }

        .chart-footer {
            align-items: flex-start;
        }
    }
</style>
