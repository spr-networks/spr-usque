package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var UNIX_PLUGIN_LISTENER = "/run/spr-krun-plugin/spr-usque.sock"

func getContainerIP() string {
	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return ""
}

func interfaceUp(name string) bool {
	iface, err := net.InterfaceByName(name)
	return err == nil && iface.Flags&net.FlagUp != 0
}

func jsonResponse(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Println("[-] response encode failed:", err)
	}
}

func httpError(w http.ResponseWriter, msg string, code int) {
	fmt.Println("[-]", msg)
	http.Error(w, msg, code)
}

func selectedEndpoint(wc WarpConfig, s Settings) string {
	endpoint := wc.EndpointV4
	if s.Transport == "http2" {
		endpoint = wc.EndpointH2V4
		if s.EndpointVersion == "v6" {
			endpoint = wc.EndpointH2V6
		}
	} else if s.EndpointVersion == "v6" {
		endpoint = wc.EndpointV6
	}
	if endpoint == "" {
		return ""
	}
	return net.JoinHostPort(endpoint, fmt.Sprintf("%d", s.ConnectPort))
}

func liveTunnelState() (ProcessStatus, TunnelState, string, bool) {
	ps := tunnel.Status()
	state := loadTunnelState()
	iface := state.Interface
	if iface == "" {
		iface = TunInterfaceName
	}
	connected := ps.Running && state.Connected && interfaceUp(iface)
	return ps, state, iface, connected
}

// GET /status
func handleStatus(w http.ResponseWriter, r *http.Request) {
	settings := loadSettings()
	ps, state, iface, connected := liveTunnelState()

	status := map[string]interface{}{
		"Registered":       warpRegistered(),
		"ProcessRunning":   ps.Running,
		"DesiredRunning":   ps.Desired,
		"Connected":        connected,
		"ForwardingActive": connected,
		"Uptime":           ps.Uptime,
		"LastError":        ps.LastError,
		"GatewayIP":        getContainerIP(),
		"GatewayInterface": "spr-usque",
		"TunnelInterface":  iface,
		"TunnelMTU":        TunnelMTU,
		"EndpointVersion":  settings.EndpointVersion,
		"Transport":        settings.Transport,
		"ConnectPort":      settings.ConnectPort,
		"TunnelIPv6":       settings.TunnelIPv6,
		"AutoStart":        settings.AutoStart,
		"ConnectedAt":      state.ConnectedAt,
	}

	if wc, err := loadWarpConfig(); err == nil {
		for key, value := range redactWarpConfig(wc) {
			status[key] = value
		}
		endpoint := state.Endpoint
		if endpoint == "" {
			endpoint = selectedEndpoint(wc, settings)
		}
		status["Endpoint"] = endpoint
	}

	connectivity := map[string]interface{}{"OK": false, "Pending": false, "Stale": false}
	if connected {
		snapshot := traceSnapshot(iface, state.ConnectedAt)
		connectivity["Pending"] = snapshot.Pending
		connectivity["Stale"] = snapshot.Stale
		connectivity["CheckedAt"] = snapshot.CheckedAt
		connectivity["VerifiedAt"] = snapshot.VerifiedAt
		if snapshot.Error != "" {
			connectivity["Error"] = snapshot.Error
		}
		if len(snapshot.Fields) > 0 {
			fields := snapshot.Fields
			connectivity["OK"] = fields["warp"] == "on" || fields["warp"] == "plus"
			connectivity["Warp"] = fields["warp"]
			connectivity["Colo"] = fields["colo"]
			connectivity["IP"] = fields["ip"]
			connectivity["Location"] = fields["loc"]
		}
	}
	status["Connectivity"] = connectivity
	jsonResponse(w, status)
}

