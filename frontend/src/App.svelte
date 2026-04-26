<script>
    import { onMount, onDestroy } from "svelte";
    import {
        GetAvailableInterfaces,
        StartScanning,
        StopScanning,
        GetNetworks,
        GetClientStats,
        GetChannelAnalysis,
        IsScanning,
        GetRoamingAnalysis,
        GetAPPlacementRecommendations,
        GetConfig,
    } from "../wailsjs/go/main/App.js";
    import { EventsOn, EventsOff } from "../wailsjs/runtime/runtime.js";

    import NetworkList from "./components/NetworkList.svelte";
    import SignalChart from "./components/SignalChart.svelte";
    import ChannelAnalyzer from "./components/ChannelAnalyzer.svelte";
    import ClientStatsPanel from "./components/ClientStatsPanel.svelte";
    import RoamingAnalysis from "./components/RoamingAnalysis.svelte";
    import SettingsPanel from "./components/SettingsPanel.svelte";
    import LatencyChart from "./components/LatencyChart.svelte";
    import ReportWindow from "./components/ReportWindow.svelte";

    let interfaces = [];
    let selectedInterface = "";
    let scanning = false;
    let networks = [];
    let clientStats = null;
    let channelAnalysis = [];
    let errorMessage = "";
    let activeTab = "networks";
    let roamingMetrics = null;
    let placementRecommendations = [];
    let reportOpen = false;

    const NAV_ANALYZE = [
        { id: "networks", label: "Networks", icon: "networks" },
        { id: "signal", label: "Signal", icon: "signal" },
        { id: "channels", label: "Channels", icon: "channels" },
        { id: "stats", label: "Stats", icon: "stats" },
        { id: "latency", label: "Latency", icon: "latency" },
        { id: "roaming", label: "Roaming", icon: "roaming" },
    ];

    const TAB_TITLES = {
        networks: { title: "Networks", sub: "Discovered access points" },
        signal: { title: "Signal", sub: "Live RSSI over time" },
        channels: { title: "Channels", sub: "Congestion by band" },
        stats: { title: "Stats", sub: "Client connection details" },
        latency: { title: "Latency", sub: "RTT to gateway and beyond" },
        roaming: { title: "Roaming", sub: "BSS transitions & behavior" },
        settings: { title: "Settings", sub: "App configuration" },
    };

    $: titleInfo = TAB_TITLES[activeTab] || { title: "", sub: "" };
    $: networksBadge = networks?.length || 0;
    $: roamingBadge =
        clientStats?.roamingHistory ? clientStats.roamingHistory.length : 0;
    $: connectedSSID =
        clientStats && clientStats.connected ? clientStats.ssid : "";
    $: connectedRSSI =
        clientStats && clientStats.connected ? clientStats.signal : null;

    onMount(async () => {
        try {
            interfaces = (await GetAvailableInterfaces()) || [];
        } catch (err) {
            errorMessage = "Failed to get WiFi interfaces: " + err;
        }

        if (interfaces.length > 0) {
            let configured = "";
            try {
                const cfg = await GetConfig();
                configured = cfg?.defaultInterface || "";
            } catch {
                // Non-fatal — defaults apply.
            }
            selectedInterface =
                configured && interfaces.includes(configured)
                    ? configured
                    : interfaces[0];
        }

        try {
            const [n, c, ch, alreadyScanning] = await Promise.all([
                GetNetworks(),
                GetClientStats(),
                GetChannelAnalysis(),
                IsScanning(),
            ]);
            if (n) networks = n;
            if (c) clientStats = c;
            if (ch) channelAnalysis = ch;
            scanning = !!alreadyScanning;
        } catch (err) {
            // Non-fatal: events will populate state on the next tick.
        }

        EventsOn("networks:updated", (data) => {
            if (typeof window !== "undefined" && import.meta.env?.development) {
                const networksCount = data ? data.length : 0;
                console.debug(`[wifi-app] Networks updated: ${networksCount} networks received`);
            }
            networks = data || [];
        });

        EventsOn("client:updated", (data) => {
            if (typeof window !== "undefined" && import.meta.env?.development) {
                console.debug(
                    `[wifi-app] Client stats updated for ${data?.bssid || "?"} at channel ${data?.channel || "?"}`,
                );
            }
            clientStats = data;
        });

        EventsOn("channels:updated", (data) => {
            channelAnalysis = data || [];
        });

        EventsOn("scan:error", (error) => {
            if (typeof window !== "undefined" && import.meta.env?.development) {
                console.error(`[wifi-app] Scan error: ${error}`);
            }
            errorMessage = error;
        });

        EventsOn("scan:debug", (message) => {
            if (typeof window !== "undefined" && import.meta.env?.development) {
                console.debug(`[wifi-app] Scan debug: ${message}`);
            }
        });

        EventsOn("scan:status", (status) => {
            if (typeof window !== "undefined" && import.meta.env?.development) {
                console.log(`[wifi-app] Scan ${status}`);
            }
        });

        EventsOn("client:warning", (warning) => {
            console.warn("Client warning:", warning);
        });

        EventsOn("roaming:detected", (event) => {
            if (typeof window !== "undefined" && import.meta.env?.development) {
                console.debug("[wifi-app] Roaming detected:", event);
            }
            if (activeTab === "roaming") {
                loadRoamingData();
            }
        });
    });

    onDestroy(() => {
        EventsOff("networks:updated");
        EventsOff("client:updated");
        EventsOff("channels:updated");
        EventsOff("scan:error");
        EventsOff("scan:debug");
        EventsOff("scan:status");
        EventsOff("roaming:detected");
        EventsOff("client:warning");
        if (scanning) {
            stopScanning();
        }
    });

    async function startScanning() {
        try {
            errorMessage = "";
            await StartScanning(selectedInterface);
            scanning = true;
        } catch (err) {
            errorMessage = "Failed to start scanning: " + err;
        }
    }

    async function stopScanning() {
        try {
            await StopScanning();
            scanning = false;
        } catch (err) {
            errorMessage = "Failed to stop scanning: " + err;
        }
    }

    function handleInterfaceChange(event) {
        selectedInterface = event.target.value;
    }

    async function setActiveTab(tab) {
        activeTab = tab;
        if (tab === "roaming") {
            await loadRoamingData();
        }
    }

    async function loadRoamingData() {
        try {
            roamingMetrics = await GetRoamingAnalysis();
            placementRecommendations = await GetAPPlacementRecommendations();
        } catch (err) {
            console.error("Failed to load roaming data:", err);
        }
    }

    function navBadge(id) {
        if (id === "networks" && networksBadge > 0) return networksBadge;
        if (id === "roaming" && roamingBadge > 0) return roamingBadge;
        return null;
    }
