package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeOUIPrefix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"AB", ""},                   // too short
		{"00:11:22", "00:11:22"},
		{"00-11-22", "00:11:22"},     // dashes
		{"00:11:22:33:44:55", "00:11:22"}, // full MAC truncated to OUI
		{"aa:bb:cc:dd:ee:ff", "AA:BB:CC"}, // lowercase upcased
		{"   00:11:22  ", "00:11:22"},     // whitespace
		{"001122334455", "00:11:22"},      // no separators
		{"00:11", ""},                     // too short after dedupe
	}
	for _, c := range cases {
		if got := normalizeOUIPrefix(c.in); got != c.want {
			t.Errorf("normalizeOUIPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLookupVendor(t *testing.T) {
	o := &OUILookup{
		ouiMap: map[string]string{
			"AA:BB:CC": "Acme Corp",
		},
		loaded: true,
	}

	cases := []struct {
		mac, want string
	}{
		{"AA:BB:CC:11:22:33", "Acme Corp"},
		{"aa:bb:cc:11:22:33", "Acme Corp"}, // case-insensitive
		{"AA-BB-CC-11-22-33", "Acme Corp"}, // dash separator
		{"DE:AD:BE:EF:00:00", "Unknown"},   // not in map
		{"", "Unknown"},                    // malformed
		{"AA:BB", "Unknown"},               // too short
	}
	for _, c := range cases {
		if got := o.LookupVendor(c.mac); got != c.want {
			t.Errorf("LookupVendor(%q) = %q, want %q", c.mac, got, c.want)
		}
	}
}

// TestLookupVendorNotLoaded verifies that lookups before the database has
// loaded return "Unknown" instead of an inconsistent answer from a partially
// populated map. This is the invariant the C4 async-load fix relies on.
func TestLookupVendorNotLoaded(t *testing.T) {
	o := &OUILookup{
		ouiMap: map[string]string{"AA:BB:CC": "Acme Corp"},
		loaded: false, // simulate mid-load state
	}
	if got := o.LookupVendor("AA:BB:CC:11:22:33"); got != "Unknown" {
		t.Errorf("LookupVendor before loaded = %q, want \"Unknown\"", got)
	}
}

func TestLoadOUIMapFromFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid CSV", func(t *testing.T) {
		path := filepath.Join(dir, "oui.csv")
		content := "Mac Prefix,Vendor Name,Private,Block Type,Last Updated\n" +
			"00:11:22,Acme Corp,false,MA-L,2024-01-01\n" +
			"AA-BB-CC,Beta Networks,false,MA-L,2024-01-01\n" +
			"DD:EE:FF:00:11:22,Gamma Inc,false,MA-L,2024-01-01\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		m, err := loadOUIMapFromFile(path)
		if err != nil {
			t.Fatalf("loadOUIMapFromFile error: %v", err)
		}
		want := map[string]string{
			"00:11:22": "Acme Corp",
			"AA:BB:CC": "Beta Networks",
			"DD:EE:FF": "Gamma Inc",
		}
		for k, v := range want {
			if got := m[k]; got != v {
				t.Errorf("m[%q] = %q, want %q", k, got, v)
			}
		}
		if len(m) != len(want) {
			t.Errorf("loaded %d entries, want %d", len(m), len(want))
		}
	})

	t.Run("empty file errors", func(t *testing.T) {
		path := filepath.Join(dir, "empty.csv")
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadOUIMapFromFile(path); err == nil {
			t.Error("loadOUIMapFromFile on empty file = nil error, want non-nil")
		}
	})

	t.Run("missing file errors", func(t *testing.T) {
		if _, err := loadOUIMapFromFile(filepath.Join(dir, "does-not-exist.csv")); err == nil {
			t.Error("loadOUIMapFromFile on missing file = nil error, want non-nil")
		}
	})
}

// TestMinimalOUIsSeed verifies the embedded minimal database contains entries
// for the major WiFi vendors so vendor-resolution works before the async
// download completes.
func TestMinimalOUIsSeed(t *testing.T) {
	wantVendors := []string{
		"Ubiquiti Networks",
		"Cisco Systems",
		"TP-Link",
		"Netgear",
		"Aruba Networks",
		"Apple",
		"MikroTik",
	}
	have := make(map[string]bool, len(minimalOUIs))
	for _, v := range minimalOUIs {
		have[v] = true
	}
	for _, want := range wantVendors {
		if !have[want] {
			t.Errorf("minimalOUIs missing entries for %q", want)
		}
	}
}
