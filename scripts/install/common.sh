#!/usr/bin/env bash
PROJECT_NAME=cockpit-ups-wol; INSTALL_LOG=/var/log/cockpit-ups-wol/install.log; LIBEXEC_DIR=/usr/libexec/cockpit-ups-wol
ETC_DIR=/etc/cockpit-ups-wol; STATE_DIR=/var/lib/cockpit-ups-wol; BACKUP_BASE=/var/backups/cockpit-ups-wol; SYSTEMD_DIR=/etc/systemd/system
CURRENT_BACKUP=""; INSTALL_COMMITTED=0
log(){ local m="[$(date -u +'%Y-%m-%dT%H:%M:%SZ')] $*"; printf '%s\n' "$m"; [[ -n "${INSTALL_LOG_ACTIVE:-}" ]]&&printf '%s\n' "$m">>"$INSTALL_LOG"||true; }
warn(){ log "WARN: $*"; }; die(){ printf 'ERROR: %s\n' "$*" >&2; exit 1; }; require_root(){ [[ ${EUID:-$(id -u)} -eq 0 ]]||die "installer must run as root"; }
log_init(){ install -d -m 0755 "$(dirname "$INSTALL_LOG")"; touch "$INSTALL_LOG"; chmod 0640 "$INSTALL_LOG"; INSTALL_LOG_ACTIVE=1; log "installer started"; }
platform_detect(){ [[ -r /etc/os-release ]]||die "/etc/os-release not found"; source /etc/os-release; local id="${ID:-}" like="${ID_LIKE:-}"; case " $id $like " in *" debian "*|*" ubuntu "*) DISTRO_FAMILY=debian;; *" arch "*) DISTRO_FAMILY=arch;; *" fedora "*|*" rhel "*|*" centos "*) DISTRO_FAMILY=fedora;; *) die "unsupported distribution: ${ID:-unknown}";; esac; case "$(uname -m)" in x86_64|amd64) TARGET_ARCH=amd64;; aarch64|arm64) TARGET_ARCH=arm64;; riscv64) TARGET_ARCH=riscv64;; *) die "unsupported architecture: $(uname -m)";; esac; }
installer_self_check(){ local f; for f in "$SELF_DIR/config/config.yaml.example" "$SELF_DIR/packaging/systemd/cockpit-ups-wol-agent.service" "$SELF_DIR/packaging/systemd/cockpit-ups-wol-health.service" "$SELF_DIR/packaging/systemd/cockpit-ups-wol-health.timer" "$SELF_DIR/packaging/systemd/cockpit-ups-wol-firewall.service" "$SELF_DIR/packaging/systemd/cockpit-ups-wol-install-recover.service" "$SELF_DIR/scripts/install/recover-interrupted-install.sh" "$SELF_DIR/scripts/install/nut.sh" "$SELF_DIR/scripts/install/discovery.sh" "$SELF_DIR/scripts/install/network.sh" "$SELF_DIR/scripts/install/configuration.sh" "$SELF_DIR/scripts/install/tui.sh" "$SELF_DIR/scripts/install/validate.sh"; do [[ -f "$f" ]]||die "required installer source missing: $f"; done; printf 'installer-check: ok distro=%s arch=%s profile=%s network=%s\n' "$DISTRO_FAMILY" "$TARGET_ARCH" "$PROFILE" "${NETWORK_MODE:-trusted-lan}"; }
# configuration.sh overrides resolve_profile_inputs/install_project_config/final_report.
resolve_profile_inputs(){ [[ "$UPS_NAME" =~ ^[A-Za-z0-9._-]+$ ]]||die "invalid UPS name"; if ((SYNOLOGY))&&[[ "$UPS_NAME" != ups ]]; then die "Synology compatibility requires UPS name 'ups'"; fi; if [[ "$PROFILE" == remote-client && -z "$NUT_HOST" ]]; then if ((SILENT)); then die "remote-client --silent requires --nut-host"; else read -r -p 'Remote NUT server hostname or IP: ' NUT_HOST; fi; fi; [[ -n "$NUT_HOST" ]]||NUT_HOST=localhost; [[ "$NUT_HOST" =~ ^[A-Za-z0-9._:-]+$ ]]||die "invalid NUT host"; }
confirm_install(){ ((SILENT))&&return 0; local text="Install cockpit-ups-wol in ${PROFILE} mode on ${DISTRO_FAMILY}/${TARGET_ARCH}?"; read -r -p "$text [y/N] " r; [[ "$r" =~ ^[Yy]$ ]]||exit 1; }
backup_path_if_exists(){ local p="$1" r="${1#/}"; if [[ -e "$p"||-L "$p" ]]; then install -d -m 0700 "$CURRENT_BACKUP/$(dirname "$r")"; cp -a "$p" "$CURRENT_BACKUP/$r"; else printf '%s\n' "$p">>"$CURRENT_BACKUP/absent.list"; fi; }
restore_one(){ local p="$1" s="$CURRENT_BACKUP/${1#/}"; if [[ -e "$s"||-L "$s" ]]; then rm -rf "$p"; install -d -m 0755 "$(dirname "$p")"; cp -a "$s" "$p"; elif grep -Fxq "$p" "$CURRENT_BACKUP/absent.list" 2>/dev/null; then rm -rf "$p"; fi; }
install_failure(){ local l="$1" rc="$2"; trap - ERR; warn "installer failed at line $l (rc=$rc)"; ((INSTALL_COMMITTED))||rollback_install; exit "$rc"; }
backup_commit(){ INSTALL_COMMITTED=1; log "installation committed; rollback snapshot retained at $CURRENT_BACKUP"; }
install_project_dirs(){ install -d -m 0755 "$LIBEXEC_DIR"; install -d -m 0750 "$ETC_DIR"; install -d -m 0700 "$ETC_DIR/secrets" "$STATE_DIR" "$STATE_DIR/config-history/revisions"; }
find_prebuilt(){ local n="$1" c; if [[ -n "${BINARY_DIR:-}" ]]; then for c in "$BINARY_DIR/$n" "$BINARY_DIR/linux-$TARGET_ARCH/$n"; do [[ -x "$c" ]]&&{ printf '%s\n' "$c"; return 0; }; done; fi; for c in "$SELF_DIR/dist/linux-$TARGET_ARCH/$n" "$SELF_DIR/dist/$TARGET_ARCH/$n"; do [[ -x "$c" ]]&&{ printf '%s\n' "$c"; return 0; }; done; return 1; }
prebuilt_bundle_available(){ local n; for n in cockpit-ups-wol-agent cockpit-ups-wolctl cockpit-ups-wol-health wolctl; do find_prebuilt "$n">/dev/null||return 1; done; }
build_from_source(){ command -v go>/dev/null||return 1; log "using development source-build fallback"; local n; for n in cockpit-ups-wol-agent cockpit-ups-wolctl cockpit-ups-wol-health wolctl; do (cd "$SELF_DIR/agent"&&CGO_ENABLED=0 GOOS=linux GOARCH="$TARGET_ARCH" go build -o "$LIBEXEC_DIR/$n" "./cmd/$n"); done; }
install_project_binaries(){ local n src; if prebuilt_bundle_available; then for n in cockpit-ups-wol-agent cockpit-ups-wolctl cockpit-ups-wol-health wolctl; do src="$(find_prebuilt "$n")"; install -m0755 "$src" "$LIBEXEC_DIR/$n"; done; else build_from_source||die "no complete prebuilt binary bundle and Go fallback unavailable"; fi; ln -sfn "$LIBEXEC_DIR/cockpit-ups-wolctl" /usr/local/sbin/cockpit-ups-wolctl; ln -sfn "$LIBEXEC_DIR/wolctl" /usr/local/sbin/wolctl; }
install_project_config(){ :; }
install_systemd_units(){
  install -m0644 "$SELF_DIR/packaging/systemd/cockpit-ups-wol-agent.service" "$SYSTEMD_DIR/"
  install -m0644 "$SELF_DIR/packaging/systemd/cockpit-ups-wol-health.service" "$SYSTEMD_DIR/"
  install -m0644 "$SELF_DIR/packaging/systemd/cockpit-ups-wol-health.timer" "$SYSTEMD_DIR/"
  install -m0644 "$SELF_DIR/packaging/systemd/cockpit-ups-wol-firewall.service" "$SYSTEMD_DIR/"
  install -m0644 "$SELF_DIR/packaging/systemd/cockpit-ups-wol-install-recover.service" "$SYSTEMD_DIR/"
}
unit_exists(){ systemctl list-unit-files "$1" --no-legend 2>/dev/null|grep -q "^$1"; }; enable_if_exists(){ unit_exists "$1"&&systemctl enable --now "$1"; }
mark_initial_known_good(){ log "recording probation-tested configuration as known-good"; "$LIBEXEC_DIR/cockpit-ups-wolctl" --config "$ETC_DIR/config.yaml" --history-dir "$STATE_DIR/config-history" config-bootstrap >/dev/null; }
final_report(){ :; }
