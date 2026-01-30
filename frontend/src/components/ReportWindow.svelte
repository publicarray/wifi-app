<script>
    import { createEventDispatcher } from "svelte";
    import ExportControls from "./ExportControls.svelte";

    export let networks = null;
    export let clientStats = null;

    const dispatch = createEventDispatcher();
</script>

<div class="report-backdrop" on:click={() => dispatch("close")}>
    <section class="report-window" on:click|stopPropagation>
        <header class="report-header">
            <div>
                <h2>Reports</h2>
                <p>Export network and client stats reports.</p>
            </div>
            <button class="close-btn" on:click={() => dispatch("close")}>
                ✕
            </button>
        </header>

        <div class="report-body">
            <ExportControls {networks} {clientStats} />
        </div>
    </section>
</div>

<style>
    .report-backdrop {
        position: fixed;
        inset: 0;
        background: rgba(7, 10, 14, 0.6);
        backdrop-filter: blur(8px);
        display: grid;
        place-items: center;
        z-index: 1000;
    }

    .report-window {
        width: min(720px, 92vw);
        max-height: min(78vh, 720px);
        display: flex;
        flex-direction: column;
        background: linear-gradient(180deg, var(--panel), var(--panel-strong));
        border: 1px solid var(--border);
        border-radius: 18px;
        box-shadow: 0 24px 60px rgba(0, 0, 0, 0.35);
        overflow: hidden;
    }

    .report-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;
        padding: 18px 20px;
        border-bottom: 1px solid var(--border);
    }

    .report-header h2 {
        margin: 0;
        font-size: 18px;
        font-weight: 600;
    }

    .report-header p {
        margin: 4px 0 0;
        color: var(--muted);
        font-size: 13px;
    }

    .close-btn {
        border: 1px solid var(--border);
        background: var(--panel-soft);
        color: var(--text);
        border-radius: 10px;
        padding: 6px 10px;
        cursor: pointer;
        font-size: 14px;
    }

    .close-btn:hover {
        border-color: var(--border-strong);
        color: var(--accent);
    }

    .report-body {
        padding: 16px 20px 20px;
        overflow: auto;
    }
</style>
