# v0.1 Implementation Decisions

**Status:** Implemented/frozen v0.1 software baseline  
**Release state:** software acceptance green for the accepted feature set; real UPS/DSM acceptance and `main` branch governance remain open

## 1. Agent and helpers

The agent, control CLI, health helper and WoL helper are implemented in **Go**:

```text
cockpit-ups-wol-agent
cockpit-ups-wolctl
cockpit-ups-wol-health
wolctl
```

Go provides small release binaries, straightforward amd64/arm64/riscv64 builds and low runtime dependency overhead. Normal release installation does not require a Go compiler.

The current WoL packet implementation is maintained in this repository. Future copied/adapted upstream code requires exact attribution before merge.

## 2. Cockpit frontend

Stack:

```text
TypeScript
React 19.3.0
PatternFly 6.6.1 family
Cockpit JavaScript API
```

Node/npm are build-time dependencies only. The Cockpit dependency graph is locked by `package-lock.json`; CI/package workflows use `npm ci`.

Cockpit is management-only and is not required for automatic outage/recovery behavior.

## 3. Installer

Implementation:

```text
Bash entry point
modular installer libraries
systemd runtime integration
```

Source-tree entry points:

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

All modes use one transactional backend. Real installation requires systemd to be PID 1 and rejects unsupported container/chroot environments before install transaction state is created.

Normal packaged/release installation consumes prebuilt Go/Cockpit artifacts and does not require a build toolchain on the target.

## 4. Configuration

Format: YAML.

Validation source of truth: `schemas/config.schema.json` plus Go semantic/cross-reference/safety validators.

Fresh installations deliberately start with empty `hosts` and `network_dependencies` and default to `dry-run`.

The schema carries some future-facing values. Armed v0.1 intentionally rejects:

```text
shutdown.method: command
ARP-only host verification
dependency Wake-on-LAN
```

## 5. Persistent state

Safety state is JSON with deterministic checksum calculation and crash-safe rotation:

- current/previous durable generations;
- sequence + checksum validation;
- temp write + fsync + atomic rename + parent-directory fsync;
- outage/action identity;
- per-host shutdown/recovery progress;
- valid previous-generation fallback;
- new outage epoch when utility fails during partial recovery.

No database is required for v0.1 safety state.

## 6. Local control boundary

### Agent IPC

The current Unix-domain socket is deliberately small. It uses one JSON request/response per connection and currently implements `GetHealth`.

It does **not** implement the broad historical draft of mutating/status methods or a completed generic `SO_PEERCRED` authorization framework.

### Configuration/management CLI

`cockpit-ups-wolctl` provides the implemented management surface for:

```text
health
config-get
config-validate
config-apply
config-status
config-rollback
plan
logs
config-bootstrap
```

Privileged configuration mutations are root-gated. `config-apply` hands activation/probation to a transient systemd unit so browser/channel loss does not interrupt the transaction.

## 7. Service manager and logging

systemd is the v0.1 runtime service model. Runtime integration includes automatic startup, agent watchdog notification, health timer, installer recovery unit and optional project firewall service.

Runtime logs use journald. Installer logs are persisted under `/var/log/cockpit-ups-wol/`.

## 8. NUT integration

v0.1 uses installed NUT tools/services rather than embedding a native NUT protocol implementation.

The agent uses argv-safe tools such as `upsc` and the validated primary FSD path. Communication failure is `UNKNOWN`, never inferred as online or full battery.

Canonical project `hosts_sync_seconds` and `final_delay_seconds` render managed NUT timing. Existing-NUT mode fails closed when multiple primary/master monitor entries make process-wide FSD ownership ambiguous.

A native Go NUT client remains optional later work.

## 9. Host adapters

Accepted armed-v0.1 shutdown behavior:

```text
nut   — NUT secondary/FSD path
ssh   — fixed argv constrained remote shutdown with bounded reconciliation/retry
none  — no controller-issued direct shutdown
```

`command` remains schema/design space only and fails closed in armed v0.1.

Accepted armed host verification uses deterministic implemented paths such as TCP/ping. ARP-only verification remains unsupported.

Managed-host WoL is implemented with durable attempts. Dependency WoL remains deferred until dependency actions receive equivalent durable semantics.

## 10. Build and tests

Current automated acceptance includes:

- Go unit tests and `go vet`;
- canonical configuration/NUT/Synology integration;
- outage/recovery/reboot/fault tests;
- state corruption and config rollback/reconciliation;
- installer discovery/network/transaction tests;
- Ubuntu 24.04 NUT `dummy-ups` + real systemd + Cockpit E2E;
- amd64 native runtime smoke;
- arm64/riscv64 QEMU runtime smoke;
- Cockpit strict TypeScript and production bundle build;
- reproducible package/checksum verification.

Unsupported future capabilities are validated as fail-closed negative cases rather than positive armed execution.

## 11. Release artifacts

Release packaging produces amd64/arm64/riscv64 appliance archives, Debian packages, `SHA256SUMS`, prebuilt Cockpit assets and the project license.

A manual GitHub testing-release path publishes verified pre-release artifacts.

## 12. Network security

Default NUT network mode: `trusted-lan`.

Optional `restricted` mode uses a dedicated project-owned nftables table for TCP/3493, protects enabled IPv4/IPv6 listeners and does not flush/replace unrelated firewall state.

## 13. Dependency and source-reuse policy

GitHub Actions are pinned to immutable commit SHAs and monitored by Dependabot. Go/npm dependencies are versioned through normal module/lock metadata.

Every copied/adapted upstream source component requires exact source project/path/commit, license review, required notice retention and an entry in `THIRD_PARTY_NOTICES.md`.

Project license: **AGPL-3.0-or-later**.

## 14. Deferred choices

Not part of accepted v0.1:

```text
native Go NUT protocol client
D-Bus management API
embedded database
command shutdown adapter
ARP-only armed verification
dependency WoL without durable action state
multi-UPS policy
Proxmox API adapter
multi-controller HA
cloud service
```

## 15. Remaining release gates

Software baseline completion does not replace physical/governance release evidence. v0.1 remains gated by issue #10 real amd64/arm64 UPS + Synology DSM acceptance and issue #39 `main` branch protection requiring pull requests and green CI.
