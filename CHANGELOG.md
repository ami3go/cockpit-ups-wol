# Changelog

All notable project changes are recorded here while the project is pre-release.

## Unreleased

### Added

- Software architecture for NUT-based outage handling, controller-last shutdown and ordered recovery.
- Synology DSM/NUT-secondary compatibility requirements.
- Recovery policy requiring stable utility, network readiness and UPS recharge (80% by default).
- Reliability specification for automatic service startup, health checks, bounded autofix and `FAILED_SAFE`.
- Interrupted-boot/brownout recovery requirements and durable shutdown/recovery commit points.
- Transactional configuration model with candidate validation, probation, immutable known-good revisions and automatic rollback.
- Canonical v1 YAML configuration schema and example configuration.
- Durable power-state schema with checksummed current/previous generations and per-host action state.
- Unix-domain-socket IPC specification for local Cockpit/CLI control.
- Go agent/CLI foundation plus durable state, NUT/FSD, policy, WoL, health and host-adapter packages.
- Installer framework with real Ubuntu 24.04/NUT/systemd/Cockpit install and broken-upgrade rollback acceptance.
- Read-only-first Cockpit React/PatternFly management UI with sanitized power plan, health/revision/log views and privileged confirmed config rollback.
- Reproducible amd64/arm64/riscv64 appliance packaging with SHA256 checksums and prebuilt Cockpit assets.
- Full armed orchestration controller and runtime wiring: durable per-host shutdown intent, primary FSD ownership, restart reconciliation, recovery gates and ordered persistent WoL recovery.
- Network dependency addresses and fail-closed armed preflight validation.
- Fault tests proving power-bounce recovery stop, ambiguous shutdown reconciliation, outage-grace behavior across reboot and post-reboot AC-stability reset.

### Changed

- Cockpit is explicitly a management surface only; the safety-critical control path remains independent.
- Unknown/failed NUT communication is normalized to `UNKNOWN`, never assumed online or fully charged.
- Controller SBC and required local network infrastructure are normative UPS-backed deployment requirements.
- A controller reboot while an outage was already in progress no longer grants a new outage grace period before threshold evaluation.
- Installer NUT readiness now waits for a valid `ups.status: OL` observation instead of treating any successful `upsc` response as ready.

### Security / safety

- Destructive configuration methods that lack a tested safe adapter (`command`, armed ARP checks, dependency WoL without durable dependency state) fail closed.
- Wake-enabled hosts in armed mode must have verifiable online status, MAC and IPv4 broadcast configuration.
- NUT-managed hosts are not sent duplicate direct shutdown commands.
- External shutdown and wake actions are preceded by durable state writes.

### Release blockers

- Project license must be selected before tagged public release or copying/adapting upstream source.
- Physical UPS acceptance remains required on representative amd64 and arm64 hardware; riscv64 requires runtime smoke acceptance.
