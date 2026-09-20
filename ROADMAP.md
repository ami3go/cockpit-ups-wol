# cockpit-ups-wol roadmap

This roadmap tracks implementation and release readiness. An item is complete only when code or an executable acceptance gate exists.

## v0.1 — safe appliance baseline

### Complete software baseline

- Go binaries: `cockpit-ups-wol-agent`, `cockpit-ups-wolctl`, `cockpit-ups-wol-health`, and `wolctl`.
- Strict versioned YAML configuration parsing and validation.
- Transactional configuration revisions with candidate/probation/known-good/rollback state.
- Durable power-state generations with checksum validation, `current`/`previous` fallback, fsync and atomic rotation.
- NUT `upsc` adapter with explicit `UNKNOWN` communication-failure semantics.
- NUT primary/master validation before `upsmon -c fsd`.
- Deterministic outage/recovery state machine with durable shutdown and recovery commit points.
- Full `armed` orchestration with persisted pre-outage snapshot, host-request persistence before side effects, FSD ownership, reboot reconciliation and recovery stop on renewed unsafe power.
- Fixed-argv SSH shutdown, NUT-managed-host separation, TCP/ping probing and consecutive confirmation.
- Wake-on-LAN packet generation, interface/broadcast selection, dependency-aware ordering and durable bounded retries.
- Health supervisor with durable repair circuit breaker, conservative autofix, systemd watchdog and `FAILED_SAFE`.
- Local Unix-socket IPC and Cockpit/CLI status/plan/config-revision access.
- Cockpit React/TypeScript/PatternFly dashboard with Overview, UPS, Devices, Automation, Reliability, Settings and Logs.
- Privileged, confirmed configuration rollback through the control boundary.
- Single transactional installer supporting default, `--silent`, and guided `--tui` paths on the same backend.
- TUI system/package/profile/UPS/Synology/security/policy/topology/dependency/host/mode/review/progress/result flow.
- Local UPS discovery using NUT scanner output when available, with explicit `--ups-driver` / `--ups-port` overrides and fail-safe ambiguity handling.
- Trusted-LAN default plus optional restricted NUT mode using an additive project-owned nftables table/service; no host firewall flush/replacement.
- IPv4/IPv6 NUT listener generation tied to the restricted policy.
- Fresh installs start with empty host/dependency inventories; example addresses/MACs are never copied into live configuration.
- Service autostart, health probation, initial last-known-good bootstrap, application/config/Cockpit/firewall rollback and idempotent reinstall.
- Real Ubuntu 24.04 CI acceptance with NUT `dummy-ups`, systemd and Cockpit, including deliberately broken-upgrade rollback.
- Generated NUT primary/secondary + Synology monitor-only integration tests.
- Power-bounce, interrupted-boot, stale/corrupt state, config-failure and ambiguous-restart fault tests.
- amd64 native runtime smoke plus arm64 and riscv64 runtime smoke under QEMU.
- Reproducible amd64/arm64/riscv64 appliance archives with SHA256SUMS and prebuilt Cockpit assets.
- Physical hardware preflight helper and evidence protocol.

### Remaining v0.1 release gates

- Physical full outage/recovery acceptance on a representative **amd64 controller + real supported UPS**.
- Physical full outage/recovery acceptance on a representative **arm64 controller + real supported UPS**.
- Real **Synology DSM** NUT-secondary shutdown/recovery acceptance.
- Select and add the root project **LICENSE** before tagged public release/source reuse.

These are intentionally not replaced by QEMU, `dummy-ups`, or documentation-only evidence.

## v0.2 — administration and integrations

- Additional safe command registry for explicitly allowlisted non-SSH shutdown actions.
- Advanced UPS commands and writable-variable management with privilege separation.
- Multi-UPS policy support.
- Proxmox API adapter and VM/container-aware shutdown planning.
- Additional dependency startup methods after durable dependency-action state is implemented.
- Optional native Go NUT client after protocol/TLS requirements are finalized.
- Richer Cockpit CRUD/configuration editors for hosts/dependencies and maintenance/manual intervention workflows.
- Optional UPS load/output management after hardware capability classification.

## v0.3 — observability and ecosystem

- Event history and power-cycle timeline.
- Prometheus/metrics export.
- Notifications and external integrations.
- Fleet/remote status aggregation where appropriate for protected homelabs.

## Release principle

A feature is not complete merely because it compiles. Safety-relevant work requires an executable acceptance path covering persistence, restart behavior and fail-closed handling. Physical electrical behavior must be verified on real hardware where simulation cannot establish it.
