#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
SELF_DIR="$ROOT"
source "$ROOT/scripts/install/common.sh"
source "$ROOT/scripts/install/nut.sh"
source "$ROOT/scripts/install/discovery.sh"
source "$ROOT/scripts/install/network.sh"
source "$ROOT/scripts/install/configuration.sh"

fail(){ echo "installer-unit: FAIL: $*" >&2; exit 1; }
assert_contains(){ local hay="$1" needle="$2"; grep -Fq -- "$needle" <<<"$hay" || fail "missing: $needle"; }

# Test-only environment seams must never affect a production root install
# unless the caller explicitly opts into controlled test mode.
if (unset COCKPIT_UPS_WOL_TEST_MODE; COCKPIT_UPS_WOL_PROBATION_SECONDS=1 validate_test_overrides) >/dev/null 2>&1; then
  fail "test-only override unexpectedly accepted without test mode"
fi
if ! (COCKPIT_UPS_WOL_TEST_MODE=1 COCKPIT_UPS_WOL_PROBATION_SECONDS=1 validate_test_overrides) >/dev/null 2>&1; then
  fail "explicit test mode did not allow test-only override"
fi

fixture="$(mktemp)"
trap 'rm -f "$fixture"' EXIT
cat >"$fixture" <<'EOF'
[nutdev-usb1]
  driver = "usbhid-ups"
  port = "auto"
  vendor = "Eaton"
  product = "5E"

[nutdev-usb2]
  driver = "blazer_usb"
  port = "auto"
  vendor = "INNO TECH"
  product = "USB to Serial"
EOF
mapfile -t parsed < <(parse_nut_scanner_output <"$fixture")
[[ ${#parsed[@]} -eq 2 ]] || fail "expected 2 scanner records, got ${#parsed[@]}"
assert_contains "${parsed[0]}" $'nutdev-usb1\tusbhid-ups\tauto\tEaton 5E'
assert_contains "${parsed[1]}" $'nutdev-usb2\tblazer_usb\tauto\tINNO TECH USB to Serial'

PROFILE=local-server; MODE=default; SILENT=1; UPS_DRIVER=""; UPS_PORT=""
cat >"$fixture" <<'EOF'
[nutdev-usb1]
  driver = "usbhid-ups"
  port = "auto"
  vendor = "Test"
  product = "UPS"
EOF
COCKPIT_UPS_WOL_NUT_SCAN_FIXTURE="$fixture" resolve_local_ups_after_packages
[[ "$UPS_DRIVER" == usbhid-ups && "$UPS_PORT" == auto ]] || fail "sole discovery was not selected"

cat >"$fixture" <<'EOF'
[nutdev-usb1]
  driver = "usbhid-ups"
  port = "auto"
[nutdev-usb2]
  driver = "blazer_usb"
  port = "auto"
EOF
if (UPS_DRIVER=""; UPS_PORT=""; COCKPIT_UPS_WOL_NUT_SCAN_FIXTURE="$fixture" resolve_local_ups_after_packages) >/dev/null 2>&1; then
  fail "ambiguous silent discovery unexpectedly succeeded"
fi

tmp="$(mktemp -d)"
trap 'rm -f "$fixture"; rm -rf "$tmp"' EXIT
ETC_DIR="$tmp/etc"
mkdir -p "$ETC_DIR"
PROFILE=local-server; UPS_NAME=labups; NUT_HOST=localhost; UPS_DRIVER=nutdrv_qx; UPS_PORT=auto; SYNOLOGY=0
NETWORK_MODE=restricted; NUT_LISTEN_IPV4=1; NUT_LISTEN_IPV6=1; NUT_ALLOWED_CLIENTS=(192.168.10.0/24 fd00:1234::/64)
OPERATING_MODE=dry-run; OUTAGE_GRACE=120; RECOVERY_CHARGE=80; UTILITY_STABLE=120; NETWORK_WAIT=300
validate_network_inputs
install_project_config
cfg="$(cat "$ETC_DIR/config.yaml")"
assert_contains "$cfg" '  profile: local-server'
assert_contains "$cfg" '  ups_name: labups'
assert_contains "$cfg" '  driver: nutdrv_qx'
assert_contains "$cfg" '  driver_port: auto'
assert_contains "$cfg" '    mode: restricted'
assert_contains "$cfg" '    listen_ipv4: true'
assert_contains "$cfg" '    listen_ipv6: true'
assert_contains "$cfg" '    allowed_clients: [192.168.10.0/24, fd00:1234::/64]'
assert_contains "$cfg" 'network_dependencies: []'
assert_contains "$cfg" 'hosts: []'
if grep -Eq '192\.168\.1\.(2|20|30)|AA:BB:CC:DD:EE:FF|11:22:33:44:55:66' <<<"$cfg"; then
  fail 'sample inventory leaked into live generated config'
fi

nutdir="$tmp/nut"; UPS_NAME=labups
COCKPIT_UPS_WOL_NUT_ETC_DIR="$nutdir" nut_write_clean_local_server secret 0 "$UPS_DRIVER" "$UPS_PORT"
upsconf="$(cat "$nutdir/ups.conf")"
upsdconf="$(cat "$nutdir/upsd.conf")"
upsmon="$(cat "$nutdir/upsmon.conf")"
assert_contains "$upsconf" '[labups]'
assert_contains "$upsconf" '  driver = nutdrv_qx'
assert_contains "$upsconf" '  port = auto'
assert_contains "$upsdconf" 'LISTEN 0.0.0.0 3493'
assert_contains "$upsdconf" 'LISTEN :: 3493'
assert_contains "$upsmon" 'HOSTSYNC 60'
assert_contains "$upsmon" 'FINALDELAY 15'

rules="$tmp/firewall.nft"
render_restricted_nft "$rules"
nft_text="$(cat "$rules")"
assert_contains "$nft_text" 'table inet cockpit_ups_wol {'
assert_contains "$nft_text" 'ip saddr 192.168.10.0/24 tcp dport 3493 accept'
assert_contains "$nft_text" 'ip6 saddr fd00:1234::/64 tcp dport 3493 accept'
assert_contains "$nft_text" 'meta nfproto ipv4 tcp dport 3493 drop'
assert_contains "$nft_text" 'meta nfproto ipv6 tcp dport 3493 drop'
if grep -Eq 'flush[[:space:]]+ruleset' "$rules"; then fail 'restricted policy must not flush host firewall'; fi

# Durable installer transaction marker: once armed, it points at a complete
# snapshot and survives until backup_commit is the durable commit point.
BACKUP_BASE="$tmp/backups"
STATE_DIR="$tmp/state"
SYSTEMD_DIR="$tmp/systemd"
LIBEXEC_DIR="$tmp/libexec"
COCKPIT_UI_DIR="$tmp/cockpit"
CURRENT_BACKUP="$BACKUP_BASE/snapshot-test"
install -d -m0700 "$CURRENT_BACKUP"
touch "$CURRENT_BACKUP/absent.list"
source "$ROOT/scripts/install/transaction.sh"
install_pending_begin
[[ -f "$INSTALL_PENDING_MARKER" ]] || fail 'install-pending marker was not created'
[[ "$(cat "$INSTALL_PENDING_MARKER")" == "$CURRENT_BACKUP" ]] || fail 'install-pending marker points at wrong snapshot'
INSTALL_COMMITTED=0
backup_commit
[[ ! -e "$INSTALL_PENDING_MARKER" ]] || fail 'backup_commit did not clear durable install marker'
[[ "$INSTALL_COMMITTED" -eq 1 ]] || fail 'backup_commit did not set commit state'

echo 'installer-unit: PASS'
