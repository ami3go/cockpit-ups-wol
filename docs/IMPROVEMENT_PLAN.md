# cockpit-ups-wol — Improvement Plan and Extended Implementation Checklist

**Plan date:** 2026-09-19  
**Status:** Execution roadmap  
**Basis:** `SOFTWARE_ARCHITECTURE.md`, `INSTALLATION_REQUIREMENTS.md`, `RELIABILITY_REQUIREMENTS.md`, `BOOT_RECOVERY_REQUIREMENTS.md`, `RELATED_PROJECTS.md`, and `READINESS_AUDIT.md`

---

# 1. Purpose

This document converts the readiness audit into an actionable improvement plan.

The goal is to move `cockpit-ups-wol` through these stages:

```text
requirements defined
        ↓
requirements reconciled
        ↓
implementation-ready specification
        ↓
minimal safe implementation
        ↓
dry-run validated implementation
        ↓
armed shutdown/recovery implementation
        ↓
installer + Cockpit integration
        ↓
hardware-tested v0.1 pre-release
```

The project SHALL prioritize safety, deterministic behavior, restart recovery, rollback, and observable failure over feature breadth.

---

# 2. Current Position

The project currently has a strong requirements base but no implementation.

Strengths already established:

- NUT is the authoritative UPS backend.
- Cockpit is management/UI and is not safety-critical.
- Synology NAS is a first-class supported NUT client.
- Managed devices shut down in configured order.
- Controller shuts down last.
- Automatic recovery defaults to enabled.
- Recovery waits for utility stability and UPS recharge, default 80%.
- Recovery restores only previously-running hosts by default.
- `BOOT_RECONCILE` protects against interrupted boot and unstable power.
- Missing NUT data is `UNKNOWN`, not falsely healthy.
- Shutdown and recovery commit points are persisted before destructive actions.
- Configuration changes are transactional.
- Successful configurations become immutable known-good revisions.
- Failed configurations automatically roll back.
- Required services start automatically.
- Health supervision and bounded autofix are mandatory.
- Failure that cannot be made trustworthy enters `FAILED_SAFE`.

Major remaining work falls into three groups:

1. **Specification closure** — remove contradictions and make unresolved behavior explicit.
2. **Implementation** — build the agent, installer, Cockpit UI, persistence, health supervision, and adapters.
3. **Verification** — simulation, CI, hardware tests, interrupted-power tests, Synology tests, release validation.

---

# 3. Priority Definitions

## P0 — blocking

Must be completed before safety-critical implementation or before enabling real destructive automation.

## P1 — required for v0.1

Can proceed in parallel with early implementation but must be complete before a public/pre-release considered usable.

## P2 — post-v0.1 enhancement

Useful but not required for a safe initial release.

---

# 4. Gap Register

| ID | Priority | Gap | Risk if unresolved | Required deliverable |
|---|---|---|---|---|
| GAP-ARCH-01 | P0 | Baseline architecture is stale relative to newer reliability/boot requirements | Different components implement different state models | Updated `SOFTWARE_ARCHITECTURE.md` |
| GAP-GOV-01 | P0 | No document precedence/source-of-truth rule | Conflicting requirements can survive unnoticed | Normative document hierarchy |
| GAP-NUT-01 | P0 | NUT shutdown/FSD/output-off ownership unresolved | Competing shutdown actors or unsafe UPS behavior | `docs/NUT_SHUTDOWN_MODEL.md` |
| GAP-POLICY-01 | P0 | Trigger precedence/hysteresis not fully specified | Oscillation or ambiguous shutdown/recovery behavior | Canonical policy rules |
| GAP-CONFIG-01 | P0 | No authoritative config schema | Installer/UI/agent may diverge | Versioned schema + examples |
| GAP-STATE-01 | P0 | No authoritative persistent power-state schema | Restart recovery cannot be implemented deterministically | `docs/STATE_MODEL.md` + schema |
| GAP-IPC-01 | P0 | Cockpit/CLI ↔ agent interface unresolved | Unsafe direct file manipulation or privilege confusion | IPC/API contract |
| GAP-LANG-01 | P0 | Runtime technology stack not normative | Build/release structure cannot stabilize | Technology decision record |
| GAP-DEPLOY-01 | P0 | Controller UPS-backed power topology not yet canonical | Controller/network can disappear during outage | `docs/DEPLOYMENT.md` + architecture update |
| GAP-LICENSE-01 | P0 | Repository has no project license | Reuse/distribution unclear | `LICENSE` + reuse policy |
| GAP-TASK-01 | P0 | No implementation backlog/issues | Requirements can drift away from execution | GitHub issue backlog / roadmap |
| GAP-STORAGE-01 | P0 | SBC durability/wear policy incomplete | Frequent fsync + brownouts may damage storage or state | Storage durability requirements |
| GAP-INSTALL-01 | P0 | Installer spec predates LKG/health/boot-reconcile requirements | Installer may declare success without safe baseline | Updated installer requirements |
| GAP-TEST-01 | P0 | Test scenarios exist but no canonical executable matrix | Critical edge cases may be missed | `docs/TEST_PLAN.md` |
| GAP-SEC-01 | P1 | Security model is distributed, not canonical | Privilege/secrets/firewall inconsistencies | `docs/SECURITY.md` |
| GAP-SYN-01 | P1 | Synology operating procedure not isolated in one document | DSM setup errors and compatibility drift | `docs/SYNOLOGY.md` |
| GAP-NUT-02 | P1 | General NUT deployment/admin behavior not documented separately | Installer/operator ambiguity | `docs/NUT.md` |
| GAP-UPGRADE-01 | P1 | Config rollback defined; binary/package rollback not exact | Upgrade can leave software/config mismatch | Binary rollback design |
| GAP-HOST-01 | P1 | Host online/offline confirmation semantics incomplete | False shutdown/wake success | Host verification policy |
| GAP-MODE-01 | P1 | `monitor`, `dry-run`, `armed`, `maintenance` not fully formalized | Dangerous automation can be enabled accidentally | Operating mode matrix |
| GAP-NET-01 | P1 | Network dependency model incomplete | Recovery may fail when switches/routers need time or power-cycle | Dependency readiness model |
| GAP-IPV6-01 | P1 | Restricted mode IPv6 behavior unresolved | IPv4 restriction may be bypassed over IPv6 | Dual-stack restriction rules |
| GAP-SECRET-01 | P1 | Secret storage and redaction rules incomplete | Credential exposure | Secret-management design |
| GAP-CI-01 | P1 | No CI/build/release pipeline | Multi-arch release cannot be trusted | GitHub Actions workflows |
| GAP-DOC-01 | P1 | README, changelog, notices missing | Poor installation/reuse clarity | User-facing docs |

