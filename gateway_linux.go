//go:build linux

package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// defaultGateway returns the system's IPv4 default gateway by parsing
// /proc/net/route. The kernel writes gateway addresses as hex in host (LE)
// byte order, so we decode them accordingly.
//
// Returns an error if no default route exists (uncommon on a WiFi client, but
// possible on an ethernet-less dev box with no DHCP lease).
func defaultGateway() (net.IP, error) {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return nil, fmt.Errorf("open /proc/net/route: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Skip header.
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty /proc/net/route")
	}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		// Destination must be 0.0.0.0 for a default route.
		if fields[1] != "00000000" {
			continue
		}
		gwHex := fields[2]
		if len(gwHex) != 8 {
			continue
		}
		raw, err := strconv.ParseUint(gwHex, 16, 32)
		if err != nil {
			continue
		}
		// /proc/net/route reports the gateway in little-endian host order.
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], uint32(raw))
		return net.IPv4(buf[0], buf[1], buf[2], buf[3]).To4(), nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read /proc/net/route: %w", err)
	}
	return nil, fmt.Errorf("no default route")
}
