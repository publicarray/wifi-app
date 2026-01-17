<script>
  import { onMount, onDestroy } from 'svelte'
  import { GetAvailableInterfaces, StartScanning, StopScanning, GetNetworks, GetClientStats, GetChannelAnalysis, IsScanning, GetRoamingAnalysis, GetAPPlacementRecommendations } from '../wailsjs/go/main/App.js'
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime.js'
  
   import NetworkList from './components/NetworkList.svelte'
   import SignalChart from './components/SignalChart.svelte'
   import ChannelAnalyzer from './components/ChannelAnalyzer.svelte'
   import ClientStatsPanel from './components/ClientStatsPanel.svelte'
   import RoamingAnalysis from './components/RoamingAnalysis.svelte'
   import Toolbar from './components/Toolbar.svelte'

  let interfaces = []
  let selectedInterface = ''
  let scanning = false
  let networks = []
  let clientStats = null
  let channelAnalysis = []
  let errorMessage = ''
  let activeTab = 'networks'
  let roamingMetrics = null
  let placementRecommendations = []

  onMount(async () => {
    try {
      interfaces = await GetAvailableInterfaces()
      if (interfaces.length > 0) {
        selectedInterface = interfaces[0]
      }
    } catch (err) {
      errorMessage = 'Failed to get WiFi interfaces: ' + err
    }

    EventsOn('networks:updated', (data) => {
      console.log('Networks updated event received:', data)
      console.log('Networks count:', data ? data.length : 0)
      networks = data || []
    })

    EventsOn('client:updated', (data) => {
      console.log('Client updated event received:', data)
      clientStats = data
    })

    EventsOn('scan:error', (error) => {
      console.error('Scan error event received:', error)
      errorMessage = error
    })

    EventsOn('scan:debug', (message) => {
      console.log('Scan debug:', message)
    })

    EventsOn('scan:status', (status) => {
      console.log('Scan status:', status)
    })

    EventsOn('client:warning', (warning) => {
      console.warn('Client warning:', warning)
    })

    EventsOn('scan:debug', (message) => {
      console.log('Scan debug:', message)
    })

    EventsOn('client:warning', (warning) => {
      console.warn('Client warning:', warning)
    })

    EventsOn('roaming:detected', (event) => {
      console.log('Roaming detected:', event)
    })

    EventsOn('client:warning', (warning) => {
      console.log('Client warning:', warning)
    })
  })

  onDestroy(() => {
    EventsOff('networks:updated')
    EventsOff('client:updated')
    EventsOff('scan:error')
    EventsOff('roaming:detected')
    EventsOff('client:warning')
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

  async function setActiveTab(tab) {
    activeTab = tab
    
    if (tab === 'roaming') {
      await loadRoamingData()
    }
  }

  async function loadRoamingData() {
    try {
      roamingMetrics = await GetRoamingAnalysis()
      placementRecommendations = await GetAPPlacementRecommendations()
    } catch (err) {
      console.error('Failed to load roaming data:', err)
    }
  }

  function getTabIcon(tab) {
    switch(tab) {
      case 'networks': return '📡'
      case 'signal': return '📊'
      case 'channels': return '📈'
      case 'stats': return '📋'
      case 'roaming': return '🔀'
      default: return ''
    }
  }
</script>

<div class="app-container">
  <Toolbar 
    {interfaces} 
    {selectedInterface} 
    {scanning} 
    {errorMessage}
    on:selectInterface={(e) => selectInterface(e.detail)}
    on:startScanning={startScanning}
    on:stopScanning={stopScanning}
  />

  <div class="main-tabs">
    {#each ['networks', 'signal', 'channels', 'stats', 'roaming'] as tab}
      <button 
        class="main-tab" 
        class:active={activeTab === tab}
        on:click={() => setActiveTab(tab)}
      >
        <span class="tab-icon">{getTabIcon(tab)}</span>
        <span class="tab-label">{tab.charAt(0).toUpperCase() + tab.slice(1)}</span>
      </button>
    {/each}
  </div>

  <div class="tab-content">
    {#if activeTab === 'networks'}
      <div class="content-panel">
        <NetworkList {networks} {clientStats} />
      </div>
    {:else if activeTab === 'signal'}
      <div class="content-panel">
        <SignalChart {clientStats} />
      </div>
    {:else if activeTab === 'channels'}
      <div class="content-panel">
        <ChannelAnalyzer {networks} />
      </div>
    {:else if activeTab === 'stats'}
      <div class="content-panel stats-panel">
        <ClientStatsPanel {clientStats} />
      </div>
    {:else if activeTab === 'roaming'}
      <div class="content-panel">
        <RoamingAnalysis {roamingMetrics} {placementRecommendations} />
      </div>
    {/if}
  </div>
</div>

<style>
  :global(body) {
    margin: 0;
    padding: 0;
    overflow: hidden;
  }

  .app-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background: #1a1a1a;
    color: #e0e0e0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
  }

  .main-tabs {
    display: flex;
    background: #2a2a2a;
    border-bottom: 2px solid #333;
    padding: 0 20px;
  }

  .main-tab {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 16px 12px;
    background: transparent;
    color: #888;
    border: none;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
    transition: all 0.2s ease;
    border-bottom: 3px solid transparent;
  }

  .main-tab:hover {
    background: #333;
    color: #e0e0e0;
  }

  .main-tab.active {
    background: #252525;
    color: #0066cc;
    border-bottom-color: #0066cc;
  }

  .tab-icon {
    font-size: 20px;
  }

  .tab-label {
    font-size: 14px;
    font-weight: 500;
  }

  .tab-content {
    flex: 1;
    overflow: hidden;
  }

  .content-panel {
    height: 100%;
    overflow: hidden;
  }

  .stats-panel {
    overflow-y: auto;
    max-width: 900px;
    margin: 0 auto;
  }

  @media (max-width: 768px) {
    .main-tabs {
      padding: 0 8px;
    }

    .main-tab {
      padding: 12px 8px;
    }

    .tab-icon {
      font-size: 18px;
    }

    .tab-label {
      font-size: 12px;
    }
  }
</style>
