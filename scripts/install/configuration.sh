#!/usr/bin/env bash

resolve_profile_inputs(){
  [[ "$UPS_NAME" =~ ^[A-Za-z0-9._-]+$ ]] || die "invalid UPS name"
  if ((SYNOLOGY)) && [[ "$UPS_NAME" != ups ]]; then
    die "Synology compatibility requires UPS name 'ups'"
  fi
  if [[ "$PROFILE" == remote-client && -z "$NUT_HOST" ]]; then
    if ((CHECK_ONLY)); then
      NUT_HOST=localhost
    elif ((SILENT)); then
      die "remote-client --silent requires --nut-host"
    elif [[ "$MODE" == tui ]] && declare -F tui_input >/dev/null; then
      NUT_HOST="$(tui_input 'Remote NUT server' 'Remote NUT server hostname or IP.' '')" || exit 1
    else
      read -r -p 'Remote NUT server hostname or IP: ' NUT_HOST
    fi
  fi
  [[ -n "$NUT_HOST" ]] || NUT_HOST=localhost
  [[ "$NUT_HOST" =~ ^[A-Za-z0-9._:-]+$ ]] || die "invalid NUT host"
  if ((SYNOLOGY)) && [[ "$NETWORK_MODE" == trusted-lan ]] && (( ! ${ACCEPT_TRUSTED_LAN_SYNOLOGY:-0} )); then
    die "Synology compatibility uses fixed monitor credentials; choose --network-mode restricted or explicitly pass --accept-trusted-lan-synology"
  fi
  if [[ -n "$UPS_DRIVER" || -n "$UPS_PORT" ]]; then
    [[ "$PROFILE" == local-server ]] || die "--ups-driver/--ups-port apply only to local-server profile"
    if ((SILENT)) && { [[ -z "$UPS_DRIVER" ]] || [[ -z "$UPS_PORT" ]]; }; then
      die "--silent requires both --ups-driver and --ups-port when either is supplied"
    fi
  fi
  [[ "$OPERATING_MODE" =~ ^(monitor|dry-run|armed|maintenance)$ ]] || die "invalid operating mode: $OPERATING_MODE"
  [[ "$OUTAGE_GRACE" =~ ^[0-9]+$ ]] || die "invalid outage grace"
  [[ "$RECOVERY_CHARGE" =~ ^[1-9][0-9]?$|^100$ ]] || die "recovery charge must be 1..100"
  [[ "$UTILITY_STABLE" =~ ^[1-9][0-9]*$ ]] || die "utility stable seconds must be > 0"
  [[ "$NETWORK_WAIT" =~ ^[0-9]+$ ]] || die "network wait seconds must be >= 0"
  validate_network_inputs
}

strip_example_inventory(){
  local cfg="$1"
  local tmp="${cfg}.inventory.tmp"
  awk '
    /^network_dependencies:/ {
      print "network_dependencies: []"
      print "hosts: []"
      found=1
      exit
    }
    { print }
    END { if (!found) exit 42 }
  ' "$cfg" >"$tmp" || { rm -f "$tmp"; die "cannot sanitize example inventory"; }
  mv "$tmp" "$cfg"
  chmod 0600 "$cfg"
}

install_project_config(){
  PROJECT_CONFIG_CREATED=0
  if [[ -f "$ETC_DIR/config.yaml" ]]; then
    log "existing project config preserved"
    return 0
  fi

  PROJECT_CONFIG_CREATED=1
  install -m0600 "$SELF_DIR/config/config.yaml.example" "$ETC_DIR/config.yaml"
  local cfg_profile="$PROFILE" driver_value port_value allowed_yaml ipv4_yaml ipv6_yaml
  [[ "$cfg_profile" == existing-nut ]] && cfg_profile=existing
  case "$PROFILE" in
    local-server)
      [[ -n "$UPS_DRIVER" && -n "$UPS_PORT" ]] || die "local-server project config requires resolved UPS driver and port"
      driver_value="$UPS_DRIVER"
      port_value="$UPS_PORT"
      ;;
    *)
      driver_value=null
      port_value=null
      ;;
  esac
  allowed_yaml="$(network_allowed_yaml)"
  ((NUT_LISTEN_IPV4)) && ipv4_yaml=true || ipv4_yaml=false
  ((NUT_LISTEN_IPV6)) && ipv6_yaml=true || ipv6_yaml=false

  sed -i \
    -e "s|^mode: .*|mode: $OPERATING_MODE|" \
    -e "s|^  profile: .*|  profile: $cfg_profile|" \
    -e "s|^  ups_name: .*|  ups_name: $UPS_NAME|" \
    -e "s|^  host: .*|  host: $NUT_HOST|" \
    -e "s|^  driver: .*|  driver: $driver_value|" \
    -e "s|^  driver_port: .*|  driver_port: $port_value|" \
    -e "/^  network:/,/^  synology_compatibility:/ s|^    mode: .*|    mode: $NETWORK_MODE|" \
    -e "/^  network:/,/^  synology_compatibility:/ s|^    listen_ipv4: .*|    listen_ipv4: $ipv4_yaml|" \
    -e "/^  network:/,/^  synology_compatibility:/ s|^    listen_ipv6: .*|    listen_ipv6: $ipv6_yaml|" \
    -e "/^  network:/,/^  synology_compatibility:/ s|^    allowed_clients: .*|    allowed_clients: $allowed_yaml|" \
    -e "s|^  grace_period_seconds: .*|  grace_period_seconds: $OUTAGE_GRACE|" \
    -e "s|^  utility_stable_seconds: .*|  utility_stable_seconds: $UTILITY_STABLE|" \
    -e "s|^  battery_charge_min: .*|  battery_charge_min: $RECOVERY_CHARGE|" \
    -e "s|^  network_wait_seconds: .*|  network_wait_seconds: $NETWORK_WAIT|" \
    "$ETC_DIR/config.yaml"

  if ((SYNOLOGY)); then
    sed -i '/synology_compatibility:/,/hosts_sync_seconds:/ s/enabled: false/enabled: true/' "$ETC_DIR/config.yaml"
  fi
  strip_example_inventory "$ETC_DIR/config.yaml"
}

final_report(){
  printf '\nInstallation completed.\n  operating mode: %s\n  profile: %s\n  UPS target: %s@%s\n' "$OPERATING_MODE" "$PROFILE" "$UPS_NAME" "$NUT_HOST"
  if [[ "$PROFILE" == local-server ]]; then
    printf '  UPS driver: %s\n  UPS port: %s\n  NUT network: %s\n' "$UPS_DRIVER" "$UPS_PORT" "$NETWORK_MODE"
  fi
  printf '  recovery gate: charge >= %s%%, utility stable %ss, network wait %ss\n' "$RECOVERY_CHARGE" "$UTILITY_STABLE" "$NETWORK_WAIT"
  printf '  managed hosts: none on fresh install (enroll real devices before arming)\n'
  printf '  config: %s/config.yaml\n  log: %s\n  rollback: %s\n\nBefore arming verify UPS-backed SBC power, automatic boot, network power, and UPS output-cycle behavior.\n' "$ETC_DIR" "$INSTALL_LOG" "$CURRENT_BACKUP"
  if [[ "$MODE" == tui ]] && declare -F tui_msg >/dev/null; then
    tui_msg 'Installation complete' "Installation and probation passed.\n\nMode: $OPERATING_MODE\nProfile: $PROFILE\nConfig: $ETC_DIR/config.yaml\n\nFresh installs contain no managed hosts. Enroll and verify real devices before arming."
  fi
}
