#!/usr/bin/env bash
set -Eeuo pipefail
SELF_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SELF_DIR/scripts/install/common.sh"
MODE=default
CHECK_ONLY=0
SILENT=0
SYNOLOGY=0
PROFILE=local-server
BINARY_DIR="${COCKPIT_UPS_WOL_BINARY_DIR:-}"
NUT_HOST=""
UPS_NAME=ups
UPS_DRIVER=""
UPS_PORT=""
usage(){ cat <<'USAGE'
Usage: sudo ./install.sh [--tui|--silent] [--synology]
                         [--profile local-server|remote-client|existing]
                         [--nut-host HOST] [--ups-name NAME]
                         [--ups-driver DRIVER] [--ups-port PORT]
                         [--binary-dir DIR] [--check]
USAGE
}
while (($#)); do
  case "$1" in
    --tui) MODE=tui ;;
    --silent) SILENT=1 ;;
    --synology) SYNOLOGY=1 ;;
    --check) CHECK_ONLY=1 ;;
    --profile) shift; [[ $# -gt 0 ]] || die "--profile requires a value"; PROFILE="$1" ;;
    --nut-host) shift; [[ $# -gt 0 ]] || die "--nut-host requires a value"; NUT_HOST="$1" ;;
    --ups-name) shift; [[ $# -gt 0 ]] || die "--ups-name requires a value"; UPS_NAME="$1" ;;
    --ups-driver) shift; [[ $# -gt 0 ]] || die "--ups-driver requires a value"; UPS_DRIVER="$1" ;;
    --ups-port) shift; [[ $# -gt 0 ]] || die "--ups-port requires a value"; UPS_PORT="$1" ;;
    --binary-dir) shift; [[ $# -gt 0 ]] || die "--binary-dir requires a value"; BINARY_DIR="$1" ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
  shift
done
if ((SILENT)) && [[ "$MODE" == tui ]]; then die "--silent and --tui cannot be combined"; fi
[[ "$PROFILE" == existing ]] && PROFILE=existing-nut
case "$PROFILE" in
  local-server|remote-client|existing-nut) ;;
  *) die "invalid profile: $PROFILE" ;;
esac
platform_detect
source "$SELF_DIR/scripts/install/distros/${DISTRO_FAMILY}.sh"
source "$SELF_DIR/scripts/install/nut.sh"
source "$SELF_DIR/scripts/install/discovery.sh"
source "$SELF_DIR/scripts/install/validate.sh"
source "$SELF_DIR/scripts/install/cockpit.sh"
source "$SELF_DIR/scripts/install/transaction.sh"
[[ -f "$SELF_DIR/scripts/install/tui.sh" ]] && source "$SELF_DIR/scripts/install/tui.sh"
if ((CHECK_ONLY)); then installer_self_check; exit 0; fi
require_root
log_init
trap 'install_failure "$LINENO" "$?"' ERR
run_stage(){ local name="$1"; shift; log "stage: $name"; "$@"; log "stage complete: $name"; }
run_stage "resolve profile" resolve_profile_inputs
run_stage "confirmation" confirm_install
run_stage "rollback snapshot" backup_begin
run_stage "packages" install_packages
run_stage "UPS discovery" resolve_local_ups_after_packages
run_stage "project directories" install_project_dirs
run_stage "project binaries" install_project_binaries
run_stage "project configuration" install_project_config
run_stage "Cockpit bundle" install_cockpit_bundle
run_stage "systemd units" install_systemd_units
run_stage "NUT configuration" nut_configure "$PROFILE" "$SYNOLOGY"
run_stage "service autostart" systemd_reload_enable
run_stage "health probation" installation_health_gate
run_stage "initial known-good configuration" mark_initial_known_good
run_stage "commit installation" backup_commit
final_report
