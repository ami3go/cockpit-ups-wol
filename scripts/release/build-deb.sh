#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="${1:-$ROOT/release}"
VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty)}"
VERSION="${VERSION//\//-}"

sanitize_debian_version() {
  local value="${1#v}"
  value="${value//-/.}"
  value="$(printf '%s' "$value" | sed -E 's/[^0-9A-Za-z.+~:]/./g')"
  if [[ ! "$value" =~ ^[0-9] ]]; then
    value="0~$value"
  fi
  printf '%s\n' "$value"
}

DEB_VERSION="$(sanitize_debian_version "$VERSION")"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

for arch in amd64 arm64 riscv64; do
  archive="$OUT/cockpit-ups-wol_${VERSION}_linux_${arch}.tar.gz"
  [[ -s "$archive" ]] || {
    echo "missing appliance archive: $archive" >&2
    echo "run scripts/release/build-bundles.sh first with the same VERSION" >&2
    exit 1
  }

  extract="$WORK/extract-$arch"
  package_root="$WORK/package-$arch"
  app_root="$package_root/usr/lib/cockpit-ups-wol"
  mkdir -p "$extract" "$app_root" "$package_root/DEBIAN" \
    "$package_root/usr/sbin" "$package_root/usr/share/doc/cockpit-ups-wol"

  tar -xzf "$archive" -C "$extract"
  source_root="$(find "$extract" -mindepth 1 -maxdepth 1 -type d -print -quit)"
  [[ -n "$source_root" ]] || { echo "invalid appliance archive: $archive" >&2; exit 1; }
  cp -a "$source_root"/. "$app_root"/

  cat >"$package_root/DEBIAN/control" <<CONTROL
Package: cockpit-ups-wol
Version: $DEB_VERSION
Section: admin
Priority: optional
Architecture: $arch
Maintainer: cockpit-ups-wol contributors <noreply@github.com>
Homepage: https://github.com/ami3go/cockpit-ups-wol
Depends: cockpit, nut-client, nut-server, dialog, jq, curl, ca-certificates, openssl, openssh-client
Recommends: nftables
Description: Cockpit-managed UPS shutdown and recovery controller
 cockpit-ups-wol combines Network UPS Tools, a persistent safety agent,
 Wake-on-LAN recovery and Cockpit management for homelab and small-network
 UPS orchestration. Package installation stages the tested application and
 dependencies; UPS enrollment and arming remain explicit administrator actions.
CONTROL

  cat >"$package_root/usr/sbin/cockpit-ups-wol-setup" <<'WRAPPER'
#!/bin/sh
set -eu
root=/usr/lib/cockpit-ups-wol
arch="$(dpkg --print-architecture)"
case "$arch" in
  amd64|arm64|riscv64) ;;
  *) echo "cockpit-ups-wol: unsupported Debian architecture: $arch" >&2; exit 1 ;;
esac
exec "$root/install.sh" --binary-dir "$root/dist/linux-$arch" "$@"
WRAPPER
  chmod 0755 "$package_root/usr/sbin/cockpit-ups-wol-setup"

  cp "$ROOT/LICENSE" "$package_root/usr/share/doc/cockpit-ups-wol/copyright"
  gzip -n -9 <"$ROOT/CHANGELOG.md" >"$package_root/usr/share/doc/cockpit-ups-wol/changelog.gz"

  deb="$OUT/cockpit-ups-wol_${DEB_VERSION}_${arch}.deb"
  dpkg-deb --build --root-owner-group "$package_root" "$deb"
done

(
  cd "$OUT"
  shopt -s nullglob
  artifacts=(cockpit-ups-wol_*.tar.gz cockpit-ups-wol_*.deb)
  ((${#artifacts[@]} > 0)) || { echo 'no release artifacts produced' >&2; exit 1; }
  sha256sum "${artifacts[@]}" >SHA256SUMS
)

echo "Debian packages written to $OUT"
