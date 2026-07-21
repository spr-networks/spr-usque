package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildTopologyDisconnectedSink(t *testing.T) {
	topo := buildTopology(false, "172.30.118.2", nil)
	if len(topo.Nodes) != 2 || len(topo.Edges) != 1 || len(topo.Sinks) != 1 {
		t.Fatalf("unexpected disconnected topology: %+v", topo)
	}
	sink := topo.Sinks[0]
	if sink.ID != "warp" || sink.Iface != "spr-usque" || sink.IP != "172.30.118.2" || sink.Online {
		t.Fatalf("bad forwarding sink: %+v", sink)
	}
}

func TestBuildTopologyConnected(t *testing.T) {
	trace := parseTrace("warp=on\ncolo=LAX\nip=104.28.1.2\n")
	topo := buildTopology(true, "172.30.118.2", trace)
	if len(topo.Nodes) != 3 || len(topo.Edges) != 2 || len(topo.Sinks) != 1 {
		t.Fatalf("unexpected connected topology: %+v", topo)
	}
	if !topo.Sinks[0].Online {
		t.Fatal("sink should be online when the tunnel verifies warp=on")
	}
	exit := topo.Nodes[2]
	if exit.ID != "cloudflare-edge" || exit.Kind != "vpn-exit" || exit.Name != "LAX" || exit.IP != "104.28.1.2" || !exit.Online {
		t.Fatalf("bad exit node: %+v", exit)
	}
	if edge := topo.Edges[1]; edge.From != "usque-gateway" || edge.To != "cloudflare-edge" {
		t.Fatalf("bad gateway-to-exit edge: %+v", edge)
	}
}

func TestBuildTopologyWarpOff(t *testing.T) {
	topo := buildTopology(true, "172.30.118.2", parseTrace("warp=off\ncolo=SJC\nip=203.0.113.2\n"))
	if topo.Sinks[0].Online || topo.Nodes[2].Online {
		t.Fatalf("warp=off must mark forwarding destination and exit offline: %+v", topo)
	}
}

func TestTopologyJSONContract(t *testing.T) {
	data, err := json.Marshal(buildTopology(false, "172.30.118.2", nil))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, field := range []string{`"Nodes"`, `"Edges"`, `"Sinks"`, `"Iface":"spr-usque"`} {
		if !strings.Contains(text, field) {
			t.Errorf("missing %s in %s", field, text)
		}
	}
}

func TestParseTrace(t *testing.T) {
	trace := parseTrace("warp=plus\ncolo=AMS\nip=2a09:bac1::1\ninvalid\n")
	if trace["warp"] != "plus" || trace["colo"] != "AMS" || trace["ip"] != "2a09:bac1::1" {
		t.Fatalf("trace parsed incorrectly: %+v", trace)
	}
}
