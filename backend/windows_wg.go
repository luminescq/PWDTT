//go:build windows

package backend

import (
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

// wintunDLL is set by InitWintun called from main_windows.go
var wintunDLL []byte

var (
	activeDevice        *device.Device
	activeTun           tun.Device
	activeExcludeRoutes []string
)

func InitWintun(dll []byte) { wintunDLL = dll }

// extractWintun writes the embedded wintun.dll next to the exe so the wintun
// package can load it via LoadLibrary.
func extractWintun() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dst := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if _, err := os.Stat(dst); err == nil {
		return nil // already extracted
	}
	return os.WriteFile(dst, wintunDLL, 0644)
}

func applyWGConfig(conf string, turnIPs []string) error {
	teardownWG()

	if err := extractWintun(); err != nil {
		return fmt.Errorf("extract wintun.dll: %w", err)
	}

	addr, mtuStr, allowedIPs, wgConf := parseWGConfig(conf)
	if addr == "" {
		return fmt.Errorf("Address not found in wg config")
	}

	mtu := 1300
	if mtuStr != "" {
		fmt.Sscanf(mtuStr, "%d", &mtu)
	}

	// Create wintun TUN interface
	tunDev, err := tun.CreateTUN(wgIface, mtu)
	if err != nil {
		return fmt.Errorf("create TUN: %w", err)
	}
	activeTun = tunDev

	// Create userspace WireGuard device
	logger := &device.Logger{
		Verbosef: func(format string, args ...interface{}) {},
		Errorf:   func(format string, args ...interface{}) { log.Printf("[WG] "+format, args...) },
	}
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), logger)
	activeDevice = dev

	if err := dev.IpcSetOperation(strings.NewReader(uapiConf(wgConf))); err != nil {
		return fmt.Errorf("IpcSet: %w", err)
	}

	if err := dev.Up(); err != nil {
		return fmt.Errorf("device up: %w", err)
	}

	// Set IP address on the interface
	if err := run("netsh", "interface", "ip", "set", "address",
		"name="+wgIface, "source=static", addr, "none"); err != nil {
		// addr may be CIDR — extract host part
		host, mask, _ := parseCIDR(addr)
		if host != "" {
			_ = run("netsh", "interface", "ip", "set", "address",
				"name="+wgIface, "source=static", host, mask)
		}
	}

	// Exclude routes BEFORE adding tunnel routes
	gw := defaultGateway()
	if gw != "" {
		var excludes []string
		for _, ip := range turnIPs {
			excludes = append(excludes, ip+"/32")
		}
		excludes = append(excludes, vkExcludeCIDRs...)
		for _, cidr := range excludes {
			ip, mask, err := parseCIDR(cidr)
			if err != nil {
				continue
			}
			_ = run("route", "add", ip, "mask", mask, gw)
		}
		activeExcludeRoutes = excludes
	}

	// Add AllowedIPs routes via the WG interface
	for _, cidr := range allowedIPs {
		_ = run("netsh", "interface", "ip", "add", "route", cidr, wgIface)
	}

	log.Printf("[WG] Туннель %s поднят (userspace)", wgIface)
	return nil
}

func teardownWG() {
	for _, cidr := range activeExcludeRoutes {
		ip, _, _ := parseCIDR(cidr)
		if ip != "" {
			_ = run("route", "delete", ip)
		}
	}
	activeExcludeRoutes = nil

	if activeDevice != nil {
		activeDevice.Close()
		activeDevice = nil
	}
	if activeTun != nil {
		activeTun.Close()
		activeTun = nil
	}
}

// uapiConf converts a wg-setconf-compatible config (with [Interface]/[Peer] sections)
// into the UAPI protocol format expected by device.IpcSetOperation.
//
// UAPI format: flat key=value, no section headers, hex keys, starts with "set=1\n",
// peers separated by a blank line.
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
				sb.WriteString("\n") // blank line separates peers
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
	sb.WriteString("\n") // final terminator
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

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w — %s", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func defaultGateway() string {
	cmd := exec.Command("cmd", "/c", "route print 0.0.0.0")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
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
