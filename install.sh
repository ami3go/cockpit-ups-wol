#!/usr/bin/env bash
set -Eeuo pipefail
SELF_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source "$SELF_DIR/scripts/install/common.sh"
MODE=default; CHECK_ONLY=0; SILENT=0; SYNOLOGY=0; PROFILE=local-server
BINARY_DIR="${COCKPIT_UPS_WOL_BINARY_DIR:-}"
usage(){ cat <<'USAGE'
Usage: sudo ./install.sh [--tui|--silent] [--synology] [--profile local-server|remote-client|existing-nut] [--binary-dir DIR] [--check]
USAGE
}
while (($#)); do
 case "$1" in
  --tui) MODE=tui;; --silent) SILENT=1;; --synology) SYNOLOGY=1;; --check) CHECK_ONLY=1;;
  --profile) shift; [[ $# -gt 0 ]]||die "--profile requires a value"; PROFILE="$1";;
  --binary-dir) shift; [[ $# -gt 0 ]]||die "--binary-dir requires a value"; BINARY_DIR="$1";;
  -h|--help) usage; exit 0;; *) die "unknown option: $1";; esac; shift; done
case "$PROFILE" in local-server|remote-client|existing-nut);; *) die "invalid profile: $PROFILE";; esac
platform_detect
source "$SELF_DIR/scripts/install/distros/${DISTRO_FAMILY}.sh"
source "$SELF_DIR/scripts/install/nut.sh"
source "$SELF_DIR/scripts/install/validate.sh"
if ((CHECK_ONLY)); then installer_self_check; exit 0; fi
require_root; log_init; trap 'install_failure "$LINENO" "$?"' ERR
confirm_install; backup_begin; install_packages; install_project_dirs; install_project_binaries; install_project_config
install_systemd_units; nut_configure "$PROFILE" "$SYNOLOGY"; systemd_reload_enable; installation_health_gate; backup_commit; final_report
