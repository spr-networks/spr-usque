package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var TraceURL = "https://www.cloudflare.com/cdn-cgi/trace"

const (
	traceProbeInterval = 30 * time.Second
	traceProbeTimeout  = 4 * time.Second
)

type TraceSnapshot struct {
	Fields     map[string]string
	Error      string
	Pending    bool
	Stale      bool
	CheckedAt  int64
	VerifiedAt int64
}

type traceCacheState struct {
	key        string
	fields     map[string]string
	lastError  string
	checkedAt  time.Time
	verifiedAt time.Time
	inFlight   bool
}

var (
	traceCacheMu sync.Mutex
	traceCache   traceCacheState
	traceFetcher = fetchTraceViaInterface
)

func fetchTraceViaInterface(ctx context.Context, interfaceName string) (string, error) {
	control := interfaceControl(interfaceName)
	dnsDialer := &net.Dialer{Timeout: 4 * time.Second, Control: control}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dnsDialer.DialContext(ctx, network, "1.1.1.1:53")
		},
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, Resolver: resolver, Control: control}
	transport := &http.Transport{
		DialContext:       dialer.DialContext,
		DisableKeepAlives: true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, TraceURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("trace returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func parseTrace(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && key != "" {
			out[key] = value
		}
	}
	return out
}

func copyTraceFields(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(fields))
	for key, value := range fields {
		out[key] = value
	}
	return out
}

func unixTimestamp(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}

// traceSnapshot returns immediately with the last probe result and starts a
// bounded refresh in the background when the cache is stale. The tunnel state
// is authoritative for forwarding; this external HTTP probe is diagnostic and
// must never make frequent status or topology requests block.
func traceSnapshot(interfaceName string, connectedAt int64) TraceSnapshot {
	key := fmt.Sprintf("%s:%d", interfaceName, connectedAt)
	now := time.Now()

	traceCacheMu.Lock()
	if traceCache.key != key {
		traceCache = traceCacheState{key: key}
	}
	if !traceCache.inFlight && (traceCache.checkedAt.IsZero() || now.Sub(traceCache.checkedAt) >= traceProbeInterval) {
		traceCache.inFlight = true
		traceCache.checkedAt = now
		go refreshTrace(key, interfaceName)
	}
	snapshot := TraceSnapshot{
		Fields:     copyTraceFields(traceCache.fields),
		Error:      traceCache.lastError,
		Pending:    traceCache.inFlight && len(traceCache.fields) == 0,
		Stale:      traceCache.lastError != "" && len(traceCache.fields) > 0,
		CheckedAt:  unixTimestamp(traceCache.checkedAt),
		VerifiedAt: unixTimestamp(traceCache.verifiedAt),
	}
	traceCacheMu.Unlock()
	return snapshot
}

func refreshTrace(key, interfaceName string) {
	ctx, cancel := context.WithTimeout(context.Background(), traceProbeTimeout)
	defer cancel()
	raw, err := traceFetcher(ctx, interfaceName)
	now := time.Now()

	traceCacheMu.Lock()
	defer traceCacheMu.Unlock()
	if traceCache.key != key {
		return
	}
	traceCache.inFlight = false
	traceCache.checkedAt = now
	if err != nil {
		traceCache.lastError = err.Error()
		return
	}
	fields := parseTrace(raw)
	if fields["warp"] == "" {
		traceCache.lastError = "Cloudflare trace did not report WARP state"
		return
	}
	traceCache.fields = fields
	traceCache.lastError = ""
	traceCache.verifiedAt = now
}
