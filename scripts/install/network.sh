#!/usr/bin/env bash

validate_network_inputs() {
  [[ "$NETWORK_MODE" == trusted-lan || "$NETWORK_MODE" == restricted ]] || die "invalid network mode: $NETWORK_MODE"
  (( NUT_LISTEN_IPV4 == 0 || NUT_LISTEN_IPV4 == 1 )) || die "invalid IPv4 listen flag"
  (( NUT_LISTEN_IPV6 == 0 || NUT_LISTEN_IPV6 == 1 )) || die "invalid IPv6 listen flag"
  (( NUT_LISTEN_IPV4 || NUT_LISTEN_IPV6 )) || die "at least one NUT address family must be enabled"
  if [[ "$NETWORK_MODE" == restricted ]]; then
    [[ "$PROFILE" == local-server ]] || die "restricted NUT network mode applies only to local-server profile"
    ((${#NUT_ALLOWED_CLIENTS[@]} > 0)) || die "restricted mode requires at least one --allow-client CIDR"
  fi
  local cidr
  for cidr in "${NUT_ALLOWED_CLIENTS[@]}"; do
    [[ "$cidr" =~ ^[0-9A-Fa-f:.]+/[0-9]{1,3}$ ]] || die "unsafe/invalid client CIDR syntax: $cidr"
    if [[ "$cidr" == *:* ]]; then
      ((NUT_LISTEN_IPV6)) || die "IPv6 allowed client configured while IPv6 NUT listening is disabled: $cidr"
    else
      ((NUT_LISTEN_IPV4)) || die "IPv4 allowed client configured while IPv4 NUT listening is disabled: $cidr"
    fi
  done
}

network_allowed_yaml() {
  if ((${#NUT_ALLOWED_CLIENTS[@]} == 0)); then printf '[]'; return; fi
  local out="[" sep="" cidr
  for cidr in "${NUT_ALLOWED_CLIENTS[@]}"; do out+="$sep$cidr"; sep=", "; done
  printf '%s]' "$out"
}

render_restricted_nft() {
  local out="$1" cidr
  {
    echo 'table inet cockpit_ups_wol {'
    echo '  chain nut_input {'
    echo '    type filter hook input priority -5; policy accept;'
    echo '    ct state established,related accept'
    echo '    iifname "lo" accept'
    for cidr in "${NUT_ALLOWED_CLIENTS[@]}"; do
      if [[ "$cidr" == *:* ]]; then
        printf '    ip6 saddr %s tcp dport 3493 accept\n' "$cidr"
      else
        printf '    ip saddr %s tcp dport 3493 accept\n' "$cidr"
      fi
    done
    echo '    tcp dport 3493 drop'
    echo '  }'
    echo '}'
  } >"$out"
}

install_firewall_apply_helper() {
  cat >"$LIBEXEC_DIR/firewall-apply" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
NFT="$(command -v nft || true)"
[[ -n "$NFT" ]] || { echo 'nft command not found' >&2; exit 1; }
case "${1:-start}" in
  start)
    "$NFT" list table inet cockpit_ups_wol >/dev/null 2>&1 && "$NFT" delete table inet cockpit_ups_wol || true
    "$NFT" -f /etc/cockpit-ups-wol/firewall.nft
    ;;
  stop)
    "$NFT" list table inet cockpit_ups_wol >/dev/null 2>&1 && "$NFT" delete table inet cockpit_ups_wol || true
    ;;
  *) echo 'usage: firewall-apply [start|stop]' >&2; exit 2 ;;
esac
EOF
  chmod 0755 "$LIBEXEC_DIR/firewall-apply"
}

network_configure_security() {
  validate_network_inputs
  [[ "$PROFILE" == local-server ]] || return 0
  if [[ "${PROJECT_CONFIG_CREATED:-0}" -ne 1 ]]; then
    log "existing project config preserved; installer leaves project firewall state unchanged"
    return 0
  fi
  if [[ "$NETWORK_MODE" == trusted-lan ]]; then
    rm -f "$ETC_DIR/firewall.nft" "$LIBEXEC_DIR/firewall-apply"
    log "NUT network mode trusted-lan; no project firewall restriction installed"
    return 0
  fi
  command -v nft >/dev/null 2>&1 || die "restricted mode requires nftables/nft"
  render_restricted_nft "$ETC_DIR/firewall.nft"
  chmod 0600 "$ETC_DIR/firewall.nft"
  # Validate syntax before a systemd unit can activate it. This must not mutate
  # the host ruleset.
  nft -c -f "$ETC_DIR/firewall.nft" || die "generated restricted NUT nftables policy failed validation"
  install_firewall_apply_helper
  log "prepared additive project-owned restricted NUT firewall policy"
}
