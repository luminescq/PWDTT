package backend_test

import (
	"os"
	"path/filepath"
	"testing"

	"pwdtt/backend"
)

// newTestStore создаёт Store с временной директорией.
func newTestStore(t *testing.T) *backend.Store {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// os.UserConfigDir() на Windows читает %AppData%, на Linux — XDG_CONFIG_HOME:
	// без подмены тесты пишут в РЕАЛЬНЫЙ конфиг пользователя (%APPDATA%\pwdtt)
	t.Setenv("AppData", filepath.Join(dir, "AppData", "Roaming"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	return backend.NewStore()
}

// pwdttDir — зеркало configDir() из store.go: где Store реально держит файлы.
func pwdttDir(t *testing.T) string {
	t.Helper()
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.Getenv("HOME")
	}
	return filepath.Join(base, "pwdtt")
}

// ═══════════════════════════════════════════════════
// LoadSettings
// ═══════════════════════════════════════════════════

func TestLoadSettings_FileNotExists(t *testing.T) {
	s := newTestStore(t)
	settings := s.LoadSettings()

	if !settings.AutoStart {
		t.Error("expected AutoStart=true by default")
	}
	if settings.ObfsMode != "audio" {
		t.Errorf("expected ObfsMode=audio by default, got %q", settings.ObfsMode)
	}
}

func TestLoadSettings_InvalidJSON(t *testing.T) {
	s := newTestStore(t)
	path := filepath.Join(pwdttDir(t), "config.json")
	os.WriteFile(path, []byte(`{invalid json`), 0o644)

	settings := s.LoadSettings()

	if !settings.AutoStart {
		t.Error("expected AutoStart=true on invalid JSON")
	}
	if settings.ObfsMode != "audio" {
		t.Errorf("expected ObfsMode=audio on invalid JSON, got %q", settings.ObfsMode)
	}
}

func TestLoadSaveSettings_Roundtrip(t *testing.T) {
	s := newTestStore(t)

	// ObfsAccepted=true, иначе LoadSettings принудительно вернёт "audio"
	original := backend.AppSettings{AutoStart: false, ObfsMode: "video", ObfsAccepted: true}
	if err := s.SaveSettings(original); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	loaded := s.LoadSettings()
	if loaded.AutoStart != original.AutoStart {
		t.Errorf("AutoStart: got %v, want %v", loaded.AutoStart, original.AutoStart)
	}
	if loaded.ObfsMode != original.ObfsMode {
		t.Errorf("ObfsMode: got %q, want %q", loaded.ObfsMode, original.ObfsMode)
	}
}

func TestLoadSettings_EmptyFile(t *testing.T) {
	s := newTestStore(t)
	path := filepath.Join(pwdttDir(t), "config.json")
	os.WriteFile(path, []byte{}, 0o644)

	settings := s.LoadSettings()
	if !settings.AutoStart {
		t.Error("expected default AutoStart on empty file")
	}
}

func TestLoadSettings_UnknownFields(t *testing.T) {
	s := newTestStore(t)
	path := filepath.Join(pwdttDir(t), "config.json")
	os.WriteFile(path, []byte(`{"unknownField": true}`), 0o644)

	settings := s.LoadSettings()
	// json.Unmarshal с unknown полями не падает — ставит zero values
	if settings.AutoStart != false {
		t.Errorf("expected AutoStart=false (zero value), got %v", settings.AutoStart)
	}
	// obfsAccepted=false → LoadSettings принудительно ставит безопасный "audio"
	if settings.ObfsMode != "audio" {
		t.Errorf("expected ObfsMode='audio' (obfsAccepted=false), got %q", settings.ObfsMode)
	}
}

// ═══════════════════════════════════════════════════
// SaveProfile / LoadProfile
// ═══════════════════════════════════════════════════

func TestSaveProfile_GeneratesDeviceID(t *testing.T) {
	s := newTestStore(t)

	p := backend.ProfileData{PeerAddr: "1.2.3.4:5555", Password: "secret"}
	if err := s.SaveProfile("test", p); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	loaded, err := s.LoadProfile("test")
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if loaded.DeviceID == "" {
		t.Error("expected DeviceID to be generated")
	}
	if loaded.PeerAddr != "1.2.3.4:5555" {
		t.Errorf("PeerAddr: got %q, want %q", loaded.PeerAddr, "1.2.3.4:5555")
	}
}

func TestSaveProfile_PreservesExistingDeviceID(t *testing.T) {
	s := newTestStore(t)

	p1 := backend.ProfileData{PeerAddr: "1.2.3.4:5555", DeviceID: "my-custom-id"}
	s.SaveProfile("test", p1)

	// Перезапись — пустой DeviceID → должен сохранить существующий
	p2 := backend.ProfileData{PeerAddr: "5.6.7.8:9999"}
	s.SaveProfile("test", p2)

	loaded, err := s.LoadProfile("test")
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if loaded.DeviceID != "my-custom-id" {
		t.Errorf("expected preserved DeviceID, got %q", loaded.DeviceID)
	}
	if loaded.PeerAddr != "5.6.7.8:9999" {
		t.Errorf("PeerAddr not updated: got %q", loaded.PeerAddr)
	}
}

func TestLoadProfile_NotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.LoadProfile("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestLoadProfile_InvalidName(t *testing.T) {
	s := newTestStore(t)

	_, err := s.LoadProfile("")
	if err == nil {
		t.Error("expected error for empty name")
	}

	_, err = s.LoadProfile("../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal name")
	}
}

