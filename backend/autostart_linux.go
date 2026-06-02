//go:build linux

package backend

import (
	"os"
	"path/filepath"
)

func (a *App) SetAutoStart(v bool) error {
	exec, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Join(os.Getenv("HOME"), ".config", "autostart")
	path := filepath.Join(dir, "pwdtt.desktop")
	if !v {
		_ = os.Remove(path)
		return nil
	}
	_ = os.MkdirAll(dir, 0755)
	content := "[Desktop Entry]\nType=Application\nName=PWDTT\nExec=" + exec + "\nX-GNOME-Autostart-enabled=true\n"
	return os.WriteFile(path, []byte(content), 0644)
}

func (a *App) GetAutoStart() bool {
	path := filepath.Join(os.Getenv("HOME"), ".config", "autostart", "pwdtt.desktop")
	_, err := os.Stat(path)
	return err == nil
}
