package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const runKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

func setAutoStart(v bool) error {
	switch runtime.GOOS {
	case "linux":
		return setAutoStartLinux(v)
	case "windows":
		return setAutoStartWindows(v)
	case "darwin":
		return setAutoStartDarwin(v)
	default:
		return fmt.Errorf("unsupported: %s", runtime.GOOS)
	}
}

const launchAgentLabel = "com.pwdtt.autostart"

func setAutoStartDarwin(v bool) error {
	dir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	path := filepath.Join(dir, launchAgentLabel+".plist")
	if !v {
		os.Remove(path)
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// .app: .../PWDTT.app/Contents/MacOS/pwdtt → запускаем через open,
	// чтобы поднялся именно бандл, а не голый бинарник
	appPath := exe
	if idx := strings.Index(exe, ".app/Contents/MacOS/"); idx != -1 {
		appPath = exe[:idx+len(".app")]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/bin/open</string>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, launchAgentLabel, appPath)
	return os.WriteFile(path, []byte(content), 0o644)
}

func setAutoStartLinux(v bool) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Join(os.Getenv("HOME"), ".config", "autostart")
	path := filepath.Join(dir, "pwdtt.desktop")
	if !v {
		os.Remove(path)
		return nil
	}
	os.MkdirAll(dir, 0o755)
	content := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=PWDTT\nExec=%s\nX-GNOME-Autostart-enabled=true\n", execPath)
	return os.WriteFile(path, []byte(content), 0o644)
}

func setAutoStartWindows(v bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	
	taskName := "PWDTT_AutoStart"
	
	if !v {
		// Remove scheduled task
		cmd := exec.Command("schtasks", "/delete", "/tn", taskName, "/f")
		hideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil && !strings.Contains(string(out), "не удается найти") && !strings.Contains(strings.ToLower(string(out)), "cannot find") {
			return fmt.Errorf("schtasks delete: %w — %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	
	// Create scheduled task to run on user logon with highest privileges (admin)
	// This prevents the UAC prompt on every system startup.
	cmd := exec.Command("schtasks", "/create", "/tn", taskName, "/tr", fmt.Sprintf("\"%s\"", exe), "/sc", "onlogon", "/rl", "HIGHEST", "/f")
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks create: %w — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetAutoStartLinux — exported wrapper for testing.
func SetAutoStartLinux(v bool) error {
	return setAutoStartLinux(v)
}
