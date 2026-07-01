package backend

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseWGConfig_FullConfig(t *testing.T) {
	input := `[Interface]
Address = 10.0.0.2/32
DNS = 1.1.1.1
MTU = 1420
PrivateKey = secret123
PostUp = iptables -A ...
PreDown = iptables -D ...

[Peer]
PublicKey = pubkey456
Endpoint = 1.2.3.4:56001
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`
	addr, mtu, allowedIPs, wgConf := parseWGConfig(input)

	if addr != "10.0.0.2/32" {
		t.Errorf("addr = %q, want %q", addr, "10.0.0.2/32")
	}
	if mtu != "1420" {
		t.Errorf("mtu = %q, want %q", mtu, "1420")
	}
	wantIPs := []string{"0.0.0.0/0", "::/0"}
	if !reflect.DeepEqual(allowedIPs, wantIPs) {
		t.Errorf("allowedIPs = %v, want %v", allowedIPs, wantIPs)
	}
	// wgConf strips Address/MTU (extracted) and wg-quick-only fields
	if containsLine(wgConf, "Address") {
		t.Error("wgConf should NOT contain Address line (extracted)")
	}
	if containsLine(wgConf, "MTU") {
		t.Error("wgConf should NOT contain MTU line (extracted)")
	}
	if containsLine(wgConf, "DNS") {
		t.Error("wgConf should NOT contain DNS line (quick-only)")
	}
	if !containsLine(wgConf, "PrivateKey = secret123") {
		t.Error("wgConf should contain PrivateKey line")
	}
	if !containsLine(wgConf, "PublicKey = pubkey456") {
		t.Error("wgConf should contain PublicKey line")
	}
}

func TestParseWGConfig_QuickOnlyFieldsStripped(t *testing.T) {
	input := `[Interface]
Address = 10.0.0.2/32
DNS = 8.8.8.8
MTU = 1300
PreUp = echo start
PostUp = echo up
PreDown = echo down
PostDown = echo stop
SaveConfig = true

[Peer]
PublicKey = abc123
Endpoint = 5.6.7.8:56001
AllowedIPs = 10.0.0.0/24
`
	addr, _, _, wgConf := parseWGConfig(input)

	if addr != "10.0.0.2/32" {
		t.Errorf("addr = %q, want %q", addr, "10.0.0.2/32")
	}
	// wg-quick-only fields should NOT appear in wgConf output
	for _, field := range []string{"DNS", "PreUp", "PostUp", "PreDown", "PostDown", "SaveConfig"} {
		if containsLine(wgConf, field) {
			t.Errorf("wgConf should NOT contain %s line", field)
		}
	}
	// Address is extracted and stripped from wgConf
	if containsLine(wgConf, "Address") {
		t.Error("wgConf should NOT contain Address line (extracted)")
	}
}

func TestParseWGConfig_NoAllowedIPs(t *testing.T) {
	input := `[Interface]
Address = 192.168.1.1/24

[Peer]
Endpoint = 1.1.1.1:51820
`
	addr, _, allowedIPs, _ := parseWGConfig(input)

	if addr != "192.168.1.1/24" {
		t.Errorf("addr = %q, want %q", addr, "192.168.1.1/24")
	}
	if len(allowedIPs) != 0 {
		t.Errorf("allowedIPs = %v, want empty", allowedIPs)
	}
}

func TestParseWGConfig_EmptyInput(t *testing.T) {
	addr, mtu, allowedIPs, wgConf := parseWGConfig("")

	if addr != "" {
		t.Errorf("addr = %q, want empty", addr)
	}
	if mtu != "" {
		t.Errorf("mtu = %q, want empty", mtu)
	}
	if len(allowedIPs) != 0 {
		t.Errorf("allowedIPs = %v, want empty", allowedIPs)
	}
	if wgConf != "" {
		t.Errorf("wgConf = %q, want empty", wgConf)
	}
}

func TestParseWGConfig_SingleAllowedIP(t *testing.T) {
	input := `AllowedIPs = 172.16.0.0/16`
	_, _, allowedIPs, _ := parseWGConfig(input)

	if len(allowedIPs) != 1 || allowedIPs[0] != "172.16.0.0/16" {
		t.Errorf("allowedIPs = %v, want [172.16.0.0/16]", allowedIPs)
	}
}

func TestParseWGConfig_CaseInsensitiveKeys(t *testing.T) {
	input := `
address = 10.0.0.5/32
MTU = 1300
allowedips = 0.0.0.0/0
`
	addr, mtu, allowedIPs, _ := parseWGConfig(input)

	if addr != "10.0.0.5/32" {
		t.Errorf("addr = %q, want %q", addr, "10.0.0.5/32")
	}
	if mtu != "1300" {
		t.Errorf("mtu = %q, want %q", mtu, "1300")
	}
	if len(allowedIPs) != 1 || allowedIPs[0] != "0.0.0.0/0" {
		t.Errorf("allowedIPs = %v, want [0.0.0.0/0]", allowedIPs)
	}
}

func TestVkExcludeCIDRsNotEmpty(t *testing.T) {
	if len(vkExcludeCIDRs) == 0 {
		t.Error("vkExcludeCIDRs should not be empty")
	}
}

func TestWgQuickOnlyFieldsExpectedKeys(t *testing.T) {
	expected := []string{"address", "dns", "mtu", "preup", "postup", "predown", "postdown", "saveconfig"}
	for _, k := range expected {
		if !wgQuickOnlyFields[k] {
			t.Errorf("wgQuickOnlyFields missing key %q", k)
		}
	}
}

func containsLine(s, sub string) bool {
	return strings.Contains(s, sub)
}
