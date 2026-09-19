#!/usr/bin/env bash
install_packages(){ pacman -Sy --needed --noconfirm cockpit nut dialog jq curl ca-certificates openssl; if ! command -v go>/dev/null&&[[ ! -x "${BINARY_DIR:-}/cockpit-ups-wol-agent" ]]; then pacman -S --needed --noconfirm go; fi; }
