<script>
  export let interfaces = []
  export let selectedInterface = ''
  export let scanning = false
  export let errorMessage = ''

  function handleInterfaceChange(event) {
    const newInterface = event.target.value
    dispatch('selectInterface', { detail: newInterface })
  }

  function handleStartScanning() {
    dispatch('startScanning')
  }

  function handleStopScanning() {
    dispatch('stopScanning')
  }

  // Event dispatcher
  import { createEventDispatcher } from 'svelte'
  const dispatch = createEventDispatcher()
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
    <div class="scan-controls">
      {#if !scanning}
        <button class="btn btn-primary" on:click={handleStartScanning}>
          Start Scanning
        </button>
      {:else}
        <div class="scanning-indicator">
          <div class="scan-dot"></div>
          <span>Scanning...</span>
          <button class="btn btn-danger" on:click={handleStopScanning}>
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
    padding: 16px 20px;
    background: #2a2a2a;
    border-bottom: 2px solid #333;
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
    color: #e0e0e0;
  }

  .interface-selector {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .interface-selector label {
    font-size: 14px;
    font-weight: 500;
    color: #aaa;
  }

  select {
    padding: 8px 12px;
    background: #1a1a1a;
    color: #e0e0e0;
    border: 1px solid #444;
    border-radius: 4px;
    cursor: pointer;
    font-size: 14px;
    min-width: 120px;
  }

  select:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  select:focus {
    outline: none;
    border-color: #0066cc;
  }

  .toolbar-right {
    display: flex;
    align-items: center;
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
    background: #0066cc;
    color: white;
  }

  .btn-primary:hover {
    background: #0052a3;
  }

  .btn-danger {
    background: #cc0000;
    color: white;
    margin-left: 12px;
  }

  .btn-danger:hover {
    background: #a30000;
  }

  .scanning-indicator {
    display: flex;
    align-items: center;
    gap: 8px;
    color: #4caf50;
    font-weight: 500;
  }

  .scan-dot {
    width: 8px;
    height: 8px;
    background: #4caf50;
    border-radius: 50%;
    animation: pulse 1.5s infinite;
  }

  @keyframes pulse {
    0%, 100% {
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
    background: #4a1515;
    border-bottom: 1px solid #8b0000;
  }

  .error-icon {
    font-size: 16px;
  }

  .error-message {
    color: #ff6b6b;
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