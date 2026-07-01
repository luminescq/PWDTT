package core

import (
	"strings"
	"testing"
)

var benchWGConf = `[Interface]
Address = 10.0.0.2/32
DNS = 1.1.1.1, 8.8.8.8
PrivateKey = very-long-private-key-base64-==
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT

[Peer]
PublicKey = another-long-base64-key==
Endpoint = 185.16.28.10:56001
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
`

func BenchmarkPatchWGConfig(b *testing.B) {
	for b.Loop() {
		patchWGConfig(benchWGConf, 1300)
	}
}

func BenchmarkPatchWGConfig_NoMTU(b *testing.B) {
	for b.Loop() {
		patchWGConfig(benchWGConf, 0)
	}
}

func BenchmarkPatchWGConfig_LargeAllowedIPs(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("[Interface]\nAddress = 10.0.0.2/32\nPrivateKey = key\n")
	sb.WriteString("[Peer]\nPublicKey = pub\nEndpoint = 1.2.3.4:51820\n")
	ips := make([]string, 100)
	for i := range ips {
		ips[i] = "0.0.0.0/0"
	}
	sb.WriteString("AllowedIPs = ")
	sb.WriteString(strings.Join(ips, ", "))
	config := sb.String()
	b.ResetTimer()
	for b.Loop() {
		patchWGConfig(config, 1300)
	}
}

func BenchmarkDeriveWrapKey(b *testing.B) {
	for b.Loop() {
		deriveWrapKey("test-password-123")
	}
}

func BenchmarkDeriveWrapKey_Long(b *testing.B) {
	for b.Loop() {
		deriveWrapKey("this-is-a-very-long-password-used-for-benchmarking-the-hkdf-key-derivation")
	}
}

func BenchmarkObfsWrapUnwrap(b *testing.B) {
	key, _ := deriveWrapKey("bench-key")
	payload := make([]byte, 1200)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	cfg := NewObfsConfig()
	state := NewObfsState()

	b.ResetTimer()
	for b.Loop() {
		wrapped, err := obfsWrapPacket(key, payload, cfg, state)
		if err != nil {
			b.Fatal(err)
		}
		dst := make([]byte, 2048)
		_, err = obfsUnwrapPacket(key, wrapped, dst)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkObfsWrapPacket(b *testing.B) {
	key, _ := deriveWrapKey("bench-key")
	payload := make([]byte, 1200)
	cfg := NewObfsConfig()
	state := NewObfsState()

	b.ResetTimer()
	for b.Loop() {
		_, _ = obfsWrapPacket(key, payload, cfg, state)
	}
}

func BenchmarkObfsUnwrapPacket(b *testing.B) {
	key, _ := deriveWrapKey("bench-key")
	payload := make([]byte, 1200)
	cfg := NewObfsConfig()
	state := NewObfsState()
	wrapped, _ := obfsWrapPacket(key, payload, cfg, state)
	dst := make([]byte, 2048)

	b.ResetTimer()
	for b.Loop() {
		_, _ = obfsUnwrapPacket(key, wrapped, dst)
	}
}

func BenchmarkObfsBuildNonce(b *testing.B) {
	for b.Loop() {
		obfsBuildNonce(12345678, 1, 960)
	}
}

func BenchmarkGenerateName(b *testing.B) {
	for b.Loop() {
		generateName()
	}
}

func BenchmarkConvertToFemaleSurname(b *testing.B) {
	surnames := []string{"Иванов", "Петров", "Сидоров", "Белый", "Волков", "Островский"}
	for b.Loop() {
		for _, s := range surnames {
			convertToFemaleSurname(s)
		}
	}
}

func BenchmarkGetAEAD(b *testing.B) {
	key, _ := deriveWrapKey("bench-key")
	for b.Loop() {
		getAEAD(key)
	}
}
