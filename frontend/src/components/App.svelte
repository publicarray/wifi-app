<script>
  import { onMount, onDestroy } from 'svelte'
  import { GetAvailableInterfaces, StartScanning, StopScanning, GetNetworks, GetClientStats, GetChannelAnalysis, IsScanning } from '../../wailsjs/go/main/App.js'
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime.js'
  
  // Import child components
  import NetworkList from './NetworkList.svelte'
  import SignalChart from './SignalChart.svelte'
  import ChannelAnalyzer from './ChannelAnalyzer.svelte'
  import ClientStatsPanel from './ClientStatsPanel.svelte'
  import Toolbar from './Toolbar.svelte'

  let interfaces = []
  let selectedInterface = ''
  let scanning = false
  let networks = []
  let clientStats = null
  let channelAnalysis = []
  let errorMessage = ''
  let activeTab = 'signal' // 'signal' or 'channels'

  onMount(async () => {
    try {
      interfaces = await GetAvailableInterfaces()
      if (interfaces.length > 0) {
        selectedInterface = interfaces[0]
      }
    } catch (err) {
      errorMessage = 'Failed to get WiFi interfaces: ' + err
    }

    // Listen for real-time events
    EventsOn('networks:updated', (data) => {
      networks = data || []
    })

    EventsOn('client:updated', (data) => {
      clientStats = data
    })

    EventsOn('scan:error', (error) => {
      errorMessage = error
    })

    EventsOn('roaming:detected', (event) => {
      console.log('Roaming detected:', event)
    })
  })

  onDestroy(() => {
    EventsOff('networks:updated')
    EventsOff('client:updated')
    EventsOff('scan:error')
    EventsOff('roaming:detected')
    if (scanning) {
      stopScanning()
    }
  })

  async function startScanning() {
    try {
      errorMessage = ''
      await StartScanning(selectedInterface)
      scanning = true
    } catch (err) {
      errorMessage = 'Failed to start scanning: ' + err
    }
  }

  async function stopScanning() {
    try {
      await StopScanning()
      scanning = false
    } catch (err) {
      errorMessage = 'Failed to stop scanning: ' + err
    }
  }

  function selectInterface(iface) {
    selectedInterface = iface
  }

  function setActiveTab(tab) {
    activeTab = tab
  }
</script>

<div class="app-container">
  <!-- Toolbar -->
  <Toolbar 
    {interfaces} 
    {selectedInterface} 
    {scanning} 
    {errorMessage}
    on:selectInterface={(e) => selectInterface(e.detail)}
    on:startScanning={startScanning}
    on:stopScanning={stopScanning}
  />

  <!-- Main Content Area -->
  <div class="main-content">
    <!-- Top Pane: Network List (60%) -->
    <div class="top-pane">
      <NetworkList {networks} {clientStats} />
    </div>

    <!-- Bottom Pane: Charts (40%) -->
    <div class="bottom-pane">
      <div class="chart-tabs">
        <button 
          class="tab-button" 
          class:active={activeTab === 'signal'}
          on:click={() => setActiveTab('signal')}
        >
          Signal Strength
        </button>
        <button 
          class="tab-button" 
          class:active={activeTab === 'channels'}
          on:click={() => setActiveTab('channels')}
        >
          Channel Analysis
        </button>
      </div>

      <div class="chart-content">
        {#if activeTab === 'signal'}
          <SignalChart {clientStats} />
        {:else}
          <ChannelAnalyzer {networks} {channelAnalysis} />
        {/if}
      </div>
    </div>
  </div>

  <!-- Side Panel: Client Stats (fixed width) -->
  <div class="side-panel">
    <ClientStatsPanel {clientStats} />
  </div>
</div>

<style>
  .app-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background: #1a1a1a;
    color: #e0e0e0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
  }

  .main-content {
    display: flex;
    flex-direction: column;
    flex: 1;
    overflow: hidden;
  }

  .top-pane {
    flex: 0.6;
    overflow: hidden;
    border-bottom: 1px solid #333;
  }

  .bottom-pane {
    flex: 0.4;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .chart-tabs {
    display: flex;
    background: #2a2a2a;
    border-bottom: 1px solid #333;
  }

  .tab-button {
    flex: 1;
    padding: 12px;
    background: transparent;
    color: #888;
    border: none;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
    transition: all 0.2s ease;
  }

  .tab-button:hover {
    background: #333;
    color: #e0e0e0;
  }

  .tab-button.active {
    background: #0066cc;
    color: white;
  }

  .chart-content {
    flex: 1;
    overflow: hidden;
    background: #1a1a1a;
  }

  .side-panel {
    width: 320px;
    background: #2a2a2a;
    border-left: 1px solid #333;
    overflow-y: auto;
  }

  /* Responsive adjustments */
  @media (max-width: 1200px) {
    .side-panel {
      width: 280px;
    }
  }

  @media (max-width: 768px) {
    .app-container {
      flex-direction: column;
    }
    
    .main-content {
      order: 1;
    }
    
    .side-panel {
      order: 2;
      width: 100%;
      max-height: 200px;
      border-left: none;
      border-top: 1px solid #333;
    }
  }
</style>