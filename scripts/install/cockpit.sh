#!/usr/bin/env bash
COCKPIT_UI_DIR=/usr/share/cockpit/cockpit-ups-wol
install_cockpit_bundle(){
  local src="${COCKPIT_UPS_WOL_COCKPIT_DIR:-$SELF_DIR/cockpit/dist}"
  if [[ ! -s "$src/manifest.json" || ! -s "$src/index.html" || ! -s "$src/index.js" ]]; then
    if [[ "${COCKPIT_UPS_WOL_REQUIRE_UI:-0}" == 1 ]]; then die "prebuilt Cockpit bundle missing at $src"; fi
    warn "prebuilt Cockpit bundle not present; power engine installed without UI"
    return 0
  fi
  local tmp="${COCKPIT_UI_DIR}.new.$$"; rm -rf "$tmp"; install -d -m0755 "$tmp"; cp -a "$src/." "$tmp/"; find "$tmp" -type d -exec chmod 0755 {} +; find "$tmp" -type f -exec chmod 0644 {} +; rm -rf "$COCKPIT_UI_DIR"; mv "$tmp" "$COCKPIT_UI_DIR"; log "installed Cockpit bundle to $COCKPIT_UI_DIR"
}
