# Changelog

All notable project changes will be documented here.

The project is currently pre-release.

## Unreleased

### Architecture and safety model

- Defined Cockpit as management-only, outside the safety-critical path.
- Defined Go agent/CLI, Go WoL helper, TypeScript/React/PatternFly Cockpit UI and Bash installer baseline.
- Added canonical power-state model including `BOOT_RECONCILE`, shutdown/recovery commit points and `FAILED_SAFE`.
- Defined outage trigger precedence and recovery hysteresis.
- Defined UPS-backed controller and required network-infrastructure deployment model.
- Defined NUT primary/secondary/FSD/final output-shutdown ownership.
- Defined default automatic recovery gate of 80% UPS charge with fallback hierarchy.

### Reliability

- Added automatic service startup requirements.
- Added independent health supervisor requirements.
- Added bounded autofix/circuit-breaker behavior.
- Added transactional configuration revisions and automatic last-known-good rollback.
- Added interrupted-boot and repeated-power-bounce recovery requirements.
- Added power-loss-safe state generation and checksum model.

### Configuration and interfaces

- Added canonical config schema and example.
- Added canonical persistent state schema.
- Added local Unix-domain JSON IPC contract.
- Added operating modes: monitor, dry-run, armed, maintenance.
- New installations are specified to default to dry-run.

### Integration

- Added first-class Synology DSM compatibility model.
- Added NUT local-server, remote-client and existing-installation profiles.
- Added network dependency model for switches/routers and ordered recovery.
- Added multi-interface/broadcast WoL requirements.

### Installation/release planning

- Reconciled installer with health, probation, rollback and interrupted-install requirements.
- Added multi-architecture targets: amd64, arm64, riscv64.
- Added implementation roadmap and GitHub implementation epics.
- Added release acceptance test plan.
- Added security and third-party attribution requirements.

### Documentation

- Added/updated:
  - `README.md`
  - `SOFTWARE_ARCHITECTURE.md`
  - `ROADMAP.md`
  - `docs/INSTALLATION_REQUIREMENTS.md`
  - `docs/RELIABILITY_REQUIREMENTS.md`
  - `docs/BOOT_RECOVERY_REQUIREMENTS.md`
  - `docs/NUT_SHUTDOWN_MODEL.md`
  - `docs/POWER_POLICY.md`
  - `docs/CONFIGURATION.md`
  - `docs/STATE_MODEL.md`
  - `docs/IPC.md`
  - `docs/IMPLEMENTATION_DECISIONS.md`
  - `docs/OPERATING_MODES.md`
  - `docs/DEPLOYMENT.md`
  - `docs/SECURITY.md`
  - `docs/NUT.md`
  - `docs/SYNOLOGY.md`
  - `docs/TEST_PLAN.md`
  - `docs/RELATED_PROJECTS.md`
  - `docs/READINESS_AUDIT.md`
  - `THIRD_PARTY_NOTICES.md`

## Pre-project design history

Earlier design iterations established the initial goals of NUT-based UPS sharing, safe shutdown, Synology support, Cockpit management, and automatic restore after utility return and UPS recharge.
