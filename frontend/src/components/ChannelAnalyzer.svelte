<script>
  export let networks = []

  $: channels2_4GHz = analyze2_4GHzChannels(networks)
  $: channels5GHz = analyze5GHzChannels(networks)
  $: channelWidthMap = getChannelWidthMap(networks)

  function analyze2_4GHzChannels(networksToAnalyze) {
    const channels = []
    for (let i = 1; i <= 14; i++) {
      const channelNetworks = networksToAnalyze.filter(network => 
        network.accessPoints && network.accessPoints.some(ap => ap.channel === i)
      )
      
      const apsOnChannel = channelNetworks.flatMap(network => 
        network.accessPoints.filter(ap => ap.channel === i)
      )

      channels.push({
        number: i,
        frequency: 2407 + (i * 5),
        networks: channelNetworks,
        aps: apsOnChannel,
        utilization: calculateUtilization(apsOnChannel),
        congestion: getCongestionLevel(apsOnChannel.length),
        overlapping: calculateOverlapping(i, networksToAnalyze)
      })
    }
    return channels
  }

  function analyze5GHzChannels(networksToAnalyze) {
    const channels = []
    const common5GHzChannels = [36, 40, 44, 48, 52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124, 128, 132, 136, 140, 144, 149, 153, 157, 161, 165]
    
    common5GHzChannels.forEach(channel => {
      const channelNetworks = networksToAnalyze.filter(network => 
        network.accessPoints && network.accessPoints.some(ap => ap.channel === channel)
      )
      
      const apsOnChannel = channelNetworks.flatMap(network => 
        network.accessPoints.filter(ap => ap.channel === channel)
      )

      channels.push({
        number: channel,
        frequency: 5000 + (channel * 5),
        networks: channelNetworks,
        aps: apsOnChannel,
        utilization: calculateUtilization(apsOnChannel),
        congestion: getCongestionLevel(apsOnChannel.length),
        overlapping: 0 // 5GHz channels don't overlap
      })
    })
    return channels
  }

  function calculateUtilization(aps) {
    if (aps.length === 0) return 0
    // Rough utilization estimate based on AP count and activity
    return Math.min(aps.length * 20, 100)
  }

  function getCongestionLevel(apCount) {
    if (apCount === 0) return 'empty'
    if (apCount === 1) return 'low'
    if (apCount <= 3) return 'medium'
    return 'high'
  }

  function calculateOverlapping(channel, networksToAnalyze) {
    if (channel > 14) return 0
    // 2.4GHz channels overlap by 4 channels
    let overlap = 0
    for (let i = channel - 4; i <= channel + 4; i++) {
      if (i !== channel && i >= 1 && i <= 14) {
        const hasNetwork = networksToAnalyze.some(network => 
          network.accessPoints && network.accessPoints.some(ap => ap.channel === i)
        )
        if (hasNetwork) overlap++
      }
    }
    return overlap
  }

  function getChannelWidthMap(networksToMap) {
    const widthMap = {}
    networksToMap.forEach(network => {
      network.accessPoints.forEach(ap => {
        const key = `${ap.channel}-${ap.band}`
        widthMap[key] = Math.max(widthMap[key] || 0, ap.channelWidth || 20)
      })
    })
    return widthMap
  }

  function getCongestionColor(congestion) {
    switch (congestion) {
      case 'empty': return '#333'
      case 'low': return '#4caf50'
      case 'medium': return '#ff9800'
      case 'high': return '#f44336'
      default: return '#666'
    }
  }

  function getUtilizationColor(utilization) {
    if (utilization < 30) return '#4caf50'
    if (utilization < 70) return '#ff9800'
    return '#f44336'
  }

  function formatFrequency(freq) {
    return (freq / 1000).toFixed(3) + ' GHz'
  }

  function getChannelWidth(channel, band) {
    const key = `${channel}-${band}`
    return channelWidthMap[key] || 20
  }
</script>

