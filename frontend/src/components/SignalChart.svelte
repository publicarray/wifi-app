<script>
  import { onMount, onDestroy } from 'svelte'
  import { Chart, registerables } from 'chart.js'
  import 'chartjs-adapter-date-fns'
  
  export let clientStats = null

  let chartElement
  let chart = null

  onMount(() => {
    Chart.register(...registerables)
    initializeChart()
  })

  onDestroy(() => {
    if (chart) {
      chart.destroy()
    }
  })

  function initializeChart() {
    const ctx = chartElement.getContext('2d')
    
    chart = new Chart(ctx, {
      type: 'line',
      data: {
        datasets: []
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          mode: 'index',
          intersect: false,
        },
        plugins: {
          title: {
            display: true,
            text: 'Signal Strength Over Time',
            color: '#e0e0e0',
            font: {
              size: 16,
              weight: '600'
            }
          },
          legend: {
            display: true,
            position: 'top',
            labels: {
              color: '#e0e0e0',
              usePointStyle: true,
              padding: 20
            }
          },
          tooltip: {
            backgroundColor: 'rgba(42, 42, 42, 0.9)',
            titleColor: '#e0e0e0',
            bodyColor: '#e0e0e0',
            borderColor: '#333',
            borderWidth: 1,
            padding: 12,
            displayColors: true,
            callbacks: {
              title: function(context) {
                return new Date(context[0].parsed.x).toLocaleTimeString()
              },
              label: function(context) {
                return `${context.dataset.label}: ${context.parsed.y} dBm`
              }
            }
          }
        },
        scales: {
          x: {
            type: 'time',
            time: {
              unit: 'minute',
              displayFormats: {
                minute: 'HH:mm'
              }
            },
            title: {
              display: true,
              text: 'Time',
              color: '#aaa'
            },
            ticks: {
              color: '#aaa'
            },
            grid: {
              color: '#333',
              borderColor: '#444'
            }
          },
          y: {
            title: {
              display: true,
              text: 'Signal Strength (dBm)',
              color: '#aaa'
            },
            ticks: {
              color: '#aaa',
              callback: function(value) {
                return value + ' dBm'
              }
            },
            grid: {
              color: '#333',
              borderColor: '#444'
            },
            min: -100,
            max: -30,
            reverse: false // Higher values (less negative) are better signals
          }
        }
      }
    })
  }

  // Update chart when clientStats changes
  $: if (chart && clientStats && clientStats.signalHistory) {
    updateChart()
  }

  function updateChart() {
    if (!clientStats || !clientStats.signalHistory || clientStats.signalHistory.length === 0) {
      chart.data.datasets = []
      chart.update()
      return
    }

    // Group signal data by BSSID to show multiple APs
    const signalDataByBSSID = {}
    
    clientStats.signalHistory.forEach(point => {
      const bssid = point.bssid || 'Unknown'
      if (!signalDataByBSSID[bssid]) {
        signalDataByBSSID[bssid] = []
      }
      signalDataByBSSID[bssid].push({
        x: point.timestamp,
        y: point.signal
      })
    })

    // Create datasets for each BSSID
    const datasets = []
    const colors = [
      '#0066cc', '#4caf50', '#ff9800', '#f44336', '#9c27b0',
      '#00bcd4', '#8bc34a', '#ffc107', '#795548', '#607d8b'
    ]
    let colorIndex = 0

    Object.entries(signalDataByBSSID).forEach(([bssid, data]) => {
      // Find the corresponding AP to get SSID
      let label = bssid
      if (clientStats.bssid === bssid) {
        label = `${clientStats.ssid || 'Connected'} (${bssid})`
      }

      datasets.push({
        label: label,
        data: data,
        borderColor: colors[colorIndex % colors.length],
        backgroundColor: colors[colorIndex % colors.length] + '20',
        borderWidth: 2,
        pointRadius: 3,
        pointHoverRadius: 5,
        tension: 0.1,
        fill: false
      })
      colorIndex++
    })

    // Add roaming events as vertical lines
    if (clientStats.roamingHistory && clientStats.roamingHistory.length > 0) {
      clientStats.roamingHistory.forEach(roamEvent => {
        datasets.push({
          label: `Roaming: ${roamEvent.previousBSSID.slice(-6)} → ${roamEvent.newBSSID.slice(-6)}`,
          data: [
            { x: roamEvent.timestamp, y: -100 },
            { x: roamEvent.timestamp, y: -30 }
          ],
          borderColor: '#ff5722',
          borderWidth: 2,
          borderDash: [5, 5],
          pointRadius: 0,
          fill: false,
          showLine: true
        })
      })
    }

    chart.data.datasets = datasets
    chart.update()
  }

  function getSignalQuality(signal) {
    if (signal > -60) return { text: 'Excellent', color: '#4caf50' }
    if (signal > -70) return { text: 'Good', color: '#8bc34a' }
    if (signal > -80) return { text: 'Fair', color: '#ff9800' }
    return { text: 'Poor', color: '#f44336' }
  }
