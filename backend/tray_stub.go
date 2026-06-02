//go:build !linux && !windows

package backend

func startTray(_ []byte, _, _ func()) {}
func setTrayVisible(_ bool)           {}
