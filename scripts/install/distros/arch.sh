#!/usr/bin/env bash
install_packages(){
  local pkgs=(cockpit nut dialog jq curl ca-certificates openssl openssh)
  [[ "$NETWORK_MODE" == restricted ]] && pkgs+=(nftables)
  pacman -Sy --needed --noconfirm "${pkgs[@]}"
  if ! command -v go>/dev/null && [[ ! -x "${BINARY_DIR:-}/cockpit-ups-wol-agent" ]]; then pacman -S --needed --noconfirm go; fi
}
