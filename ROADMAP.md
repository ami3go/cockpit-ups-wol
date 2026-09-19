# cockpit-ups-wol Roadmap

## Current phase

**Phase:** safety-core implementation

The P0 architecture/readiness blockers are resolved. Core safety libraries are now being implemented and continuously validated by CI.

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
- [x] configuration loader + strict YAML/semantic validation
- [x] atomic state store with checksum + previous-generation fallback
- [x] normalized NUT adapter (`OL`/`OB`/`LB`/`FSD`/`UNKNOWN`)
- [x] canonical state machine library
- [x] shutdown commit/recovery commit persistence coordinator
- [ ] host snapshot and full interrupted-action reconciliation
- [ ] SSH/command/NUT host shutdown adapters
- [x] ordered pre-FSD shutdown planning
- [x] recovery gating and dependency-aware ordered restore
- [ ] runtime enforcement of dry-run / armed / monitor / maintenance modes
- [x] Unix-socket IPC server foundation
- [x] local CLI client foundation
- [ ] integrated long-running agent event loop

### Reliability

- [x] systemd unit for agent
- [x] systemd watchdog support
- [x] health oneshot + timer
- [x] bounded repair/circuit breaker with durable counters
- [x] config revision manager
- [x] runtime probation
- [x] automatic last-known-good rollback
- [x] interrupted-config recovery
- [x] health details over local IPC
- [ ] `FAILED_SAFE` acknowledgement/reconciliation command path

### NUT / Synology

- [ ] distro-aware NUT service discovery
- [ ] fully integrated local-server runtime profile
- [ ] fully integrated remote-client runtime profile
- [ ] existing-NUT profile installer integration
- [ ] Synology compatibility installer preset
- [x] primary/master validation from `upsmon.conf`
- [x] UPS output power-cycle capability represented in config
- [x] safe FSD request through primary `upsmon`

### WoL

- [x] Go magic-packet package
- [x] `wolctl`
- [x] interface/broadcast selection
- [x] bounded retry state
- [ ] concrete host status verification adapters (TCP/ping/ARP)
- [x] dependency-aware ordered recovery

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

- [x] Go unit-test suite running in CI
- [x] fake-command NUT parser/FSD tests
- [x] state corruption/current→previous fallback tests
- [x] config rollback/probation/interrupted-validation tests
- [x] recovery 80% gate and post-commit charge hysteresis tests
- [x] basic power-bounce/recovery-interruption tests
- [x] Unix-socket GetHealth integration test
- [ ] IPC peer-credential/authorization tests
- [ ] torn-write fault injection at every state-store rename/fsync point
- [ ] repeated interrupted-boot integration tests
- [ ] real NUT primary/secondary FSD integration tests
- [x] Synology acceptance test procedure documented
- [ ] hardware UPS acceptance on amd64
- [ ] hardware UPS acceptance on arm64
- [ ] riscv64 hardware/smoke acceptance

### Release

- [x] CI test/vet/multi-arch build workflow
- [ ] publish multi-arch binary artifacts
- [ ] Cockpit bundle
- [ ] SHA256SUMS release artifact
- [ ] release installer
- [ ] upgrade/rollback release test
- [x] README
- [ ] LICENSE decision
- [x] SECURITY
- [x] THIRD_PARTY_NOTICES before copied source
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