---

# 5. Target Runtime Architecture

The implementation should keep three independent state domains.

## 5.1 Power state

```text
BOOT_RECONCILE
NORMAL
ON_BATTERY
SHUTDOWN_COMMITTED
SHUTDOWN_IN_PROGRESS
WAITING_FOR_AC
RECOVERY_WAIT
RECOVERY_STARTED
RESTORE_HOSTS
```

`FAILED_SAFE` should be modeled as an inhibiting health/safety state rather than overloaded into every power transition.

## 5.2 Health/config state

```text
HEALTHY
DEGRADED
RECOVERING
CONFIG_VALIDATING
ROLLING_BACK
FAILED_SAFE
```

## 5.3 Operating mode

```text
MONITOR
DRY_RUN
ARMED
MAINTENANCE
```

Automatic destructive action is allowed only when all three domains permit it.

Example:

```text
power  = RECOVERY_WAIT
health = HEALTHY
mode   = ARMED
AC stable >= threshold
battery >= recovery threshold
network ready
        ↓
RECOVERY_STARTED
```

But:

```text
power  = RECOVERY_WAIT
health = FAILED_SAFE
mode   = ARMED
        ↓
NO AUTOMATIC WAKE
```

---

# 6. Workstream 0 — Specification Governance

**Priority:** P0

## Gap

Requirements are spread across several documents written at different times. Newer safety documents currently contain behavior not reflected in the original architecture.

## Work items

- Define which documents are normative.
- Define conflict precedence.
- Add version/status headers consistently.
- Cross-link architecture, config, state, NUT, reliability, boot, deployment, security, and test documents.
- Remove duplicated rules where duplication can drift.
- Keep normative values in one authoritative location where practical.

## Recommended precedence

```text
1. explicit safety/security requirements
2. state/config schemas
3. architecture
4. installation/deployment requirements
5. user documentation
6. related-project research/reference material
```

A simpler alternative is to state that all normative docs must be mutually consistent and any conflict blocks release.

## Deliverables

- Updated `SOFTWARE_ARCHITECTURE.md` with a "Normative documents" section.
- Document index in `README.md`.
- Consistent document versioning.

## Acceptance criteria

- [ ] No known contradictory requirements remain.
- [ ] Every major behavior has one canonical owner document.
- [ ] Reference/research docs cannot override normative design.
- [ ] CI later includes a lightweight documentation consistency check where practical.

---

# 7. Workstream 1 — Canonical Architecture Reconciliation

**Priority:** P0

## Gap

The baseline architecture still reflects the original simpler lifecycle.

## Work items

- Add `BOOT_RECONCILE`.
- Add persisted shutdown commit point.
- Add persisted recovery commit point.
- Add health supervisor.
- Add config transaction manager.
- Add LKG revision store.
- Add operating mode.
- Make required automatic service startup mandatory.
- Add controller UPS-backed power requirement.
- Add network-infrastructure dependency requirement.
- Define controller late-shutdown behavior.

## Deliverable

Updated `SOFTWARE_ARCHITECTURE.md` version 0.3 or later.

## Acceptance criteria

- [ ] Architecture contains every required power state.
- [ ] Architecture separates power state, health state, and operating mode.
- [ ] Boot never directly transitions to restore without NUT validation.
- [ ] Controller shutdown is explicitly last.
- [ ] Controller power source is explicitly battery-backed UPS output or equivalent.
- [ ] Cockpit is explicitly non-critical.
- [ ] State/config persistence relationships are shown.

---

# 8. Workstream 2 — NUT Shutdown Ownership and UPS Output Policy

**Priority:** P0 — highest safety priority

## Gap

It is not yet defined exactly which actor owns NUT FSD and final UPS output shutdown.

## Decisions required

Define:

- controller NUT role: primary/secondary/client/server
- which hosts use native NUT secondary behavior
- whether the agent requests FSD or observes NUT-driven FSD
- who invokes final UPS shutdown handling
- when `upsdrvctl shutdown` is appropriate
- policy for `shutdown.return`
- policy for `load.off`
- handling of UPSes that do not support output-off/return commands
- behavior when utility returns while shutdown is already committed
- relationship between Synology native NUT shutdown and agent orchestration

## Required safety principle

There SHALL be exactly one authority for each destructive action.

Example:

```text
NUT primary responsibility
    └─ NUT-coordinated shutdown/FSD

agent responsibility
    └─ policy/order/state/recovery

Synology secondary
    └─ reacts to NUT event and shuts itself down
```

The final model may differ, but ownership must never be ambiguous.

## Deliverable

`docs/NUT_SHUTDOWN_MODEL.md`

## Acceptance criteria

- [ ] One owner for FSD.
- [ ] One owner for UPS output-off/return behavior.
- [ ] Synology behavior documented.
- [ ] Non-NUT hosts documented.
- [ ] Power-return-before-commit behavior defined.
- [ ] Power-return-after-commit behavior defined.
- [ ] Unsupported UPS shutdown capabilities handled safely.
- [ ] Dangerous commands require explicit authorization/confirmation outside automatic NUT flow.

---

# 9. Workstream 3 — Power Policy, Trigger Precedence and Hysteresis

**Priority:** P0

## Gap

Triggers are identified, but precedence and hysteresis need exact semantics.

## Work items

Define policy evaluation for:

```text
OB
LB
FSD
battery.charge
battery.runtime
time on battery
NUT communication loss
UPS disconnected
```

Suggested model:

- `FSD` and explicit critical UPS state override ordinary timers.
- `LB` can trigger immediate or accelerated shutdown according to profile.
- percentage/runtime/time thresholds can be OR/AND policy inputs but must have deterministic precedence.
- communication loss does not become `OL`; it becomes `UNKNOWN` and inhibits recovery.
- recovery threshold (e.g. 80%) gates start of recovery.
- a small battery drop after recovery commit does not reverse recovery by itself.
- any renewed `OB` or unsafe UPS condition aborts/re-enters outage handling.

## Deliverables

- Policy section in `SOFTWARE_ARCHITECTURE.md`.
- Canonical fields in configuration schema.
- Tests in `docs/TEST_PLAN.md`.

## Acceptance criteria

- [ ] Every combination of OL/OB/LB/FSD/UNKNOWN has deterministic behavior.
- [ ] Pre-commit and post-commit power restoration differ explicitly.
- [ ] Recovery hysteresis prevents 80% ↔ 79% oscillation.
- [ ] Communication loss cannot trigger false recovery.

---

# 10. Workstream 4 — Canonical Configuration Schema

**Priority:** P0

## Gap

Example YAML exists, but no authoritative schema does.

## Recommended format

Human-editable YAML backed by a formal JSON Schema or equivalent validation model.

## Required top-level areas

```text
config_version
system
nut
outage
controller
recovery
health
operating_mode
hosts
```

## Host model should include

```text
id
name
address
status method
status parameters
shutdown method
shutdown priority
timeout/retry
wake enabled
MAC
interface
broadcast
wake priority
wake delay
restore policy
dependencies
```

## Deliverables

- `config/schema.json` or equivalent.
- `config/config.yaml.example`.
- `config/hosts.yaml.example` if hosts remain split.
- `docs/CONFIGURATION.md`.
- migration policy.

## Acceptance criteria

- [ ] Schema validates all examples.
- [ ] Unknown enum values fail cleanly.
- [ ] Unsafe ranges are rejected.
- [ ] Duplicate host IDs fail.
- [ ] Invalid MAC/IP/port values fail.
- [ ] Cross references/dependencies are validated.
- [ ] Config version migrations are testable.
- [ ] Secrets are referenced/stored safely rather than leaked into logs/history.

---

# 11. Workstream 5 — Persistent Power State Model

**Priority:** P0

## Gap

Power transaction semantics are strong, but storage structure is not canonical.

## Required state fields

At minimum:

```text
state_schema_version
transaction_id
sequence
power_state
shutdown_committed
recovery_started
active_config_revision
UPS observation metadata
per-host was_online
per-host shutdown state
per-host recovery state
retry counters
last action/result
checksum/integrity metadata
```

## Per-host shutdown state

```text
not_required
planned
requested
acknowledged
completed
unknown
failed
```

## Per-host recovery state

```text
not_required
waiting
wol_sent
starting
online
failed
```

## Durability requirements

- write temp
- flush
- fsync file
- atomic rename
- fsync parent directory
- retain previous valid generation
- verify checksum/version on load

## Storage wear requirements

State MUST be durable but SHALL avoid unnecessary high-frequency writes.

Recommended policy:

- persist state transitions, not every polling sample
- persist before/after destructive actions
- coalesce non-critical progress writes
- avoid using high-frequency health telemetry as persistent transaction state
- recommend eMMC/SSD/high-endurance microSD for production controllers

## Deliverables

- `docs/STATE_MODEL.md`.
- state schema definition.
- migration/version rules.
- corruption-recovery procedure.

## Acceptance criteria

- [ ] State survives agent process restart.
- [ ] State survives controller reboot.
- [ ] Torn newest generation can fall back to previous valid generation.
- [ ] No valid generation → `FAILED_SAFE` and no automatic wake.
- [ ] Duplicate/replayed actions are prevented or reconciled.
- [ ] Wall-clock correctness is not required.

---

# 12. Workstream 6 — Agent IPC, Authorization and Control Plane

**Priority:** P0

## Gap

Cockpit-to-agent communication is not defined.

## Recommended direction

Use a local Unix domain socket with a versioned request/response protocol unless D-Bus provides a clear Cockpit/polkit advantage during prototype evaluation.

## Required operations

Read-only:

```text
GetStatus
GetPowerState
GetHealth
GetHosts
GetConfigRevision
GetPlan
GetEvents/RecentActions
```

Controlled actions:

```text
ValidateConfig
ApplyConfigTransaction
SetOperatingMode
SetMaintenance
TriggerWake(host)
TriggerShutdown(host)
CancelPendingRecovery
AcknowledgeFailedSafe
RollbackConfig(revision)
RunHealthCheck
```

## Security requirements

- local-only IPC by default
- socket permissions restricted
- Cockpit privileged actions through appropriate authorization/polkit boundary
- no arbitrary shell command execution from request fields
- versioned protocol
- bounded request sizes
- auditable destructive calls

## Deliverables

- `docs/AGENT_API.md`.
- protocol definitions/types.
- privilege model in `docs/SECURITY.md`.

## Acceptance criteria

- [ ] UI never edits runtime state files directly.
- [ ] Read-only operations can be separated from privileged actions.
- [ ] Destructive requests are auditable.
- [ ] Invalid protocol requests cannot crash the agent.

---

# 13. Workstream 7 — Technology Stack Decision

**Priority:** P0

## Recommended stack

```text
agent             Go
wolctl            Go
health helper     Go or small shell wrapper around agent health API
Cockpit frontend  TypeScript + React + PatternFly
installer         Bash with distro modules
CI/build          GitHub Actions
config             YAML + JSON Schema
state              JSON or compact structured format with explicit schema version
```

## Rationale

Go is well suited to the target because it provides:

- static/self-contained binaries
- low deployment complexity
- amd64/arm64/riscv64 cross compilation
- low enough memory usage for SBC targets
- straightforward systemd watchdog integration
- good testing support
- reusable patterns from identified MIT Go projects

## Deliverable

Architecture decision record, for example `docs/ADR/0001-technology-stack.md`.

## Acceptance criteria