func TestSaveProfile_SanitizesName(t *testing.T) {
	s := newTestStore(t)

	p := backend.ProfileData{PeerAddr: "1.2.3.4:5555"}
	if err := s.SaveProfile("test@#$server", p); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	// Должен быть загружен как "testserver"
	loaded, err := s.LoadProfile("testserver")
	if err != nil {
		t.Fatalf("LoadProfile sanitized name: %v", err)
	}
	if loaded.PeerAddr != "1.2.3.4:5555" {
		t.Errorf("PeerAddr: got %q", loaded.PeerAddr)
	}
}

// ═══════════════════════════════════════════════════
// ListProfiles
// ═══════════════════════════════════════════════════

func TestListProfiles_EmptyDir(t *testing.T) {
	s := newTestStore(t)

	profiles := s.ListProfiles()
	if len(profiles) != 0 {
		t.Errorf("expected empty map, got %d profiles", len(profiles))
	}
}

func TestListProfiles_Multiple(t *testing.T) {
	s := newTestStore(t)

	s.SaveProfile("alpha", backend.ProfileData{PeerAddr: "1.1.1.1:1111"})
	s.SaveProfile("beta", backend.ProfileData{PeerAddr: "2.2.2.2:2222"})
	s.SaveProfile("gamma", backend.ProfileData{PeerAddr: "3.3.3.3:3333"})

	profiles := s.ListProfiles()
	if len(profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(profiles))
	}
	if profiles["alpha"].PeerAddr != "1.1.1.1:1111" {
		t.Errorf("alpha PeerAddr: got %q", profiles["alpha"].PeerAddr)
	}
	if profiles["beta"].PeerAddr != "2.2.2.2:2222" {
		t.Errorf("beta PeerAddr: got %q", profiles["beta"].PeerAddr)
	}
}

func TestListProfiles_SkipsNonJSON(t *testing.T) {
	s := newTestStore(t)
	serversDir := filepath.Join(pwdttDir(t), "servers")

	s.SaveProfile("valid", backend.ProfileData{PeerAddr: "1.1.1.1:1111"})
	os.WriteFile(filepath.Join(serversDir, "readme.txt"), []byte("hello"), 0o644)
	os.MkdirAll(filepath.Join(serversDir, "subdir"), 0o755)

	profiles := s.ListProfiles()
	if len(profiles) != 1 {
		t.Errorf("expected 1 profile (only valid JSON), got %d", len(profiles))
	}
}

// ═══════════════════════════════════════════════════
// DeleteProfile
// ═══════════════════════════════════════════════════

