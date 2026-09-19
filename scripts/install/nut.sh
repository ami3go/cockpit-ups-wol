#!/usr/bin/env bash
nut_write_clean_local_server(){ local pw="$1" syn="$2"; install -d -m0750 /etc/nut; printf 'MODE=netserver\n'>/etc/nut/nut.conf; cat >/etc/nut/ups.conf <<EOF
[$UPS_NAME]
  driver = usbhid-ups
  port = auto
EOF
cat >/etc/nut/upsd.conf <<'EOF'
LISTEN 0.0.0.0 3493
EOF
cat >/etc/nut/upsd.users <<EOF
[ups-primary]
  password = $pw
  upsmon primary
EOF
if ((syn)); then cat >>/etc/nut/upsd.users <<'EOF'

[monuser]
  password = secret
  upsmon secondary
EOF
fi
cat >/etc/nut/upsmon.conf <<EOF
MONITOR $UPS_NAME@localhost 1 ups-primary $pw primary
MINSUPPLIES 1
SHUTDOWNCMD "/sbin/shutdown -h +0"
POWERDOWNFLAG /etc/killpower
FINALDELAY 15
EOF
chmod 0640 /etc/nut/upsd.users /etc/nut/upsmon.conf; chmod 0644 /etc/nut/nut.conf /etc/nut/ups.conf /etc/nut/upsd.conf; }
nut_configure(){ local profile="$1" syn="$2"; case "$profile" in existing-nut) log "existing-NUT profile: /etc/nut untouched"; return;; remote-client) log "remote-client profile: existing/client NUT files are not rewritten by the installer"; return;; esac; if [[ -s /etc/nut/ups.conf||-s /etc/nut/upsd.users||-s /etc/nut/upsmon.conf ]]; then log "existing NUT config preserved unchanged"; warn "verify primary upsmon role before arming"; return; fi; local pw; pw="$(openssl rand -hex 24)"; printf '%s\n' "$pw">"$ETC_DIR/secrets/nut-primary-password"; chmod 0600 "$ETC_DIR/secrets/nut-primary-password"; nut_write_clean_local_server "$pw" "$syn"; log "created minimal local NUT server config for UPS '$UPS_NAME'"; ((syn))&&warn "Synology monuser/secret enabled; keep NUT on a trusted LAN"||true; }