- [ ] Supported CPU targets compile in CI.
- [ ] Runtime requires no compiler/toolchain.
- [ ] Release artifacts can be checksum-verified.

---

# 14. Workstream 8 — Controller and Network Power Topology

**Priority:** P0

## Required deployment rule

The controller SHALL be powered from a **battery-backed UPS output**, unless it has another equally reliable backed power source.

It SHALL NOT be intentionally connected to a surge-only UPS outlet.

## Network rule

Any switch/router/VLAN path required to reach managed systems SHALL remain available long enough to complete shutdown coordination.

## Controller lifecycle

```text
utility failure
      ↓
controller remains alive
      ↓
managed systems shut down
      ↓
network-dependent operations finish
      ↓
controller persists state
      ↓
controller shuts down last if its late threshold is reached
```

## Decisions required

- controller shutdown threshold by LB / runtime / percentage
- whether controller can remain running indefinitely after large loads are off
- whether UPS output is intentionally turned off after controller shutdown

## Deliverables

- `docs/DEPLOYMENT.md`.
- architecture update.
- installer/TUI deployment warnings.

## Acceptance criteria

- [ ] Backed vs surge-only outlet requirement is explicit.
- [ ] SBC automatic boot-after-power-return is mandatory.
- [ ] Switch/router requirement documented.
- [ ] Late controller shutdown policy is configurable and safe.

---

# 15. Workstream 9 — Reliability, Health and Autofix Implementation

**Priority:** P0/P1

## Work items

Implement:

- systemd service restart policy
- optional systemd watchdog heartbeat
- oneshot health service + timer
- full-stack health checks
- bounded autofix
- exponential/backoff repair policy
- circuit breaker
- `FAILED_SAFE`
- restart/repair counters
- journald events

## Health checks

At minimum:

```text
required services enabled
required services active
agent heartbeat
config revision valid
state readable/writable
NUT command availability
upsd reachability
UPS query when expected
TCP 3493 when LAN NUT enabled
wolctl executable
required network interface available
Synology monitor config valid when enabled
```

## Deliverables

- health package/module
- `cockpit-ups-wol-health.service`
- `cockpit-ups-wol-health.timer`
- health API/CLI
- tests

## Acceptance criteria

- [ ] Killing agent triggers bounded restart.
- [ ] Temporarily unavailable UPS does not create false OL.
- [ ] Temporarily unavailable network does not create restart storm.
- [ ] Safe repair actions are logged.
- [ ] Failure limit transitions to `FAILED_SAFE`.

---

# 16. Workstream 10 — Transactional Configuration and LKG Store

**Priority:** P0

## Work items

Implement:

- config lock
- candidate revision creation
- schema validation
- cross-file validation
- preflight
- atomic activation
- targeted service reload/restart
- immediate health check
- probation
- known-good promotion
- automatic rollback
- previous-known-good fallback
- retention/pruning
- manual rollback

## Required revision states

```text
candidate
validating
known-good
failed
rolled-back
```

## Required pointers

```text
active
last-known-good
previous-known-good
```

## Deliverables

- config transaction package
- revision store
- manifest schema
- CLI/API methods
- tests

## Acceptance criteria

- [ ] Failed candidate never becomes LKG.
- [ ] Reboot during probation cannot promote candidate.
- [ ] Rollback itself must pass health validation.
- [ ] LKG survives package upgrade and power loss.
- [ ] Concurrent configuration changes are rejected/serialized.

---

# 17. Workstream 11 — Operating Modes

**Priority:** P1

## Required semantics

### MONITOR

- observe UPS/hosts
- no automatic shutdown
- no automatic wake
- manual operations may require explicit authorization

### DRY_RUN

- evaluate full policy
- persist simulation/audit plan
- log actions that would occur
- do not send shutdown or WoL actions

### ARMED

- full automatic shutdown/recovery allowed
- all safety gates active

### MAINTENANCE

- suppress automatic managed-host actions
- keep UPS monitoring and health supervision active
- clearly visible warning/status

## Recommended default

Fresh installation should start in **DRY_RUN** until explicitly armed.

## Deliverables

- operating mode section in config schema
- agent enforcement
- Cockpit control
- tests

## Acceptance criteria

- [ ] DRY_RUN produces same decisions as ARMED without destructive actions.
- [ ] MAINTENANCE survives UI disconnect/reload.
- [ ] Mode changes are logged and authorized.

---

# 18. Workstream 12 — Host Adapters and Verification

**Priority:** P1

## Shutdown methods

```text
nut
ssh
command
none
```

## Status methods

```text
auto
ping
tcp
arp
none
```

## Verification rule

Do not treat a single ping result as definitive.

Suggested configurable logic:

```text
shutdown success = N consecutive offline observations
wake success     = N consecutive online observations
```

## Work items

- generic status checker
- NUT-native shutdown adapter semantics
- SSH shutdown adapter
- safe command adapter
- retry/timeouts
- reconciliation after uncertain acknowledgement

## Deliverables

- host adapter packages
- tests/mocks
- examples

## Acceptance criteria

- [ ] Host failures do not automatically block unrelated hosts unless dependency policy requires it.
- [ ] `unknown` action is reconciled before repeating destructive commands.
- [ ] Previously-off hosts remain off under `previous-state`.

---

# 19. Workstream 13 — Wake-on-LAN

**Priority:** P1

## Work items

- magic packet construction
- MAC validation
- interface selection
- broadcast selection
- UDP port selection
- multi-subnet support
- retry/backoff
- post-wake verification
- dependency ordering

## Deliverables

- `wolctl`
- package tests
- CLI help

## Acceptance criteria

- [ ] Magic packet is exactly 6 × `FF` + MAC repeated 16 times.
- [ ] Invalid MACs fail before send.
- [ ] Interface/broadcast errors are explicit.
- [ ] Retry is bounded.
- [ ] Duplicate packet behavior is safe.

---

# 20. Workstream 14 — Synology Integration

**Priority:** P1

## Work items

- current DSM compatibility verification
- preset for UPS name `ups`
- NUT TCP port 3493
- monitor-only compatibility account
- current `upsmon secondary` syntax
- legacy fallback where required
- installer validation
- DSM setup guide
- outage test with real Synology

