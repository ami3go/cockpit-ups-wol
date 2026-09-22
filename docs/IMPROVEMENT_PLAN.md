# cockpit-ups-wol — Improvement Plan Status

**Original plan date:** 2026-09-19  
**Reconciled:** 2026-09-20  
**Status:** Original v0.1 implementation plan substantially completed; retained as a completion record

## 1. Purpose

This document originally converted the first readiness audit into an implementation checklist. The implementation work has now advanced past that stage.

Current authoritative status lives in:

- `docs/READINESS_AUDIT.md` — release-gate readiness
- `ROADMAP.md` — remaining v0.1 gates and future v0.2/v0.3 work
- GitHub issues — executable/open work

This file no longer represents an open task queue.

## 2. Completed v0.1 workstreams

The original blocking workstreams are complete:

- canonical architecture/state reconciliation
- NUT primary/FSD/output ownership model
- trigger precedence, outage/recovery hysteresis and fail-safe `UNKNOWN`
- canonical YAML configuration + JSON Schema
- crash-safe persistent state model
- Unix-socket Cockpit/CLI control boundary
- Go agent/CLI implementation stack
- UPS-backed controller/network deployment requirements
- automatic service startup, watchdog and bounded health autofix
- `FAILED_SAFE` behavior
- transactional configuration with probation, immutable known-good revisions and rollback
- application/Cockpit/systemd/NUT rollback during failed upgrades
- deterministic armed outage/recovery orchestration
- constrained SSH/NUT shutdown adapters
- durable Wake-on-LAN recovery
- Cockpit management UI
- Synology NUT-secondary software integration
- single default/silent/TUI installer backend
- guided TUI with system/package/profile/UPS/Synology/security/policy/topology/review/progress/result flow
- local UPS discovery and explicit driver/port selection
- trusted-LAN and optional restricted NUT networking
- additive project-owned nftables policy with IPv4/IPv6 protection
- fresh-install empty device/dependency inventory
- real Ubuntu/NUT/systemd/Cockpit installation and rollback acceptance
- amd64/arm64/riscv64 software build/runtime gates
- reproducible packages and SHA256 checksums
- hardware acceptance procedure and preflight helper

The final installer reconciliation was tracked as issue #15 and completed after the complete agent/architecture/installer/E2E CI gate passed.

## 3. Remaining v0.1 release gates

Only work that cannot be legitimately completed by software simulation remains:

```text
[ ] amd64 controller + real supported UPS full outage/recovery test
[ ] arm64 controller + real supported UPS full outage/recovery test
[ ] real Synology DSM NUT-secondary shutdown/recovery test
[ ] select/add root project LICENSE
```

The three physical tests are tracked by issue #10. Project licensing is tracked by issue #12.

## 4. Release boundary

A software/QEMU/`dummy-ups` result does not satisfy a real hardware gate.

A v0.1 public/tagged release should not be created until:

1. issue #10 has retained physical evidence for the required platforms and DSM;
2. the final release CI/package pipeline remains green on the release commit.

The project license is selected as `AGPL-3.0-or-later`. Any future copied/adapted upstream source still requires per-component license compatibility and attribution review before inclusion.

## 5. Deferred enhancements

The following belong to later releases rather than unfinished v0.1 safety work:

- multi-UPS policy
- Proxmox API/VM-aware orchestration
- additional allowlisted shutdown adapters
- dependency power/WoL actions with durable action state
- advanced UPS writable-variable/instant-command management
- native Go NUT client if later justified
- richer Cockpit host/dependency CRUD
- event history, metrics and notifications
- fleet/remote aggregation

See `ROADMAP.md` for the current ordering.

## 6. Completion principle

The original plan's safety principle remains unchanged:

> A power-management feature is complete only when its failure, reboot and rollback behavior has an executable acceptance path. Hardware-specific electrical behavior requires evidence from real hardware.
