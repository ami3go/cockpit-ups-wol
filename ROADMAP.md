# cockpit-ups-wol roadmap

This roadmap tracks implementation and release readiness. A safety-relevant item is complete only when code and an executable acceptance path exist.

## v0.1 — safe appliance baseline

### Complete software baseline

- Go binaries: `cockpit-ups-wol-agent`, `cockpit-ups-wolctl`, `cockpit-ups-wol-health`, and `wolctl`.
- Strict versioned YAML parsing plus semantic/cross-reference validation.
- Transactional configuration revisions with candidate/probation/known-good/rollback state.
- Durable power-state generations with checksums, fsync, atomic rotation and valid previous-generation fallback.
- NUT `upsc` normalization with explicit `UNKNOWN` communication-failure semantics.
- NUT primary/master ownership validation before `upsmon -c fsd`, including fail-closed multi-primary handling.
- Deterministic outage/recovery state machine with durable shutdown and recovery commit points.
- Full `armed` orchestration with pre-outage host snapshots, side-effect intent persistence, FSD ownership, reboot reconciliation and renewed-outage handling during partial recovery.
- Fixed-argv SSH shutdown, NUT-managed-host separation, TCP/ping verification, bounded retries and `FAILED_SAFE` exhaustion handling.
- Managed-host Wake-on-LAN with durable bounded retries, dependency-aware ordering and reboot-safe inter-host delay.
- Stable-utility, UPS charge/runtime/recharge, network and health recovery gates.
- Health supervisor with durable repair circuit breaker, conservative autofix and systemd watchdog integration.
- Minimal local Unix-socket health IPC plus privileged CLI/revision-manager configuration control.
- Cockpit React/TypeScript/PatternFly management UI with overview, UPS, devices, automation, reliability, settings and logs.
- Single transactional installer supporting default, `--silent`, and guided `--tui` paths.
- Durable interrupted-install marker and boot-time rollback/recovery guard.
- Local UPS discovery using NUT scanner output with explicit driver/port overrides and ambiguity failure.
- Trusted-LAN default plus optional additive restricted NUT nftables policy with IPv4/IPv6 protection.
- Fresh installs start with empty host/dependency inventories and `dry-run` mode.
- Service autostart, health probation, initial last-known-good bootstrap and idempotent reinstall.
- Real Ubuntu 24.04 NUT `dummy-ups` + systemd + Cockpit install/probation/idempotency/rollback acceptance.
- Systemd-enabled `.deb` installation acceptance on Debian 12, Debian 13, Ubuntu 24.04 and Ubuntu 26.04, including APT dependency resolution, NUT dummy UPS configuration, service/health/Cockpit verification and idempotent setup.
- Generated NUT primary/secondary + Synology monitor-only integration tests.
- Power-bounce, interrupted-boot, stale/corrupt state, config failure, communication-loss and ambiguous-restart tests.
- amd64 native runtime smoke plus arm64/riscv64 runtime smoke under QEMU.
- Reproducible amd64/arm64/riscv64 appliance archives and Debian packages with SHA256 checksums.
- Locked Cockpit npm graph using `package-lock.json`/`npm ci`, current React 19.3.0 and PatternFly 6.6.1 stack.
- GitHub Actions pinned to immutable SHAs with Dependabot monitoring.
- GNU AGPL-3.0-or-later project license and release-bundle license inclusion.
- Hardware acceptance procedure and non-destructive preflight helper.

### Explicit v0.1 capability boundary

The schema retains some future-facing values, but armed v0.1 deliberately rejects capabilities whose safe durable execution model is not complete:

```text
shutdown.method: command
ARP-only host verification
dependency Wake-on-LAN
```

Those remain future work; they are not release blockers because the accepted v0.1 feature set fails closed when they are configured for armed operation.

### Remaining v0.1 release gates

- Physical full outage/recovery acceptance on a representative **amd64 controller + real supported UPS**.
- Physical full outage/recovery acceptance on a representative **arm64 controller + real supported UPS**.
- Real **Synology DSM** NUT-secondary shutdown/recovery acceptance.
- Configure repository-admin protection for **`main`** so changes require pull requests and green project CI, and force-push/deletion are blocked (issue #39).

The physical gates are intentionally not replaced by QEMU, `dummy-ups`, or documentation-only evidence. Branch protection is a repository-governance gate, not a runtime implementation task.

## v0.2 — administration and integrations

- Safe allowlisted command registry with durable retry/reconciliation semantics.
- ARP-capable verification only after deterministic/reliable acceptance coverage exists.
- Durable dependency power/WoL actions.
- Advanced UPS commands and writable-variable management with privilege separation.
- Multi-UPS policy support.
- Proxmox API adapter and VM/container-aware shutdown planning.
- Optional native Go NUT client after protocol/TLS requirements are finalized.
- Richer form-based Cockpit CRUD for hosts/dependencies and maintenance/manual-intervention workflows.
- Optional UPS load/output management after hardware capability classification.
- Broader IPC methods only where they add value beyond the privileged CLI/control boundary.

## v0.3 — observability and ecosystem

- Event history and power-cycle timeline.
- Prometheus/metrics export.
- Notifications and external integrations.
- Fleet/remote status aggregation where appropriate for protected homelabs.

## Release principle

A feature is not complete merely because it compiles. Safety-relevant work requires an executable acceptance path covering persistence, restart behavior and fail-closed handling. Physical electrical behavior must be verified on real hardware where simulation cannot establish it.
