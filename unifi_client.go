package main

// Client for the official UniFi Network Integration API (Network
// Application >= 9.x), the API-key-authenticated HTTP API generated from
// Settings -> Control Plane -> Integrations in the UniFi console.
//
// Auth is a stateless `X-API-KEY` header — no cookie login/logout like the
// legacy /api/s/{site} API. On UniFi OS consoles (UDM/UDR/CloudKey) the API
// is served under /proxy/network/integration/v1; self-hosted Network Server
// installs expose the same routes without the /proxy/network prefix, so the
// client probes both once and caches the working base path.
//
// Controllers ship with self-signed TLS certificates by default; TLS
// verification is only relaxed when the user opts in via
// `unifi_allow_insecure_tls` (see config.go).

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var uniFiBasePathCandidates = []string{
	"/proxy/network/integration/v1", // UniFi OS consoles
	"/integration/v1",               // self-hosted Network Server
}

const (
	uniFiRequestTimeout = 15 * time.Second
	uniFiPageLimit      = 200
	uniFiMaxPages       = 50 // hard stop: 10k records is far beyond any sane site
)

type uniFiSite struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type uniFiDevice struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Model           string   `json:"model"`
	MACAddress      string   `json:"macAddress"`
	IPAddress       string   `json:"ipAddress"`
	State           string   `json:"state"` // e.g. ONLINE / OFFLINE
	FirmwareVersion string   `json:"firmwareVersion"`
	Features        []string `json:"features"`
}

// uniFiClientRecord is one entry from the connected-clients endpoint. For
// wireless clients UplinkDeviceID identifies the AP the client is associated
// with, which is how per-AP client counts are derived.
type uniFiClientRecord struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	MACAddress     string `json:"macAddress"`
	IPAddress      string `json:"ipAddress"`
	Type           string `json:"type"` // WIRED / WIRELESS / VPN
	UplinkDeviceID string `json:"uplinkDeviceId"`
}

type uniFiInfo struct {
	ApplicationVersion string `json:"applicationVersion"`
}

// uniFiWLAN is one configured SSID broadcast. Field names are parsed
// tolerantly (name/ssid, hidden/hideSsid) because the WLAN endpoint shape
// varies across Network app versions and is not runtime-verified here.
type uniFiWLAN struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SSID     string `json:"ssid"`
	Enabled  *bool  `json:"enabled"`
	Hidden   bool   `json:"hidden"`
	HideSSID *bool  `json:"hideSsid"`
}

// errUniFiNotFound marks an HTTP 404 so callers can distinguish "endpoint
// not available on this controller version" from real failures.
var errUniFiNotFound = errors.New("not found")

// errUniFiWLANUnsupported is returned once WLAN listing has been probed and
// found unavailable; callers should stop asking.
var errUniFiWLANUnsupported = errors.New("unifi: wlan listing not supported by this controller")

// uniFiWLANPathCandidates are the known homes of the SSID-broadcast list
// across Network app versions, probed in order.
var uniFiWLANPathCandidates = []string{
	"/sites/%s/wlans",
	"/sites/%s/wifi/broadcasts",
}

// uniFiPage is the pagination envelope every v1 list endpoint uses.
type uniFiPage[T any] struct {
	Offset     int `json:"offset"`
	Limit      int `json:"limit"`
	Count      int `json:"count"`
	TotalCount int `json:"totalCount"`
	Data       []T `json:"data"`
}

type uniFiClient struct {
	controllerURL string
	apiKey        string
	httpClient    *http.Client

	mu              sync.Mutex
	apiBase         string // resolved controllerURL + base path; "" until discovered
	wlanPath        string // resolved WLAN list path pattern; "" until probed
	wlanUnsupported bool   // set when no WLAN path candidate worked
}

func newUniFiClient(controllerURL, apiKey string, allowInsecureTLS bool) *uniFiClient {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	if allowInsecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &uniFiClient{
		controllerURL: strings.TrimRight(controllerURL, "/"),
		apiKey:        apiKey,
		httpClient: &http.Client{
			Timeout:   uniFiRequestTimeout,
			Transport: transport,
		},
	}
}

