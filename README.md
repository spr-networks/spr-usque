# spr-usque

## NOTE: made by codex

A Linux-only Cloudflare WARP egress gateway plugin for
[SPR](https://github.com/spr-networks/super), powered by
[usque](https://github.com/Diniboy1123/usque).

spr-usque runs usque in native TUN mode and exposes the plugin container as an
SPR forwarding destination. Traffic selected by an SPR forwarding policy enters
the stable gateway IP, crosses `warp0`, and exits through Cloudflare WARP over
MASQUE. Client devices do not need SOCKS, HTTP proxy, or WARP software.

## What it does

- Enrolls a free WARP device or a Cloudflare Zero Trust device from the SPR UI
- Runs upstream usque `nativetun` on Linux (`warp0`, MTU 1400)
- Advertises **Cloudflare WARP** through the topology `Sinks` contract so SPR can
  offer the container as a forwarding destination
- Uses a stable custom-interface gateway: `spr-usque` / `172.30.118.2`
- SNATs forwarded LAN traffic to the WARP-assigned address before it enters the TUN
- Supports HTTP/3 over QUIC (default) or usque's HTTP/2 over TCP transport
- Reports live WARP state, Cloudflare colo, public exit IP, endpoint, interface,
  and uptime; the trace probe binds both DNS and HTTPS to `warp0`
- Contributes the gateway and Cloudflare edge to SPR's topology view
- Stores WARP credentials mode 0600 and never returns key material to the UI

## Forwarding architecture

```text
selected SPR traffic
        |
        v
spr-usque bridge / 172.30.118.2
        |
        | policy route table 51820 + nftables SNAT
        v
warp0 (native Linux TUN)
        |
        | CONNECT-IP over MASQUE
        v
Cloudflare WARP
```

Only transit packets arriving on the container's `eth0` use routing table 51820.
usque's own MASQUE connection is locally generated and continues to use the normal
`eth0` underlay, so it cannot recurse into its own tunnel.

The gateway is fail-closed. Table 51820 always contains an unreachable default;
the live `default dev warp0` route has a better metric and exists only while the
TUN is connected. The container forward chain defaults to drop and opens only
`eth0 -> warp0` plus established replies. If usque, WARP, or `warp0` disappears,
selected traffic is rejected instead of falling through to the ordinary WAN.

## Requirements

- SPR running on Linux
- Host kernel TUN support and `/dev/net/tun`
- Docker/Compose support for device passthrough, `NET_ADMIN`, and container sysctls
- Outbound UDP/443 for HTTP/3, or TCP/443 when HTTP/2 transport is selected

Windows-specific usque code and Wintun are intentionally not included.

## Install from the SPR UI

1. Open **Plugins**, choose **+ New Plugin**, and enter the spr-usque repository URL.
2. Enable the plugin and open **WARP gateway** from the navigation.
3. Enroll a WARP device. A free WARP enrollment needs no credentials; Zero Trust
   enrollment accepts the team JWT.
4. Add devices eligible for this path to the `warp` group.
5. In an SPR forwarding rule, select **Cloudflare WARP** as the destination.

The UI shows the complete path and keeps the destination offline until the native
TUN is connected. Primary gateway discovery uses the fixed IPv4 destination
`172.30.118.2`; WARP IPv6 remains available on `warp0` when enabled, while routed
client IPv6 additionally depends on the host and SPR forwarding path supporting
IPv6 next hops.

## CLI install

```bash
cd /home/spr/super/plugins/
git clone https://github.com/spr-networks/spr-usque
cd spr-usque
./install.sh
```

The installer builds and starts the container, stores the SPR API token, and
registers the stable container address as the `spr-usque` custom interface with
`wan` and `dns` policies plus the `warp` group.

For a direct firewall API rule, the route-destination shape used by SPR is:

```json
{
  "SrcIP": "<device-ip>",
  "Interface": "spr-usque",
  "RouteDst": "172.30.118.2",
  "Groups": ["warp"]
}
```

Use SPR's UI when possible so the rule is associated with the intended device or
traffic policy.

## API

All endpoints are served on `/state/plugins/spr-usque/socket`; SPR proxies them at
`/plugins/spr-usque/...`.

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/register` | Enroll WARP. Body: `DeviceName`, optional `JWT`, optional `Force`. Accepts the WARP TOS. |
| `GET` | `/status` | Enrollment, process/TUN state, forwarding state, redacted device facts, endpoint, and a live trace bound to `warp0`. |
| `GET` | `/config` | Non-secret tunnel settings. |
| `PUT` | `/config` | Validate and persist settings; restarts a running tunnel. |
| `PUT` | `/tunnel` | Start or stop. Body: `{"Running":true}`. |
| `POST` | `/restart` | Restart the usque native tunnel. |
| `GET` | `/trace` | Raw Cloudflare trace fetched with DNS and HTTPS bound to `warp0`. |
| `GET` | `/topology` | Root, gateway, Cloudflare edge, and `Sinks` forwarding-destination graph. |

## Configuration

`/configs/plugins/spr-usque/settings.json` contains only non-secret settings:

| Field | Default | Meaning |
| --- | --- | --- |
| `EndpointVersion` | `"v4"` | IPv4 or IPv6 underlay used to reach the Cloudflare endpoint |
| `ConnectPort` | `443` | MASQUE endpoint port |
| `Transport` | `"http3"` | HTTP/3 over QUIC, or `"http2"` over TCP/TLS |
| `TunnelIPv6` | `true` | Keep the WARP IPv6 address enabled inside `warp0` |
| `AutoStart` | `true` | Start the tunnel when the container starts |
| `DeviceName` | `"spr-usque"` | Device name used at enrollment |

usque's native credentials live at
`/configs/plugins/spr-usque/config.json` (mode 0600). A successful forced
re-enrollment retains the previous file as `config.json.bak`. Failed
re-enrollment restores the prior credentials automatically.

## Security model

- The container receives only the privileges required by a native Linux gateway:
  `NET_ADMIN` and `/dev/net/tun`; `no-new-privileges` remains enabled.
- No host TCP or UDP ports are published. The management API is available only on
  the SPR-proxied Unix socket.
- nftables accepts only bridge-to-TUN transit and established return traffic.
- Transit traffic is source-NATed on `warp0`, and no forwarding rule permits it to
  exit `eth0`.
- Policy routing retains an unreachable default during startup, disconnects,
  crashes, re-enrollment, and manual stops.
- JWTs, private keys, endpoint public keys, and access tokens are never returned by
  the API. User input is allow-list validated and passed to usque as argv entries,
  never through a shell.

## Upstream and reproducible builds

The Docker image builds upstream usque from the full commit recorded in
`reproducible.env` (currently release `v4.2.0`). Base images, BuildKit, Go
toolchains, the Ubuntu snapshot, and usque are pinned. Linux `amd64` and `arm64`
images are produced by the CI workflow.

```bash
./build_docker_compose.sh
./update-pins.sh
```

`update-pins.sh` refreshes image digests, Go checksums, and the latest usque
release/commit, then synchronizes the Dockerfile defaults for review.

## Development

```bash
cd code && go test ./...
cd frontend && yarn install --frozen-lockfile && yarn run bundle
```

Shell routing scripts can be syntax-checked with `bash -n scripts/*.sh` and
`dash -n scripts/*.sh`. Container-level validation requires a Linux host with
`/dev/net/tun`.
