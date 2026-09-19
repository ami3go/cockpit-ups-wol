# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

### Architecture and requirements

- Defined NUT/Cockpit/WoL safety architecture.
- Added Synology DSM compatibility requirements.
- Added deterministic shutdown and recovery state model including `BOOT_RECONCILE`, commit points and `FAILED_SAFE`.
- Added explicit NUT FSD/output-off ownership model.
- Added outage trigger precedence, recovery hysteresis and communication-loss behavior.
- Made UPS-backed controller/network topology a deployment requirement.
- Added health supervision, bounded autofix, automatic service startup and configuration rollback requirements.
- Added interrupted-boot and repeated-power-loss recovery requirements.

### Configuration and state

- Added canonical versioned configuration schema and example.
- Added persistent power-state schema with sequence ordering and per-host progress.
- Added immutable known-good configuration revision design.
- Added power-loss-safe state/config write requirements.
- Added Unix-socket IPC contract between Cockpit/CLI and the agent.
- Added formal `monitor`, `dry-run`, `armed` and `maintenance` operating modes.

### Research and compatibility

- Added deep open-source related-project reuse assessment.
- Added focused NUT and Synology integration documentation.
- Added security model and third-party attribution process.
- Added project readiness audit and v0.1 roadmap/backlog.

### Implementation

- Added Go agent/CLI foundation and multi-architecture compile verification.
- Added crash-safe persistent state store with checksum and previous-generation fallback.
- Added normalized NUT adapter with fail-safe `UNKNOWN` handling and primary-role FSD validation.
- Added deterministic power state machine, durable commit coordinator, host planning and ordered recovery logic.
- Added WoL packet generator, sender and durable retry/reconciliation logic.
- Added transactional configuration manager with runtime probation and automatic last-known-good rollback.
- Added durable health supervisor, bounded autofix circuit breaker, systemd watchdog support and health IPC.
- Added transactional installer framework with distro modules, service autostart, NUT-safe setup, rollback snapshots, health probation and installer CI checks.

### Documentation

- Added `README.md`.
- Added `docs/CONFIGURATION.md`.
- Added `docs/STATE_MODEL.md`.
- Added `docs/NUT_SHUTDOWN_MODEL.md`.
- Added `docs/POWER_POLICY.md`.
- Added `docs/IPC_API.md`.
- Added `docs/IMPLEMENTATION_DECISIONS.md`.
- Added `docs/DEPLOYMENT.md`.
- Added `docs/OPERATING_MODES.md`.
- Added `docs/SECURITY.md`.
- Added `docs/TEST_PLAN.md`.
- Added `docs/NUT.md`.
- Added `docs/SYNOLOGY.md`.
- Added `THIRD_PARTY_NOTICES.md`.

### Pending before first release

- Full long-running agent runtime loop and host action adapters.
- Cockpit management frontend.
- End-to-end installer acceptance on supported distributions.
- Real UPS/Synology hardware acceptance.
- Release artifact/checksum pipeline.
- Project license decision.
