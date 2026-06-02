//go:build windows

package backend

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// wireguard.exe path — official WireGuard for Windows client
const wireguardExe = `C:\Program Files\WireGuard\wireguard.exe`

var activeTunnelConf string // path to the temp conf file
var activeExcludeRoutes []string

func applyWGConfig(conf string, turnIPs []string) error {
	teardownWG()

	addr, mtuStr, allowedIPs, _ := parseWGConfig(conf)
	if addr == "" {
		return fmt.Errorf("Address not found in wg config")
	}
	_ = allowedIPs // wireguard.exe handles AllowedIPs routing natively

	// Write config without PostUp/PreDown — we add routes manually below
	finalConf := injectWGQuickFields(conf, mtuStr, "", "")
	confPath := filepath.Join(os.TempDir(), wgIface+".conf")
	if err := os.WriteFile(confPath, []byte(finalConf), 0600); err != nil {
		return fmt.Errorf("write wg conf: %w", err)
	}
	activeTunnelConf = confPath

	// Add exclude routes BEFORE installing tunnel so VK API stays reachable
	gw := defaultGateway()
	if gw != "" {
		var excludeIPs []string
		for _, ip := range turnIPs {
			excludeIPs = append(excludeIPs, ip+"/32")
		}
		excludeIPs = append(excludeIPs, vkExcludeCIDRs...)
		for _, cidr := range excludeIPs {
			ip, mask, err := parseCIDR(cidr)
			if err != nil {
				continue
			}
			// Ignore errors — route may already exist
			_ = run("route", "add", ip, "mask", mask, gw)
		}
		activeExcludeRoutes = excludeIPs
	}

	// Install tunnel service
	if err := run(wireguardExe, "/installtunnelservice", confPath); err != nil {
		return fmt.Errorf("wireguard install tunnel: %w", err)
	}

	// Wait for interface to come up
	time.Sleep(2 * time.Second)
	log.Printf("[WG] Туннель %s поднят через wireguard.exe", wgIface)
	return nil
}

func teardownWG() {
	// Remove exclude routes first so VK API stays reachable during teardown
	for _, cidr := range activeExcludeRoutes {
		ip, _, _ := parseCIDR(cidr)
		if ip != "" {
			_ = run("route", "delete", ip)
		}
	}
	activeExcludeRoutes = nil

	if err := run(wireguardExe, "/uninstalltunnelservice", wgIface); err != nil {
		_ = err
	}
	if activeTunnelConf != "" {
		_ = os.Remove(activeTunnelConf)
		activeTunnelConf = ""
	}
}

// injectWGQuickFields adds PostUp/PreDown/MTU to the [Interface] section.
func injectWGQuickFields(conf, mtu, postUp, preDown string) string {
	var sb strings.Builder
	injected := false
	for _, line := range strings.Split(conf, "\n") {
		sb.WriteString(line + "\n")
		if !injected && strings.TrimSpace(line) == "[Interface]" {
			if mtu != "" {
				sb.WriteString("MTU = " + mtu + "\n")
			}
			if postUp != "" {
				sb.WriteString("PostUp = " + strings.TrimSuffix(strings.TrimSpace(postUp), "&") + "\n")
			}
			if preDown != "" {
				sb.WriteString("PreDown = " + strings.TrimSuffix(strings.TrimSpace(preDown), "&") + "\n")
			}
			injected = true
		}
	}
	return sb.String()
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w — %s", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func defaultGateway() string {
	// Use `route print` to find default gateway
	out, err := exec.Command("cmd", "/c", "route print 0.0.0.0").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// Format: Network Dest  Netmask  Gateway  Interface  Metric
		if len(fields) >= 3 && fields[0] == "0.0.0.0" && fields[1] == "0.0.0.0" {
			return fields[2]
		}
	}
	return ""
}

// parseCIDR converts "10.0.0.2/24" → ("10.0.0.2", "255.255.255.0", nil).
func parseCIDR(cidr string) (ip, mask string, err error) {
	parts := strings.SplitN(cidr, "/", 2)
	if len(parts) != 2 {
		return cidr, "255.255.255.255", nil
	}
	ip = parts[0]
	var prefix int
	if _, e := fmt.Sscanf(parts[1], "%d", &prefix); e != nil || prefix < 0 || prefix > 32 {
		return "", "", fmt.Errorf("invalid prefix %q", parts[1])
	}
	var m uint32
	if prefix > 0 {
		m = ^uint32(0) << (32 - prefix)
	}
	mask = fmt.Sprintf("%d.%d.%d.%d", m>>24, (m>>16)&0xff, (m>>8)&0xff, m&0xff)
	return ip, mask, nil
}
