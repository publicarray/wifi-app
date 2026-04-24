<script>
    import { onMount, onDestroy } from "svelte";
    import { Chart, registerables } from "chart.js";
    import "chartjs-adapter-date-fns";
    import { GetLatency } from "../../wailsjs/go/main/App.js";
    import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime.js";

    let summaries = [];
    let chartEl;
    let chart = null;
    let themeMedia = null;

    // One series per target; stable colour assignment keeps lines visually
    // consistent across renders even as probes stream in.
    const SERIES_KEYS = [
        "--series-1",
        "--series-2",
        "--series-3",
        "--series-4",
        "--series-5",
        "--series-6",
    ];

    onMount(async () => {
        Chart.register(...registerables);
        buildChart();
        themeMedia = window.matchMedia("(prefers-color-scheme: dark)");
        themeMedia.addEventListener("change", applyTheme);

        // Hydrate synchronously so the first render isn't empty.
        try {
            const initial = await GetLatency();
            if (initial) {
                summaries = initial;
                updateChart();
            }
        } catch (err) {
            // Non-fatal — events will populate state on the next sampler tick.
        }

        EventsOn("latency:updated", (data) => {
            summaries = data || [];
            updateChart();
        });
    });

    onDestroy(() => {
        themeMedia?.removeEventListener("change", applyTheme);
        EventsOff("latency:updated");
        chart?.destroy();
    });

    function theme() {
        const styles = getComputedStyle(document.documentElement);
        const series = [];
        for (const key of SERIES_KEYS) {
            const v = styles.getPropertyValue(key).trim();
            if (v) series.push(v);
        }
        return {
            text: styles.getPropertyValue("--text").trim() || "#e6e8eb",
            muted: styles.getPropertyValue("--muted").trim() || "#9aa3ad",
            border: styles.getPropertyValue("--border").trim() || "#333",
            borderStrong:
                styles.getPropertyValue("--border-strong").trim() || "#444",
            grid:
                styles.getPropertyValue("--chart-grid").trim() ||
                "rgba(255,255,255,0.08)",
            tooltipBg:
                styles.getPropertyValue("--tooltip-bg").trim() ||
                "rgba(20, 22, 27, 0.92)",
            series: series.length ? series : ["#60a5fa", "#34d399", "#fbbf24"],
        };
    }

    function buildChart() {
        const t = theme();
        const ctx = chartEl.getContext("2d");
        chart = new Chart(ctx, {
            type: "line",
            data: { datasets: [] },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: { mode: "index", intersect: false },
                plugins: {
                    legend: {
                        display: true,
                        position: "top",
                        labels: {
                            color: t.text,
                            usePointStyle: true,
                            boxWidth: 8,
                            boxHeight: 8,
                            padding: 20,
                        },
                    },
                    tooltip: {
                        backgroundColor: t.tooltipBg,
                        titleColor: t.text,
                        bodyColor: t.text,
                        borderColor: t.border,
                        borderWidth: 1,
                        callbacks: {
                            title: (items) =>
                                new Date(
                                    items[0].parsed.x,
                                ).toLocaleTimeString(),
                            label: (item) => {
                                const ds = item.dataset;
                                const v = item.parsed.y;
                                if (v == null) return `${ds.label}: lost`;
                                return `${ds.label}: ${v.toFixed(1)} ms`;
                            },
                        },
                    },
                },
                scales: {
                    x: {
                        type: "time",
                        time: {
                            unit: "second",
                            displayFormats: {
                                second: "HH:mm:ss",
                                minute: "HH:mm",
                            },
                        },
                        title: { display: true, text: "Time", color: t.muted },
                        ticks: { color: t.muted, maxTicksLimit: 8 },
                        grid: { color: t.grid, borderColor: t.borderStrong },
                    },
                    y: {
                        title: {
                            display: true,
                            text: "RTT (ms)",
                            color: t.muted,
                        },
                        ticks: {
                            color: t.muted,
                            callback: (v) => `${v} ms`,
                        },
                        grid: { color: t.grid, borderColor: t.borderStrong },
                        min: 0,
                        suggestedMax: 50,
                    },
                },
            },
        });
    }

    function applyTheme() {
        if (!chart) return;
        const t = theme();
        chart.options.plugins.legend.labels.color = t.text;
        chart.options.plugins.tooltip.backgroundColor = t.tooltipBg;
        chart.options.plugins.tooltip.titleColor = t.text;
        chart.options.plugins.tooltip.bodyColor = t.text;
        chart.options.plugins.tooltip.borderColor = t.border;
        chart.options.scales.x.title.color = t.muted;
        chart.options.scales.x.ticks.color = t.muted;
        chart.options.scales.x.grid.color = t.grid;
        chart.options.scales.x.grid.borderColor = t.borderStrong;
        chart.options.scales.y.title.color = t.muted;
        chart.options.scales.y.ticks.color = t.muted;
        chart.options.scales.y.grid.color = t.grid;
        chart.options.scales.y.grid.borderColor = t.borderStrong;
        updateChart();
    }

    function updateChart() {
        if (!chart) return;
        const t = theme();
        chart.data.datasets = summaries.map((s, i) => {
            const color = t.series[i % t.series.length];
            // Lost probes are rendered as null (gap) rather than zero so they
            // don't visually imply sub-ms latency.
            const points = (s.history || []).map((p) => ({
                x: new Date(p.timestamp).getTime(),
                y: p.lost ? null : p.rttMs,
            }));
            return {
                label: `${s.label}${s.target && s.target !== s.label ? ` (${s.target})` : ""}`,
                data: points,
                borderColor: color,
                backgroundColor: color,
                borderWidth: 1.5,
                pointRadius: 0,
                pointHoverRadius: 4,
                spanGaps: false,
                tension: 0.2,
            };
        });
        chart.update("none");
    }

    function fmt(v) {
        if (!Number.isFinite(v)) return "—";
        if (v === 0) return "0";
        return v.toFixed(1);
    }

    function windowLabel(w) {
        if (w.windowSeconds >= 60) return `${w.windowSeconds / 60}m`;
        return `${w.windowSeconds}s`;
    }

    function lossClass(pct) {
        if (pct >= 5) return "bad";
        if (pct >= 1) return "warn";
        return "ok";
    }

    function latencyClass(ms) {
        if (!Number.isFinite(ms) || ms <= 0) return "";
        if (ms >= 100) return "bad";
        if (ms >= 30) return "warn";
        return "ok";
    }
