<script>
    import { onMount, onDestroy } from "svelte";
    import { Chart, registerables } from "chart.js";
    import "chartjs-adapter-date-fns";

    export let clientStats = null;

    let chartElement;
    let chart = null;
    let themeMedia = null;

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
        if (chart) {
            chart.destroy();
        }
    });

    function initializeChart() {
        const ctx = chartElement.getContext("2d");
        const theme = getThemeColors();

        chart = new Chart(ctx, {
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
                plugins: {
                    title: {
                        display: true,
                        text: "Signal Strength Over Time",
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
                            padding: 20,
                        },
                    },
                    tooltip: {
                        backgroundColor: theme.tooltipBg,
                        titleColor: theme.text,
                        bodyColor: theme.text,
                        borderColor: theme.border,
                        borderWidth: 1,
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

    // Update chart when clientStats changes
    $: if (chart && clientStats && clientStats.signalHistory) {
        updateChart();
    }

    function getThemeColors() {
        const styles = getComputedStyle(document.documentElement);
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
        };
    }

    function applyChartTheme() {
        if (!chart) return;
        const theme = getThemeColors();
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
        chart.update("none");
    }

    function updateChart() {
        if (
            !clientStats ||
            !clientStats.signalHistory ||
            clientStats.signalHistory.length === 0
        ) {
            chart.data.datasets = [];
            chart.update();
            return;
        }

        const theme = getThemeColors();

        // Group signal data by BSSID to show multiple APs
        const signalDataByBSSID = {};

        clientStats.signalHistory.forEach((point) => {
            const bssid = point.bssid || "Unknown";
            if (!signalDataByBSSID[bssid]) {
                signalDataByBSSID[bssid] = [];
            }
            signalDataByBSSID[bssid].push({
                x: point.timestamp,
                y: point.signal,
            });
        });

        // Create datasets for each BSSID
        const datasets = [];
        const colors = [
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
        ];
        let colorIndex = 0;

        Object.entries(signalDataByBSSID).forEach(([bssid, data]) => {
            // Find the corresponding AP to get SSID
            let label = bssid;
            if (clientStats.bssid === bssid) {
                label = `${clientStats.ssid || "Connected"} (${bssid})`;
            }

            datasets.push({
                label: label,
                data: data,
                borderColor: colors[colorIndex % colors.length],
                backgroundColor: colors[colorIndex % colors.length] + "20",
                borderWidth: 2,
                pointRadius: 3,
                pointHoverRadius: 5,
                tension: 0.1,
                fill: false,
            });
            colorIndex++;
        });

        // Add roaming events as vertical lines
        if (
            clientStats.roamingHistory &&
            clientStats.roamingHistory.length > 0
        ) {
            clientStats.roamingHistory.forEach((roamEvent) => {
                datasets.push({
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

        chart.data.datasets = datasets;
        chart.update();
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
        <canvas bind:this={chartElement}></canvas>
    </div>

    {#if clientStats && clientStats.signalHistory && clientStats.signalHistory.length > 0}
        <div class="chart-footer">
            <div class="history-info">
                <span
                    >History: {clientStats.signalHistory.length} data points</span
                >
                {#if clientStats.roamingHistory && clientStats.roamingHistory.length > 0}
                    <span
                        >Roaming events: {clientStats.roamingHistory
                            .length}</span
                    >
                {/if}
            </div>
        </div>
    {/if}
</div>

<style>
    .signal-chart-container {
        height: 100%;
        display: flex;
        flex-direction: column;
        background: var(--bg-0);
        padding: 16px;
    }

    .chart-header {
        margin-bottom: 16px;
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

    .no-connection {
        color: var(--muted-2);
        font-size: 14px;
        margin: 0;
    }

    .chart-wrapper {
        flex: 1;
        position: relative;
        min-height: 200px;
    }

    .chart-footer {
        margin-top: 12px;
        padding-top: 8px;
        border-top: 1px solid var(--border);
    }

    .history-info {
        display: flex;
        gap: 16px;
        font-size: 12px;
        color: var(--muted-2);
    }

    /* Responsive adjustments */
    @media (max-width: 768px) {
        .signal-chart-container {
            padding: 12px;
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
    }
</style>
