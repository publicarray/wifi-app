<script>
  export let networks = []
  export let clientStats = null

  // Type helper functions
  function isConnected(stats) {
    return stats && stats.connected === true
  }

  function getConnectedSSID(stats) {
    return isConnected(stats) ? stats.ssid : null
  }

  let expandedNetworks = new Set()
  let sortBy = 'signal' // 'ssid', 'signal', 'channel', 'security'
  let sortOrder = 'desc' // 'asc', 'desc'
  let filterText = ''
  let filterChannel = ''
  let filterSecurity = ''
  let showHidden = false

  $: filteredNetworks = filterNetworks(networks)
  $: sortedNetworks = sortNetworks(filteredNetworks)

  function filterNetworks(networksToFilter) {
    return networksToFilter.filter(network => {
      // Text filter
      if (filterText !== '' && !network.ssid.toLowerCase().includes(filterText.toLowerCase())) {
        return false
      }

      // Channel filter
      if (filterChannel !== '' && network.channel.toString() !== filterChannel) {
        return false
      }

      // Security filter
      if (filterSecurity !== '' && network.security !== filterSecurity) {
        return false
      }

      // Hidden networks filter - only filter if explicitly hiding
      if (showHidden === false && network.ssid === '<Hidden Network>') {
        return false
      }

      return true
    })
  }

  function sortNetworks(networksToSort) {
    return [...networksToSort].sort((a, b) => {
      let aValue, bValue

      switch (sortBy) {
        case 'ssid':
          aValue = a.ssid.toLowerCase()
          bValue = b.ssid.toLowerCase()
          break
        case 'signal':
          aValue = a.bestSignal
          bValue = b.bestSignal
          break
        case 'channel':
          aValue = a.channel
          bValue = b.channel
          break
        case 'security':
          aValue = a.security
          bValue = b.security
          break
        case 'apCount':
          aValue = a.apCount
          bValue = b.apCount
          break
        default:
          return 0
      }

      let comparison = 0
      if (aValue > bValue) comparison = 1
      if (aValue < bValue) comparison = -1

      return sortOrder === 'asc' ? comparison : -comparison
    })
  }

  function toggleSort(column) {
    if (sortBy === column) {
      sortOrder = sortOrder === 'asc' ? 'desc' : 'asc'
    } else {
      sortBy = column
      sortOrder = 'desc' // Default to descending for most columns
    }
  }

  function toggleNetwork(ssid) {
    if (expandedNetworks.has(ssid)) {
      expandedNetworks.delete(ssid)
    } else {
      expandedNetworks.add(ssid)
    }
    expandedNetworks = expandedNetworks
  }

  function getSignalClass(signal) {
    if (signal > -60) return 'signal-good'
    if (signal > -75) return 'signal-medium'
    return 'signal-poor'
  }

  function getSecurityClass(security) {
    if (security === 'Open' || security === 'WEP') return 'security-poor'
    if (security === 'WPA2/TKIP') return 'security-medium'
    return 'security-good'
  }

  function getQamClass(qam) {
    if (!qam) return ''
    return `qam-${qam}`
  }

  // Get unique channels for filter dropdown
  $: availableChannels = [...new Set(networks.map(n => n.channel))].sort((a, b) => a - b)
  
  // Get unique security types for filter dropdown
  $: availableSecurityTypes = [...new Set(networks.map(n => n.security))]
</script>

