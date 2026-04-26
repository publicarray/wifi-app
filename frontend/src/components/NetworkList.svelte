<script context="module">
    // Per-network bestSignal history. Kept at module scope so sparkline trails
    // survive a tab switch — same pattern as SignalChart.svelte. Bounded to
    // SPARK_MAX_POINTS samples per BSSID; entries for BSSIDs that vanish from
    // the scan results are dropped after the next two updates that don't
    // mention them, so the map can't grow unboundedly across long sessions.
    const moduleSignalHistory = new Map(); // key: bssid, value: number[]
    const moduleSeenAt = new Map();        // key: bssid, value: counter
    const SPARK_MAX_POINTS = 13;
    let updateCounter = 0;

    function recordNetworkSamples(networks) {
        updateCounter += 1;
        const seen = new Set();
        for (const n of networks || []) {
            const bssid = n.bestSignalAP || (n.accessPoints && n.accessPoints[0] && n.accessPoints[0].bssid);
            if (!bssid) continue;
            seen.add(bssid);
            const arr = moduleSignalHistory.get(bssid) || [];
            arr.push(n.bestSignal);
            if (arr.length > SPARK_MAX_POINTS) arr.shift();
            moduleSignalHistory.set(bssid, arr);
            moduleSeenAt.set(bssid, updateCounter);
        }
        // Drop entries not seen in the last two updates.
        for (const [bssid, lastSeen] of moduleSeenAt) {
            if (!seen.has(bssid) && updateCounter - lastSeen > 2) {
                moduleSignalHistory.delete(bssid);
                moduleSeenAt.delete(bssid);
            }
        }
    }

    function getSamples(network) {
        const bssid = network && (network.bestSignalAP || (network.accessPoints && network.accessPoints[0] && network.accessPoints[0].bssid));
        return (bssid && moduleSignalHistory.get(bssid)) || [];
    }
</script>

