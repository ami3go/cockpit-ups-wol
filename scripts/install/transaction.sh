#!/usr/bin/env bash
# Service/file-state aware transaction hooks. Sourced after common/cockpit helpers.
TRACKED_UNITS=(cockpit-ups-wol-agent.service cockpit-ups-wol-health.timer cockpit-ups-wol-firewall.service cockpit.socket nut-server.service nut-monitor.service nut-driver-enumerator.service)
record_unit_states(){
  : >"$CURRENT_BACKUP/unit-states.tsv"
  local u enabled active
  for u in "${TRACKED_UNITS[@]}"; do
    enabled="$(systemctl is-enabled "$u" 2>/dev/null || true)"; [[ -n "$enabled" ]] || enabled=not-found
    active="$(systemctl is-active "$u" 2>/dev/null || true)"; [[ -n "$active" ]] || active=inactive
    printf '%s\t%s\t%s\n' "$u" "$enabled" "$active" >>"$CURRENT_BACKUP/unit-states.tsv"
  done
}
backup_begin(){
  CURRENT_BACKUP="$BACKUP_BASE/$(date -u +'%Y%m%dT%H%M%SZ')-$$"
  install -d -m 0700 "$CURRENT_BACKUP"
  : >"$CURRENT_BACKUP/absent.list"
  record_unit_states
  local p
  for p in "$LIBEXEC_DIR" "$ETC_DIR" "$STATE_DIR/config-history" /usr/local/sbin/cockpit-ups-wolctl /usr/local/sbin/wolctl "$COCKPIT_UI_DIR" "$SYSTEMD_DIR/cockpit-ups-wol-agent.service" "$SYSTEMD_DIR/cockpit-ups-wol-health.service" "$SYSTEMD_DIR/cockpit-ups-wol-health.timer" "$SYSTEMD_DIR/cockpit-ups-wol-firewall.service" /etc/nut/nut.conf /etc/nut/ups.conf /etc/nut/upsd.conf /etc/nut/upsd.users /etc/nut/upsmon.conf; do
    backup_path_if_exists "$p"
  done
  log "rollback snapshot: $CURRENT_BACKUP"
}
restore_unit_states(){
  [[ -f "$CURRENT_BACKUP/unit-states.tsv" ]] || return 0
  local u enabled active
  while IFS=$'\t' read -r u enabled active; do
    case "$enabled" in enabled|enabled-runtime|static|indirect|generated) systemctl enable "$u" >/dev/null 2>&1 || true ;; *) systemctl disable "$u" >/dev/null 2>&1 || true ;; esac
    case "$active" in active|activating|reloading) systemctl restart "$u" >/dev/null 2>&1 || systemctl start "$u" >/dev/null 2>&1 || true ;; *) systemctl stop "$u" >/dev/null 2>&1 || true ;; esac
  done <"$CURRENT_BACKUP/unit-states.tsv"
}
rollback_install(){
  [[ -n "$CURRENT_BACKUP" && -d "$CURRENT_BACKUP" ]] || return 0
  warn "rolling back project-managed changes"
  systemctl stop cockpit-ups-wol-firewall.service >/dev/null 2>&1 || true
  local p
  for p in "$LIBEXEC_DIR" "$ETC_DIR" "$STATE_DIR/config-history" /usr/local/sbin/cockpit-ups-wolctl /usr/local/sbin/wolctl "$COCKPIT_UI_DIR" "$SYSTEMD_DIR/cockpit-ups-wol-agent.service" "$SYSTEMD_DIR/cockpit-ups-wol-health.service" "$SYSTEMD_DIR/cockpit-ups-wol-health.timer" "$SYSTEMD_DIR/cockpit-ups-wol-firewall.service" /etc/nut/nut.conf /etc/nut/ups.conf /etc/nut/upsd.conf /etc/nut/upsd.users /etc/nut/upsmon.conf; do
    restore_one "$p"
  done
  systemctl daemon-reload >/dev/null 2>&1 || true
  restore_unit_states
}
systemd_reload_enable(){
  systemctl daemon-reload
  enable_if_exists cockpit.socket || warn "cockpit.socket not found"

  case "${NETWORK_FIREWALL_ACTION:-unchanged}" in
    enable) systemctl enable --now cockpit-ups-wol-firewall.service ;;
    disable) systemctl disable --now cockpit-ups-wol-firewall.service >/dev/null 2>&1 || true ;;
    unchanged) : ;;
    *) die "invalid firewall transaction action: $NETWORK_FIREWALL_ACTION" ;;
  esac

  case "$PROFILE" in
    local-server)
      enable_if_exists nut-driver-enumerator.service || true
      enable_if_exists nut-server.service || true
      enable_if_exists nut-monitor.service || true
      ;;
    remote-client)
      enable_if_exists nut-monitor.service || true
      ;;
  esac

  systemctl enable cockpit-ups-wol-agent.service
  systemctl restart cockpit-ups-wol-agent.service
  systemctl enable --now cockpit-ups-wol-health.timer
}
