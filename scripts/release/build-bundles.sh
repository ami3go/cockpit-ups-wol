#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="${1:-$ROOT/release}"
VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty)}"
VERSION="${VERSION//\//-}"
COMMIT="${COMMIT:-$(git -C "$ROOT" rev-parse HEAD)}"
SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git -C "$ROOT" show -s --format=%ct HEAD)}"
BUILD_DATE="$(date -u -d "@$SOURCE_DATE_EPOCH" +'%Y-%m-%dT%H:%M:%SZ')"
[[ -s "$ROOT/cockpit/dist/manifest.json" && -s "$ROOT/cockpit/dist/index.js" ]] || { echo 'prebuilt Cockpit bundle missing; run cockpit build first' >&2; exit 1; }
rm -rf "$OUT"; mkdir -p "$OUT"
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
bins=(cockpit-ups-wol-agent cockpit-ups-wolctl cockpit-ups-wol-health wolctl)
for arch in amd64 arm64 riscv64; do
  stage="$WORK/cockpit-ups-wol-$VERSION-linux-$arch"
  mkdir -p "$stage/dist/linux-$arch" "$stage/cockpit"
  cp "$ROOT/install.sh" "$ROOT/README.md" "$ROOT/CHANGELOG.md" "$ROOT/THIRD_PARTY_NOTICES.md" "$ROOT/LICENSE" "$stage/"
  cp -a "$ROOT/scripts" "$ROOT/packaging" "$ROOT/config" "$ROOT/schemas" "$ROOT/docs" "$stage/"
  cp -a "$ROOT/cockpit/dist" "$stage/cockpit/"
  for n in "${bins[@]}"; do
    (cd "$ROOT/agent" && CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags="-s -w -X github.com/ami3go/cockpit-ups-wol/agent/internal/version.Version=$VERSION -X github.com/ami3go/cockpit-ups-wol/agent/internal/version.Commit=$COMMIT -X github.com/ami3go/cockpit-ups-wol/agent/internal/version.Date=$BUILD_DATE" -o "$stage/dist/linux-$arch/$n" "./cmd/$n")
  done
  archive="$OUT/cockpit-ups-wol_${VERSION}_linux_${arch}.tar.gz"
  tar --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner -C "$WORK" -cf - "$(basename "$stage")" | gzip -n >"$archive"
done
(
  cd "$OUT"
  sha256sum cockpit-ups-wol_*.tar.gz > SHA256SUMS
)
{
  echo "# cockpit-ups-wol $VERSION"
  echo
  echo "Commit: \`$COMMIT\`"
  echo
  awk 'BEGIN{p=0} /^## Unreleased/{p=1;next} /^## /{if(p)exit} p{print}' "$ROOT/CHANGELOG.md"
} >"$OUT/RELEASE_NOTES.md"
echo "release bundles written to $OUT"
