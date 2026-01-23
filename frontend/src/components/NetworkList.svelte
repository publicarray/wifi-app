<script>
    export let networks = [];
    export let clientStats = null;

    // Type helper functions
    function isConnected(stats) {
        return stats && stats.connected === true;
    }

    function getConnectedSSID(stats) {
        return isConnected(stats) ? stats.ssid : null;
    }

    let expandedNetworks = new Set();
    let sortBy = "signal"; // 'ssid', 'signal', 'channel', 'security'
    let sortOrder = "desc"; // 'asc', 'desc'
    let filterText = "";
    let filterChannel = "";
    let filterSecurity = "";
    let showHidden = false;

    $: filteredNetworks = filterNetworks(networks);
    $: sortedNetworks = sortNetworks(filteredNetworks);

    function filterNetworks(networksToFilter) {
        return networksToFilter.filter((network) => {
            // Text filter
            if (
                filterText !== "" &&
                !network.ssid.toLowerCase().includes(filterText.toLowerCase())
            ) {
                return false;
            }

            // Channel filter
            if (
                filterChannel !== "" &&
                network.channel.toString() !== filterChannel
            ) {
                return false;
            }

            // Security filter
            if (filterSecurity !== "" && network.security !== filterSecurity) {
                return false;
            }

            // Hidden networks filter - only filter if explicitly hiding
            if (showHidden === false && network.ssid === "<Hidden Network>") {
                return false;
            }

            return true;
        });
    }

    function sortNetworks(networksToSort) {
        return [...networksToSort].sort((a, b) => {
            let aValue, bValue;

            switch (sortBy) {
                case "ssid":
                    aValue = a.ssid.toLowerCase();
                    bValue = b.ssid.toLowerCase();
                    break;
                case "signal":
                    aValue = a.bestSignal;
                    bValue = b.bestSignal;
                    break;
                case "channel":
                    aValue = a.channel;
                    bValue = b.channel;
                    break;
                case "security":
                    aValue = a.security;
                    bValue = b.security;
                    break;
                case "apCount":
                    aValue = a.apCount;
                    bValue = b.apCount;
                    break;
                default:
                    return 0;
            }

            let comparison = 0;
            if (aValue > bValue) comparison = 1;
            if (aValue < bValue) comparison = -1;

            return sortOrder === "asc" ? comparison : -comparison;
        });
    }

    function toggleSort(column) {
        if (sortBy === column) {
            sortOrder = sortOrder === "asc" ? "desc" : "asc";
        } else {
            sortBy = column;
            sortOrder = "desc"; // Default to descending for most columns
        }
    }

    function toggleNetwork(ssid) {
        if (expandedNetworks.has(ssid)) {
            expandedNetworks.delete(ssid);
        } else {
            expandedNetworks.add(ssid);
        }
        expandedNetworks = expandedNetworks;
    }

    function getSignalClass(signal) {
        if (signal > -60) return "signal-good";
        if (signal > -75) return "signal-medium";
        return "signal-poor";
    }

    function getSecurityClass(security) {
        if (security === "Open" || security === "WEP") return "security-poor";
        if (security === "WPA2/TKIP") return "security-medium";
        return "security-good";
    }

    function getQamClass(qam) {
        if (!qam) return "";
        return `qam-${qam}`;
    }

    // Value pill color class helper functions
    function getCapabilityStatusClass(isSupported) {
        return isSupported ? "value-good" : "value-bad";
    }

    function getPMFStatusClass(pmfStatus) {
        if (pmfStatus === "Required") return "value-good";
        if (pmfStatus === "Optional") return "value-neutral";
        return "value-bad";
    }

    function getSNRStatusClass(snr) {
        if (snr > 20) return "value-good";
        if (snr > 10) return "value-neutral";
        return "value-bad";
    }

    function getUtilizationStatusClass(utilization) {
        if (utilization < 0) return "value-neutral"; // N/A
        if (utilization < 60) return "value-good";
        if (utilization < 80) return "value-neutral";
        return "value-bad";
    }

    function getCipherStatusClass(ciphers) {
        if (!ciphers || ciphers.length === 0) return "value-neutral";
        for (let c of ciphers) {
            if (c === "TKIP" || c === "WEP") return "value-bad";
        }
        return "value-good";
    }

    function getAuthStatusClass(authMethods) {
        if (!authMethods || authMethods.length === 0) return "value-neutral";
        for (let a of authMethods) {
            if (a.includes("SAE")) return "value-good";
            if (a.includes("PSK")) return "value-neutral";
        }
        if (authMethods.includes("Open")) return "value-bad";
        return "value-neutral";
    }

    // Get unique channels for filter dropdown
    $: availableChannels = [...new Set(networks.map((n) => n.channel))].sort(
        (a, b) => a - b,
    );

    // Get unique security types for filter dropdown
    $: availableSecurityTypes = [...new Set(networks.map((n) => n.security))];
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
                    <th class="sortable" on:click={() => toggleSort("ssid")}>
                        SSID
                        {#if sortBy === "ssid"}
                            <span class="sort-indicator"
                                >{sortOrder === "asc" ? "↑" : "↓"}</span
                            >
                        {/if}
                    </th>
                    <th class="sortable" on:click={() => toggleSort("apCount")}>
                        APs
                        {#if sortBy === "apCount"}
                            <span class="sort-indicator"
                                >{sortOrder === "asc" ? "↑" : "↓"}</span
                            >
                        {/if}
                    </th>
                    <th class="sortable" on:click={() => toggleSort("channel")}>
                        Channel
                        {#if sortBy === "channel"}
                            <span class="sort-indicator"
                                >{sortOrder === "asc" ? "↑" : "↓"}</span
                            >
                        {/if}
                    </th>
                    <th class="sortable" on:click={() => toggleSort("signal")}>
                        Signal
                        {#if sortBy === "signal"}
                            <span class="sort-indicator"
                                >{sortOrder === "asc" ? "↑" : "↓"}</span
                            >
                        {/if}
                    </th>
                    <th
                        class="sortable"
                        on:click={() => toggleSort("security")}
                    >
                        Security
                        {#if sortBy === "security"}
                            <span class="sort-indicator"
                                >{sortOrder === "asc" ? "↑" : "↓"}</span
                            >
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
                        class:connected={isConnected(clientStats) &&
                            getConnectedSSID(clientStats) === network.ssid}
                    >
                        <td
                            class="ssid-cell"
                            on:click={() => toggleNetwork(network.ssid)}
                        >
                            <div class="ssid-content">
                                <span class="ssid-text">{network.ssid}</span>
                                {#if network.accessPoints && network.accessPoints.length > 0}
                                    <span class="vendor-hint"
                                        >{network.accessPoints[0].vendor}</span
                                    >
                                {/if}
                            </div>
                            {#if network.apCount > 1}
                                <div class="expand-indicator">
                                    {expandedNetworks.has(network.ssid)
                                        ? "▼"
                                        : "▶"}
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
                                <span class="status-connected"
                                    >🔗 Connected</span
                                >
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
                                                <span class="ap-bssid"
                                                    >{ap.bssid}</span
                                                >
                                                <span class="ap-band"
                                                    >{ap.band}</span
                                                >
                                            </div>
                                            <div class="ap-metrics">
                                                <div class="ap-metric">
                                                    <span class="metric-label"
                                                        >Signal:</span
                                                    >
                                                    <span
                                                        class="metric-value-with-tooltip"
                                                    >
                                                        <span
                                                            class={getSignalClass(
                                                                ap.signal,
                                                            )}>{ap.signal} dBm</span
                                                        >
                                                        <span class="capability-tooltip"><strong>Signal Strength</strong><br/>Closer to 0 = stronger signal<br/>&lt;-50: Excellent | -50 to -65: Good | &gt;-70: Poor</span>
                                                    </span>
                                                </div>
                                                <div class="ap-metric">
                                                    <span class="metric-label"
                                                        >Channel:</span
                                                    >
                                                    <span
                                                        class="metric-value-with-tooltip"
                                                    >
                                                        <span
                                                            >{ap.channel} ({ap.channelWidth}MHz){#if ap.dfs} <span class="dfs-badge">DFS</span>{/if}</span
                                                        >
                                                    </span>
                                                </div>
                                                <div class="ap-metric">
                                                    <span class="metric-label"
                                                        >TX Power:</span
                                                    >
                                                    <span
                                                        class="metric-value-with-tooltip"
                                                    >
                                                        <span>{ap.txPower} dBm</span
                                                        >
                                                        <span class="capability-tooltip"><strong>Transmit Power</strong><br/>AP broadcast power. Higher = better range but more interference</span>
                                                    </span>
                                                </div>
                                                <div class="ap-metric">
                                                    <span class="metric-label"
                                                        >Vendor:</span
                                                    >
                                                    <span
                                                        class="metric-value-with-tooltip"
                                                    >
                                                        <span>{ap.vendor}</span>
                                                        <span class="capability-tooltip"><strong>Vendor</strong><br/>Identified from MAC address OUI prefix</span>
                                                    </span>
                                                </div>
                                            </div>
                                            <div class="ap-capabilities">
                                                <div class="capability-title">
                                                    Advanced Capabilities
                                                </div>
                                                <div class="capability-grid">
                                                    <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                        >
                                                            BSS Transition
                                                            (802.11v)
                                                            <span class="capability-tooltip"><strong>BSS Transition (802.11v)</strong><br/>AP-assisted roaming for better handoff between APs</span>
                                                        </span>
                                                        <span
                                                            class="value-pill {getCapabilityStatusClass(
                                                                ap.bsstransition,
                                                            )}"
                                                        >
                                                            {ap.bsstransition
                                                                ? "Supported"
                                                                : "Not supported"}
                                                        </span>
                                                    </div>
                                                    <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                        >
                                                            Fast Roaming
                                                            (802.11r)
                                                        </span>
                                                        <span
                                                            class="value-pill {getCapabilityStatusClass(
                                                                ap.fastroaming,
                                                            )}"
                                                        >
                                                            {ap.fastroaming
                                                                ? "Supported"
                                                                : "Not supported"}
                                                        </span>
                                                    </div>
                                                    {#if ap.twtSupport}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                TWT Support
                                                                (Target Wake
                                                                Time)
                                                                <span class="capability-tooltip"><strong>Target Wake Time (WiFi 6)</strong><br/>Scheduled wake times for better battery life on clients</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {getCapabilityStatusClass(
                                                                    ap.twtSupport,
                                                                )}"
                                                            >
                                                                {ap.twtSupport
                                                                    ? "Supported"
                                                                    : "Not supported"}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                        >
                                                            UAPSD (U-APSD)
                                                        </span>
                                                        <span
                                                            class="value-pill {getCapabilityStatusClass(
                                                                ap.uapsd,
                                                            )}"
                                                        >
                                                            {ap.uapsd
                                                                ? "Supported"
                                                                : "Not supported"}
                                                        </span>
                                                    </div>
                                                </div>

                                                <div class="capability-title">
                                                    Performance Metrics
                                                </div>
                                                <div class="capability-grid">
                                                    {#if ap.snr && ap.snr > 0}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                SNR
                                                                <span class="capability-tooltip"><strong>SNR</strong><br/>Signal minus noise. &gt;25dB: Excellent | 15-25: Good | &lt;15: Poor</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {getSNRStatusClass(
                                                                    ap.snr,
                                                                )}"
                                                            >
                                                                {ap.snr} dB
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.realWorldSpeed}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                Real-world Speed
                                                                <span class="capability-tooltip"><strong>Real-world Speed</strong><br/>~60-70% of theoretical max accounting for overhead</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {ap.realWorldSpeed >
                                                                100
                                                                    ? 'value-good'
                                                                    : 'value-neutral'}"
                                                            >
                                                                {ap.realWorldSpeed}
                                                                Mbps
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.estimatedRange}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                Estimated Range
                                                                <span class="capability-tooltip"><strong>Estimated Range</strong><br/>Free-space estimate. Walls/obstacles reduce actual range</span>
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral"
                                                            >
                                                                {Math.round(
                                                                    ap.estimatedRange,
                                                                )} m
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.channelUtilization !== undefined && ap.channelUtilization !== null}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                Channel
                                                                Utilization
                                                                <span class="capability-tooltip"><strong>Channel Utilization</strong><br/>Airtime in use. &lt;50%: Good | 50-80%: Busy | &gt;80%: Congested</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {getUtilizationStatusClass(
                                                                    ap.channelUtilization,
                                                                )}"
                                                            >
                                                                {ap.channelUtilization >=
                                                                0
                                                                    ? ap.channelUtilization +
                                                                      "%"
                                                                    : "N/A"}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                </div>

                                                <div class="capability-title">
                                                    Security Settings
                                                </div>
                                                <div class="capability-grid">
                                                    <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                        >
                                                            PMF (Protected
                                                            Management Frames)
                                                            <span class="capability-tooltip"><strong>PMF (802.11w)</strong><br/>Protects against deauth attacks. Required for WPA3</span>
                                                        </span>
                                                        <span
                                                            class="value-pill {getPMFStatusClass(
                                                                ap.pmf,
                                                            )}"
                                                        >
                                                            {ap.pmf ||
                                                                "Disabled"}
                                                        </span>
                                                    </div>
                                                    {#if ap.securityCiphers && ap.securityCiphers.length > 0}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                Encryption
                                                                Ciphers
                                                                <span class="capability-tooltip"><strong>Ciphers</strong><br/>CCMP/GCMP: Good | TKIP: Weak | WEP: Broken</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {getCipherStatusClass(
                                                                    ap.securityCiphers,
                                                                )}"
                                                            >
                                                                {ap.securityCiphers.join(
                                                                    ", ",
                                                                )}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.securityAuth && ap.securityAuth.length > 0}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                Auth Methods
                                                                <span class="capability-tooltip"><strong>Auth Methods</strong><br/>SAE: WPA3 | PSK: WPA2 | 802.1X: Enterprise</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {getAuthStatusClass(
                                                                    ap.securityAuth,
                                                                )}"
                                                            >
                                                                {ap.securityAuth.join(
                                                                    ", ",
                                                                )}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.wps !== undefined}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                WPS
                                                                <span class="capability-tooltip"><strong>WPS</strong><br/>Easy setup but security risk. Recommend disabled</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {ap.wps
                                                                    ? 'value-bad'
                                                                    : 'value-good'}"
                                                            >
                                                                {ap.wps
                                                                    ? "Enabled"
                                                                    : "Disabled"}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                </div>

                                                <div class="capability-title">
                                                    WiFi 6/7 Features
                                                </div>
                                                <div class="capability-grid">
                                                    {#if ap.bssColor !== undefined && ap.bssColor !== null}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                BSS Color
                                                                <span class="capability-tooltip"><strong>BSS Color</strong><br/>WiFi 6 identifier (0-63) for spatial reuse</span>
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral"
                                                            >
                                                                {ap.bssColor}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.obssPD}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                OBSS PD (Spatial
                                                                Reuse)
                                                                <span class="capability-tooltip"><strong>OBSS PD</strong><br/>WiFi 6 spatial reuse for dense environments</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {getCapabilityStatusClass(
                                                                    ap.obssPD,
                                                                )}"
                                                            >
                                                                {ap.obssPD
                                                                    ? "Supported"
                                                                    : "Not supported"}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.qamSupport}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                Max QAM
                                                                Modulation
                                                                <span class="capability-tooltip"><strong>Max QAM</strong><br/>256: WiFi 5 | 1024: WiFi 6 | 4096: WiFi 7</span>
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral {getQamClass(
                                                                    ap.qamSupport,
                                                                )}"
                                                            >
                                                                {ap.qamSupport}-QAM
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.mumimo}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                MU-MIMO
                                                                <span class="capability-tooltip"><strong>MU-MIMO</strong><br/>Simultaneous transmission to multiple clients</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {getCapabilityStatusClass(
                                                                    ap.mumimo,
                                                                )}"
                                                            >
                                                                {ap.mumimo
                                                                    ? "Supported"
                                                                    : "Not supported"}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.neighborReport !== undefined}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                Neighbor Report
                                                                (802.11k)
                                                                <span class="capability-tooltip"><strong>802.11k</strong><br/>AP shares nearby AP info for smarter roaming</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {getCapabilityStatusClass(
                                                                    ap.neighborReport,
                                                                )}"
                                                            >
                                                                {ap.neighborReport
                                                                    ? "Supported"
                                                                    : "Not supported"}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                </div>

                                                <div class="capability-title">
                                                    Management & QoS
                                                </div>
                                                <div class="capability-grid">
                                                    {#if ap.qosSupport}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                QoS (WMM)
                                                                <span class="capability-tooltip"><strong>WMM</strong><br/>Traffic prioritization for voice/video</span>
                                                            </span>
                                                            <span
                                                                class="value-pill {getCapabilityStatusClass(
                                                                    ap.qosSupport,
                                                                )}"
                                                            >
                                                                {ap.qosSupport
                                                                    ? "Supported"
                                                                    : "Not supported"}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.countryCode}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                Country Code
                                                                <span class="capability-tooltip"><strong>Country Code</strong><br/>Regulatory domain for TX power and channels</span>
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral"
                                                            >
                                                                {ap.countryCode}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.apName}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                AP Name
                                                                <span class="capability-tooltip"><strong>AP Name</strong><br/>Admin-configured identifier</span>
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral"
                                                            >
                                                                {ap.apName}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                </div>

                                                <div class="capability-title">
                                                    Other Settings
                                                </div>
                                                <div class="capability-grid">
                                                    <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                        >
                                                            DTIM Interval
                                                            <span class="capability-tooltip"><strong>DTIM</strong><br/>Beacon interval. Lower = better for power save</span>
                                                        </span>
                                                        <span
                                                            class="value-pill value-neutral"
                                                        >
                                                            {ap.dtim}
                                                        </span>
                                                    </div>
                                                    {#if ap.mimoStreams}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                MIMO Streams
                                                                <span class="capability-tooltip"><strong>MIMO</strong><br/>Spatial streams. More = higher throughput</span>
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral"
                                                            >
                                                                {ap.mimoStreams}×{ap.mimoStreams}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.maxTheoreticalSpeed}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                            >
                                                                Max Theoretical
                                                                Speed
                                                                <span class="capability-tooltip"><strong>Max Speed</strong><br/>PHY rate. Real-world is ~60-70% of this</span>
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral"
                                                            >
                                                                {ap.maxTheoreticalSpeed}
                                                                Mbps
                                                            </span>
                                                        </div>
                                                    {/if}
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
                                            <span class="issue-text"
                                                >{issue}</span
                                            >
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
                        <p>
                            No networks found. Start scanning to discover WiFi
                            networks.
                        </p>
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
        display: flex;
        align-items: center;
        position: relative;
        cursor: help;
    }

    .capability-value {
        font-weight: 500;
    }

    /* Value pills */
    .value-pill {
        display: inline-block;
        padding: 2px 10px;
        border-radius: 12px;
        font-size: 11px;
        font-weight: 500;
        white-space: nowrap;
    }

    .value-good {
        background: rgba(76, 175, 80, 0.2);
        color: #4caf50;
        border: 1px solid rgba(76, 175, 80, 0.4);
    }

    .value-bad {
        background: rgba(244, 67, 54, 0.2);
        color: #f44336;
        border: 1px solid rgba(244, 67, 54, 0.4);
    }

    .value-neutral {
        background: rgba(136, 136, 136, 0.15);
        color: #fff;
        border: 1px solid rgba(136, 136, 136, 0.3);
    }

    .dfs-badge {
        display: inline-block;
        padding: 1px 5px;
        border-radius: 3px;
        font-size: 9px;
        font-weight: 600;
        background: rgba(255, 152, 0, 0.2);
        color: #ff9800;
        border: 1px solid rgba(255, 152, 0, 0.4);
        margin-left: 4px;
        vertical-align: middle;
    }

    .capability-tooltip {
        position: fixed;
        background: rgba(0, 0, 0, 0.95);
        color: #fff;
        padding: 10px 14px;
        border-radius: 6px;
        font-size: 12px;
        max-width: 320px;
        z-index: 1000;
        pointer-events: none;
        opacity: 0;
        transition: opacity 0.2s;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
        left: 50%;
        top: 50%;
        transform: translate(-50%, -100%) translateY(-12px);
        line-height: 1.5;
        user-select: none;
    }

    .technical-section {
        display: block;
        margin-top: 10px;
        padding-top: 10px;
        border-top: 1px solid rgba(136, 136, 136, 0.3);
        font-size: 11px;
        line-height: 1.6;
        color: #ccc;
    }

    .technical-label {
        display: block;
        color: #aaa;
        font-size: 10px;
        font-weight: 600;
        margin-bottom: 6px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .technical-bullet {
        display: block;
        margin-bottom: 3px;
        padding-left: 12px;
        position: relative;
        color: #bbb;
    }

    .technical-bullet:before {
        content: "•";
        position: absolute;
        left: 0;
        color: #66b3ff;
        font-weight: bold;
    }

    .technical-bullet:last-child {
        margin-bottom: 0;
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

    .capability-label:hover .capability-tooltip {
        opacity: 1;
    }

    .metric-value-with-tooltip {
        position: relative;
        cursor: help;
    }

    .metric-value-with-tooltip:hover .capability-tooltip {
        opacity: 1;
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
