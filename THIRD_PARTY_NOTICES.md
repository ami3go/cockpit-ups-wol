# Third-Party Notices

**Project license:** GNU AGPL-3.0-or-later  
**Status:** attribution/reuse register for the implemented `cockpit-ups-wol` codebase

This repository now contains a substantial project-authored Go/TypeScript/Shell implementation in addition to design and operating documentation. The projects below have been reviewed as behavioral, architecture, UX or test references. Review/reference does **not** mean their source has been copied into this repository.

At the time of this documentation reconciliation, no reviewed upstream application source listed below is recorded as copied/adapted into the project. Runtime/build dependencies installed through Go/npm/distribution package managers remain governed by their own licenses and package metadata.

This file must be updated **before** copied/adapted upstream source is merged.

## Reuse procedure

For every copied/adapted source component:

1. record the upstream project/repository;
2. record the exact source path;
3. record the exact upstream commit/tag;
4. record the license/SPDX identifier for the reused source;
5. verify compatibility with `AGPL-3.0-or-later` and any additional upstream obligations;
6. retain required copyright/license text;
7. record the local destination and modifications;
8. add the completed notice here before merge.

No-license/unclear-license source remains reference-only until permission is established.

## Runtime/platform dependencies

### Network UPS Tools (NUT)

Repository: https://github.com/networkupstools/nut  
Purpose: runtime UPS backend and authoritative NUT behavior reference  
Reuse model: use distribution NUT packages/services; do not vendor driver/source code by default  
License: component/file specific; review exact source before any copying

### Cockpit

Repository: https://github.com/cockpit-project/cockpit  
Purpose: management platform, authentication, bridge/service/journal APIs  
Reuse model: system/runtime dependency and API reference; no blanket source-copy permission is inferred

## Reviewed implementation/design references

| Project | Primary use in this project | Reported/reference license | Current source-reuse status |
|---|---|---|---|
| `hardwarehaven/wolnut` | persisted outage/recovery and prior-host-state reference | MIT | reference only; no source recorded as copied |
| `world-wide-dev/nutcracker` | deterministic shutdown/state-machine reference | MIT | reference only |
| `m4r1k/Eneru` | orchestration/testing/observability reference | MIT | reference only |
| `wijits36/hypercore-power-manager` | shutdown/recovery lifecycle reference | MIT | reference only |
| `ffind-dev/pve-ups` | Proxmox/dry-run/appliance reference | MIT | reference only |
| `deviationist/cockpit-upside` | Cockpit + NUT UI/config reference | LGPL-2.1 (re-check exact files before reuse) | reference only |
| `cockpit-project/starter-kit` | Cockpit frontend/build reference | LGPL-2.1 | reference only unless exact copied files are later recorded |
| `JuanCF/nutwatch` | NUT config/USB/WoL/event patterns | MIT | reference only |
| `rtorcato/homelab-nut` | Go/TUI/installer/inventory patterns | MIT | reference only |
| `riofutab/nut-server` | Go persistence/idempotency/systemd patterns | MIT | reference only |
| `Trugamr/wol` | compact WoL packet implementation reference | MIT | project has its own implementation; no copied source recorded |
| `exelban/nutshell` | possible future native Go NUT client | MIT | reference only |
| `ScottPierce/synology-ecoflow-nut` | Synology/NUT compatibility reference | GPL-2.0-or-later for project-authored files | reference only |
| `Brandawg93/PeaNUT` | NUT dashboard/API UX reference | Apache-2.0 | reference only |
| `SuperioOne/nut_webgui` | NUT dashboard/UX reference | Apache-2.0 | reference only |
| `seriousm4x/UpSnap` | WoL host-management UX reference | MIT | reference only |
| `geerlingguy/pi-nut` | SBC NUT deployment reference | GPL-3.0 | reference only |

The detailed research notes and links are maintained in `docs/RELATED_PROJECTS.md`.

## Companion test project

`ami3go/USB-UPS-Simulator` is a companion hardware/software test project for presenting a controllable USB HID UPS to NUT. It is useful for hardware-in-loop and fault-injection work, but it does **not** satisfy the real-UPS physical release gate by itself. No simulator source is recorded as copied into this repository.

Before any simulator code is copied/adapted here, record the exact source revision and license obligations using the same procedure above.

## Future incorporated-code entry format

```text
Component: <local package/path>
Upstream: <repository URL>
Upstream path: <path>
Upstream commit/tag: <sha/tag>
License: <SPDX>
Copyright: <upstream notice>
Local destination: <path>
Modifications: <summary>
License notice retained at: <path>
```