// POST /register
func handleRegister(w http.ResponseWriter, r *http.Request) {
	req := struct {
		DeviceName string
		JWT        string
		Force      bool
	}{}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			httpError(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	settings := loadSettings()
	name := strings.TrimSpace(req.DeviceName)
	if name == "" {
		name = settings.DeviceName
	}
	wasDesired := tunnel.Status().Desired
	if req.Force {
		tunnel.Stop()
	}

	if err := runRegister(name, strings.TrimSpace(req.JWT), req.Force); err != nil {
		if wasDesired && warpRegistered() {
			_ = tunnel.Start(settings)
		}
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already registered") {
			code = http.StatusConflict
		} else if strings.Contains(err.Error(), "invalid") {
			code = http.StatusBadRequest
		}
		httpError(w, err.Error(), code)
		return
	}

	settings.DeviceName = name
	if err := saveSettings(settings); err != nil {
		httpError(w, "registered, but failed to persist device name: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if settings.AutoStart || wasDesired {
		if err := tunnel.Start(settings); err != nil {
			httpError(w, "registered, but tunnel start failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	jsonResponse(w, map[string]bool{"Registered": true})
}

// GET /config, PUT /config
func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		jsonResponse(w, loadSettings())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()
	settings := defaultSettings()
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := saveSettings(settings); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if warpRegistered() {
		ps := tunnel.Status()
		var err error
		if ps.Running || ps.Desired {
			err = tunnel.Restart(settings)
		} else if settings.AutoStart {
			err = tunnel.Start(settings)
		}
		if err != nil {
			httpError(w, "settings saved, but tunnel apply failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	jsonResponse(w, settings)
}

// PUT /tunnel {"Running":true|false}
func handleTunnel(w http.ResponseWriter, r *http.Request) {
	req := struct{ Running bool }{}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Running {
		if err := tunnel.Start(loadSettings()); err != nil {
			httpError(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		tunnel.Stop()
	}
	jsonResponse(w, map[string]bool{"Running": req.Running})
}

// POST /restart
func handleRestart(w http.ResponseWriter, r *http.Request) {
	if err := tunnel.Restart(loadSettings()); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonResponse(w, map[string]bool{"Restarted": true})
}

// GET /trace
func handleTrace(w http.ResponseWriter, r *http.Request) {
	_, _, iface, connected := liveTunnelState()
	if !connected {
		httpError(w, "WARP tunnel is not connected", http.StatusConflict)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	raw, err := fetchTraceViaInterface(ctx, iface)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprint(w, raw)
}

type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path, err := filepath.Abs(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path = filepath.Join(h.staticPath, path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func main() {
	syscall.Umask(0007)
	if err := ensureConfigDirs(); err != nil {
		panic(err)
	}
	if _, err := os.Stat(WarpConfigFile); err == nil {
		_ = os.Chmod(WarpConfigFile, 0600)
	}

	settings := loadSettings()
	if warpRegistered() && settings.AutoStart {
		if err := tunnel.Start(settings); err != nil {
			fmt.Println("[-] tunnel autostart failed:", err)
		}
	} else if !warpRegistered() {
		fmt.Println("[ ] no WARP enrollment yet; forwarding remains fail-closed")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", handleRegister)
	mux.HandleFunc("GET /status", handleStatus)
	mux.HandleFunc("GET /config", handleConfig)
	mux.HandleFunc("PUT /config", handleConfig)
	mux.HandleFunc("PUT /tunnel", handleTunnel)
	mux.HandleFunc("POST /restart", handleRestart)
	mux.HandleFunc("GET /trace", handleTrace)
	mux.HandleFunc("GET /topology", handleTopology)
	mux.Handle("/", spaHandler{staticPath: "/ui", indexPath: "index.html"})

	_ = os.Remove(UNIX_PLUGIN_LISTENER)
	if err := os.MkdirAll(filepath.Dir(UNIX_PLUGIN_LISTENER), 0755); err != nil {
		panic(err)
	}
	listener, err := net.Listen("unix", UNIX_PLUGIN_LISTENER)
	if err != nil {
		panic(err)
	}
	if err := os.Chmod(UNIX_PLUGIN_LISTENER, 0770); err != nil {
		fmt.Println("[!] socket chmod unavailable; using creation permissions:", err)
	}

	server := http.Server{
		Handler:           logRequest(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
