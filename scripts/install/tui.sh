#!/usr/bin/env bash

TUI_BIN=""
TUI_CONFIRM_UPS_BACKED=0
TUI_CONFIRM_AUTO_POWER=0
TUI_CONFIRM_NETWORK_POWER=0

_tui_run(){
  if [[ "$TUI_BIN" == dialog ]]; then
    dialog --stdout "$@"
  else
    whiptail "$@" 3>&1 1>&2 2>&3
  fi
}

tui_prepare_backend(){
  if command -v dialog >/dev/null 2>&1; then TUI_BIN=dialog; return 0; fi
  if command -v whiptail >/dev/null 2>&1; then TUI_BIN=whiptail; return 0; fi
  log "TUI backend missing; installing dialog bootstrap package"
  case "$DISTRO_FAMILY" in
    debian)
      export DEBIAN_FRONTEND=noninteractive
      apt-get update
      apt-get install -y --no-install-recommends dialog
      ;;
    arch) pacman -Sy --needed --noconfirm dialog ;;
    fedora) dnf install -y dialog ;;
    *) die "cannot bootstrap TUI backend on $DISTRO_FAMILY" ;;
  esac
  command -v dialog >/dev/null 2>&1 || die "dialog installation succeeded but command is unavailable"
  TUI_BIN=dialog
}

tui_msg(){ _tui_run --title "$1" --msgbox "$2" 18 76; }
tui_yesno(){ _tui_run --title "$1" --yesno "$2" 18 76; }
tui_input(){ _tui_run --title "$1" --inputbox "$2" 12 76 "${3:-}"; }
tui_menu(){ local title="$1" text="$2"; shift 2; _tui_run --title "$title" --menu "$text" 20 82 10 "$@"; }
tui_progress(){ [[ "$MODE" == tui && -n "$TUI_BIN" ]] || return 0; _tui_run --title 'Installation progress' --infobox "$1" 7 70 || true; }

tui_parse_clients(){
  local raw="$1" item
  NUT_ALLOWED_CLIENTS=()
  raw="${raw//,/ }"
  for item in $raw; do NUT_ALLOWED_CLIENTS+=("$item"); done
}

