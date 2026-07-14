package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var TraceURL = "https://www.cloudflare.com/cdn-cgi/trace"

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