func TestDeleteProfile_Existing(t *testing.T) {
	s := newTestStore(t)
	s.SaveProfile("to-delete", backend.ProfileData{PeerAddr: "1.2.3.4:5555"})

	if err := s.DeleteProfile("to-delete"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}

	_, err := s.LoadProfile("to-delete")
	if err == nil {
		t.Error("expected profile to be deleted")
	}
}

func TestDeleteProfile_NotFound(t *testing.T) {
	s := newTestStore(t)

	err := s.DeleteProfile("ghost")
	if err == nil {
		t.Error("expected error deleting nonexistent profile")
	}
}

func TestDeleteProfile_EmptyName(t *testing.T) {
	s := newTestStore(t)

	err := s.DeleteProfile("")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

// ═══════════════════════════════════════════════════
// CreateLogFile
// ═══════════════════════════════════════════════════

func TestCreateLogFile(t *testing.T) {
	s := newTestStore(t)
	logsDir := filepath.Join(pwdttDir(t), "logs")

	f, err := s.CreateLogFile("1.2.3.4")
	if err != nil {
		t.Fatalf("CreateLogFile: %v", err)
	}
	defer f.Close()

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("ReadDir logs: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 log file, got %d", len(entries))
	}
	if filepath.Ext(entries[0].Name()) != ".log" {
		t.Errorf("expected .log extension, got %q", filepath.Ext(entries[0].Name()))
	}
}

func TestCreateLogFile_Writable(t *testing.T) {
	s := newTestStore(t)

	f, err := s.CreateLogFile("1.2.3.4")
	if err != nil {
		t.Fatalf("CreateLogFile: %v", err)
	}
	defer f.Close()

	n, err := f.Write([]byte("test log line\n"))
	if err != nil {
		t.Fatalf("Write to log file: %v", err)
	}
	if n != 14 {
		t.Errorf("expected 14 bytes written, got %d", n)
	}
}

// ═══════════════════════════════════════════════════
// OpenLogFile / LogFile.Write
// ═══════════════════════════════════════════════════

func TestOpenLogFile(t *testing.T) {
	s := newTestStore(t)
	logsDir := filepath.Join(pwdttDir(t), "logs")

	lf := s.OpenLogFile()
	if lf == nil {
		t.Fatal("OpenLogFile returned nil")
	}

	lf.Write("INFO", "test message")
	lf.Close()

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one log file")
	}

	data, _ := os.ReadFile(filepath.Join(logsDir, entries[0].Name()))
	content := string(data)
	if !strContains(content, "test message") {
		t.Errorf("log file missing expected content, got: %q", content)
	}
	if !strContains(content, "[INFO]") {
		t.Errorf("log file missing level tag, got: %q", content)
	}
}

func TestLogFile_NilSafe(t *testing.T) {
	var lf *backend.LogFile
	// Не должен паниковать
	lf.Write("INFO", "no crash")
	lf.Close()
}

// ═══════════════════════════════════════════════════
// ReadLatestLog
// ═══════════════════════════════════════════════════

func TestReadLatestLog_NoLogs(t *testing.T) {
	s := newTestStore(t)
	content := s.ReadLatestLog()
	if content != "" {
		t.Errorf("expected empty, got %q", content)
	}
}

func TestReadLatestLog_WithLogs(t *testing.T) {
	s := newTestStore(t)
	logDir := filepath.Join(pwdttDir(t), "logs")

	os.WriteFile(filepath.Join(logDir, "2025-01-01_10-00-00.log"), []byte("old log"), 0o644)
	os.WriteFile(filepath.Join(logDir, "2025-06-01_12-00-00.log"), []byte("latest log"), 0o644)

	content := s.ReadLatestLog()
	if content != "latest log" {
		t.Errorf("expected latest log, got %q", content)
	}
}

// ═══════════════════════════════════════════════════
// helpers
// ═══════════════════════════════════════════════════

func strContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Ensure types are used to suppress unused import warnings.
var (
	_ = backend.ProfileData{}
	_ = backend.AppSettings{}
)
