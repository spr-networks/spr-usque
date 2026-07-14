#!/bin/bash
set -euo pipefail

set -a
. /configs/base/config.sh
if [ -f /configs/spr-usque/config.sh ]; then
  . /configs/spr-usque/config.sh
fi
set +a

# Keep the routing policy fail-closed before enrollment and between tunnel
# reconnects. tunnel-up.sh opens only the eth0 <-> warp0 forwarding path.
mkdir -p /state/plugins/spr-usque /configs/spr-usque
chmod 700 /state/plugins/spr-usque /configs/spr-usque
/scripts/gateway-init.sh

exec /usque_plugin
