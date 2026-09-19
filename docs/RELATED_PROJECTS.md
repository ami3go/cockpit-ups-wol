# Related GitHub Projects and Code Reuse Assessment

**Project:** `cockpit-ups-wol`  
**Research date:** 2026-09-19  
**Purpose:** Identify related open-source projects that can reduce implementation effort, provide proven design patterns, or supply reusable code for Cockpit, NUT, Wake-on-LAN, installation, and power-management features.

## Summary

The most relevant projects currently identified are:

- [`hardwarehaven/wolnut`](https://github.com/hardwarehaven/wolnut) — very close to the planned persistent recovery agent: monitors NUT, remembers which hosts were online before an outage, persists state, waits for utility/battery recovery, and sends Wake-on-LAN packets.
- [`deviationist/cockpit-upside`](https://github.com/deviationist/cockpit-upside) — very close to the planned Cockpit/NUT frontend: NUT monitoring, `upscmd`, `upsrw`, setup wizard, remote NUT support and safe configuration editing.
- [`Trugamr/wol`](https://github.com/Trugamr/wol) — compact Go Wake-on-LAN implementation with a clean, testable magic-packet package.
- [`exelban/nutshell`](https://github.com/exelban/nutshell) — compact Go NUT client that may be useful if the agent later moves away from spawning `upsc`.

The recommended strategy remains **selective reuse**. No single upstream project covers the complete `cockpit-ups-wol` design: ordered shutdown, controller-last shutdown, Synology support, persistent outage state, stable-AC recovery gating, configurable battery threshold defaulting to 80%, ordered host restoration, Cockpit management, and multi-distribution installation.

---

## Reuse decision matrix

| Project | Area | License | Reuse value | Recommended use |
|---|---|---:|---|---|
| [`hardwarehaven/wolnut`](https://github.com/hardwarehaven/wolnut) | NUT + outage recovery + WoL | MIT | **Very high** | Reuse concepts and selected state/recovery code after fixing safety issues |
| [`deviationist/cockpit-upside`](https://github.com/deviationist/cockpit-upside) | Cockpit + NUT | LGPL-2.1 | **Very high** | Reuse/adapt Cockpit/NUT patterns and selected code if license strategy is compatible |
| [`cockpit-project/starter-kit`](https://github.com/cockpit-project/starter-kit) | Cockpit plugin foundation | LGPL-2.1 | **Very high** | Use as frontend/build foundation |
| [`Trugamr/wol`](https://github.com/Trugamr/wol) | Wake-on-LAN | MIT | **Very high** | Reuse or adapt the small `magicpacket` package |
| [`networkupstools/nut`](https://github.com/networkupstools/nut) | UPS backend/protocol | Project-specific/mixed | **Essential reference** | Use installed NUT runtime and official behavior; avoid vendoring source without file-level license review |
| [`SuperioOne/nut_webgui`](https://github.com/SuperioOne/nut_webgui) | NUT web UI | Apache-2.0 | **High** | UX/capability reference; selective compatible reuse possible |
| [`exelban/nutshell`](https://github.com/exelban/nutshell) | Native NUT Go client | MIT | **High, later** | Adapt if replacing `upsc` polling with direct NUT TCP access |
| [`seriousm4x/UpSnap`](https://github.com/seriousm4x/UpSnap) | WoL management UI | MIT | **Medium/high** | Feature/UX reference; avoid importing its larger application stack |
| [`geerlingguy/pi-nut`](https://github.com/geerlingguy/pi-nut) | NUT appliance/configuration | GPL-3.0 | **Medium** | Deployment/configuration reference; avoid direct reuse unless GPL implications are accepted |
| [`cockpit-project/cockpit`](https://github.com/cockpit-project/cockpit) | Cockpit internals/API | Per-component/project licensing | **Essential reference** | API/security/systemd/journal/PatternFly reference |
| [`ahmetozer/wakeonlan`](https://github.com/ahmetozer/wakeonlan) | Go WoL service | No detected license | **Low for code reuse** | Reference only unless licensing is clarified |

---

# 1. hardwarehaven/wolnut

Repository: <https://github.com/hardwarehaven/wolnut>  
Language: Python  
License: MIT  
Runtime: Python 3.11+, Click, PyYAML, `wakeonlan`  
Status at review: repository active as a public project; latest source push observed in December 2025

## Why it is especially relevant

WOLNUT implements almost the same recovery concept planned for `cockpit-ups-wol-agent`:

```text
NUT/upsc
   │
   ▼
observe UPS status
   │
   ├── detect OB
   ├── record hosts that were online
   ├── persist outage/client state
   │
   ▼
wait for utility restoration
   │
   ▼
wait for battery threshold
   │
   ▼
send WoL to hosts that were online before outage
```

Its documented behavior includes:

- NUT monitoring through `upsc`
- client state checks via ping
- recording which clients were online before the outage
- persistent JSON state across process/controller restart
- configurable restore delay
- configurable minimum battery percentage
- WoL retries
- client recovery timeout
- automatic MAC discovery using ARP
- Docker and standalone execution

Example upstream configuration:

```yaml
wake_on:
  restore_delay_sec: 30
  min_battery_percent: 25
  client_timeout_sec: 600
  reattempt_delay: 30
```

For `cockpit-ups-wol`, the equivalent default battery threshold remains **80%**, not WOLNUT's lower example/default values.

## Strong code/design candidates

### Persistent host state

WOLNUT's `ClientStateTracker` is useful reference code for:

- `was_online_before_battery`
- current online state
- WoL-attempt state
- WoL retry timestamps
- per-host skip state
- persisted UPS-on-battery state

It also uses a temporary file followed by replacement when saving state. That aligns with the project's requirement for atomic state persistence.

Recommended reuse approach:

```text
Study/adapt state model
       +
retain atomic-write concept
       +
add explicit state/schema version
       +
add outage ID/state-machine state
       +
add crash-safe validation
```

### Recovery loop

The upstream recovery loop demonstrates a compact working implementation of:

- detecting `OB`
- detecting restored `OL`
- waiting for a battery threshold
- waiting for a restore delay
- restoring only previously-running hosts
- retrying WoL

This should inform the implementation of `cockpit-ups-wol-agent` rather than designing the whole loop from scratch.

## Safety issues that MUST NOT be copied unchanged

WOLNUT is a useful source, but its current implementation is not strict enough for the safety policy of this project.

### 1. Missing battery data defaults to 100%

Its helper effectively behaves like:

```python
ups_status.get("battery.charge", 100)
```

That means a UPS which does not report `battery.charge`, or a failed status read, can appear to have a full battery.

`cockpit-ups-wol` SHALL instead treat missing battery data as **unknown** and use the configured fallback policy:

```text
percentage
runtime
time-based recharge
manual
```

Never assume 100%.

### 2. Missing UPS status defaults to OL

When `upsc` fails, WOLNUT returns an empty status dictionary and later defaults `ups.status` to `OL`.

This can turn a communication failure into an apparent "utility online" condition.

`cockpit-ups-wol-agent` SHALL instead distinguish:

```text
OL
OB
UNKNOWN / COMMUNICATION_FAILURE
```

Recovery MUST NOT start from an unknown UPS state.

### 3. Restart recovery resets state too early

When WOLNUT loads state indicating the UPS had been on battery, it marks a restoration event and immediately resets stored state. A restart at the wrong point in an outage can therefore lose the original "was online before outage" information or re-snapshot hosts after they have already shut down.

Our agent SHALL preserve the outage snapshot until the outage reaches a terminal state:

```text
NORMAL
ON_BATTERY
SHUTDOWN_IN_PROGRESS
WAITING_FOR_AC
RECOVERY_WAIT
RESTORE_HOSTS
NORMAL
```

The snapshot should only be cleared after successful completion or explicit administrative reset.

### 4. Simpler state machine than required

WOLNUT mainly handles recovery. `cockpit-ups-wol` additionally needs:

- ordered shutdown
- multiple shutdown methods (`nut`, `ssh`, `command`, `none`)
- controller-last shutdown
- network-readiness gating
- stable-AC timer resistant to power flapping
- ordered recovery/wake priorities
- Synology-specific integration
- explicit failure states and journald events

So the WOLNUT loop should be treated as a starting pattern, not the final agent architecture.

## Python vs Go decision

WOLNUT is small and readable, but importing it wholesale would introduce a Python runtime plus Click, PyYAML and `wakeonlan` dependencies.

The current `cockpit-ups-wol` goal is to keep the appliance small and provide architecture-specific prebuilt artifacts. Therefore the preferred implementation may still be Go for `wolctl` and possibly for the persistent agent.

The algorithm/state model can be reused independently of language.

## Assessment

**Recommended action: ADD TO THE HIGH-PRIORITY REUSE LIST.**

WOLNUT is the strongest existing reference for the **recovery-agent** portion of this project.

Recommended reuse:

- state model concepts
- atomic state-file strategy
- "restore only previously-online hosts" behavior
- configurable battery threshold
- retry and timeout concepts
- tests/scenarios around restart persistence

Do not reuse unchanged:

- fail-open UPS-state defaults
- 100% fallback for missing battery charge
- early state reset on restart
- simplistic outage/recovery state handling

If source is copied or substantially adapted, preserve the MIT notice and document the exact upstream files/commit in `THIRD_PARTY_NOTICES.md`.

---

# 2. deviationist/cockpit-upside

Repository: <https://github.com/deviationist/cockpit-upside>  
Language: TypeScript / React / Cockpit  
License: LGPL-2.1  
Status at review: active; recent commits in September 2026

UPSide is the strongest existing reference for the Cockpit/NUT frontend. It already demonstrates:

- PatternFly/Cockpit project structure
- `cockpit.spawn()` wrappers around `upsc`
- multiple UPS devices
- `upscmd` controls
- `upsrw` writable values
- capability-driven UI
- dangerous-command confirmation
- local/remote NUT sources
- NUT setup wizard
- `nut-scanner` based UPS detection
- standalone/network-server/network-client roles
- config preview and backup
- least-privilege NUT control users
- low-battery `upsmon` setup

Recommended reuse areas:

1. Cockpit project/build layout
2. NUT data parsing/normalization
3. `upsc`, `upsrw`, `upscmd` Cockpit wrappers
4. multi-UPS model
5. status rendering and capability detection
6. setup wizard patterns
7. safe configuration-write workflow
8. remote NUT handling

Do not put outage/recovery policy in the frontend. That belongs in `cockpit-ups-wol-agent`.

**Assessment: HIGH-PRIORITY frontend reuse candidate.**

---

# 3. cockpit-project/starter-kit

Repository: <https://github.com/cockpit-project/starter-kit>  
License: LGPL-2.1

Use as the authoritative foundation for:

- `manifest.json`
- React/PatternFly integration
- Cockpit JavaScript APIs
- build tooling
- development/testing layout
- packaging conventions

Where UPSide differs, use the starter kit for current Cockpit conventions and UPSide for NUT-specific implementation ideas.

**Assessment: use as frontend/build foundation.**

---

# 4. Trugamr/wol

Repository: <https://github.com/Trugamr/wol>  
Language: Go  
License: MIT

The `magicpacket` package cleanly implements the standard 102-byte WoL packet:

```text
6 × 0xFF
+
16 × target MAC
```

It separates packet construction, serialization, `io.Writer` output and UDP broadcast, making it easy to test.

It also handles both global and subnet-directed broadcast targets.

Recommended local use:

```text
wolctl/
└── magicpacket/
```

Add project-specific:

- host IDs
- interface/broadcast selection
- validation
- retry policy
- status verification
- structured exit codes/logging

**Assessment: strong direct-code-reuse candidate, with MIT attribution.**

---

# 5. networkupstools/nut

Repository: <https://github.com/networkupstools/nut>

This is the authoritative source for NUT behavior:

- protocol semantics
- UPS status tokens
- drivers
- `upsd`
- `upsmon`
- `upsc`
- `upsrw`
- `upscmd`
- `nut-scanner`
- primary/secondary behavior

Use NUT as an external system/runtime dependency instead of embedding or forking it.

Whenever another project disagrees with NUT syntax or behavior, official NUT behavior wins.

Because the repository does not present one simple repository-wide SPDX license, inspect file-level licensing before copying source.

**Assessment: essential runtime/specification reference; do not vendor by default.**

---

# 6. SuperioOne/nut_webgui

Repository: <https://github.com/SuperioOne/nut_webgui>  
Language: Rust  
License: Apache-2.0

Useful as a mature NUT UX/behavior reference for:

- variables
- writable variables
- instant commands
- authentication
- multi-UPS handling
- remote NUT
- error/state presentation

Do not import its web-server architecture because Cockpit already provides the management UI/server layer.

**Assessment: strong UX/behavior reference; selective reuse only.**

---

# 7. exelban/nutshell

Repository: <https://github.com/exelban/nutshell>  
Language: Go  
License: MIT

NutShell includes a small native NUT TCP client demonstrating:

- TCP connection to `upsd`
- `VER`
- `NETVER`
- `LIST UPS`
- authentication
- response framing
- timeouts

For v0.x, spawning the installed NUT CLI remains simpler and safer for compatibility.

If process spawning later becomes a measurable performance/problem area, NutShell is a good reference for implementing a native Go adapter.

Any local implementation should strengthen response parsing, reconnect logic, cancellation and least-privilege authentication handling.

**Assessment: high-value future native-client reference.**

---

# 8. seriousm4x/UpSnap

Repository: <https://github.com/seriousm4x/UpSnap>  
Language: Go + SvelteKit  
License: MIT

Useful reference areas:

- host/device UX
- wake controls
- online/offline state
- WoL configuration
- broadcast handling
- scheduling concepts
- error feedback

Do not import PocketBase or the separate web application/backend into this project.

**Assessment: UX/feature reference.**

---

# 9. geerlingguy/pi-nut

Repository: <https://github.com/geerlingguy/pi-nut>  
License: GPL-3.0

Useful for practical SBC/NUT deployment patterns:

- USB UPS configuration
- small-device UPS appliance setup
- safe shutdown configuration
- operational documentation

GPL code should not be copied unless the project's licensing strategy intentionally accepts the implications.

**Assessment: deployment/configuration reference.**

---

# 10. cockpit-project/cockpit

Repository: <https://github.com/cockpit-project/cockpit>

Authoritative reference for:

- `cockpit.spawn()`
- systemd/DBus integration
- privilege escalation
- journald
- page status
- PatternFly conventions
- Cockpit security model

Prefer official Cockpit APIs whenever they already provide a required facility.

Inspect individual file licenses before copying source.

**Assessment: authoritative platform reference.**

---

# 11. ahmetozer/wakeonlan

Repository: <https://github.com/ahmetozer/wakeonlan>  
Language: Go

Useful for comparing service/API and IPv4/IPv6 WoL implementation approaches.

GitHub currently reports no detected repository license. Source therefore should not be copied unless licensing is clarified.

**Assessment: reference only.**

---

# Recommended implementation strategy

## Phase 1 — Cockpit/NUT frontend

Use:

```text
cockpit-project/starter-kit
        +
deviationist/cockpit-upside
```

Reuse/adapt mature Cockpit/NUT UI and configuration patterns.

## Phase 2 — Persistent power/recovery agent

Use:

```text
hardwarehaven/wolnut
```

as the primary implementation reference for:

- persistent outage/client state
- online-before-outage snapshot
- recovery threshold
- delayed restore
- WoL retry/recovery monitoring

But implement the stricter project state machine and failure policy:

```text
NORMAL
  ↓
ON_BATTERY
  ↓
SHUTDOWN_IN_PROGRESS
  ↓
WAITING_FOR_AC
  ↓
RECOVERY_WAIT
  ↓
RESTORE_HOSTS
  ↓
NORMAL
```

Never interpret missing NUT data as `OL` or 100% battery.

## Phase 3 — WoL packet helper

Use/adapt:

```text
Trugamr/wol/magicpacket
```

for the low-level packet implementation.

## Phase 4 — Native NUT adapter, only if justified

Evaluate:

```text
exelban/nutshell
```

if repeated `upsc` process spawning becomes a measurable issue.

---

# Components that remain project-specific

Even with upstream reuse, these should be designed specifically for `cockpit-ups-wol`:

- ordered shutdown engine
- NUT/SSH/command shutdown adapters
- controller-last shutdown
- explicit crash-safe outage state machine
- stable-AC recovery timer
- 80% default battery recovery threshold
- recovery fallback when `battery.charge` is unavailable
- network-readiness gate
- ordered Wake-on-LAN restore sequence
- Synology compatibility preset
- Cockpit integration with the persistent agent
- single `install.sh` with `--tui` and `--silent`
- multi-distribution installer abstraction
- upgrade/rollback/config migration

---

# Dependency policy

Prefer this hierarchy:

```text
1. Standard Linux/NUT/Cockpit functionality
2. Small permissively licensed modules
3. Selective LGPL reuse when compatible
4. Larger applications as design references
5. GPL/unclear-license source only when licensing is intentionally accepted
```

This keeps the appliance lightweight and maintainable.

---

# License and attribution requirements

Before source-code reuse begins, add:

```text
LICENSE
THIRD_PARTY_NOTICES.md
```

For every copied or substantially adapted component record:

- upstream project
- upstream URL
- source path
- upstream commit/tag
- license
- copyright notice
- local destination
- modifications

For WOLNUT, if state/recovery code is adapted, the notice should identify the exact upstream files, likely including `wolnut/state.py` and/or relevant recovery logic from `wolnut/cli.py`, and state that the implementation was modified to use strict failure handling and the `cockpit-ups-wol` state machine.

---

# Current recommendation

The most efficient source strategy is now:

```text
Cockpit frontend
    └── cockpit-project/starter-kit
         + selected code/patterns from deviationist/cockpit-upside

UPS communication
    └── installed NUT CLI/services

Persistent recovery agent
    └── new cockpit-ups-wol implementation
         strongly informed by hardwarehaven/wolnut

WoL packet core
    └── selected MIT code from Trugamr/wol

Native NUT client
    └── defer; evaluate exelban/nutshell later
```

`hardwarehaven/wolnut` should therefore be treated as a **high-priority reference and selective-reuse candidate**, particularly for the persistent recovery-agent logic, while its fail-open UPS defaults and restart-state handling must not be copied unchanged.