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

KRUN_MAC="02:53:50:52:4b:12"
KRUN_TAP="kusque0"
curl --fail-with-body --silent --show-error "http://127.0.0.1/device?identity=${KRUN_MAC}" \
  -H "Authorization: Bearer ${SPR_API_TOKEN}" -H "Content-Type: application/json" \
  -X PUT --data-raw "{\"MAC\":\"${KRUN_MAC}\",\"Name\":\"spr-usque\",\"Policies\":[\"wan\",\"dns\"],\"Groups\":[\"warp\"]}" >/dev/null
if ! sudo nft get element inet filter dhcp_access "{ \"${KRUN_TAP}\" . ${KRUN_MAC} }" >/dev/null 2>&1; then
  sudo nft add element inet filter dhcp_access "{ \"${KRUN_TAP}\" . ${KRUN_MAC} : accept }"
fi

docker compose -f docker-compose-kvm.yml build
docker compose -f docker-compose-kvm.yml up -d

CONTAINER_IP=
for _ in $(seq 1 30); do
  CONTAINER_IP="$(jq -r --arg mac "$KRUN_MAC" '.[$mac].RecentIP // empty' "$SUPERDIR/state/public/devices-public.json")"
  [ -n "$CONTAINER_IP" ] && break
  sleep 1
done
[ -n "$CONTAINER_IP" ] || { echo "spr-usque did not obtain an SPR DHCP lease" >&2; exit 1; }
API=127.0.0.1

# Grant the gateway's underlay connection wan+dns access. The topology endpoint
# separately advertises this stable IP as an SPR forwarding destination.
curl "http://${API}/firewall/custom_interface" \
-H "Authorization: Bearer ${SPR_API_TOKEN}" \
-H 'Content-Type: application/json' \
-X 'PUT' \
--data-raw "{\"SrcIP\":\"${CONTAINER_IP}\",\"Interface\":\"${KRUN_TAP}\",\"Policies\":[\"wan\",\"dns\"],\"Groups\":[\"warp\"]}"

echo ""
echo "[+] spr-usque installed at forwarding destination ${CONTAINER_IP}."
echo "    Enroll WARP in Plugins > spr-usque, add eligible devices to the"
echo "    'warp' group, then select Cloudflare WARP in an SPR forwarding rule."
