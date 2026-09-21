#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
export COCKPIT_UPS_WOL_TEST_MODE=1
DIST="$ROOT/.e2e-dist"; BAD="$ROOT/.e2e-bad-dist"; BAD_UI="$ROOT/.e2e-bad-ui"
cleanup(){ sudo systemctl stop nut-driver.target nut-server.service >/dev/null 2>&1||true; }
trap cleanup EXIT
rm -rf "$DIST" "$BAD" "$BAD_UI"; mkdir -p "$DIST" "$BAD" "$BAD_UI"
for n in cockpit-ups-wol-agent cockpit-ups-wolctl cockpit-ups-wol-health wolctl; do (cd "$ROOT/agent"&&CGO_ENABLED=0 go build -o "$DIST/$n" "./cmd/$n"); done
[[ -s "$ROOT/cockpit/dist/manifest.json" && -s "$ROOT/cockpit/dist/index.js" ]] || { echo 'Cockpit bundle missing'; exit 1; }
sudo apt-get update
sudo apt-get install -y --no-install-recommends nut-server nut-client
sudo systemctl stop nut-server.service nut-monitor.service nut-driver.target >/dev/null 2>&1||true
sudo install -d -m0750 -o root -g nut /etc/nut
sudo install -d -m0770 -o nut -g nut /run/nut
sudo install -m0644 "$ROOT/tests/nut-sim/online.dev" /etc/nut/online.dev
printf 'MODE=netserver\n'|sudo tee /etc/nut/nut.conf >/dev/null
cat <<'EOF'|sudo tee /etc/nut/ups.conf >/dev/null
[ups]
  driver = dummy-ups
  port = /etc/nut/online.dev
  mode = dummy-once
EOF
cat <<'EOF'|sudo tee /etc/nut/upsd.conf >/dev/null
LISTEN 127.0.0.1 3493
EOF
sudo chown root:nut /etc/nut/nut.conf /etc/nut/ups.conf /etc/nut/upsd.conf
sudo chmod 0640 /etc/nut/nut.conf /etc/nut/ups.conf /etc/nut/upsd.conf
sudo systemctl restart nut-driver-enumerator.service
sudo systemctl restart nut-server.service
NUT_READY=0
for _ in {1..30}; do
  if UPS_OUT="$(upsc ups@localhost 2>/dev/null)" && grep -q '^ups.status: OL$' <<<"$UPS_OUT"; then
    NUT_READY=1
    break
  fi
  sleep 1
done
if (( ! NUT_READY )); then
  echo 'NUT dummy UPS did not reach a valid OL state' >&2
  sudo systemctl --no-pager --full status nut-driver-enumerator.service nut-driver.target nut-server.service || true
  sudo journalctl --no-pager -n 200 -u 'nut-driver@*' -u nut-driver-enumerator.service -u nut-server.service || true
  exit 1
fi
COCKPIT_UPS_WOL_REQUIRE_UI=1 COCKPIT_UPS_WOL_PROBATION_SECONDS=5 sudo -E "$ROOT/install.sh" --silent --profile existing-nut --binary-dir "$DIST"
sudo systemctl is-enabled --quiet cockpit-ups-wol-agent.service
sudo systemctl is-active --quiet cockpit-ups-wol-agent.service
sudo systemctl is-enabled --quiet cockpit-ups-wol-health.timer
sudo "$DIST/cockpit-ups-wolctl" --socket /run/cockpit-ups-wol/agent.sock health >/dev/null
test -s /usr/share/cockpit/cockpit-ups-wol/manifest.json
test -s /usr/share/cockpit/cockpit-ups-wol/index.js
LKG1="$(sudo cat /var/lib/cockpit-ups-wol/config-history/last-known-good)"; [[ -n "$LKG1" ]]
GOOD_HASH="$(sha256sum "$DIST/cockpit-ups-wol-agent"|awk '{print $1}')"
GOOD_UI_HASH="$(sudo sha256sum /usr/share/cockpit/cockpit-ups-wol/index.js|awk '{print $1}')"
COCKPIT_UPS_WOL_REQUIRE_UI=1 COCKPIT_UPS_WOL_PROBATION_SECONDS=3 sudo -E "$ROOT/install.sh" --silent --profile existing-nut --binary-dir "$DIST"
LKG2="$(sudo cat /var/lib/cockpit-ups-wol/config-history/last-known-good)"; [[ "$LKG1" == "$LKG2" ]]
cp -a "$DIST/." "$BAD/"
cat >"$BAD/cockpit-ups-wol-agent" <<'EOF'
#!/usr/bin/env bash
exit 42
EOF
chmod +x "$BAD/cockpit-ups-wol-agent"
cp -a "$ROOT/cockpit/dist/." "$BAD_UI/"
printf '\n/* deliberately bad upgrade marker */\n' >>"$BAD_UI/index.js"
set +e
COCKPIT_UPS_WOL_COCKPIT_DIR="$BAD_UI" COCKPIT_UPS_WOL_REQUIRE_UI=1 COCKPIT_UPS_WOL_PROBATION_SECONDS=2 sudo -E "$ROOT/install.sh" --silent --profile existing-nut --binary-dir "$BAD"
BAD_RC=$?
set -e
[[ "$BAD_RC" -ne 0 ]]
RESTORED_HASH="$(sudo sha256sum /usr/libexec/cockpit-ups-wol/cockpit-ups-wol-agent|awk '{print $1}')"; [[ "$RESTORED_HASH" == "$GOOD_HASH" ]]
RESTORED_UI_HASH="$(sudo sha256sum /usr/share/cockpit/cockpit-ups-wol/index.js|awk '{print $1}')"; [[ "$RESTORED_UI_HASH" == "$GOOD_UI_HASH" ]]
sudo systemctl is-active --quiet cockpit-ups-wol-agent.service
sudo /usr/libexec/cockpit-ups-wol/cockpit-ups-wolctl health >/dev/null
[[ "$(sudo cat /var/lib/cockpit-ups-wol/config-history/last-known-good)" == "$LKG1" ]]
echo 'installer-e2e: PASS'
