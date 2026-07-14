package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidateSettings(t *testing.T) {
	if err := validateSettings(defaultSettings()); err != nil {
		t.Fatalf("defaults rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Settings)
	}{
		{"endpoint", func(s *Settings) { s.EndpointVersion = "auto" }},
		{"port zero", func(s *Settings) { s.ConnectPort = 0 }},
		{"port high", func(s *Settings) { s.ConnectPort = 65536 }},
		{"transport", func(s *Settings) { s.Transport = "quic" }},
		{"device name", func(s *Settings) { s.DeviceName = "bad name$(id)" }},
		{"device name long", func(s *Settings) { s.DeviceName = strings.Repeat("a", 65) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := defaultSettings()
			tc.mutate(&s)
			if err := validateSettings(s); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	good := defaultSettings()
	good.EndpointVersion = "v6"
	good.Transport = "http2"
	good.DeviceName = "spr-router_01.example"
	if err := validateSettings(good); err != nil {
		t.Fatalf("valid advanced settings rejected: %v", err)
	}
}

func TestValidateJWT(t *testing.T) {
	for _, good := range []string{"", "eyJ.header.payload_sig"} {
		if err := validateJWT(good); err != nil {
			t.Errorf("valid token %q rejected: %v", good, err)
		}
	}
	for _, bad := range []string{"has space", "$(id)", "a;b", "line\nbreak", strings.Repeat("a", 4097)} {
		if err := validateJWT(bad); err == nil {
			t.Errorf("invalid token %q accepted", bad)
		}
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	oldSettings, oldState := SettingsFile, TunnelStateFile
	SettingsFile = filepath.Join(dir, "config", "settings.json")
	TunnelStateFile = filepath.Join(dir, "state", "tunnel.state")
	defer func() { SettingsFile, TunnelStateFile = oldSettings, oldState }()

	want := defaultSettings()
	want.ConnectPort = 8443
	want.Transport = "http2"
	want.TunnelIPv6 = false
	if err := saveSettings(want); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip: got %+v want %+v", got, want)
	}
	info, err := os.Stat(SettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode = %o, want 0600", info.Mode().Perm())
	}

	if err := os.WriteFile(SettingsFile, []byte(`{"Transport":"bogus"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(); !reflect.DeepEqual(got, defaultSettings()) {
		t.Fatalf("invalid persisted settings should fall back to defaults: %+v", got)
	}
}

func TestWarpConfigRedaction(t *testing.T) {
	wc := WarpConfig{
		PrivateKey: "private-secret", AccessToken: "access-secret", ID: "device-id",
		EndpointV4: "162.159.198.1", IPv4: "100.96.0.2", IPv6: "2606:4700::2",
	}
	redacted := redactWarpConfig(wc)
	data := strings.Join([]string{
		redacted["DeviceID"].(string), redacted["EndpointV4"].(string),
		redacted["WarpIPv4"].(string), redacted["WarpIPv6"].(string),
	}, " ")
	if strings.Contains(data, "private-secret") || strings.Contains(data, "access-secret") {
		t.Fatal("secret leaked from redacted config")
	}
	if redacted["HasPrivateKey"] != true || redacted["HasAccessToken"] != true {
		t.Fatalf("secret presence flags wrong: %+v", redacted)
	}
}

func TestBuildNativeTunArgs(t *testing.T) {
	s := defaultSettings()
	got := buildNativeTunArgs(s)
	want := []string{
		"-c", WarpConfigFile, "nativetun", "--interface-name", "warp0",
		"--mtu", "1280", "--connect-port", "443", "--always-reconnect",
		"--on-connect", TunnelUpHook, "--on-disconnect", TunnelDownHook,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default args: got %v want %v", got, want)
	}

	s.EndpointVersion = "v6"
	s.Transport = "http2"
	s.TunnelIPv6 = false
	got = buildNativeTunArgs(s)
	for _, flag := range []string{"--ipv6", "--http2", "--no-tunnel-ipv6"} {
		if !contains(got, flag) {
			t.Errorf("advanced args missing %s: %v", flag, got)
		}
	}
}

func TestBuildRegisterArgs(t *testing.T) {
	got := buildRegisterArgs("router", "eyJ.token.sig")
	want := []string{"-c", WarpConfigFile, "register", "--accept-tos", "--name", "router", "--jwt", "eyJ.token.sig"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("register args: got %v want %v", got, want)
	}
}

func TestTunnelStateParsing(t *testing.T) {
	dir := t.TempDir()
	old := TunnelStateFile
	TunnelStateFile = filepath.Join(dir, "tunnel.state")
	defer func() { TunnelStateFile = old }()

	data := "Connected=true\nInterface=warp0\nEndpoint=162.159.198.1:443\nIPv4=100.96.0.2\nConnectedAt=123\nUpdatedAt=124\n"
	if err := os.WriteFile(TunnelStateFile, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	got := loadTunnelState()
	if !got.Connected || got.Interface != "warp0" || got.Endpoint != "162.159.198.1:443" || got.ConnectedAt != 123 {
		t.Fatalf("state parsed incorrectly: %+v", got)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