</script>

<div class="signal-chart-container">
  {#if clientStats && clientStats.connected}
    <div class="chart-header">
      <div class="connection-info">
        <h3>Connected: {clientStats.ssid}</h3>
        <div class="signal-summary">
          <div class="current-signal">
            <span class="signal-label">Current:</span>
            <span class="signal-value" class:signal-good={clientStats.signal > -60} 
                  class:signal-medium={clientStats.signal > -75 && clientStats.signal <= -60}
                  class:signal-poor={clientStats.signal <= -75}>
              {clientStats.signal} dBm
            </span>
            <span class="signal-quality" style="color: {getSignalQuality(clientStats.signal).color}">
              ({getSignalQuality(clientStats.signal).text})
            </span>
          </div>
          <div class="signal-stats">
            <span>Avg: {clientStats.signalAvg || clientStats.signal} dBm</span>
            <span>SNR: {clientStats.snr} dB</span>
          </div>
        </div>
      </div>
    </div>
  {:else}
    <div class="chart-header">
      <h3>Signal Strength</h3>
      <p class="no-connection">Not connected to any WiFi network</p>
    </div>
  {/if}

  <div class="chart-wrapper">
    <canvas bind:this={chartElement}></canvas>
  </div>

  {#if clientStats && clientStats.signalHistory && clientStats.signalHistory.length > 0}
    <div class="chart-footer">
      <div class="history-info">
        <span>History: {clientStats.signalHistory.length} data points</span>
        {#if clientStats.roamingHistory && clientStats.roamingHistory.length > 0}
          <span>Roaming events: {clientStats.roamingHistory.length}</span>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .signal-chart-container {
    height: 100%;
    display: flex;
    flex-direction: column;
    background: #1a1a1a;
    padding: 16px;
  }

  .chart-header {
    margin-bottom: 16px;
  }

  .chart-header h3 {
    margin: 0 0 8px 0;
    font-size: 18px;
    font-weight: 600;
    color: #e0e0e0;
  }

  .connection-info {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .signal-summary {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .current-signal {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .signal-label {
    color: #aaa;
    font-size: 14px;
  }

  .signal-value {
    font-weight: 600;
    font-size: 16px;
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

  .signal-quality {
    font-size: 14px;
    font-weight: 500;
  }

  .signal-stats {
    display: flex;
    gap: 16px;
    font-size: 13px;
    color: #aaa;
  }

  .no-connection {
    color: #888;
    font-size: 14px;
    margin: 0;
  }

  .chart-wrapper {
    flex: 1;
    position: relative;
    min-height: 200px;
  }

  .chart-footer {
    margin-top: 12px;
    padding-top: 8px;
    border-top: 1px solid #333;
  }

  .history-info {
    display: flex;
    gap: 16px;
    font-size: 12px;
    color: #888;
  }

  /* Responsive adjustments */
  @media (max-width: 768px) {
    .signal-chart-container {
      padding: 12px;
    }

    .chart-header h3 {
      font-size: 16px;
    }

    .current-signal {
      flex-direction: column;
      align-items: flex-start;
      gap: 4px;
    }

    .signal-stats {
      flex-direction: column;
      gap: 2px;
    }

    .history-info {
      flex-direction: column;
      gap: 2px;
    }
  }
</style>