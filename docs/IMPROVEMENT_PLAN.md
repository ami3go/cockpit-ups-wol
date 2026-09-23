# cockpit-ups-wol — Improvement Plan Status

**Original plan date:** 2026-09-19  
**Reconciled:** 2026-09-23  
**Status:** Original v0.1 implementation plan substantially completed; retained as a completion record

## 1. Purpose

This document originally converted the first readiness audit into an implementation checklist. The implementation has advanced past that stage.

Current authoritative status lives in:

- `docs/READINESS_AUDIT.md` — release-gate readiness;
- `ROADMAP.md` — current v0.1 boundary and future v0.2/v0.3 work;
- GitHub issues — open executable/admin work.

This file no longer represents an active engineering queue.

## 2. Completed v0.1 workstreams

Completed software work includes:

- canonical architecture/state reconciliation;
- NUT primary/FSD/output ownership model;
- outage/recovery hysteresis, bounded communication-loss handling and fail-safe `UNKNOWN` semantics;
- canonical YAML configuration + schema/semantic validation;
- crash-safe persistent state with current/previous generations;
- Go agent/CLI implementation stack;
- UPS-backed controller/network deployment requirements;
- automatic service startup, watchdog and bounded health autofix;
- durable `FAILED_SAFE` behavior;
- transactional configuration with probation, immutable known-good revisions and rollback;
- application/Cockpit/systemd/NUT/firewall rollback during failed upgrades;
- deterministic armed outage/recovery orchestration;
- accepted SSH/NUT direct shutdown paths with bounded reconciliation/retry;
- durable managed-host Wake-on-LAN recovery;
- Cockpit management UI and privileged transactional configuration workflow;
- Synology NUT-secondary software integration;
- one default/silent/TUI installer backend;
- durable interrupted-install boot recovery;
- local UPS discovery and explicit driver/port selection;
- trusted-LAN and optional restricted NUT networking;
- additive project-owned nftables policy with IPv4/IPv6 protection;
- fresh-install empty device/dependency inventory and `dry-run` default;
- real Ubuntu/NUT/systemd/Cockpit installation/rollback acceptance;
- amd64/arm64/riscv64 software build/runtime gates;
- reproducible appliance archives, Debian packages and SHA256 checksums;
- immutable Action pinning, Dependabot and locked npm dependencies;
- AGPL-3.0-or-later project licensing;
- hardware acceptance procedure and preflight helper.

## 3. Explicitly deferred capabilities

The schema contains some future-facing values, but these are intentionally **not** accepted armed-v0.1 capabilities:

```text
shutdown.method: command
ARP-only host verification
dependency Wake-on-Lan
```

They fail closed rather than becoming release blockers for the accepted v0.1 feature set. Their safe durable execution/verification models belong to later work.

## 4. Remaining v0.1 release gates

```text
[ ] amd64 controller + real supported UPS full outage/recovery test
[ ] arm64 controller + real supported UPS full outage/recovery test
[ ] real Synology DSM NUT-secondary shutdown/recovery test
[ ] main branch protection/ruleset configured (issue #39)
```

The three physical tests are tracked by issue #10. Repository governance is tracked by issue #39.

The project license is already selected as `AGPL-3.0-or-later`; the former license-selection issue is complete and is no longer a release gate.

## 5. Release boundary

A software/QEMU/`dummy-ups` or USB-simulator result does not satisfy a real-UPS hardware gate.

A public/tagged v0.1 should not be created until:

1. issue #10 has retained physical evidence for amd64, arm64 and DSM acceptance;
2. issue #39 is resolved with `main` protected by pull-request/green-CI governance;
3. final release CI/package checks remain green on the release commit.

Any future copied/adapted upstream source still requires exact source/version/license attribution in `THIRD_PARTY_NOTICES.md` before merge.

## 6. Deferred enhancements

Later releases may add:

- safe allowlisted command adapters;
- ARP verification after deterministic acceptance coverage;
- durable dependency power/WoL actions;
- multi-UPS policy;
- Proxmox API/VM-aware orchestration;
- advanced UPS writable-variable/instant-command management;
- native Go NUT client if justified;
- richer form-based Cockpit host/dependency CRUD;
- event history, metrics, notifications and fleet aggregation.

See `ROADMAP.md` for current ordering.

## 7. Completion principle

> A power-management feature is complete only when its failure, reboot and rollback behavior has an executable acceptance path. Hardware-specific electrical behavior requires evidence from real hardware.
