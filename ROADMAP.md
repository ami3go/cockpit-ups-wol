# cockpit-ups-wol roadmap

This roadmap tracks implementation readiness after the architecture and reliability review. It is intentionally conservative: an item is complete only when code or an executable acceptance gate exists.

## v0.1 — safe appliance baseline

### Complete

- Go module and executable foundations for `cockpit-ups-wol-agent`, `cockpit-ups-wolctl`, `cockpit-ups-wol-health`, and `wolctl`.
- Strict versioned YAML configuration parsing and validation.
- Transactional configuration revisions with candidate, validating, known-good, failed and rolled-back states.
- Durable power state generations with checksum validation, `current`/`previous` fallback, fsync and atomic rotation.
- NUT `upsc` status adapter with explicit `UNKNOWN` failure semantics.
- Primary/master ownership validation before `upsmon -c fsd`.
- Deterministic outage/recovery policy state machine and durable commit points.
- Dependency-aware host shutdown/recovery planning.
- Wake-on-LAN packet generation, broadcast/interface selection, durable retry state and bounded retries.
- Health supervisor with durable repair circuit breaker, conservative autofix and `FAILED_SAFE`.
- Local Unix-socket IPC for health status.
- Safe long-running monitor/dry-run/maintenance agent shell with systemd READY/watchdog support.
- Constrained host action/status adapters: fixed-argv SSH shutdown, NUT-managed-host separation, TCP/ping probing and consecutive confirmation.
- Installer framework with distro detection, dependency installation, service autostart, health probation, initial last-known-good bootstrap, application/config/UI rollback and a real Ubuntu/NUT/systemd/Cockpit acceptance test.
- Cockpit TypeScript/React/PatternFly dashboard with Overview, UPS, Devices, Automation, Reliability, Settings and Logs; sanitized plan/status reporting and privileged confirmed configuration rollback.
- Reproducible multi-arch packaging for linux/amd64, linux/arm64 and linux/riscv64 with SHA256SUMS and prebuilt Cockpit assets.
- Full `armed` outage/recovery orchestration: persisted pre-outage snapshot, deterministic pre-FSD shutdown, durable per-host requests before side effects, primary FSD ownership, restart reconciliation, recovery gating, ordered durable WoL restoration and immediate recovery stop on unsafe power.

### In progress / release gates

- Expand simulation and fault-injection coverage for repeated boot interruption, power bounce, NUT role handling, config failure recovery and architecture smoke tests.
- Physical UPS acceptance on supported amd64 and arm64 hardware.
- RISC-V runtime smoke acceptance (cross-build already passes).
- Final Synology DSM acceptance with a real NAS/NUT-secondary configuration.
- Select and add the project license before tagged public release/source reuse.

## v0.2 — administration and integrations

- Additional safe command registry for explicitly allowlisted non-SSH shutdown actions.
- Advanced UPS commands and writable-variable management with privilege separation.
- Multi-UPS policy support.
- Proxmox API adapter and VM/container-aware shutdown planning.
- Additional dependency startup methods after durable dependency-action state is implemented.
- Optional native Go NUT client after protocol/TLS requirements are finalized.
- Maintenance/manual intervention workflows and richer Cockpit configuration editors.
- Optional UPS load/output management after hardware capability classification.

## v0.3 — observability and ecosystem

- Event history and power-cycle timeline.
- Prometheus/metrics export.
- Notifications and external integrations.
- Fleet/remote status aggregation where appropriate for protected homelabs.

## Release principle

A feature is not considered complete merely because the source compiles. Safety-relevant work must have an executable acceptance path covering persistence, restart behavior and fail-closed handling before it is enabled by default.
