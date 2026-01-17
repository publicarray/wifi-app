<script>
  export let clientStats = null

  function getSignalClass(signal) {
    if (signal > -60) return 'signal-good'
    if (signal > -75) return 'signal-medium'
    return 'signal-poor'
  }

  function getSignalQuality(signal) {
    if (signal > -60) return { text: 'Excellent', color: '#4caf50' }
    if (signal > -70) return { text: 'Good', color: '#8bc34a' }
    if (signal > -80) return { text: 'Fair', color: '#ff9800' }
    return { text: 'Poor', color: '#f44336' }
  }

  function formatBytes(bytes) {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  function formatDuration(seconds) {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    const secs = seconds % 60

    if (hours > 0) {
      return `${hours}h ${minutes}m ${secs}s`
    } else if (minutes > 0) {
      return `${minutes}m ${secs}s`
    }
    return `${secs}s`
  }

  function getRetryRateClass(retryRate) {
    if (retryRate < 5) return 'rate-good'
    if (retryRate < 10) return 'rate-medium'
    return 'rate-poor'
  }
</script>

<div class="client-stats-panel">
  <div class="panel-header">
    <h3>Client Connection Stats</h3>
  </div>

  {#if clientStats && clientStats.connected}
    <div class="section">
      <h4>Connection</h4>
      <div class="info-grid">
        <div class="info-item">
          <span class="label">SSID</span>
          <span class="value ssid">{clientStats.ssid || 'Unknown'}</span>
        </div>
        <div class="info-item">
          <span class="label">BSSID</span>
          <span class="value bssid">{clientStats.bssid || 'Unknown'}</span>
        </div>
        <div class="info-item">
          <span class="label">Interface</span>
          <span class="value">{clientStats.interface || 'Unknown'}</span>
        </div>
        <div class="info-item">
          <span class="label">Duration</span>
          <span class="value">{formatDuration(clientStats.connectedTime || 0)}</span>
        </div>
      </div>
    </div>

    <div class="section">
      <h4>Signal Quality</h4>
      <div class="info-grid">
        <div class="info-item full-width">
          <span class="label">Current Signal</span>
          <span class="value {getSignalClass(clientStats.signal)}">
            {clientStats.signal} dBm
          </span>
          <span class="quality-badge" style="background: {getSignalQuality(clientStats.signal).color}">
            {getSignalQuality(clientStats.signal).text}
          </span>
        </div>
        <div class="info-item">
          <span class="label">Average Signal</span>
          <span class="value {getSignalClass(clientStats.signalAvg || clientStats.signal)}">
            {clientStats.signalAvg || clientStats.signal} dBm
          </span>
        </div>
        <div class="info-item">
          <span class="label">Noise</span>
          <span class="value">{clientStats.noise} dBm</span>
        </div>
        <div class="info-item full-width">
          <span class="label">SNR</span>
          <span class="value">{clientStats.snr} dB</span>
          <div class="snr-bar">
            <div class="snr-fill" style="width: {Math.min(Math.max(clientStats.snr, 0), 50) * 2}%"></div>
          </div>
        </div>
        <div class="info-item">
          <span class="label">Last ACK Signal</span>
          <span class="value {getSignalClass(clientStats.lastAckSignal)}">
            {clientStats.lastAckSignal} dBm
          </span>
        </div>
      </div>
    </div>

    <div class="section">
      <h4>Data Rates</h4>
      <div class="info-grid">
        <div class="info-item">
          <span class="label">TX Rate</span>
          <span class="value rate">{clientStats.txBitrate.toFixed(1)} Mbps</span>
        </div>
        <div class="info-item">
          <span class="label">RX Rate</span>
          <span class="value rate">{clientStats.rxBitrate.toFixed(1)} Mbps</span>
        </div>
        <div class="info-item">
          <span class="label">Channel</span>
          <span class="value">{clientStats.channel} ({clientStats.channelWidth}MHz)</span>
        </div>
        <div class="info-item">
          <span class="label">Frequency</span>
          <span class="value">{(clientStats.frequency / 1000).toFixed(3)} GHz</span>
        </div>
      </div>
    </div>

    <div class="section">
      <h4>Traffic Statistics</h4>
      <div class="info-grid">
        <div class="info-item">
          <span class="label">TX Bytes</span>
          <span class="value">{formatBytes(clientStats.txBytes)}</span>
        </div>
        <div class="info-item">
          <span class="label">RX Bytes</span>
          <span class="value">{formatBytes(clientStats.rxBytes)}</span>
        </div>
        <div class="info-item">
          <span class="label">TX Packets</span>
          <span class="value">{clientStats.txPackets.toLocaleString()}</span>
        </div>
        <div class="info-item">
          <span class="label">RX Packets</span>
          <span class="value">{clientStats.rxPackets.toLocaleString()}</span>
        </div>
      </div>
    </div>

    <div class="section">
      <h4>Error Statistics</h4>
      <div class="info-grid">
        <div class="info-item full-width">
          <span class="label">Retry Rate</span>
          <span class="value {getRetryRateClass(clientStats.retryRate)}">
            {clientStats.retryRate.toFixed(1)}%
          </span>
          <div class="retry-bar">
            <div
              class="retry-fill {getRetryRateClass(clientStats.retryRate)}"
              style="width: {Math.min(clientStats.retryRate, 100)}%"
            ></div>
          </div>
        </div>
        <div class="info-item">
          <span class="label">TX Retries</span>
          <span class="value">{clientStats.txRetries.toLocaleString()}</span>
        </div>
        <div class="info-item">
          <span class="label">TX Failed</span>
          <span class="value">{clientStats.txFailed.toLocaleString()}</span>
        </div>
      </div>
    </div>

    {#if clientStats.roamingHistory && clientStats.roamingHistory.length > 0}
      <div class="section">
        <h4>Roaming History</h4>
        <div class="roaming-list">
          {#each clientStats.roamingHistory.slice().reverse() as event}
            <div class="roaming-event">
              <div class="roaming-time">
                {new Date(event.timestamp).toLocaleTimeString()}
              </div>
              <div class="roaming-details">
                <div class="roaming-path">
                  <span class="bssid-from">{event.previousBssid.slice(-6)}</span>
                  <span class="arrow">→</span>
                  <span class="bssid-to">{event.newBssid.slice(-6)}</span>
                </div>
                <div class="roaming-signals">
                  <span class="signal-change">
                    {event.previousSignal} dBm → {event.newSignal} dBm
                  </span>
                  <span class="channel-change">
                    Ch {event.previousChannel} → {event.newChannel}
                  </span>
                </div>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if clientStats.signalHistory && clientStats.signalHistory.length > 0}
      <div class="section">
        <h4>Signal History</h4>
        <div class="history-stats">
          <div class="history-item">
            <span class="label">Data Points</span>
            <span class="value">{clientStats.signalHistory.length}</span>
          </div>
          <div class="history-item">
            <span class="label">Roaming Events</span>
            <span class="value">{(clientStats.roamingHistory || []).length}</span>
          </div>
        </div>
      </div>
    {/if}

  {:else}
    <div class="not-connected">
      <div class="not-connected-icon">📡</div>
      <p>Not connected to any WiFi network</p>
      <p class="hint">Start scanning and connect to see detailed statistics</p>
    </div>
  {/if}
</div>

<style>
  .client-stats-panel {
    height: 100%;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }

  .panel-header {
    padding: 16px;
    background: #2a2a2a;
    border-bottom: 1px solid #333;
  }

  .panel-header h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: #e0e0e0;
  }

  .section {
    padding: 16px;
    border-bottom: 1px solid #333;
  }

  .section h4 {
    margin: 0 0 12px 0;
    font-size: 14px;
    font-weight: 600;
    color: #aaa;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .info-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .info-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .info-item.full-width {
    grid-column: 1 / -1;
  }

  .label {
    font-size: 12px;
    color: #888;
    font-weight: 500;
  }

  .value {
    font-size: 14px;
    color: #e0e0e0;
    font-weight: 500;
  }

  .value.ssid {
    font-weight: 600;
    color: #66b3ff;
  }

  .value.bssid {
    font-family: monospace;
    font-size: 12px;
    color: #aaa;
  }

  .value.rate {
    font-weight: 600;
    color: #4caf50;
  }

  /* Signal quality indicators */
  .signal-good {
    color: #4caf50;
    font-weight: 600;
  }

  .signal-medium {
    color: #ff9800;
    font-weight: 600;
  }

  .signal-poor {
    color: #f44336;
    font-weight: 600;
  }

  .quality-badge {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 3px;
    font-size: 11px;
    font-weight: 600;
    color: white;
    margin-left: 8px;
  }

  /* SNR Bar */
  .snr-bar {
    width: 100%;
    height: 4px;
    background: #333;
    border-radius: 2px;
    margin-top: 4px;
    overflow: hidden;
  }

  .snr-fill {
    height: 100%;
    background: linear-gradient(90deg, #f44336, #ff9800, #4caf50);
    transition: width 0.3s ease;
  }

  /* Retry Rate */
  .rate-good {
    color: #4caf50;
  }

  .rate-medium {
    color: #ff9800;
  }

  .rate-poor {
    color: #f44336;
  }

  .retry-bar {
    width: 100%;
    height: 4px;
    background: #333;
    border-radius: 2px;
    margin-top: 4px;
    overflow: hidden;
  }

  .retry-fill {
    height: 100%;
    transition: width 0.3s ease;
  }

  .retry-fill.rate-good {
    background: #4caf50;
  }

  .retry-fill.rate-medium {
    background: #ff9800;
  }

  .retry-fill.rate-poor {
    background: #f44336;
  }

  /* Roaming History */
  .roaming-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .roaming-event {
    background: #1a1a1a;
    border: 1px solid #333;
    border-radius: 4px;
    padding: 12px;
  }

  .roaming-time {
    font-size: 12px;
    color: #888;
    margin-bottom: 6px;
  }

  .roaming-details {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .roaming-path {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
  }

  .bssid-from,
  .bssid-to {
    font-family: monospace;
    font-weight: 500;
  }

  .bssid-from {
    color: #ff9800;
  }

  .bssid-to {
    color: #4caf50;
  }

  .arrow {
    color: #888;
  }

  .roaming-signals {
    display: flex;
    gap: 12px;
    font-size: 12px;
    color: #aaa;
  }

  .signal-change {
    color: #66b3ff;
  }

  .channel-change {
    color: #8bc34a;
  }

  /* History Stats */
  .history-stats {
    display: flex;
    gap: 16px;
  }

  .history-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .history-item .value {
    font-size: 16px;
    font-weight: 600;
    color: #0066cc;
  }

  /* Not Connected State */
  .not-connected {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px 20px;
    text-align: center;
    color: #888;
  }

  .not-connected-icon {
    font-size: 48px;
    margin-bottom: 16px;
    opacity: 0.5;
  }

  .not-connected p {
    margin: 4px 0;
    font-size: 14px;
  }

  .not-connected .hint {
    font-size: 12px;
    color: #666;
    margin-top: 8px;
  }

  /* Responsive adjustments */
  @media (max-width: 1200px) {
    .info-grid {
      grid-template-columns: 1fr;
    }

    .section {
      padding: 12px;
    }
  }
</style>
