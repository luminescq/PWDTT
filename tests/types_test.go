package backend_test

import (
	"encoding/json"
	"testing"

	"pwdtt/backend"
)

// ═══════════════════════════════════════════════════
// ConnectParams — маршалинг/анмаршалинг
// ═══════════════════════════════════════════════════

func TestConnectParams_MarshalRoundtrip(t *testing.T) {
	p := backend.ConnectParams{
		PeerAddr:    "1.2.3.4:5555",
		Password:    "secret",
		Hashes:      []string{"abc", "def"},
		DeviceID:    "device-123",
		Workers:     9,
		CaptchaMode: "auto",
		ObfsMode:    "audio",
		Fingerprint: "chrome",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got backend.ConnectParams
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.PeerAddr != p.PeerAddr {
		t.Errorf("PeerAddr: got %q, want %q", got.PeerAddr, p.PeerAddr)
	}
	if got.Password != p.Password {
		t.Errorf("Password: got %q, want %q", got.Password, p.Password)
	}
	if len(got.Hashes) != 2 || got.Hashes[0] != "abc" || got.Hashes[1] != "def" {
		t.Errorf("Hashes: got %v, want [abc def]", got.Hashes)
	}
	if got.DeviceID != p.DeviceID {
		t.Errorf("DeviceID: got %q, want %q", got.DeviceID, p.DeviceID)
	}
	if got.Workers != p.Workers {
		t.Errorf("Workers: got %d, want %d", got.Workers, p.Workers)
	}
	if got.CaptchaMode != p.CaptchaMode {
		t.Errorf("CaptchaMode: got %q, want %q", got.CaptchaMode, p.CaptchaMode)
	}
	if got.ObfsMode != p.ObfsMode {
		t.Errorf("ObfsMode: got %q, want %q", got.ObfsMode, p.ObfsMode)
	}
	if got.Fingerprint != p.Fingerprint {
		t.Errorf("Fingerprint: got %q, want %q", got.Fingerprint, p.Fingerprint)
	}
}

func TestConnectParams_EmptyOmittedFields(t *testing.T) {
	p := backend.ConnectParams{
		PeerAddr: "1.2.3.4:5555",
		Password: "secret",
		Hashes:   []string{"abc"},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// omitempty поля не должны присутствовать в JSON
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	if _, ok := raw["deviceId"]; ok {
		t.Error("expected deviceId to be omitted when empty")
	}
	if _, ok := raw["workers"]; ok {
		t.Error("expected workers to be omitted when zero")
	}
	if _, ok := raw["captchaMode"]; ok {
		t.Error("expected captchaMode to be omitted when empty")
	}
	if _, ok := raw["obfsMode"]; ok {
		t.Error("expected obfsMode to be omitted when empty")
	}
	if _, ok := raw["fingerprint"]; ok {
		t.Error("expected fingerprint to be omitted when empty")
	}
}

func TestConnectParams_FromJSON(t *testing.T) {
	input := `{
		"peerAddr": "5.6.7.8:443",
		"password": "pass123",
		"hashes": ["hash1"],
		"deviceId": "abc",
		"workers": 18,
		"captchaMode": "wv",
		"obfsMode": "video",
		"fingerprint": "firefox"
	}`

	var p backend.ConnectParams
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if p.PeerAddr != "5.6.7.8:443" {
		t.Errorf("PeerAddr: got %q", p.PeerAddr)
	}
	if p.Password != "pass123" {
		t.Errorf("Password: got %q", p.Password)
	}
	if p.Workers != 18 {
		t.Errorf("Workers: got %d", p.Workers)
	}
	if p.ObfsMode != "video" {
		t.Errorf("ObfsMode: got %q", p.ObfsMode)
	}
	if p.Fingerprint != "firefox" {
		t.Errorf("Fingerprint: got %q", p.Fingerprint)
	}
}

func TestConnectParams_NullHashes(t *testing.T) {
	input := `{"peerAddr":"1.2.3.4:5555","password":"x"}`
	var p backend.ConnectParams
	json.Unmarshal([]byte(input), &p)

	if p.Hashes != nil {
		t.Errorf("expected nil Hashes, got %v", p.Hashes)
	}
}

// ═══════════════════════════════════════════════════
// ProfileData — маршалинг/анмаршалинг
// ═══════════════════════════════════════════════════

func TestProfileData_MarshalRoundtrip(t *testing.T) {
	p := backend.ProfileData{
		PeerAddr: "1.2.3.4:5555",
		Password: "secret",
		Hashes:   []string{"h1", "h2"},
		Workers:  18,
		Listen:   "127.0.0.1:9000",
		TurnHost: "turn.example.com",
		TurnPort: "3478",
		DeviceID: "dev-001",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got backend.ProfileData
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.PeerAddr != p.PeerAddr {
		t.Errorf("PeerAddr: got %q, want %q", got.PeerAddr, p.PeerAddr)
	}
	if got.Password != p.Password {
		t.Errorf("Password: got %q, want %q", got.Password, p.Password)
	}
	if len(got.Hashes) != 2 {
		t.Errorf("Hashes length: got %d, want 2", len(got.Hashes))
	}
	if got.Workers != p.Workers {
		t.Errorf("Workers: got %d, want %d", got.Workers, p.Workers)
	}
	if got.Listen != p.Listen {
		t.Errorf("Listen: got %q, want %q", got.Listen, p.Listen)
	}
	if got.TurnHost != p.TurnHost {
		t.Errorf("TurnHost: got %q, want %q", got.TurnHost, p.TurnHost)
	}
	if got.TurnPort != p.TurnPort {
		t.Errorf("TurnPort: got %q, want %q", got.TurnPort, p.TurnPort)
	}
	if got.DeviceID != p.DeviceID {
		t.Errorf("DeviceID: got %q, want %q", got.DeviceID, p.DeviceID)
	}
}

func TestProfileData_EmptyFields(t *testing.T) {
	p := backend.ProfileData{
		PeerAddr: "1.2.3.4:5555",
		Password: "pass",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	// JSON теги: peer, password, hashes, listen, turn, port, device_id
	if _, ok := raw["peer"]; !ok {
		t.Error("expected 'peer' in JSON")
	}
	if _, ok := raw["password"]; !ok {
		t.Error("expected 'password' in JSON")
	}
	// hashes, listen, turn, port, device_id должны быть нулевыми/пустыми
}

func TestProfileData_JSONTags(t *testing.T) {
	p := backend.ProfileData{
		PeerAddr: "1.2.3.4:5555",
		Password: "x",
		Workers:  18,
		DeviceID: "id",
	}

	data, _ := json.Marshal(p)
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	// Проверяем что JSON ключи соответствуют тегам
	for _, key := range []string{"peer", "password", "hashes", "workers", "listen", "turn", "port", "device_id"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected key %q in JSON output", key)
		}
	}
	// Старые ключи не должны присутствовать
	for _, bad := range []string{"PeerAddr", "peerAddr", "Password", "deviceId"} {
		if _, ok := raw[bad]; ok {
			t.Errorf("unexpected key %q in JSON output", bad)
		}
	}
}

func TestProfileData_FromJSON(t *testing.T) {
	input := `{
		"peer": "5.6.7.8:443",
		"password": "pw",
		"hashes": ["a"],
		"workers": 18,
		"listen": "0.0.0.0:1080",
		"turn": "1.1.1.1",
		"port": "8443",
		"device_id": "abc"
	}`

	var p backend.ProfileData
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if p.PeerAddr != "5.6.7.8:443" {
		t.Errorf("PeerAddr: got %q", p.PeerAddr)
	}
	if p.Workers != 18 {
		t.Errorf("Workers: got %d, want 18", p.Workers)
	}
	if p.Listen != "0.0.0.0:1080" {
		t.Errorf("Listen: got %q", p.Listen)
	}
	if p.TurnHost != "1.1.1.1" {
		t.Errorf("TurnHost: got %q", p.TurnHost)
	}
	if p.TurnPort != "8443" {
		t.Errorf("TurnPort: got %q", p.TurnPort)
	}
}

// ═══════════════════════════════════════════════════
// AppSettings — сериализация
// ═══════════════════════════════════════════════════

func TestAppSettings_MarshalRoundtrip(t *testing.T) {
	s := backend.AppSettings{AutoStart: true, ObfsMode: "video"}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got backend.AppSettings
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.AutoStart != s.AutoStart {
		t.Errorf("AutoStart: got %v, want %v", got.AutoStart, s.AutoStart)
	}
	if got.ObfsMode != s.ObfsMode {
		t.Errorf("ObfsMode: got %q, want %q", got.ObfsMode, s.ObfsMode)
	}
}

func TestAppSettings_JSONKeys(t *testing.T) {
	s := backend.AppSettings{AutoStart: false, ObfsMode: "audio"}

	data, _ := json.Marshal(s)
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	if _, ok := raw["autoStart"]; !ok {
		t.Error("expected 'autoStart' key")
	}
	if _, ok := raw["obfsMode"]; !ok {
		t.Error("expected 'obfsMode' key")
	}
}

func TestAppSettings_Defaults(t *testing.T) {
	// Пустой JSON — должно десериализоваться в zero values
	input := `{}`
	var s backend.AppSettings
	if err := json.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if s.AutoStart != false {
		t.Error("expected AutoStart=false for empty JSON")
	}
	if s.ObfsMode != "" {
		t.Errorf("expected empty ObfsMode for empty JSON, got %q", s.ObfsMode)
	}
}

func TestAppSettings_AllFields(t *testing.T) {
	input := `{"autoStart":true,"obfsMode":"video"}`
	var s backend.AppSettings
	if err := json.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !s.AutoStart {
		t.Error("expected AutoStart=true")
	}
	if s.ObfsMode != "video" {
		t.Errorf("expected ObfsMode=video, got %q", s.ObfsMode)
	}
}

func TestAppSettings_InvalidJSON(t *testing.T) {
	var s backend.AppSettings
	err := json.Unmarshal([]byte(`{invalid`), &s)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
