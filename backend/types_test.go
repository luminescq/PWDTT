package backend

import (
	"encoding/json"
	"testing"
)

func TestProfileDataJSON(t *testing.T) {
	p := ProfileData{
		PeerAddr: "1.2.3.4:56000",
		Password: "secret",
		Hashes:   []string{"a", "b", "c"},
		Listen:   "0.0.0.0:56001",
		TurnHost: "turn.example.com",
		TurnPort: "3478",
		DeviceID: "uuid-123",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	var got ProfileData
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.PeerAddr != p.PeerAddr {
		t.Errorf("PeerAddr = %q, want %q", got.PeerAddr, p.PeerAddr)
	}
	if got.Password != p.Password {
		t.Errorf("Password = %q, want %q", got.Password, p.Password)
	}
	if len(got.Hashes) != 3 {
		t.Errorf("Hashes len = %d, want 3", len(got.Hashes))
	}
	if got.DeviceID != p.DeviceID {
		t.Errorf("DeviceID = %q, want %q", got.DeviceID, p.DeviceID)
	}
}

func TestProfileDataOptionalFields(t *testing.T) {
	jsonStr := `{"peer":"10.0.0.1:56000","password":"x","hashes":["h1"]}`
	var p ProfileData
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		t.Fatal(err)
	}
	if p.Listen != "" {
		t.Errorf("Listen should be empty, got %q", p.Listen)
	}
	if p.TurnHost != "" {
		t.Errorf("TurnHost should be empty, got %q", p.TurnHost)
	}
	if p.DeviceID != "" {
		t.Errorf("DeviceID should be empty, got %q", p.DeviceID)
	}
}

func TestConnectParamsJSON(t *testing.T) {
	cp := ConnectParams{
		Profile:     "my-profile",
		CaptchaMode: "auto",
		Workers:     5,
		MTU:         1300,
		Hashes:      []string{"h1", "h2"},
	}

	data, err := json.Marshal(cp)
	if err != nil {
		t.Fatal(err)
	}

	var got ConnectParams
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Profile != cp.Profile {
		t.Errorf("Profile = %q, want %q", got.Profile, cp.Profile)
	}
	if got.Workers != cp.Workers {
		t.Errorf("Workers = %d, want %d", got.Workers, cp.Workers)
	}
	if got.MTU != cp.MTU {
		t.Errorf("MTU = %d, want %d", got.MTU, cp.MTU)
	}
}

func TestWgIfaceConst(t *testing.T) {
	if wgIface != "wg-turn" {
		t.Errorf("wgIface = %q, want %q", wgIface, "wg-turn")
	}
}
