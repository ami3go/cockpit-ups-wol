#!/usr/bin/env bash
set -Eeuo pipefail

IMAGE="${1:?usage: systemd-container-install.sh IMAGE PACKAGE.deb}"
DEB="${2:?usage: systemd-container-install.sh IMAGE PACKAGE.deb}"
[[ -f "$DEB" ]] || { echo "missing Debian package: $DEB" >&2; exit 1; }
command -v docker >/dev/null || { echo 'docker is required' >&2; exit 1; }

SAFE_IMAGE="$(printf '%s' "$IMAGE" | tr '/:.' '---')"
NAME="cockpit-ups-wol-${SAFE_IMAGE}-${RANDOM}-${RANDOM}"
TAG="cockpit-ups-wol-ci-${SAFE_IMAGE}:${RANDOM}"
BUILD_DIR="$(mktemp -d)"
STARTED=0

cleanup() {
  rc=$?
  set +e
  if ((STARTED)) && [[ "$rc" -ne 0 ]]; then
    echo '--- container diagnostics: systemd ---' >&2
    docker exec "$NAME" systemctl --no-pager --full --failed >&2 || true
    echo '--- container diagnostics: cockpit-ups-wol services ---' >&2
    docker exec "$NAME" systemctl --no-pager --full status \
      cockpit-ups-wol-agent.service cockpit-ups-wol-health.timer \
      cockpit.socket nut-server.service nut-driver.target >&2 || true
    echo '--- container diagnostics: recent journal ---' >&2
    docker exec "$NAME" journalctl --no-pager -n 250 >&2 || true
    echo '--- container diagnostics: installer log ---' >&2
    docker exec "$NAME" cat /var/log/cockpit-ups-wol/install.log >&2 || true
  fi
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  docker rmi -f "$TAG" >/dev/null 2>&1 || true
  rm -rf "$BUILD_DIR"
  trap - EXIT
  exit "$rc"
}
trap cleanup EXIT

cat >"$BUILD_DIR/Dockerfile" <<EOF_DOCKER
FROM $IMAGE
ARG DEBIAN_FRONTEND=noninteractive
ENV container=docker
RUN apt-get update \
 && apt-get install -y --no-install-recommends systemd systemd-sysv dbus ca-certificates \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
EOF_DOCKER

docker build --pull -t "$TAG" "$BUILD_DIR"
docker run -d \
  --name "$NAME" \
  --privileged \
  --cgroupns=host \
  --tmpfs /run \
  --tmpfs /run/lock \
  -v /sys/fs/cgroup:/sys/fs/cgroup:rw \
  "$TAG" >/dev/null
STARTED=1

SYSTEMD_READY=0
for _ in {1..60}; do
  state="$(docker exec "$NAME" systemctl is-system-running 2>/dev/null || true)"
  case "$state" in
    running|degraded)
      SYSTEMD_READY=1
      break
      ;;
  esac
  sleep 1
done
if (( ! SYSTEMD_READY )); then
  echo "systemd did not become ready in $IMAGE" >&2
  exit 1
fi

echo "systemd ready in $IMAGE"
docker cp "$DEB" "$NAME:/tmp/cockpit-ups-wol.deb"
docker exec "$NAME" bash -Eeuo pipefail -c '
  export DEBIAN_FRONTEND=noninteractive
  test -s /tmp/cockpit-ups-wol.deb
  dpkg-deb --info /tmp/cockpit-ups-wol.deb >/dev/null
  apt-get update
  cd /tmp
  apt-get install -y ./cockpit-ups-wol.deb
  cockpit-ups-wol-setup --check --profile existing
'

docker exec "$NAME" bash -Eeuo pipefail -c '
  systemctl stop nut-server.service nut-monitor.service nut-driver.target >/dev/null 2>&1 || true
  install -d -m0750 -o root -g nut /etc/nut
  install -d -m0770 -o nut -g nut /run/nut
  cat >/etc/nut/online.dev <<"EOF_NUT_STATE"
ups.status: OL
battery.charge: 100
battery.runtime: 3600
ups.model: cockpit-ups-wol distro matrix
EOF_NUT_STATE
  cat >/etc/nut/nut.conf <<"EOF_NUT_CONF"
MODE=netserver
EOF_NUT_CONF
  cat >/etc/nut/ups.conf <<"EOF_UPS_CONF"
[ups]
  driver = dummy-ups
  port = /etc/nut/online.dev
  mode = dummy-once
EOF_UPS_CONF
  cat >/etc/nut/upsd.conf <<"EOF_UPSD_CONF"
LISTEN 127.0.0.1 3493
EOF_UPSD_CONF
  chown root:nut /etc/nut/nut.conf /etc/nut/ups.conf /etc/nut/upsd.conf
  chmod 0640 /etc/nut/nut.conf /etc/nut/ups.conf /etc/nut/upsd.conf
  systemctl restart nut-driver-enumerator.service
  systemctl restart nut-server.service
  ready=0
  for _ in {1..30}; do
    if out="$(upsc ups@localhost 2>/dev/null)" && grep -q "^ups.status: OL$" <<<"$out"; then
      ready=1
      break
    fi
    sleep 1
  done
  ((ready)) || { echo "NUT dummy UPS did not reach OL" >&2; exit 1; }
'

docker exec "$NAME" env \
  COCKPIT_UPS_WOL_TEST_MODE=1 \
  COCKPIT_UPS_WOL_REQUIRE_UI=1 \
  COCKPIT_UPS_WOL_PROBATION_SECONDS=3 \
  cockpit-ups-wol-setup --silent --profile existing-nut

docker exec "$NAME" bash -Eeuo pipefail -c '
  systemctl is-enabled --quiet cockpit-ups-wol-agent.service
  systemctl is-active --quiet cockpit-ups-wol-agent.service
  systemctl is-enabled --quiet cockpit-ups-wol-health.timer
  systemctl is-active --quiet cockpit-ups-wol-health.timer
  test -s /usr/share/cockpit/cockpit-ups-wol/manifest.json
  test -s /usr/share/cockpit/cockpit-ups-wol/index.js
  grep -Eq "^[[:space:]]*mode:[[:space:]]*dry-run([[:space:]]*#.*)?$" /etc/cockpit-ups-wol/config.yaml
  /usr/libexec/cockpit-ups-wol/cockpit-ups-wolctl --socket /run/cockpit-ups-wol/agent.sock health >/dev/null
'

# Re-running setup with the same known-good configuration must be idempotent.
docker exec "$NAME" env \
  COCKPIT_UPS_WOL_TEST_MODE=1 \
  COCKPIT_UPS_WOL_REQUIRE_UI=1 \
  COCKPIT_UPS_WOL_PROBATION_SECONDS=2 \
  cockpit-ups-wol-setup --silent --profile existing-nut

docker exec "$NAME" systemctl is-active --quiet cockpit-ups-wol-agent.service
docker exec "$NAME" /usr/libexec/cockpit-ups-wol/cockpit-ups-wolctl \
  --socket /run/cockpit-ups-wol/agent.sock health >/dev/null

echo "systemd-container-install: PASS image=$IMAGE"
