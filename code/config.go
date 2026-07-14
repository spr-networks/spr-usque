package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var TEST_PREFIX = os.Getenv("TEST_PREFIX")

var SettingsFile = TEST_PREFIX + "/configs/spr-usque/settings.json"
var WarpConfigFile = TEST_PREFIX + "/configs/spr-usque/config.json"
var TunnelStateFile = TEST_PREFIX + "/state/plugins/spr-usque/tunnel.state"

var settingsMtx sync.RWMutex

type Settings struct {
	EndpointVersion string // v4 or v6 underlay used to reach Cloudflare
	ConnectPort     int
	Transport       string // http3 or http2
	TunnelIPv6      bool
	AutoStart       bool
	DeviceName      string
}

func defaultSettings() Settings {
	return Settings{
		EndpointVersion: "v4",
		ConnectPort:     443,
		Transport:       "http3",
		TunnelIPv6:      true,
		AutoStart:       true,
		DeviceName:      "spr-usque",
	}
}

var deviceNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
var jwtRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validateDeviceName(name string) error {
	if name == "" {
		return nil
	}
	if !deviceNameRe.MatchString(name) {
		return fmt.Errorf("invalid device name: only [A-Za-z0-9._-], max 64 chars")
	}
	return nil
}

func validateJWT(jwt string) error {
	if jwt == "" {
		return nil
	}
	if len(jwt) > 4096 || !jwtRe.MatchString(jwt) {
		return fmt.Errorf("invalid enrollment token format")
	}
	return nil
}

func validateSettings(s Settings) error {
	if s.EndpointVersion != "v4" && s.EndpointVersion != "v6" {
		return fmt.Errorf("EndpointVersion must be \"v4\" or \"v6\"")
	}
	if s.ConnectPort < 1 || s.ConnectPort > 65535 {
		return fmt.Errorf("ConnectPort must be between 1 and 65535")
	}
	if s.Transport != "http3" && s.Transport != "http2" {
		return fmt.Errorf("Transport must be \"http3\" or \"http2\"")
	}
	return validateDeviceName(s.DeviceName)
}

func loadSettings() Settings {
	settingsMtx.RLock()
	defer settingsMtx.RUnlock()

	s := defaultSettings()
	data, err := os.ReadFile(SettingsFile)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, &s); err != nil || validateSettings(s) != nil {
		fmt.Println("[-] invalid settings file, using safe defaults")
		return defaultSettings()
	}
	return s
}

func saveSettings(s Settings) error {
	if err := validateSettings(s); err != nil {
		return err
	}
	settingsMtx.Lock()
	defer settingsMtx.Unlock()

	if err := ensureConfigDirs(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := SettingsFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, SettingsFile)
}

type WarpConfig struct {
	PrivateKey     string `json:"private_key"`
	EndpointV4     string `json:"endpoint_v4"`
	EndpointV6     string `json:"endpoint_v6"`
	EndpointH2V4   string `json:"endpoint_h2_v4"`
	EndpointH2V6   string `json:"endpoint_h2_v6"`
	EndpointPubKey string `json:"endpoint_pub_key"`
	ID             string `json:"id"`
	AccessToken    string `json:"access_token"`
	IPv4           string `json:"ipv4"`
	IPv6           string `json:"ipv6"`
}

func loadWarpConfig() (WarpConfig, error) {
	wc := WarpConfig{}
	data, err := os.ReadFile(WarpConfigFile)
	if err != nil {
		return wc, err
	}
	err = json.Unmarshal(data, &wc)
	return wc, err
}

func warpRegistered() bool {
	wc, err := loadWarpConfig()
	return err == nil && wc.PrivateKey != "" && wc.ID != ""
}

func redactWarpConfig(wc WarpConfig) map[string]interface{} {
	return map[string]interface{}{
		"DeviceID":       wc.ID,
		"EndpointV4":     wc.EndpointV4,
		"EndpointV6":     wc.EndpointV6,
		"EndpointH2V4":   wc.EndpointH2V4,
		"EndpointH2V6":   wc.EndpointH2V6,
		"WarpIPv4":       wc.IPv4,
		"WarpIPv6":       wc.IPv6,
		"HasPrivateKey":  wc.PrivateKey != "",
		"HasAccessToken": wc.AccessToken != "",
	}
}

func ensureConfigDirs() error {
	for _, dir := range []string{filepath.Dir(SettingsFile), filepath.Dir(TunnelStateFile)} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

type TunnelState struct {
	Connected   bool
	Interface   string
	Endpoint    string
	IPv4        string
	IPv6        string
	ConnectedAt int64
	UpdatedAt   int64
}

func loadTunnelState() TunnelState {
	state := TunnelState{}
	f, err := os.Open(TunnelStateFile)
	if err != nil {
		return state
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		switch key {
		case "Connected":
			state.Connected = value == "true"
		case "Interface":
			state.Interface = value
		case "Endpoint":
			state.Endpoint = value
		case "IPv4":
			state.IPv4 = value
		case "IPv6":
			state.IPv6 = value
		case "ConnectedAt":
			state.ConnectedAt, _ = strconv.ParseInt(value, 10, 64)
		case "UpdatedAt":
			state.UpdatedAt, _ = strconv.ParseInt(value, 10, 64)
		}
	}
	return state
}

func writeDisconnectedState() {
	if err := ensureConfigDirs(); err != nil {
		return
	}
	state := fmt.Sprintf("Connected=false\nInterface=%s\nUpdatedAt=%d\n", TunInterfaceName, nowUnix())
	tmp := TunnelStateFile + ".backend.tmp"
	if err := os.WriteFile(tmp, []byte(state), 0600); err == nil {
		_ = os.Rename(tmp, TunnelStateFile)
	}
}
