# Changelog

All notable project changes are recorded here while the project is pre-release.

## Unreleased

### Added

- NUT-based outage handling with controller-last shutdown and ordered recovery.
- Synology DSM/NUT-secondary compatibility profile and generated privilege checks.
- Stable-utility and UPS recharge recovery policy, defaulting to 80% charge when available.
- Automatic service startup, watchdog supervision, bounded health autofix and durable `FAILED_SAFE` handling.
- Interrupted-boot and repeated-power-bounce reconciliation with durable shutdown/recovery commit points.
- Transactional configuration revisions with candidate validation, probation, last-known-good promotion and automatic rollback.
- Canonical v1 YAML configuration schema and example configuration.
- Checksummed current/previous durable power-state generations and per-host action state.
- Minimal Unix-domain-socket agent IPC used by the health CLI path; privileged configuration management is implemented by `cockpit-ups-wolctl` and the revision manager.
- Go agent/CLI implementation covering durable state, NUT/FSD policy, host adapters, WoL and health supervision.
- Transactional installer with Ubuntu 24.04 NUT `dummy-ups`/systemd/Cockpit E2E and broken-upgrade rollback acceptance.
- Guided installer TUI using `dialog` with `whiptail` fallback and the same backend as default/silent installation.
- NUT local UPS discovery with explicit `--ups-driver` / `--ups-port` overrides and fail-safe ambiguity handling.
- Optional project-owned restricted NUT nftables policy with IPv4/IPv6 coverage and no host firewall flush.
- Cockpit React/PatternFly management UI with overview, UPS, devices, automation, reliability, settings/log views and privileged transactional configuration workflows.
- Full armed orchestration with durable per-host shutdown intent, primary FSD ownership, restart reconciliation, recovery gates and persistent WoL recovery.
- Network dependency validation and fail-closed armed preflight.
- Power-bounce, ambiguous-shutdown, communication-loss and post-reboot stability regression tests.
- Native amd64 plus QEMU arm64/riscv64 runtime smoke gates.
- Reproducible amd64/arm64/riscv64 appliance archives and Debian packages with SHA256 checksums.
- Manual GitHub testing-release workflow for verified pre-release artifacts.
- Project licensing under GNU AGPL-3.0-or-later with canonical root `LICENSE`.
- Dependabot monitoring for GitHub Actions, Go modules and Cockpit npm dependencies.
- Hardware acceptance protocol and non-destructive hardware preflight helper.

### Changed

- Cockpit is a management surface; the safety-critical outage/recovery engine remains independent of the browser session.
- Cockpit frontend dependencies are now locked and installed with `npm ci`; the current stack uses React/React DOM 19.3.0 and aligned PatternFly 6.6.1 packages.
- GitHub Actions are pinned to immutable commit SHAs and have been updated through the dependency review workflow.
- Unknown/failed NUT communication is normalized to `UNKNOWN`, never assumed online or fully charged.
- The controller SBC and required local network infrastructure are normative UPS-backed deployment requirements.
- A reboot during an active outage does not grant a new outage grace period.
- A new outage during partial recovery creates a new durable outage epoch and re-evaluates already-restored hosts for shutdown.
- Installer NUT readiness requires a trustworthy online observation rather than treating any successful `upsc` response as ready.
- Fresh installations generate empty `hosts` and `network_dependencies`; example IP/MAC targets remain documentation-only.
- New installations default to `dry-run`; the first-install TUI deliberately defers `armed` mode until real targets/topology are validated.
- NUT listener generation follows selected IPv4/IPv6 network policy.
- Installer rollback covers project binaries, config, Cockpit assets, systemd state, NUT files and project firewall state.
- Interrupted installation is recovered from a durable pending marker before automation resumes after boot.
- Real installation now rejects environments where systemd is not PID 1 before creating transaction state.
- Release bundles include the project license and checksums cover appliance archives and Debian packages.

### Security / safety

- `shutdown.method: command`, armed ARP-only verification and dependency WoL intentionally fail closed in v0.1 until their durable execution/verification models are implemented.
- Supported direct armed shutdown behavior is limited to accepted `ssh`, `nut`, and `none` semantics.
- Wake-enabled managed hosts require verifiable online state, MAC and IPv4 broadcast configuration.
- NUT-managed hosts are not sent duplicate direct shutdown commands.
- External shutdown, FSD and wake side effects are preceded by durable state transitions.
- Restricted NUT mode creates only the additive `inet cockpit_ups_wol` nftables table and never emits `flush ruleset`.
- Silent local UPS setup fails instead of guessing when discovery is missing or ambiguous unless driver/port are explicit.
- Existing-NUT mode rejects ambiguous multi-primary FSD ownership.

### Release blockers

- Physical UPS acceptance on representative amd64 hardware.
- Physical UPS acceptance on representative arm64 hardware.
- Real Synology DSM NUT-secondary shutdown/recovery acceptance.
- Repository-admin protection of `main` requiring pull requests and green CI (issue #39).
