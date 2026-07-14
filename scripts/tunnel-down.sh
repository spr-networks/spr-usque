#!/bin/sh
set -eu

TABLE=51820
OIF_PREF=101
TUN_IFACE=${USQUE_IFACE:-warp0}

# Remove the live TUN route but retain the unreachable default installed by
# gateway-init.sh. Transit traffic therefore fails closed until reconnect.
ip route del default dev "$TUN_IFACE" table "$TABLE" 2>/dev/null || true
ip -6 route del default dev "$TUN_IFACE" table "$TABLE" 2>/dev/null || true
ip rule del pref "$OIF_PREF" 2>/dev/null || true
ip -6 rule del pref "$OIF_PREF" 2>/dev/null || true

nft flush chain inet spr_usque forward 2>/dev/null || true
nft flush chain inet spr_usque postrouting 2>/dev/null || true

umask 077
tmp=/state/plugins/spr-usque/tunnel.state.tmp
{
  printf 'Connected=false\n'
  printf 'Interface=%s\n' "$TUN_IFACE"
  printf 'Endpoint=%s\n' "${USQUE_ENDPOINT:-}"
  printf 'IPv4=%s\n' "${USQUE_IPV4:-}"
  printf 'IPv6=%s\n' "${USQUE_IPV6:-}"
  printf 'UpdatedAt=%s\n' "$(date +%s)"
} > "$tmp"
mv "$tmp" /state/plugins/spr-usque/tunnel.state
