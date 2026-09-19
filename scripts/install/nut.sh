#!/usr/bin/env bash

nut_write_clean_local_server(){
  local pw="$1" syn="$2" nut_dir="${COCKPIT_UPS_WOL_NUT_ETC_DIR:-/etc/nut}"
  install -d -m0750 "$nut_dir"
  printf 'MODE=netserver\n' >"$nut_dir/nut.conf"
  cat >"$nut_dir/ups.conf" <<EOF
[$UPS_NAME]
  driver = usbhid-ups
  port = auto
EOF
  cat >"$nut_dir/upsd.conf" <<'EOF'
LISTEN 0.0.0.0 3493
EOF
  cat >"$nut_dir/upsd.users" <<EOF
[ups-primary]
  password = $pw
  upsmon primary
EOF
  if ((syn)); then
    cat >>"$nut_dir/upsd.users" <<'EOF'

[monuser]
  password = secret
  upsmon secondary
EOF
  fi
  cat >"$nut_dir/upsmon.conf" <<EOF
MONITOR $UPS_NAME@localhost 1 ups-primary $pw primary
MINSUPPLIES 1
SHUTDOWNCMD "/sbin/shutdown -h +0"
POWERDOWNFLAG /etc/killpower
FINALDELAY 15
EOF
  chmod 0640 "$nut_dir/upsd.users" "$nut_dir/upsmon.conf"
  chmod 0644 "$nut_dir/nut.conf" "$nut_dir/ups.conf" "$nut_dir/upsd.conf"
}

nut_configure(){
  local profile="$1" syn="$2" nut_dir="${COCKPIT_UPS_WOL_NUT_ETC_DIR:-/etc/nut}"
  case "$profile" in
    existing-nut)
      log "existing-NUT profile: $nut_dir untouched"
      return
      ;;
    remote-client)
      log "remote-client profile: existing/client NUT files are not rewritten by the installer"
      return
      ;;
  esac
  if [[ -s "$nut_dir/ups.conf" || -s "$nut_dir/upsd.users" || -s "$nut_dir/upsmon.conf" ]]; then
    log "existing NUT config preserved unchanged"
    warn "verify primary upsmon role before arming"
    return
  fi
  local pw
  pw="$(openssl rand -hex 24)"
  printf '%s\n' "$pw" >"$ETC_DIR/secrets/nut-primary-password"
  chmod 0600 "$ETC_DIR/secrets/nut-primary-password"
  nut_write_clean_local_server "$pw" "$syn"
  log "created minimal local NUT server config for UPS '$UPS_NAME'"
  ((syn)) && warn "Synology monuser/secret enabled; keep NUT on a trusted LAN" || true
}