<div class="channel-analyzer-container">
  <div class="analyzer-header">
    <h3>Channel Analysis</h3>
    <div class="legend">
      <div class="legend-item">
        <div class="legend-color empty"></div>
        <span>Empty</span>
      </div>
      <div class="legend-item">
        <div class="legend-color low"></div>
        <span>Low (1 AP)</span>
      </div>
      <div class="legend-item">
        <div class="legend-color medium"></div>
        <span>Medium (2-3 APs)</span>
      </div>
      <div class="legend-item">
        <div class="legend-color high"></div>
        <span>High (4+ APs)</span>
      </div>
    </div>
  </div>

  <!-- 2.4GHz Band -->
  <div class="band-section">
    <h4>2.4GHz Band</h4>
    <div class="channel-grid">
      {#each channels2_4GHz as channel}
        <div 
          class="channel-block" 
          class:has-aps={channel.aps.length > 0}
          style="--congestion-color: {getCongestionColor(channel.congestion)}"
          title="Channel {channel.number} ({formatFrequency(channel.frequency)}) - {channel.aps.length} APs - {channel.overlapping} overlapping"
        >
          <div class="channel-number">{channel.number}</div>
          <div class="channel-info">
            <div class="ap-count">{channel.aps.length}</div>
            {#if channel.overlapping > 0}
              <div class="overlap-indicator">+{channel.overlapping}</div>
            {/if}
          </div>
        </div>
      {/each}
    </div>
    <div class="band-overview">
      <div class="overview-stat">
        <span class="stat-label">Active Channels:</span>
        <span class="stat-value">{channels2_4GHz.filter(c => c.aps.length > 0).length}/14</span>
      </div>
      <div class="overview-stat">
        <span class="stat-label">Total APs:</span>
        <span class="stat-value">{channels2_4GHz.reduce((sum, c) => sum + c.aps.length, 0)}</span>
      </div>
      <div class="overview-stat">
        <span class="stat-label">Most Congested:</span>
        <span class="stat-value">
          {channels2_4GHz.reduce((max, c) => c.aps.length > max.aps.length ? c : max, {number: 'N/A', aps: []}).number}
        </span>
      </div>
    </div>
  </div>

  <!-- 5GHz Band -->
  <div class="band-section">
    <h4>5GHz Band</h4>
    <div class="channel-grid fiveghz">
      {#each channels5GHz as channel}
        <div 
          class="channel-block" 
          class:has-aps={channel.aps.length > 0}
          style="--congestion-color: {getCongestionColor(channel.congestion)}"
          title="Channel {channel.number} ({formatFrequency(channel.frequency)}) - {channel.aps.length} APs"
        >
          <div class="channel-number">{channel.number}</div>
          <div class="channel-info">
            <div class="ap-count">{channel.aps.length}</div>
          </div>
        </div>
      {/each}
    </div>
    <div class="band-overview">
      <div class="overview-stat">
        <span class="stat-label">Active Channels:</span>
        <span class="stat-value">{channels5GHz.filter(c => c.aps.length > 0).length}/{channels5GHz.length}</span>
      </div>
      <div class="overview-stat">
        <span class="stat-label">Total APs:</span>
        <span class="stat-value">{channels5GHz.reduce((sum, c) => sum + c.aps.length, 0)}</span>
      </div>
      <div class="overview-stat">
        <span class="stat-label">Most Congested:</span>
        <span class="stat-value">
          {channels5GHz.reduce((max, c) => c.aps.length > max.aps.length ? c : max, {number: 'N/A', aps: []}).number}
        </span>
      </div>
    </div>
  </div>

  <!-- Detailed Channel List -->
  <div class="channel-details">
    <h4>Channel Details</h4>
    <div class="channel-list">
      {#each [...channels2_4GHz, ...channels5GHz].filter(c => c.aps.length > 0) as channel}
        <div class="channel-detail-item">
          <div class="channel-header">
            <span class="channel-id">Ch {channel.number}</span>
            <span class="channel-freq">{formatFrequency(channel.frequency)}</span>
            <span class="channel-band">{channel.number <= 14 ? '2.4GHz' : '5GHz'}</span>
            <div class="channel-metrics">
              <span class="congestion-badge {channel.congestion}">{channel.congestion}</span>
              <span class="utilization-bar" style="width: {channel.utilization}%; background: {getUtilizationColor(channel.utilization)}"></span>
              <span class="utilization-text">{channel.utilization}%</span>
            </div>
          </div>
          <div class="channel-networks">
            {#each channel.aps.slice(0, 3) as ap}
              <div class="ap-item">
                <span class="ap-ssid">{ap.ssid || '<Hidden>'}</span>
                <span class="ap-signal">{ap.signal} dBm</span>
                <span class="ap-width">{ap.channelWidth || 20}MHz</span>
              </div>
            {/each}
            {#if channel.aps.length > 3}
              <div class="more-aps">+{channel.aps.length - 3} more APs</div>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .channel-analyzer-container {
    height: 100%;
    overflow-y: auto;
    background: #1a1a1a;
    padding: 16px;
  }

  .analyzer-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
  }

  .analyzer-header h3 {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: #e0e0e0;
  }

  .legend {
    display: flex;
    gap: 16px;
    flex-wrap: wrap;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: #aaa;
  }

  .legend-color {
    width: 12px;
    height: 12px;
    border-radius: 2px;
  }

  .legend-color.empty { background: #333; }
  .legend-color.low { background: #4caf50; }
  .legend-color.medium { background: #ff9800; }
  .legend-color.high { background: #f44336; }

  .band-section {
    margin-bottom: 24px;
  }

  .band-section h4 {
    margin: 0 0 12px 0;
    font-size: 16px;
    font-weight: 500;
    color: #e0e0e0;
  }

  .channel-grid {
    display: grid;
    grid-template-columns: repeat(14, 1fr);
    gap: 4px;
    margin-bottom: 12px;
  }

  .channel-grid.fiveghz {
    grid-template-columns: repeat(8, 1fr);
  }

  .channel-block {
    aspect-ratio: 1;
    background: var(--congestion-color, #333);
    border: 1px solid #444;
    border-radius: 4px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    min-height: 60px;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .channel-block:hover {
    transform: scale(1.05);
    border-color: #666;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
  }

  .channel-block.has-aps {
    border-color: #66b3ff;
  }

  .channel-number {
    font-size: 14px;
    font-weight: 600;
    color: #e0e0e0;
  }

  .channel-info {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    font-size: 11px;
  }

  .ap-count {
    color: #66b3ff;
    font-weight: 600;
  }

  .overlap-indicator {
    color: #ff9800;
    font-size: 10px;
  }

  .band-overview {
    display: flex;
    gap: 24px;
    padding: 12px;
    background: #2a2a2a;
    border-radius: 4px;
    flex-wrap: wrap;
  }

  .overview-stat {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .stat-label {
    font-size: 12px;
    color: #888;
  }

  .stat-value {
    font-size: 14px;
    font-weight: 600;
    color: #e0e0e0;
  }

  .channel-details h4 {
    margin: 20px 0 12px 0;
    font-size: 16px;
    font-weight: 500;
    color: #e0e0e0;
  }

  .channel-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .channel-detail-item {
    background: #2a2a2a;
    border: 1px solid #333;
    border-radius: 4px;
    overflow: hidden;
  }

  .channel-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    background: #1f1f1f;
    border-bottom: 1px solid #333;
  }

  .channel-id {
    font-weight: 600;
    color: #e0e0e0;
    min-width: 40px;
  }

  .channel-freq {
    color: #888;
    font-size: 13px;
  }

  .channel-band {
    background: #333;
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 11px;
    color: #aaa;
  }

  .channel-metrics {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .congestion-badge {
    padding: 2px 8px;
    border-radius: 3px;
    font-size: 11px;
    font-weight: 500;
    text-transform: uppercase;
  }

  .congestion-badge.empty { background: #333; color: #888; }
  .congestion-badge.low { background: #4caf50; color: white; }
  .congestion-badge.medium { background: #ff9800; color: white; }
  .congestion-badge.high { background: #f44336; color: white; }

  .utilization-bar {
    height: 8px;
    border-radius: 4px;
    min-width: 40px;
  }

  .utilization-text {
    font-size: 12px;
    color: #aaa;
    min-width: 30px;
  }

  .channel-networks {
    padding: 8px 16px;
  }

  .ap-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 4px 0;
    font-size: 13px;
  }

  .ap-ssid {
    color: #e0e0e0;
    flex: 1;
  }

  .ap-signal {
    color: #66b3ff;
    font-weight: 500;
    min-width: 50px;
    text-align: right;
  }

  .ap-width {
    color: #aaa;
    min-width: 40px;
    text-align: right;
  }

  .more-aps {
    color: #888;
    font-style: italic;
    font-size: 12px;
    padding: 4px 0;
  }

  /* Responsive adjustments */
  @media (max-width: 768px) {
    .channel-analyzer-container {
      padding: 12px;
    }

    .analyzer-header {
      flex-direction: column;
      align-items: flex-start;
      gap: 12px;
    }

    .channel-grid {
      grid-template-columns: repeat(7, 1fr);
    }

    .channel-grid.fiveghz {
      grid-template-columns: repeat(4, 1fr);
    }

    .band-overview {
      flex-direction: column;
      gap: 8px;
    }

    .channel-header {
      flex-wrap: wrap;
      gap: 8px;
    }

    .channel-metrics {
      width: 100%;
      margin-left: 0;
      justify-content: space-between;
    }
  }
</style>