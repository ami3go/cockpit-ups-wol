# cockpit-ups-wol

`cockpit-ups-wol` is a small homelab UPS-management appliance built around Network UPS Tools (NUT), a persistent safety agent, Wake-on-LAN, and Cockpit.

The project is currently **pre-release**. The v0.1 software baseline is implemented and continuously tested, including armed orchestration, Cockpit management, transactional installation/rollback, Debian packaging and multi-architecture release artifacts.

The remaining public-v0.1 gates are deliberately narrow:

- physical outage/recovery acceptance on a representative amd64 controller + real UPS;
- physical outage/recovery acceptance on a representative arm64 controller + real UPS;
- real Synology DSM NUT-secondary shutdown/recovery acceptance;
- repository-admin protection for `main` requiring pull requests and green CI (issue #39).

Software/QEMU/`dummy-ups` results do not substitute for physical acceptance.

## Goals

- share a UPS safely with Linux, Synology DSM and other network clients;
- shut managed systems down in deterministic order during an outage;
- shut the controller down last when required;
- remember what was running before the outage;
- resume safely after controller reboot or interrupted boot;
- restore only eligible systems after utility power is stable and the UPS has recovered;
- default recovery gate: **80% battery charge**;
- provide Cockpit-based management without putting Cockpit in the safety-critical path;
- automatically start required services after reboot;
- continuously health-check the stack and perform bounded safe repair;
- make project-managed configuration changes transactional with automatic rollback.

## Safety model

The controller SBC must be powered from a **battery-backed UPS output** unless it has an equivalently reliable independent backed supply, and it must boot automatically whenever backed power returns. Required Ethernet switching/routing must remain powered long enough for shutdown coordination.

A boot is never treated as proof that utility power has recovered. Recovery requires trustworthy NUT state, a full stable-utility interval, the configured charge/runtime/recharge gate, network readiness, a known-good configuration and acceptable runtime health.

New installations start in **dry-run** mode. Fresh installs contain no managed hosts or network dependencies; example addresses and MACs are never copied into live configuration. Enroll real devices, review the rendered shutdown/recovery plan and physical topology, then arm automation.

## Current implementation

The v0.1 software baseline includes:

- Go safety agent plus `cockpit-ups-wolctl`, `cockpit-ups-wol-health`, and `wolctl`;
- NUT normalization with explicit `UNKNOWN` on communication failure;
- primary/master ownership validation before FSD;
- crash-safe power state with checksums, fsync and previous-generation fallback;
- deterministic outage/recovery state machine with durable commit points;
- durable pre-outage host snapshots and per-host action state;
- ordered fixed-argv SSH shutdown with bounded retry/reconciliation;
- NUT-managed secondary separation so Synology/Linux secondaries are not shut down twice;
- controller/primary FSD ownership after pre-FSD hosts settle;
- Wake-on-LAN with durable bounded retry state and dependency-aware host ordering;
- power-bounce/reboot reconciliation, including renewed outages during partial recovery;
- stable-AC, UPS charge/runtime/recharge, network and health recovery gates;
- transactional configuration revisions with probation, last-known-good promotion and rollback;
- durable health circuit breaker, conservative autofix and `FAILED_SAFE`;
- a minimal Unix-socket IPC health endpoint plus privileged CLI/config transaction control;
- Cockpit TypeScript/React/PatternFly UI for overview, UPS, devices, automation, reliability, settings and logs;
- transactional installer with default, `--silent` and guided `--tui` modes;
- durable interrupted-install recovery on the next boot;
- NUT scanner-based local UPS discovery plus explicit driver/port selection;
- trusted-LAN default and optional additive restricted nftables policy for NUT TCP/3493;
- IPv4/IPv6 listener generation tied to the selected policy;
- generated NUT primary/secondary + Synology monitor-only integration tests;
- amd64 native runtime smoke plus arm64/riscv64 QEMU runtime smoke;
- reproducible amd64/arm64/riscv64 appliance archives and Debian packages with SHA256 checksums;
- a manual testing-release workflow for pre-release artifacts.

## Capability boundary

The configuration schema includes some values reserved for later safe implementations. In **armed v0.1** the following intentionally fail closed:

- `shutdown.method: command` — no accepted allowlisted command registry yet;
- ARP-only host verification — use supported TCP/ping verification instead;
- dependency Wake-on-LAN — dependency actions do not yet have the same durable action semantics as managed hosts.

Supported direct v0.1 shutdown methods are `ssh`, `nut`, and `none` as appropriate to the host role.

## Installation

Main source-tree entry points:

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

Useful non-destructive checks:

```bash
./install.sh --check
./install.sh --check --profile remote-client
./install.sh --check --profile local-server --network-mode restricted --allow-client 192.168.1.0/24
sudo ./scripts/test/hardware-preflight.sh --ups-target ups@localhost
```

For unattended local-UPS installs, explicit hardware selection is available with:

```bash
sudo ./install.sh --silent --profile local-server \
  --ups-driver usbhid-ups --ups-port auto
```

The installer fails rather than guessing when safe hardware/profile selection is ambiguous. It requires systemd to be the active system manager for real installation and rejects unsupported container environments before creating install transaction state.

The installer is transactional for project-managed binaries, Cockpit assets, configuration, systemd units, project firewall state and NUT files. A failed install/upgrade restores the previous coherent application/config/service state; a sudden power interruption is reconciled on the next boot before automation starts.

## NUT network modes

`trusted-lan` is the default and does not install project firewall restrictions.

`restricted` is optional for a local NUT server. It requires explicit allowed client CIDRs and uses a project-owned nftables table that only governs NUT TCP/3493. It never flushes or replaces unrelated administrator firewall rules. If IPv6 NUT listening is enabled, IPv6 is restricted as well.

## Operating modes

- `monitor` — observe state without automatic power actions;
- `dry-run` — evaluate and display the plan without destructive/wake actions; default for a new installation;
- `armed` — execute the validated persistent shutdown/recovery lifecycle;
- `maintenance` — inhibit automatic power orchestration during maintenance.

The first-install TUI deliberately does not arm the system automatically.

## Synology

Synology DSM compatibility is built in but disabled by default:

```bash
sudo ./install.sh --synology
```

The preferred model is DSM as a NUT secondary/client. Where the tested DSM version requires the compatibility account, the generated profile uses UPS name `ups` and monitor-only `monuser` / `secret` credentials. The account receives only `upsmon secondary` capability; generated tests reject `actions`/`instcmds` privileges.

A Synology host using `shutdown.method: nut` is left to the NUT/FSD path; the agent does not issue a duplicate SSH shutdown.

Real DSM hardware acceptance is still required before v0.1 release.

## Testing and release integrity

CI currently gates Go unit/vet tests, strict configuration/NUT integration, installer regression tests, Ubuntu 24.04 NUT `dummy-ups` + systemd + Cockpit E2E, multi-architecture runtime smoke, Cockpit TypeScript/build validation with locked dependencies, reproducible package/checksum generation, and a systemd-enabled Debian-package installation matrix on **Debian 12, Debian 13, Ubuntu 24.04, and Ubuntu 26.04**. The distro matrix installs the generated `.deb` through APT, runs safe `dry-run` setup with a NUT dummy UPS, verifies services/health/Cockpit assets, and repeats setup for idempotency.

GitHub Actions are pinned to immutable commit SHAs and monitored by Dependabot. Cockpit dependencies are locked with `package-lock.json` and CI/package builds use `npm ci`.

The exact physical release protocol is in `docs/HARDWARE_ACCEPTANCE.md`.

## Documentation

Primary references:

- `SOFTWARE_ARCHITECTURE.md`
- `docs/CONFIGURATION.md`
- `docs/INSTALLATION_REQUIREMENTS.md`
- `docs/RELIABILITY_REQUIREMENTS.md`
- `docs/BOOT_RECOVERY_REQUIREMENTS.md`
- `docs/STATE_MODEL.md`
- `docs/NUT_SHUTDOWN_MODEL.md`
- `docs/POWER_POLICY.md`
- `docs/IPC.md`
- `docs/DEPLOYMENT.md`
- `docs/OPERATING_MODES.md`
- `docs/SECURITY.md`
- `docs/TEST_PLAN.md`
- `docs/HARDWARE_ACCEPTANCE.md`
- `docs/NUT.md`
- `docs/SYNOLOGY.md`
- `docs/RELATED_PROJECTS.md`
- `docs/READINESS_AUDIT.md`
- `ROADMAP.md`

## License

This project is licensed under the **GNU Affero General Public License v3.0 or later**.

`SPDX-License-Identifier: AGPL-3.0-or-later`

See `LICENSE`. Third-party projects and dependencies retain their own licenses and attribution obligations; reuse tracking is in `THIRD_PARTY_NOTICES.md` and `docs/RELATED_PROJECTS.md`.
