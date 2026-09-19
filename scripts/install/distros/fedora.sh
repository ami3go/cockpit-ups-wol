#!/usr/bin/env bash
install_packages(){ dnf install -y cockpit nut nut-client dialog jq curl ca-certificates openssl; if ! command -v go>/dev/null&&[[ ! -x "${BINARY_DIR:-}/cockpit-ups-wol-agent" ]]; then dnf install -y golang; fi; }
