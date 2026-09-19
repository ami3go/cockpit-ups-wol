#!/usr/bin/env bash

resolve_profile_inputs(){
  [[ "$UPS_NAME" =~ ^[A-Za-z0-9._-]+$ ]] || die "invalid UPS name"
  if ((SYNOLOGY)) && [[ "$UPS_NAME" != ups ]]; then
    die "Synology compatibility requires UPS name 'ups'"
  fi
  if [[ "$PROFILE" == remote-client && -z "$NUT_HOST" ]]; then
    if ((SILENT)); then
      die "remote-client --silent requires --nut-host"
    elif [[ "$MODE" == tui ]] && declare -F tui_input >/dev/null; then
      NUT_HOST="$(tui_input 'Remote NUT server' 'Remote NUT server hostname or IP.' '')" || exit 1
    else
      read -r -p 'Remote NUT server hostname or IP: ' NUT_HOST
    fi
  fi
  [[ -n "$NUT_HOST" ]] || NUT_HOST=localhost
  [[ "$NUT_HOST" =~ ^[A-Za-z0-9._:-]+$ ]] || die "invalid NUT host"
  if [[ -n "$UPS_DRIVER" || -n "$UPS_PORT" ]]; then
    [[ "$PROFILE" == local-server ]] || die "--ups-driver/--ups-port apply only to local-server profile"
    if ((SILENT)) && { [[ -z "$UPS_DRIVER" ]] || [[ -z "$UPS_PORT" ]]; }; then
      die "--silent requires both --ups-driver and --ups-port when either is supplied"
    fi
  fi
}

install_project_config(){
  if [[ -f "$ETC_DIR/config.yaml" ]]; then
    log "existing project config preserved"
    return 0
  fi

  install -m0600 "$SELF_DIR/config/config.yaml.example" "$ETC_DIR/config.yaml"
  local cfg_profile="$PROFILE" driver_value port_value
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

  sed -i \
    -e "s|^  profile: .*|  profile: $cfg_profile|" \
    -e "s|^  ups_name: .*|  ups_name: $UPS_NAME|" \
    -e "s|^  host: .*|  host: $NUT_HOST|" \
    -e "s|^  driver: .*|  driver: $driver_value|" \
    -e "s|^  driver_port: .*|  driver_port: $port_value|" \
    "$ETC_DIR/config.yaml"

  if ((SYNOLOGY)); then
    sed -i '/synology_compatibility:/,/hosts_sync_seconds:/ s/enabled: false/enabled: true/' "$ETC_DIR/config.yaml"
  fi
}

final_report(){
  printf '\nInstallation completed in dry-run mode.\n  profile: %s\n  UPS target: %s@%s\n' "$PROFILE" "$UPS_NAME" "$NUT_HOST"
  if [[ "$PROFILE" == local-server ]]; then
    printf '  UPS driver: %s\n  UPS port: %s\n' "$UPS_DRIVER" "$UPS_PORT"
  fi
  printf '  config: %s/config.yaml\n  log: %s\n  rollback: %s\n\nBefore arming verify UPS-backed SBC power, automatic boot, network power, and UPS output-cycle behavior.\n' "$ETC_DIR" "$INSTALL_LOG" "$CURRENT_BACKUP"
}
