<script>
    import { createEventDispatcher } from "svelte";
    import ExportControls from "./ExportControls.svelte";

    export let networks = null;
    export let clientStats = null;

    const dispatch = createEventDispatcher();

    function handleKeydown(event) {
        if (event.key === "Escape") {
            dispatch("close");
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- svelte-ignore a11y-click-events-have-key-events -->
<div
    class="modal-scrim"
    role="presentation"
    on:click={() => dispatch("close")}
>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <section
        class="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="report-title"
        on:click|stopPropagation
    >
        <header class="modal-header">
            <div class="header-icon" aria-hidden="true">
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                    <path
                        d="M7 1.5v8M3.5 7l3.5 3.5L10.5 7M2 12.5h10"
                        stroke="currentColor"
                        stroke-width="1.6"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                    />
                </svg>
            </div>
            <div class="header-text">
                <div class="header-title" id="report-title">
                    Export reports
                </div>
                <div class="header-sub">
                    Network survey and client telemetry
                </div>
            </div>
            <button
                type="button"
                class="header-close"
                on:click={() => dispatch("close")}
                aria-label="Close"
            >
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                    <path
                        d="M3 3l8 8M11 3l-8 8"
                        stroke="currentColor"
                        stroke-width="1.5"
                        stroke-linecap="round"
                    />
                </svg>
            </button>
        </header>

        <div class="modal-body">
            <ExportControls
                {networks}
                {clientStats}
                on:exported={() => dispatch("close")}
            />
        </div>
    </section>
</div>

<style>
    .modal-scrim {
        position: fixed;
        inset: 0;
        background: rgba(5, 7, 10, 0.6);
        backdrop-filter: blur(4px);
        display: grid;
        place-items: center;
        z-index: 1000;
        animation: fade-in 0.15s ease;
    }

    @keyframes fade-in {
        from {
            opacity: 0;
        }
        to {
            opacity: 1;
        }
    }

    .modal {
        background: var(--bg-2);
        border: 1px solid var(--line-2);
        border-radius: 10px;
        box-shadow: 0 24px 60px rgba(0, 0, 0, 0.5);
        width: min(560px, 92vw);
        max-height: min(80vh, 720px);
        overflow: hidden;
        display: flex;
        flex-direction: column;
        animation: slide-up 0.18s ease;
    }

    @keyframes slide-up {
        from {
            transform: translateY(8px);
            opacity: 0;
        }
        to {
            transform: translateY(0);
            opacity: 1;
        }
    }

    .modal-header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 16px 20px;
        border-bottom: 1px solid var(--line-1);
    }

    .header-icon {
        width: 30px;
        height: 30px;
        border-radius: 6px;
        background: var(--acc-1-bg);
        color: var(--acc-1);
        display: flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
    }

    .header-text {
        flex: 1;
        min-width: 0;
    }

    .header-title {
        font-size: 14px;
        font-weight: 600;
        color: var(--fg-1);
    }

    .header-sub {
        font-size: 11.5px;
        color: var(--fg-3);
        margin-top: 2px;
    }

    .header-close {
        width: 28px;
        height: 22px;
        background: transparent;
        border: none;
        color: var(--fg-2);
        cursor: pointer;
        border-radius: 4px;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: background 0.12s;
    }

    .header-close:hover {
        background: var(--bg-3);
        color: var(--fg-1);
    }

    .modal-body {
        flex: 1;
        overflow: auto;
    }
</style>