</script>

<div class="app-container">
    <div class="shell">
        <aside class="sidebar">
            <div class="sidebar-brand">
                <div class="sidebar-brand-title">
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                        <path d="M8 12v0M4.5 9a5 5 0 017 0M2 6.5a8 8 0 0112 0" stroke="var(--acc-1)" stroke-width="1.5" stroke-linecap="round"/>
                        <circle cx="8" cy="12" r="1" fill="var(--acc-1)"/>
                    </svg>
                    <span>WiFi Diagnostic</span>
                </div>
                <div class="sidebar-brand-sub">linux · wails</div>
            </div>

            <div class="sidebar-section-label">Analyze</div>
            {#each NAV_ANALYZE as item}
                <button
                    type="button"
                    class="nav-item"
                    class:active={activeTab === item.id}
                    on:click={() => setActiveTab(item.id)}
                >
                    {#if item.icon === "networks"}
                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M8 11.5v0M4.5 8.5a5 5 0 017 0M2 6a8 8 0 0112 0M8 11.5a.5.5 0 110 1 .5.5 0 010-1z" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
                    {:else if item.icon === "signal"}
                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M2 13V10M6 13V7M10 13V5M14 13V3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
                    {:else if item.icon === "channels"}
                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><rect x="2" y="3" width="3" height="10" rx="0.5" stroke="currentColor" stroke-width="1.2"/><rect x="6.5" y="6" width="3" height="7" rx="0.5" stroke="currentColor" stroke-width="1.2"/><rect x="11" y="4" width="3" height="9" rx="0.5" stroke="currentColor" stroke-width="1.2"/></svg>
                    {:else if item.icon === "stats"}
                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M2 13l3-4 3 2 6-7" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/><circle cx="14" cy="4" r="1.2" fill="currentColor"/></svg>
                    {:else if item.icon === "latency"}
                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8a5 5 0 0110 0M8 3v0M8 8l3-2" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/><circle cx="8" cy="8" r="1" fill="currentColor"/></svg>
                    {:else if item.icon === "roaming"}
                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 5h8l-2-2M13 11H5l2 2" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
                    {/if}
                    <span>{item.label}</span>
                    {#if navBadge(item.id) !== null}
                        <span class="nav-item-badge">{navBadge(item.id)}</span>
                    {/if}
                </button>
            {/each}

            <div class="sidebar-section-label">Tools</div>
            <button
                type="button"
                class="nav-item"
                on:click={() => (reportOpen = true)}
            >
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><rect x="3" y="2" width="10" height="12" rx="1.5" stroke="currentColor" stroke-width="1.3"/><path d="M5.5 6h5M5.5 8.5h5M5.5 11h3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
                <span>Reports</span>
            </button>
            <button
                type="button"
                class="nav-item"
                class:active={activeTab === "settings"}
                on:click={() => setActiveTab("settings")}
            >
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="2" stroke="currentColor" stroke-width="1.3"/><path d="M8 2v1.5M8 12.5V14M14 8h-1.5M3.5 8H2M12.24 3.76l-1.06 1.06M4.82 11.18l-1.06 1.06M12.24 12.24l-1.06-1.06M4.82 4.82L3.76 3.76" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
                <span>Preferences</span>
            </button>

            <div class="sidebar-footer">
                <div class="status-row">
                    <span>Interface</span>
                    <span class="val">{selectedInterface || "—"}</span>
                </div>
                <div class="status-row">
                    <span>Connected</span>
                    <span
                        class="val"
                        style={connectedSSID ? "color: var(--ok);" : ""}
                    >
                        {connectedSSID || "—"}
                    </span>
                </div>
                <div class="status-row">
                    <span>RSSI</span>
                    <span
                        class="val"
                        style={connectedRSSI != null ? "color: var(--ok);" : ""}
                    >
                        {connectedRSSI != null ? `${connectedRSSI} dBm` : "—"}
                    </span>
                </div>
                <div class="status-row">
                    <span>Scan</span>
                    <span class="val scan-state" class:live={scanning}>
                        {#if scanning}
                            <span class="dot live"></span>live
                        {:else}
                            paused
                        {/if}
                    </span>
                </div>
            </div>
        </aside>

        <main class="main">
            <header class="topbar">
                <div class="topbar-titles">
                    <div class="topbar-title">{titleInfo.title}</div>
                    <div class="topbar-sub">{titleInfo.sub}</div>
                </div>

                <div class="topbar-spacer"></div>

                <div class="topbar-iface">
                    <span class="topbar-label">Iface</span>
                    <select
                        class="select"
                        bind:value={selectedInterface}
                        on:change={handleInterfaceChange}
                        disabled={scanning}
                    >
                        {#each interfaces as iface}
                            <option value={iface}>{iface}</option>
                        {/each}
                    </select>
                </div>

                <div class="topbar-divider"></div>

                {#if connectedRSSI != null}
                    <span class="chip ok mono rssi-chip">
                        <span class="dot live"></span>
                        {connectedRSSI} dBm
                    </span>
                {/if}

                <button class="btn ghost" on:click={() => (reportOpen = true)}>
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none"><path d="M6 1.5v7M3 6l3 3 3-3M2 10.5h8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
                    Reports
                </button>

                {#if scanning}
                    <button class="btn danger" on:click={stopScanning}>
                        <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor"><rect x="1" y="1" width="8" height="8" rx="1"/></svg>
                        Stop scan
                    </button>
                {:else}
                    <button class="btn primary" on:click={startScanning}>
                        <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor"><path d="M3 2l7 4-7 4V2z"/></svg>
                        Start scan
                    </button>
                {/if}
            </header>

            {#if errorMessage}
                <div class="error-bar">
                    <span class="error-icon">⚠️</span>
                    <span class="error-message">{errorMessage}</span>
                </div>
            {/if}

            <div class="tab-content">
                {#if activeTab === "networks"}
                    <div class="content-panel">
                        <NetworkList {networks} {clientStats} />
                    </div>
                {:else if activeTab === "signal"}
                    <div class="content-panel signal-panel">
                        <SignalChart {clientStats} {networks} />
                    </div>
                {:else if activeTab === "channels"}
                    <div class="content-panel channel-panel">
                        <ChannelAnalyzer
                            {networks}
                            {clientStats}
                            {channelAnalysis}
                        />
                    </div>
                {:else if activeTab === "stats"}
                    <div class="content-panel stats-panel">
                        <ClientStatsPanel {clientStats} {networks} />
                    </div>
                {:else if activeTab === "latency"}
                    <div class="content-panel signal-panel">
                        <LatencyChart />
                    </div>
                {:else if activeTab === "roaming"}
                    <div class="content-panel channel-panel">
                        <RoamingAnalysis
                            {roamingMetrics}
                            {placementRecommendations}
                            {clientStats}
                            {networks}
                        />
                    </div>
                {:else if activeTab === "settings"}
                    <div class="content-panel stats-panel">
                        <SettingsPanel />
                    </div>
                {/if}
            </div>
        </main>
    </div>
</div>

{#if reportOpen}
    <ReportWindow
        {networks}
        {clientStats}
        on:close={() => (reportOpen = false)}
    />
{/if}

<style>
    @import url("https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap");

    :global(:root) {
        color-scheme: dark light;

        /* Design palette — source of truth */
        --bg-0: #0a0d12;
        --bg-1: #0e1218;
        --bg-2: #12171f;
        --bg-3: #171d27;
        --bg-4: #1e2530;

        --line-1: #1f2630;
        --line-2: #2a313d;
        --line-3: #394251;

        --fg-1: #e6ebf2;
        --fg-2: #a5adbb;
        --fg-3: #6b7383;
        --fg-4: #4b5160;

        --acc-1: #5ce1e6;
        --acc-1-dim: #2b8e92;
        --acc-1-bg: rgba(92, 225, 230, 0.10);
        --acc-1-line: rgba(92, 225, 230, 0.32);

        --ok: #4ade80;
        --ok-bg: rgba(74, 222, 128, 0.10);
        --ok-line: rgba(74, 222, 128, 0.30);

        --warn: #fbbf24;
        --warn-bg: rgba(251, 191, 36, 0.10);
        --warn-line: rgba(251, 191, 36, 0.30);

        --bad: #f87171;
        --bad-bg: rgba(248, 113, 113, 0.10);
        --bad-line: rgba(248, 113, 113, 0.30);

        --font-ui: "Inter", system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", "Helvetica Neue", "Cantarell", "Noto Sans", sans-serif;
        --font-mono: "JetBrains Mono", ui-monospace, "SF Mono", "Cascadia Mono", "DejaVu Sans Mono", Menlo, Consolas, monospace;

        /* Legacy aliases — existing screen components still reference these.
           Mapping them onto the design palette gives those components the new
           look without per-file edits. */
        --bg-0-legacy: var(--bg-0);
        --bg-1-legacy: var(--bg-1);
        --panel: var(--bg-2);
        --panel-strong: var(--bg-3);
        --panel-soft: var(--bg-2);
        --panel-gradient-1: var(--bg-2);
        --panel-gradient-2: var(--bg-1);
        --panel-shadow: 0 18px 30px rgba(0, 0, 0, 0.36);
        --field-bg: var(--bg-4);
        --border: var(--line-1);
        --border-strong: var(--line-2);
        --text: var(--fg-1);
        --muted: var(--fg-2);
        --muted-2: var(--fg-3);
        --accent: var(--acc-1);
        --accent-strong: var(--acc-1);
        --accent-2: var(--acc-1);
        --success: var(--ok);
        --warning: var(--warn);
        --danger: var(--bad);
        --row-hover: var(--bg-3);
        --row-active: var(--ok-bg);
        --tooltip-bg: rgba(10, 13, 18, 0.94);
        --chart-grid: var(--line-1);
        --text-on-accent: #0a0d12;

        /* Channel analyzer block tokens — repointed onto the new palette */
        --channel-block-gloss-1: rgba(255, 255, 255, 0.04);
        --channel-block-gloss-2: rgba(255, 255, 255, 0);
        --channel-block-shade-1: rgba(0, 0, 0, 0.2);
        --channel-block-shade-2: rgba(0, 0, 0, 0.65);
        --channel-block-border: var(--line-1);
        --channel-bg-radial-1: rgba(92, 225, 230, 0.18);
        --channel-bg-radial-2: rgba(92, 225, 230, 0.10);
        --channel-legend-bg: var(--bg-3);
        --channel-legend-border: var(--line-1);
        --channel-hover-border: var(--line-3);
        --channel-hover-shadow: 0 12px 24px rgba(0, 0, 0, 0.4);
        --channel-active-border: var(--acc-1-line);
        --channel-active-shadow: inset 0 0 0 1px var(--acc-1-line);
        --channel-meter-track: var(--bg-4);
        --channel-meter-highlight: var(--fg-2);
        --channel-overview-bg: var(--bg-1);
        --channel-overview-border: var(--line-1);
        --channel-header-border: var(--line-1);
        --channel-id-bg: var(--acc-1-bg);
        --channel-id-border: var(--acc-1-line);
        --channel-band-bg: var(--bg-3);
        --channel-divider: var(--line-1);

        /* Series palette for charts — keep distinct hues */
        --series-1: #5ce1e6;
        --series-2: #4ade80;
        --series-3: #fbbf24;
        --series-4: #f87171;
        --series-5: #a78bfa;
        --series-6: #22d3ee;
        --series-7: #f472b6;
        --series-8: #fb923c;
        --series-9: #94a3b8;
        --series-10: #38bdf8;
    }

    @media (prefers-color-scheme: light) {
        :global(:root) {
            /* Light palette borrows from Sample Diagnostic Report.html — print-leaning */
            --bg-0: #f5f7fb;
            --bg-1: #fafbfc;
            --bg-2: #ffffff;
            --bg-3: #f4f6f9;
            --bg-4: #eef1f6;

            --line-1: #e4e7ec;
            --line-2: #cfd4dc;
            --line-3: #b3bac4;

            --fg-1: #0f141b;
            --fg-2: #3b4352;
            --fg-3: #6b7383;
            --fg-4: #9ca3af;

            --acc-1: #0b6e74;
            --acc-1-dim: #145b60;
            --acc-1-bg: rgba(11, 110, 116, 0.10);
            --acc-1-line: rgba(11, 110, 116, 0.32);

            --ok: #15803d;
            --ok-bg: rgba(21, 128, 61, 0.10);
            --ok-line: rgba(21, 128, 61, 0.30);

            --warn: #a16207;
            --warn-bg: rgba(161, 98, 7, 0.10);
            --warn-line: rgba(161, 98, 7, 0.30);

            --bad: #b91c1c;
            --bad-bg: rgba(185, 28, 28, 0.10);
            --bad-line: rgba(185, 28, 28, 0.30);

            --panel: var(--bg-2);
            --panel-strong: var(--bg-3);
            --panel-soft: var(--bg-3);
            --panel-gradient-1: var(--bg-2);
            --panel-gradient-2: var(--bg-3);
            --panel-shadow: 0 18px 30px rgba(15, 23, 42, 0.10);
            --field-bg: var(--bg-2);
            --border: var(--line-1);
            --border-strong: var(--line-2);
            --text: var(--fg-1);
            --muted: var(--fg-2);
            --muted-2: var(--fg-3);
            --accent: var(--acc-1);
            --accent-strong: var(--acc-1);
            --accent-2: var(--acc-1);
            --success: var(--ok);
            --warning: var(--warn);
            --danger: var(--bad);
            --row-hover: rgba(15, 23, 42, 0.04);
            --row-active: var(--ok-bg);
            --tooltip-bg: rgba(15, 23, 42, 0.92);
            --chart-grid: var(--line-1);
            --text-on-accent: #ffffff;

            --channel-block-gloss-1: rgba(255, 255, 255, 0.85);
            --channel-block-gloss-2: rgba(255, 255, 255, 0);
            --channel-block-shade-1: rgba(15, 23, 42, 0.04);
            --channel-block-shade-2: rgba(15, 23, 42, 0.10);
            --channel-block-border: var(--line-1);
            --channel-bg-radial-1: rgba(11, 110, 116, 0.10);
            --channel-bg-radial-2: rgba(11, 110, 116, 0.06);
            --channel-legend-bg: var(--bg-3);
            --channel-legend-border: var(--line-1);
            --channel-hover-border: var(--line-2);
            --channel-hover-shadow: 0 12px 24px rgba(15, 23, 42, 0.10);
            --channel-active-border: var(--acc-1-line);
            --channel-active-shadow: inset 0 0 0 1px var(--acc-1-line);
            --channel-meter-track: rgba(15, 23, 42, 0.10);
            --channel-meter-highlight: var(--fg-2);
            --channel-overview-bg: var(--bg-3);
            --channel-overview-border: var(--line-1);
            --channel-header-border: var(--line-1);
            --channel-id-bg: var(--acc-1-bg);
            --channel-id-border: var(--acc-1-line);
            --channel-band-bg: var(--bg-3);
            --channel-divider: var(--line-1);

            --series-1: #0b6e74;
            --series-2: #15803d;
            --series-3: #a16207;
            --series-4: #b91c1c;
            --series-5: #6d28d9;
            --series-6: #0891b2;
            --series-7: #be185d;
            --series-8: #c2410c;
            --series-9: #475569;
            --series-10: #0369a1;
        }
    }

    :global(body) {
        margin: 0;
        padding: 0;
        overflow: hidden;
        background: var(--bg-1);
        color: var(--fg-1);
        font-family: var(--font-ui);
        -webkit-font-smoothing: subpixel-antialiased;
        -moz-osx-font-smoothing: auto;
        text-rendering: optimizeLegibility;
        font-synthesis: none;
    }

    .app-container {
        display: flex;
        flex-direction: column;
        height: 100vh;
        background: var(--bg-1);
        color: var(--fg-1);
        font-family: var(--font-ui);
        font-size: 13px;
    }

    .shell {
        flex: 1;
        display: flex;
        min-height: 0;
    }

    /* ── Sidebar ─────────────────────────────────────────────── */
    .sidebar {
        width: 220px;
        background: var(--bg-2);
        border-right: 1px solid var(--line-1);
        display: flex;
        flex-direction: column;
        flex-shrink: 0;
        overflow: hidden;
    }

    .sidebar-brand {
        padding: 18px 16px 14px;
        border-bottom: 1px solid var(--line-1);
    }

    .sidebar-brand-title {
        font-size: 13px;
        font-weight: 600;
        letter-spacing: 0.01em;
        color: var(--fg-1);
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .sidebar-brand-sub {
        font-size: 10px;
        color: var(--fg-3);
        text-transform: uppercase;
        letter-spacing: 0.12em;
        margin-top: 4px;
        font-family: var(--font-mono);
    }

    .sidebar-section-label {
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.12em;
        color: var(--fg-3);
        padding: 14px 16px 6px;
        font-weight: 600;
    }

    .nav-item {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 7px 14px;
        margin: 1px 8px;
        border: none;
        background: transparent;
        border-radius: 5px;
        cursor: pointer;
        color: var(--fg-2);
        font-size: 13px;
        font-weight: 500;
        text-align: left;
        font-family: inherit;
        transition: background 0.1s, color 0.1s;
        position: relative;
    }

    .nav-item:hover {
        background: var(--bg-3);
        color: var(--fg-1);
    }

    .nav-item.active {
        background: var(--acc-1-bg);
        color: var(--acc-1);
    }

    .nav-item.active::before {
        content: "";
        position: absolute;
        left: -8px;
        top: 6px;
        bottom: 6px;
        width: 2px;
        background: var(--acc-1);
        border-radius: 0 2px 2px 0;
    }

    .nav-item :global(svg) {
        flex-shrink: 0;
        opacity: 0.8;
    }

    .nav-item.active :global(svg) {
        opacity: 1;
    }

    .nav-item-badge {
        margin-left: auto;
        font-family: var(--font-mono);
        font-size: 11px;
        color: var(--fg-3);
        background: var(--bg-3);
        padding: 1px 6px;
        border-radius: 3px;
        font-weight: 500;
    }

    .nav-item.active .nav-item-badge {
        background: var(--acc-1-bg);
        color: var(--acc-1);
    }

    .sidebar-footer {
        margin-top: auto;
        padding: 12px;
        border-top: 1px solid var(--line-1);
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .status-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        font-size: 11px;
        color: var(--fg-3);
        font-family: var(--font-mono);
    }

    .status-row .val {
        color: var(--fg-1);
    }

    .scan-state {
        display: inline-flex;
        align-items: center;
        gap: 5px;
        color: var(--fg-3);
    }

    .scan-state.live {
        color: var(--ok);
    }

    .dot {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        display: inline-block;
        flex-shrink: 0;
    }

    .dot.live {
        background: var(--ok);
        box-shadow: 0 0 0 3px rgba(74, 222, 128, 0.15);
        animation: dot-pulse 1.8s ease-in-out infinite;
    }

    @keyframes dot-pulse {
        0%, 100% { box-shadow: 0 0 0 3px rgba(74, 222, 128, 0.15); }
        50% { box-shadow: 0 0 0 5px rgba(74, 222, 128, 0.25); }
    }

    /* ── Main / topbar ───────────────────────────────────────── */
    .main {
        flex: 1;
        display: flex;
        flex-direction: column;
        min-width: 0;
        min-height: 0;
        background: var(--bg-1);
    }

    .topbar {
        height: 52px;
        flex-shrink: 0;
        border-bottom: 1px solid var(--line-1);
        background: var(--bg-1);
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 0 20px;
    }

    .topbar-titles {
        display: flex;
        flex-direction: column;
        line-height: 1.2;
    }

    .topbar-title {
        font-size: 15px;
        font-weight: 600;
        color: var(--fg-1);
        letter-spacing: -0.005em;
    }

    .topbar-sub {
        font-size: 12px;
        color: var(--fg-3);
        margin-top: 2px;
    }

    .topbar-spacer {
        flex: 1;
    }

    .topbar-iface {
        display: flex;
        align-items: center;
        gap: 6px;
    }

    .topbar-label {
        font-size: 11px;
        color: var(--fg-3);
        text-transform: uppercase;
        letter-spacing: 0.1em;
        font-weight: 600;
    }

    .topbar-divider {
        width: 1px;
        height: 20px;
        background: var(--line-2);
    }

    .rssi-chip {
        padding: 4px 10px;
        font-size: 11.5px;
    }

    /* ── Buttons ─────────────────────────────────────────────── */
    .btn {
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
        transition: all 0.1s;
        font-family: inherit;
        white-space: nowrap;
    }

    .btn:hover {
        background: var(--bg-4);
        border-color: var(--line-3);
    }

    .btn.primary {
        background: var(--acc-1);
        color: var(--text-on-accent);
        border-color: var(--acc-1);
        font-weight: 600;
    }

    .btn.primary:hover {
        background: color-mix(in srgb, var(--acc-1) 80%, white);
    }

    .btn.danger {
        background: transparent;
        color: var(--bad);
        border-color: var(--bad-line);
    }

    .btn.danger:hover {
        background: var(--bad-bg);
    }

    .btn.ghost {
        background: transparent;
        border-color: var(--line-2);
    }

    .btn.ghost:hover {
        background: var(--bg-3);
    }

    /* ── Form controls ───────────────────────────────────────── */
    .select {
        background: var(--bg-4);
        border: 1px solid var(--line-2);
        border-radius: 5px;
        padding: 6px 28px 6px 10px;
        font-size: 12px;
        color: var(--fg-1);
        outline: none;
        font-family: inherit;
        appearance: none;
        background-image: url("data:image/svg+xml;charset=utf-8,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='10' viewBox='0 0 10 10'%3E%3Cpath d='M2 4l3 3 3-3' stroke='%23a5adbb' stroke-width='1.2' fill='none' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E");
        background-repeat: no-repeat;
        background-position: right 8px center;
        transition: border-color 0.1s;
    }

    .select:focus {
        border-color: var(--acc-1-line);
    }

    .select:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    /* ── Chips ───────────────────────────────────────────────── */
    .chip {
        display: inline-flex;
        align-items: center;
        gap: 5px;
        padding: 3px 8px;
        border-radius: 12px;
        font-size: 11px;
        font-weight: 500;
        font-family: var(--font-mono);
        border: 1px solid var(--line-2);
        color: var(--fg-2);
        background: var(--bg-3);
        white-space: nowrap;
    }

    .chip.ok {
        color: var(--ok);
        border-color: var(--ok-line);
        background: var(--ok-bg);
    }

    .mono {
        font-family: var(--font-mono);
        font-variant-numeric: tabular-nums;
    }

    /* ── Tab content ─────────────────────────────────────────── */
    .tab-content {
        flex: 1;
        overflow: hidden;
        min-height: 0;
    }

    .content-panel {
        height: 100%;
        overflow: hidden;
        min-height: 0;
    }

    .signal-panel {
        overflow-y: auto;
        scrollbar-gutter: stable;
        -webkit-overflow-scrolling: touch;
        scroll-padding-bottom: 72px;
    }

    .channel-panel {
        overflow-y: auto;
        scrollbar-gutter: stable;
        -webkit-overflow-scrolling: touch;
        scroll-padding-bottom: 72px;
    }

    .stats-panel {
        overflow-y: auto;
        scrollbar-gutter: stable;
        -webkit-overflow-scrolling: touch;
        scroll-padding-bottom: 72px;
    }

    /* ── Error bar ───────────────────────────────────────────── */
    .error-bar {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 10px 20px;
        background: var(--bad-bg);
        border-bottom: 1px solid var(--bad-line);
        flex-shrink: 0;
    }

    .error-icon {
        font-size: 14px;
    }

    .error-message {
        color: var(--bad);
        font-size: 12px;
        flex: 1;
    }

    /* ── Scrollbars ──────────────────────────────────────────── */
    :global(::-webkit-scrollbar) {
        width: 10px;
        height: 10px;
    }

    :global(::-webkit-scrollbar-track) {
        background: transparent;
    }

    :global(::-webkit-scrollbar-thumb) {
        background: var(--line-1);
        border-radius: 5px;
        border: 2px solid transparent;
        background-clip: padding-box;
    }

    :global(::-webkit-scrollbar-thumb:hover) {
        background: var(--line-2);
        background-clip: padding-box;
        border: 2px solid transparent;
    }

    /* ── Responsive ──────────────────────────────────────────── */
    @media (max-width: 768px) {
        .sidebar {
            width: 64px;
        }

        .sidebar-brand-title span,
        .sidebar-brand-sub,
        .sidebar-section-label,
        .nav-item span,
        .nav-item-badge,
        .sidebar-footer {
            display: none;
        }

        .topbar {
            padding: 0 12px;
            gap: 8px;
        }

        .topbar-label {
            display: none;
        }
    }
</style>
