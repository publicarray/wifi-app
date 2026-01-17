<script>
  export let roamingMetrics = null
  export let placementRecommendations = []

  function getMetricsClass(value, goodThreshold = true) {
    if (value === false) return 'metric-good'
    if (value === true) return 'metric-bad'
    if (value > 0) return 'metric-good'
    if (value < 0) return 'metric-bad'
    return 'metric-neutral'
  }

  function formatDuration(seconds) {
    if (seconds < 60) return `${seconds}s`
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
  }
</script>

<div class="roaming-analysis-container">
  {#if !roamingMetrics}
    <div class="no-data">
      <span class="no-data-icon">📊</span>
      <p>No roaming data available</p>
      <p class="hint">Connect to a network and wait for roaming events to occur</p>
    </div>
  {:else}
    <div class="section">
      <h3>Roaming Summary</h3>
      <div class="metrics-grid">
        <div class="metric-card">
          <div class="metric-label">Total Roams</div>
          <div class="metric-value">{roamingMetrics.totalRoams || 0}</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">Good Roams</div>
          <div class="metric-value metric-good">{roamingMetrics.goodRoams || 0}</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">Bad Roams</div>
          <div class="metric-value {roamingMetrics.badRoams > 0 ? 'metric-bad' : 'metric-good'}">
            {roamingMetrics.badRoams || 0}
          </div>
        </div>
        <div class="metric-card">
          <div class="metric-label">Avg Signal Change</div>
          <div class="metric-value {getMetricsClass(roamingMetrics.avgSignalChange)}">
            {roamingMetrics.avgSignalChange > 0 ? '+' : ''}{roamingMetrics.avgSignalChange} dBm
          </div>
        </div>
      </div>
    </div>

    <div class="section">
      <h3>Roaming Behavior</h3>
      <div class="behavior-grid">
        <div class="behavior-item">
          <span class="behavior-label">Excessive Roaming</span>
          <span class="behavior-value {roamingMetrics.excessiveRoaming ? 'metric-bad' : 'metric-good'}">
            {roamingMetrics.excessiveRoaming ? '⚠️ Yes' : '✓ No'}
          </span>
        </div>
        <div class="behavior-item">
          <span class="behavior-label">Sticky Client</span>
          <span class="behavior-value {roamingMetrics.stickyClient ? 'metric-bad' : 'metric-good'}">
            {roamingMetrics.stickyClient ? '⚠️ Yes' : '✓ No'}
          </span>
        </div>
        {#if roamingMetrics.timeSinceLastRoam}
          <div class="behavior-item">
            <span class="behavior-label">Time Since Last Roam</span>
            <span class="behavior-value">
              {roamingMetrics.timeSinceLastRoam}
            </span>
          </div>
        {/if}
      </div>
    </div>

    <div class="section">
      <h3>Analysis & Advice</h3>
      <div class="advice-box {roamingMetrics.excessiveRoaming || roamingMetrics.stickyClient ? 'advice-warning' : 'advice-good'}">
        <span class="advice-icon">
          {roamingMetrics.excessiveRoaming || roamingMetrics.stickyClient ? '⚠️' : '✓'}
        </span>
        <span class="advice-text">{roamingMetrics.roamingAdvice}</span>
      </div>
    </div>
  {/if}

  {#if placementRecommendations && placementRecommendations.length > 0}
    <div class="section">
      <h3>AP Placement Recommendations</h3>
      <div class="recommendations-list">
        {#each placementRecommendations as recommendation}
          <div class="recommendation-item">
            <span class="rec-icon">💡</span>
            <span class="rec-text">{recommendation}</span>
          </div>
        {/each}
      </div>
    </div>
  {:else if placementRecommendations && placementRecommendations.length === 0}
    <div class="section">
      <h3>AP Placement Recommendations</h3>
      <div class="no-recommendations">
        <span class="no-rec-icon">✓</span>
        <p>No recommendations at this time</p>
        <p class="hint">Your current AP placement appears optimal</p>
      </div>
    </div>
  {/if}
</div>

<style>
  .roaming-analysis-container {
    height: 100%;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 20px;
    padding: 20px;
  }

  .section {
    background: #2a2a2a;
    border-radius: 6px;
    padding: 16px;
    border: 1px solid #333;
  }

  .section h3 {
    margin: 0 0 16px 0;
    font-size: 16px;
    font-weight: 600;
    color: #e0e0e0;
  }

  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 12px;
  }

  .metric-card {
    background: #1a1a1a;
    padding: 16px;
    border-radius: 4px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    border: 1px solid #333;
  }

  .metric-label {
    font-size: 12px;
    color: #888;
    font-weight: 500;
  }

  .metric-value {
    font-size: 20px;
    font-weight: 600;
    color: #e0e0e0;
  }

  .metric-good {
    color: #4caf50;
  }

  .metric-bad {
    color: #f44336;
  }

  .metric-neutral {
    color: #ff9800;
  }

  .behavior-grid {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .behavior-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px;
    background: #1a1a1a;
    border-radius: 4px;
    border: 1px solid #333;
  }

  .behavior-label {
    font-size: 14px;
    color: #e0e0e0;
    font-weight: 500;
  }

  .behavior-value {
    font-size: 14px;
    font-weight: 600;
  }

  .advice-box {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 16px;
    border-radius: 4px;
  }

  .advice-box.advice-good {
    background: rgba(76, 175, 80, 0.1);
    border: 1px solid #4caf50;
  }

  .advice-box.advice-warning {
    background: rgba(255, 152, 0, 0.1);
    border: 1px solid #ff9800;
  }

  .advice-icon {
    font-size: 20px;
    flex-shrink: 0;
  }

  .advice-text {
    font-size: 14px;
    color: #e0e0e0;
    line-height: 1.5;
  }

  .recommendations-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .recommendation-item {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px;
    background: #1a1a1a;
    border-radius: 4px;
    border-left: 3px solid #ff9800;
  }

  .rec-icon {
    font-size: 16px;
    flex-shrink: 0;
  }

  .rec-text {
    font-size: 14px;
    color: #e0e0e0;
    line-height: 1.4;
  }

  .no-data,
  .no-recommendations {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px 20px;
    text-align: center;
    color: #888;
  }

  .no-data-icon,
  .no-rec-icon {
    font-size: 48px;
    margin-bottom: 16px;
    opacity: 0.5;
  }

  .no-data p,
  .no-recommendations p {
    margin: 4px 0;
    font-size: 14px;
  }

  .hint {
    font-size: 12px;
    color: #666;
    margin-top: 8px;
  }

  @media (max-width: 768px) {
    .roaming-analysis-container {
      padding: 12px;
    }

    .metrics-grid {
      grid-template-columns: 1fr 1fr;
    }

    .metric-value {
      font-size: 18px;
    }
  }
</style>