## Deliverables

- `docs/SYNOLOGY.md`
- installer preset
- integration test procedure

## Acceptance criteria

- [ ] Synology can query UPS server.
- [ ] Monitor user has no admin/SET/FSD permissions.
- [ ] DSM performs safe shutdown during configured outage condition.
- [ ] Agent does not duplicate SSH shutdown when method is `nut`.
- [ ] Synology can participate in recovery state without being incorrectly awakened when previously off.

---

# 21. Workstream 15 — Installer and Upgrade Framework

**Priority:** P0/P1

## Required installer modes

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

## Work items

- root/privilege handling
- OS detection
- package manager abstraction
- architecture detection
- RAM/storage check
- NUT installation/configuration
- Cockpit installation
- agent installation
- wolctl installation
- Cockpit extension installation
- systemd units
- health timer
- config revision store initialization
- initial LKG establishment
- idempotent re-run
- backup
- upgrade migration
- binary/config rollback
- final health validation

## Additional required checks

Installer SHOULD report/advice:

```text
controller is on UPS-backed power
controller auto-starts when power is applied
network infrastructure needed for shutdown is backed/resilient
persistent storage is suitable for crash-safe state
```

## Deliverables

- root `install.sh`
- `scripts/lib/distro/*.sh`
- installer tests
- uninstall/repair strategy

## Acceptance criteria

- [ ] Clean supported OS installs with no manual dependency installation.
- [ ] Required services enable/start automatically.
- [ ] Initial config becomes LKG only after probation.
- [ ] Failure rolls back project-owned changes where practical.
- [ ] Re-running is safe.
- [ ] Installer survives interrupted prior config transaction safely.

---

# 22. Workstream 16 — Cockpit UI

**Priority:** P1

## Pages

```text
Overview
UPS
Devices
Automation
Reliability
Settings
Logs
```

## Overview

Show:

- UPS state
- battery charge/runtime/load
- utility state
- power state
- health state
- operating mode
- active config revision
- LKG revision
- managed host states

## Automation

Show/edit:

- outage grace
- shutdown triggers
- priorities
- controller late-shutdown policy
- recovery threshold
- utility stable time
- network readiness
- wake order/delay
- restore policy

## Reliability

Show:

- health checks
- restart counts
- autofix events
- candidate/probation state
- rollback events
- config history
- manual rollback

## Deliverables

- Cockpit starter-kit based frontend
- agent API client
- PatternFly components
- privilege-aware actions

## Acceptance criteria

- [ ] UI failure does not impair power protection.
- [ ] UI cannot directly edit runtime state files.
- [ ] Dangerous actions require explicit confirmation/authorization.
- [ ] Config save visibly reports validation/probation/rollback result.

---

# 23. Workstream 17 — Security

**Priority:** P1

## Threat areas

- LAN exposure of NUT
- monitor/admin privilege separation
- Cockpit privileged actions
- SSH keys
- NUT credentials
- command injection
- arbitrary file modification
- restricted mode firewall gaps
- IPv6 bypass
- secrets in logs/config history
- malicious/invalid config

## Deliverables

- `docs/SECURITY.md`
- file permissions specification
- polkit/authorization design
- secret storage layout
- restricted-mode firewall rules

## Acceptance criteria

- [ ] Synology account monitor-only.
- [ ] Admin commands unavailable to monitor accounts.
- [ ] Shell arguments use safe argv invocation.
- [ ] Secrets redacted from logs and failed-revision diagnostics.
- [ ] Restricted mode handles both IPv4 and IPv6.
- [ ] No public Internet exposure is configured by default.

---

# 24. Workstream 18 — Simulation and Test Framework

**Priority:** P0/P1

## Required simulation events

```text
OL
OB
LB
FSD
UNKNOWN
power-restored
battery-threshold
network-delay
UPS disconnect/reconnect
host shutdown success/failure
host wake success/failure
controller reboot
state corruption
config rollback
```

Simulation SHALL not execute real shutdown or WoL unless an explicit destructive-test flag is enabled.

## Deliverables

- `docs/TEST_PLAN.md`
- mock NUT provider
- mock host adapters
- deterministic state-machine tests
- integration tests
- hardware acceptance procedure

## Acceptance criteria

- [ ] Every power-state transition is unit tested.
- [ ] Every interrupted-boot scenario is tested.
- [ ] Every config transaction state is tested.
- [ ] Real hardware destructive tests are isolated and explicit.

---

# 25. Workstream 19 — CI, Packaging and Release Engineering

**Priority:** P1

## Work items

- Go unit tests
- frontend tests/build
- shell lint/tests
- schema validation
- multi-arch builds
- amd64
- arm64
- riscv64
- checksum generation
- release artifact assembly
- dependency/license check
- reusable simulation test job
- optional QEMU smoke tests

## Deliverables

```text
.github/workflows/test.yml
.github/workflows/build.yml
.github/workflows/release.yml
SHA256SUMS
```

## Acceptance criteria

- [ ] PR/main CI passes before release.
- [ ] All supported architectures build.
- [ ] Release installer verifies checksum.
- [ ] No release is marked successful if critical simulation suite fails.

---

# 26. Workstream 20 — Licensing and Third-Party Reuse

**Priority:** P0 before direct code reuse

## Gap

Repository currently has no license.

## Work items

- choose project license
- verify compatibility with selected Cockpit frontend foundation and reused snippets
- create attribution policy
- document copied/adapted source precisely

## Deliverables

```text
LICENSE
THIRD_PARTY_NOTICES.md
```

## Acceptance criteria

- [ ] Project license exists before public code release.
- [ ] Every directly reused source file/function has source + license attribution where required.
- [ ] GPL reference code is not copied into a differently licensed implementation unless intentionally compatible.

---

# 27. Milestone Plan

## M0 — Specification Closure

Goal: implementation-ready design.

Must complete:

- architecture reconciliation
- NUT shutdown ownership
- policy/hysteresis rules
- config schema
- state schema
- IPC decision
- technology stack decision
- deployment/power topology
- installer reconciliation
- test plan structure
- license decision

