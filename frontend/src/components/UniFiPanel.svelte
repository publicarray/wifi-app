<script>
    // Compact UniFi controller card. Rendered under the network list when the
    // integration is configured (Settings → UniFi controller). Receives the
    // UniFiStatus snapshot from App.svelte (hydrated via GetUniFiStatus and
    // kept live by the `unifi:updated` event).
    export let unifiStatus = null;

    $: devices = unifiStatus?.devices || [];
    $: hasError = !!unifiStatus?.error;

    function stateClass(state) {
        const s = (state || "").toUpperCase();
        if (s === "ONLINE") return "ok";
        if (s === "OFFLINE") return "bad";
        return "warn";
    }

    function lastUpdatedLabel(ts) {
        if (!ts) return "";
        const d = new Date(ts);
        if (isNaN(d.getTime()) || d.getFullYear() < 2000) return "";
        return d.toLocaleTimeString();
    }
</script>

{#if unifiStatus && unifiStatus.configured}
    <div class="unifi-card">
        <div class="unifi-head">
            <span class="dot {unifiStatus.connected && !hasError ? 'ok' : hasError ? 'bad' : 'warn'}"></span>
            <span class="title">UniFi controller</span>
            <span class="meta mono">{unifiStatus.controllerUrl}</span>
            {#if unifiStatus.siteName}
                <span class="meta">site: {unifiStatus.siteName}</span>
            {/if}
            {#if unifiStatus.applicationVersion}
                <span class="meta">v{unifiStatus.applicationVersion}</span>
            {/if}
            {#if unifiStatus.connected}
                <span class="meta">
                    {unifiStatus.wirelessClients} wireless · {unifiStatus.wiredClients} wired clients
                </span>
            {/if}
            {#if lastUpdatedLabel(unifiStatus.lastUpdated)}
                <span class="meta faint">updated {lastUpdatedLabel(unifiStatus.lastUpdated)}</span>
            {/if}
        </div>

        {#if hasError}
            <div class="unifi-error">{unifiStatus.error}</div>
        {/if}

        {#if devices.length > 0}
            <table class="unifi-devices">
                <thead>
                    <tr>
                        <th>Device</th>
                        <th>Model</th>
                        <th>State</th>
                        <th>IP</th>
                        <th title="Device firmware version reported by the controller. Mismatched firmware across APs is a common cause of roaming problems.">Firmware</th>
                        <th title="Wireless clients associated to this device (from the controller)">Clients</th>
                    </tr>
                </thead>
                <tbody>
                    {#each devices as d (d.id)}
                        <tr>
                            <td>{d.name || d.mac}</td>
                            <td class="mono">{d.model}</td>
                            <td><span class="state-pill {stateClass(d.state)}">{(d.state || "?").toLowerCase()}</span></td>
                            <td class="mono">{d.ip}</td>
                            <td class="mono">{d.firmwareVersion || "—"}</td>
                            <td>{d.clientCount}</td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        {/if}
    </div>
{/if}

<style>
    .unifi-card {
        margin-top: 16px;
        background: var(--panel);
        border: 1px solid var(--border);
        border-radius: 10px;
        padding: 14px 16px;
    }

    .unifi-head {
        display: flex;
        align-items: center;
        gap: 12px;
        flex-wrap: wrap;
    }

    .dot {
        width: 9px;
        height: 9px;
        border-radius: 50%;
        flex-shrink: 0;
    }
    .dot.ok { background: var(--ok, #4ade80); }
    .dot.warn { background: var(--warn, #fbbf24); }
    .dot.bad { background: var(--bad, #f87171); }

    .title {
        font-weight: 600;
        color: var(--text);
        font-size: 14px;
    }

    .meta {
        color: var(--muted);
        font-size: 12px;
    }
    .meta.faint { opacity: 0.7; }

    .mono {
        font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }

    .unifi-error {
        margin-top: 10px;
        color: var(--bad, #f87171);
        font-size: 12px;
    }

    .unifi-devices {
        width: 100%;
        margin-top: 12px;
        border-collapse: collapse;
        font-size: 12px;
    }

    .unifi-devices th {
        text-align: left;
        color: var(--muted);
        font-weight: 500;
        padding: 4px 10px 6px 0;
        border-bottom: 1px solid var(--border);
    }

    .unifi-devices td {
        padding: 6px 10px 6px 0;
        color: var(--text);
        border-bottom: 1px solid var(--border);
    }

    .unifi-devices tbody tr:last-child td {
        border-bottom: none;
    }

    .state-pill {
        display: inline-block;
        padding: 1px 8px;
        border-radius: 999px;
        font-size: 11px;
        text-transform: lowercase;
    }
    .state-pill.ok { color: var(--ok, #4ade80); background: var(--ok-bg, rgba(74, 222, 128, 0.12)); }
    .state-pill.warn { color: var(--warn, #fbbf24); background: var(--warn-bg, rgba(251, 191, 36, 0.12)); }
    .state-pill.bad { color: var(--bad, #f87171); background: var(--bad-bg, rgba(248, 113, 113, 0.12)); }
</style>
