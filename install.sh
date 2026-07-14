#!/bin/bash
# Command line install alternative to the SPR UI plugin installer.
set -euo pipefail
cd "$(dirname "$0")"

echo "Please enter your SPR path (/home/spr/super/)"
read -r SUPERDIR

if [ -z "$SUPERDIR" ]; then
    SUPERDIR="/home/spr/super/"
fi

export SUPERDIR

echo "Please enter your SPR API token:"
read -r SPR_API_TOKEN

if [ -z "$SPR_API_TOKEN" ]; then
  echo "need api token, generate one on the auth keys page"
  exit 1
fi

CONFIG_DIR="$SUPERDIR/configs/plugins/spr-usque"
STATE_DIR="$SUPERDIR/state/plugins/spr-usque"
mkdir -p "$CONFIG_DIR" "$STATE_DIR"

# Token used by SPR to talk to the plugin (InstallTokenPath convention).
printf '%s' "$SPR_API_TOKEN" > "$CONFIG_DIR/api-token"
chmod 600 "$CONFIG_DIR/api-token"

docker compose build
docker compose up -d

CONTAINER_IP=$(docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "spr-usque")
API=127.0.0.1

# Grant the gateway's underlay connection wan+dns access. The topology endpoint
# separately advertises this stable IP as an SPR forwarding destination.
curl "http://${API}/firewall/custom_interface" \
-H "Authorization: Bearer ${SPR_API_TOKEN}" \
-H 'Content-Type: application/json' \
-X 'PUT' \
--data-raw "{\"SrcIP\":\"${CONTAINER_IP}\",\"Interface\":\"spr-usque\",\"Policies\":[\"wan\",\"dns\"],\"Groups\":[\"warp\"]}"

echo ""
echo "[+] spr-usque installed at forwarding destination ${CONTAINER_IP}."
echo "    Enroll WARP in Plugins > spr-usque, add eligible devices to the"
echo "    'warp' group, then select Cloudflare WARP in an SPR forwarding rule."
