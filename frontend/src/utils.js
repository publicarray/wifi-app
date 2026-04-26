export function isNumber(value) {
    return typeof value === "number" && !Number.isNaN(value);
}

export function signalToneClass(dBm) {
    if (!isNumber(dBm)) return "bad";
    if (dBm >= -60) return "ok";
    if (dBm >= -72) return "warn";
    return "bad";
}

export function signalTone(dBm) {
    if (!isNumber(dBm)) return "muted";
    if (dBm >= -60) return "ok";
    if (dBm >= -72) return "warn";
    return "bad";
}

export function signalBarCount(dBm) {
    if (!isNumber(dBm)) return 0;
    if (dBm >= -55) return 4;
    if (dBm >= -65) return 3;
    if (dBm >= -75) return 2;
    if (dBm >= -85) return 1;
    return 0;
}

export function getSignalClass(signal) {
    if (!isNumber(signal)) return "signal-poor";
    if (signal > -60) return "signal-good";
    if (signal > -75) return "signal-medium";
    return "signal-poor";
}

export function pad2(n) {
    return n < 10 ? "0" + n : "" + n;
}

export function escapeHtml(value) {
    return String(value ?? "")
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#39;");
}

export function formatBytes(bytes) {
    if (bytes === 0) return "0 B";
    if (!isNumber(bytes)) return "—";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.min(
        sizes.length - 1,
        Math.floor(Math.log(bytes) / Math.log(k)),
    );
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

export function formatCount(n) {
    if (!isNumber(n)) return "—";
    return n.toLocaleString();
}
