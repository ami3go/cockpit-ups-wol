#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
SELF_DIR="$ROOT"
source "$ROOT/scripts/install/common.sh"
source "$ROOT/scripts/install/nut.sh"
source "$ROOT/scripts/install/discovery.sh"
source "$ROOT/scripts/install/configuration.sh"

fail(){ echo "installer-unit: FAIL: $*" >&2; exit 1; }
assert_contains(){ local hay="$1" needle="$2"; grep -Fq -- "$needle" <<<"$hay" || fail "missing: $needle"; }

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

# A sole discovered device may be selected unattended; ambiguous discovery must
# fail rather than guess.
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

# Render the canonical project config from the exact values selected for NUT.
tmp="$(mktemp -d)"
trap 'rm -f "$fixture"; rm -rf "$tmp"' EXIT
ETC_DIR="$tmp/etc"
mkdir -p "$ETC_DIR"
PROFILE=local-server; UPS_NAME=labups; NUT_HOST=localhost; UPS_DRIVER=nutdrv_qx; UPS_PORT=auto; SYNOLOGY=0
install_project_config
cfg="$(cat "$ETC_DIR/config.yaml")"
assert_contains "$cfg" '  profile: local-server'
assert_contains "$cfg" '  ups_name: labups'
assert_contains "$cfg" '  driver: nutdrv_qx'
assert_contains "$cfg" '  driver_port: auto'

# The NUT file must receive the same selected values.
nutdir="$tmp/nut"; UPS_NAME=labups
COCKPIT_UPS_WOL_NUT_ETC_DIR="$nutdir" nut_write_clean_local_server secret 0 "$UPS_DRIVER" "$UPS_PORT"
upsconf="$(cat "$nutdir/ups.conf")"
assert_contains "$upsconf" '[labups]'
assert_contains "$upsconf" '  driver = nutdrv_qx'
assert_contains "$upsconf" '  port = auto'

echo 'installer-unit: PASS'