// resolveAPIBase discovers which base path this controller serves the
// Integration API under. A 401/403 still identifies the path (the route
// exists; the key is wrong) so it is cached too and the real request will
// surface the auth error.
func (c *uniFiClient) resolveAPIBase(ctx context.Context) (string, error) {
	c.mu.Lock()
	base := c.apiBase
	c.mu.Unlock()
	if base != "" {
		return base, nil
	}

	var lastErr error
	for _, candidate := range uniFiBasePathCandidates {
		probe := c.controllerURL + candidate + "/sites?limit=1"
		status, _, err := c.doRaw(ctx, probe)
		if err != nil {
			lastErr = err
			continue
		}
		if status == http.StatusOK || status == http.StatusUnauthorized || status == http.StatusForbidden {
			resolved := c.controllerURL + candidate
			c.mu.Lock()
			c.apiBase = resolved
			c.mu.Unlock()
			return resolved, nil
		}
		lastErr = fmt.Errorf("unexpected status %d probing %s", status, candidate)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("integration API not found on controller")
	}
	return "", fmt.Errorf("unifi: could not locate integration API at %s: %w", c.controllerURL, lastErr)
}

func (c *uniFiClient) doRaw(ctx context.Context, url string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

// getJSON fetches base+path and decodes the response into out, translating
// the common failure statuses into actionable error text for the UI.
func (c *uniFiClient) getJSON(ctx context.Context, path string, out any) error {
	base, err := c.resolveAPIBase(ctx)
	if err != nil {
		return err
	}
	status, body, err := c.doRaw(ctx, base+path)
	if err != nil {
		return fmt.Errorf("unifi: request %s failed: %w", path, err)
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("unifi: API key rejected (HTTP %d) — check the key in Settings", status)
	case status == http.StatusNotFound:
		return fmt.Errorf("unifi: endpoint %s: %w", path, errUniFiNotFound)
	case status != http.StatusOK:
		return fmt.Errorf("unifi: HTTP %d for %s", status, path)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("unifi: decode %s: %w", path, err)
	}
	return nil
}

// fetchAllPages walks a paginated v1 list endpoint until totalCount is
// reached (or a short/empty page ends the walk). pathFn receives the offset.
func fetchAllPages[T any](ctx context.Context, c *uniFiClient, path string) ([]T, error) {
	var all []T
	offset := 0
	for page := 0; page < uniFiMaxPages; page++ {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		var envelope uniFiPage[T]
		if err := c.getJSON(ctx, fmt.Sprintf("%s%soffset=%d&limit=%d", path, sep, offset, uniFiPageLimit), &envelope); err != nil {
			return nil, err
		}
		all = append(all, envelope.Data...)
		offset += len(envelope.Data)
		if len(envelope.Data) == 0 || offset >= envelope.TotalCount {
			break
		}
	}
	return all, nil
}

func (c *uniFiClient) Info(ctx context.Context) (uniFiInfo, error) {
	var info uniFiInfo
	err := c.getJSON(ctx, "/info", &info)
	return info, err
}

func (c *uniFiClient) Sites(ctx context.Context) ([]uniFiSite, error) {
	return fetchAllPages[uniFiSite](ctx, c, "/sites")
}

func (c *uniFiClient) Devices(ctx context.Context, siteID string) ([]uniFiDevice, error) {
	return fetchAllPages[uniFiDevice](ctx, c, "/sites/"+siteID+"/devices")
}

func (c *uniFiClient) Clients(ctx context.Context, siteID string) ([]uniFiClientRecord, error) {
	return fetchAllPages[uniFiClientRecord](ctx, c, "/sites/"+siteID+"/clients")
}

// WLANs lists the configured SSID broadcasts. The endpoint path moved across
// Network app versions, so candidates are probed once; a controller that
// serves none of them is remembered as unsupported and the feature (hidden
// SSID naming) silently degrades.
func (c *uniFiClient) WLANs(ctx context.Context, siteID string) ([]uniFiWLAN, error) {
	c.mu.Lock()
	path := c.wlanPath
	unsupported := c.wlanUnsupported
	c.mu.Unlock()

	if unsupported {
		return nil, errUniFiWLANUnsupported
	}
	if path != "" {
		return fetchAllPages[uniFiWLAN](ctx, c, fmt.Sprintf(path, siteID))
	}

	for _, candidate := range uniFiWLANPathCandidates {
		wlans, err := fetchAllPages[uniFiWLAN](ctx, c, fmt.Sprintf(candidate, siteID))
		if err == nil {
			c.mu.Lock()
			c.wlanPath = candidate
			c.mu.Unlock()
			return wlans, nil
		}
		if !errors.Is(err, errUniFiNotFound) {
			// Auth/network problem — don't mark unsupported, just fail this call.
			return nil, err
		}
	}

	c.mu.Lock()
	c.wlanUnsupported = true
	c.mu.Unlock()
	return nil, errUniFiWLANUnsupported
}
