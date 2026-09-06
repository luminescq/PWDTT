package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// maxProfileNameRunes — предел длины имени профиля (имя = имя файла)
const maxProfileNameRunes = 100

// Store — пассивный модуль хранения данных.
// Все записи атомарные (write to tmp → rename).
// Не инициирует ничего — только отвечает на запросы.
//
// Структура файлов:
//
//	~/.config/pwdtt/config.json        — AppSettings
//	~/.config/pwdtt/servers/<name>.json — ProfileData (серверы)
//	~/.config/pwdtt/logs/*.log          — логи сессий
type Store struct {
	baseDir string // ~/.config/pwdtt
}

// NewStore создаёт Store и создаёт директории если нужно.
func NewStore() *Store {
	dir := configDir()
	os.MkdirAll(filepath.Join(dir, "servers"), 0o755)
	os.MkdirAll(filepath.Join(dir, "logs"), 0o755)
	return &Store{baseDir: dir}
}

// ═══════════════════════════════════════════════════
// SETTINGS — настройки приложения
// ═══════════════════════════════════════════════════

// LoadSettings загружает настройки из config.json.
// Если файла нет — возвращает дефолтные настройки.
// Если obfsAccepted = false — obfsMode принудительно 'audio'.
func (s *Store) LoadSettings() AppSettings {
	defaults := AppSettings{AutoStart: true, ObfsMode: "audio", ObfsAccepted: false}
	data, err := os.ReadFile(filepath.Join(s.baseDir, "config.json"))
	if err != nil {
		return defaults
	}
	var settings AppSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaults
	}
	if !settings.ObfsAccepted {
		settings.ObfsMode = "audio"
	}
	return settings
}

// SaveSettings сохраняет настройки в config.json (атомарно).
func (s *Store) SaveSettings(settings AppSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(s.baseDir, "config.json"), data)
}

// ═══════════════════════════════════════════════════
// PROFILES — CRUD для серверов
// ═══════════════════════════════════════════════════

// sanitizeProfileName валидирует имя профиля: буквы и цифры (включая кириллицу),
// дефисы, подчёркивания, пробелы. Символы, опасные для файловой системы
// (\/ : * ? " < > | и управляющие), вырезаются.
func sanitizeProfileName(name string) string {
	var out []rune
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_', r == ' ':
			out = append(out, r)
		}
	}
	if len(out) > maxProfileNameRunes {
		out = out[:maxProfileNameRunes]
	}
	return strings.TrimSpace(string(out))
}

// LoadProfile загружает профиль сервера по имени.
func (s *Store) LoadProfile(name string) (*ProfileData, error) {
	name = sanitizeProfileName(name)
	if name == "" {
		return nil, fmt.Errorf("invalid profile name")
	}
	path := filepath.Join(s.baseDir, "servers", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("profile %q: %w", name, err)
	}
	var p ProfileData
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("profile %q parse: %w", name, err)
	}
	return &p, nil
}

// SaveProfile сохраняет профиль сервера (атомарно).
func (s *Store) SaveProfile(name string, p ProfileData) error {
	name = sanitizeProfileName(name)
	if name == "" {
		return fmt.Errorf("invalid profile name")
	}
	if p.DeviceID == "" {
		if existing, err := s.LoadProfile(name); err == nil && existing.DeviceID != "" {
			p.DeviceID = existing.DeviceID
		} else {
			p.DeviceID = uuid.New().String()
		}
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(s.baseDir, "servers", name+".json"), data)
}

// DeleteProfile удаляет профиль сервера.
func (s *Store) DeleteProfile(name string) error {
	name = sanitizeProfileName(name)
	if name == "" {
		return fmt.Errorf("invalid profile name")
	}
	return os.Remove(filepath.Join(s.baseDir, "servers", name+".json"))
}

// ListProfiles возвращает все профили серверов.
func (s *Store) ListProfiles() map[string]ProfileData {
	dir := filepath.Join(s.baseDir, "servers")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	result := make(map[string]ProfileData)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var p ProfileData
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		result[name] = p
	}
	return result
}

// CreateLogFile создаёт файл лога для сессии.
func (s *Store) CreateLogFile(peerIP string) (*os.File, error) {
	dir := filepath.Join(s.baseDir, "logs")
	return os.CreateTemp(dir, "*.log")
}

// ═══════════════════════════════════════════════════
// LOG FILE — запись полных логов в файл
// ═══════════════════════════════════════════════════

type LogFile struct {
	file    *os.File
	pending int // буферизованные строки до sync
}

// OpenLogFile открывает новый файл лога для сессии.
func (s *Store) OpenLogFile() *LogFile {
	dir := filepath.Join(s.baseDir, "logs")
	os.MkdirAll(dir, 0o755)
	ts := time.Now().Format("2006-01-02_15-04-05")
	f, err := os.Create(filepath.Join(dir, ts+".log"))
	if err != nil {
		return nil
	}
	return &LogFile{file: f}
}

// Write пишет строку в файл лога.
// Sync каждые 50 строк чтобы не душить I/O.
func (lf *LogFile) Write(level, msg string) {
	if lf == nil || lf.file == nil {
		return
	}
	ts := time.Now().Format("15:04:05")
	fmt.Fprintf(lf.file, "[%s] [%s] %s\n", ts, level, msg)
	lf.pending++
	if lf.pending >= 50 {
		lf.file.Sync()
		lf.pending = 0
	}
}

// Close закрывает файл лога.
func (lf *LogFile) Close() {
	if lf != nil && lf.file != nil {
		lf.file.Sync()
		lf.file.Close()
	}
}

// ReadLatestLog читает содержимое последнего лог-файла.
func (s *Store) ReadLatestLog() string {
	dir := filepath.Join(s.baseDir, "logs")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return ""
	}
	// берём последний (сортировка по имени = по времени)
	latest := entries[len(entries)-1]
	data, err := os.ReadFile(filepath.Join(dir, latest.Name()))
	if err != nil {
		return ""
	}
	return string(data)
}

// ═══════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════

// configDir возвращает ~/.config/pwdtt
func configDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.Getenv("HOME")
	}
	return filepath.Join(base, "pwdtt")
}

// atomicWrite записывает файл атомарно: tmp → rename.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// NewTestStore создаёт Store с временной директорией (для тестов).
// Подменяет HOME и, на Windows, AppData — os.UserConfigDir() читает именно их,
// иначе тесты пишут в реальный конфиг пользователя.
func NewTestStore(t interface{ TempDir() string; Setenv(string, string) }) *Store {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("AppData", filepath.Join(dir, "AppData", "Roaming"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	return NewStore()
}