</script>

<div class="latency-panel">
    <div class="header">
        <h2>Latency &amp; Loss</h2>
        <p class="sub">
            Active probes to the configured targets. Edit targets in the
            Settings tab (<code>latency_targets</code>). The
            <code>gateway</code> magic value resolves to your current default route.
        </p>
    </div>

    {#if summaries.length === 0}
        <div class="empty">
            <p>Waiting for the first latency probe…</p>
        </div>
    {:else}
        <div class="cards">
            {#each summaries as s}
                <div class="card" class:unavailable={!s.available}>
                    <div class="card-head">
                        <div class="title">
                            <span class="label">{s.label}</span>
                            {#if s.target && s.target !== s.label}
                                <span class="addr">{s.target}</span>
                            {/if}
                        </div>
                        <span class="transport">{s.transport}</span>
                    </div>

                    {#if !s.available}
                        <div class="unavail-msg">{s.target || "unavailable"}</div>
                    {:else}
                        <div class="metrics">
                            {#each s.windows as w}
                                <div class="metric">
                                    <div class="metric-label">{windowLabel(w)}</div>
                                    <div class="metric-values">
                                        <span
                                            class="avg {latencyClass(w.avgMs)}"
                                            title="Average RTT"
                                        >
                                            {fmt(w.avgMs)} <small>ms</small>
                                        </span>
                                        <span class="minmax" title="Min / Max RTT">
                                            {fmt(w.minMs)}–{fmt(w.maxMs)}
                                        </span>
                                        <span
                                            class="jitter"
                                            title="Stddev (jitter)"
                                        >
                                            ± {fmt(w.stddevMs)}
                                        </span>
                                        <span
                                            class="loss {lossClass(w.lossPercent)}"
                                            title="Loss %"
                                        >
                                            {fmt(w.lossPercent)}% loss
                                        </span>
                                    </div>
                                </div>
                            {/each}
                        </div>
                    {/if}
                </div>
            {/each}
        </div>
    {/if}

    <div class="chart-wrap">
        <canvas bind:this={chartEl}></canvas>
    </div>
</div>

<style>
    .latency-panel {
        padding: 20px;
        display: flex;
        flex-direction: column;
        gap: 20px;
        min-height: 0;
        height: 100%;
        box-sizing: border-box;
    }

    .header h2 {
        margin: 0 0 4px 0;
        color: var(--text);
    }
    .header .sub {
        margin: 0;
        color: var(--muted);
        font-size: 13px;
    }
    .header code {
        background: var(--panel-soft);
        padding: 1px 5px;
        border-radius: 3px;
        font-size: 12px;
    }

    .empty {
        color: var(--muted);
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 8px;
        padding: 16px;
        text-align: center;
    }

    .cards {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
        gap: 14px;
    }

    .card {
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 10px;
        padding: 14px 16px;
        box-shadow: var(--panel-shadow);
    }

    .card.unavailable {
        opacity: 0.7;
    }

    .card-head {
        display: flex;
        justify-content: space-between;
        align-items: baseline;
        margin-bottom: 10px;
    }

    .title {
        display: flex;
        flex-direction: column;
        gap: 2px;
    }
    .title .label {
        font-weight: 600;
        font-size: 15px;
        color: var(--text);
    }
    .title .addr {
        font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
        font-size: 12px;
        color: var(--muted-2);
    }

    .transport {
        font-size: 11px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        color: var(--muted-2);
        background: var(--panel-soft);
        padding: 2px 6px;
        border-radius: 4px;
    }

    .unavail-msg {
        color: var(--warning);
        font-size: 13px;
    }

    .metrics {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }

    .metric {
        display: grid;
        grid-template-columns: 36px 1fr;
        align-items: center;
        gap: 10px;
    }

    .metric-label {
        font-size: 11px;
        text-transform: uppercase;
        color: var(--muted-2);
        letter-spacing: 0.5px;
    }

    .metric-values {
        display: flex;
        flex-wrap: wrap;
        gap: 10px;
        align-items: baseline;
        font-size: 13px;
        color: var(--text);
    }

    .metric-values .avg {
        font-weight: 600;
        min-width: 70px;
    }
    .metric-values .avg small {
        font-weight: 400;
        color: var(--muted);
    }

    .metric-values .minmax,
    .metric-values .jitter {
        color: var(--muted);
        font-size: 12px;
    }

    .metric-values .loss {
        margin-left: auto;
        font-size: 12px;
    }

    .ok {
        color: var(--success);
    }
    .warn {
        color: var(--warning);
    }
    .bad {
        color: var(--danger);
    }

    .chart-wrap {
        flex: 1;
        min-height: 240px;
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 10px;
        padding: 12px;
    }
</style>
