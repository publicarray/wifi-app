//go:build windows

package main

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/sys/windows"
)

func defaultGateway() (net.IP, error) {
	slog.Debug("gateway_windows: starting gateway resolution")

	cmd := exec.Command("route", "print", "0.0.0.0")
	cmd.SysProcAttr = &windows.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("gateway_windows: route print failed", "err", err)
		return nil, fmt.Errorf("route print: %w", err)
	}

	ip, err := parseWindowsGateway(output)
	if err != nil {
		slog.Warn("gateway_windows: no default gateway found", "err", err)
		return nil, err
	}

	slog.Info("gateway_windows: found gateway", "ip", ip.String())
	return ip, nil
}

func parseWindowsGateway(output []byte) (net.IP, error) {
	lines := strings.Split(string(output), "\n")
	var defaultGateway string
	sep := 0
	ipRegex := regexp.MustCompile(`^(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4})`)

	for idx, line := range lines {
		if sep == 3 {
			if len(lines) <= idx+2 {
				return nil, fmt.Errorf("no gateway")
			}

			inputLine := lines[idx+2]
			if strings.HasPrefix(inputLine, "=======") {
				break
			}

			fields := strings.Fields(inputLine)
			if len(fields) < 5 || !ipRegex.MatchString(fields[0]) {
				return nil, fmt.Errorf("parse error")
			}

			if fields[0] != "0.0.0.0" {
				break
			}

			gateway := fields[2]
			if len(gateway) > 0 && !unicode.IsLetter(rune(gateway[0])) {
				metric, _ := strconv.Atoi(fields[4])
				defaultGateway = gateway
				_ = metric
				break
			}
		}
		if strings.HasPrefix(line, "=======") {
			sep++
		}
	}

	if defaultGateway == "" {
		return nil, fmt.Errorf("no gateway")
	}

	return net.ParseIP(defaultGateway), nil
}
