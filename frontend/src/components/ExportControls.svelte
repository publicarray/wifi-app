<script>
  export let networks = null
  export let clientStats = null

  function downloadFile(content, filename) {
    const blob = new Blob([content], { type: 'text/plain;charset=utf-8;' })
    const link = document.createElement('a')
    
    if ('msSaveBlob' in navigator) {
      navigator.msSaveBlob(blob, filename)
    } else {
      const url = URL.createObjectURL(blob)
      link.href = url
      link.download = filename
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
    }
  }
</script>

<div class="export-controls">
  {#if networks && networks.length > 0}
    <div class="export-group">
      <span class="export-label">Export Networks:</span>
      <button 
        class="export-btn btn-csv" 
        on:click={() => downloadFile(networks, 'networks.csv')}
        title="Export networks to CSV"
      >
        CSV
      </button>
      <button 
        class="export-btn btn-json" 
        on:click={() => downloadFile(JSON.stringify(networks, null, 2), 'networks.json')}
        title="Export networks to JSON"
      >
        JSON
      </button>
    </div>
  {/if}

  {#if clientStats}
    <div class="export-group">
      <span class="export-label">Export Stats:</span>
      <button 
        class="export-btn btn-json" 
        on:click={() => downloadFile(JSON.stringify(clientStats, null, 2), 'client-stats.json')}
        title="Export client statistics to JSON"
      >
        JSON
      </button>
    </div>
  {/if}
</div>

<style>
  .export-controls {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
    background: #2a2a2a;
    border-radius: 6px;
  }

  .export-group {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .export-label {
    font-size: 14px;
    font-weight: 500;
    color: #e0e0e0;
    white-space: nowrap;
  }

  .export-btn {
    padding: 8px 16px;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
    transition: all 0.2s ease;
  }

  .export-btn:hover {
    transform: translateY(-1px);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  }

  .export-btn:active {
    transform: translateY(0);
  }

  .btn-csv {
    background: #009688;
    color: white;
  }

  .btn-csv:hover {
    background: #00796b;
  }

  .btn-json {
    background: #f57c00;
    color: white;
  }

  .btn-json:hover {
    background: #d35400;
  }
</style>
