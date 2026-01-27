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

    $: filteredNetworks = filterNetworks(
        networks,
        filterText,
        filterChannel,
        filterSecurity,
        showHidden,
    );
    $: sortedNetworks = sortNetworks(filteredNetworks, sortBy, sortOrder);

    function filterNetworks(
        networksToFilter,
        filterText,
        filterChannel,
        filterSecurity,
        showHidden,
    ) {
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
                Number(network.channel) !== Number(filterChannel)
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

    function sortNetworks(networksToSort, sortBy, sortOrder) {
        return [...networksToSort].sort((a, b) => {
            let aValue, bValue;

            const securityRank = {
                Open: 0,
                WEP: 1,
                WPA: 2,
                WPA2: 3,
                WPA3: 4,
            };

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
                    aValue = securityRank[a.security] ?? 0;
                    bValue = securityRank[b.security] ?? 0;
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
        expandedNetworks = new Set(expandedNetworks);
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

    function getClientCountClass(count) {
        if (count === undefined || count === null || count < 0)
            return "value-neutral";
        if (count <= 10) return "value-good";
        if (count <= 25) return "value-neutral";
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

    function getWiFiStandard(ap) {
        if (!ap || !ap.capabilities) return null;

        const caps = ap.capabilities;

        if (caps.includes("WiFi7") || caps.includes("EHT")) return "WiFi 7";

        // Check for 6GHz band for WiFi 6E
        // Frequency > 5925 MHz is 6GHz band
        if (
            ap.frequency > 5925 &&
            (caps.includes("HE") || caps.includes("WiFi6"))
        ) {
            return "WiFi 6E";
        }

        if (caps.includes("HE") || caps.includes("WiFi6")) return "WiFi 6";
        if (caps.includes("VHT")) return "WiFi 5";
        if (caps.includes("HT")) return "WiFi 4";

        return null;
    }

    function getWiFiStandardClass(standard) {
        if (!standard) return "";
        const base = "wifi-standard-badge";
        switch (standard) {
            case "WiFi 7":
                return `${base} wifi-7`;
            case "WiFi 6E":
                return `${base} wifi-6e`;
            case "WiFi 6":
                return `${base} wifi-6`;
            case "WiFi 5":
                return `${base} wifi-5`;
            case "WiFi 4":
                return `${base} wifi-4`;
            default:
                return base;
        }
    }
</script>

<div class="network-list-container">
    <!-- Filters -->
    <div class="filters">
        <div class="filter-row">
            <input
                type="text"
                placeholder="Filter by SSID..."
                bind:value={filterText}
                on:keydown={(e) => e.key === "Enter" && e.preventDefault()}
                class="filter-input"
                title="Filter networks by SSID name"
            />

            <select
                bind:value={filterChannel}
                class="filter-select"
                title="Filter networks by primary channel"
            >
                <option value="">All Channels</option>
                {#each availableChannels as channel}
                    <option value={channel}>Channel {channel}</option>
                {/each}
            </select>

            <select
                bind:value={filterSecurity}
                class="filter-select"
                title="Filter networks by security type"
            >
                <option value="">All Security</option>
                {#each availableSecurityTypes as security}
                    <option value={security}>{security}</option>
                {/each}
            </select>

            <label
                class="checkbox-label"
                title="Include networks with hidden SSIDs"
            >
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
                    <th
                        class="sortable"
                        on:click={() => toggleSort("ssid")}
                        title="Service Set Identifier (Network Name)

Network name broadcast by APs.
• Maximum 32 characters
• Case sensitive
• Hidden networks may not broadcast SSID
• SSID clustering can cause roaming issues
• Special characters may cause client compatibility issues"
                    >
                        SSID
                        {#if sortBy === "ssid"}
                            <span class="sort-indicator"
                                >{sortOrder === "asc" ? "↑" : "↓"}</span
                            >
                        {/if}
                    </th>
                    <th
                        class="sortable"
                        on:click={() => toggleSort("apCount")}
                        title="Number of Access Points in this network

Count of APs broadcasting the same SSID.
• Multiple APs enable roaming and coverage
• More than 3-4 APs may indicate poor channel planning
• APs should have coordinated channel assignments
• Check for duplicate BSSIDs
• High AP count with low signal may indicate coverage gaps"
                    >
                        APs
                        {#if sortBy === "apCount"}
                            <span class="sort-indicator"
                                >{sortOrder === "asc" ? "↑" : "↓"}</span
                            >
                        {/if}
                    </th>
                    <th
                        class="sortable"
                        on:click={() => toggleSort("channel")}
                        title="Primary channel number

RF channel used by the AP.
• 2.4GHz: Channels 1-11 (US), 1-13 (EU), 1-14 (JP)
• 5GHz: Channels 36-165, non-overlapping 20MHz spacing
• 6GHz: Channels 1-233, all 20MHz non-overlapping
• Channel overlap causes interference in 2.4GHz band
• DFS channels (52-144, 100-140) require radar detection
• Check for proper channel planning in multi-AP environments"
                    >
                        Channel
                        {#if sortBy === "channel"}
                            <span class="sort-indicator"
                                >{sortOrder === "asc" ? "↑" : "↓"}</span
                            >
                        {/if}
                    </th>
                    <th
                        class="sortable"
                        on:click={() => toggleSort("signal")}
                        title="Signal strength (dBm). Closer to 0 is better.

Received signal strength indicator.
• Excellent: > -50 dBm (near field)
• Good: -50 to -65 dBm (optimal range)
• Fair: -65 to -75 dBm (usable range)
• Poor: < -75 dBm (connection issues)
• Minimum viable: -85 to -90 dBm
• SNR (Signal-to-Noise Ratio) more important than absolute signal
• Check for signal fluctuations (interference sources)"
                    >
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
                        title="Security protocol (e.g., WPA2, WPA3)

Authentication and encryption standard.
• Open: No security (vulnerable)
• WEP: Broken encryption (legacy, insecure)
• WPA: TKIP encryption (deprecated, weak)
• WPA2: CCMP/AES encryption (current standard)
• WPA3: SAE encryption (enhanced security)
• WPA2/WPA3 Mixed: Backwards compatibility mode
• Check for deprecated protocols in enterprise environments
• PMF (Protected Management Frames) adds deauth protection"
                    >
                        Security
                        {#if sortBy === "security"}
                            <span class="sort-indicator"
                                >{sortOrder === "asc" ? "↑" : "↓"}</span
                            >
                        {/if}
                    </th>
                    <th
                        title="Connection status or detected issues

Network health and connection state.
• Connected: Currently associated with this network
• OK: Network available, no issues detected
• Issues: Problems detected (click to expand details)
• Issues may include: weak signal, channel overlap, security problems
• Check expanded AP details for specific problem indicators
• Status reflects real-time analysis of network conditions
• Issues may trigger client connectivity or performance problems">Status</th
                    >
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
                            on:keypress={() => toggleNetwork(network.ssid)}
                        >
                            <div class="ssid-content">
                                <span class="ssid-text">{network.ssid}</span>
                                {#if network.accessPoints && network.accessPoints.length > 0}
                                    {@const standard = getWiFiStandard(
                                        network.accessPoints[0],
                                    )}
                                    {#if standard}
                                        <span
                                            class={getWiFiStandardClass(
                                                standard,
                                            )}>{standard}</span
                                        >
                                    {/if}
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
                                                <span
                                                    class="ap-bssid"
                                                    title="BSSID (MAC Address)"
                                                    >{ap.bssid}</span
                                                >
                                                <span
                                                    class="ap-band"
                                                    title="Frequency Band"
                                                    >{ap.band}</span
                                                >
                                            </div>
                                            <div class="ap-metrics">
                                                <div class="ap-metric">
                                                    <span
                                                        class="metric-label"
                                                        title="Signal Strength
Closer to 0 = stronger signal
&lt;-50: Excellent
-50 to -65: Good
&gt;-70: Poor">Signal:</span
                                                    >
                                                    <span class="metric-value">
                                                        <span
                                                            class={getSignalClass(
                                                                ap.signal,
                                                            )}
                                                            >{ap.signal} dBm</span
                                                        >
                                                    </span>
                                                </div>
                                                <div class="ap-metric">
                                                    <span
                                                        class="metric-label"
                                                        title="Channel {ap.channel}
• {ap.channelWidth}MHz Width
• Wider channels increase speed and interference">Channel:</span
                                                    >
                                                    <span class="metric-value">
                                                        <span
                                                            >{ap.channel} ({ap.channelWidth}MHz){#if ap.dfs}
                                                                <span
                                                                    class="dfs-badge"
                                                                    >DFS</span
                                                                >{/if}</span
                                                        >
                                                    </span>
                                                </div>
                                                <div class="ap-metric">
                                                    <span
                                                        class="metric-label"
                                                        title="Transmit power in dBm
Higher = better range but more interference">TX Power:</span
                                                    >
                                                    <span class="metric-value">
                                                        <span
                                                            >{ap.txPower} dBm</span
                                                        >
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
                                                            title="BSS Transition (802.11v) - Wireless Network Management for enhanced roaming.
• Enables AP to assist client in finding better APs
• Provides neighbor reports and transition guidance
• Reduces scanning time and improves roaming decisions
• Works with 802.11r for optimal fast roaming
• Essential for large enterprise deployments
• Helps prevent sticky client behavior

COMPATIBILITY WARNINGS FOR MSP:
• Requires WNM (Wireless Network Management) support
• Windows 7/8: Partial support, may ignore transition requests
• iOS devices: Good support in iOS 9+, older devices limited
• Android: Mixed support, vendor-dependent implementation
• UniFi 6/7: Supported, but may cause client disconnects on older devices
• MSP Advice: Test thoroughly in lab before production deployment
• Mixed device fleets: Consider separate SSID for devices lacking 802.11v
• Enterprise vs BYOD: Disable in environments with uncontrolled devices

NOT RECOMMENDED FOR:
• Public hotspots with diverse device types
• Healthcare environments with legacy medical equipment
• Industrial settings with specialized wireless devices
• Small offices without IT management resources"
                                                            >BSS Transition
                                                            (802.11v)
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
                                                            title="Fast Roaming (802.11r)

Fast BSS Transition for seamless roaming.
• Reduces roaming time from 100-500ms to 50ms or less
• Essential for voice/video applications and mobile devices
• Uses pre-authentication and key caching
• Works with 802.11v (BSS Transition) for optimal performance

**COMPATIBILITY WARNINGS FOR MSP:**
• Old devices may have issues with connecting, compatibility issues may arise
• Windows 7/8 devices: May experience authentication failures
• Older Android (<6.0): Limited or broken 802.11r support
• Some IoT devices: Complete incompatibility, connection failures
• UniFi 6/7 APs: Enable via &quot;Advanced Settings&quot; - test thoroughly
• Mixed environments: Disable if client devices < 2 years old
• MSP Advice: Only enable in enterprise environments with controlled device fleets
• Legacy device fallback: May require separate SSID for older devices

**NOT RECOMMENDED FOR:**
• Public WiFi networks with unknown device types
• Environments with legacy IoT or industrial equipment
• Small offices with mixed BYOD policies
• Residential deployments without device control"
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
                                                                title="Target Wake Time (WiFi 6)
Advanced power scheduling for WiFi 6/6E/7 devices.
• Allows clients to schedule specific wake times
• Reduces power consumption by 60-80% for IoT devices
• Enables predictable latency for real-time applications
• Critical for battery-powered sensors and mobile devices
• Improves network efficiency with many sleeping clients
• Requires WiFi 6 (802.11ax) or later support
**COMPATIBILITY WARNINGS FOR MSP:**
• Limited client device support: Mostly high-end devices only
• UniFi 6/7 APs: TWT enabled by default on supported firmware
• iPhone 12+: Supports TWT, battery savings noticeable
• Android 11+: Limited support, vendor-specific implementation
• Windows 10/11: Minimal support, mostly experimental drivers
• Legacy devices: No TWT support, may experience scheduling conflicts
• MSP Advice: Enable only in IoT-heavy environments with compatible devices
• Mixed fleets: No negative impact on non-TWT devices
• Enterprise: Consider for sensor networks and smart building deployments
**NOT RECOMMENDED FOR:**
• Environments with predominantly legacy devices
• High-density networks requiring maximum airtime utilization
• Real-time voice networks where latency consistency is critical
• Networks without WiFi 6/6E client penetration > 50%"
                                                                >TWT Support
                                                                (Target Wake
                                                                Time)</span
                                                            >
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
                                                            title="UAPSD (Unscheduled Automatic Power Save Delivery)
Power save mechanism for VoIP and real-time applications.
• Allows clients to sleep and wake for specific traffic delivery
• Reduces power consumption on mobile devices by 15-30%
• Essential for VoIP handsets, tablets, and battery-powered devices
• Requires QoS/WMM support for proper operation
• Can improve voice call quality and battery life
• Critical for enterprise VoWiFi deployments
**COMPATIBILITY WARNINGS FOR MSP:**
• May cause latency issues if not properly configured
• VoIP phones: UAPSD mandatory for battery-powered handsets
• UniFi 6/7: Supported, but requires WMM QoS enabled
• iOS devices: Excellent UAPSD support, minimal issues
• Android: Variable support, vendor-dependent implementation
• Windows: Limited support, may cause VoIP quality degradation
• Legacy devices: Poor UAPSD handling, connection instability
• MSP Advice: Test VoIP devices thoroughly in lab environment
• Enterprise phones: Enable only for certified VoIP endpoints
• Mixed environments: Monitor for voice quality issues **NOT RECOMMENDED FOR:**
• Gaming networks where latency is critical
• High-frequency trading or real-time control systems
• Networks with poor QoS implementation
• Environments with predominantly non-VoIP clients"
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
                                                                title="SNR (Signal-to-Noise Ratio)
Signal quality metric more important than absolute signal.
• Signal strength divided by noise floor
• >25dB: Excellent (high throughput, stable connection)
• 15-25dB: Good (reliable performance, minor packet loss)
• 10-15dB: Fair (usable, may experience performance issues)
• &lt;10dB: Poor (connection instability, high error rate)
• Critical for determining actual connection quality
• High signal with low SNR indicates interference issues
• Use SNR over signal strength for performance assessment"
                                                            >
                                                                SNR
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
                                                                title="Real-world Speed - ~60-70% of theoretical max accounting for overhead"
                                                            >
                                                                Real-world Speed
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
                                                                title="Estimated Range - Free-space estimate. Walls/obstacles reduce actual range"
                                                            >
                                                                Estimated Range
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
                                                    {#if ap.bssLoadUtilization !== undefined && ap.bssLoadUtilization !== null}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                                title="Channel Utilization (BSS Load) - Percentage of airtime occupied by this BSS.
• Measures channel congestion and interference
• <50%: Good (plenty of capacity available)
• 50-80%: Busy (performance may degrade during peak times)
• >80%: Congested (significant throughput reduction)
• High utilization causes latency and packet loss
• Consider channel changes or adding APs for relief
• Critical for capacity planning in dense environments
• Doesn't account for non-WiFi interference sources"
                                                            >
                                                                Channel
                                                                Utilization
                                                            </span>
                                                            <span
                                                                class="value-pill {getUtilizationStatusClass(
                                                                    ap.bssLoadUtilization,
                                                                )}"
                                                            >
                                                                {ap.bssLoadUtilization >=
                                                                0
                                                                    ? ap.bssLoadUtilization +
                                                                      "%"
                                                                    : "N/A"}
                                                            </span>
                                                        </div>
                                                    {/if}
                                                    {#if ap.bssLoadStations !== undefined && ap.bssLoadStations !== null}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                                title="Connected Clients - Active devices associated with this AP.
• Real-time count of connected stations
• Critical for capacity planning and load balancing
• High client count may indicate need for additional APs
• Typical AP capacity: 25-50 active clients
• Enterprise APs can handle 100+ but performance degrades
• Correlates with channel utilization and throughput
• Monitor for sudden changes (rogue client activity)
• Helps identify over-subscribed access points"
                                                            >
                                                                Connected
                                                                Clients
                                                            </span>
                                                            <span
                                                                class="value-pill {getClientCountClass(
                                                                    ap.bssLoadStations,
                                                                )}"
                                                            >
                                                                {ap.bssLoadStations >=
                                                                0
                                                                    ? ap.bssLoadStations +
                                                                      " clients"
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
                                                            title="PMF (802.11w)
Protects against deauth attacks. Required for WPA3"
                                                        >
                                                            PMF (Protected
                                                            Management Frames)
                                                            802.11w
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
                                                                title="Ciphers - CCMP/GCMP: Good | TKIP: Weak | WEP: Broken"
                                                            >
                                                                Encryption
                                                                Ciphers
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
                                                    {#if ap.authMethods && ap.authMethods.length > 0}
                                                        <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                                title="Auth Methods - SAE: WPA3 | PSK: WPA2 | 802.1X: Enterprise"
                                                            >
                                                                Auth Methods
                                                            </span>
                                                            <span
                                                                class="value-pill {getAuthStatusClass(
                                                                    ap.authMethods,
                                                                )}"
                                                            >
                                                                {ap.authMethods.join(
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
                                                                title="WPS (WiFi Protected Setup) - Simplified connection method with security vulnerabilities.
• Allows connection via PIN or push-button
• Vulnerable to brute force attacks (PIN method)
• Historically compromised (WPS flaw discovered 2011)
• Enterprise environments should disable WPS
• Home use acceptable but monitor for suspicious activity
• Can be exploited for unauthorized network access
• Disabling improves overall security posture
• Consider alternative secure provisioning methods"
                                                            >
                                                                WPS
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
                                                                title="BSS (Basic Service Set) Color is a unique identifier (0-63) for spatial reuse.
It is used to distinguish between different BSSs in the same frequency band.
This helps in managing multiple access points in the same frequency band without interference."
                                                            >
                                                                BSS Color
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
                                                                title="OBSS PD (Overlapping BSS Packet Detect) - WiFi 6 spatial reuse for dense environments.
• Allows APs to transmit on channels used by neighboring networks
• Improves spectrum efficiency in crowded WiFi environments
• Requires signal strength assessment before transmitting
• Critical for dense deployments (apartments, offices, stadiums)
• Can increase network capacity by 20-30% in busy areas
• WiFi 6/6E feature for better coexistence
• Helps mitigate interference in high-density deployments

COMPATIBILITY WARNINGS FOR MSP:
• Only WiFi 6/6E devices support OBSS PD spatial reuse
• Legacy WiFi 5/4 devices don't benefit from this feature
• Mixed environments may see limited improvement
• UniFi 7 WAP implements OBSS PD differently than competitors

NOT RECOMMENDED FOR:
• Networks with mostly legacy devices (WiFi 5 or older)
• Sparse deployments with minimal interference
• Environments where all devices support WiFi 6/6E
• Simple setups where complexity outweighs benefits

UNIFI 7 CONSIDERATIONS:
• UniFi 7 WAP has aggressive OBSS PD implementation
• Can cause issues with non-UniFi neighboring networks
• Enable only in truly dense multi-AP environments
• Monitor for client connectivity issues after enabling"
                                                            >
                                                                OBSS PD (Spatial
                                                                Reuse)
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
                                                                title="Maximum Quadrature Amplitude Modulation (QAM) supported by the access point.
Highest modulation scheme supported by the AP.
• 256-QAM: WiFi 5 (ac) - 8 bits per symbol
• 1024-QAM: WiFi 6 (ax) - 10 bits per symbol
• 4096-QAM: WiFi 7 (be) - 12 bits per symbol
• Higher QAM = higher data rates but requires better signal
• Automatic modulation adaptation based on signal quality
• Critical for determining maximum throughput capability
• Real-world speeds depend on signal conditions and interference
• Higher QAM more susceptible to noise and interference"
                                                            >
                                                                Max QAM
                                                                Modulation
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
                                                                title="MU-MIMO (Multi-User MIMO) - Simultaneous transmission to multiple clients.
• Allows AP to communicate with multiple devices simultaneously
• Increases total network capacity by 2-4x in ideal conditions
• Requires MU-MIMO support on both AP and client devices
• Downlink MU-MIMO more common than uplink
• Critical for dense environments with many active clients
• WiFi 5 (ac) introduced 4x4 MU-MIMO
• WiFi 6/6E improved efficiency with OFDMA
• Limited benefit with few clients or low traffic

COMPATIBILITY WARNINGS FOR MSP:
• Many client devices have poor MU-MIMO implementation
• iOS devices have limited MU-MIMO support compared to Android
• Older WiFi 5 clients may not benefit significantly
• Spatial streams limited by device antenna configurations
• Mixed environments see reduced benefits

NOT RECOMMENDED FOR:
• Networks with mostly single-stream devices (phones, tablets)
• Low-density deployments with few concurrent clients
• Environments with many legacy WiFi 4/5 devices
• Simple setups where complexity outweighs benefits

UNIFI 7 CONSIDERATIONS:
• UniFi 7 WAP has excellent 4x4 MU-MIMO implementation
• Works best with UniFi 6/7 Pro clients and access points
• Enable MU-MIMO only in high-density multi-client environments
• Monitor client device capabilities for optimal MU-MIMO usage"
                                                            >
                                                                MU-MIMO
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
                                                                title="Neighbor Report (802.11k) provides information about nearby access points and their capabilities.
This feature is useful for roaming clients to find the best access point to connect to."
                                                            >
                                                                Neighbor Report
                                                                (802.11k)
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
                                                                title="WMM (Wi-Fi Multimedia) - Traffic prioritization for voice/video applications.
• Prioritizes voice/video over data traffic
• Required for 802.11e QoS compliance
• Four access categories: Voice, Video, Best Effort, Background
• Essential for VoIP and video streaming quality
• Most modern devices support WMM by default
• Can improve performance in congested networks
• Standard feature in all WiFi 5/6/6E devices

COMPATIBILITY WARNINGS FOR MSP:
• Some legacy devices may have broken WMM implementations
• Misconfigured QoS can cause network performance issues
• WMM conflicts can lead to connection drops
• Not all applications properly utilize QoS markings
• Over-reliance on QoS can mask underlying network issues

NOT RECOMMENDED FOR:
• Networks with no real-time applications (voice/video)
• Environments with many legacy devices
• Simple setups where traffic prioritization adds complexity
• Networks where all traffic has equal priority

UNIFI 7 CONSIDERATIONS:
• UniFi 7 WAP has advanced QoS with automatic traffic classification
• Can prioritize UniFi Voice and Video products automatically
• Enable WMM only when using real-time applications
• Monitor QoS metrics in UniFi Network application
• Consider disabling if no VoIP/video applications are present"
                                                            >
                                                                QoS (WMM)
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
                                                                title="Regulatory countrydomain for TX power and channels"
                                                            >
                                                                Country Code
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
                                                                title="Admin-configured identifier"
                                                            >
                                                                AP Name
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
                                                            title="Beacon interval. Lower = better for power save"
                                                        >
                                                            DTIM Interval
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
                                                                title=" MIMO Spatial streams. More = higher throughput"
                                                            >
                                                                MIMO Streams
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
                                                                title="The maximum theoretical speed of the network in Mbps. PHY rate. Real-world is ~60-70% of this"
                                                            >
                                                                Max Theoretical
                                                                Speed
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

    .wifi-standard-badge {
        display: inline-block;
        padding: 1px 5px;
        border-radius: 3px;
        font-size: 9px;
        font-weight: 600;
        margin-left: 6px;
        vertical-align: middle;
        border: 1px solid rgba(255, 255, 255, 0.1);
    }

    .wifi-7 {
        background: rgba(156, 39, 176, 0.2);
        color: #e040fb;
        border-color: rgba(156, 39, 176, 0.4);
    }

    .wifi-6e {
        background: rgba(33, 150, 243, 0.2);
        color: #448aff;
        border-color: rgba(33, 150, 243, 0.4);
    }

    .wifi-6 {
        background: rgba(76, 175, 80, 0.2);
        color: #69f0ae;
        border-color: rgba(76, 175, 80, 0.4);
    }

    .wifi-5 {
        background: rgba(136, 136, 136, 0.2);
        color: #bdbdbd;
        border-color: rgba(136, 136, 136, 0.4);
    }

    .wifi-4 {
        background: rgba(100, 100, 100, 0.2);
        color: #9e9e9e;
        border-color: rgba(100, 100, 100, 0.4);
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