**Exit criterion:** an engineer can implement the core agent without inventing safety behavior.

---

## M1 — Repository and Safe Core Skeleton

Goal: compiling/testable project skeleton.

Deliver:

- Go module
- agent process skeleton
- wolctl skeleton
- config parser/schema validation
- state-store package
- logging
- systemd units
- initial CI
- simulation harness

**Exit criterion:** supported architectures compile and core unit tests run.

---

## M2 — Dry-Run Power Engine

Goal: full decision engine without destructive actions.

Deliver:

- NUT adapter
- complete power state machine
- persisted transaction state
- host inventory/status
- recovery gates
- dry-run plan output
- boot reconcile
- UNKNOWN fail-safe behavior

**Exit criterion:** simulated outage/recovery suite passes without real shutdown/WoL.

---

## M3 — Armed Shutdown Path

Goal: safe real shutdown.

Deliver:

- operating modes
- host shutdown adapters
- NUT/Synology shutdown model
- shutdown commit point
- per-host progress persistence
- controller-last logic

**Exit criterion:** hardware test shows ordered shutdown and safe controller-last behavior.

---

## M4 — Automatic Recovery Path

Goal: safe unattended restore.

Deliver:

- AC stability timer
- battery >= threshold gate
- network readiness
- recovery commit point
- wolctl
- ordered host restore
- interrupted recovery reconciliation

**Exit criterion:** repeated boot/power-bounce tests never cause premature wake and stable recovery restores only eligible hosts.

---

## M5 — Reliability + Installer + Cockpit

Goal: usable appliance.

Deliver:

- health supervisor
- autofix/circuit breaker
- config transactions/LKG rollback
- installer/TUI/silent mode
- Cockpit pages
- Synology preset
- upgrade path

**Exit criterion:** clean OS install produces a self-starting, health-checked, rollback-capable appliance.

---

## M6 — v0.1 Pre-release

Goal: reproducible tested release.

Deliver:

- full test matrix
- amd64/arm64/riscv64 artifacts
- checksums
- README
- security docs
- deployment docs
- Synology docs
- release notes

**Exit criterion:** release acceptance matrix passes on at least representative x86_64 and ARM64 hardware, with RISC-V build/smoke validation and documented limitations.

---

# 28. Dependency Order

Critical dependency graph:

```text
Architecture reconciliation
        ↓
NUT shutdown model ───────┐
Config schema ────────────┤
State schema ─────────────┤
IPC/API ──────────────────┤
Technology decision ──────┤
Deployment model ─────────┘
        ↓
Agent skeleton + state store
        ↓
NUT adapter + dry-run state machine
        ↓
Shutdown path
        ↓
Recovery/WoL path
        ↓
Health + config transactions
        ↓
Installer + Cockpit
        ↓
Hardware acceptance + release
```

Do not implement real destructive shutdown before NUT ownership, state schema, and commit-point persistence are finalized.

---

# 29. Extended Checklist

## A. Governance and architecture

- [ ] Define normative document precedence.
- [ ] Update architecture version.
- [ ] Add `BOOT_RECONCILE`.
- [ ] Add `SHUTDOWN_COMMITTED`.
- [ ] Add recovery commit semantics.
- [ ] Add health state domain.
- [ ] Add operating mode domain.
- [ ] Add config transaction subsystem.
- [ ] Add health supervisor.
- [ ] Add LKG store.
- [ ] Make service autostart mandatory.
- [ ] Make controller UPS-backed power mandatory.
- [ ] Document switch/router power dependency.
- [ ] Define controller late-shutdown policy.
- [ ] Define unsupported UPS capability behavior.

## B. NUT safety model

- [ ] Decide controller primary/secondary role.
- [ ] Define FSD owner.
- [ ] Define final UPS shutdown owner.
- [ ] Define `upsdrvctl shutdown` use/non-use.
- [ ] Define `shutdown.return` policy.
- [ ] Define `load.off` policy.
- [ ] Define return-before-commit behavior.
- [ ] Define return-after-commit behavior.
- [ ] Define Synology NUT role.
- [ ] Define other NUT client roles.
- [ ] Define remote NUT server mode.
- [ ] Define NUT communication-loss behavior.
- [ ] Ensure UNKNOWN never becomes OL implicitly.

## C. Trigger/recovery policy

- [ ] Define OB behavior.
- [ ] Define LB behavior.
- [ ] Define FSD behavior.
- [ ] Define battery percentage trigger.
- [ ] Define battery runtime trigger.
- [ ] Define time-on-battery trigger.
- [ ] Define trigger precedence.
- [ ] Define grace period semantics.
- [ ] Define recovery threshold semantics.
- [ ] Define 80→79 hysteresis behavior.
- [ ] Define renewed outage during recovery.
- [ ] Define network timeout behavior.
- [ ] Define missing `battery.charge` fallback.
- [ ] Define missing `battery.runtime` fallback.
- [ ] Define manual recovery fallback.

## D. Configuration

- [ ] Choose one canonical config layout.
- [ ] Add `config_version`.
- [ ] Define system section.
- [ ] Define NUT section.
- [ ] Define network policy section.
- [ ] Define Synology section.
- [ ] Define outage section.
- [ ] Define controller section.
- [ ] Define recovery section.
- [ ] Define health section.
- [ ] Define operating mode.
- [ ] Define host schema.
- [ ] Define status schema.
- [ ] Define shutdown schema.
- [ ] Define WoL schema.
- [ ] Define dependency schema.
- [ ] Define restore policy.
- [ ] Define schema ranges/defaults.
- [ ] Define migration policy.
- [ ] Provide valid examples.
- [ ] Provide invalid-schema tests.

## E. Persistent power state

- [ ] Define state schema version.
- [ ] Define transaction ID.
- [ ] Define sequence counter.
- [ ] Define power state enum.
- [ ] Define shutdown commit flag/state.
- [ ] Define recovery commit flag/state.
- [ ] Store active config revision.
- [ ] Store per-host pre-outage state.
- [ ] Store shutdown action state.
- [ ] Store recovery action state.
- [ ] Store retry counters.
- [ ] Define checksum/integrity field.
- [ ] Define previous generation retention.
- [ ] Define corruption fallback.
- [ ] Define no-valid-state behavior.
- [ ] Define state migration.
- [ ] Avoid persisting every poll.
- [ ] Test fsync/rename path.

