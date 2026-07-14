package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type TopoNode struct {
	ID       string
	Kind     string
	Name     string
	IP       string `json:",omitempty"`
	ConnType string `json:",omitempty"`
	Online   bool
}

type TopoEdge struct {
	From  string
	To    string
	Layer string
	Kind  string
}

type TopoSink struct {
	ID     string
	Name   string
	Iface  string
	IP     string `json:",omitempty"`
	Online bool
}

type Topology struct {
	Nodes []TopoNode
	Edges []TopoEdge
	Sinks []TopoSink `json:",omitempty"`
}

func buildTopology(connected bool, containerIP string, trace map[string]string) Topology {
	warpOn := trace["warp"] == "on" || trace["warp"] == "plus"
	sinkOnline := connected
	if trace["warp"] != "" {
		sinkOnline = connected && warpOn
	}
	topo := Topology{
		Nodes: []TopoNode{{ID: "root", ConnType: "masque", Online: true}},
		Edges: []TopoEdge{},
	}
	if containerIP != "" {
		topo.Nodes = append(topo.Nodes, TopoNode{
			ID: "usque-gateway", Kind: "gateway", Name: "WARP gateway",
			IP: containerIP, ConnType: "masque", Online: connected,
		})
		topo.Edges = append(topo.Edges, TopoEdge{
			From: "root", To: "usque-gateway", Layer: "vpn", Kind: "masque",
		})
		topo.Sinks = []TopoSink{{
			ID: "warp", Name: "Cloudflare WARP", Iface: "spr-usque",
			IP: containerIP, Online: sinkOnline,
		}}
	}

	if trace["colo"] != "" {
		topo.Nodes = append(topo.Nodes, TopoNode{
			ID: "cloudflare-edge", Kind: "vpn-exit", Name: trace["colo"],
			IP: trace["ip"], ConnType: "masque", Online: connected && warpOn,
		})
		from := "root"
		if containerIP != "" {
			from = "usque-gateway"
		}
		topo.Edges = append(topo.Edges, TopoEdge{
			From: from, To: "cloudflare-edge", Layer: "vpn", Kind: "masque",
		})
	}
	return topo
}

func handleTopology(w http.ResponseWriter, r *http.Request) {
	ps := tunnel.Status()
	state := loadTunnelState()
	iface := state.Interface
	if iface == "" {
		iface = TunInterfaceName
	}
	connected := ps.Running && state.Connected && interfaceUp(iface)

	var trace map[string]string
	if connected {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		if raw, err := fetchTraceViaInterface(ctx, iface); err == nil {
			trace = parseTrace(raw)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(buildTopology(connected, getContainerIP(), trace))
}