tui_wizard(){
  tui_msg 'System summary' "cockpit-ups-wol installer\n\nPlatform: $DISTRO_FAMILY / $TARGET_ARCH\nController: $(hostname)\n\nThe controller should be powered from a UPS battery-backed outlet and required network infrastructure should remain powered during shutdown."
  tui_msg 'Package plan' "The installer will ensure Cockpit, Network UPS Tools (NUT), required utilities and project services are installed.\n\nProject configuration and application artifacts are transactional and roll back if probation fails. Package-manager dependency installation itself is not removed on rollback."

  PROFILE="$(tui_menu 'UPS / NUT profile' 'Select how this controller obtains UPS data.' \
    local-server 'USB/local UPS; this controller runs the NUT server and primary monitor' \
    remote-client 'Read UPS data from another NUT server' \
    existing-nut 'Use existing NUT configuration without rewriting it')" || exit 1

  if [[ "$PROFILE" == remote-client ]]; then
    NUT_HOST="$(tui_input 'Remote NUT server' 'Hostname or IP address of the existing NUT server.' "${NUT_HOST:-}")" || exit 1
  else
    NUT_HOST=localhost
  fi

  UPS_NAME="$(tui_input 'UPS name' 'Logical NUT UPS name. Synology compatibility requires the name ups.' "$UPS_NAME")" || exit 1
  if tui_yesno 'Synology compatibility' 'Enable the Synology DSM NUT-secondary compatibility account?\n\nThis uses the compatibility username/password expected by DSM and should only be exposed on a trusted or restricted LAN.'; then
    SYNOLOGY=1
    UPS_NAME=ups
  else
    SYNOLOGY=0
  fi

  if [[ "$PROFILE" == local-server ]]; then
    NETWORK_MODE="$(tui_menu 'NUT network security' 'Select how TCP port 3493 is exposed.' \
      trusted-lan 'Listen on enabled address families; rely on trusted LAN perimeter' \
      restricted 'Install additive project-owned nftables rules for explicit client CIDRs')" || exit 1
    if [[ "$NETWORK_MODE" == restricted ]]; then
      if tui_yesno 'IPv6' 'Also listen for NUT clients over IPv6?\n\nIf enabled, IPv6 is protected by the same project-owned restricted policy.'; then NUT_LISTEN_IPV6=1; else NUT_LISTEN_IPV6=0; fi
      local clients
      clients="$(tui_input 'Allowed NUT clients' 'Enter one or more allowed client CIDRs separated by spaces or commas.\nExample: 192.168.1.0/24 fd00:1234::/64' '')" || exit 1
      tui_parse_clients "$clients"
      ((${#NUT_ALLOWED_CLIENTS[@]} > 0)) || { tui_msg 'Input required' 'Restricted mode requires at least one client CIDR.'; exit 1; }
    else
      NUT_ALLOWED_CLIENTS=()
      NUT_LISTEN_IPV6=0
    fi
  fi

  OUTAGE_GRACE="$(tui_input 'Outage policy' 'Grace period in seconds before configured shutdown thresholds may commit the outage.' "$OUTAGE_GRACE")" || exit 1
  RECOVERY_CHARGE="$(tui_input 'Recovery policy' 'Minimum UPS battery charge percentage required to begin automatic recovery.' "$RECOVERY_CHARGE")" || exit 1
  UTILITY_STABLE="$(tui_input 'Recovery policy' 'How many continuous seconds utility power must remain stable before recovery.' "$UTILITY_STABLE")" || exit 1
  NETWORK_WAIT="$(tui_input 'Recovery policy' 'Maximum network-readiness wait in seconds during recovery.' "$NETWORK_WAIT")" || exit 1

  tui_yesno 'Controller power' 'Confirm the controller is/will be connected to a UPS battery-backed output (not surge-only).' && TUI_CONFIRM_UPS_BACKED=1 || TUI_CONFIRM_UPS_BACKED=0
  tui_yesno 'Automatic boot' 'Confirm the controller automatically boots when UPS output power returns.' && TUI_CONFIRM_AUTO_POWER=1 || TUI_CONFIRM_AUTO_POWER=0
  tui_yesno 'Network power' 'Confirm the switch/router/VLAN path needed for shutdown coordination remains powered long enough.' && TUI_CONFIRM_NETWORK_POWER=1 || TUI_CONFIRM_NETWORK_POWER=0

  tui_msg 'Network dependencies' 'Fresh installation intentionally creates no managed network dependencies.\n\nAdd the real switch/router readiness dependencies in Cockpit after installation; sample addresses are never copied into live configuration.'
  tui_msg 'Managed hosts' 'Fresh installation intentionally creates no managed hosts.\n\nEnroll the real Synology/Proxmox/workstation targets in Cockpit, validate their status/shutdown/WoL settings, then review the complete power plan before arming.'

  OPERATING_MODE="$(tui_menu 'Initial operating mode' 'Choose the safe initial mode. Armed mode is intentionally enabled only after post-install validation.' \
    dry-run 'Evaluate the complete policy and log intended actions; execute no destructive actions' \
    monitor 'Observe UPS and host state only' \
    maintenance 'Suppress automatic power actions during maintenance')" || exit 1
}

tui_review_plan(){
  local clients='none'
  ((${#NUT_ALLOWED_CLIENTS[@]})) && clients="${NUT_ALLOWED_CLIENTS[*]}"
  local topology="UPS-backed=$TUI_CONFIRM_UPS_BACKED, auto-boot=$TUI_CONFIRM_AUTO_POWER, network-backed=$TUI_CONFIRM_NETWORK_POWER"
  local ups_detail='existing/remote NUT'
  [[ "$PROFILE" == local-server ]] && ups_detail="driver=${UPS_DRIVER:-auto-discover}, port=${UPS_PORT:-auto-discover}"
  tui_yesno 'Review installation plan' "Platform: $DISTRO_FAMILY/$TARGET_ARCH\nProfile: $PROFILE\nUPS: $UPS_NAME@$NUT_HOST\nUPS setup: $ups_detail\nSynology: $SYNOLOGY\nNetwork mode: $NETWORK_MODE\nAllowed clients: $clients\nOutage grace: ${OUTAGE_GRACE}s\nRecovery: >=${RECOVERY_CHARGE}% after ${UTILITY_STABLE}s stable utility\nNetwork wait: ${NETWORK_WAIT}s\nInitial mode: $OPERATING_MODE\nTopology confirmations: $topology\nManaged hosts: none (enroll post-install)\n\nProceed with the transactional installation?"
}

confirm_install(){
  ((SILENT)) && return 0
  if [[ "$MODE" == tui ]]; then tui_review_plan || exit 1; return 0; fi
  local text="Install cockpit-ups-wol in ${PROFILE} mode on ${DISTRO_FAMILY}/${TARGET_ARCH}? New installs contain no managed hosts and default to ${OPERATING_MODE}."
  read -r -p "$text [y/N] " r
  [[ "$r" =~ ^[Yy]$ ]] || exit 1
}

tui_post_discovery_review(){
  [[ "$MODE" == tui && "$PROFILE" == local-server ]] || return 0
  tui_yesno 'UPS selection confirmed' "NUT discovery/selection resolved:\n\nUPS name: $UPS_NAME\nDriver: $UPS_DRIVER\nPort: $UPS_PORT\n\nContinue with configuration and service activation?" || exit 1
}
