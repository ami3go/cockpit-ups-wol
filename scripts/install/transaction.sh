#!/usr/bin/env bash
# Service/file-state aware transaction hooks. Sourced after common/cockpit helpers.
TRACKED_UNITS=(cockpit-ups-wol-agent.service cockpit-ups-wol-health.timer cockpit-ups-wol-firewall.service cockpit-ups-wol-install-recover.service cockpit.socket nut-server.service nut-monitor.service nut-driver-enumerator.service)
INSTALL_PENDING_MARKER="$BACKUP_BASE/install-pending"
BACKUP_KEEP=3

sync_transaction_dir(){
  local d="$1"
  sync -f "$d" 2>/dev/null || sync || true
}

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
  [[ ! -e "$INSTALL_PENDING_MARKER" ]] || die "an interrupted installation is still pending; reboot or start cockpit-ups-wol-install-recover.service before retrying"
  CURRENT_BACKUP="$BACKUP_BASE/$(date -u +'%Y%m%dT%H%M%SZ')-$$"
  install -d -m 0700 "$CURRENT_BACKUP"
  : >"$CURRENT_BACKUP/absent.list"
  record_unit_states
  local p
  for p in "$LIBEXEC_DIR" "$ETC_DIR" "$STATE_DIR/config-history" /usr/local/sbin/cockpit-ups-wolctl /usr/local/sbin/wolctl "$COCKPIT_UI_DIR" "$SYSTEMD_DIR/cockpit-ups-wol-agent.service" "$SYSTEMD_DIR/cockpit-ups-wol-agent.service.d" "$SYSTEMD_DIR/cockpit-ups-wol-health.service" "$SYSTEMD_DIR/cockpit-ups-wol-health.timer" "$SYSTEMD_DIR/cockpit-ups-wol-firewall.service" "$SYSTEMD_DIR/cockpit-ups-wol-install-recover.service" /etc/nut/nut.conf /etc/nut/ups.conf /etc/nut/upsd.conf /etc/nut/upsd.users /etc/nut/upsmon.conf; do
    backup_path_if_exists "$p"
  done
  sync_transaction_dir "$CURRENT_BACKUP"
  log "rollback snapshot: $CURRENT_BACKUP"
}

# Install and enable the minimal boot recovery guard before the pending marker is
# armed. On first installation this intentionally creates only the recovery
# mechanism itself before the transaction starts; all functional project/NUT
# changes happen after install_pending_begin.
install_boot_recovery_guard(){
  install -d -m0755 "$LIBEXEC_DIR"
  install -m0755 "$SELF_DIR/scripts/install/recover-interrupted-install.sh" "$LIBEXEC_DIR/cockpit-ups-wol-install-recover"
  install -m0644 "$SELF_DIR/packaging/systemd/cockpit-ups-wol-install-recover.service" "$SYSTEMD_DIR/cockpit-ups-wol-install-recover.service"
  systemctl daemon-reload
  systemctl enable cockpit-ups-wol-install-recover.service >/dev/null
}

install_pending_begin(){
  [[ -n "$CURRENT_BACKUP" && -d "$CURRENT_BACKUP" ]] || die "cannot arm install transaction without rollback snapshot"
  [[ ! -e "$INSTALL_PENDING_MARKER" ]] || die "install transaction marker already exists"
  install -d -m0700 "$BACKUP_BASE"
  local tmp="$BACKUP_BASE/.install-pending.$$"
  printf '%s\n' "$CURRENT_BACKUP" >"$tmp"
  chmod 0600 "$tmp"
  sync -f "$tmp" 2>/dev/null || sync || true
  mv "$tmp" "$INSTALL_PENDING_MARKER"
  sync_transaction_dir "$BACKUP_BASE"
  log "durable install transaction armed: $INSTALL_PENDING_MARKER"
}

install_pending_clear(){
  [[ -e "$INSTALL_PENDING_MARKER" ]] || return 0
  rm -f "$INSTALL_PENDING_MARKER"
  sync_transaction_dir "$BACKUP_BASE"
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
  [[ -n "$CURRENT_BACKUP" && -d "$CURRENT_BACKUP" ]] || return 1
  warn "rolling back project-managed changes"
  systemctl stop cockpit-ups-wol-health.timer >/dev/null 2>&1 || true
  systemctl stop cockpit-ups-wol-agent.service >/dev/null 2>&1 || true
  systemctl stop cockpit-ups-wol-firewall.service >/dev/null 2>&1 || true
  local p
  for p in "$LIBEXEC_DIR" "$ETC_DIR" "$STATE_DIR/config-history" /usr/local/sbin/cockpit-ups-wolctl /usr/local/sbin/wolctl "$COCKPIT_UI_DIR" "$SYSTEMD_DIR/cockpit-ups-wol-agent.service" "$SYSTEMD_DIR/cockpit-ups-wol-agent.service.d" "$SYSTEMD_DIR/cockpit-ups-wol-health.service" "$SYSTEMD_DIR/cockpit-ups-wol-health.timer" "$SYSTEMD_DIR/cockpit-ups-wol-firewall.service" "$SYSTEMD_DIR/cockpit-ups-wol-install-recover.service" /etc/nut/nut.conf /etc/nut/ups.conf /etc/nut/upsd.conf /etc/nut/upsd.users /etc/nut/upsmon.conf; do
    restore_one "$p"
  done
  systemctl daemon-reload
  restore_unit_states
}

# Override the common fallback now that the durable transaction helpers exist.
# The pending marker is cleared only after a complete synchronous rollback.
install_failure(){
  local l="$1" rc="$2"
  trap - ERR
  warn "installer failed at line $l (rc=$rc)"
  if (( ! INSTALL_COMMITTED )); then
    if rollback_install; then
      install_pending_clear || warn "failed to clear install-pending marker; boot recovery will retry rollback"
    else
      warn "rollback did not complete; install-pending marker retained for boot recovery"
    fi
  fi
  exit "$rc"
}

prune_committed_backups(){
  [[ -d "$BACKUP_BASE" ]] || return 0
  [[ "$BACKUP_KEEP" =~ ^[1-9][0-9]*$ ]] || die "invalid BACKUP_KEEP: $BACKUP_KEEP"

  local -a backups=()
  local entry i
  mapfile -t backups < <(find "$BACKUP_BASE" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort -r)
  for ((i=BACKUP_KEEP; i<${#backups[@]}; i++)); do
    entry="$BACKUP_BASE/${backups[$i]}"
    # The current transaction has already committed, but preserve it
    # defensively even if a non-standard backup name sorts unexpectedly.
    [[ "$entry" == "$CURRENT_BACKUP" ]] && continue
    log "pruning old rollback snapshot: $entry"
    rm -rf -- "$entry" || { warn "failed to prune $entry (installation already committed)"; continue; }
  done
  sync_transaction_dir "$BACKUP_BASE" || warn "failed to fsync rollback snapshot directory after committed housekeeping"
}

backup_commit(){
  # Removing and fsyncing the marker is the durable commit point. If power is
  # lost before this succeeds, boot recovery rolls back to CURRENT_BACKUP.
  install_pending_clear
  INSTALL_COMMITTED=1
  prune_committed_backups
  log "installation committed; rollback snapshot retained at $CURRENT_BACKUP"
}

systemd_reload_enable(){
  systemctl daemon-reload
  systemctl enable cockpit-ups-wol-install-recover.service >/dev/null
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
