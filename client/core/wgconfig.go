package core

import (
	"fmt"
	"strings"
)

func patchWGConfig(raw string, mtu int) string {
	if mtu <= 0 {
		mtu = 1300
	}
	mtuLine := fmt.Sprintf("MTU = %d", mtu)
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines)+2)
	inInterface := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[Interface]" {
			inInterface = true
		} else if strings.HasPrefix(trimmed, "[") {
			inInterface = false
		}
		if inInterface && strings.HasPrefix(trimmed, "DNS") {
			continue
		}
		if strings.HasPrefix(trimmed, "AllowedIPs") {
			if val := strings.SplitN(trimmed, "=", 2); len(val) == 2 {
				ips := strings.TrimSpace(val[1])
				if ips == "0.0.0.0/0" || ips == "0.0.0.0/0, ::/0" || ips == "0.0.0.0/0,::/0" {
					out = append(out, "AllowedIPs = 0.0.0.0/1, 128.0.0.0/1")
					continue
				}
			}
		}
		if trimmed == "[Interface]" {
			out = append(out, line, mtuLine)
			continue
		}
		if inInterface && strings.HasPrefix(trimmed, "MTU") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
