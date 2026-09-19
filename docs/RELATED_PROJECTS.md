# Related GitHub Projects and Code Reuse Assessment

**Project:** `cockpit-ups-wol`  
**Research date:** 2026-09-19  
**Scope:** Deep search for open-source projects related to NUT, UPS shutdown/recovery orchestration, Wake-on-LAN, Cockpit/web administration, Synology DSM, Proxmox, persistent outage state, installation, and small-SBC deployment.

## Executive summary

The deeper search changes the reuse picture substantially. No single upstream project matches the complete `cockpit-ups-wol` design, but several projects cover major parts of it well enough that we should deliberately reuse patterns and, where licensing allows, selected code.

The strongest references are now:

- [`hardwarehaven/wolnut`](https://github.com/hardwarehaven/wolnut) — closest match to our **automatic recovery** path: remembers which hosts were online, persists outage state, waits for utility/battery recovery, and sends WoL.
- [`world-wide-dev/nutcracker`](https://github.com/world-wide-dev/nutcracker) — strongest small reference for a **deterministic, restart-safe shutdown state machine**, staged graceful/forced shutdown, controller-last behavior, UPS communication failure handling, and power-restoration debounce.
- [`m4r1k/Eneru`](https://github.com/m4r1k/Eneru) — broadest **shutdown-orchestration architecture** found: multi-UPS policy, remote systems, VM/container shutdown, multiple triggers, observability, dry-run, and extensive testing.
- [`wijits36/hypercore-power-manager`](https://github.com/wijits36/hypercore-power-manager) — very close to our complete lifecycle: NUT monitoring, remembered pre-outage VM state, staged shutdown, an abort window, host power-off, automatic recovery, and ordered restart.
- [`ffind-dev/pve-ups`](https://github.com/ffind-dev/pve-ups) — strong appliance design for **safe Proxmox shutdown**, read-only NUT input, host ordering, dry-run/armed operation, and controller-host-last behavior.
- [`deviationist/cockpit-upside`](https://github.com/deviationist/cockpit-upside) — closest match to the **Cockpit + NUT frontend**.
- [`JuanCF/nutwatch`](https://github.com/JuanCF/nutwatch) — strong NUT administration reference with WoL mappings, NUT config editing, USB detection, event hooks, atomic writes, and installer/provisioning patterns.
- [`rtorcato/homelab-nut`](https://github.com/rtorcato/homelab-nut) — useful Go/TUI/fleet-setup reference with plan/apply workflow, NUT server/client installation, shutdown targets, and automation-friendly JSON output.
- [`riofutab/nut-server`](https://github.com/riofutab/nut-server) — useful Go implementation reference for **persistent orchestration**, idempotent commands, remote acknowledgements, atomic state, systemd hardening, policy evaluation, and packaging.
- [`ScottPierce/synology-ecoflow-nut`](https://github.com/ScottPierce/synology-ecoflow-nut) — focused validation/reference for **Synology DSM NUT compatibility**.
- [`Trugamr/wol`](https://github.com/Trugamr/wol) — best small permissively licensed WoL packet implementation found.

The preferred strategy remains **selective reuse** rather than embedding an entire upstream application.

---

## Search methodology

The research deliberately used overlapping searches rather than only component names. Search themes included:

- NUT + Wake-on-LAN + power recovery
- UPS shutdown/recovery daemon
- UPS orchestration + homelab
- Proxmox + NUT + shutdown
- Synology + NUT
- Cockpit + UPS + NUT
- persistent UPS state / restart-safe shutdown
- multi-node graceful shutdown
- power restoration + WoL
- Go-based UPS orchestration

Forks and obvious clones were de-duplicated against their upstream project. Repositories with no clear license, very limited activity, or no meaningful additional design value are listed separately instead of being promoted as reuse candidates.

---

# Reuse decision matrix

| Project | Main value | Language | License | Reuse priority | Recommended use |
|---|---|---|---|---|---|
| [`hardwarehaven/wolnut`](https://github.com/hardwarehaven/wolnut) | Recovery + prior-host state + WoL | Python | MIT | **Very high** | Selectively adapt recovery/state concepts; fix unsafe unknown-data behavior |
| [`world-wide-dev/nutcracker`](https://github.com/world-wide-dev/nutcracker) | Deterministic shutdown state machine | Shell | MIT | **Very high** | Reuse state-machine/safety concepts; do not copy shell daemon wholesale |
| [`m4r1k/Eneru`](https://github.com/m4r1k/Eneru) | Full shutdown orchestration | Python | MIT | **Very high** | Architecture, adapters, policy triggers, testing and observability reference |
| [`wijits36/hypercore-power-manager`](https://github.com/wijits36/hypercore-power-manager) | Shutdown + recovery lifecycle | Python | MIT | **Very high** | Recovery state-machine and pre-outage-state reference |
| [`ffind-dev/pve-ups`](https://github.com/ffind-dev/pve-ups) | Proxmox power appliance | Python | MIT | **Very high** | Proxmox adapter, dry-run/armed model, fail-safe policy and shutdown planning |
| [`deviationist/cockpit-upside`](https://github.com/deviationist/cockpit-upside) | Cockpit + NUT UI/control | TypeScript | LGPL-2.1 | **Very high** | Cockpit/NUT frontend patterns and selected compatible code |
| [`JuanCF/nutwatch`](https://github.com/JuanCF/nutwatch) | NUT administration + WoL | TS/Python/Shell | MIT | **Very high** | Config parsers, atomic writes, USB detection, WoL/event model, tests |
| [`rtorcato/homelab-nut`](https://github.com/rtorcato/homelab-nut) | Fleet installer/TUI/shutdown | Go | MIT | **High** | TUI plan/apply UX, inventory model, roles, Go packaging |
| [`riofutab/nut-server`](https://github.com/riofutab/nut-server) | Go orchestration internals | Go | MIT | **High** | Atomic state, idempotent actions, ACKs, policies, systemd hardening |
| [`Trugamr/wol`](https://github.com/Trugamr/wol) | WoL packet core | Go | MIT | **Very high** | Directly adapt small `magicpacket` package |
| [`ScottPierce/synology-ecoflow-nut`](https://github.com/ScottPierce/synology-ecoflow-nut) | Synology DSM compatibility | Shell/Docker | GPL-2.0-or-later | **High reference** | DSM/NUT compatibility and security behavior; avoid copying GPL code unless intended |
| [`Brandawg93/PeaNUT`](https://github.com/Brandawg93/PeaNUT) | Mature NUT dashboard/API | TypeScript | Apache-2.0 | **High reference** | NUT UX/API/capability handling |
| [`SuperioOne/nut_webgui`](https://github.com/SuperioOne/nut_webgui) | NUT dashboard | Rust | Apache-2.0 | **High reference** | NUT variables/commands/UX reference |
| [`exelban/nutshell`](https://github.com/exelban/nutshell) | Native NUT TCP client | Go | MIT | **High later** | Possible future native NUT client instead of repeated `upsc` processes |
| [`MarekWo/UPS_Server_Docker`](https://github.com/MarekWo/UPS_Server_Docker) | Virtual NUT + recovery + client status | Python | MIT | **Medium/high** | Simulation, client state reporting, delayed WoL and management concepts |
| [`seriousm4x/UpSnap`](https://github.com/seriousm4x/UpSnap) | Mature WoL manager | Go/SvelteKit | MIT | **Medium/high** | Host/WoL UX reference |
| [`geerlingguy/pi-nut`](https://github.com/geerlingguy/pi-nut) | SBC NUT appliance | Shell/config | GPL-3.0 | **Medium reference** | Deployment/configuration lessons, not preferred direct reuse |
| [`networkupstools/nut`](https://github.com/networkupstools/nut) | Authoritative UPS backend | C | mixed/project-specific | **Essential** | Runtime dependency and behavioral specification |
| [`cockpit-project/starter-kit`](https://github.com/cockpit-project/starter-kit) | Cockpit plugin foundation | JS/TS | LGPL-2.1 | **Essential** | Frontend/build foundation |
| [`cockpit-project/cockpit`](https://github.com/cockpit-project/cockpit) | Cockpit platform/API | mixed | component-specific | **Essential** | Authoritative Cockpit API/security reference |

---

# 1. hardwarehaven/wolnut — recovery-agent reference

Repository: <https://github.com/hardwarehaven/wolnut>  
License: MIT  
Language: Python

WOLNUT is the closest small project to the **recovery half** of `cockpit-ups-wol`.

It already implements:

- NUT polling through `upsc`
- detection of `OB` / `OL`
- tracking which clients were online before a battery event
- persistent JSON state
- resume after service/controller restart
- configurable minimum battery percentage before recovery
- configurable delay after utility restoration
- WoL retries
- ping-based client status
- optional ARP MAC discovery
- atomic state-file replacement

## Strong reuse candidates

- previous-online host snapshot semantics
- per-host WoL attempt state
- retry timing
- persisted outage marker
- state serialization tests
- recovery loop test cases

## Important problems not to copy

The current implementation uses unsafe defaults when NUT data is absent:

- missing `battery.charge` effectively becomes `100%`
- missing `ups.status` effectively becomes `OL`

For a safety-oriented recovery controller, a NUT communication failure MUST become `UNKNOWN`, never "mains online / battery full".

Its restart path also resets parts of persisted state too early. `cockpit-ups-wol` should preserve the complete outage transaction until recovery is conclusively finished.

## Decision

**Use WOLNUT as a very high-value algorithm/test reference, but implement a stricter state machine.**

---

# 2. world-wide-dev/nutcracker — shutdown state-machine reference

Repository: <https://github.com/world-wide-dev/nutcracker>  
License: MIT  
Language: Shell

Nutcracker is small but architecturally valuable. It is explicitly designed as a deterministic, restart-safe NUT shutdown orchestrator for Proxmox.

Its state machine is approximately:

```text
NORMAL
  ↓
OUTAGE
  ↓
GRACEFUL
  ↓
FORCE
  ↓
HOST
```

Useful behavior includes:

- outage grace timer
- runtime-based escalation thresholds
- dependency-aware shutdown ordering
- graceful shutdown followed by force-stop escalation
- controller/host shutdown last
- persisted stage/state
- restart-safe execution
- `UNKNOWN` state on UPS read failure
- UPS communication failure counters
- stable-`OL` debounce before considering utility restored
- health/status file
- explicit transition-reason logging
- simulation input support

## Strong reuse candidates

These concepts should strongly influence `cockpit-ups-wol-agent`:

- monotonic/forward-only destructive state transitions
- explicit stage entry actions
- transition reasons in logs
- graceful → force escalation
- utility-restored debounce
- communication-loss handling
- health snapshot
- restart-safe stage persistence

The shell implementation is Proxmox-specific and should not become our runtime implementation.

## Decision

**Very high-value state-machine and failure-handling reference.**

---

# 3. m4r1k/Eneru — broad orchestration architecture

Repository: <https://github.com/m4r1k/Eneru>  
License: MIT  
Language: Python

Eneru is the broadest shutdown-management project found during this research.

Features relevant to us include:

- multiple UPS devices
- independent UPS/shutdown groups
- remote servers over SSH
- VM/container shutdown
- Proxmox, ESXi, XCP-ng and libvirt pre-shutdown actions
- configurable custom shutdown commands
- multiple independent shutdown triggers
- time-on-battery trigger
- battery percentage/runtime triggers
- depletion-rate trigger
- FSD and communication-loss handling
- dry-run operation
- shutdown-plan preview
- browser dashboard and TUI
- persistent event records
- Prometheus/MQTT/Grafana integrations
- comprehensive automated and end-to-end tests

## What to reuse

Use Eneru primarily for:

- target/adapter interface design
- trigger/policy model
- shutdown plan generation
- dry-run execution model
- per-phase timeouts
- error isolation between targets
- observability schema
- integration-test design

## What not to reuse wholesale

Eneru is much larger than the target appliance and includes its own browser API/UI, metrics stack and many optional integrations. That duplicates Cockpit and increases runtime dependencies.

## Decision

**Top-tier architecture reference; selective algorithm and adapter reuse only.**

---

# 4. wijits36/hypercore-power-manager — full shutdown/recovery lifecycle

Repository: <https://github.com/wijits36/hypercore-power-manager>  
License: MIT  
Language: Python

This project is particularly useful because it covers both directions of the lifecycle:

```text
NUT OB
 → monitor threshold
 → shut down VMs
 → abort window
 → power off hosts
 → wait for AC
 → power hosts on
 → wait for API
 → restart previously-running VMs
```

It records which VMs were running before shutdown and restores those VMs after recovery. It also separates a reversible **abort window** from the final host-power-off stage.

## Important architectural lesson

`cockpit-ups-wol` should explicitly distinguish:

1. a reversible pre-shutdown phase where utility restoration can cancel the outage action; and
2. a **shutdown commit point** after which shutdown is latched and recovery happens only after the shutdown transaction completes.

This avoids trying to reverse a partially completed shutdown sequence.

## Decision

**Very high-value recovery-state and shutdown-latching reference.**

---

# 5. ffind-dev/pve-ups — Proxmox appliance and safety model

Repository: <https://github.com/ffind-dev/pve-ups>  
License: MIT  
Language: Python

PVE-UPS treats NUT as a read-only UPS data source while keeping shutdown policy inside an appliance.

Relevant concepts:

- dedicated Proxmox API tokens rather than root SSH
- per-host shutdown order
- controller/appliance host always last
- host↔UPS mapping
- AND/OR logic for redundant/multiple power feeds
- web wizard with test buttons
- explicit dry-run versus armed mode
- persistent state/event history
- health/status endpoints
- Proxmox cluster/HA preparation
- cluster-aware shutdown
- failure-safe treatment of unavailable UPS data

## Future adapter opportunity

Add a future shutdown adapter such as:

```text
proxmox-api
```

instead of requiring SSH for Proxmox hosts.

This can use a least-privilege `Sys.PowerMgmt` API token.

## Decision

**Very high-value Proxmox and safety-policy reference.**

---

# 6. deviationist/cockpit-upside — Cockpit/NUT frontend

Repository: <https://github.com/deviationist/cockpit-upside>  
License: LGPL-2.1  
Language: TypeScript / React / PatternFly

This remains the closest project to our Cockpit UPS frontend.

Relevant functionality:

- `cockpit.spawn()` wrappers for `upsc`, `upsrw` and `upscmd`
- multi-UPS model
- local and remote NUT sources
- capability-driven controls
- instant-command risk grouping
- writable UPS variables
- setup wizard
- NUT mode/server/client configuration
- `nut-scanner` integration
- configuration backup/write workflow
- journal/service diagnostics
- low-battery `upsmon` setup

## Decision

**Use as the principal specialized Cockpit/NUT reference.**

Automatic outage/recovery policy must still remain in `cockpit-ups-wol-agent`, not in the browser extension.

---

# 7. JuanCF/nutwatch — NUT administration and WoL configuration

Repository: <https://github.com/JuanCF/nutwatch>  
License: MIT  
Languages: TypeScript / Python / Shell

NutWatch is especially useful for NUT administration rather than recovery sequencing.

It provides:

- UPS CRUD and telemetry
- NUT user management
- full `upsmon.conf` editing
- NUT event hooks
- live journal streaming
- raw config editing
- WoL target management
- UPS-event → WoL-target mappings
- manual Wake / Wake All
- ARP-based host discovery
- atomic config writes
- validation against newline/identifier injection
- USB UPS detection
- `nut-scanner`
- Proxmox VM provisioning script
- SHA-256 verification of downloaded VM image
- whiptail-guided setup
- test suites for parsers, services and WoL

## Strong reuse candidates

- NUT parser/serializer tests
- atomic config writer patterns
- USB UPS discovery logic
- WoL host model
- event hook structure
- validation rules
- installer hardware-detection workflow

Event-triggered `ONLINE → WOL` alone does not satisfy our recovery policy; battery ≥80% and stable AC remain agent-controlled conditions.

## Decision

**Very high-value configuration and NUT-management reference.**

---

# 8. rtorcato/homelab-nut — Go TUI/install/fleet reference

Repository: <https://github.com/rtorcato/homelab-nut>  
License: MIT  
Language: Go

This project is useful for our installer/TUI and machine-readable management approach.

Relevant concepts:

- Go CLI and TUI
- guided inventory generation
- `plan` before `apply`
- dry-run/diff style workflow
- stable JSON output
- documented exit codes
- NUT server/client/exporter roles
- remote SSH application
- shutdown targets
- per-device shutdown recipes
- systemd shutdown daemon
- Go release binaries
- Prometheus/exporter setup

## Design lesson

Our `--tui` installer should show an explicit **installation plan** before changes. Cockpit should similarly be able to preview the automatic shutdown/recovery plan.

## Limitation

Current published architecture support is focused on amd64/arm64. `cockpit-ups-wol` still requires riscv64 as a first-class release target.

## Decision

**High-value TUI, provisioning and Go packaging reference.**

---

# 9. riofutab/nut-server — Go orchestration implementation patterns

Repository: <https://github.com/riofutab/nut-server>  
License: MIT  
Language: Go

Although this project reads UPS state through SNMP rather than NUT by default, its orchestration internals are highly relevant.

Useful implementation ideas:

- separate master/slave binaries
- persistent command IDs
- acknowledgement of remote shutdown execution
- prevention of duplicate commands after restart
- atomic state writes using temp file + `fsync` + rename
- mode `0600` state files
- dry-run default
- configurable multi-policy engine
- local/controller shutdown after remote completion
- emergency runtime threshold
- timeout handling
- structured logging
- Prometheus health data
- systemd sandboxing
- graceful process cancellation
- `.deb`, `.rpm`, `.tar.gz` releases
- SHA256SUMS
- amd64/arm64 builds
- end-to-end integration tests

## Strong reuse candidates

For our Go agent implementation, copy/adapt concepts for:

- `outage_id` / action IDs
- idempotent host actions
- atomic durable persistence
- action acknowledgement state
- service hardening
- release packaging/checksum flow

Do not adopt its custom master/slave network protocol as the default architecture; normal NUT clients should remain normal NUT clients where possible.

## Decision

**High-value Go implementation reference.**

---

# 10. ScottPierce/synology-ecoflow-nut — Synology compatibility reference

Repository: <https://github.com/ScottPierce/synology-ecoflow-nut>  
License: GPL-2.0-or-later for project-authored files

This is a focused DSM/NUT compatibility project.

Particularly relevant findings:

- default UPS name is `ups`
- DSM compatibility uses a monitor-only NUT account
- `upsmon secondary` is used on modern NUT
- DSM 7 commonly generates `monuser` / `secret` in its compatibility path
- the account is intentionally denied UPS commands, writable variables and FSD authority
- DSM itself owns the NAS shutdown decision
- the project strongly recommends validating automatic shutdown on the exact DSM/model/UPS combination

This independently supports the Synology compatibility direction already selected for `cockpit-ups-wol`.

## Licensing note

Use the project for behavioral/reference validation. Do not copy GPL code into a differently licensed codebase unless that licensing decision is intentional.

## Decision

**High-value Synology behavior/reference project.**

---

# 11. Trugamr/wol — WoL packet implementation

Repository: <https://github.com/Trugamr/wol>  
License: MIT  
Language: Go

The small `magicpacket` package cleanly implements the standard 102-byte WoL packet:

```text
6 × FF
+
16 × target MAC
```

It separates packet creation from `io.Writer` output and UDP broadcast, making the code easy to test.

## Decision

**Preferred direct-code candidate for the `wolctl` packet core.**

Retain upstream MIT attribution if adapted.

---

# 12. Brandawg93/PeaNUT — mature NUT UI/API reference

Repository: <https://github.com/Brandawg93/PeaNUT>  
License: Apache-2.0  
Language: TypeScript

PeaNUT is a mature and actively maintained NUT dashboard with a large user base.

Useful reference areas:

- multiple NUT servers/UPSes
- dashboard visualization
- direct NUT API/terminal interactions
- commands and writable values
- authentication
- Prometheus/Influx integrations
- configuration model

Cockpit already supplies our web platform, so its standalone Next.js architecture should not be imported.

## Decision

**Strong NUT UX/API reference, but UPSide remains closer to our Cockpit frontend.**

---

# 13. SuperioOne/nut_webgui

Repository: <https://github.com/SuperioOne/nut_webgui>  
License: Apache-2.0  
Language: Rust

Useful for:

- NUT variable grouping
- command/control behavior
- multiple UPS sources
- authentication/error presentation

## Decision

**Strong behavior/UX reference; do not import its separate web-server architecture.**

---

# 14. exelban/nutshell

Repository: <https://github.com/exelban/nutshell>  
License: MIT  
Language: Go

Contains a compact native Go NUT protocol client implementing TCP connection, `VER`, `NETVER`, `LIST UPS`, authentication and response parsing.

For v0.x, spawning distro-provided NUT CLI tools remains lower risk. If repeated process spawning later becomes measurable overhead, NutShell is a useful starting point for a native agent-side NUT client.

## Decision

**Future optimization candidate, not necessary for v0.1.**

---

# 15. MarekWo/UPS_Server_Docker

Repository: <https://github.com/MarekWo/UPS_Server_Docker>  
License: MIT  
Language: Python

This project solves a somewhat different hardware problem by synthesizing a virtual NUT UPS from non-UPS-powered sentinel hosts, but several concepts are relevant:

- standard NUT clients, including Synology DSM
- live per-client shutdown status/countdown
- configurable delayed WoL recovery
- per-host broadcast address
- automatic WoL enable/disable
- outage simulation
- scheduled outage simulation
- optional external battery-monitor data

## Decision

**Useful simulation/client-status reference; not a preferred runtime dependency.**

---

# 16. seriousm4x/UpSnap

Repository: <https://github.com/seriousm4x/UpSnap>  
License: MIT  
Language: Go / SvelteKit

Useful for mature WoL host-management UX, device status and network handling. Its full application stack is unnecessary because Cockpit is our UI/authentication platform.

## Decision

**WoL UX reference only.**

---

# 17. geerlingguy/pi-nut

Repository: <https://github.com/geerlingguy/pi-nut>  
License: GPL-3.0

Useful practical reference for small-SBC NUT deployment and safe server shutdown configuration.

## Decision

**Deployment/configuration reference. Avoid direct GPL code reuse unless intentionally compatible with project licensing.**

---

# 18. Official projects

## networkupstools/nut

Repository: <https://github.com/networkupstools/nut>

NUT remains the authoritative behavioral reference for:

- drivers
- `upsd`
- `upsmon`
- `upsc`
- `upsrw`
- `upscmd`
- `nut-scanner`
- status tokens
- primary/secondary semantics
- shutdown/FSD behavior

Use installed NUT binaries/services instead of vendoring NUT source.

## cockpit-project/starter-kit

Repository: <https://github.com/cockpit-project/starter-kit>

Use as the authoritative frontend/build baseline.

## cockpit-project/cockpit

Repository: <https://github.com/cockpit-project/cockpit>

Use as the authoritative source for Cockpit APIs, privilege handling, service/journal integration and security conventions.

---

# Deep-search design findings for cockpit-ups-wol

The related projects reveal several requirements that should be treated as safety properties rather than optional features.

## 1. Unknown UPS data must fail safe

Never map a communication error or missing variable to a healthy state.

Required semantics:

```text
missing/stale NUT data
        ↓
      UNKNOWN
        ↓
NO automatic wake/recovery decision
```

Recovery must require positively confirmed current data.

---

## 2. Add a shutdown commit point / latch

Before destructive shutdown begins, restored utility may cancel an outage after the stable-AC debounce.

After the shutdown commit point, the transaction should not be reversed halfway through.

Suggested model:

```text
NORMAL
  ↓
ON_BATTERY            ← reversible
  ↓
SHUTDOWN_COMMITTED    ← point of no return
  ↓
SHUTDOWN_IN_PROGRESS
  ↓
WAITING_FOR_AC
  ↓
RECOVERY_WAIT
  ↓
RESTORE_HOSTS
```

This avoids mixed infrastructure states caused by trying to restart some devices while others are still shutting down.

---

## 3. Add operational modes

Simulation exists in the current architecture, but the agent should expose a clearer operational mode:

```text
monitor   — observe and log only
dry-run   — evaluate full policy and record intended actions, execute nothing destructive
armed     — execute configured shutdown/recovery actions
```

New installations SHOULD begin in `dry-run` or require an explicit acknowledgement before becoming `armed`.

Cockpit should display the mode prominently.

---

## 4. Use monotonic elapsed-time decisions

Outage duration, stable-AC intervals and timeouts should use a monotonic clock in-process.

Wall-clock timestamps remain useful for logs and persistent history, but NTP/time changes must not shorten or extend a safety timeout unexpectedly.

After restart, persist enough timestamps/state to reconstruct conservative behavior.

---

## 5. Make host actions idempotent

Each outage should have a durable ID, for example:

```text
outage_id
```

Each host action should persist:

```text
planned
requested
acknowledged
failed
timed_out
completed
```

A restart must not blindly issue duplicate destructive commands or duplicate WoL actions.

---

## 6. Strengthen persistent-state writes

Minimum durable pattern:

```text
write temporary file
fsync temporary file
rename atomically
fsync parent directory
```

State files containing internal action state should use restrictive permissions, preferably `0600` unless another service must read them.

---

## 7. Add an agent health snapshot

Provide machine-readable health separate from journald, e.g.:

```text
/run/cockpit-ups-wol/agent-health.json
```

Potential contents:

- agent version
- current state
- UPS status
- age of last successful NUT read
- outage ID
- active action
- recovery threshold
- battery charge/runtime
- last error

Cockpit can read this for a fast overview.

---

## 8. Preview the complete power plan

Before arming automation, Cockpit should show the effective sequence:

```text
Power failure
  → grace 120 s
  → desktop shutdown
  → Synology/NUT shutdown
  → Proxmox shutdown
  → controller last

Recovery
  → AC stable 120 s
  → battery >= 80%
  → network ready
  → NAS wake
  → Proxmox wake
  → workstation wake
```

This is easier to validate than scattered per-host settings.

---

## 9. Use adapters rather than hard-coded target types

Recommended target adapter interface:

```text
status()
shutdown()
wake()
verify_shutdown()
verify_recovery()
```

Initial implementations:

```text
nut
ssh
wol
none
```

Future adapters:

```text
proxmox-api
ipmi
redfish
home-assistant
custom-helper
```

This follows the useful extensibility seen in Eneru and appliance-style projects without importing their entire runtime.

---

## 10. Expand automated testing

Beyond unit tests, add scenario tests for:

- short outage
- AC bounce during grace period
- AC return after shutdown commit
- NUT communication loss while OL
- NUT communication loss while OB
- missing `battery.charge`
- invalid/stale UPS data
- controller restart during each state
- duplicate host action prevention
- host shutdown timeout
- partial shutdown failure
- network unavailable at recovery
- battery remains below 80%
- battery crosses 80%
- WoL retry
- host already online
- host was off before outage
- Synology secondary client
- multi-UPS future policy

A fake/simulated NUT source should be part of CI.

---

# Screened but lower-priority projects

These were found but should not be primary reuse sources.

## Maxi2710/nut-wake-on-lan-recovery

Repository: <https://github.com/Maxi2710/nut-wake-on-lan-recovery>

Small Python NUT→WoL daemon, but no detected license and little activity. WOLNUT covers the same problem with clearer licensing and a stronger code/test base.

**Decision:** reference only.

## tanandy/powerpulse

Repository: <https://github.com/tanandy/powerpulse>

Another small NUT→WoL recovery project with no detected license and minimal activity.

**Decision:** reference only.

## ahmetozer/wakeonlan

Repository: <https://github.com/ahmetozer/wakeonlan>

Useful Go networking example, but no detected license and much less active than `Trugamr/wol`.

**Decision:** do not copy code unless licensing is clarified.

## Forks of WOLNUT / Eneru

Multiple forks appeared in repository searches. Unless a fork contains a specific fix we need, review and attribute the upstream project rather than treating each fork as a separate reuse candidate.

---

# Recommended implementation source map

```text
Cockpit frontend
├── cockpit-project/starter-kit
└── deviationist/cockpit-upside

NUT administration/config
├── official NUT
├── JuanCF/nutwatch
├── deviationist/cockpit-upside
└── PeaNUT / nut_webgui as secondary UX references

Recovery behavior
├── hardwarehaven/wolnut
└── wijits36/hypercore-power-manager

Shutdown safety/state machine
├── world-wide-dev/nutcracker
├── m4r1k/Eneru
└── ffind-dev/pve-ups

Go agent implementation patterns
├── riofutab/nut-server
├── rtorcato/homelab-nut
└── exelban/nutshell (future direct NUT protocol)

WoL core
└── Trugamr/wol

Synology compatibility
├── official NUT behavior
└── ScottPierce/synology-ecoflow-nut
```

---

# Recommended v0.1 reuse strategy

## Cockpit

Start from the official Cockpit starter kit and selectively adapt UPS-specific patterns from UPSide.

## Agent

Implement a new lightweight `cockpit-ups-wol-agent` rather than embedding Eneru/WOLNUT wholesale.

Its design should combine the strongest proven concepts found during this research:

```text
WOLNUT
  previous-online snapshot + recovery/WoL behavior

Nutcracker
  deterministic shutdown stages + fail-safe UNKNOWN + debounce

HyperCore Power Manager
  explicit recovery states + shutdown commit/abort boundary

PVE-UPS
  dry-run/armed model + controller-last + future Proxmox API adapter

riofutab/nut-server
  durable atomic state + idempotent action IDs + systemd hardening
```

For a small SBC, Go remains attractive because the result can be a single static multi-architecture binary with low memory usage and no Python runtime dependency.

## WoL

Adapt the small MIT `Trugamr/wol/magicpacket` implementation into `wolctl` or into an internal Go package shared by the agent and CLI.

## NUT

Continue using the distribution's NUT server/client/tools in v0.x. Reevaluate a direct Go NUT protocol client only if CLI process-spawn overhead becomes meaningful.

---

# License and attribution policy

Before importing upstream source, add:

```text
LICENSE
THIRD_PARTY_NOTICES.md
```

For every copied/adapted component record:

- upstream project
- upstream URL
- upstream path
- exact commit/tag
- license
- copyright notice
- local destination
- modifications made

Preferred reuse order:

```text
1. Standard Linux / NUT / Cockpit functionality
2. Small MIT/BSD/Apache components
3. LGPL code where project licensing is intentionally compatible
4. Larger applications as architecture/test references
5. GPL code as reference unless compatible licensing is deliberately chosen
6. No-license repositories: reference only, no source copying
```

---

# Research conclusion

The project does **not** need to invent the full problem from scratch. Mature or near-matching open-source work exists for nearly every subsystem, but the complete combination remains distinct:

```text
Cockpit management
+
NUT server/client compatibility
+
Synology support
+
restart-safe ordered shutdown
+
controller-last behavior
+
remember-what-was-running
+
stable-AC gate
+
UPS recharge >= 80%
+
ordered Wake-on-LAN recovery
+
small ARM64/RISC-V appliance deployment
```

The most important outcome of the deep search is therefore not to replace the architecture with one upstream project, but to base each subsystem on the strongest existing implementation patterns and preserve `cockpit-ups-wol-agent` as the small safety-critical integration layer.