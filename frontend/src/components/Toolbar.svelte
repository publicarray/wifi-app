<script>
    export let interfaces = [];
    export let selectedInterface = "";
    export let scanning = false;
    export let errorMessage = "";
    export let clientStats = null;

    function handleInterfaceChange(event) {
        const newInterface = event.target.value;
        dispatch("selectInterface", { detail: newInterface });
    }

    function handleStartScanning() {
        dispatch("startScanning");
    }

    function handleStopScanning() {
        dispatch("stopScanning");
    }

    function getSignalClass(signal) {
        if (signal > -60) return "signal-good";
        if (signal > -75) return "signal-medium";
        return "signal-poor";
    }

    // Event dispatcher
    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();
</script>

<div class="toolbar">
    <div class="toolbar-left">
        <h1>WiFi Diagnostic Tool</h1>
        <div class="interface-selector">
            <label for="interface-select">Interface:</label>
            <select
                id="interface-select"
                bind:value={selectedInterface}
                disabled={scanning}
                on:change={handleInterfaceChange}
            >
                {#each interfaces as iface}
                    <option value={iface}>{iface}</option>
                {/each}
            </select>
        </div>
    </div>

    <div class="toolbar-right">
        {#if clientStats && clientStats.connected}
            <div class="signal-pill {getSignalClass(clientStats.signal)}">
                {clientStats.signal} dBm
            </div>
        {/if}
        <div class="scan-controls">
            {#if !scanning}
                <button class="btn btn-primary" on:click={handleStartScanning}>
                    Start Scanning
                </button>
            {:else}
                <div class="scanning-indicator">
                    <div class="scan-dot"></div>
                    <span>Scanning...</span>
                    <button
                        class="btn btn-danger"
                        on:click={handleStopScanning}
                    >
                        Stop
                    </button>
                </div>
            {/if}
        </div>
    </div>
</div>

{#if errorMessage}
    <div class="error-bar">
        <span class="error-icon">⚠️</span>
        <span class="error-message">{errorMessage}</span>
    </div>
{/if}

<style>
    .toolbar {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0px 20px;
        background: var(--panel-soft);
        border-bottom: 2px solid var(--border);
        min-height: 70px;
    }

    .toolbar-left {
        display: flex;
        align-items: center;
        gap: 24px;
    }

    h1 {
        margin: 0;
        font-size: 24px;
        font-weight: 600;
        color: var(--text);
    }

    .interface-selector {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .interface-selector label {
        font-size: 14px;
        font-weight: 500;
        color: var(--muted);
    }

    select {
        padding: 8px 12px;
        background: var(--field-bg);
        color: var(--text);
        border: 1px solid var(--border-strong);
        border-radius: 4px;
        cursor: pointer;
        font-size: 14px;
        min-width: 120px;
        color-scheme: light dark;
    }

    select option {
        background: var(--field-bg);
        color: var(--text);
    }

    select:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    select:focus {
        outline: none;
        border-color: var(--accent-strong);
    }

    .toolbar-right {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .scan-controls {
        display: flex;
        align-items: center;
    }

    .btn {
        padding: 10px 20px;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
        font-size: 14px;
        transition: all 0.2s ease;
    }

    .btn-primary {
        background: var(--accent-strong);
        color: white;
    }

    .btn-primary:hover {
        background: var(--accent);
    }

    .btn-danger {
        background: var(--danger);
        color: white;
        margin-left: 12px;
    }

    .btn-danger:hover {
        background: color-mix(in srgb, var(--danger) 85%, black);
    }

    .scanning-indicator {
        display: flex;
        align-items: center;
        gap: 8px;
        color: var(--success);
        font-weight: 500;
    }

    .signal-pill {
        padding: 6px 12px;
        border-radius: 999px;
        font-size: 12px;
        font-weight: 600;
        letter-spacing: 0.02em;
        border: 1px solid currentColor;
        background: color-mix(in srgb, currentColor 18%, transparent);
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

    .scan-dot {
        width: 8px;
        height: 8px;
        background: var(--success);
        border-radius: 50%;
        animation: pulse 1.5s infinite;
    }

    @keyframes pulse {
        0%,
        100% {
            opacity: 1;
        }
        50% {
            opacity: 0.3;
        }
    }

    .error-bar {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 12px 20px;
        background: color-mix(in srgb, var(--danger) 18%, transparent);
        border-bottom: 1px solid var(--danger);
    }

    .error-icon {
        font-size: 16px;
    }

    .error-message {
        color: var(--danger);
        font-size: 14px;
        flex: 1;
    }

    /* Responsive adjustments */
    @media (max-width: 768px) {
        .toolbar {
            flex-direction: column;
            gap: 12px;
            align-items: stretch;
            padding: 12px 16px;
        }

        .toolbar-left {
            flex-direction: column;
            align-items: stretch;
            gap: 12px;
        }

        h1 {
            font-size: 20px;
            text-align: center;
        }

        .interface-selector {
            justify-content: center;
        }

        .toolbar-right {
            justify-content: center;
        }
    }
</style>
