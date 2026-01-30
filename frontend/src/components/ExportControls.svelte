<script>
    import { SaveReport } from "../../wailsjs/go/main/App.js";

    export let networks = null;
    export let clientStats = null;

    function downloadFile(content, filename, type = "text/plain") {
        const blob = new Blob([content], { type: `${type};charset=utf-8;` });
        const link = document.createElement("a");

        if ("msSaveBlob" in navigator) {
            navigator.msSaveBlob(blob, filename);
        } else {
            const url = URL.createObjectURL(blob);
            link.href = url;
            link.download = filename;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            URL.revokeObjectURL(url);
        }
    }

    async function saveFile(content, filename, type = "text/plain") {
        try {
            if (typeof SaveReport === "function") {
                const path = await SaveReport(filename, content);
                if (path) return;
            }
        } catch (err) {
            console.warn("SaveReport failed, falling back to download:", err);
        }
        downloadFile(content, filename, type);
    }

    function escapeHtml(value) {
        return String(value ?? "")
            .replaceAll("&", "&amp;")
            .replaceAll("<", "&lt;")
            .replaceAll(">", "&gt;")
            .replaceAll('"', "&quot;")
            .replaceAll("'", "&#39;");
    }

    function sanitizeAccessPoint(ap) {
        if (!ap) return ap;
        const { noise, txPower, ...rest } = ap;
        return rest;
    }

    function sanitizeNetworks(data) {
        if (!Array.isArray(data)) return data;
        return data.map((network) => {
            const accessPoints = (network.accessPoints || []).map(
                sanitizeAccessPoint,
            );
            return {
                ...network,
                accessPoints,
            };
        });
    }

    function sanitizeClientStats(stats) {
        if (!stats) return stats;
        const { noise, snr, ...rest } = stats;
        return {
            ...rest,
            snr: typeof snr === "number" && snr > 0 ? snr : null,
        };
    }

    function buildNetworkCsv() {
        if (!networks || networks.length === 0) return "";
        const headers = [
            "SSID",
            "BSSID",
            "Vendor",
            "Band",
            "FrequencyMHz",
            "Channel",
            "ChannelWidthMHz",
            "SignalDbm",
            "SignalQuality",
            "Security",
            "Capabilities",
            "DFS",
            "LastSeen",
            "APName",
            "BSSLoadStations",
            "BSSLoadUtilization",
            "MaxPhyRateMbps",
            "MIMOStreams",
            "QoSSupport",
            "PMF",
            "QAMSupport",
            "UAPSD",
            "BSSColor",
            "OBSSPD",
            "CountryCode",
            "SecurityCiphers",
            "AuthMethods",
            "WiFiStandard",
            "APCount",
            "BestSignal",
            "BestSignalAP",
        ];
        const rows = [];
        networks.forEach((network) => {
            const base = {
                ssid: network.ssid,
                apCount: network.apCount,
                bestSignal: network.bestSignal,
                bestSignalAP: network.bestSignalAP || "",
            };
            if (network.accessPoints && network.accessPoints.length > 0) {
                network.accessPoints.forEach((ap) => {
                    rows.push([
                        base.ssid,
                        ap.bssid,
                        ap.vendor,
                        ap.band,
                        ap.frequency,
                        ap.channel,
                        ap.channelWidth,
                        ap.signal,
                        ap.signalQuality,
                        ap.security,
                        (ap.capabilities || []).join("|"),
                        ap.dfs,
                        ap.lastSeen,
                        ap.apName,
                        ap.bssLoadStations,
                        ap.bssLoadUtilization,
                        ap.maxPhyRate > 0 ? ap.maxPhyRate : "",
                        ap.mimoStreams,
                        ap.qosSupport,
                        ap.pmf,
                        ap.qamSupport,
                        ap.uapsd,
                        ap.bssColor,
                        ap.obssPD,
                        ap.countryCode,
                        (ap.securityCiphers || []).join("|"),
                        (ap.authMethods || []).join("|"),
                        ap.wifiStandard,
                        base.apCount,
                        base.bestSignal,
                        base.bestSignalAP,
                    ]);
                });
            } else {
                rows.push([
                    base.ssid,
                    "",
                    "",
                    "",
                    "",
                    network.channel,
                    "",
                    "",
                    "",
                    network.security,
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    "",
                    base.apCount,
                    base.bestSignal,
                    base.bestSignalAP,
                ]);
            }
        });
        return [headers, ...rows]
            .map((row) =>
                row
                    .map(
                        (value) =>
                            `"${String(value ?? "").replaceAll('"', '""')}"`,
                    )
                    .join(","),
            )
            .join("\n");
    }

    function buildReportData() {
        const totalNetworks = networks?.length || 0;
        const totalAPs =
            networks?.reduce(
                (count, network) => count + (network.accessPoints?.length || 0),
                0,
            ) || 0;

        return {
            generatedAt: new Date().toISOString(),
            summary: {
                totalNetworks,
                totalAPs,
                connected: clientStats?.connected || false,
                connectedSSID: clientStats?.ssid || null,
                connectedBSSID: clientStats?.bssid || null,
            },
            clientStats: sanitizeClientStats(clientStats) || null,
            networks: sanitizeNetworks(networks) || [],
        };
    }

    function buildHtmlReport() {
        const report = buildReportData();
        const apRows = (report.networks || []).flatMap((network) => {
            if (!network.accessPoints || network.accessPoints.length === 0) {
                return [
                    `
                <tr>
                    <td>${escapeHtml(network.ssid)}</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>${escapeHtml(network.channel ?? "—")}</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>${escapeHtml(network.security ?? "—")}</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>—</td>
                    <td>${escapeHtml(network.apCount ?? "—")}</td>
                    <td>${escapeHtml(network.bestSignal ?? "—")}</td>
                    <td>${escapeHtml(network.bestSignalAP ?? "—")}</td>
                </tr>
                    `,
                ];
            }
            return network.accessPoints.map(
                (ap) => `
                <tr>
                    <td>${escapeHtml(network.ssid)}</td>
                    <td>${escapeHtml(ap.bssid)}</td>
                    <td>${escapeHtml(ap.vendor)}</td>
                    <td>${escapeHtml(ap.band)}</td>
                    <td>${escapeHtml(ap.frequency)}</td>
                    <td>${escapeHtml(ap.channel)}</td>
                    <td>${escapeHtml(ap.channelWidth)}</td>
                    <td>${escapeHtml(ap.signal)}</td>
                    <td>${escapeHtml(ap.signalQuality)}</td>
                    <td>${escapeHtml(ap.security)}</td>
                    <td>${escapeHtml((ap.capabilities || []).join(" | "))}</td>
                    <td>${escapeHtml(ap.dfs)}</td>
                    <td>${escapeHtml(ap.lastSeen)}</td>
                    <td>${escapeHtml(ap.apName)}</td>
                    <td>${escapeHtml(ap.bssLoadStations)}</td>
                    <td>${escapeHtml(ap.bssLoadUtilization)}</td>
                    <td>${escapeHtml(ap.maxPhyRate > 0 ? ap.maxPhyRate : "")}</td>
                    <td>${escapeHtml(ap.mimoStreams)}</td>
                    <td>${escapeHtml(ap.qosSupport)}</td>
                    <td>${escapeHtml(ap.pmf)}</td>
                    <td>${escapeHtml(ap.qamSupport)}</td>
                    <td>${escapeHtml(ap.uapsd)}</td>
                    <td>${escapeHtml(ap.bssColor)}</td>
                    <td>${escapeHtml(ap.obssPD)}</td>
                    <td>${escapeHtml(ap.countryCode)}</td>
                    <td>${escapeHtml((ap.securityCiphers || []).join(" | "))}</td>
                    <td>${escapeHtml((ap.authMethods || []).join(" | "))}</td>
                    <td>${escapeHtml(ap.wifiStandard)}</td>
                    <td>${escapeHtml(network.apCount)}</td>
                    <td>${escapeHtml(network.bestSignal)}</td>
                    <td>${escapeHtml(network.bestSignalAP)}</td>
                </tr>
                `,
            );
        });

        const networksTableRows = apRows.join("");

        return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>WiFi Report</title>
  <style>
    body { font-family: "Inter", "Segoe UI", Arial, sans-serif; margin: 24px; color: #0f172a; }
    h1 { margin: 0 0 4px; }
    .meta { color: #475569; margin-bottom: 16px; }
    .summary { display: grid; grid-template-columns: repeat(auto-fit,minmax(200px,1fr)); gap: 12px; margin-bottom: 20px; }
    .card { border: 1px solid #e2e8f0; border-radius: 10px; padding: 12px; background: #f8fafc; }
    table { width: 100%; border-collapse: collapse; margin-top: 12px; }
    th, td { border-bottom: 1px solid #e2e8f0; padding: 8px 10px; text-align: left; font-size: 13px; }
    th { background: #f1f5f9; font-weight: 600; }
  </style>
</head>
<body>
  <h1>WiFi Report</h1>
  <div class="meta">Generated: ${escapeHtml(report.generatedAt)}</div>
  <div class="summary">
    <div class="card"><strong>Networks</strong><div>${escapeHtml(report.summary.totalNetworks)}</div></div>
    <div class="card"><strong>APs</strong><div>${escapeHtml(report.summary.totalAPs)}</div></div>
    <div class="card"><strong>Connected</strong><div>${escapeHtml(report.summary.connected)}</div></div>
    <div class="card"><strong>SSID</strong><div>${escapeHtml(report.summary.connectedSSID || "—")}</div></div>
  </div>

  <h2>Networks & Access Points</h2>
  <table>
    <thead>
      <tr>
        <th>SSID</th>
        <th>BSSID</th>
        <th>Vendor</th>
        <th>Band</th>
        <th>Frequency</th>
        <th>Channel</th>
        <th>Width</th>
        <th>Signal</th>
        <th>Quality</th>
        <th>Security</th>
        <th>Capabilities</th>
        <th>DFS</th>
        <th>Last Seen</th>
        <th>AP Name</th>
        <th>BSS Stations</th>
        <th>BSS Utilization</th>
        <th>Max PHY Rate</th>
        <th>MIMO</th>
        <th>QoS</th>
        <th>PMF</th>
        <th>QAM</th>
        <th>UAPSD</th>
        <th>BSS Color</th>
        <th>OBSS PD</th>
        <th>Country</th>
        <th>Ciphers</th>
        <th>Auth Methods</th>
        <th>WiFi Standard</th>
        <th>AP Count</th>
        <th>Best Signal</th>
        <th>Best BSSID</th>
      </tr>
    </thead>
    <tbody>
      ${networksTableRows || "<tr><td colspan='6'>No networks</td></tr>"}
    </tbody>
  </table>
</body>
</html>`;
    }
</script>

<div class="export-controls">
    {#if networks && networks.length > 0}
        <div class="export-group">
            <span class="export-label">Export Networks:</span>
            <button
                class="export-btn btn-csv"
                on:click={() =>
                    saveFile(buildNetworkCsv(), "networks.csv", "text/csv")}
                title="Export networks to CSV"
            >
                CSV
            </button>
            <button
                class="export-btn btn-json"
                on:click={() =>
                    saveFile(
                        JSON.stringify(sanitizeNetworks(networks), null, 2),
                        "networks.json",
                        "application/json",
                    )}
                title="Export networks to JSON"
            >
                JSON
            </button>
        </div>
    {/if}

    {#if clientStats}
        <div class="export-group">
            <span class="export-label">Export Stats:</span>
            <button
                class="export-btn btn-json"
                on:click={() =>
                    saveFile(
                        JSON.stringify(
                            sanitizeClientStats(clientStats),
                            null,
                            2,
                        ),
                        "client-stats.json",
                        "application/json",
                    )}
                title="Export client statistics to JSON"
            >
                JSON
            </button>
        </div>
    {/if}

    <div class="export-group">
        <span class="export-label">Export Report:</span>
        <button
            class="export-btn btn-json"
            on:click={() =>
                saveFile(
                    JSON.stringify(buildReportData(), null, 2),
                    "wifi-report.json",
                    "application/json",
                )}
            title="Export full report to JSON"
        >
            JSON
        </button>
        <button
            class="export-btn btn-html"
            on:click={() =>
                saveFile(buildHtmlReport(), "wifi-report.html", "text/html")}
            title="Export full report to HTML"
        >
            HTML
        </button>
    </div>
</div>

<style>
    .export-controls {
        display: flex;
        flex-direction: column;
        gap: 12px;
        padding: 16px;
        background: var(--panel-soft);
        border-radius: 6px;
        border: 1px solid var(--border);
    }

    .export-group {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .export-label {
        font-size: 14px;
        font-weight: 500;
        color: var(--text);
        white-space: nowrap;
    }

    .export-btn {
        padding: 8px 16px;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 13px;
        font-weight: 500;
        transition: all 0.2s ease;
    }

    .export-btn:hover {
        transform: translateY(-1px);
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
    }

    .export-btn:active {
        transform: translateY(0);
    }

    .btn-csv {
        background: var(--success);
        color: white;
    }

    .btn-csv:hover {
        background: color-mix(in srgb, var(--success) 85%, black);
    }

    .btn-json {
        background: var(--warning);
        color: white;
    }

    .btn-json:hover {
        background: color-mix(in srgb, var(--warning) 85%, black);
    }

    .btn-html {
        background: var(--accent);
        color: white;
    }

    .btn-html:hover {
        background: color-mix(in srgb, var(--accent) 85%, black);
    }
</style>