<script>
    export let networks = [];
    export let clientStats = null;

    // Append the latest scan into the per-BSSID history store so the sparkline
    // column has data to draw. Reactive on `networks` so it fires once per
    // backend update tick.
    $: recordNetworkSamples(networks);

    // Derivation helpers used to highlight the currently-connected network in
    // the table. Previously these were referenced in the template without
    // being defined, which caused a runtime ReferenceError on render.
    function isConnected(stats) {
        return !!(stats && stats.connected);
    }

    function getConnectedSSID(stats) {
        return stats && stats.connected ? stats.ssid : "";
    }

    function getConnectedBSSID(stats) {
        return stats && stats.connected ? (stats.bssid || "").toLowerCase() : "";
    }

    // networkKey returns a stable identifier for a network entry — used as
    // the key for the expanded-row Set and for display labels. Hidden
    // networks (empty SSID) share no natural string identifier; we fall back
    // to the strongest-AP BSSID so two different hidden networks don't
    // collapse into a single expandable row.
    function networkKey(network) {
        if (!network) return "";
        if (network.ssid) return network.ssid;
        return network.bestSignalAP || (network.accessPoints && network.accessPoints[0] && network.accessPoints[0].bssid) || "";
    }

    function getUniqueChannels(networks) {
        return [...new Set(networks.map((n) => n.channel))].sort((a, b) => parseInt(a) - parseInt(b));
    }

    function getUniqueSecurityTypes(networks) {
        const map = new Map();
        networks.forEach((n) => {
            if (!map.has(n.security)) map.set(n.security, true);
        });
        return [...map.keys()];
    }

    $: availableChannels = getUniqueChannels(networks);
    $: availableSecurityTypes = getUniqueSecurityTypes(networks);
    let expandedNetworks = new Set();
    let sortBy = "signal"; // 'ssid', 'signal', 'channel', 'security'
    let sortOrder = "desc"; // 'asc', 'desc'
    let filterText = "";
    let filterChannel = "";
    let filterSecurity = "";
    let filterBand = "All";
    let filterSecurityChip = "All";
    let showHidden = true;

    function networkBand(network) {
        const ap = network && network.accessPoints && network.accessPoints[0];
        if (ap && ap.band) return ap.band;
        if (!ap) return "";
        // Fallback: derive from frequency.
        if (ap.frequency >= 5925) return "6GHz";
        if (ap.frequency >= 4900) return "5GHz";
        if (ap.frequency > 0) return "2.4GHz";
        return "";
    }

    function networkChannelWidth(network) {
        const ap = network && network.accessPoints && network.accessPoints[0];
        return ap && ap.channelWidth ? ap.channelWidth : 0;
    }

    function signalQualityLabel(dBm) {
        if (dBm == null) return "";
        if (dBm >= -50) return "Excellent";
        if (dBm >= -60) return "Good";
        if (dBm >= -67) return "Fair";
        if (dBm >= -75) return "Weak";
        return "Poor";
    }

    function sparklinePath(samples, width, height) {
        if (!samples || samples.length < 2) return { d: "", area: "", last: null };
        const min = Math.min(...samples) - 2;
        const max = Math.max(...samples) + 2;
        const range = max - min || 1;
        const step = width / (samples.length - 1);
        const points = samples.map((v, i) => [
            i * step,
            height - ((v - min) / range) * height,
        ]);
        const d = points
            .map((p, i) => `${i === 0 ? "M" : "L"}${p[0].toFixed(1)},${p[1].toFixed(1)}`)
            .join(" ");
        const area = `${d} L${width.toFixed(1)},${height} L0,${height} Z`;
        return { d, area, last: points[points.length - 1] };
    }

    function sparklineColor(dBm) {
        if (dBm > -60) return "var(--ok)";
        if (dBm > -72) return "var(--warn)";
        return "var(--bad)";
    }

    // KPI derivations for the strip above the filters.
    $: connectedClient = clientStats && clientStats.connected ? clientStats : null;
    $: kpiNetworks = (networks || []).filter((n) =>
        showHidden ||
        !(typeof n.ssid !== "string" || n.ssid === "" || n.ssid === "<Hidden Network>"),
    ).length;

    function collectAPs(source) {
        return (source || []).flatMap((network) => network.accessPoints || []);
    }

    function hasAnyAPValue(aps, predicate) {
        return aps.some((ap) => ap && predicate(ap));
    }

    function isNonEmptyString(value) {
        return typeof value === "string" && value.trim().length > 0;
    }

    function isNonEmptyArray(value) {
        return Array.isArray(value) && value.length > 0;
    }

    function isNumberDefined(value) {
        return typeof value === "number" && !Number.isNaN(value);
    }

    $: apList = collectAPs(networks);

    $: capabilityMap = {
        securityCiphers: hasAnyAPValue(apList, (ap) =>
            isNonEmptyArray(ap.securityCiphers),
        ),
        authMethods: hasAnyAPValue(apList, (ap) =>
            isNonEmptyArray(ap.authMethods),
        ),
        pmf: hasAnyAPValue(apList, (ap) => isNonEmptyString(ap.pmf)),
        uapsd: hasAnyAPValue(apList, (ap) => ap.uapsd !== undefined),
        qosSupport: hasAnyAPValue(apList, (ap) => ap.qosSupport !== undefined),
        qamSupport: hasAnyAPValue(apList, (ap) =>
            isNumberDefined(ap.qamSupport),
        ),
        bssColor: hasAnyAPValue(
            apList,
            (ap) => ap.bssColor !== undefined && ap.bssColor !== null,
        ),
        obssPD: hasAnyAPValue(apList, (ap) => ap.obssPD !== undefined),
        countryCode: hasAnyAPValue(apList, (ap) =>
            isNonEmptyString(ap.countryCode),
        ),
        mimoStreams: hasAnyAPValue(
            apList,
            (ap) => isNumberDefined(ap.mimoStreams) && ap.mimoStreams > 0,
        ),
        maxPhyRate: hasAnyAPValue(
            apList,
            (ap) => isNumberDefined(ap.maxPhyRate) && ap.maxPhyRate > 0,
        ),
    };

    $: filteredNetworks = filterNetworks(
        networks,
        filterText,
        filterChannel,
        filterSecurity,
        filterBand,
        filterSecurityChip,
        showHidden,
    );
    $: sortedNetworks = sortNetworks(filteredNetworks, sortBy, sortOrder);

    function filterNetworks(
        networksToFilter,
        filterText,
        filterChannel,
        filterSecurity,
        filterBand,
        filterSecurityChip,
        showHidden,
    ) {
        return networksToFilter.filter((network) => {
            // Text filter — match SSID or any AP's BSSID.
            const ssidValue =
                typeof network.ssid === "string" ? network.ssid : "";
            if (filterText !== "") {
                const q = filterText.toLowerCase();
                const ssidHit = ssidValue.toLowerCase().includes(q);
                const bssidHit = (network.accessPoints || []).some((ap) =>
                    (ap.bssid || "").toLowerCase().includes(q),
                );
                if (!ssidHit && !bssidHit) return false;
            }

            // Channel filter (legacy dropdown)
            if (
                filterChannel !== "" &&
                Number(network.channel) !== Number(filterChannel)
            ) {
                return false;
            }

            // Security filter (legacy dropdown)
            if (filterSecurity !== "" && network.security !== filterSecurity) {
                return false;
            }

            // Band segmented filter
            if (filterBand && filterBand !== "All") {
                if (networkBand(network) !== filterBand) return false;
            }

            // Security chip — coarse buckets that map to the design.
            if (filterSecurityChip && filterSecurityChip !== "All") {
                const sec = (network.security || "").toUpperCase();
                if (filterSecurityChip === "Open" && sec !== "OPEN") return false;
                if (filterSecurityChip === "WPA2" && !sec.includes("WPA2")) return false;
                if (filterSecurityChip === "WPA3" && !sec.includes("WPA3")) return false;
            }

            // Hidden networks filter - only filter if explicitly hiding
            if (
                showHidden === false &&
                (ssidValue === "" || ssidValue === "<Hidden Network>")
            ) {
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
        if (pmfStatus === "Optional") return "value-warn";
        return "value-bad";
    }

    function getSNRStatusClass(snr) {
        if (snr > 20) return "value-good";
        if (snr > 10) return "value-warn";
        return "value-bad";
    }

    function getUtilizationStatusClass(utilization) {
        if (utilization < 0) return "value-neutral"; // N/A
        if (utilization < 60) return "value-good";
        if (utilization < 80) return "value-warn";
        return "value-bad";
    }

    function deriveWiFiGeneration(capabilities) {
        if (!capabilities || capabilities.length === 0) return "Unknown";
        const lower = capabilities.map(c => (c || "").toLowerCase());

        if (lower.some(c => c.includes("wifi7") || c.includes("802.11be") || c.includes("eht"))) {
            return "7";
        }
        if (lower.some(c => c.includes("wifi6") || c.includes("802.11ax") || c.includes("he"))) {
            return "6";
        }
        if (lower.some(c => c.includes("wifi5") || c.includes("802.11ac") || c.includes("vht"))) {
            return "5";
        }
        if (lower.some(c => c.includes("wifi4") || c.includes("802.11n") || c.includes("ht"))) {
            return "4";
        }
        return "Unknown";
    }

    function getDominantWiFiStandard(capabilities, band) {
        if (!capabilities || capabilities.length === 0) return "Unknown";
        const lower = capabilities.map(c => (c || "").toLowerCase());

        if (lower.some(c => c.includes("eht") || c.includes("wifi7"))) {
            return "WiFi 7 (802.11be)";
        }

        const hasHE = lower.some(c => c.includes("he") || c.includes("wifi6") || c.includes("802.11ax"));
        if (hasHE && band === "6GHz") {
            return "WiFi 6E (802.11ax)";
        }
        if (hasHE) {
            return "WiFi 6 (802.11ax)";
        }

        if (lower.some(c => c.includes("vht") || c.includes("wifi5") || c.includes("802.11ac"))) {
            return "WiFi 5 (802.11ac)";
        }

        if (lower.some(c => c.includes("ht") || c.includes("wifi4") || c.includes("802.11n"))) {
            return "WiFi 4 (802.11n)";
        }

        if (lower.some(c => c.includes("legacy") || c.includes("802.11a") || c.includes("802.11b") || c.includes("802.11g"))) {
            return "Legacy (802.11a/b/g)";
        }

        return "Unknown";
    }

    function hasBeamformingSupport(capabilities, muMIMO) {
        if (muMIMO) return true;
        if (!capabilities || capabilities.length === 0) return false;

        const lower = capabilities.map(c => (c || "").toLowerCase());
        if (lower.some(c => c.includes("txbf") || c.includes("beamform"))) {
            return true;
        }

        // VHT and HE standards inherently support beamforming
        if (lower.some(c => c.includes("vht") || c.includes("he") || c.includes("802.11ac") || c.includes("802.11ax") || c.includes("wifi5") || c.includes("wifi6"))) {
            return true;
        }

        return false;
    }

    function formatSecurityDetails(security, ciphers, authMethods) {
        let details = security || "Unknown";

        const cipherList = (ciphers && ciphers.length > 0) ? ciphers.join(", ") : "";
        const authList = (authMethods && authMethods.length > 0) ? authMethods.join(", ") : "";

        if (cipherList || authList) {
            const parts = [];
            if (cipherList) parts.push(cipherList);
            if (authList) parts.push(authList);
            details += ` (${parts.join(", ")})`;
        }

        return details;
    }

    function getClientCountClass(count) {
        if (count === undefined || count === null || count < 0)
            return "value-neutral";
        if (count <= 10) return "value-good";
        if (count <= 25) return "value-warn";
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
            if (a.includes("PSK")) return "value-warn";
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

    // Convert dBm into 0–4 bars for the inline visual signal indicator.
    function signalBarCount(dBm) {
        if (dBm == null) return 0;
        if (dBm >= -55) return 4;
        if (dBm >= -65) return 3;
        if (dBm >= -75) return 2;
        if (dBm >= -85) return 1;
        return 0;
    }

    function signalBarTone(dBm) {
        if (dBm == null) return "";
        if (dBm > -60) return "ok";
        if (dBm > -72) return "warn";
        return "bad";
    }

    function bssidVendorLine(network) {
        const ap = network && network.accessPoints && network.accessPoints[0];
        if (!ap) return "";
        const parts = [ap.bssid];
        if (ap.vendor) parts.push(ap.vendor);
        return parts.join(" · ");
    }

    function networkBandWidth(network) {
        const ap = network && network.accessPoints && network.accessPoints[0];
        const band = networkBand(network);
        const width = networkChannelWidth(network);
        if (!band) return "";
        return width ? `${band} / ${width}` : band;
    }

    function isOpenSecurity(security) {
        return !security || security === "Open" || security === "None";
    }
</script>

<div class="network-list-container">
    <!-- KPI strip -->
    <div class="kpi-strip">
        <div class="kpi-tile">
            <div class="kpi-label">Networks</div>
            <div class="kpi-value mono">{kpiNetworks}</div>
            <div class="kpi-sub mono">visible SSIDs</div>
        </div>
        <div class="kpi-tile">
            <div class="kpi-label">Connected</div>
            <div class="kpi-value kpi-accent" class:kpi-empty={!connectedClient}>
                {connectedClient ? (connectedClient.ssid || "(hidden)") : "—"}
            </div>
            <div class="kpi-sub mono">
                {connectedClient ? connectedClient.bssid : "not connected"}
            </div>
        </div>
        <div class="kpi-tile">
            <div class="kpi-label">Signal</div>
            <div
                class="kpi-value mono"
                style={connectedClient ? `color: ${sparklineColor(connectedClient.signal)};` : ""}
            >
                {connectedClient ? `${connectedClient.signal} dBm` : "—"}
            </div>
            <div class="kpi-sub mono">
                {connectedClient ? signalQualityLabel(connectedClient.signal) : ""}
            </div>
        </div>
        <div class="kpi-tile">
            <div class="kpi-label">Band / Width</div>
            <div class="kpi-value mono">
                {#if connectedClient}
                    {connectedClient.frequency >= 5925
                        ? "6GHz"
                        : connectedClient.frequency >= 4900
                          ? "5GHz"
                          : "2.4GHz"}
                {:else}
                    —
                {/if}
            </div>
            <div class="kpi-sub mono">
                {connectedClient && connectedClient.channelWidth
                    ? `${connectedClient.channelWidth} MHz · ch ${connectedClient.channel}`
                    : ""}
            </div>
        </div>
        <div class="kpi-tile">
            <div class="kpi-label">Link rate</div>
            <div class="kpi-value mono">
                {connectedClient && connectedClient.txBitrate
                    ? `${connectedClient.txBitrate.toFixed(1)} Mbps`
                    : "—"}
            </div>
            <div class="kpi-sub mono">TX bitrate</div>
        </div>
    </div>

    <!-- Filter bar -->
    <div class="filter-bar">
        <div class="input-group">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" class="search-icon">
                <circle cx="6" cy="6" r="4" stroke="currentColor" stroke-width="1.4"/>
                <path d="M9 9l3 3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            </svg>
            <input
                type="text"
                placeholder="Filter by SSID or BSSID…"
                bind:value={filterText}
                on:keydown={(e) => e.key === "Enter" && e.preventDefault()}
                title="Filter networks by SSID or BSSID"
            />
        </div>

        <div class="segmented" role="tablist">
            {#each ["All", "2.4GHz", "5GHz", "6GHz"] as band}
                <button
                    type="button"
                    class:active={filterBand === band}
                    on:click={() => (filterBand = band)}
                >{band}</button>
            {/each}
        </div>

        <div class="segmented" role="tablist">
            {#each ["All", "WPA3", "WPA2", "Open"] as sec}
                <button
                    type="button"
                    class:active={filterSecurityChip === sec}
                    on:click={() => (filterSecurityChip = sec)}
                >{sec}</button>
            {/each}
        </div>

        <label
            class="hidden-toggle"
            class:on={showHidden}
            title="Include networks with hidden SSIDs"
        >
            <input type="checkbox" bind:checked={showHidden} />
            Hidden
        </label>

        <div class="filter-spacer"></div>
        <span class="count-chip mono">
            {filteredNetworks.length} / {networks.length}
        </span>
    </div>

    <!-- Network Table -->
    <div class="network-table-wrapper">
        <table class="network-table">
            <thead>
                <tr>
                    <th class="chevron-col" aria-hidden="true"></th>
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
                        class="sortable num-col"
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
                        class="band-col"
                        title="Frequency band and channel width

• 2.4GHz: longer range, more interference
• 5GHz: higher capacity, shorter range
• 6GHz: WiFi 6E/7, cleanest spectrum
• Wider channels (40/80/160 MHz) increase throughput but use more spectrum"
                    >Band</th>
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
                        class="history-col"
                        title="Recent signal trend across the last few scans"
                    >History</th>
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
                {#each sortedNetworks as network (networkKey(network))}
                    {@const key = networkKey(network)}
                    {@const isConnectedNetwork = isConnected(clientStats) && ((network.ssid && getConnectedSSID(clientStats) === network.ssid) || (!network.ssid && network.accessPoints && network.accessPoints.some((ap) => (ap.bssid || "").toLowerCase() === getConnectedBSSID(clientStats))))}
                    {@const samples = getSamples(network)}
                    {@const sp = sparklinePath(samples, 80, 18)}
                    {@const trendColor = sparklineColor(network.bestSignal)}
                    {@const isExpanded = expandedNetworks.has(key)}
                    {@const isHidden = !network.ssid || network.ssid === "<Hidden Network>"}
                    {@const standard = network.accessPoints && network.accessPoints[0] ? getWiFiStandard(network.accessPoints[0]) : null}
                    {@const bars = signalBarCount(network.bestSignal)}
                    {@const barTone = signalBarTone(network.bestSignal)}
                    <tr
                        class="network-row"
                        class:has-issues={network.hasIssues}
                        class:connected={isConnectedNetwork}
                        class:expanded={isExpanded}
                        on:click={() => toggleNetwork(key)}
                        on:keypress={(e) => e.key === "Enter" && toggleNetwork(key)}
                    >
                        <td class="chevron-cell" aria-hidden="true">
                            <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
                                {#if isExpanded}
                                    <path d="M2 4l3 3 3-3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
                                {:else}
                                    <path d="M4 2l3 3-3 3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
                                {/if}
                            </svg>
                        </td>
                        <td class="ssid-cell">
                            <div class="ssid-content">
                                <div class="ssid-line">
                                    <span class="ssid-text" class:hidden-ssid={isHidden}>
                                        {network.ssid || "(hidden)"}
                                    </span>
                                    {#if standard}
                                        <span class={getWiFiStandardClass(standard)}>{standard}</span>
                                    {/if}
                                </div>
                                <span class="ssid-sub mono">
                                    {bssidVendorLine(network)}
                                </span>
                            </div>
                        </td>
                        <td class="ap-count-cell num">{network.apCount}</td>
                        <td class="band-cell">
                            <span class="mono band-text">
                                {networkBand(network)}
                                {#if networkChannelWidth(network)}
                                    <span class="band-width">/{networkChannelWidth(network)}</span>
                                {/if}
                            </span>
                        </td>
                        <td class="channel-cell num">{network.channel}</td>
                        <td class="signal-cell">
                            <div class="signal-row">
                                <div class="sig-bar" title="{network.bestSignal} dBm">
                                    {#each [0,1,2,3] as i}
                                        <span
                                            class:on={i < bars}
                                            class:warn={i < bars && barTone === "warn"}
                                            class:bad={i < bars && barTone === "bad"}
                                            style="height: {4 + i * 2.5}px"
                                        ></span>
                                    {/each}
                                </div>
                                <span class="mono signal-value {getSignalClass(network.bestSignal)}">
                                    {network.bestSignal} dBm
                                </span>
                            </div>
                        </td>
                        <td class="history-cell">
                            {#if sp.last}
                                <svg width="80" height="18" class="sparkline" aria-hidden="true">
                                    <path d={sp.area} fill={trendColor} opacity="0.18"/>
                                    <path
                                        d={sp.d}
                                        fill="none"
                                        stroke={trendColor}
                                        stroke-width="1.5"
                                        stroke-linecap="round"
                                        stroke-linejoin="round"
                                    />
                                    <circle
                                        cx={sp.last[0]}
                                        cy={sp.last[1]}
                                        r="1.8"
                                        fill={trendColor}
                                    />
                                </svg>
                            {:else}
                                <span class="history-empty mono">—</span>
                            {/if}
                        </td>
                        <td class="security-cell">
                            {#if isOpenSecurity(network.security)}
                                <span class="chip bad ghost">
                                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                                        <rect x="2.5" y="5.5" width="7" height="5" rx="1" stroke="currentColor" stroke-width="1.2"/>
                                        <path d="M4 5.5V4a2 2 0 014 0v1.5" stroke="currentColor" stroke-width="1.2"/>
                                    </svg>
                                    {network.security || "Open"}
                                </span>
                            {:else}
                                <span class="security-inline mono {getSecurityClass(network.security)}">
                                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" style="opacity: 0.6">
                                        <rect x="2.5" y="5.5" width="7" height="5" rx="1" stroke="currentColor" stroke-width="1.2"/>
                                        <path d="M4 5.5V4a2 2 0 014 0v1.5" stroke="currentColor" stroke-width="1.2"/>
                                    </svg>
                                    {network.security}
                                </span>
                            {/if}
                        </td>
                        <td class="status-cell">
                            {#if isConnectedNetwork}
                                <span class="chip ok">
                                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                                        <path d="M5 7a2 2 0 002.83 0l2-2a2 2 0 00-2.83-2.83l-.5.5M7 5a2 2 0 00-2.83 0l-2 2a2 2 0 002.83 2.83l.5-.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                                    </svg>
                                    Connected
                                </span>
                            {:else if network.hasIssues}
                                <span class="chip warn">⚠ Issues</span>
                            {:else}
                                <span class="status-available">Available</span>
                            {/if}
                        </td>
                    </tr>

                    <!-- Expanded AP Details -->
                    {#if expandedNetworks.has(key)}
                        <tr class="ap-details-row">
                            <td colspan="9">
                                <div class="ap-details">
                                    {#each network.accessPoints as ap}
                                        {@const apStandard = getWiFiStandard(ap)}
                                        {@const apIsConnected = isConnected(clientStats) && (ap.bssid || "").toLowerCase() === getConnectedBSSID(clientStats)}
                                        {@const apSamples = getSamples({ accessPoints: [ap], bestSignalAP: ap.bssid })}
                                        {@const apSp = sparklinePath(apSamples, 160, 36)}
                                        {@const apTrendColor = sparklineColor(ap.signal)}
                                        <div class="ap-card">
                                            <div class="ap-header">
                                                <div class="ap-header-main">
                                                    <div class="ap-header-line">
                                                        <span class="ap-ssid">{network.ssid || "(hidden)"}</span>
                                                        {#if apStandard}
                                                            <span class={getWiFiStandardClass(apStandard)}>{apStandard}</span>
                                                        {/if}
                                                        {#if apIsConnected}
                                                            <span class="chip ok">
                                                                <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                                                                    <path d="M5 7a2 2 0 002.83 0l2-2a2 2 0 00-2.83-2.83l-.5.5M7 5a2 2 0 00-2.83 0l-2 2a2 2 0 002.83 2.83l.5-.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                                                                </svg>
                                                                Connected
                                                            </span>
                                                        {/if}
                                                    </div>
                                                    <div class="ap-header-sub mono">
                                                        {ap.bssid}{#if ap.vendor} · {ap.vendor}{/if} · {ap.band || networkBand(network)}{#if ap.channel} channel {ap.channel}{/if}{#if ap.channelWidth} ({ap.channelWidth} MHz){/if}{#if ap.dfs}<span class="dfs-badge">DFS</span>{/if}
                                                    </div>
                                                </div>
                                                {#if apSp.last}
                                                    <svg width="160" height="36" class="ap-sparkline" aria-hidden="true">
                                                        <path d={apSp.area} fill={apTrendColor} opacity="0.18"/>
                                                        <path d={apSp.d} fill="none" stroke={apTrendColor} stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                                                        <circle cx={apSp.last[0]} cy={apSp.last[1]} r="2" fill={apTrendColor}/>
                                                    </svg>
                                                {/if}
                                            </div>
                                            <div class="detail-grid">
                                        <div class="detail-section">
                                            <div class="detail-section-title">Performance</div>
                                            <div class="capability-item">
                                                <span class="capability-label" title="Signal Strength
Closer to 0 = stronger signal
&lt;-50: Excellent
-50 to -65: Good
&gt;-70: Poor">Signal</span>
                                                <span class="value-pill {getSignalClass(ap.signal) === 'signal-good' ? 'value-good' : getSignalClass(ap.signal) === 'signal-medium' ? 'value-warn' : 'value-bad'}">
                                                    {ap.signal} dBm
                                                </span>
                                            </div>
                                            <div class="capability-item">
                                                <span class="capability-label" title="Transmit Power - Output power of this access point">Transmit Power</span>
                                                <span class="value-pill {ap.txPower ? 'value-neutral' : 'value-unknown'}">
                                                    {ap.txPower ? `${ap.txPower} dBm` : "N/A"}
                                                </span>
                                            </div>
                                            <div class="capability-item">
                                                <span class="capability-label" title="WiFi Mode - 802.11 standard this AP operates on">WiFi Mode</span>
                                                <span class="value-pill value-neutral">
                                                    {getDominantWiFiStandard(ap.capabilities, ap.band)}
                                                </span>
                                            </div>
                                                <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                            title="Maximum PHY rate in Mbps (theoretical peak). Real-world throughput is lower."
                                                        >
                                                            Max PHY Rate
                                                        </span>
                                                        <span
                                                            class="value-pill {capabilityMap.maxPhyRate &&
                                                            isNumberDefined(
                                                                ap.maxPhyRate,
                                                            ) &&
                                                            ap.maxPhyRate > 0
                                                                ? 'value-neutral'
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.maxPhyRate &&
                                                            isNumberDefined(
                                                                ap.maxPhyRate,
                                                            ) &&
                                                            ap.maxPhyRate > 0
                                                                ? `${ap.maxPhyRate} Mbps`
                                                                : "N/A"}
                                                        </span>
                                                    </div>
                                                <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                            title="MIMO Spatial Streams
Number of independent data streams the access point can transmit and receive.
• More spatial streams increase potential throughput
• Expressed as NxN (e.g., 2x2, 4x4, 8x8)
• Requires matching client antenna and radio support
• Each stream adds capacity, not guaranteed speed per client
• Critical for aggregate performance in multi-client environments

COMPATIBILITY WARNINGS FOR MSP:
• Most phones and tablets are only 1x1 or 2x2
• Laptops commonly support 2x2 or 3x3
• Single-stream clients cannot benefit from higher stream counts
• High-stream APs do not improve range
• Poor SNR prevents effective use of multiple streams

UNIFI CONSIDERATIONS:
• UniFi reports maximum supported spatial streams per band
• UniFi APs dynamically allocate streams per client
• MU-MIMO required to use multiple streams across clients simultaneously
• OFDMA (WiFi 6/7) often provides more benefit than extra streams
• 8x8 APs mainly benefit very high-density environments"
                                                        >
                                                            MIMO Streams
                                                        </span>
                                                        <span
                                                            class="value-pill {capabilityMap.mimoStreams &&
                                                            isNumberDefined(
                                                                ap.mimoStreams,
                                                            ) &&
                                                            ap.mimoStreams > 0
                                                                ? 'value-neutral'
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.mimoStreams &&
                                                            isNumberDefined(
                                                                ap.mimoStreams,
                                                            ) &&
                                                            ap.mimoStreams > 0
                                                                ? `${ap.mimoStreams}×${ap.mimoStreams}`
                                                                : "N/A"}
                                                        </span>
                                                    </div>
                                                <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                            title="Beamforming (TXBF) - Transmit beamforming for improved signal strength
• Focuses wireless signal toward receiving devices
• Improves throughput and range for compatible clients
• Reduces interference with neighboring networks
• Supported by VHT (802.11ac), HE (802.11ax), and newer standards
• Particularly effective with MU-MIMO for multiple clients
• Essential for high-speed, long-range connections
• Automatic on most modern APs when supported"
                                                        >VHT/HE Beamforming</span
                                                        >
                                                        <span
                                                            class="value-pill {getCapabilityStatusClass(
                                                                hasBeamformingSupport(
                                                                    ap.capabilities,
                                                                    ap.mumimo,
                                                                ),
                                                            )}"
                                                        >
                                                            {hasBeamformingSupport(
                                                                ap.capabilities,
                                                                ap.mumimo,
                                                            )
                                                                ? "Supported"
                                                                : "Not supported"}
                                                        </span>
                                                    </div>
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
                                                {#if ap.noise && ap.noise < 0}
                                                    <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                                title="Background noise floor in dBm for this channel (lower is better)."
                                                            >
                                                                Noise Floor
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral"
                                                            >
                                                                {ap.noise} dBm
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
                                                {#if ap.surveyUtilization && ap.surveyUtilization > 0}
                                                    <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                                title="Channel busy percentage from nl80211 survey data (airtime usage)."
                                                            >
                                                                Survey
                                                                Utilization
                                                            </span>
                                                            <span
                                                                class="value-pill {getUtilizationStatusClass(
                                                                    ap.surveyUtilization,
                                                                )}"
                                                            >
                                                                {ap.surveyUtilization}%
                                                            </span>
                                                        </div>
                                                {/if}
                                                {#if ap.surveyBusyMs && ap.surveyBusyMs > 0}
                                                    <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                                title="Total channel busy time from nl80211 survey data."
                                                            >
                                                                Survey Busy
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral"
                                                            >
                                                                {ap.surveyBusyMs}
                                                                ms
                                                            </span>
                                                        </div>
                                                {/if}
                                                {#if ap.surveyExtBusyMs && ap.surveyExtBusyMs > 0}
                                                    <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                                title="Channel busy time caused by non‑Wi‑Fi interference (nl80211 survey)."
                                                            >
                                                                Ext Busy
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral"
                                                            >
                                                                {ap.surveyExtBusyMs}
                                                                ms
                                                            </span>
                                                        </div>
                                                {/if}
                                                {#if ap.maxTxPowerDbm && ap.maxTxPowerDbm > 0}
                                                    <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                                title="Maximum regulatory TX power for this channel (nl80211 survey)."
                                                            >
                                                                Max TX Power
                                                            </span>
                                                            <span
                                                                class="value-pill value-neutral"
                                                            >
                                                                {ap.maxTxPowerDbm}
                                                                dBm
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
                                                <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                            title="DTIM Interval (Delivery Traffic Indication Message)
Controls how often buffered broadcast and multicast traffic is delivered.
• Measured in beacon intervals (DTIM = every N beacons)
• Lower DTIM = more frequent wake-ups for clients
• Higher DTIM = better battery life, higher latency for multicast
• Critical for power-saving behavior on mobile devices
• Directly impacts VoIP, push notifications, and IoT responsiveness

COMPATIBILITY WARNINGS FOR MSP:
• Too low DTIM increases battery drain on phones and tablets
• Too high DTIM delays multicast, mDNS, and ARP traffic
• Can break push notifications on iOS and Android
• VoIP and WiFi calling may suffer at high DTIM values
• IoT devices often require specific DTIM behavior

UNIFI CONSIDERATIONS:
• UniFi defaults: 2.4 GHz = DTIM 1, 5 GHz = DTIM 3
• UniFi applies DTIM per SSID, not per AP
• UniFi Talk and VoIP endpoints prefer lower DTIM
• High DTIM can cause perceived 'slow wake' on mobile devices
• DTIM interacts closely with WMM and UAPSD

MSP ADVICE:
• Use defaults unless there is an issue
• Use lower DTIM (1–2) for VoIP and real-time SSIDs
• Use higher DTIM (3–5) for guest or battery-focused SSIDs
• Separate SSIDs for voice, user, and IoT devices when possible
• Always test iOS and Android push behavior after changes"
                                                        >
                                                            DTIM Interval
                                                        </span>
                                                        <span
                                                            class="value-pill {ap.dtim >
                                                            0
                                                                ? 'value-neutral'
                                                                : 'value-unknown'}"
                                                        >
                                                            {ap.dtim > 0
                                                                ? ap.dtim
                                                                : "N/A"}
                                                        </span>
                                                    </div>
                                        </div>
                                        <div class="detail-section">
                                            <div class="detail-section-title">Security</div>
                                                <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                            title="PMF (Protected Management Frames – 802.11w)
Protects management frames from spoofing and deauthentication attacks.
• Prevents deauth/disassoc attack vectors
• Mandatory for WPA3-Personal and WPA3-Enterprise
• Improves overall wireless security posture
• Required for modern compliance frameworks

COMPATIBILITY WARNINGS FOR MSP:
• Legacy clients may fail to connect when PMF is Required
• Some IoT devices only support Optional or Disabled
• Older printers, scanners, and VoIP phones often incompatible
• Windows 7/8 and old Android versions may break

UNIFI CONSIDERATIONS:
• UniFi defaults to PMF Optional on WPA2/WPA3 mixed mode
• PMF Required enforces WPA3-only behavior
• Fast roaming (802.11r) + PMF can cause client auth loops if misconfigured
• Always test IoT and voice devices before enforcing PMF Required

MSP ADVICE:
• Use PMF Optional in mixed environments
• Use PMF Required only on WPA3-only SSIDs
• Create separate SSIDs for legacy or IoT devices"
                                                        >
                                                            PMF (Protected
                                                            Management Frames)
                                                            802.11w
                                                        </span>

                                                        <span
                                                            class="value-pill {capabilityMap.pmf &&
                                                            isNonEmptyString(
                                                                ap.pmf,
                                                            )
                                                                ? getPMFStatusClass(
                                                                      ap.pmf,
                                                                  )
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.pmf &&
                                                            isNonEmptyString(
                                                                ap.pmf,
                                                            )
                                                                ? ap.pmf
                                                                : "N/A"}
                                                        </span>
                                                    </div>
                                                <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                            title="Encryption Ciphers used to protect wireless data in transit.
• CCMP (AES): Secure and recommended
• GCMP: Stronger, used with WiFi 6/6E/7
• TKIP: Deprecated and insecure
• WEP: Broken and unsafe (should never be used)

COMPATIBILITY WARNINGS FOR MSP:
• TKIP forces WiFi 4/5 legacy rates
• Enabling TKIP disables 802.11n/ac/ax features
• Mixed cipher environments reduce performance and security
• Some legacy handhelds require TKIP (avoid if possible)

UNIFI CONSIDERATIONS:
• UniFi automatically prefers CCMP/GCMP when available
• Presence of TKIP can drop entire SSID to legacy mode
• WPA3 requires GCMP or CCMP only

MSP ADVICE:
• Enforce CCMP/GCMP only
• Remove TKIP unless supporting unavoidable legacy hardware and use a new SSID"
                                                        >
                                                            Encryption Ciphers
                                                        </span>
                                                        <span
                                                            class="value-pill {capabilityMap.securityCiphers &&
                                                            isNonEmptyArray(
                                                                ap.securityCiphers,
                                                            )
                                                                ? getCipherStatusClass(
                                                                      ap.securityCiphers,
                                                                  )
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.securityCiphers &&
                                                            isNonEmptyArray(
                                                                ap.securityCiphers,
                                                            )
                                                                ? ap.securityCiphers.join(
                                                                      ", ",
                                                                  )
                                                                : "N/A"}
                                                        </span>
                                                    </div>
                                                <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                            title="Authentication methods used to control network access.
• SAE: WPA3-Personal (most secure)
• PSK: WPA2-Personal (shared password)
• 802.1X: Enterprise authentication using RADIUS

COMPATIBILITY WARNINGS FOR MSP:
• SAE not supported by older clients and IoT devices
• PSK vulnerable to password sharing and brute-force attacks
• 802.1X requires properly configured RADIUS infrastructure
• Misconfigured RADIUS causes widespread client failures

UNIFI CONSIDERATIONS:
• UniFi supports WPA2/WPA3 mixed mode (transition mode)
• Mixed mode may cause slow association or roaming delays
• Fast roaming (802.11r) interacts heavily with auth methods
• UniFi RADIUS outages impact all Enterprise SSIDs

MSP ADVICE:
• Use WPA3-SAE for modern user devices
• Use WPA2-PSK only for legacy or guest access
• Use 802.1X for enterprise, healthcare, or compliance environments"
                                                        >
                                                            Auth Methods
                                                        </span>
                                                        <span
                                                            class="value-pill {capabilityMap.authMethods &&
                                                            isNonEmptyArray(
                                                                ap.authMethods,
                                                            )
                                                                ? getAuthStatusClass(
                                                                      ap.authMethods,
                                                                  )
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.authMethods &&
                                                            isNonEmptyArray(
                                                                ap.authMethods,
                                                            )
                                                                ? ap.authMethods.join(
                                                                      ", ",
                                                                  )
                                                                : "N/A"}
                                                        </span>
                                                    </div>
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
                                                <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                            title="Country Code (Regulatory Domain)
Defines legal transmit power limits and allowed WiFi channels.
• Controls maximum TX power per band and frequencies (2.4 / 5 / 6 GHz)
• Determines available channels and DFS requirements
• Enforced by local regulatory authorities (FCC, ETSI, etc.)
• Critical for legal compliance and RF performance
• Affects roaming behavior and channel planning"
                                                        >
                                                            Country Code
                                                        </span>
                                                        <span
                                                            class="value-pill {capabilityMap.countryCode &&
                                                            isNonEmptyString(
                                                                ap.countryCode,
                                                            )
                                                                ? 'value-neutral'
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.countryCode &&
                                                            isNonEmptyString(
                                                                ap.countryCode,
                                                            )
                                                                ? ap.countryCode
                                                                : "N/A"}
                                                        </span>
                                                    </div>
                                        </div>
                                        <div class="detail-section">
                                            <div class="detail-section-title">WiFi 6 / 7</div>
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
                                                            Max QAM Modulation
                                                        </span>
                                                        <span
                                                            class="value-pill {capabilityMap.qamSupport &&
                                                            isNumberDefined(
                                                                ap.qamSupport,
                                                            )
                                                                ? `value-neutral ${getQamClass(
                                                                      ap.qamSupport,
                                                                  )}`
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.qamSupport &&
                                                            isNumberDefined(
                                                                ap.qamSupport,
                                                            )
                                                                ? `${ap.qamSupport}-QAM`
                                                                : "N/A"}
                                                        </span>
                                                    </div>
                                                {#if ap.mumimo}
                                                    <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                                title="MU-MIMO (Multi-User MIMO) - Simultaneous transmission to multiple clients.
Allows an access point to transmit data to multiple clients simultaneously.
• Increases total network capacity with many active clients
• Uses spatial streams to serve multiple devices at once
• Requires MU-MIMO support on both AP and client devices
• Downlink MU-MIMO is widely supported
• Uplink MU-MIMO introduced with WiFi 6 (ax)
• Most effective in dense, multi-client deployments
• Limited benefit with few active clients or light traffic
• WiFi 5 (ac) introduced 4x4 MU-MIMO
• WiFi 6/6E improved efficiency with OFDMA

COMPATIBILITY WARNINGS FOR MSP:
• Many client devices have limited or inconsistent MU-MIMO support
• iOS devices primarily benefit from OFDMA, not MU-MIMO
• Most phones and tablets are only 1x1 or 2x2
• MU-MIMO efficiency depends heavily on client scheduling
• Mixed client capabilities reduce overall MU-MIMO gains

NOT RECOMMENDED FOR:
• Low-density networks with few concurrent clients
• Environments dominated by single-stream or legacy devices
• Expecting higher single-client speed test results
• Small deployments where OFDMA provides greater benefit

UNIFI CONSIDERATIONS:
• UniFi APs dynamically manage MU-MIMO per client
• WiFi 6/7 UniFi APs rely more on OFDMA than MU-MIMO
• 4x4 and higher APs benefit dense office and classroom layouts
• MU-MIMO works best when paired with proper channel planning
• Client capability visibility in UniFi is essential for tuning

MSP ADVICE:
• Treat MU-MIMO as a capacity feature, not a speed feature
• Prioritize OFDMA and airtime fairness in mixed environments
• Use higher-stream APs for conference rooms and dense areas
• Validate real client capabilities before expecting gains"
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
                                                <div class="capability-item">
                                                    <span class="capability-label" title="OFDMA (Orthogonal Frequency-Division Multiple Access)
WiFi 6 (802.11ax) feature that splits a channel into resource units (RUs) so the AP can serve multiple clients simultaneously in one transmission.
• Downlink OFDMA: AP → many clients in parallel (mandatory for HE APs)
• Uplink OFDMA: many clients → AP in parallel (HE MAC 'OFDMA RA Support' bit; optional, vendor-dependent)
• Reduces contention and per-packet overhead
• Largest gains in dense, mixed-traffic networks
• Distinct from MU-MIMO (spatial streams); modern WiFi 6 stacks combine both
• Inherited by WiFi 6E (6 GHz) and extended in WiFi 7 (multi-RU)

COMPATIBILITY WARNINGS FOR MSP:
• Requires WiFi 6+ clients to benefit
• Some early WiFi 6 clients implement DL OFDMA only
• Mixed legacy fleets see limited improvement
• Buggy AP firmware can degrade throughput when OFDMA enabled — keep firmware current">
                                                        OFDMA
                                                    </span>
                                                    <span class="value-pill {ap.ofdmaDownlink && ap.ofdmaUplink ? 'value-good' : ap.ofdmaDownlink ? 'value-warn' : 'value-unknown'}">
                                                        {ap.ofdmaDownlink && ap.ofdmaUplink
                                                            ? "Downlink + Uplink"
                                                            : ap.ofdmaDownlink
                                                                ? "Downlink only"
                                                                : "N/A"}
                                                    </span>
                                                </div>
                                                <div class="capability-item">
                                                    <span class="capability-label" title="MLO (Multi-Link Operation)
WiFi 7 (802.11be) marquee feature. Lets a client associate over multiple radios/bands at once and aggregate, switch, or load-balance traffic across them.
• STR (Simultaneous TX/RX): full duplex across links — peak throughput
• NSTR: links share TX/RX timing — partial gains
• EMLSR: client uses one link at a time but switches fast — battery-friendly
• Reduces latency tail by avoiding retries on a busy link
• Lifts effective throughput beyond any single channel's max
• Requires WiFi 7 client AND AP; legacy clients fall back to single link

COMPATIBILITY WARNINGS FOR MSP:
• MLO presence here is heuristic — based on EHT Capabilities IE; refine when Multi-Link element parsed
• Real benefit depends on client mode (STR vs EMLSR) and per-link channel quality
• Mixed channel widths or poorly tuned 2.4 GHz links can drag overall MLO performance down
• MLO + 6 GHz LPI/SP power asymmetry can produce surprising roaming behaviour
• UniFi 7 stacks ship MLO on by default on WiFi 7 SSIDs — verify firmware before enabling on production">
                                                        MLO (Multi-Link)
                                                    </span>
                                                    <span class="value-pill {ap.mlo ? 'value-good' : 'value-unknown'}">
                                                        {ap.mlo ? "Supported" : "N/A"}
                                                    </span>
                                                </div>
                                                <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                            title="BBSS Color (0–63) is a WiFi 6+ spatial reuse identifier.
• Helps devices distinguish overlapping access points with the same SSID on same channel
• Enables simultaneous transmissions in dense environments
• Reduces contention and improves airtime efficiency

COMPATIBILITY WARNINGS FOR MSP:
• Only WiFi 6/6E/7 clients benefit
• Legacy devices ignore BSS Color entirely
• Misconfigured dense deployments may see minimal gains

UNIFI CONSIDERATIONS:
• UniFi auto-assigns BSS Color by default
• Manual overrides rarely needed
• Works best with OBSS PD enabled in dense AP layouts

MSP ADVICE:
• Leave enabled in high-density environments
• No downside for legacy clients
• Combine with proper channel planning for best results"
                                                        >
                                                            BSS Color
                                                        </span>
                                                        <span
                                                            class="value-pill {capabilityMap.bssColor &&
                                                            ap.bssColor !==
                                                                undefined &&
                                                            ap.bssColor !== null
                                                                ? 'value-neutral'
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.bssColor &&
                                                            ap.bssColor !==
                                                                undefined &&
                                                            ap.bssColor !== null
                                                                ? ap.bssColor
                                                                : "N/A"}
                                                        </span>
                                                    </div>
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
                                                            class="value-pill {capabilityMap.obssPD &&
                                                            ap.obssPD !==
                                                                undefined
                                                                ? getCapabilityStatusClass(
                                                                      ap.obssPD,
                                                                  )
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.obssPD &&
                                                            ap.obssPD !==
                                                                undefined
                                                                ? ap.obssPD
                                                                    ? "Supported"
                                                                    : "Not supported"
                                                                : "N/A"}
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
• Allows clients and APs to negotiate specific wake and sleep times
• Significantly reduces power consumption for supported devices
• Enables predictable latency for real-time applications
• Critical for battery-powered sensors and mobile devices
• Improves network efficiency with many sleeping clients
• Requires WiFi 6 (802.11ax) or later support

COMPATIBILITY WARNINGS FOR MSP:
• Limited client device support: Mostly high-end devices only
• UniFi 6/7 APs: TWT enabled by default on supported firmware
• iPhone 12+: Supports TWT, battery savings noticeable
• Android 11+: Limited support, vendor-specific implementation
• Windows 10/11: Minimal support, mostly experimental drivers
• Legacy devices: No TWT support, may experience scheduling conflicts
• MSP Advice: Enable only in IoT-heavy environments with compatible devices
• Mixed fleets: No negative impact on non-TWT devices
• Enterprise: Consider for sensor networks and smart building deployments

UNIFI CONSIDERATIONS:
• UniFi 6/7 APs advertise TWT capability automatically
• UniFi does not provide granular per-client TWT tuning
• Benefits depend entirely on client adoption

NOT RECOMMENDED FOR:
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
                                        </div>
                                        <div class="detail-section last">
                                            <div class="detail-section-title">Roaming / QoS</div>
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
• UniFi Supported, but may cause client disconnects on very old devices
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

Enables rapid re-authentication when clients move between access points.
• Reduces roaming time from 100-500ms to 50ms or less
• Critical for VoIP, WiFi calling, and real-time applications
• Uses Fast BSS Transition (FT) key negotiation
• Works best when combined with 802.11k (Neighbor Reports) and 802.11v (BSS Transition)

COMPATIBILITY WARNINGS FOR MSP:
• Some legacy clients do not support 802.11r and may fail to connect
• Windows 7/8: Known authentication and association issues
• Older Android devices (< Android 6): Partial or broken 802.11r support
• Many IoT devices do not support 802.11r (printers, TVs, speakers)
• WPA3 and PMF settings can increase compatibility risk
• Legacy device fallback: May require separate SSID for older devices

UNIFI CONSIDERATIONS:
• UniFi labels 802.11r as 'Fast Roaming'
• Best used on user-only SSIDs with modern clients
• Enabling 802.11r does not force clients to use it
• Roaming behavior still depends on client decisions

NOT RECOMMENDED FOR:
• Public WiFi networks with unknown device types
• Environments with legacy IoT or industrial equipment
• Small offices with unmanaged BYOD and legacy devices
• Residential or home-office deployments without testing

MSP ADVICE:
• Enable only on user SSIDs with controlled device fleets
• Pair with 802.11k and 802.11v for best results
• Always test legacy and IoT devices before rollout
• Use separate SSIDs for modern vs legacy clients when needed"
                                                        >
                                                            Fast BSS Transition
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
                                                {#if ap.neighborReport !== undefined}
                                                    <div
                                                            class="capability-item"
                                                        >
                                                            <span
                                                                class="capability-label"
                                                                title="Neighbor Report (802.11k) assists client roaming decisions.
• Provides list of nearby APs and their capabilities
• Reduces roaming scan time
• Improves handoff speed between access points
• Critical for voice and real-time applications

COMPATIBILITY WARNINGS FOR MSP:
• Some legacy clients ignore or mishandle 802.11k
• Poor roaming clients may still stick to weak APs
• Works best when paired with 802.11v and 802.11r

UNIFI CONSIDERATIONS:
• UniFi enables 802.11k by default
• Essential for UniFi fast roaming performance
• Voice and WiFi calling benefit significantly

MSP ADVICE:
• Keep enabled for multi-AP environments
• Disable only if troubleshooting specific roaming bugs
• Essential for VoIP, WiFi calling, and mobile devices"
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
• Poorly behaving IoT devices may abuse high-priority queues
• WMM does not fix insufficient airtime, interference, or bad RF design

NOT RECOMMENDED FOR:
• Networks with no real-time applications (voice/video)
• Environments with many legacy devices
• Simple setups where traffic prioritization adds complexity
• Networks where all traffic has equal priority

UNIFI 7 CONSIDERATIONS:
• UniFi enables WMM by default on most SSIDs
• Disabling WMM breaks VoIP, WiFi calling, and UAPSD
• Required for proper operation of UniFi Talk and voice endpoints
• UniFi does not expose deep per-client QoS tuning
• Fast roaming (802.11r) assumes WMM is enabled

MSP ADVICE:
• Leave WMM enabled in almost all modern networks
• Mandatory for VoIP, Teams, Zoom, WiFi calling, and SIP phones"
                                                        >
                                                            QoS (WMM)
                                                        </span>
                                                        <span
                                                            class="value-pill {capabilityMap.qosSupport &&
                                                            ap.qosSupport !==
                                                                undefined
                                                                ? getCapabilityStatusClass(
                                                                      ap.qosSupport,
                                                                  )
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.qosSupport &&
                                                            ap.qosSupport !==
                                                                undefined
                                                                ? ap.qosSupport
                                                                    ? "Supported"
                                                                    : "Not supported"
                                                                : "N/A"}
                                                        </span>
                                                    </div>
                                                <div
                                                        class="capability-item"
                                                    >
                                                        <span
                                                            class="capability-label"
                                                            title="UAPSD (Unscheduled Automatic Power Save Delivery)
Power save mechanism for VoIP and real-time applications.
• Allows clients to sleep and wake for specific traffic delivery
• Reduces WiFi power consumption on mobile devices by 15-30%
• Essential for VoIP handsets, tablets, and battery-powered devices
• Requires QoS/WMM support for proper operation
• Can improve voice call quality and battery life
• Critical for enterprise VoWiFi deployments

COMPATIBILITY WARNINGS FOR MSP:
• May cause latency issues if not properly configured
• VoIP phones: UAPSD mandatory for battery-powered handsets
• UniFi: Supported, but requires WMM QoS enabled
• iOS devices: Excellent UAPSD support, minimal issues
• Android: Variable support, vendor-dependent implementation
• Windows: Limited support, may cause VoIP quality degradation
• Legacy devices: Poor UAPSD handling, connection instability
• MSP Advice: Test VoIP devices thoroughly in lab environment
• Enterprise phones: Enable only for certified VoIP endpoints
• Mixed environments: Monitor for voice quality issues

NOT RECOMMENDED FOR:
• Gaming networks where latency is critical
• High-frequency trading or real-time control systems
• Networks with poor QoS implementation
• Environments with predominantly non-VoIP clients• Significantly reduces power consumption for supported devices"
                                                        >
                                                            UAPSD (U-APSD)
                                                        </span>
                                                        <span
                                                            class="value-pill {capabilityMap.uapsd &&
                                                            ap.uapsd !==
                                                                undefined
                                                                ? getCapabilityStatusClass(
                                                                      ap.uapsd,
                                                                  )
                                                                : 'value-unknown'}"
                                                        >
                                                            {capabilityMap.uapsd &&
                                                            ap.uapsd !==
                                                                undefined
                                                                ? ap.uapsd
                                                                    ? "Supported"
                                                                    : "Not supported"
                                                                : "N/A"}
                                                        </span>
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
                            <td colspan="9">
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
        background: var(--bg-1);
        gap: 12px;
        padding: 16px;
        min-height: 0;
    }

    /* ── KPI strip ───────────────────────────────────────────── */
    .kpi-strip {
        display: grid;
        grid-template-columns: repeat(5, 1fr);
        gap: 8px;
        flex-shrink: 0;
    }

    .kpi-tile {
        padding: 12px 14px;
        border: 1px solid var(--line-1);
        border-radius: 6px;
        background: var(--bg-2);
        display: flex;
        flex-direction: column;
        gap: 4px;
        min-width: 0;
    }

    .kpi-label {
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.1em;
        color: var(--fg-3);
        font-weight: 600;
    }

    .kpi-value {
        font-size: 18px;
        font-weight: 500;
        color: var(--fg-1);
        line-height: 1.1;
        letter-spacing: -0.01em;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .kpi-value.kpi-accent {
        color: var(--acc-1);
        font-family: var(--font-ui);
        font-weight: 600;
    }

    .kpi-value.kpi-empty {
        color: var(--fg-2);
    }

    .kpi-sub {
        font-size: 10.5px;
        color: var(--fg-3);
    }

    /* ── Filter bar ──────────────────────────────────────────── */
    .filter-bar {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 10px 12px;
        background: var(--bg-2);
        border: 1px solid var(--line-1);
        border-radius: 8px;
        flex-shrink: 0;
        flex-wrap: wrap;
    }

    .input-group {
        display: inline-flex;
        align-items: center;
        background: var(--bg-4);
        border: 1px solid var(--line-2);
        border-radius: 5px;
        padding: 0 10px;
        gap: 6px;
        flex: 1;
        max-width: 340px;
        min-width: 200px;
        transition: border-color 0.1s;
    }

    .input-group:focus-within {
        border-color: var(--acc-1-line);
    }

    .input-group .search-icon {
        color: var(--fg-3);
        flex-shrink: 0;
    }

    .input-group input {
        background: transparent;
        border: none;
        outline: none;
        padding: 6px 0;
        font-size: 12px;
        color: var(--fg-1);
        width: 100%;
        font-family: inherit;
    }

    .segmented {
        display: inline-flex;
        background: var(--bg-3);
        border: 1px solid var(--line-2);
        border-radius: 6px;
        padding: 2px;
        gap: 2px;
    }

    .segmented button {
        background: transparent;
        border: none;
        color: var(--fg-2);
        font-size: 12px;
        padding: 4px 10px;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
        font-family: inherit;
    }

    .segmented button.active {
        background: var(--bg-1);
        color: var(--fg-1);
        box-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    }

    .segmented button:hover:not(.active) {
        color: var(--fg-1);
    }

    .hidden-toggle {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 5px 10px;
        font-size: 12px;
        color: var(--fg-2);
        cursor: pointer;
        border: 1px solid var(--line-2);
        border-radius: 5px;
        background: transparent;
        transition: background 0.1s;
    }

    .hidden-toggle.on {
        background: var(--bg-3);
    }

    .hidden-toggle input {
        accent-color: var(--acc-1);
        margin: 0;
    }

    .filter-spacer {
        flex: 1;
    }

    .count-chip {
        font-size: 11px;
        color: var(--fg-3);
    }

    .network-table-wrapper {
        flex: 1;
        overflow: auto;
        background: var(--bg-2);
        border: 1px solid var(--line-1);
        border-radius: 8px;
        min-height: 0;
    }

    .network-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 12px;
    }

    .network-table th {
        background: var(--bg-2);
        padding: 8px 12px;
        text-align: left;
        font-weight: 600;
        font-size: 10px;
        letter-spacing: 0.1em;
        text-transform: uppercase;
        color: var(--fg-3);
        border-bottom: 1px solid var(--line-1);
        position: sticky;
        top: 0;
        z-index: 10;
        user-select: none;
    }

    .network-table th.sortable {
        cursor: pointer;
        transition: color 0.15s ease;
    }

    .network-table th.sortable:hover {
        color: var(--fg-1);
    }

    .sort-indicator {
        margin-left: 4px;
        color: var(--acc-1);
    }

    .history-col {
        width: 100px;
    }

    .network-table td {
        padding: 10px 12px;
        border-bottom: 1px solid var(--line-1);
        vertical-align: middle;
        color: var(--fg-1);
    }

    .history-cell {
        width: 100px;
    }

    .sparkline {
        display: block;
    }

    .history-empty {
        font-size: 11px;
        color: var(--fg-4);
    }

    .network-row {
        transition: background-color 0.2s ease;
        cursor: pointer;
    }

    .network-row:hover {
        background: var(--bg-3);
    }

    .network-row.connected {
        background: rgba(74, 222, 128, 0.04);
    }

    .network-row.connected:hover {
        background: rgba(74, 222, 128, 0.07);
    }

    .network-row.expanded {
        background: var(--bg-3);
    }

    .network-row.expanded td {
        border-bottom-color: transparent;
    }

    .network-row.has-issues {
        border-left: 3px solid var(--warning);
    }

    .chevron-col {
        width: 30px;
    }

    .chevron-cell {
        color: var(--fg-3);
        text-align: center;
        width: 30px;
    }

    .num-col {
        width: 62px;
    }

    .band-col {
        width: 90px;
    }

    .ssid-cell {
        font-weight: 500;
    }

    .ssid-content {
        display: flex;
        flex-direction: column;
        gap: 3px;
    }

    .ssid-line {
        display: flex;
        align-items: center;
        gap: 6px;
    }

    .ssid-text {
        font-size: 13px;
        font-weight: 500;
        color: var(--fg-1);
    }

    .ssid-text.hidden-ssid {
        color: var(--fg-3);
        font-style: italic;
    }

    .ssid-sub {
        font-size: 10.5px;
        color: var(--fg-3);
    }

    .ap-count-cell {
        color: var(--fg-2);
    }

    .num {
        text-align: right;
        font-family: var(--font-mono);
        font-variant-numeric: tabular-nums;
    }

    .band-cell {
        font-size: 11px;
    }

    .band-text {
        color: var(--fg-2);
    }

    .band-width {
        color: var(--fg-4);
        margin-left: 4px;
    }

    .channel-cell {
        color: var(--fg-1);
    }

    .signal-cell {
        font-weight: 500;
    }

    .signal-row {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .signal-value {
        font-size: 11px;
    }

    .sig-bar {
        display: inline-flex;
        align-items: flex-end;
        gap: 2px;
        height: 12px;
    }

    .sig-bar span {
        width: 3px;
        background: var(--fg-4);
        border-radius: 1px;
    }

    .sig-bar span.on {
        background: var(--ok);
    }

    .sig-bar span.on.warn {
        background: var(--warn);
    }

    .sig-bar span.on.bad {
        background: var(--bad);
    }

    .security-cell {
        font-weight: 500;
    }

    .security-inline {
        display: inline-flex;
        align-items: center;
        gap: 5px;
        font-size: 11px;
    }

    .signal-good {
        color: var(--ok);
    }

    .signal-medium {
        color: var(--warn);
    }

    .signal-poor {
        color: var(--bad);
    }

    .security-good {
        color: var(--fg-1);
    }

    .security-medium {
        color: var(--warn);
    }

    .security-poor {
        color: var(--bad);
    }

    .status-cell {
        font-size: 11px;
    }

    .status-available {
        color: var(--fg-3);
        font-size: 11px;
    }

    /* Chip / pill */
    .chip {
        display: inline-flex;
        align-items: center;
        gap: 5px;
        padding: 3px 8px;
        border-radius: 12px;
        font-size: 11px;
        font-weight: 500;
        font-family: var(--font-mono);
        border: 1px solid var(--line-2);
        color: var(--fg-2);
        background: var(--bg-3);
        white-space: nowrap;
    }

    .chip.ok {
        color: var(--ok);
        border-color: var(--ok-line);
        background: var(--ok-bg);
    }

    .chip.warn {
        color: var(--warn);
        border-color: var(--warn-line);
        background: var(--warn-bg);
    }

    .chip.bad {
        color: var(--bad);
        border-color: var(--bad-line);
        background: var(--bad-bg);
    }

    .chip.acc {
        color: var(--acc-1);
        border-color: var(--acc-1-line);
        background: var(--acc-1-bg);
    }

    .chip.ghost {
        background: transparent;
    }

    .ap-details-row {
        background: var(--bg-1);
    }

    .ap-details {
        padding: 4px 16px 16px;
        display: flex;
        flex-direction: column;
        gap: 0;
    }

    .ap-card {
        padding: 0;
        border-bottom: 1px solid var(--line-1);
    }

    .ap-card:last-child {
        border-bottom: none;
    }

    .ap-header {
        display: flex;
        align-items: center;
        gap: 12px;
        margin-bottom: 14px;
        padding-bottom: 12px;
        border-bottom: 1px solid var(--line-1);
    }

    .ap-header-main {
        flex: 1;
        min-width: 0;
    }

    .ap-header-line {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 2px;
        flex-wrap: wrap;
    }

    .ap-ssid {
        font-size: 14px;
        font-weight: 600;
        color: var(--fg-1);
    }

    .ap-header-sub {
        font-size: 11px;
        color: var(--fg-3);
    }

    .ap-sparkline {
        flex-shrink: 0;
        display: block;
    }

    /* Detail grid — 4-column expanded view */
    .detail-grid {
        display: grid;
        grid-template-columns: repeat(4, 1fr);
        gap: 0;
    }

    .detail-section {
        padding: 14px 16px;
        border-right: 1px solid var(--line-1);
        min-width: 0;
    }

    .detail-section.last,
    .detail-section:last-child {
        border-right: none;
    }

    .detail-section-title {
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.12em;
        color: var(--fg-3);
        font-weight: 600;
        margin-bottom: 10px;
    }

    @media (max-width: 1100px) {
        .detail-grid {
            grid-template-columns: repeat(2, 1fr);
        }
        .detail-section {
            border-right: none;
            border-bottom: 1px solid var(--line-1);
        }
        .detail-section:nth-child(odd) {
            border-right: 1px solid var(--line-1);
        }
    }

    @media (max-width: 700px) {
        .detail-grid {
            grid-template-columns: 1fr;
        }
        .detail-section:nth-child(odd) {
            border-right: none;
        }
    }

    .capability-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 8px;
        padding: 5px 0;
        font-size: 12px;
        border-bottom: 1px dashed var(--line-1);
    }

    .capability-item:last-child {
        border-bottom: none;
    }

    .capability-label {
        color: var(--fg-3);
        display: flex;
        align-items: center;
        position: relative;
        cursor: help;
        font-size: 11.5px;
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
        background: color-mix(in srgb, var(--success) 18%, transparent);
        color: var(--success);
        border: 1px solid color-mix(in srgb, var(--success) 45%, transparent);
    }

    .value-bad {
        background: color-mix(in srgb, var(--danger) 18%, transparent);
        color: var(--danger);
        border: 1px solid color-mix(in srgb, var(--danger) 45%, transparent);
    }

    .value-neutral {
        background: color-mix(in srgb, var(--muted-2) 18%, transparent);
        color: var(--text);
        border: 1px solid color-mix(in srgb, var(--muted-2) 35%, transparent);
    }

    .value-warn {
        background: color-mix(in srgb, var(--warning) 18%, transparent);
        color: var(--warning);
        border: 1px solid color-mix(in srgb, var(--warning) 45%, transparent);
    }

    .value-unknown {
        background: color-mix(in srgb, var(--panel-soft) 70%, transparent);
        color: var(--muted-2);
        border: 1px dashed color-mix(in srgb, var(--muted-2) 35%, transparent);
    }

    .dfs-badge {
        display: inline-block;
        padding: 1px 5px;
        border-radius: 3px;
        font-size: 9px;
        font-weight: 600;
        background: color-mix(in srgb, var(--warning) 20%, transparent);
        color: var(--warning);
        border: 1px solid color-mix(in srgb, var(--warning) 45%, transparent);
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
