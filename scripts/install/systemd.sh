#!/usr/bin/env bash

install_systemd_units(){
  install -m0644 "$SELF_DIR/packaging/systemd/cockpit-ups-wol-agent.service" "$SYSTEMD_DIR/"
  install -m0644 "$SELF_DIR/packaging/systemd/cockpit-ups-wol-health.service" "$SYSTEMD_DIR/"
  install -m0644 "$SELF_DIR/packaging/systemd/cockpit-ups-wol-health.timer" "$SYSTEMD_DIR/"
  install -m0644 "$SELF_DIR/packaging/systemd/cockpit-ups-wol-firewall.service" "$SYSTEMD_DIR/"
}
