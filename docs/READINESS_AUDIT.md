# cockpit-ups-wol — Readiness Audit

**Audit date:** 2026-09-19  
**Status:** Pre-implementation readiness review  
**Scope:** architecture, installation, reliability, interrupted-boot recovery, related-project research, implementation tasks, testing, release readiness and operational safety

## 1. Executive verdict

The project has a strong requirements foundation, but it is **not implementation-ready without a short reconciliation pass** and is **not release-ready**.

Current state:

| Area | Readiness | Assessment |
|---|---:|---|
| Product / functional requirements | ~85% | Strong; core behavior is well defined |
| Safety / recovery requirements | ~90% | Strong; reliability and interrupted-boot behavior are unusually well specified |
| Architecture consistency | ~65% | Baseline architecture predates newer normative reliability/boot requirements |
| Installation specification | ~75% | Strong baseline, but missing several newer mandatory reliability requirements |
| Configuration model | ~55% | Concepts are defined; canonical schema and transaction/state formats are not |
| Security model | ~60% | Good principles; dedicated security specification is still missing |
| Test specification | ~45% | Many scenarios are named; no complete executable acceptance-test matrix exists |
| Implementation | ~0% | Repository currently contains documentation only |
| CI/release engineering | ~0% | No workflows, build, packaging or release artifacts yet |
| Deployment/release readiness | ~10% | Hardware/deployment model exists, but no installer or tested artifact exists |

These percentages are directional audit indicators, not mathematical quality scores.

**Overall conclusion:** requirements are mature enough to begin implementation **after the P0 specification blockers below are resolved and the canonical documents are synchronized**.

---

## 2. Repository inventory

Current repository content is documentation-only:

```text
SOFTWARE_ARCHITECTURE.md

docs/
├── BOOT_RECOVERY_REQUIREMENTS.md
├── INSTALLATION_REQUIREMENTS.md
├── RELATED_PROJECTS.md
├── RELIABILITY_REQUIREMENTS.md
└── READINESS_AUDIT.md
```

There is currently no:

```text
README.md
LICENSE
CHANGELOG.md
SECURITY.md
source code
installer
systemd units
configuration examples
schema files
tests
CI workflow
release workflow
GitHub issue/task backlog
```

This is acceptable for the design phase, but means implementation and release readiness are currently near zero.

---

# 3. Document audit

## 3.1 SOFTWARE_ARCHITECTURE.md

### Status

**Good foundation, but stale relative to the newer normative documents.**

### Strong areas

- NUT remains the UPS backend rather than reimplementing UPS drivers.
- Cockpit is management/UI only and is not on the safety-critical path.
- Persistent agent is a core component.
- Synology is a first-class NUT client.
- Ordered shutdown and ordered recovery are defined.
- Controller-last shutdown is defined.
- 80% default recovery threshold is defined.
- Trusted-LAN default and optional restricted NUT access are defined.
- Multi-interface/VLAN WoL is considered.

### Required synchronization

The architecture state machine does not yet include all states/concepts defined later in normative requirements. It SHALL be updated to include or explicitly reference:

```text
BOOT_RECONCILE
SHUTDOWN_COMMITTED
RECOVERY_STARTED / recovery commit point
FAILED_SAFE
configuration validation / rollback states
health supervisor
dry-run / armed operating mode
```

The architecture currently describes restart policy as a recommendation. The reliability document makes automatic service startup/recovery a core requirement; architecture wording should be made consistent.

The architecture also needs the newly established deployment rule:

> The controller SBC SHALL be powered from a battery-backed UPS output, unless it has an equivalent independent backed power source.

It should also state that network infrastructure required for control traffic, especially the relevant Ethernet switch/router/VLAN path, must remain available long enough for shutdown coordination.

### Readiness

**PARTIAL — update before coding the agent state machine.**

---

## 3.2 INSTALLATION_REQUIREMENTS.md

### Status

