//go:build linux

package backend

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var (
	activeRoutes   []string
	activeRoutesMu sync.Mutex
)

func applyWGConfig(conf string, turnIPs []string) error {
	addr, mtu, allowedIPs, wgConf := parseWGConfig(conf)
	if addr == "" {
		return fmt.Errorf("Address not found in wg config")
	}

	// Проверяем доступность sudo без пароля
	if err := exec.Command("sudo", "-n", "true").Run(); err != nil {
		return fmt.Errorf("sudo недоступен (нет прав или требует пароль): %w", err)
	}

	tmp, err := os.CreateTemp("", "wg-turn-*.conf")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(wgConf); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	// Делаем файл читаемым для root (sudo)
	_ = os.Chmod(tmpName, 0644)

	// Создаём интерфейс; если уже существует — не ошибка
	if err := run("ip", "link", "add", wgIface, "type", "wireguard"); err != nil {
		if !strings.Contains(err.Error(), "File exists") {
			return fmt.Errorf("ip link add: %w", err)
		}
	}

	if err := run("wg", "setconf", wgIface, tmpName); err != nil {
		return fmt.Errorf("wg setconf: %w", err)
	}

	_ = run("ip", "addr", "flush", "dev", wgIface)
	if err := run("ip", "addr", "add", addr, "dev", wgIface); err != nil {
		return fmt.Errorf("ip addr add: %w", err)
	}
	if mtu != "" {
		_ = run("ip", "link", "set", wgIface, "mtu", mtu)
	}
	if err := run("ip", "link", "set", wgIface, "up"); err != nil {
		return fmt.Errorf("ip link set up: %w", err)
	}

	var routes []string
	gw := defaultGateway()
	if gw != "" {
		for _, ip := range turnIPs {
			cidr := ip + "/32"
			if run("ip", "route", "add", cidr, "via", gw) == nil {
				routes = append(routes, cidr)
			}
		}
		for _, cidr := range vkExcludeCIDRs {
			if run("ip", "route", "add", cidr, "via", gw) == nil {
				routes = append(routes, cidr)
			}
		}
		for _, dns := range localDNSServers() {
			cidr := dns + "/32"
			if run("ip", "route", "add", cidr, "via", gw) == nil {
				routes = append(routes, cidr)
			}
		}
	}
	for _, cidr := range allowedIPs {
		if run("ip", "route", "add", cidr, "dev", wgIface) == nil {
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
		if strings.HasPrefix(entry, "dev:") {
			cidr := strings.TrimPrefix(entry, "dev:")
			_ = run("ip", "route", "del", cidr, "dev", wgIface)
		} else {
			_ = run("ip", "route", "del", entry)
		}
	}
	_ = run("ip", "link", "del", wgIface)
}

func run(name string, args ...string) error {
	cmd := exec.Command("sudo", append([]string{"-n", name}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w — %s", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func defaultGateway() string {
	cmd := exec.Command("ip", "route", "show", "default")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "via" && i+1 < len(fields) {
			return fields[i+1]
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
