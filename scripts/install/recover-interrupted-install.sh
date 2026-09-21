#!/usr/bin/env bash
set -Eeuo pipefail

LIBEXEC_DIR=/usr/libexec/cockpit-ups-wol
ETC_DIR=/etc/cockpit-ups-wol
STATE_DIR=/var/lib/cockpit-ups-wol
BACKUP_BASE=/var/backups/cockpit-ups-wol
SYSTEMD_DIR=/etc/systemd/system
COCKPIT_UI_DIR=/usr/share/cockpit/cockpit-ups-wol
MARKER="$BACKUP_BASE/install-pending"
LOG=/var/log/cockpit-ups-wol/install-recovery.log

log(){
  install -d -m0755 "$(dirname "$LOG")"
  printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*" | tee -a "$LOG"
}

die(){ log "ERROR: $*"; exit 1; }

sync_dir(){
  local d="$1"
  if command -v sync >/dev/null 2>&1; then
    sync -f "$d" 2>/dev/null || sync || true
  fi
}

[[ -f "$MARKER" ]] || exit 0
CURRENT_BACKUP="$(head -n1 "$MARKER" || true)"
[[ "$CURRENT_BACKUP" == "$BACKUP_BASE/"* ]] || die "invalid backup path in pending marker"
[[ -d "$CURRENT_BACKUP" ]] || die "pending backup does not exist: $CURRENT_BACKUP"
[[ -f "$CURRENT_BACKUP/absent.list" ]] || die "pending backup is incomplete: absent.list missing"

restore_one(){
  local p="$1" s="$CURRENT_BACKUP/${1#/}"
  if [[ -e "$s" || -L "$s" ]]; then
    rm -rf "$p"
    install -d -m0755 "$(dirname "$p")"
    cp -a "$s" "$p"
  elif grep -Fxq "$p" "$CURRENT_BACKUP/absent.list" 2>/dev/null; then
    rm -rf "$p"
  fi
}

restore_unit_states(){
  [[ -f "$CURRENT_BACKUP/unit-states.tsv" ]] || return 0
  local u enabled active
  while IFS=$'\t' read -r u enabled active; do
    case "$enabled" in
      enabled|enabled-runtime|static|indirect|generated)
        systemctl enable "$u" >/dev/null 2>&1 || true
        ;;
      *)
        systemctl disable "$u" >/dev/null 2>&1 || true
        ;;
    esac
    case "$active" in
      active|activating|reloading)
        systemctl restart "$u" >/dev/null 2>&1 || systemctl start "$u" >/dev/null 2>&1 || true
        ;;
      *)
        systemctl stop "$u" >/dev/null 2>&1 || true
        ;;
    esac
  done <"$CURRENT_BACKUP/unit-states.tsv"
}

log "interrupted installation detected; rolling back from $CURRENT_BACKUP"

# Prevent partially installed automation from producing side effects while the
# snapshot is restored. Failures here are intentionally non-fatal: rollback of
# the files/unit-state snapshot remains authoritative.
systemctl stop cockpit-ups-wol-health.timer >/dev/null 2>&1 || true
systemctl stop cockpit-ups-wol-agent.service >/dev/null 2>&1 || true
systemctl stop cockpit-ups-wol-firewall.service >/dev/null 2>&1 || true

for p in \
  "$LIBEXEC_DIR" \
  "$ETC_DIR" \
  "$STATE_DIR/config-history" \
  /usr/local/sbin/cockpit-ups-wolctl \
  /usr/local/sbin/wolctl \
  "$COCKPIT_UI_DIR" \
  "$SYSTEMD_DIR/cockpit-ups-wol-agent.service" \
  "$SYSTEMD_DIR/cockpit-ups-wol-agent.service.d" \
  "$SYSTEMD_DIR/cockpit-ups-wol-health.service" \
  "$SYSTEMD_DIR/cockpit-ups-wol-health.timer" \
  "$SYSTEMD_DIR/cockpit-ups-wol-firewall.service" \
  "$SYSTEMD_DIR/cockpit-ups-wol-install-recover.service" \
  /etc/nut/nut.conf \
  /etc/nut/ups.conf \
  /etc/nut/upsd.conf \
  /etc/nut/upsd.users \
  /etc/nut/upsmon.conf; do
  restore_one "$p"
done

systemctl daemon-reload
restore_unit_states

# The marker is the commit/rollback authority. Clear it only after every restore
# step above has completed. If power fails during recovery it remains present,
# so this service retries the rollback on the next boot.
rm -f "$MARKER"
sync_dir "$BACKUP_BASE"
log "interrupted installation rollback completed"