## F. Agent runtime

- [ ] Create Go module.
- [ ] Define package layout.
- [ ] Implement structured logging.
- [ ] Implement signal handling.
- [ ] Implement graceful shutdown.
- [ ] Implement config loading.
- [ ] Implement state loading.
- [ ] Implement `BOOT_RECONCILE`.
- [ ] Implement monotonic timers.
- [ ] Implement power-state engine.
- [ ] Implement health-state integration.
- [ ] Implement operating-mode enforcement.
- [ ] Implement state persistence before destructive actions.
- [ ] Implement idempotent/reconciled action behavior.
- [ ] Implement systemd watchdog support.

## G. IPC/API

- [ ] Select Unix socket vs D-Bus.
- [ ] Version protocol.
- [ ] Define status response.
- [ ] Define health response.
- [ ] Define plan/dry-run response.
- [ ] Define config transaction calls.
- [ ] Define operating mode calls.
- [ ] Define manual wake call.
- [ ] Define manual shutdown call.
- [ ] Define cancel recovery call.
- [ ] Define rollback call.
- [ ] Define authorization boundaries.
- [ ] Define socket ownership/permissions.
- [ ] Reject malformed/oversized requests.

## H. Health/autofix

- [ ] Add health service.
- [ ] Add health timer.
- [ ] Check service enabled state.
- [ ] Check service active state.
- [ ] Check watchdog heartbeat.
- [ ] Check NUT binaries.
- [ ] Check upsd.
- [ ] Check UPS query.
- [ ] Check TCP 3493 when enabled.
- [ ] Check config hash/schema.
- [ ] Check LKG pointers.
- [ ] Check state store.
- [ ] Check state write ability.
- [ ] Check wolctl.
- [ ] Check network interface.
- [ ] Add safe service restart repair.
- [ ] Add safe re-enable repair.
- [ ] Add permission repair for project files.
- [ ] Add LKG restore repair.
- [ ] Add bounded retry/backoff.
- [ ] Add circuit breaker.
- [ ] Implement `FAILED_SAFE`.

## I. Config transaction/LKG

- [ ] Implement transaction lock.
- [ ] Create candidate.
- [ ] Static validation.
- [ ] Cross-file validation.
- [ ] Component preflight.
- [ ] Atomic activation.
- [ ] Targeted service reload/restart.
- [ ] Immediate health validation.
- [ ] Probation timer.
- [ ] Promote to known-good.
- [ ] Update active/LKG pointers atomically.
- [ ] Roll back failed candidate.
- [ ] Validate rollback.
- [ ] Fall back to previous LKG if configured.
- [ ] Retain diagnostics for failed revision.
- [ ] Prune old revisions safely.
- [ ] Reconcile interrupted validation after reboot.

## J. WoL

- [ ] Implement MAC parser.
- [ ] Implement magic packet.
- [ ] Unit test packet length/content.
- [ ] Interface selection.
- [ ] Broadcast selection.
- [ ] Port configuration.
- [ ] Multi-subnet support.
- [ ] Retry/backoff.
- [ ] Host online verification.
- [ ] Dependency wait.
- [ ] Prevent wake of previous-state=false host.

## K. Host shutdown/status

- [ ] Generic status interface.
- [ ] Ping status.
- [ ] TCP status.
- [ ] ARP status if practical.
- [ ] N consecutive checks policy.
- [ ] NUT shutdown method.
- [ ] SSH shutdown method.
- [ ] Safe command method.
- [ ] None/observe-only method.
- [ ] Per-host timeout.
- [ ] Per-host retry.
- [ ] Per-host priority.
- [ ] Shutdown completion verification.
- [ ] Unknown acknowledgement reconciliation.

## L. Synology

- [ ] Verify current DSM behavior.
- [ ] Configure UPS name `ups`.
- [ ] Configure port 3493.
- [ ] Configure monitor account.
- [ ] Ensure account is monitor-only.
- [ ] Support `upsmon secondary`.
- [ ] Support legacy syntax only where required.
- [ ] Validate Synology preset in installer.
- [ ] Write DSM setup instructions.
- [ ] Test real shutdown.
- [ ] Test reconnect after power return.

## M. Installer

- [ ] Root `install.sh`.
- [ ] Default mode.
- [ ] TUI mode.
- [ ] Silent mode.
- [ ] Invalid option combination handling.
- [ ] Persistent installer log.
- [ ] OS detection.
- [ ] package-manager detection.
- [ ] architecture normalization.
- [ ] RAM/storage check.
- [ ] Cockpit install.
- [ ] NUT install.
- [ ] agent install.
- [ ] wolctl install.
- [ ] frontend install.
- [ ] systemd unit install.
- [ ] health timer install.
- [ ] auto-enable required units.
- [ ] UPS detection.
- [ ] ambiguous UPS handling.
- [ ] Synology option.
- [ ] trusted-LAN option.
- [ ] restricted mode.
- [ ] preserve existing NUT config.
- [ ] backup project config.
- [ ] initialize revision store.
- [ ] establish first LKG.
- [ ] rollback on failure.
- [ ] idempotent rerun.
- [ ] report UPS-backed controller requirement.
- [ ] report auto-power-on requirement.
- [ ] report stable address recommendation.

## N. Cockpit UI

- [ ] Starter-kit integration.
- [ ] Overview page.
- [ ] UPS page.
- [ ] Devices page.
- [ ] Automation page.
- [ ] Reliability page.
- [ ] Settings page.
- [ ] Logs page.
- [ ] Operating mode control.
- [ ] Maintenance control.
- [ ] Dry-run plan preview.
- [ ] Config validation progress.
- [ ] Probation progress.
- [ ] Rollback result display.
- [ ] Active/LKG revision display.
- [ ] Manual rollback UI.
- [ ] Dangerous action confirmation.
- [ ] Privilege authorization.

