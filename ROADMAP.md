# cockpit-ups-wol Roadmap

## Current phase

**Phase:** safety core implementation

The P0 design blockers identified by `docs/READINESS_AUDIT.md` have been reconciled. Core safety libraries are being implemented before the project is considered usable.

## v0.1 — Safety core

### Foundation

- [x] repository/build skeleton
- [x] implementation stack decision
- [x] canonical architecture reconciliation
- [x] canonical configuration schema
- [x] canonical persistent state schema
- [x] local IPC contract
- [x] NUT shutdown/FSD ownership model
- [x] power-policy precedence/hysteresis
- [x] controller/network UPS-backed deployment model

### Agent

- [x] Go module and command skeleton
- [x] configuration loader + schema/semantic validation
- [x] atomic state store with checksum + previous-generation fallback
- [x] normalized NUT adapter (`OL`/`OB`/`LB`/`FSD`/`UNKNOWN`)
- [x] canonical state machine library
- [x] shutdown commit/recovery commit persistence coordinator
- [x] host snapshot and reconciliation model
- [ ] SSH/command/NUT host adapters
- [x] ordered pre-FSD shutdown planning
- [x] recovery gating and ordered restore planning
- [x] dry-run / armed / monitor / maintenance policy
- [x] Unix-socket IPC foundation
- [x] local CLI health client
- [ ] full long-running agent runtime loop

### Reliability

- [x] systemd unit for agent
- [x] systemd watchdog support
- [x] health oneshot + timer
- [x] bounded repair/circuit breaker
- [x] config revision manager
- [x] runtime probation model
- [x] automatic last-known-good rollback
- [x] interrupted-config recovery
- [x] `FAILED_SAFE` state and health exposure
- [ ] full runtime wiring of all health/autofix checks

### NUT / Synology

- [ ] distro-aware NUT service discovery beyond installer unit probing
- [x] local-server profile design
- [x] remote-client profile design
- [x] existing-NUT profile design
- [x] Synology compatibility preset/specification
- [x] primary/secondary validation
- [x] UPS output power-cycle capability classification model
- [x] safe FSD integration through primary `upsmon`

### WoL

- [x] Go magic-packet package
- [x] `wolctl`
- [x] interface/broadcast selection
- [x] bounded retry persistence
- [x] status/reconciliation hooks
- [x] dependency-aware ordered recovery

### Installer

- [x] single `install.sh` framework
- [x] default / `--tui` / `--silent`
- [x] Debian/Ubuntu module
- [x] Arch module
- [x] Fedora-family module
- [x] amd64/arm64/riscv64 selection
- [x] dependency installation
- [x] service enable/autostart transaction
- [ ] initial known-good config creation after runtime health probation
- [x] health/probation gate logic
- [x] upgrade snapshot + project/NUT rollback framework
- [x] optional Synology profile
- [x] final validation/deployment report
- [ ] close installer acceptance after full agent runtime stays healthy

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

- [x] Go unit tests for implemented core libraries
- [ ] fake NUT integration harness
- [x] state-store fault injection unit coverage
- [ ] IPC authorization tests
- [x] config rollback tests
- [ ] repeated interrupted-boot integration tests
- [x] power-bounce state-machine tests
- [x] recovery 80% gate tests
- [ ] FSD primary/secondary integration tests
- [ ] Synology acceptance test procedure execution
- [ ] hardware UPS acceptance on amd64
- [ ] hardware UPS acceptance on arm64
- [ ] riscv64 hardware smoke test

### Release

- [x] CI build/test workflow
- [x] multi-arch compile verification
- [ ] release multi-arch binary artifacts
- [ ] Cockpit bundle
- [ ] SHA256SUMS
- [ ] release installer bundle
- [ ] upgrade/rollback end-to-end test
- [x] README
- [ ] LICENSE decision
- [x] SECURITY
- [x] THIRD_PARTY_NOTICES
- [x] CHANGELOG

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
