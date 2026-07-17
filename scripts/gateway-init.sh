#!/bin/sh
set -eu

# Policy routing table used only for packets delivered to this container as a
# forwarding destination. The unreachable defaults are deliberate: if warp0
# does not exist, Linux must not fall through to eth0 and leak traffic.
TABLE=51820
PREF=100

# These are guest-kernel settings. The krun Compose override intentionally
# clears Docker host-network sysctls and configures them here instead.
printf '1' > /proc/sys/net/ipv4/ip_forward
printf '1' > /proc/sys/net/ipv6/conf/all/forwarding
printf '0' > /proc/sys/net/ipv4/conf/all/rp_filter
printf '0' > /proc/sys/net/ipv4/conf/default/rp_filter

ip rule del pref "$PREF" 2>/dev/null || true
ip rule add pref "$PREF" iif eth0 lookup "$TABLE"
ip -6 rule del pref "$PREF" 2>/dev/null || true
ip -6 rule add pref "$PREF" iif eth0 lookup "$TABLE"

ip route replace unreachable default metric 32760 table "$TABLE"
ip -6 route replace unreachable default metric 32760 table "$TABLE"

nft delete table inet spr_usque 2>/dev/null || true
nft -f - <<'EOF'
table inet spr_usque {
  chain forward {
    type filter hook forward priority filter; policy drop;
  }

  chain postrouting {
    type nat hook postrouting priority srcnat; policy accept;
  }
}
EOF

umask 077
tmp=/state/plugins/spr-usque/tunnel.state.tmp
{
  printf 'Connected=false\n'
  printf 'UpdatedAt=%s\n' "$(date +%s)"
} > "$tmp"
mv "$tmp" /state/plugins/spr-usque/tunnel.state
