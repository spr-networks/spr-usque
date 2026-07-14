package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var UsquePath = "/usr/bin/usque"

const TunInterfaceName = "warp0"
const TunnelMTU = 1280
const TunnelUpHook = "/scripts/tunnel-up.sh"
const TunnelDownHook = "/scripts/tunnel-down.sh"

var nowUnix = func() int64 { return time.Now().Unix() }

func buildNativeTunArgs(s Settings) []string {
	args := []string{
		"-c", WarpConfigFile,
		"nativetun",
		"--interface-name", TunInterfaceName,
		"--mtu", strconv.Itoa(TunnelMTU),
		"--connect-port", strconv.Itoa(s.ConnectPort),
		"--always-reconnect",
		"--on-connect", TunnelUpHook,
		"--on-disconnect", TunnelDownHook,
	}
	if s.EndpointVersion == "v6" {
		args = append(args, "--ipv6")
	}
	if s.Transport == "http2" {
		args = append(args, "--http2")
	}
	if !s.TunnelIPv6 {
		args = append(args, "--no-tunnel-ipv6")
	}
	return args
}

func buildRegisterArgs(deviceName, jwt string) []string {
	args := []string{"-c", WarpConfigFile, "register", "--accept-tos"}
	if deviceName != "" {
		args = append(args, "--name", deviceName)
	}
	if jwt != "" {
		args = append(args, "--jwt", jwt)
	}
	return args
}

type ProcessStatus struct {
	Running   bool
	Desired   bool
	Uptime    string
	LastError string
}

type TunnelManager struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	gen       int
	running   bool
	desired   bool
	settings  Settings
	startedAt time.Time
	lastErr   string
}

var tunnel = &TunnelManager{}

func (tm *TunnelManager) Start(s Settings) error {
	if !warpRegistered() {
		return fmt.Errorf("not registered: complete WARP enrollment first")
	}
	if err := validateSettings(s); err != nil {
		return err
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.running {
		tm.desired = true
		tm.settings = s
		return nil
	}

	tm.gen++
	gen := tm.gen
	writeDisconnectedState()
	cmd := exec.Command(UsquePath, buildNativeTunArgs(s)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		tm.desired = false
		tm.lastErr = err.Error()
		return err
	}

	tm.cmd = cmd
	tm.running = true
	tm.desired = true
	tm.settings = s
	tm.startedAt = time.Now()
	tm.lastErr = ""
	fmt.Printf("[+] usque native tunnel started (pid %d, interface %s)\n", cmd.Process.Pid, TunInterfaceName)

	go tm.wait(cmd, gen)
	return nil
}

func (tm *TunnelManager) wait(cmd *exec.Cmd, gen int) {
	err := cmd.Wait()

	tm.mu.Lock()
	if tm.gen != gen {
		tm.mu.Unlock()
		return
	}
	tm.running = false
	tm.cmd = nil
	if err != nil {
		tm.lastErr = err.Error()
	} else {
		tm.lastErr = "tunnel process exited"
	}
	shouldRestart := tm.desired
	settings := tm.settings
	tm.mu.Unlock()

	writeDisconnectedState()
	fmt.Println("[-] usque exited:", err)
	if !shouldRestart {
		return
	}

	time.Sleep(5 * time.Second)
	tm.mu.Lock()
	stillDesired := tm.desired && tm.gen == gen && !tm.running
	tm.mu.Unlock()
	if stillDesired {
		if err := tm.Start(settings); err != nil {
			fmt.Println("[-] usque relaunch failed:", err)
		}
	}
}

func (tm *TunnelManager) Stop() {
	tm.mu.Lock()
	tm.gen++
	tm.desired = false
	cmd := tm.cmd
	tm.cmd = nil
	tm.running = false
	tm.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	writeDisconnectedState()
}

func (tm *TunnelManager) Restart(s Settings) error {
	tm.Stop()
	time.Sleep(350 * time.Millisecond)
	return tm.Start(s)
}

func (tm *TunnelManager) Status() ProcessStatus {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	status := ProcessStatus{Running: tm.running, Desired: tm.desired, LastError: tm.lastErr}
	if tm.running {
		status.Uptime = time.Since(tm.startedAt).Round(time.Second).String()
	}
	return status
}

func runRegister(deviceName, jwt string, force bool) error {
	if err := validateDeviceName(deviceName); err != nil {
		return err
	}
	if err := validateJWT(jwt); err != nil {
		return err
	}
	if err := ensureConfigDirs(); err != nil {
		return err
	}

	backup := WarpConfigFile + ".bak"
	hadExisting := false
	if _, err := os.Stat(WarpConfigFile); err == nil {
		if !force {
			return fmt.Errorf("already registered; confirm re-registration first")
		}
		hadExisting = true
		_ = os.Remove(backup)
		if err := os.Rename(WarpConfigFile, backup); err != nil {
			return fmt.Errorf("failed to back up existing config: %v", err)
		}
		_ = os.Chmod(backup, 0600)
	}

	restore := func() {
		if !hadExisting {
			return
		}
		_ = os.Remove(WarpConfigFile)
		_ = os.Rename(backup, WarpConfigFile)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, UsquePath, buildRegisterArgs(deviceName, jwt)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		restore()
		detail := strings.TrimSpace(strings.ReplaceAll(string(out), jwt, "[redacted]"))
		if len(detail) > 2048 {
			detail = detail[len(detail)-2048:]
		}
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("registration timed out")
		}
		if detail != "" {
			return fmt.Errorf("registration failed: %s", detail)
		}
		return fmt.Errorf("registration failed: %v", err)
	}

	if err := os.Chmod(WarpConfigFile, 0600); err != nil {
		restore()
		return fmt.Errorf("registration succeeded but credential permissions failed: %v", err)
	}
	if !warpRegistered() {
		restore()
		return fmt.Errorf("registration did not produce valid credentials")
	}
	return nil
}
