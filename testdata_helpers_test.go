package main

import (
	"os"
	"path/filepath"
	"testing"
)

// mustReadFixtureAirport reads a file under testdata/. Lives in a
// build-tag-free file so it's available to every test in the package
// regardless of build tags.
func mustReadFixtureAirport(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", path))
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

// contains reports whether haystack contains needle. Tiny helper used by
// several parser tests when asserting capability/cipher/auth slices.
func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
