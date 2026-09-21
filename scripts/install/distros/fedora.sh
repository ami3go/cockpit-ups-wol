#!/usr/bin/env bash
install_packages(){
  local pkgs=(cockpit nut nut-client dialog jq curl ca-certificates openssl openssh-clients)
  [[ "$NETWORK_MODE" == restricted ]] && pkgs+=(nftables)
  dnf install -y "${pkgs[@]}"
  if ! command -v go>/dev/null && [[ ! -x "${BINARY_DIR:-}/cockpit-ups-wol-agent" ]]; then dnf install -y golang; fi
}
