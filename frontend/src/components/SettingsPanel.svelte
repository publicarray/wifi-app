<script>
    import { onMount } from "svelte";
    import {
        GetConfig,
        SaveConfig,
        GetAvailableInterfaces,
    } from "../../wailsjs/go/main/App.js";

    // Local copy of the config we're editing. Initialized from GetConfig()
    // on mount; saved back via SaveConfig(). Until the load completes the
    // form is disabled so the user can't submit a half-populated config.
    let config = null;
    let interfaces = [];
    let saving = false;
    let savedAt = null;
    let errorMessage = "";
    let dirty = false;

    // latencyTargets is shown as a comma-separated string for human editing
    // and parsed back to an array only at save time — avoids a Svelte
    // reactive cycle that a $: bridge would create.
    let latencyTargetsText = "";

    onMount(async () => {
        try {
            config = await GetConfig();
            latencyTargetsText = (config.latencyTargets || []).join(", ");
        } catch (err) {
            errorMessage = "Failed to load config: " + err;
        }
        try {
            interfaces = (await GetAvailableInterfaces()) || [];
        } catch {
            // non-fatal: interface list is just a dropdown convenience
        }
    });

    async function save() {
        if (!config) return;
        config.latencyTargets = latencyTargetsText
            .split(",")
            .map((s) => s.trim())
            .filter(Boolean);
        saving = true;
        errorMessage = "";
        try {
            await SaveConfig(config);
            savedAt = new Date();
            dirty = false;
        } catch (err) {
            errorMessage = "Save failed: " + err;
        } finally {
            saving = false;
        }
    }

    async function reload() {
        savedAt = null;
        errorMessage = "";
        dirty = false;
        try {
            config = await GetConfig();
            latencyTargetsText = (config.latencyTargets || []).join(", ");
        } catch (err) {
            errorMessage = "Reload failed: " + err;
        }
    }

    // Shared input change handler — flips the dirty flag so the Save button
    // isn't a no-op for users who tab out of an unchanged field.
    function markDirty() {
        dirty = true;
        savedAt = null;
    }
</script>

<div class="settings-panel">
    <h2>Settings</h2>
    <p class="hint">
        Changes apply on the next scan tick. Stored at
        <code>~/.config/wifi-app/config.toml</code> on Linux,
        <code>~/Library/Application Support/wifi-app/config.toml</code> on macOS,
        <code>%APPDATA%\wifi-app\config.toml</code> on Windows.
    </p>

    {#if !config}
        <p class="hint">Loading…</p>
    {:else}
        <form on:submit|preventDefault={save}>
            <label>
                <span>Scan interval (seconds)</span>
                <input
                    type="number"
                    min="1"
                    max="600"
                    bind:value={config.scanIntervalSeconds}
                    on:input={markDirty}
                />
                <small>How often the backend triggers a fresh scan. 4 s is a sensible default. Lower values increase responsiveness at the cost of more contention with NetworkManager and battery drain.</small>
            </label>

            <label>
                <span>Signal history window (minutes)</span>
                <input
                    type="number"
                    min="1"
                    max="240"
                    bind:value={config.signalHistoryMinutes}
                    on:input={markDirty}
                />
                <small>How much per-AP signal history the chart and report can draw on.</small>
            </label>

            <label>
                <span>Roaming history size</span>
                <input
                    type="number"
                    min="1"
                    max="10000"
                    bind:value={config.roamingHistorySize}
                    on:input={markDirty}
                />
                <small>Maximum number of roaming events retained in memory.</small>
            </label>

            <label>
                <span>Default interface</span>
                {#if interfaces.length > 0}
                    <select
                        bind:value={config.defaultInterface}
                        on:change={markDirty}
                    >
                        <option value="">— first available —</option>
                        {#each interfaces as iface}
                            <option value={iface}>{iface}</option>
                        {/each}
                    </select>
                {:else}
                    <input
                        type="text"
                        bind:value={config.defaultInterface}
                        on:input={markDirty}
                        placeholder="e.g. wlp1s0"
                    />
                {/if}
                <small>Pre-selected on launch when present. Leave blank to use the first interface returned by the backend.</small>
            </label>

            <label>
                <span>Latency probe targets</span>
                <input
                    type="text"
                    bind:value={latencyTargetsText}
                    on:input={markDirty}
                    placeholder="gateway, 1.1.1.1"
                />
                <small>Comma-separated. Used by the upcoming latency sampler. <code>gateway</code> resolves to the active default route.</small>
            </label>

            <label>
                <span>Report template override (optional)</span>
                <input
                    type="text"
                    bind:value={config.reportTemplatePath}
                    on:input={markDirty}
                    placeholder="/path/to/template.html"
                />
                <small>Override the default MSP report HTML template. Leave blank to use the embedded default.</small>
            </label>

            <div class="actions">
                <button
                    type="submit"
                    class="primary"
                    disabled={saving || !dirty}
                >
                    {saving ? "Saving…" : "Save"}
                </button>
                <button type="button" on:click={reload} disabled={saving}>
                    Reload from disk
                </button>
                {#if savedAt}
                    <span class="success">Saved at {savedAt.toLocaleTimeString()}</span>
                {/if}
                {#if errorMessage}
                    <span class="error">{errorMessage}</span>
                {/if}
            </div>
        </form>
    {/if}
</div>

<style>
    .settings-panel {
        padding: 24px 32px;
        max-width: 720px;
        margin: 0 auto;
    }

    h2 {
        margin: 0 0 8px;
        color: var(--text);
    }

    .hint {
        color: var(--muted);
        font-size: 13px;
        margin: 0 0 24px;
    }

    code {
        background: var(--field-bg);
        padding: 1px 6px;
        border-radius: 4px;
        font-size: 12px;
    }

    form {
        display: flex;
        flex-direction: column;
        gap: 18px;
    }

    label {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }

    label span {
        font-weight: 500;
        color: var(--text);
        font-size: 14px;
    }

    label small {
        color: var(--muted-2);
        font-size: 12px;
    }

    input,
    select {
        background: var(--field-bg);
        color: var(--text);
        border: 1px solid var(--border);
        border-radius: 6px;
        padding: 8px 10px;
        font-size: 14px;
        font-family: inherit;
    }

    input:focus,
    select:focus {
        outline: none;
        border-color: var(--accent);
    }

    .actions {
        display: flex;
        align-items: center;
        gap: 12px;
        margin-top: 8px;
    }

    button {
        background: var(--panel-soft);
        color: var(--text);
        border: 1px solid var(--border-strong);
        border-radius: 6px;
        padding: 8px 16px;
        font-size: 14px;
        cursor: pointer;
        font-family: inherit;
    }

    button:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    button.primary {
        background: var(--accent);
        color: var(--text-on-accent);
        border-color: var(--accent-strong);
    }

    .success {
        color: var(--success);
        font-size: 13px;
    }

    .error {
        color: var(--danger);
        font-size: 13px;
    }
</style>
