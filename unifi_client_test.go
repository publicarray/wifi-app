package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const testAPIKey = "test-key-123"

// newUniFiTestServer serves a minimal Integration API under basePath with
// nDevices devices and nClients wireless clients, all requiring the X-API-KEY
// header. Uses TLS with a self-signed cert, mirroring a real controller.
func newUniFiTestServer(t *testing.T, basePath string, nDevices, nClients int) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-API-KEY") != testAPIKey {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}

	paginate := func(w http.ResponseWriter, r *http.Request, total int, item func(i int) any) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 25
		}
		var data []any
		for i := offset; i < total && i < offset+limit; i++ {
			data = append(data, item(i))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"offset": offset, "limit": limit,
			"count": len(data), "totalCount": total,
			"data": data,
		})
	}

	mux.HandleFunc(basePath+"/info", auth(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"applicationVersion": "9.4.1"})
	}))
	mux.HandleFunc(basePath+"/sites", auth(func(w http.ResponseWriter, r *http.Request) {
		paginate(w, r, 1, func(i int) any {
			return map[string]any{"id": "site-1", "name": "Default"}
		})
	}))
	mux.HandleFunc(basePath+"/sites/site-1/devices", auth(func(w http.ResponseWriter, r *http.Request) {
		paginate(w, r, nDevices, func(i int) any {
			return map[string]any{
				"id":         fmt.Sprintf("dev-%d", i),
				"name":       fmt.Sprintf("Office AP %d", i),
				"model":      "U6-Pro",
				"macAddress": fmt.Sprintf("68:D7:9A:00:00:%02X", i),
				"ipAddress":  fmt.Sprintf("192.168.1.%d", 10+i),
				"state":      "ONLINE",
			}
		})
	}))
	mux.HandleFunc(basePath+"/sites/site-1/wlans", auth(func(w http.ResponseWriter, r *http.Request) {
		paginate(w, r, 2, func(i int) any {
			return map[string]any{
				"id":     fmt.Sprintf("wlan-%d", i),
				"name":   []string{"Main", "IoT"}[i],
				"hidden": i == 1,
			}
		})
	}))
	mux.HandleFunc(basePath+"/sites/site-1/clients", auth(func(w http.ResponseWriter, r *http.Request) {
		paginate(w, r, nClients, func(i int) any {
			return map[string]any{
				"id":             fmt.Sprintf("client-%d", i),
				"name":           fmt.Sprintf("laptop-%d", i),
				"macAddress":     fmt.Sprintf("aa:bb:cc:00:00:%02x", i),
				"type":           "WIRELESS",
				"uplinkDeviceId": fmt.Sprintf("dev-%d", i%nDevicesOr1(nDevices)),
			}
		})
	}))

	return httptest.NewTLSServer(mux)
}

func nDevicesOr1(n int) int {
	if n == 0 {
		return 1
	}
	return n
}

func TestUniFiClient_DevicesAndPagination(t *testing.T) {
	// 450 devices forces three pages at the client's 200-per-page limit.
	srv := newUniFiTestServer(t, "/proxy/network/integration/v1", 450, 3)
	defer srv.Close()

	c := newUniFiClient(srv.URL, testAPIKey, true)
	devices, err := c.Devices(context.Background(), "site-1")
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devices) != 450 {
		t.Fatalf("got %d devices, want 450", len(devices))
	}
	if devices[0].Name != "Office AP 0" || devices[0].Model != "U6-Pro" {
		t.Errorf("unexpected first device: %+v", devices[0])
	}
}

func TestUniFiClient_BasePathFallback(t *testing.T) {
	// Self-hosted layout: API under /integration/v1, /proxy/... returns 404.
	srv := newUniFiTestServer(t, "/integration/v1", 2, 0)
	defer srv.Close()

	c := newUniFiClient(srv.URL, testAPIKey, true)
	sites, err := c.Sites(context.Background())
	if err != nil {
		t.Fatalf("Sites: %v", err)
	}
	if len(sites) != 1 || sites[0].ID != "site-1" {
		t.Fatalf("unexpected sites: %+v", sites)
	}
}

func TestUniFiClient_BadAPIKey(t *testing.T) {
	srv := newUniFiTestServer(t, "/proxy/network/integration/v1", 1, 0)
	defer srv.Close()

	c := newUniFiClient(srv.URL, "wrong-key", true)
	_, err := c.Sites(context.Background())
	if err == nil || !strings.Contains(err.Error(), "API key rejected") {
		t.Fatalf("expected API-key-rejected error, got: %v", err)
	}
}

func TestUniFiClient_TLSVerificationRespected(t *testing.T) {
	// The httptest server uses a self-signed certificate. With insecure TLS
	// disabled the client must refuse the connection.
	srv := newUniFiTestServer(t, "/proxy/network/integration/v1", 1, 0)
	defer srv.Close()

	c := newUniFiClient(srv.URL, testAPIKey, false)
	if _, err := c.Sites(context.Background()); err == nil {
		t.Fatalf("expected TLS verification failure against self-signed cert")
	}
}

func TestUniFiClient_WLANs(t *testing.T) {
	srv := newUniFiTestServer(t, "/proxy/network/integration/v1", 1, 0)
	defer srv.Close()

	c := newUniFiClient(srv.URL, testAPIKey, true)
	wlans, err := c.WLANs(context.Background(), "site-1")
	if err != nil {
		t.Fatalf("WLANs: %v", err)
	}
	if len(wlans) != 2 || wlans[1].Name != "IoT" || !wlans[1].Hidden {
		t.Fatalf("unexpected wlans: %+v", wlans)
	}
	if pickHiddenWLANName(wlans) != "IoT" {
		t.Errorf("pickHiddenWLANName = %q, want IoT", pickHiddenWLANName(wlans))
	}
}

func TestUniFiClient_WLANsUnsupported(t *testing.T) {
	// A server without any WLAN endpoint: the client must probe once, mark
	// the feature unsupported, and keep returning the sentinel without
	// re-probing (other endpoints stay functional).
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/integration/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != testAPIKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"offset": 0, "limit": 25, "count": 1, "totalCount": 1,
			"data": []any{map[string]any{"id": "site-1", "name": "Default"}},
		})
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	c := newUniFiClient(srv.URL, testAPIKey, true)
	for i := 0; i < 2; i++ {
		if _, err := c.WLANs(context.Background(), "site-1"); err != errUniFiWLANUnsupported {
			t.Fatalf("attempt %d: err = %v, want errUniFiWLANUnsupported", i, err)
		}
	}
	if _, err := c.Sites(context.Background()); err != nil {
		t.Fatalf("Sites should still work after WLAN probe: %v", err)
	}
}

func TestUniFiClient_Info(t *testing.T) {
	srv := newUniFiTestServer(t, "/proxy/network/integration/v1", 1, 0)
	defer srv.Close()

	c := newUniFiClient(srv.URL, testAPIKey, true)
	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.ApplicationVersion != "9.4.1" {
		t.Errorf("ApplicationVersion = %q, want 9.4.1", info.ApplicationVersion)
	}
}
