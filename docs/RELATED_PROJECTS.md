# Related GitHub Projects and Code Reuse Assessment

**Project:** `cockpit-ups-wol`  
**Research date:** 2026-09-19  
**Purpose:** Identify related open-source projects that can reduce implementation effort, provide proven design patterns, or supply reusable code for Cockpit, NUT, Wake-on-LAN, installation, and power-management features.

## Summary

There is now one project that is especially close to the UPS/Cockpit part of `cockpit-ups-wol`:

- [`deviationist/cockpit-upside`](https://github.com/deviationist/cockpit-upside) — Cockpit plugin for NUT, including monitoring, `upscmd`, `upsrw`, setup wizard, remote NUT support and safe configuration editing.

For Wake-on-LAN, the strongest small reusable implementation remains:

- [`Trugamr/wol`](https://github.com/Trugamr/wol) — small Go implementation with a clean, testable magic-packet package under the MIT license.

For a possible future native NUT protocol client:

- [`exelban/nutshell`](https://github.com/exelban/nutshell) — compact Go NUT client under the MIT license.

The recommended approach is **selective reuse**, not combining whole applications. `cockpit-ups-wol` has requirements that these projects do not cover together: persistent outage state, ordered host shutdown, controller-last shutdown, automatic recovery after AC returns, waiting for UPS recharge to a configurable threshold (default 80%), WoL sequencing, and Synology-oriented deployment.

---

## Reuse decision matrix

| Project | Area | License | Reuse value | Recommended use |
|---|---|---:|---|---|
| [`deviationist/cockpit-upside`](https://github.com/deviationist/cockpit-upside) | Cockpit + NUT | LGPL-2.1 | **Very high** | Reuse/adapt Cockpit/NUT patterns and selected code if license strategy is compatible |
| [`cockpit-project/starter-kit`](https://github.com/cockpit-project/starter-kit) | Cockpit plugin foundation | LGPL-2.1 | **Very high** | Use as project/frontend build foundation |
| [`Trugamr/wol`](https://github.com/Trugamr/wol) | Wake-on-LAN | MIT | **Very high** | Reuse or adapt the small `magicpacket` package |
| [`networkupstools/nut`](https://github.com/networkupstools/nut) | UPS backend/protocol | Project-specific/mixed | **Essential reference** | Use installed NUT runtime and official behavior; avoid copying code without file-level license review |
| [`SuperioOne/nut_webgui`](https://github.com/SuperioOne/nut_webgui) | NUT web UI | Apache-2.0 | **High** | UX, capability and NUT-operation reference; selective compatible reuse possible |
| [`exelban/nutshell`](https://github.com/exelban/nutshell) | Native NUT Go client | MIT | **High, later** | Adapt if replacing `upsc` polling with direct NUT TCP protocol access |
| [`seriousm4x/UpSnap`](https://github.com/seriousm4x/UpSnap) | WoL management UI | MIT | **Medium/high** | Feature/UX reference; avoid importing its larger application architecture |
| [`geerlingguy/pi-nut`](https://github.com/geerlingguy/pi-nut) | NUT appliance/configuration | GPL-3.0 | **Medium** | Configuration/deployment reference; avoid direct code reuse unless GPL implications are accepted |
| [`cockpit-project/cockpit`](https://github.com/cockpit-project/cockpit) | Cockpit internals/API | Per-component/project licensing | **Essential reference** | API/security/systemd/PatternFly examples; check individual file licenses before copying |
| [`ahmetozer/wakeonlan`](https://github.com/ahmetozer/wakeonlan) | Go WoL service | No detected license | **Low for code reuse** | Architecture/API reference only unless permission/license is clarified |

---

# 1. deviationist/cockpit-upside

Repository: <https://github.com/deviationist/cockpit-upside>  
Language: TypeScript / React / Cockpit  
License: LGPL-2.1  
Status at review: active; recent commits in September 2026

## Why it is highly relevant

UPSide is a Cockpit extension specifically for Network UPS Tools. Its architecture closely matches the UPS-facing part of this project:

```text
Cockpit UI
    │
    ▼
cockpit.spawn()
    │
    ├── upsc
    ├── upsrw
    └── upscmd
    │
    ▼
NUT upsd
```

Its documented functionality includes:

- local and remote NUT sources
- multiple UPS devices
- PatternFly/Cockpit UI
- `upsc` polling through `cockpit.spawn()`
- `upscmd` instant commands
- `upsrw` writable variable editing
- capability-driven UI
- control-mode privilege separation
- dangerous-command confirmation
- NUT setup wizard
- `nut-scanner` based UPS detection
- standalone / network-server / network-client roles
- NUT configuration backup before modification
- low-battery `upsmon` configuration
- service/setup diagnostics
- historical data integration using PCP

## What should be reused

This is the strongest source for direct Cockpit-side reuse.

Candidate areas:

1. **Cockpit project structure and build system**
2. **NUT data parsing and normalization**
3. **`cockpit.spawn()` wrappers for `upsc`, `upsrw` and `upscmd`**
4. **Multi-UPS data model**
5. **UPS status rendering**
6. **Capability-driven controls**
7. **Risk grouping for UPS instant commands**
8. **PatternFly setup wizard patterns**
9. **NUT configuration preview/backup/write workflow**
10. **Remote NUT source support**
11. **Least-privilege control-user design**
12. **USB/NUT detection workflow using `nut-scanner`**

## What should not simply be copied

UPSide deliberately has no persistent power-management backend service. `cockpit-ups-wol` does require one because automatic recovery must continue without Cockpit and across controller reboots.

Do not adopt an architecture where Cockpit itself owns:

- outage state machine
- host shutdown sequencing
- persistent recovery state
- automatic 80% battery recovery threshold
- recovery sequencing
- WoL restoration policy

Those belong in `cockpit-ups-wol-agent`.

## Assessment

**Recommended action: HIGH-PRIORITY reuse candidate.**

Before implementing our UPS frontend, compare the starter-kit baseline against UPSide and selectively port mature UPS-specific components instead of recreating them.

If code is copied or substantially adapted, retain required LGPL notices and record the source in `THIRD_PARTY_NOTICES.md`.

---

# 2. cockpit-project/starter-kit

Repository: <https://github.com/cockpit-project/starter-kit>  
License: LGPL-2.1  
Status at review: actively maintained

The official Cockpit starter kit provides the preferred baseline for:

- `manifest.json`
- React integration
- PatternFly integration
- Cockpit JavaScript APIs
- build tooling
- test setup
- packaging patterns
- development installation

## Assessment

**Recommended action: use as the frontend/project foundation.**

Where UPSide already extends this starter kit with useful NUT-specific behavior, use UPSide as the specialized reference and the starter kit as the authoritative baseline for current Cockpit conventions.

---

# 3. Trugamr/wol

Repository: <https://github.com/Trugamr/wol>  
Language: Go  
License: MIT  
Status at review: active

This project contains a particularly clean package:

```text
magicpacket/
```

Its implementation builds the standard WoL packet as:

```text
6 × 0xFF
+
16 × target MAC address
```

for the standard 102-byte payload.

The package separates:

- packet construction
- byte serialization
- writing to an `io.Writer`
- UDP broadcast

That makes unit testing straightforward without requiring a real network interface.

It also supports both global and subnet-directed broadcast targets such as:

```text
255.255.255.255:9
192.168.1.255:9
```

which aligns well with the `cockpit-ups-wol` multi-network requirements.

## Recommended reuse

Reuse or adapt the small magic-packet implementation for `wolctl` rather than implementing WoL from scratch.

Suggested local structure:

```text
wolctl/
└── magicpacket/
    ├── packet.go
    └── packet_test.go
```

Extend it with project-specific features:

- configuration by host ID
- interface selection
- MAC validation
- IPv4 broadcast selection
- retry count
- optional host-state verification
- structured exit codes/logging

## License handling

MIT reuse is straightforward, but the original copyright/license notice must be retained.

Add attribution to `THIRD_PARTY_NOTICES.md` if code is incorporated.

## Assessment

**Recommended action: direct code reuse candidate.**

---

# 4. networkupstools/nut

Repository: <https://github.com/networkupstools/nut>  
Language: primarily C  
Project: authoritative Network UPS Tools implementation

This is the authoritative source for:

- NUT network protocol behavior
- status tokens
- drivers
- `upsd`
- `upsmon`
- `upsc`
- `upsrw`
- `upscmd`
- `nut-scanner`
- configuration semantics
- primary/secondary monitoring behavior

## Recommended use

Use NUT as an installed system dependency rather than embedding or forking it.

For v0.x:

```text
cockpit-ups-wol
    │
    ├── upsc
    ├── upsrw
    ├── upscmd
    └── upsmon / upsd configuration
```

The official source is the final behavioral reference whenever another project disagrees about NUT syntax or semantics.

## Licensing caution

GitHub does not represent the repository with a single SPDX license. NUT contains project/file-specific licensing history.

Therefore:

- using NUT executables as external runtime dependencies is preferred
- do not copy NUT source code into this repository without reviewing the specific source file's license

## Assessment

**Recommended action: essential runtime dependency and specification reference; do not vendor.**

---

# 5. SuperioOne/nut_webgui

Repository: <https://github.com/SuperioOne/nut_webgui>  
Language: Rust  
License: Apache-2.0  
Status at review: active

`nut_webgui` is useful for understanding how a mature dedicated NUT UI exposes:

- UPS state
- variables
- writable variables
- instant commands
- authentication
- multiple UPS devices
- remote NUT servers
- command/error presentation

## Recommended reuse

Primarily use it as a behavior and UX reference.

Potential selective reuse areas after license review:

- NUT response handling concepts
- variable categorization
- control flow around commands and writable values
- error/state presentation

Do not import its entire Rust web-server architecture because `cockpit-ups-wol` already has Cockpit as its management frontend and should avoid another HTTP server.

## Assessment

**Recommended action: strong NUT behavior/UX reference; selective reuse only.**

---

# 6. exelban/nutshell

Repository: <https://github.com/exelban/nutshell>  
Language: Go  
License: MIT

NutShell includes a compact native Go client for the NUT TCP protocol.

The client demonstrates:

- TCP connection to `upsd`
- `VER`
- `NETVER`
- `LIST UPS`
- command/response framing
- UPS discovery
- connection timeouts
- authentication

## Why it may matter later

The current architecture intentionally uses NUT command-line tools first because that is simpler and lets the distribution's NUT implementation own protocol compatibility.

If the persistent agent later needs higher-frequency polling or wants to avoid repeatedly spawning `upsc`, a native Go NUT client could become useful:

```text
cockpit-ups-wol-agent
       │
       ▼
 native NUT TCP client
       │
       ▼
      upsd
```

## Required adaptation

Do not copy the client unchanged.

The current implementation authenticates during every `Connect()`. NUT permits many read-only queries without authenticated control credentials depending on server configuration, and `cockpit-ups-wol` should preserve least privilege.

A project-specific client should therefore separate:

```text
Connect()
Authenticate()
Read operations
Privileged operations
```

It should also strengthen:

- response framing
- malformed response handling
- reconnect behavior
- context cancellation
- credential handling
- tests against different NUT versions

## Assessment

**Recommended action: future native NUT-client reference/adaptation; not required for v0.1.**

---

# 7. seriousm4x/UpSnap

Repository: <https://github.com/seriousm4x/UpSnap>  
Language: Go + SvelteKit  
License: MIT  
Status at review: very active and mature

UpSnap is a mature Wake-on-LAN management application.

Useful reference areas include:

- host/device model
- Wake buttons
- device online/offline state
- WoL configuration UX
- broadcast/network handling
- scheduling concepts
- host organization
- error feedback

## Why not use it as a dependency

Its application stack includes components unnecessary for this project, such as its own web application/backend and persistence architecture.

`cockpit-ups-wol` already has:

- Cockpit for UI/authentication
- YAML/project configuration
- a dedicated lightweight power agent
- no requirement for PocketBase or another web server

## Assessment

**Recommended action: UX and feature reference; do not embed the full application.**

---

# 8. geerlingguy/pi-nut

Repository: <https://github.com/geerlingguy/pi-nut>  
License: GPL-3.0

The project is focused on NUT deployment on a small Raspberry Pi acting as a UPS monitoring/server appliance, which is operationally close to the intended deployment of `cockpit-ups-wol` on SBCs.

Useful reference areas:

- small-device NUT installation
- practical NUT configuration
- USB UPS deployment
- safe server shutdown concepts
- appliance documentation

## Licensing consideration

GPL-3.0 code should not be copied into a differently licensed project without intentionally accepting the resulting licensing obligations.

Configuration concepts and operational lessons can still be studied and independently implemented.

## Assessment

**Recommended action: deployment/configuration reference, not preferred for direct code reuse.**

---

# 9. cockpit-project/cockpit

Repository: <https://github.com/cockpit-project/cockpit>

Cockpit itself is the authoritative reference for:

- `cockpit.spawn()`
- DBus/systemd integration
- privilege escalation
- navigation/status integration
- journald access
- PatternFly conventions
- file operations
- package/manifest behavior
- security model

## Recommended use

Prefer official Cockpit APIs over custom wrappers whenever the platform already provides the required capability.

Examples:

- systemd service status/control
- journal viewing
- administrative privilege prompts
- page status indicators

Because the repository contains many components and GitHub does not expose one repository-wide license in metadata, inspect the license/header of any individual file before copying code.

## Assessment

**Recommended action: authoritative API and implementation reference.**

---

# 10. ahmetozer/wakeonlan

Repository: <https://github.com/ahmetozer/wakeonlan>  
Language: Go

This is a small Go Wake-on-LAN service and can be useful for comparing:

- API design
- IPv4/IPv6 networking
- service layout
- Go WoL implementation approaches

However, GitHub currently reports no detected repository license.

Without a clear license, source code SHOULD NOT be copied into `cockpit-ups-wol`.

## Assessment

**Recommended action: reference only unless licensing is clarified.**

---

# Recommended reuse plan

## Phase 1 — Cockpit frontend

Start from:

```text
cockpit-project/starter-kit
        +
deviationist/cockpit-upside patterns/code
```

Reuse/adapt:

- project/build layout
- PatternFly UI patterns
- NUT polling wrappers
- NUT status parser
- UPS capability handling
- multi-UPS model
- setup wizard concepts
- safe command confirmation
- remote NUT source logic

Do not move automatic power policy into the Cockpit frontend.

---

## Phase 2 — WoL helper

Base the core packet implementation on:

```text
Trugamr/wol/magicpacket
```

Add project-specific CLI/configuration around it:

```text
wolctl list
wolctl wake HOST
wolctl status HOST
wolctl validate
```

Retain MIT attribution.

---

## Phase 3 — Persistent power agent

This component is largely project-specific.

Do **not** try to derive it from a generic WoL or NUT GUI project.

Implement independently around the architecture defined in `SOFTWARE_ARCHITECTURE.md`:

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

It should use standard NUT commands for v0.x.

Possible later optimization:

```text
exelban/nutshell-derived native NUT protocol client
```

only if process-spawn polling becomes a measurable problem.

---

# Components that should be written specifically for cockpit-ups-wol

The following are sufficiently project-specific that independent implementation is recommended:

- outage/recovery state machine
- atomic persistent state storage
- previous-host-state restoration policy
- shutdown priority/dependency engine
- recovery/WoL priority engine
- 80% recharge threshold and fallback policies
- stable-AC timer
- network-readiness gate
- controller-last shutdown behavior
- Synology compatibility preset integration
- installer (`install.sh`, `--tui`, `--silent`)
- multi-distribution package abstraction
- upgrade/rollback logic

---

# Dependency policy

Prefer this hierarchy:

```text
1. Use standard Linux/NUT/Cockpit functionality directly
2. Reuse small permissively licensed modules where they clearly reduce risk
3. Adapt LGPL code when project licensing is compatible
4. Use larger applications as design references
5. Avoid vendoring GPL or unclear-license code unless intentionally required
```

This keeps the final appliance small and reduces maintenance and supply-chain complexity.

---

# License and attribution requirements

Before the first source-code reuse commit, the project SHOULD add:

```text
LICENSE
THIRD_PARTY_NOTICES.md
```

`THIRD_PARTY_NOTICES.md` should record at minimum:

- upstream project
- upstream URL
- copied/adapted files
- upstream commit/tag
- license
- copyright notice
- local modifications

For example, if the WoL package is adapted:

```text
Component: Wake-on-LAN magic packet implementation
Source: https://github.com/Trugamr/wol
Upstream path: magicpacket/
License: MIT
Local use: wolctl/magicpacket/
Modification: integrated with cockpit-ups-wol host configuration and logging
```

The same process should be followed for any code imported from Cockpit starter-kit, UPSide, NutShell or other projects.

---

# Current recommendation

For the first implementation, the practical source strategy is:

```text
Cockpit frontend
    └── cockpit-project/starter-kit
         + selected ideas/code from deviationist/cockpit-upside

UPS communication
    └── installed NUT CLI/services

WoL packet core
    └── selected MIT code from Trugamr/wol

Power automation agent
    └── new cockpit-ups-wol implementation

Native NUT client
    └── defer; evaluate exelban/nutshell later
```

This provides the maximum useful reuse while keeping the architecture aligned with the project's main differentiator: reliable unattended shutdown and controlled automatic recovery of a small home/lab network.