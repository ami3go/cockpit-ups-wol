# cockpit-ups-wol Roadmap

## Current phase

**Phase:** requirements reconciled → implementation foundation

The P0 design blockers identified by `docs/READINESS_AUDIT.md` are being resolved before core code is considered stable.

## v0.1 — Safety core

### Foundation

- [ ] repository/build skeleton
- [x] implementation stack decision
- [x] canonical architecture reconciliation
- [x] canonical configuration schema
- [x] canonical persistent state schema
- [x] local IPC contract
- [x] NUT shutdown/FSD ownership model
- [x] power-policy precedence/hysteresis
- [x] controller/network UPS-backed deployment model

### Agent

- [ ] Go module and command skeleton
- [ ] configuration loader + schema/semantic validation
- [ ] atomic state store with checksum + previous-generation fallback
- [ ] normalized NUT adapter (`OL`/`OB`/`LB`/`FSD`/`UNKNOWN`)
- [ ] canonical state machine
- [ ] shutdown commit/recovery commit persistence
- [ ] host snapshot and reconciliation
- [ ] SSH/command/NUT host adapters
- [ ] ordered pre-FSD shutdown planning
- [ ] recovery gating and ordered restore
- [ ] dry-run / armed / monitor / maintenance modes
- [ ] Unix-socket IPC server
- [ ] local CLI client

### Reliability

- [ ] systemd unit for agent
- [ ] systemd watchdog integration
- [ ] health oneshot + timer
- [ ] bounded repair/circuit breaker
- [ ] config revision manager
- [ ] runtime probation
- [ ] automatic last-known-good rollback
- [ ] interrupted-config recovery
- [ ] `FAILED_SAFE` inhibition and acknowledgement path

### NUT / Synology

- [ ] distro-aware NUT service discovery
- [ ] local-server profile
- [ ] remote-client profile
- [ ] existing-NUT profile
- [ ] Synology compatibility preset
- [ ] primary/secondary validation
- [ ] UPS output power-cycle capability classification
- [ ] safe FSD integration through primary `upsmon`

### WoL

- [ ] Go magic-packet package
- [ ] `wolctl`
- [ ] interface/broadcast selection
- [ ] bounded retry
- [ ] status verification
- [ ] dependency-aware ordered recovery

### Installer

- [ ] single `install.sh`
- [ ] default / `--tui` / `--silent`
- [ ] Debian/Ubuntu module
- [ ] Arch module
- [ ] Fedora-family module
- [ ] amd64/arm64/riscv64 release selection
- [ ] dependency installation
- [ ] service enable/autostart
- [ ] initial known-good config creation
- [ ] health/probation gate
- [ ] upgrade + binary/config rollback
- [ ] Synology profile
- [ ] final validation report

### Cockpit

- [ ] starter-kit based frontend
- [ ] Overview
- [ ] UPS
- [ ] Devices
- [ ] Automation
- [ ] Reliability
- [ ] Settings
- [ ] Logs
- [ ] dry-run power plan preview
- [ ] config revision/rollback UI
- [ ] privileged actions through local CLI/superuser path

### Tests

- [ ] unit tests
- [ ] fake NUT integration harness
- [ ] state-store fault injection
- [ ] IPC authorization tests
- [ ] config rollback tests
- [ ] repeated interrupted-boot tests
- [ ] power-bounce tests
- [ ] recovery 80% gate tests
- [ ] FSD primary/secondary integration tests
- [ ] Synology acceptance test procedure
- [ ] hardware UPS acceptance on amd64
- [ ] hardware UPS acceptance on arm64
- [ ] riscv64 smoke test

### Release

- [ ] CI build/test workflow
- [ ] multi-arch binaries
- [ ] Cockpit bundle
- [ ] SHA256SUMS
- [ ] release installer
- [ ] upgrade/rollback test
- [ ] README
- [ ] LICENSE decision
- [ ] SECURITY
- [ ] THIRD_PARTY_NOTICES before copied source
- [ ] CHANGELOG

## v0.2 — Administration and adapters

- Proxmox API adapter
- richer NUT administrative commands with explicit authorization
- multiple UPSes
- host CRUD improvements
- maintenance workflows
- notification integrations
- optional load-aware policy

## v0.3 — Observability and advanced deployment

- historical graphs/metrics
- Prometheus integration
- notifications/reporting
- native Go NUT client evaluation
- advanced routed/VLAN WoL helpers
- multi-controller/HA research

## Definition of v0.1 usable

v0.1 is usable only when a clean supported system can install the stack and successfully complete this hardware/simulation lifecycle:

```text
install → dry-run → arm
utility loss
ordered shutdown
NUT secondary shutdown
controller last
interrupted boot/power bounce
valid OL stability
UPS >= 80%
ordered restore
config failure rollback
health/autofix validation
```

No browser may be required for the automatic safety lifecycle.