## O. Security

- [ ] Threat model.
- [ ] NUT monitor/admin split.
- [ ] Cockpit privilege boundary.
- [ ] Secret file permissions.
- [ ] SSH key permissions.
- [ ] Secret redaction.
- [ ] No arbitrary shell interpolation.
- [ ] Trusted-LAN warning.
- [ ] Restricted IPv4 policy.
- [ ] Restricted IPv6 policy.
- [ ] Firewall change rollback.
- [ ] Local IPC permissions.
- [ ] Command audit logging.

## P. Storage/power-loss durability

- [ ] Recommend eMMC/SSD/high-endurance microSD.
- [ ] Document filesystem assumptions.
- [ ] Minimize write amplification.
- [ ] Persist only meaningful state transitions.
- [ ] Retain previous valid generation.
- [ ] Test corrupted newest generation.
- [ ] Test unexpected read-only filesystem.
- [ ] Test repeated brownout boot cycles.
- [ ] Test no RTC/time synchronization.

## Q. Tests

- [ ] OL normal operation.
- [ ] Short outage below grace.
- [ ] Long outage.
- [ ] LB event.
- [ ] FSD event.
- [ ] NUT communication loss.
- [ ] USB disconnect/reconnect.
- [ ] Power restoration before shutdown commit.
- [ ] Restoration after shutdown commit.
- [ ] Power failure during boot.
- [ ] Repeated interrupted boot.
- [ ] Boot while OB.
- [ ] Boot while OL but battery <80%.
- [ ] Failure during stable-AC timer.
- [ ] Reboot during stable-AC timer.
- [ ] Power loss during shutdown.
- [ ] Power loss after shutdown commit.
- [ ] Power loss during recovery.
- [ ] Power loss after recovery commit.
- [ ] One host restored, then outage.
- [ ] Previously-off host stays off.
- [ ] Missing battery charge.
- [ ] Missing battery runtime.
- [ ] Network delayed.
- [ ] Host shutdown failure.
- [ ] Host wake failure.
- [ ] Corrupt newest state.
- [ ] All state corrupt.
- [ ] Invalid candidate config.
- [ ] Candidate causes service failure.
- [ ] Candidate fails probation.
- [ ] Reboot during candidate validation.
- [ ] Rollback succeeds.
- [ ] Rollback fails → FAILED_SAFE.
- [ ] Systemd restart loop breaker.
- [ ] Synology integration.

## R. CI/release

- [ ] Go formatting/vet/tests.
- [ ] frontend lint/test/build.
- [ ] shell lint/tests.
- [ ] schema validation.
- [ ] docs link/basic validation.
- [ ] amd64 build.
- [ ] arm64 build.
- [ ] riscv64 build.
- [ ] simulation acceptance job.
- [ ] release artifact packaging.
- [ ] SHA256SUMS.
- [ ] installer checksum verification.
- [ ] release notes.
- [ ] versioning policy.

## S. Documentation/repository hygiene

- [ ] `README.md`.
- [ ] `LICENSE`.
- [ ] `CHANGELOG.md`.
- [ ] `THIRD_PARTY_NOTICES.md`.
- [ ] `docs/CONFIGURATION.md`.
- [ ] `docs/STATE_MODEL.md`.
- [ ] `docs/NUT_SHUTDOWN_MODEL.md`.
- [ ] `docs/NUT.md`.
- [ ] `docs/SYNOLOGY.md`.
- [ ] `docs/SECURITY.md`.
- [ ] `docs/DEPLOYMENT.md`.
- [ ] `docs/AGENT_API.md`.
- [ ] `docs/TEST_PLAN.md`.
- [ ] GitHub issue backlog.
- [ ] release milestone structure.

---

# 30. Definition of Implementation-Ready

The project is implementation-ready when all of these are true:

```text
✓ architecture reconciled with reliability and boot requirements
✓ NUT FSD/output-off ownership defined
✓ shutdown/recovery trigger precedence defined
✓ canonical config schema exists
✓ canonical state schema exists
✓ IPC/API decision exists
✓ implementation stack is fixed
✓ controller/network UPS power topology is normative
✓ installer requirements include health/LKG/boot recovery
✓ test plan maps each critical safety requirement to a test
✓ repository license is selected
```

At that point implementation can proceed without inventing safety behavior.

---

# 31. Definition of v0.1 Pre-release Ready

A v0.1 pre-release is ready only when:

```text
✓ clean installer works on at least one Tier-1 distro
✓ required services auto-start after reboot
✓ dry-run and armed modes work
✓ NUT local server works
✓ Synology compatibility is validated
✓ ordered shutdown works
✓ controller shuts down last
✓ outage transaction survives reboot
✓ interrupted boot never triggers premature wake
✓ AC stable timer works
✓ battery >=80% default recovery gate works
✓ network readiness gate works
✓ ordered WoL recovery works
✓ previously-off devices remain off
✓ health supervisor detects and repairs bounded faults
✓ failed repair enters FAILED_SAFE
✓ config revisions become known-good only after probation
✓ failed config automatically rolls back
✓ LKG survives reboot and upgrade
✓ amd64/arm64/riscv64 release artifacts build
✓ critical simulation suite passes
✓ hardware acceptance suite passes on representative hardware
✓ documentation, license, notices and checksums are present
```

---

# 32. Recommended Immediate Next Work

The highest-value next sequence is:

```text
1. Reconcile SOFTWARE_ARCHITECTURE.md
2. Write NUT_SHUTDOWN_MODEL.md
3. Write CONFIGURATION.md + schema
4. Write STATE_MODEL.md + schema
5. Decide agent IPC/API
6. Fix technology stack as Go/TS/Bash or document alternative
7. Write DEPLOYMENT.md
8. Synchronize INSTALLATION_REQUIREMENTS.md
9. Write TEST_PLAN.md
10. Choose LICENSE
11. Convert this plan into GitHub issues/milestones
12. Start M1 repository/build skeleton
```

The project should avoid adding broad new features until these items are closed. The existing concept is already sufficiently broad; the main risk now is specification drift rather than missing functionality.