<div class="network-list-container">
  <!-- Filters -->
  <div class="filters">
    <div class="filter-row">
      <input 
        type="text" 
        placeholder="Filter by SSID..." 
        bind:value={filterText}
        class="filter-input"
      />
      
      <select bind:value={filterChannel} class="filter-select">
        <option value="">All Channels</option>
        {#each availableChannels as channel}
          <option value={channel}>Channel {channel}</option>
        {/each}
      </select>

      <select bind:value={filterSecurity} class="filter-select">
        <option value="">All Security</option>
        {#each availableSecurityTypes as security}
          <option value={security}>{security}</option>
        {/each}
      </select>

      <label class="checkbox-label">
        <input type="checkbox" bind:checked={showHidden} />
        Show Hidden
      </label>
    </div>
  </div>

  <!-- Network Table -->
  <div class="network-table-wrapper">
    <table class="network-table">
      <thead>
        <tr>
          <th class="sortable" on:click={() => toggleSort('ssid')}>
            SSID
            {#if sortBy === 'ssid'}
              <span class="sort-indicator">{sortOrder === 'asc' ? '↑' : '↓'}</span>
            {/if}
          </th>
          <th class="sortable" on:click={() => toggleSort('apCount')}>
            APs
            {#if sortBy === 'apCount'}
              <span class="sort-indicator">{sortOrder === 'asc' ? '↑' : '↓'}</span>
            {/if}
          </th>
          <th class="sortable" on:click={() => toggleSort('channel')}>
            Channel
            {#if sortBy === 'channel'}
              <span class="sort-indicator">{sortOrder === 'asc' ? '↑' : '↓'}</span>
            {/if}
          </th>
          <th class="sortable" on:click={() => toggleSort('signal')}>
            Signal
            {#if sortBy === 'signal'}
              <span class="sort-indicator">{sortOrder === 'asc' ? '↑' : '↓'}</span>
            {/if}
          </th>
          <th class="sortable" on:click={() => toggleSort('security')}>
            Security
            {#if sortBy === 'security'}
              <span class="sort-indicator">{sortOrder === 'asc' ? '↑' : '↓'}</span>
            {/if}
          </th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        {#each sortedNetworks as network}
          <tr 
            class="network-row" 
            class:has-issues={network.hasIssues}
            class:connected={isConnected(clientStats) && getConnectedSSID(clientStats) === network.ssid}
          >
            <td class="ssid-cell" on:click={() => toggleNetwork(network.ssid)}>
              <div class="ssid-content">
                <span class="ssid-text">{network.ssid}</span>
                {#if network.accessPoints && network.accessPoints.length > 0}
                  <span class="vendor-hint">{network.accessPoints[0].vendor}</span>
                {/if}
              </div>
              {#if network.apCount > 1}
                <div class="expand-indicator">
                  {expandedNetworks.has(network.ssid) ? '▼' : '▶'}
                </div>
              {/if}
            </td>
            <td class="ap-count-cell">{network.apCount}</td>
            <td class="channel-cell">{network.channel}</td>
            <td class="signal-cell">
              <span class={getSignalClass(network.bestSignal)}>
                {network.bestSignal} dBm
              </span>
            </td>
            <td class="security-cell">
              <span class={getSecurityClass(network.security)}>
                {network.security}
              </span>
            </td>
            <td class="status-cell">
              {#if network.hasIssues}
                <span class="status-warning">⚠️ Issues</span>
              {:else if isConnected(clientStats) && getConnectedSSID(clientStats) === network.ssid}
                <span class="status-connected">🔗 Connected</span>
              {:else}
                <span class="status-ok">✓ OK</span>
              {/if}
            </td>
          </tr>

          <!-- Expanded AP Details -->
          {#if expandedNetworks.has(network.ssid)}
            <tr class="ap-details-row">
              <td colspan="6">
                <div class="ap-details">
                  {#each network.accessPoints as ap}
                    <div class="ap-card">
                      <div class="ap-header">
                        <span class="ap-bssid">{ap.bssid}</span>
                        <span class="ap-band">{ap.band}</span>
                      </div>
                      <div class="ap-metrics">
                        <div class="ap-metric">
                          <span class="metric-label">Signal:</span>
                          <span class={getSignalClass(ap.signal)}>{ap.signal} dBm</span>
                        </div>
                        <div class="ap-metric">
                          <span class="metric-label">Channel:</span>
                          <span>{ap.channel} ({ap.channelWidth}MHz)</span>
                        </div>
                        <div class="ap-metric">
                          <span class="metric-label">TX Power:</span>
                          <span>{ap.txPower} dBm</span>
                        </div>
                        <div class="ap-metric">
                          <span class="metric-label">Vendor:</span>
                          <span>{ap.vendor}</span>
                        </div>
                      </div>
                      <div class="ap-capabilities">
                        <div class="capability-title">Advanced Capabilities</div>
                        <div class="capability-grid">
                          <div class="capability-item">
                            <span class="capability-label">BSS Transition (802.11v):</span>
                            <span class="capability-value">{ap.bsstransition ? '✓ Supported' : '✗ Not supported'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">UAPSD:</span>
                            <span class="capability-value">{ap.uapsd ? '✓ Enabled' : '✗ Disabled'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">Fast Roaming (802.11r):</span>
                            <span class="capability-value">{ap.fastroaming ? '✓ Supported' : '✗ Not supported'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">DTIM:</span>
                            <span class="capability-value">{ap.dtim}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">PMF:</span>
                            <span class="capability-value pmf-{ap.pmf.toLowerCase()}">{ap.pmf}</span>
                          </div>
                        </div>
                        <div class="capability-title perf-section" style="margin-top: 12px;">Performance & Signal</div>
                        <div class="capability-grid">
                          <div class="capability-item">
                            <span class="capability-label">Real-world Speed:</span>
                            <span class="capability-value">{ap.realWorldSpeed ? `${ap.realWorldSpeed} Mbps` : 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">Estimated Range:</span>
                            <span class="capability-value">{ap.estimatedRange ? `${Math.round(ap.estimatedRange)}m` : 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">SNR:</span>
                            <span class="capability-value">{ap.snr && ap.snr > 0 ? `${ap.snr} dB` : 'N/A'}</span>
                          </div>
                        </div>
                        <div class="capability-title security-section" style="margin-top: 12px;">Security Details</div>
                        <div class="capability-grid full-width">
                          <div class="capability-item">
                            <span class="capability-label">Ciphers:</span>
                            <span class="capability-value">{ap.securityCiphers && ap.securityCiphers.length > 0 ? ap.securityCiphers.join(', ') : 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">Auth Methods:</span>
                            <span class="capability-value">{ap.authMethods && ap.authMethods.length > 0 ? ap.authMethods.join(', ') : 'N/A'}</span>
                          </div>
                        </div>
                        <div class="capability-title wifi6-section" style="margin-top: 12px;">WiFi 6/7 Features</div>
                        <div class="capability-grid">
                          <div class="capability-item">
                            <span class="capability-label">BSS Color:</span>
                            <span class="capability-value">{ap.bssColor || 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">OBSS PD:</span>
                            <span class="capability-value">{ap.obssPD ? '✓ Supported' : '✗ Not supported'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">Max QAM:</span>
                            <span class="capability-value {getQamClass(ap.qamSupport)}">{ap.qamSupport ? `${ap.qamSupport}-QAM` : 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">MU-MIMO:</span>
                            <span class="capability-value">{ap.mumimo ? '✓ Supported' : '✗ Not supported'}</span>
                          </div>
                        </div>
                        <div class="capability-title mgmt-section" style="margin-top: 12px;">Network Management</div>
                        <div class="capability-grid">
                          <div class="capability-item">
                            <span class="capability-label">QoS/WMM:</span>
                            <span class="capability-value">{ap.qosSupport ? '✓ Supported' : '✗ Not supported'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">Country:</span>
                            <span class="capability-value">{ap.countryCode || 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">AP Name:</span>
                            <span class="capability-value">{ap.apName || 'N/A'}</span>
                          </div>
                        </div>
                        <div class="capability-title perf-section" style="margin-top: 12px;">Performance & Signal</div>
                        <div class="capability-grid">
                          <div class="capability-item">
                            <span class="capability-label">Real-world Speed:</span>
                            <span class="capability-value">{ap.realWorldSpeed ? `${ap.realWorldSpeed} Mbps` : 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">Estimated Range:</span>
                            <span class="capability-value">{ap.estimatedRange ? `${Math.round(ap.estimatedRange)}m` : 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">SNR:</span>
                            <span class="capability-value">{ap.snr && ap.snr > 0 ? `${ap.snr} dB` : 'N/A'}</span>
                          </div>
                        </div>
                        <div class="capability-title security-section" style="margin-top: 12px;">Security Details</div>
                        <div class="capability-grid full-width">
                          <div class="capability-item">
                            <span class="capability-label">Ciphers:</span>
                            <span class="capability-value">{ap.securityCiphers && ap.securityCiphers.length > 0 ? ap.securityCiphers.join(', ') : 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">Auth Methods:</span>
                            <span class="capability-value">{ap.authMethods && ap.authMethods.length > 0 ? ap.authMethods.join(', ') : 'N/A'}</span>
                          </div>
                        </div>
                        <div class="capability-title wifi6-section" style="margin-top: 12px;">WiFi 6/7 Features</div>
                        <div class="capability-grid">
                          <div class="capability-item">
                            <span class="capability-label">BSS Color:</span>
                            <span class="capability-value">{ap.bssColor || 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">OBSS PD:</span>
                            <span class="capability-value">{ap.obssPD ? '✓ Supported' : '✗ Not supported'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">Max QAM:</span>
                            <span class="capability-value {getQamClass(ap.qamSupport)}">{ap.qamSupport ? `${ap.qamSupport}-QAM` : 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">MU-MIMO:</span>
                            <span class="capability-value">{ap.mumimo ? '✓ Supported' : '✗ Not supported'}</span>
                          </div>
                        </div>
                        <div class="capability-title mgmt-section" style="margin-top: 12px;">Network Management</div>
                        <div class="capability-grid">
                          <div class="capability-item">
                            <span class="capability-label">QoS/WMM:</span>
                            <span class="capability-value">{ap.qosSupport ? '✓ Supported' : '✗ Not supported'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">Country:</span>
                            <span class="capability-value">{ap.countryCode || 'N/A'}</span>
                          </div>
                          <div class="capability-item">
                            <span class="capability-label">AP Name:</span>
                            <span class="capability-value">{ap.apName || 'N/A'}</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  {/each}
                </div>
              </td>
            </tr>
          {/if}

          <!-- Issues Row -->
          {#if network.hasIssues}
            <tr class="issues-row">
              <td colspan="6">
                <div class="issues-container">
                  {#each network.issueMessages as issue}
                    <div class="issue-item">
                      <span class="issue-icon">⚠️</span>
                      <span class="issue-text">{issue}</span>
                    </div>
                  {/each}
                </div>
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>

    {#if sortedNetworks.length === 0}
      <div class="no-results">
        {#if networks.length === 0}
          <div class="no-networks">
            <span class="no-data-icon">📡</span>
            <p>No networks found. Start scanning to discover WiFi networks.</p>
          </div>
        {:else}
          <p>No networks match the current filters.</p>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .network-list-container {
    height: 100%;
    display: flex;
    flex-direction: column;
    background: #1a1a1a;
  }

  .filters {
    padding: 16px;
    background: #2a2a2a;
    border-bottom: 1px solid #333;
  }

  .filter-row {
    display: flex;
    gap: 12px;
    align-items: center;
    flex-wrap: wrap;
  }

  .filter-input {
    flex: 1;
    min-width: 200px;
    padding: 8px 12px;
    background: #1a1a1a;
    color: #e0e0e0;
    border: 1px solid #444;
    border-radius: 4px;
    font-size: 14px;
  }

  .filter-input:focus {
    outline: none;
    border-color: #0066cc;
  }

  .filter-select {
    padding: 8px 12px;
    background: #1a1a1a;
    color: #e0e0e0;
    border: 1px solid #444;
    border-radius: 4px;
    font-size: 14px;
    min-width: 120px;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 14px;
    color: #aaa;
    cursor: pointer;
  }

  .network-table-wrapper {
    flex: 1;
    overflow: auto;
    border-radius: 0;
  }

  .network-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 14px;
  }

  .network-table th {
    background: #2a2a2a;
    padding: 12px 16px;
    text-align: left;
    font-weight: 600;
    color: #aaa;
    border-bottom: 2px solid #333;
    position: sticky;
    top: 0;
    z-index: 10;
  }

  .network-table th.sortable {
    cursor: pointer;
    user-select: none;
    transition: background-color 0.2s ease;
  }

  .network-table th.sortable:hover {
    background: #333;
  }

  .sort-indicator {
    margin-left: 4px;
    color: #0066cc;
  }

  .network-table td {
    padding: 12px 16px;
    border-bottom: 1px solid #333;
  }

  .network-row {
    transition: background-color 0.2s ease;
  }

  .network-row:hover {
    background: #252525;
  }

  .network-row.connected {
    background: #1a2a1a;
    border-left: 3px solid #4caf50;
  }

  .network-row.has-issues {
    border-left: 3px solid #ff9800;
  }

  .ssid-cell {
    cursor: pointer;
    font-weight: 600;
  }

  .ssid-content {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .ssid-text {
    font-size: 15px;
  }

  .vendor-hint {
    font-size: 12px;
    color: #66b3ff;
    font-style: italic;
  }

  .expand-indicator {
    font-size: 12px;
    color: #888;
    margin-top: 2px;
  }

  .ap-count-cell {
    text-align: center;
    color: #aaa;
  }

  .channel-cell {
    text-align: center;
    font-weight: 500;
  }

  .signal-cell {
    font-weight: 600;
  }

  .security-cell {
    font-weight: 500;
  }

  .signal-good {
    color: #4caf50;
  }

  .signal-medium {
    color: #ff9800;
  }

  .signal-poor {
    color: #f44336;
  }

  .security-good {
    color: #4caf50;
  }

  .security-medium {
    color: #ff9800;
  }

  .security-poor {
    color: #f44336;
  }

  .status-cell {
    text-align: center;
  }

  .status-connected {
    color: #4caf50;
    font-weight: 600;
  }

  .status-warning {
    color: #ff9800;
    font-weight: 600;
  }

  .status-ok {
    color: #888;
  }

  .ap-details-row {
    background: #0f0f0f;
  }

  .ap-details {
    padding: 16px;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 12px;
  }

  .ap-card {
    background: #1a1a1a;
    border: 1px solid #333;
    border-radius: 4px;
    padding: 12px;
  }

  .ap-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
    padding-bottom: 6px;
    border-bottom: 1px solid #333;
  }

  .ap-bssid {
    font-family: monospace;
    font-size: 13px;
    color: #66b3ff;
  }

  .ap-band {
    background: #333;
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 11px;
    color: #aaa;
  }

  .ap-metrics {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px;
  }

  .ap-metric {
    display: flex;
    justify-content: space-between;
    font-size: 12px;
  }

  .metric-label {
    color: #888;
  }

  .ap-capabilities {
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid #333;
  }

  .capability-title {
    font-size: 12px;
    font-weight: 600;
    color: #aaa;
    margin-bottom: 8px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .capability-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px;
  }

  .capability-item {
    display: flex;
    justify-content: space-between;
    font-size: 11px;
    align-items: center;
  }

  .capability-label {
    color: #888;
  }

  .capability-value {
    font-weight: 500;
  }

  .pmf-required {
    color: #4caf50;
  }

  .pmf-optional {
    color: #ff9800;
  }

  .pmf-disabled {
    color: #888;
  }

  .capability-title.perf-section {
    color: #0066cc;
  }

  .capability-title.security-section {
    color: #ff9800;
  }

  .capability-title.wifi6-section {
    color: #9c27b0;
  }

  .capability-title.mgmt-section {
    color: #795548;
  }

  .capability-grid.full-width {
    grid-template-columns: 1fr;
  }

  .qam-256 {
    color: #aaa;
  }

  .qam-1024 {
    color: #0066cc;
    font-weight: 600;
  }

  .qam-4096 {
    color: #4caf50;
    font-weight: 600;
  }

  .issues-row {
    background: #2a1a1a;
  }

  .issues-container {
    padding: 8px 16px;
  }

  .issue-item {
    display: flex;
    align-items: center;
    gap: 6px;
    color: #ff9800;
    font-size: 13px;
    padding: 2px 0;
  }

  .issue-icon {
    font-size: 12px;
  }

  .no-results {
    padding: 40px;
    text-align: center;
    color: #888;
  }

  .no-networks {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
  }

  .no-data-icon {
    font-size: 48px;
    opacity: 0.5;
  }

  /* Responsive adjustments */
  @media (max-width: 768px) {
    .filter-row {
      flex-direction: column;
      align-items: stretch;
    }

    .filter-input {
      min-width: auto;
    }

    .network-table {
      font-size: 12px;
    }

    .network-table th,
    .network-table td {
      padding: 8px 12px;
    }

    .ap-details {
      grid-template-columns: 1fr;
    }
  }
</style>