//go:build darwin

package backend

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

var (
	activeDevice    *device.Device
	activeTun       tun.Device
	activeIfaceName string
	activeRoutes    []string
	activeRoutesMu  sync.Mutex
)

func applyWGConfig(conf string, turnIPs []string) error {
	teardownWG()

	if err := exec.Command("sudo", "-n", "true").Run(); err != nil {
		if err := runWithAuth("true"); err != nil {
			return fmt.Errorf("sudo недоступен: %w", err)
		}
	}

	addr, mtuStr, allowedIPs, wgConf := parseWGConfig(conf)
	if addr == "" {
		return fmt.Errorf("Address not found in wg config")
	}

	mtu := 1300
	if mtuStr != "" {
		fmt.Sscanf(mtuStr, "%d", &mtu)
	}

	// "utun" lets the kernel pick the next free utunN device.
	tunDev, err := tun.CreateTUN("utun", mtu)
	if err != nil {
		return fmt.Errorf("create TUN: %w", err)
	}
	activeTun = tunDev

	ifaceName, err := tunDev.Name()
	if err != nil {
		tunDev.Close()
		activeTun = nil
		return fmt.Errorf("tun name: %w", err)
	}
	activeIfaceName = ifaceName

	logger := &device.Logger{
		Verbosef: func(format string, args ...interface{}) {},
		Errorf:   func(format string, args ...interface{}) { log.Printf("[WG] "+format, args...) },
	}
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), logger)
	activeDevice = dev

	if err := dev.IpcSetOperation(strings.NewReader(uapiConf(wgConf))); err != nil {
		teardownWG()
		return fmt.Errorf("IpcSet: %w", err)
	}

	if err := dev.Up(); err != nil {
		teardownWG()
		return fmt.Errorf("device up: %w", err)
	}

	ip, mask, err := parseCIDR(addr)
	if err != nil {
		teardownWG()
		return fmt.Errorf("parse address: %w", err)
	}
	if err := run("ifconfig", ifaceName, "inet", ip, ip, "netmask", mask); err != nil {
		teardownWG()
		return fmt.Errorf("ifconfig addr: %w", err)
	}
	if err := run("ifconfig", ifaceName, "up"); err != nil {
		teardownWG()
		return fmt.Errorf("ifconfig up: %w", err)
	}

	var routes []string
	gw := defaultGateway()
	if gw != "" {
		for _, ip := range turnIPs {
			if run("route", "add", "-host", ip, gw) == nil {
				routes = append(routes, "host:"+ip)
			}
		}
		for _, cidr := range vkExcludeCIDRs {
			if run("route", "add", "-net", cidr, gw) == nil {
				routes = append(routes, "net:"+cidr)
			}
		}
		for _, dns := range localDNSServers() {
			if run("route", "add", "-host", dns, gw) == nil {
				routes = append(routes, "host:"+dns)
			}
		}
	}
	for _, cidr := range allowedIPs {
		if run("route", "add", "-net", cidr, "-interface", ifaceName) == nil {
			routes = append(routes, "dev:"+cidr)
		}
	}

	activeRoutesMu.Lock()
	activeRoutes = routes
	activeRoutesMu.Unlock()
	return nil
}

func teardownWG() {
	activeRoutesMu.Lock()
	routes := activeRoutes
	activeRoutes = nil
	activeRoutesMu.Unlock()

	for _, entry := range routes {
		switch {
		case strings.HasPrefix(entry, "host:"):
			_ = run("route", "delete", "-host", strings.TrimPrefix(entry, "host:"))
		case strings.HasPrefix(entry, "net:"):
			_ = run("route", "delete", "-net", strings.TrimPrefix(entry, "net:"))
		case strings.HasPrefix(entry, "dev:"):
			_ = run("route", "delete", "-net", strings.TrimPrefix(entry, "dev:"))
		}
	}

	if activeDevice != nil {
		activeDevice.Close()
		activeDevice = nil
	}
	if activeTun != nil {
		activeTun.Close()
		activeTun = nil
	}
	activeIfaceName = ""
}

func run(name string, args ...string) error {
	cmd := exec.Command("sudo", append([]string{"-n", name}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isPasswordRequired(out) {
			return runWithAuth(name, args...)
		}
		return fmt.Errorf("%s %v: %w — %s", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isPasswordRequired(output []byte) bool {
	s := string(output)
	return strings.Contains(s, "a password is required") ||
		strings.Contains(s, "not in the sudoers") ||
		strings.Contains(s, "try again")
}

func runWithAuth(name string, args ...string) error {
	cmdArgs := name
	for _, a := range args {
		cmdArgs += " " + shellEscape(a)
	}
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, cmdArgs)
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w — %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func shellEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func defaultGateway() string {
	cmd := exec.Command("route", "-n", "get", "default")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
		}
	}
	return ""
}

func localDNSServers() []string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	var result []string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "nameserver" {
			continue
		}
		ip := net.ParseIP(fields[1])
		// Пропускаем loopback (127.x.x.x, ::1) — маршрут на него бессмысленен
		if ip == nil || ip.IsLoopback() {
			continue
		}
		result = append(result, fields[1])
	}
	return result
}

// uapiConf converts a wg-setconf-compatible config (with [Interface]/[Peer] sections)
// into the UAPI protocol format expected by device.IpcSetOperation.
func uapiConf(wgConf string) string {
	var sb strings.Builder
	inPeer := false
	for _, line := range strings.Split(wgConf, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "[Interface]" {
			inPeer = false
			continue
		}
		if trimmed == "[Peer]" {
			if inPeer {
				sb.WriteString("\n")
			}
			inPeer = true
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "privatekey":
			sb.WriteString("private_key=" + toHex(val) + "\n")
		case "listenport":
			sb.WriteString("listen_port=" + val + "\n")
		case "publickey":
			sb.WriteString("public_key=" + toHex(val) + "\n")
		case "presharedkey":
			sb.WriteString("preshared_key=" + toHex(val) + "\n")
		case "endpoint":
			sb.WriteString("endpoint=" + val + "\n")
		case "allowedips":
			for _, cidr := range strings.Split(val, ",") {
				if c := strings.TrimSpace(cidr); c != "" {
					sb.WriteString("allowed_ip=" + c + "\n")
				}
			}
		case "persistentkeepalive":
			sb.WriteString("persistent_keepalive_interval=" + val + "\n")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// toHex converts a base64-encoded WireGuard key to lowercase hex.
func toHex(b64 string) string {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return b64 // already hex or garbage — return as-is
	}
	return hex.EncodeToString(raw)
}

// parseCIDR converts "10.0.0.2/24" → ("10.0.0.2", "255.255.255.0").
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
