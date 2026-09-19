#!/usr/bin/env bash

# Parse nut-scanner's NUT-config output into tab-separated records:
# name<TAB>driver<TAB>port<TAB>description
parse_nut_scanner_output() {
  awk '
    function trim(v) { sub(/^[[:space:]]+/, "", v); sub(/[[:space:]]+$/, "", v); return v }
    function value(line, v) {
      v=line; sub(/^[^=]*=[[:space:]]*/, "", v); v=trim(v)
      if (v ~ /^".*"$/) { sub(/^"/, "", v); sub(/"$/, "", v) }
      return v
    }
    function emit(desc) {
      if (name != "" && driver != "") {
        if (port == "") port="auto"
        desc=vendor
        if (product != "") desc=(desc=="" ? product : desc " " product)
        gsub(/\t/, " ", desc)
        print name "\t" driver "\t" port "\t" desc
      }
      name=driver=port=vendor=product=""
    }
    /^\[[^]]+\][[:space:]]*$/ { emit(); name=$0; gsub(/^\[|\][[:space:]]*$/, "", name); next }
    /^[[:space:]]*driver[[:space:]]*=/ { driver=value($0); next }
    /^[[:space:]]*port[[:space:]]*=/ { port=value($0); next }
    /^[[:space:]]*vendor[[:space:]]*=/ { vendor=value($0); next }
    /^[[:space:]]*product[[:space:]]*=/ { product=value($0); next }
    END { emit() }
  '
}

nut_scan_candidates() {
  command -v nut-scanner >/dev/null 2>&1 || return 0
  # -U: USB only. -N: ups.conf-compatible output. -q: suppress scanner chatter.
  # A single -U intentionally avoids volatile bus/device pinning; current NUT
  # documentation recommends stable vendor/product/serial matching instead.
  nut-scanner -U -N -q 2>/dev/null | parse_nut_scanner_output || true
}

validate_ups_driver_port() {
  [[ "$UPS_DRIVER" =~ ^[A-Za-z0-9._-]+$ ]] || die "invalid UPS driver: $UPS_DRIVER"
  [[ "$UPS_PORT" =~ ^[A-Za-z0-9_./:@+-]+$ ]] || die "invalid UPS port: $UPS_PORT"
  [[ "$UPS_PORT" != -* ]] || die "invalid UPS port: $UPS_PORT"
}

prompt_driver_port() {
  local d p
  if [[ "$MODE" == tui ]] && declare -F tui_input >/dev/null; then
    [[ -n "$UPS_DRIVER" ]] || UPS_DRIVER="$(tui_input 'UPS driver' 'Enter the NUT driver (for example usbhid-ups or nutdrv_qx).' '')" || exit 1
    [[ -n "$UPS_PORT" ]] || UPS_PORT="$(tui_input 'UPS port' 'Enter the NUT driver port (for USB devices this is commonly auto).' 'auto')" || exit 1
  else
    if [[ -z "$UPS_DRIVER" ]]; then read -r -p 'NUT UPS driver: ' d; UPS_DRIVER="$d"; fi
    if [[ -z "$UPS_PORT" ]]; then read -r -p 'NUT UPS port [auto]: ' p; UPS_PORT="${p:-auto}"; fi
  fi
  [[ -n "$UPS_DRIVER" && -n "$UPS_PORT" ]] || die "UPS driver and port are required"
  validate_ups_driver_port
}

select_discovered_ups() {
  local -n records_ref=$1
  local choice line idx driver port desc
  if [[ "$MODE" == tui ]] && declare -F tui_menu >/dev/null; then
    local args=()
    for idx in "${!records_ref[@]}"; do
      IFS=$'\t' read -r _ driver port desc <<<"${records_ref[$idx]}"
      args+=("$((idx+1))" "$driver / $port${desc:+ — $desc}")
    done
    choice="$(tui_menu 'UPS selection' 'Multiple NUT-compatible USB devices were found. Select the UPS for this controller.' "${args[@]}")" || exit 1
  else
    printf 'Multiple NUT-compatible USB devices found:\n' >&2
    for idx in "${!records_ref[@]}"; do
      IFS=$'\t' read -r _ driver port desc <<<"${records_ref[$idx]}"
      printf '  %d) %s / %s%s\n' "$((idx+1))" "$driver" "$port" "${desc:+ — $desc}" >&2
    done
    read -r -p 'Select UPS number: ' choice
  fi
  [[ "$choice" =~ ^[0-9]+$ ]] || die "invalid UPS selection"
  (( choice >= 1 && choice <= ${#records_ref[@]} )) || die "UPS selection out of range"
  line="${records_ref[$((choice-1))]}"
  IFS=$'\t' read -r _ UPS_DRIVER UPS_PORT _ <<<"$line"
  validate_ups_driver_port
}

resolve_local_ups_after_packages() {
  [[ "$PROFILE" == local-server ]] || return 0

  if [[ -n "$UPS_DRIVER" && -n "$UPS_PORT" ]]; then
    validate_ups_driver_port
    log "using explicit UPS driver '$UPS_DRIVER' port '$UPS_PORT'"
    return 0
  fi

  local records=() line driver port desc
  mapfile -t records < <(nut_scan_candidates)
  case ${#records[@]} in
    0)
      if ((SILENT)); then
        die "no unambiguous USB UPS discovered; --silent local-server requires --ups-driver and --ups-port"
      fi
      warn "NUT USB discovery found no usable device; explicit driver/port required"
      prompt_driver_port
      ;;
    1)
      line="${records[0]}"
      IFS=$'\t' read -r _ driver port desc <<<"$line"
      if [[ -z "$UPS_DRIVER" ]]; then UPS_DRIVER="$driver"; fi
      if [[ -z "$UPS_PORT" ]]; then UPS_PORT="$port"; fi
      validate_ups_driver_port
      if ((SILENT)); then
        log "selected sole discovered UPS: driver=$UPS_DRIVER port=$UPS_PORT"
      elif [[ "$MODE" == tui ]] && declare -F tui_yesno >/dev/null; then
        tui_yesno 'UPS discovery' "Use discovered UPS?\n\nDriver: $UPS_DRIVER\nPort: $UPS_PORT${desc:+\nDevice: $desc}" || prompt_driver_port
      else
        local r
        read -r -p "Use discovered UPS driver=$UPS_DRIVER port=$UPS_PORT${desc:+ ($desc)}? [Y/n] " r
        [[ ! "$r" =~ ^[Nn]$ ]] || { UPS_DRIVER=""; UPS_PORT=""; prompt_driver_port; }
      fi
      ;;
    *)
      if ((SILENT)); then
        die "multiple USB UPS devices discovered; --silent requires explicit --ups-driver and --ups-port"
      fi
      select_discovered_ups records
      ;;
  esac
}
