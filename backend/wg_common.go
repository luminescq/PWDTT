package backend

import (
	"bufio"
	"strings"
)

const wgIface = "wg-turn"

// vkExcludeCIDRs — подсети которые должны идти напрямую, а не через туннель.
var vkExcludeCIDRs = []string{
	"87.240.128.0/18",  // VK
	"87.240.192.0/19",  // VK
	"90.156.0.0/16",    // VK TURN (90.156.234.x, 90.156.236.x и др.)
	"93.186.224.0/21",  // VK
	"95.142.192.0/21",  // VK
	"95.163.0.0/16",    // VK TURN (95.163.34.x и др.)
	"95.213.0.0/18",    // VK (id.vk.ru, login.vk.com)
	"155.212.192.0/20", // OK/VK (calls.okcdn.ru)
	"185.16.28.0/22",   // VK
	"194.67.64.0/18",   // VK
	"195.82.146.0/23",  // VK
	"213.180.193.0/24", // Яндекс DNS
	"77.88.0.0/18",     // Яндекс
	"8.8.8.0/24",       // Google DNS
	"1.1.1.0/24",       // Cloudflare DNS
}

// wg-quick-only fields that wg setconf doesn't understand
var wgQuickOnlyFields = map[string]bool{
	"address": true, "dns": true, "mtu": true,
	"preup": true, "postup": true, "predown": true, "postdown": true,
	"saveconfig": true,
}

// parseWGConfig extracts Address, MTU, AllowedIPs and returns a wg-setconf-compatible config.
func parseWGConfig(conf string) (addr, mtu string, allowedIPs []string, wgConf string) {
	var out strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(conf))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])
			switch key {
			case "address":
				addr = val
				continue
			case "mtu":
				mtu = val
				continue
			case "allowedips":
				for _, cidr := range strings.Split(val, ",") {
					if c := strings.TrimSpace(cidr); c != "" {
						allowedIPs = append(allowedIPs, c)
					}
				}
			default:
				if wgQuickOnlyFields[key] {
					continue
				}
			}
		}
		out.WriteString(line + "\n")
	}
	wgConf = out.String()
	return
}
