#!/bin/sh
set -eu

TABLE=51820
OIF_PREF=101
TUN_IFACE=${USQUE_IFACE:-warp0}

# Locally generated diagnostics bind to warp0. An oif policy rule makes those
# sockets use the same table without moving usque's own MASQUE underlay off
# eth0 (which would recurse back into the tunnel).
ip rule del pref "$OIF_PREF" 2>/dev/null || true
ip rule add pref "$OIF_PREF" oif "$TUN_IFACE" lookup "$TABLE"
ip -6 rule del pref "$OIF_PREF" 2>/dev/null || true
ip -6 rule add pref "$OIF_PREF" oif "$TUN_IFACE" lookup "$TABLE"

ip route replace default dev "$TUN_IFACE" metric 10 table "$TABLE"
ip route replace unreachable default metric 32760 table "$TABLE"
ip -6 route replace default dev "$TUN_IFACE" metric 10 table "$TABLE"
ip -6 route replace unreachable default metric 32760 table "$TABLE"

# Disable strict reverse-path checks on the newly created TUN. Replies from
# the internet legitimately arrive on warp0 even though the main table's
# ordinary default route points at eth0.
if [ -w "/proc/sys/net/ipv4/conf/${TUN_IFACE}/rp_filter" ]; then
  printf '0' > "/proc/sys/net/ipv4/conf/${TUN_IFACE}/rp_filter"
fi

# Only transit traffic between the SPR bridge and WARP is accepted. SNAT is
# required because Cloudflare expects the source assigned to this WARP device,
# not an RFC1918 client address carried from the SPR LAN.
nft -f - <<EOF
flush chain inet spr_usque forward
flush chain inet spr_usque postrouting
add rule inet spr_usque forward iifname "eth0" oifname "${TUN_IFACE}" ct state new,established,related accept
add rule inet spr_usque forward iifname "${TUN_IFACE}" oifname "eth0" ct state established,related accept
add rule inet spr_usque postrouting meta nfproto ipv4 oifname "${TUN_IFACE}" masquerade
add rule inet spr_usque postrouting meta nfproto ipv6 oifname "${TUN_IFACE}" masquerade
EOF

umask 077
tmp=/state/plugins/spr-usque/tunnel.state.tmp
{
  printf 'Connected=true\n'
  printf 'Interface=%s\n' "$TUN_IFACE"
  printf 'Endpoint=%s\n' "${USQUE_ENDPOINT:-}"
  printf 'IPv4=%s\n' "${USQUE_IPV4:-}"
  printf 'IPv6=%s\n' "${USQUE_IPV6:-}"
  printf 'ConnectedAt=%s\n' "$(date +%s)"
  printf 'UpdatedAt=%s\n' "$(date +%s)"
} > "$tmp"
mv "$tmp" /state/plugins/spr-usque/tunnel.state
