#!/usr/bin/env bash
set -Eeuo pipefail

UPS_TARGET="ups@localhost"
CTL=""
CONFIG="/etc/cockpit-ups-wol/config.yaml"

usage(){
  cat <<'EOF'
Usage: sudo ./scripts/test/hardware-preflight.sh [--ups-target ups@host[:port]] [--ctl PATH] [--config PATH]

Non-destructive preflight for physical UPS acceptance. It does not arm the
system, request FSD, shut down hosts, switch UPS output, or send Wake-on-LAN.
EOF
}

while (($#)); do
  case "$1" in
    --ups-target) shift; UPS_TARGET="${1:?--ups-target requires a value}" ;;
    --ctl) shift; CTL="${1:?--ctl requires a value}" ;;
    --config) shift; CONFIG="${1:?--config requires a value}" ;;
    -h|--help) usage; exit 0 ;;
    *) echo "hardware-preflight: unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

if [[ -z "$CTL" ]]; then
  if command -v cockpit-ups-wolctl >/dev/null 2>&1; then
    CTL="$(command -v cockpit-ups-wolctl)"
  elif [[ -x /usr/libexec/cockpit-ups-wol/cockpit-ups-wolctl ]]; then
    CTL=/usr/libexec/cockpit-ups-wol/cockpit-ups-wolctl
  else
    echo 'FAIL: cockpit-ups-wolctl not found' >&2
    exit 1
  fi
fi

fail=0
pass(){ printf 'PASS: %s\n' "$*"; }
warn(){ printf 'WARN: %s\n' "$*"; }
check(){
  local desc="$1"; shift
  if "$@" >/dev/null 2>&1; then pass "$desc"; else echo "FAIL: $desc" >&2; fail=1; fi
}

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) pass "supported amd64 architecture ($arch)" ;;
  aarch64|arm64) pass "supported arm64 architecture ($arch)" ;;
  riscv64) pass "supported riscv64 architecture ($arch)" ;;
  *) echo "FAIL: unsupported architecture $arch" >&2; fail=1 ;;
esac

check 'cockpit-ups-wol-agent.service enabled' systemctl is-enabled --quiet cockpit-ups-wol-agent.service
check 'cockpit-ups-wol-agent.service active' systemctl is-active --quiet cockpit-ups-wol-agent.service
check 'cockpit-ups-wol-health.timer enabled' systemctl is-enabled --quiet cockpit-ups-wol-health.timer
check 'cockpit-ups-wol-health.timer active' systemctl is-active --quiet cockpit-ups-wol-health.timer

if ! command -v upsc >/dev/null 2>&1; then
  echo 'FAIL: upsc not found' >&2
  fail=1
else
  if ups_out="$(upsc "$UPS_TARGET" 2>/dev/null)"; then
    raw_status="$(awk -F': ' '$1=="ups.status"{print $2}' <<<"$ups_out")"
    if [[ -n "$raw_status" ]]; then
      pass "NUT status available ($raw_status)"
    else
      echo 'FAIL: NUT response has no ups.status' >&2
      fail=1
    fi
    charge="$(awk -F': ' '$1=="battery.charge"{print $2}' <<<"$ups_out")"
    runtime="$(awk -F': ' '$1=="battery.runtime"{print $2}' <<<"$ups_out")"
    [[ -n "$charge" ]] && pass "UPS reports battery.charge=$charge%" || warn 'UPS does not report battery.charge; verify configured recovery fallback'
    [[ -n "$runtime" ]] && pass "UPS reports battery.runtime=${runtime}s" || warn 'UPS does not report battery.runtime'
  else
    echo "FAIL: cannot query NUT target $UPS_TARGET" >&2
    fail=1
  fi
fi

if health="$($CTL health 2>/dev/null)" && grep -q '"ok": true' <<<"$health"; then
  pass 'agent IPC health query succeeds'
  if grep -q '"state": "FAILED_SAFE"' <<<"$health"; then
    echo 'FAIL: agent reports FAILED_SAFE' >&2
    fail=1
  fi
else
  echo 'FAIL: agent IPC health query failed' >&2
  fail=1
fi

if status="$($CTL --config "$CONFIG" config-status 2>/dev/null)"; then
  active="$(sed -n 's/.*"active": "\([^"]*\)".*/\1/p' <<<"$status" | head -n1)"
  lkg="$(sed -n 's/.*"last_known_good": "\([^"]*\)".*/\1/p' <<<"$status" | head -n1)"
  if [[ -n "$active" && "$active" == "$lkg" ]]; then
    pass "active configuration is last-known-good ($active)"
  else
    echo "FAIL: active revision ($active) differs from last-known-good ($lkg)" >&2
    fail=1
  fi
else
  echo 'FAIL: cannot read configuration revision status' >&2
  fail=1
fi

if plan="$($CTL --config "$CONFIG" plan 2>/dev/null)"; then
  mode="$(sed -n 's/.*"mode": "\([^"]*\)".*/\1/p' <<<"$plan" | head -n1)"
  pass "power plan parses successfully (mode=$mode)"
  printf '%s\n' "$plan" > /tmp/cockpit-ups-wol-hardware-plan.json
  warn 'sanitized power plan saved to /tmp/cockpit-ups-wol-hardware-plan.json for review'
else
  echo 'FAIL: power plan cannot be generated' >&2
  fail=1
fi

cat <<'EOF'

MANUAL CHECKS REQUIRED BEFORE PHYSICAL OUTAGE TEST:
  [ ] controller SBC power supply is connected to a UPS battery-backed output
  [ ] required switch/router/VLAN path remains powered for the shutdown sequence
  [ ] controller automatically boots when UPS output is restored
  [ ] Wake-on-LAN is enabled in target firmware/NIC/OS where required
  [ ] the test starts in dry-run until the displayed plan has been reviewed
  [ ] emergency local access is available before testing real FSD/output-off
EOF

if ((fail)); then
  echo 'hardware-preflight: FAIL' >&2
  exit 1
fi
echo 'hardware-preflight: SOFTWARE PREFLIGHT PASS (manual checks still required)'
