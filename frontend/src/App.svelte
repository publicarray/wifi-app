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
    } from "../wailsjs/go/main/App.js";
    import { EventsOn, EventsOff } from "../wailsjs/runtime/runtime.js";

    import NetworkList from "./components/NetworkList.svelte";
    import SignalChart from "./components/SignalChart.svelte";
    import ChannelAnalyzer from "./components/ChannelAnalyzer.svelte";
    import ClientStatsPanel from "./components/ClientStatsPanel.svelte";
    import RoamingAnalysis from "./components/RoamingAnalysis.svelte";
    import Toolbar from "./components/Toolbar.svelte";

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

    onMount(async () => {
        try {
            interfaces = await GetAvailableInterfaces();
            if (interfaces.length > 0) {
                selectedInterface = interfaces[0];
            }
        } catch (err) {
            errorMessage = "Failed to get WiFi interfaces: " + err;
        }

        EventsOn("networks:updated", (data) => {
            console.log("Networks updated event received:", data);
            console.log("Networks count:", data ? data.length : 0);
            networks = data || [];
        });

        EventsOn("client:updated", (data) => {
            console.log("Client updated event received:", data);
            clientStats = data;
        });

        EventsOn("scan:error", (error) => {
            console.error("Scan error event received:", error);
            errorMessage = error;
        });

        EventsOn("scan:debug", (message) => {
            console.log("Scan debug:", message);
        });

        EventsOn("scan:status", (status) => {
            console.log("Scan status:", status);
        });

        EventsOn("client:warning", (warning) => {
            console.warn("Client warning:", warning);
        });

        EventsOn("roaming:detected", (event) => {
            console.log("Roaming detected:", event);
        });
    });

    onDestroy(() => {
        EventsOff("networks:updated");
        EventsOff("client:updated");
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

    function selectInterface(iface) {
        selectedInterface = iface;
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

    function getTabIcon(tab) {
        switch (tab) {
            case "networks":
                return "📡";
            case "signal":
                return "📊";
            case "channels":
                return "📈";
            case "stats":
                return "📋";
            case "roaming":
                return "🔀";
            default:
                return "";
        }
    }
</script>

<div class="app-container">
    <Toolbar
        {interfaces}
        {selectedInterface}
        {scanning}
        {errorMessage}
        on:selectInterface={(e) => selectInterface(e.detail)}
        on:startScanning={startScanning}
        on:stopScanning={stopScanning}
    />

    <div class="main-tabs">
        {#each ["networks", "signal", "channels", "stats", "roaming"] as tab}
            <button
                class="main-tab"
                class:active={activeTab === tab}
                on:click={() => setActiveTab(tab)}
            >
                <span class="tab-icon">{getTabIcon(tab)}</span>
                <span class="tab-label"
                    >{tab.charAt(0).toUpperCase() + tab.slice(1)}</span
                >
            </button>
        {/each}
    </div>

    <div class="tab-content">
        {#if activeTab === "networks"}
            <div class="content-panel">
                <NetworkList {networks} {clientStats} />
            </div>
        {:else if activeTab === "signal"}
            <div class="content-panel">
                <SignalChart {clientStats} />
            </div>
        {:else if activeTab === "channels"}
            <div class="content-panel channel-panel">
                <ChannelAnalyzer {networks} />
            </div>
        {:else if activeTab === "stats"}
            <div class="content-panel stats-panel">
                <ClientStatsPanel {clientStats} />
            </div>
        {:else if activeTab === "roaming"}
            <div class="content-panel channel-panel">
                <RoamingAnalysis {roamingMetrics} {placementRecommendations} />
            </div>
        {/if}
    </div>
</div>

<style>
    :global(:root) {
        color-scheme: dark light;
        --bg-0: #101216;
        --bg-1: #14171c;
        --panel: #20252b;
        --panel-strong: #1a1f24;
        --panel-soft: #2a2f36;
        --panel-gradient-1: rgba(32, 37, 43, 0.8);
        --panel-gradient-2: rgba(24, 28, 34, 0.8);
        --panel-shadow: 0 18px 30px rgba(0, 0, 0, 0.28);
        --channel-block-gloss-1: rgba(255, 255, 255, 0.04);
        --channel-block-gloss-2: rgba(255, 255, 255, 0);
        --channel-block-shade-1: rgba(0, 0, 0, 0.2);
        --channel-block-shade-2: rgba(0, 0, 0, 0.65);
        --channel-block-border: rgba(255, 255, 255, 0.08);
        --channel-bg-radial-1: rgba(79, 209, 197, 0.18);
        --channel-bg-radial-2: rgba(59, 130, 246, 0.14);
        --channel-legend-bg: rgba(255, 255, 255, 0.04);
        --channel-legend-border: rgba(255, 255, 255, 0.06);
        --channel-hover-border: rgba(255, 255, 255, 0.2);
        --channel-hover-shadow: 0 12px 24px rgba(0, 0, 0, 0.35);
        --channel-active-border: rgba(79, 209, 197, 0.5);
        --channel-active-shadow: inset 0 0 0 1px rgba(79, 209, 197, 0.2);
        --channel-meter-track: rgba(255, 255, 255, 0.1);
        --channel-meter-highlight: rgba(255, 255, 255, 0.75);
        --channel-overview-bg: rgba(0, 0, 0, 0.35);
        --channel-overview-border: rgba(255, 255, 255, 0.06);
        --channel-header-border: rgba(255, 255, 255, 0.06);
        --channel-id-bg: rgba(79, 209, 197, 0.12);
        --channel-id-border: rgba(79, 209, 197, 0.25);
        --channel-band-bg: rgba(255, 255, 255, 0.08);
        --channel-divider: rgba(255, 255, 255, 0.06);
        --text-on-accent: #0f172a;
        --field-bg: #15181d;
        --border: rgba(255, 255, 255, 0.08);
        --border-strong: rgba(255, 255, 255, 0.16);
        --text: #e6e8eb;
        --muted: #9aa3ad;
        --muted-2: #7d8793;
        --accent: #3b82f6;
        --accent-strong: #2563eb;
        --accent-2: #7dd3fc;
        --success: #22c55e;
        --warning: #f59e0b;
        --danger: #ef4444;
        --row-hover: rgba(255, 255, 255, 0.04);
        --row-active: rgba(34, 197, 94, 0.12);
        --tooltip-bg: rgba(20, 22, 27, 0.92);
        --chart-grid: rgba(255, 255, 255, 0.08);
    }

    @media (prefers-color-scheme: light) {
        :global(:root) {
            --bg-0: #f5f7fb;
            --bg-1: #eef1f6;
            --panel: #ffffff;
            --panel-strong: #f1f3f6;
            --panel-soft: #e7ecf2;
            --panel-gradient-1: rgba(255, 255, 255, 0.9);
            --panel-gradient-2: rgba(235, 240, 247, 0.9);
            --panel-shadow: 0 18px 30px rgba(15, 23, 42, 0.14);
            --channel-block-gloss-1: rgba(255, 255, 255, 0.9);
            --channel-block-gloss-2: rgba(255, 255, 255, 0);
            --channel-block-shade-1: rgba(15, 23, 42, 0.04);
            --channel-block-shade-2: rgba(15, 23, 42, 0.12);
            --channel-block-border: rgba(15, 23, 42, 0.12);
            --channel-bg-radial-1: rgba(14, 165, 233, 0.12);
            --channel-bg-radial-2: rgba(37, 99, 235, 0.12);
            --channel-legend-bg: rgba(15, 23, 42, 0.04);
            --channel-legend-border: rgba(15, 23, 42, 0.08);
            --channel-hover-border: rgba(15, 23, 42, 0.2);
            --channel-hover-shadow: 0 12px 24px rgba(15, 23, 42, 0.12);
            --channel-active-border: rgba(14, 165, 233, 0.4);
            --channel-active-shadow: inset 0 0 0 1px rgba(14, 165, 233, 0.18);
            --channel-meter-track: rgba(15, 23, 42, 0.12);
            --channel-meter-highlight: rgba(55, 60, 45, 0.7);
            --channel-overview-bg: rgba(15, 23, 42, 0.04);
            --channel-overview-border: rgba(15, 23, 42, 0.08);
            --channel-header-border: rgba(15, 23, 42, 0.08);
            --channel-id-bg: rgba(14, 165, 233, 0.14);
            --channel-id-border: rgba(14, 165, 233, 0.28);
            --channel-band-bg: rgba(15, 23, 42, 0.08);
            --channel-divider: rgba(15, 23, 42, 0.08);
            --text-on-accent: #0f172a;
            --field-bg: #ffffff;
            --border: rgba(15, 23, 42, 0.12);
            --border-strong: rgba(15, 23, 42, 0.2);
            --text: #1b1f24;
            --muted: #5f6b7a;
            --muted-2: #7a8696;
            --accent: #2563eb;
            --accent-strong: #1d4ed8;
            --accent-2: #0ea5e9;
            --success: #16a34a;
            --warning: #d97706;
            --danger: #dc2626;
            --row-hover: rgba(15, 23, 42, 0.06);
            --row-active: rgba(22, 163, 74, 0.12);
            --tooltip-bg: rgba(15, 23, 42, 0.92);
            --chart-grid: rgba(15, 23, 42, 0.12);
        }
    }

    :global(body) {
        margin: 0;
        padding: 0;
        overflow: hidden;
        background: var(--bg-0);
        color: var(--text);
    }

    .app-container {
        display: flex;
        flex-direction: column;
        height: 100vh;
        background: var(--bg-0);
        color: var(--text);
        font-family:
            -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen,
            Ubuntu, Cantarell, sans-serif;
    }

    .main-tabs {
        display: flex;
        background: var(--panel-soft);
        border-bottom: 2px solid var(--border);
        padding: 0 20px;
    }

    .main-tab {
        flex: 1;
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 8px;
        padding: 16px 12px;
        background: transparent;
        color: var(--muted-2);
        border: none;
        cursor: pointer;
        font-size: 14px;
        font-weight: 500;
        transition: all 0.2s ease;
        border-bottom: 3px solid transparent;
    }

    .main-tab:hover {
        background: var(--panel-strong);
        color: var(--text);
    }

    .main-tab.active {
        background: var(--panel-strong);
        color: var(--accent-strong);
        border-bottom-color: var(--accent-strong);
    }

    .tab-icon {
        font-size: 20px;
    }

    .tab-label {
        font-size: 14px;
        font-weight: 500;
    }

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

    .channel-panel {
        overflow-y: auto;
        scrollbar-gutter: stable;
        -webkit-overflow-scrolling: touch;
        scroll-padding-bottom: 72px;
    }

    .stats-panel {
        overflow-y: auto;
        max-width: 900px;
        margin: 0 auto;
    }

    @media (max-width: 768px) {
        .main-tabs {
            padding: 0 8px;
        }

        .main-tab {
            padding: 12px 8px;
        }

        .tab-icon {
            font-size: 18px;
        }

        .tab-label {
            font-size: 12px;
        }
    }
</style>
