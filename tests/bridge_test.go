package backend_test

import (
	"context"
	"testing"
	"time"

	"pwdtt/backend"
)

func newTestBridge(t *testing.T) *backend.Bridge {
	t.Helper()
	store := backend.NewTestStore(t)
	return backend.NewBridge(context.Background(), store, func(name string, args ...any) {})
}

// stopBridge останавливает ядро и ждёт завершения forwardEvents: иначе горутина
// удержит открытый лог-файл в TempDir, и Windows не сможет выполнить очистку.
func stopBridge(t *testing.T, b *backend.Bridge) {
	t.Helper()
	b.Disconnect()
	deadline := time.Now().Add(30 * time.Second)
	for b.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
}

// ═══════════════════════════════════════════════════
// Connect — ошибки
// ═══════════════════════════════════════════════════

func TestConnect_NoHashes(t *testing.T) {
	b := newTestBridge(t)
	defer stopBridge(t, b)

	err := b.Connect(backend.ConnectParams{
		PeerAddr: "1.2.3.4:5555",
		Password: "secret",
		Hashes:   nil,
	})

	if err == nil {
		t.Error("expected error for empty hashes")
	}
	if err != nil && err.Error() != "нет хешей VK" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConnect_EmptyHashes(t *testing.T) {
	b := newTestBridge(t)
	defer stopBridge(t, b)

	err := b.Connect(backend.ConnectParams{
		PeerAddr: "1.2.3.4:5555",
		Password: "secret",
		Hashes:   []string{},
	})

	if err == nil {
		t.Error("expected error for empty hashes slice")
	}
}

func TestConnect_AlreadyRunning(t *testing.T) {
	b := newTestBridge(t)
	defer stopBridge(t, b)

	// Первый вызов — running ставится true до core.Start()
	// core.Start() может упасть (нет сети), тогда running сбросится в false
	err1 := b.Connect(backend.ConnectParams{
		PeerAddr: "1.2.3.4:5555",
		Password: "secret",
		Hashes:   []string{"abc"},
	})
	_ = err1

	// Если bridge running — второй вызов должен вернуть ошибку
	if b.IsRunning() {
		err2 := b.Connect(backend.ConnectParams{
			PeerAddr: "5.6.7.8:9999",
			Password: "pw",
			Hashes:   []string{"def"},
		})
		if err2 == nil {
			t.Error("expected 'already running' error")
		}
		if err2 != nil && err2.Error() != "already running" {
			t.Errorf("unexpected error: %v", err2)
		}
	}
}

// ═══════════════════════════════════════════════════
// IsRunning
// ═══════════════════════════════════════════════════

func TestIsRunning_InitiallyFalse(t *testing.T) {
	b := newTestBridge(t)
	defer stopBridge(t, b)
	if b.IsRunning() {
		t.Error("expected IsRunning=false for new Bridge")
	}
}

// ═══════════════════════════════════════════════════
// Disconnect
// ═══════════════════════════════════════════════════

func TestDisconnect_NoPanic(t *testing.T) {
	b := newTestBridge(t)
	defer stopBridge(t, b)

	// Disconnect на неактивном bridge не должен паниковать
	b.Disconnect()
	b.Disconnect()
	b.Disconnect()
}

// ═══════════════════════════════════════════════════
// Defaults — workers и deviceID
// ═══════════════════════════════════════════════════

func TestConnect_DefaultWorkers(t *testing.T) {
	b := newTestBridge(t)
	defer stopBridge(t, b)

	// Workers=0 → bridge поставит 24
	err := b.Connect(backend.ConnectParams{
		PeerAddr: "1.2.3.4:5555",
		Password: "secret",
		Hashes:   []string{"abc"},
		Workers:  0,
	})
	_ = err // может упасть на core — это нормально
}

func TestConnect_DefaultDeviceID(t *testing.T) {
	b := newTestBridge(t)
	defer stopBridge(t, b)

	// DeviceID="" → bridge поставит "unknown"
	err := b.Connect(backend.ConnectParams{
		PeerAddr: "1.2.3.4:5555",
		Password: "secret",
		Hashes:   []string{"abc"},
		DeviceID: "",
	})
	_ = err
}

// ═══════════════════════════════════════════════════
// Connect — полный набор параметров
// ═══════════════════════════════════════════════════

func TestConnect_AllParams(t *testing.T) {
	b := newTestBridge(t)
	defer stopBridge(t, b)

	err := b.Connect(backend.ConnectParams{
		PeerAddr:    "1.2.3.4:5555",
		Password:    "secret",
		Hashes:      []string{"hash1", "hash2"},
		DeviceID:    "my-device",
		Workers:     18,
		CaptchaMode: "wv",
		ObfsMode:    "video",
		Fingerprint: "firefox",
	})
	_ = err
}

// ═══════════════════════════════════════════════════
// NewBridge
// ═══════════════════════════════════════════════════

func TestNewBridge(t *testing.T) {
	store := backend.NewTestStore(t)
	called := false
	b := backend.NewBridge(context.Background(), store, func(name string, args ...any) {
		called = true
	})

	if b == nil {
		t.Fatal("NewBridge returned nil")
	}
	if b.IsRunning() {
		t.Error("new bridge should not be running")
	}
	if called {
		t.Error("onEvent should not be called during construction")
	}
}