**Strong installer baseline, but predates the full reliability/config-transaction model.**

### Strong areas

- single `install.sh` entry point
- `--tui` and `--silent`
- clean-OS installation
- multi-distro abstraction
- amd64 / arm64 / riscv64 targets
- Synology compatibility
- trusted-LAN default
- optional NUT network restrictions
- preservation of existing NUT configuration
- upgrade backup/rollback principle
- release checksum verification
- installation validation
- idempotency

### Missing or stale requirements

Installation requirements SHALL be synchronized with `RELIABILITY_REQUIREMENTS.md` and `BOOT_RECOVERY_REQUIREMENTS.md` to explicitly require:

- installation and enablement of `cockpit-ups-wol-health.timer`
- automatic startup of every selected required service
- creation of the initial immutable configuration revision
- initial `active`, `last-known-good`, and `previous-known-good` metadata
- probation/health validation before the initial config is considered known-good
- rollback if post-install runtime validation fails
- interrupted-install/config-transaction recovery after sudden power loss
- `BOOT_RECONCILE` behavior after reboot
- controller hardware auto-power-on requirement
- controller powered from UPS-backed output deployment check/warning
- dry-run default for a newly installed automation policy, if adopted as the final policy

The installer should report whether it can verify:

```text
controller auto-boots after AC return
controller is connected to backed power (usually advisory/manual confirmation)
network path required for managed devices is UPS-backed or otherwise resilient
```

### Readiness

**PARTIAL — suitable as a base, but update before implementing `install.sh`.**

---

## 3.3 RELIABILITY_REQUIREMENTS.md

### Status

**Strong and implementation-oriented.**

### Strong areas

- automatic service startup
- service restart/backoff
- independent health supervisor
- bounded autofix
- health state model
- transactional configuration
- immutable known-good revisions
- automatic rollback
- reboot-safe config validation state
- retention policy
- manual rollback
- config locking
- upgrade transaction
- FAILED_SAFE behavior

### Remaining design decisions

Before implementation, define exact values/interfaces for:

- health check API/exit-code contract
- watchdog heartbeat mechanism
- circuit-breaker persistence across reboot
- which repair counters reset on successful operation and after how long
- exact revision manifest schema
- whether config history uses directories + pointer files, symlinks, or a small metadata DB
- atomic locking mechanism (`flock`, lock file + PID/start-time, or agent-owned IPC transaction)

### Readiness

**HIGH — can guide implementation after the canonical schemas/interfaces are selected.**

---

## 3.4 BOOT_RECOVERY_REQUIREMENTS.md

### Status

**Strong and safety-oriented.**

### Strong areas

- explicit `BOOT_RECONCILE`
- boot is never proof of restored utility
- NUT `UNKNOWN` is fail-safe
- repeated interrupted boots
- AC stability timer resets after uncontrolled reboot
- durable outage transaction
- atomic power-loss-safe state writes
- config transaction interruption handling
- service startup ordering
- recovery health gate
- power loss during restore
- power loss during shutdown
- shutdown and recovery commit points
- wall-clock independence
- controller auto-power-on requirement
- corrupt-state behavior

### Remaining design decisions

- exact persistent transaction/state schema
- state-file generation naming/versioning
- exact checksum/hash and corruption-detection approach
- maximum retained state generations
- exact reconciliation algorithm for `requested` / `acknowledged` / `unknown` host actions
- behavior when the controller repeatedly boots with insufficient UPS battery but valid `OL`
- whether the controller may remain running indefinitely after host shutdown or should have a configurable late shutdown threshold

### Readiness

**HIGH — ready to drive implementation once the state schema is finalized.**

---

## 3.5 RELATED_PROJECTS.md

### Status

**Good research/reference document.**

The deep search now covers useful references for:

- recovery state: WOLNUT
- deterministic shutdown: Nutcracker
- orchestration/testing: Eneru
- complete shutdown/recovery lifecycle: HyperCore Power Manager
- Proxmox: PVE-UPS
- Cockpit/NUT: Cockpit UPSide
- NUT configuration/USB/WoL: NutWatch
- Go fleet installer: homelab-nut
- Go orchestration/persistence: riofutab/nut-server
- Synology: synology-ecoflow-nut
- WoL: Trugamr/wol
- native Go NUT client: NutShell

### Remaining action

Before copying any source code, create `THIRD_PARTY_NOTICES.md` and document exact reused files/functions and licenses.

### Readiness

**HIGH — no blocking research gap currently identified.**

---

# 4. P0 blockers before core implementation

These should be resolved before implementing the power agent beyond a prototype.

## P0.1 Synchronize the canonical architecture

Update `SOFTWARE_ARCHITECTURE.md` so the main state machine and component model include the newer normative requirements.

Required concepts:

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
FAILED_SAFE
```

Also include service health/config rollback relationships.

---

## P0.2 Define exact NUT shutdown ownership

This is the most important remaining power-control ambiguity.

Define exactly:

- whether the controller is the NUT primary
- how `upsmon` primary/secondary roles are used
- who initiates NUT FSD
- who is allowed to invoke UPS shutdown commands
- when/if `upsdrvctl shutdown` or equivalent is used
- how `shutdown.return`, `load.off`, or vendor-specific commands are handled
- whether UPS output is intentionally switched off after controller shutdown
- how this interacts with Synology and other NUT secondary clients

The project must not have two independent actors competing to shut down the UPS/load.

---

## P0.3 Finalize shutdown trigger precedence and hysteresis

Define exact semantics for combinations of:

```text
time on battery
battery percentage
battery runtime
LB
FSD
communication loss
```

Need explicit precedence and fail-safe behavior.

Also define:

- power returns before commit
- power returns after `SHUTDOWN_COMMITTED`
- power fails during `RECOVERY_WAIT`
- power fails during `RESTORE_HOSTS`
- battery reaches 80%, recovery starts, then falls to 79% because load was restored

Recommended principle: battery threshold gates the **start** of recovery; after recovery has committed, a small percentage drop alone does not reverse recovery unless a real outage/unsafe state occurs.

---

## P0.4 Define canonical configuration schema

Create a versioned schema covering at least:

```text
NUT connection/profile
NUT network mode
Synology compatibility
outage trigger policy
controller policy
recovery policy
host status method
shutdown method/order/timeout
wake order/retry/broadcast/interface
restore policy
maintenance/dry-run/armed mode
health/autofix policy
```

Prefer one authoritative schema definition plus example YAML.

---

## P0.5 Define canonical persistent state schema

Formalize the state described in boot-recovery requirements:

```text
transaction_id
schema/state version
sequence
power state
shutdown committed flag
recovery committed flag
active config revision
per-host pre-outage state
per-host shutdown state
per-host recovery state
retry counters
last trustworthy UPS observations where appropriate
```

State schema must explicitly support migrations.

---

## P0.6 Decide Cockpit ↔ agent control interface

Cockpit SHALL NOT directly manipulate agent runtime state files.

Select one interface:

```text
Unix domain socket
D-Bus
small privileged helper + CLI protocol
```

Required operations likely include:

```text
GetStatus
GetHealth
ReloadConfig
BeginConfigTransaction
SetMaintenance
SetOperatingMode
TriggerWake(host)
TriggerShutdown(host)
CancelPendingRecovery
AcknowledgeFailure
```

Authorization/polkit boundaries should be defined at the same time.

---

## P0.7 Select implementation language/runtime

The design currently leans toward Go for the agent and `wolctl`, but this is not yet a normative decision.

Recommendation:

```text
agent      Go
wolctl     Go
Cockpit UI TypeScript/React/PatternFly
installer  POSIX/Bash shell modules
```

Benefits: small static binaries, straightforward amd64/arm64/riscv64 releases, no Python runtime dependency on target systems, and useful MIT Go references already identified.

---

## P0.8 Make controller power topology normative

Add to architecture/deployment requirements:

- controller SBC is powered from a battery-backed UPS output
- controller is never intentionally connected to surge-only output
- controller automatically boots when backed power returns
- required switch/router/VLAN path remains powered long enough for shutdown coordination
- managed heavy loads shut down before the controller

---

## P0.9 Add an implementation task/roadmap tracker

There are currently no GitHub issues or canonical task list.

Create a roadmap/backlog so requirements do not become disconnected from implementation.

Suggested v0.1 epics:

```text
repository/build skeleton
config + state schemas
Go agent foundation
NUT adapter
state machine
transactional configuration manager
health supervisor
wolctl
host adapters/status checks
Synology preset
installer
Cockpit UI
simulation framework
acceptance tests
CI/release packaging
```

---

# 5. P1 design tasks

These are important but can proceed alongside early implementation.

## P1.1 Maintenance and operating modes

Formalize:

```text
monitor
dry-run
armed
maintenance
```

Define which automatic actions each mode permits.

New installations should strongly consider starting in `dry-run` until the user explicitly arms automation.

---

## P1.2 Host shutdown/wake confirmation rules

Define exact verification semantics, for example:

```text
shutdown success = host fails configured status check N consecutive times
wake success     = host passes configured status check N consecutive times
```

Make N, interval and timeout configurable.

Avoid relying on a single ping result.

---

## P1.3 Network-infrastructure dependency model

Document devices such as switches/routers that may:

- stay powered for the entire outage
- power-cycle automatically with UPS output
- not support WoL

Recovery dependencies should allow "wait until reachable" without assuming every dependency can be awakened by the controller.

---

## P1.4 IPv6 restricted-access semantics

Restricted NUT mode must either:

- configure restrictions for both IPv4 and IPv6, or
- intentionally disable the unprotected address family.

An IPv4-only firewall restriction must not leave an unrestricted IPv6 listener.

---

## P1.5 Secret management

Define storage and permissions for:

- SSH keys
- NUT admin credentials
- Synology compatibility credential
- future API tokens

Secrets must not be included in config revision diagnostics/log output.

---

## P1.6 Binary/application rollback

Config rollback is well specified, but package/binary rollback needs a concrete mechanism.

Define whether upgrades retain:

```text
previous binaries
previous Cockpit bundle
previous systemd units
previous installer version metadata
```

and for how many versions.

---

# 6. Missing documents before first public/pre-release

The following should exist before a usable v0.1 release:

| Document | Priority | Purpose |
|---|---|---|
| `README.md` | P0 | project purpose, install overview, safety warning, screenshots/status |
| `LICENSE` | P0 | project licensing decision |
| `docs/CONFIGURATION.md` | P0 | canonical user-facing config |
| `docs/STATE_MODEL.md` | P0 | persisted outage/recovery state schema |
| `docs/NUT_SHUTDOWN_MODEL.md` | P0 | FSD/primary/secondary/output-off ownership |
| `docs/TEST_PLAN.md` | P0 | executable acceptance matrix |
| `docs/SECURITY.md` | P1 | threat model, privilege boundaries, firewall, secrets |
| `docs/SYNOLOGY.md` | P1 | DSM setup and compatibility procedure |
| `docs/NUT.md` | P1 | supported modes/drivers/service behavior |
| `docs/DEPLOYMENT.md` | P1 | controller UPS power, switch/router topology, auto-power-on |
| `THIRD_PARTY_NOTICES.md` | P1 before copied code | attribution and reused source inventory |
| `CHANGELOG.md` | P1 before releases | release changes |

---

# 7. Test readiness audit

The existing documents identify many critical scenarios but they are not yet organized into a complete test plan.

The v0.1 acceptance suite SHALL cover at least:

### UPS/power state

```text
normal OL
short outage
long outage
LB
FSD
NUT communication loss
UPS USB disconnect/reconnect
power bounce
power restored during pre-commit grace
power restored after shutdown commit
```

### Controller interruption

```text
reboot during ON_BATTERY
power loss during boot
repeated interrupted boot
reboot after shutdown commit
reboot during WAITING_FOR_AC
reboot during RECOVERY_WAIT
power loss after first WoL
reboot during RESTORE_HOSTS
```

### Configuration/reliability

```text
invalid syntax
invalid semantic config
service fails after config activation
power loss during config probation
rollback to LKG
LKG corrupt, previous LKG valid
all known-good configs invalid -> FAILED_SAFE
agent crash
hung agent watchdog
health autofix succeeds
health autofix reaches circuit breaker
```

### Managed hosts

```text
Synology NUT shutdown
SSH shutdown
host fails to shut down
host already off before outage
host already online during restore
WoL retry
unreachable host during recovery
ordered shutdown
ordered recovery
```

### Installer

```text
clean Debian/Ubuntu
repeat install
upgrade
failed upgrade rollback
silent mode
TUI mode
local UPS
remote NUT
Synology profile
trusted LAN
restricted mode
amd64
arm64
riscv64
```

No physical-machine destructive action should be required for most CI tests; provide a simulation/mock NUT layer.

---

# 8. Implementation roadmap

## Phase 0 — specification closeout

- synchronize architecture and installation docs
- decide Go/runtime stack
- define NUT shutdown ownership
- define config schema
- define persisted state schema
- define agent IPC
- define operating modes
- add README/license/task tracker

**Exit criterion:** no unresolved P0 behavioral ambiguity in the safety-critical path.

## Phase 1 — core engine

- repository/build skeleton
- Go agent
- NUT read adapter
- explicit state machine
- crash-safe state store
- config revision manager/LKG rollback
- health service/timer
- `wolctl`
- simulation harness

**Exit criterion:** full outage/recovery can run against mocks without Cockpit.

## Phase 2 — host management

- status adapters
- shutdown adapters
- retry/verification logic
- Synology/NUT flow
- SSH flow
- ordering/dependencies

**Exit criterion:** multiple simulated/real lab hosts safely shut down and restore.

## Phase 3 — installer + Cockpit

- install.sh backend
- TUI/silent modes
- Cockpit plugin
- config editor using transactional agent API
- health/revision/rollback UI
- dry-run/armed controls

**Exit criterion:** clean supported OS can install and manage the complete stack without manual package setup.

## Phase 4 — release hardening

- CI multiarch builds
- SHA256SUMS
- upgrade/rollback tests
- hardware matrix
- Synology DSM validation
- UPS hardware tests
- security review
- documentation completion

**Exit criterion:** v0.1 pre-release candidate.

---

# 9. Recommended immediate next tasks

The highest-value next work is **not more broad research**. It is to close the remaining P0 design contracts and then start the skeleton.

Recommended order:

1. update `SOFTWARE_ARCHITECTURE.md` to incorporate boot/reliability requirements
2. update `INSTALLATION_REQUIREMENTS.md` to include health/LKG/boot requirements
3. create `docs/NUT_SHUTDOWN_MODEL.md`
4. create canonical configuration schema/example
5. create canonical persistent state schema
6. choose Go as the agent/`wolctl` implementation language (unless a contrary requirement appears)
7. define agent IPC/authorization model
8. create v0.1 GitHub issue backlog
9. create source/build skeleton
10. implement simulation-first state machine

---

# 10. Go / no-go assessment

## Go for prototype implementation

**YES**, after the P0 state/config/NUT-ownership contracts are closed.

## Go for production implementation today

**NO** — the main architecture is not yet synchronized with the newer normative requirements, and key safety contracts remain undefined.

## Go for public v0.1 release

**NO** — no implementation, installer, CI, test evidence or release packaging exists yet.

The project is in a strong **design-complete / specification-closeout** phase and should now transition into controlled implementation.