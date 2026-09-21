#!/usr/bin/env bash
install_packages(){
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  local pkgs=(cockpit nut-client nut-server dialog jq curl ca-certificates openssl openssh-client)
  [[ "$NETWORK_MODE" == restricted ]] && pkgs+=(nftables)
  apt-get install -y --no-install-recommends "${pkgs[@]}"
  if ! command -v go>/dev/null && [[ ! -x "${BINARY_DIR:-}/cockpit-ups-wol-agent" ]]; then apt-get install -y --no-install-recommends golang-go; fi
}
