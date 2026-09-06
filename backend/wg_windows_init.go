//go:build windows

package backend

import (
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/tun"
)

func init() {
	// Фиксируем GUID для Wintun, чтобы избежать создания новых адаптеров.
	guid, _ := windows.GUIDFromString("{59B518DA-F85C-4DF7-BEE7-4B29E3063C43}")
	tun.WintunStaticRequestedGUID = &guid
}
