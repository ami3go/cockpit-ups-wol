# cockpit-ups-wol

`cockpit-ups-wol` is a small homelab UPS-management appliance built around Network UPS Tools (NUT), a persistent safety agent, Wake-on-LAN, and Cockpit.

The project is currently **pre-alpha**. Core safety libraries and installer scaffolding exist, but the full long-running runtime loop and Cockpit frontend are still under active implementation.

## Goals

- share a UPS safely with Linux, Synology DSM and other network clients
- shut managed systems down in deterministic order during an outage
- shut the controller down last when required
- remember what was running before the outage
- resume safely after controller reboot or interrupted boot
- restore only eligible systems after utility power is stable and the UPS has recovered
- default recovery gate: **80% battery charge**
- provide Cockpit-based management without making Cockpit part of the safety-critical path
- automatically start required services after reboot
- continuously health-check the stack and perform bounded safe repairs
- make project-managed configuration changes transactional and automatically roll back failures

## Safety model

The controller SBC should be powered from a **battery-backed UPS output** and should boot automatically whenever UPS output returns. Required Ethernet switching/routing must remain powered long enough for shutdown coordination.

A boot is never treated as proof that utility power has recovered. Recovery requires valid NUT status, a stable-utility interval, the configured battery/runtime gate, network readiness, a known-good configuration and healthy runtime state.

New installations start in **dry-run** mode. Arming automation should happen only after shutdown and recovery plans have been verified.

## Current implementation

Implemented foundations include:

- Go agent/CLI module
- normalized NUT parsing with explicit `UNKNOWN` on communication failure
- crash-safe state persistence with checksum and previous-generation fallback
- deterministic power-state machine and durable shutdown/recovery commit points
- ordered host shutdown/recovery planning
- WoL packet sender and durable retry state
- transactional configuration revisions with probation and last-known-good rollback
- durable health circuit breaker and `FAILED_SAFE`
- Unix-socket health IPC
- systemd agent/watchdog and health timer units
- multi-architecture compile verification for amd64, arm64 and riscv64
- transactional installer framework with Debian/Ubuntu, Arch and Fedora-family modules
- installer rollback snapshots, service autostart, health probation and optional Synology profile

Still incomplete:

- full long-running agent runtime wiring
- SSH/command host shutdown adapters
- end-to-end installer acceptance on real supported systems
- Cockpit frontend
- hardware UPS/Synology acceptance
- release artifacts/checksums
- project license selection

## Development installer

The repository now contains a single installer entry point:

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

Useful development checks:

```bash
./install.sh --check
./install.sh --check --profile remote-client
```

The installer is transactional at the project-file level: it snapshots project-managed binaries, configuration, systemd units and NUT files before changes and restores them if the installation health/probation gate fails. Existing non-empty NUT configuration is preserved rather than overwritten.

Because the long-running runtime loop is not yet complete, **the installer should not yet be treated as production-ready**.

## Synology

Synology DSM compatibility is built into the design but is **disabled by default**. The installer can explicitly enable the compatibility profile with:

```bash
sudo ./install.sh --synology
```

The preferred design is for DSM to act as a NUT secondary/client and perform its own safe shutdown. The project avoids sending duplicate SSH shutdown commands to a Synology host configured for NUT shutdown.

## Documentation

Key design documents include:

- `SOFTWARE_ARCHITECTURE.md`
- `docs/INSTALLATION_REQUIREMENTS.md`
- `docs/RELIABILITY_REQUIREMENTS.md`
- `docs/BOOT_RECOVERY_REQUIREMENTS.md`
- `docs/CONFIGURATION.md`
- `docs/STATE_MODEL.md`
- `docs/NUT_SHUTDOWN_MODEL.md`
- `docs/POWER_POLICY.md`
- `docs/IPC_API.md`
- `docs/DEPLOYMENT.md`
- `docs/OPERATING_MODES.md`
- `docs/SECURITY.md`
- `docs/TEST_PLAN.md`
- `docs/NUT.md`
- `docs/SYNOLOGY.md`
- `docs/RELATED_PROJECTS.md`
- `docs/READINESS_AUDIT.md`
- `ROADMAP.md`

## License

A project license has not yet been selected. Do not assume permission to reuse project source code until a root `LICENSE` file is added.

Upstream projects reviewed for possible reuse and their licenses are tracked in `THIRD_PARTY_NOTICES.md` and `docs/RELATED_PROJECTS.md`.
