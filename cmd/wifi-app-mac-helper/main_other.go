//go:build !darwin || !cgo

// Stub so `go build ./...` succeeds on non-darwin or non-cgo builds. The
// helper only exists to be invoked from a darwin/cgo runtime; on any other
// target it just prints a message and exits non-zero.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "wifi-app-mac-helper: this binary is only meaningful on macOS with cgo")
	os.Exit(1)
}
