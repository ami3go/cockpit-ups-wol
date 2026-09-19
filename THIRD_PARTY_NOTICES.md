# Third-Party Notices

**Status:** Initial attribution register — no third-party source code has been copied into this repository at the time this file was created.

This file must be updated **before** any upstream code is copied or adapted.

## 1. Current status

The repository currently contains project-authored design/documentation only.

Related projects have been reviewed for patterns and potential reuse, but review/reference alone does not mean their source code has been incorporated.

## 2. Reuse procedure

Before copying/adapting upstream source:

1. record project/repository
2. record exact source path
3. record commit/tag
4. record license/SPDX identifier
5. confirm compatibility with this project's selected license
6. retain required copyright/license text
7. describe local modifications
8. add the entry below before merging copied code

## 3. Candidate upstreams

### Network UPS Tools (NUT)

Repository: https://github.com/networkupstools/nut  
Purpose: runtime dependency and authoritative behavior reference  
Reuse intent: use installed NUT packages/services; do not copy driver source by default  
License: project/file-specific; review exact files before copying

### hardwarehaven/wolnut

Repository: https://github.com/hardwarehaven/wolnut  
Purpose: persisted recovery/state design reference  
License: MIT  
Current status: no source copied

### world-wide-dev/nutcracker

Repository: https://github.com/world-wide-dev/nutcracker  
Purpose: deterministic shutdown/state-machine reference  
License: MIT  
Current status: no source copied

### m4r1k/Eneru

Repository: https://github.com/m4r1k/Eneru  
Purpose: orchestration/testing/observability reference  
License: MIT  
Current status: no source copied

### wijits36/hypercore-power-manager

Repository: https://github.com/wijits36/hypercore-power-manager  
Purpose: shutdown/recovery lifecycle reference  
License: MIT  
Current status: no source copied

### ffind-dev/pve-ups

Repository: https://github.com/ffind-dev/pve-ups  
Purpose: Proxmox shutdown/dry-run/appliance reference  
License: MIT  
Current status: no source copied

### deviationist/cockpit-upside

Repository: https://github.com/deviationist/cockpit-upside  
Purpose: Cockpit + NUT UI/config patterns  
License: LGPL-2.1 (repository metadata/reviewed source; re-check exact files before reuse)  
Current status: no source copied

### cockpit-project/starter-kit

Repository: https://github.com/cockpit-project/starter-kit  
Purpose: intended Cockpit frontend foundation  
License: LGPL-2.1  
Current status: not yet copied

### JuanCF/nutwatch

Repository: https://github.com/JuanCF/nutwatch  
Purpose: NUT config/USB/WoL/event patterns  
License: MIT  
Current status: no source copied

### rtorcato/homelab-nut

Repository: https://github.com/rtorcato/homelab-nut  
Purpose: Go/TUI/installer/inventory patterns  
License: MIT  
Current status: no source copied

### riofutab/nut-server

Repository: https://github.com/riofutab/nut-server  
Purpose: Go persistent orchestration/systemd-hardening patterns  
License: MIT  
Current status: no source copied

### Trugamr/wol

Repository: https://github.com/Trugamr/wol  
Purpose: possible Go Wake-on-LAN packet implementation reuse  
License: MIT  
Current status: no source copied

If code is adapted, record exact package/file/commit and retain the MIT notice.

### exelban/nutshell

Repository: https://github.com/exelban/nutshell  
Purpose: possible future native Go NUT client reference  
License: MIT  
Current status: no source copied

### ScottPierce/synology-ecoflow-nut

Repository: https://github.com/ScottPierce/synology-ecoflow-nut  
Purpose: Synology/NUT compatibility and security reference  
License: GPL-2.0-or-later for project-authored files  
Current status: reference only; no source copied

### Brandawg93/PeaNUT

Repository: https://github.com/Brandawg93/PeaNUT  
Purpose: NUT dashboard/API UX reference  
License: Apache-2.0  
Current status: no source copied

### SuperioOne/nut_webgui

Repository: https://github.com/SuperioOne/nut_webgui  
Purpose: NUT dashboard/UX reference  
License: Apache-2.0  
Current status: no source copied

### seriousm4x/UpSnap

Repository: https://github.com/seriousm4x/UpSnap  
Purpose: Wake-on-LAN host management UX reference  
License: MIT  
Current status: no source copied

### geerlingguy/pi-nut

Repository: https://github.com/geerlingguy/pi-nut  
Purpose: operational/deployment reference  
License: GPL-3.0  
Current status: reference only; no source copied

## 4. No-license sources

A repository/source without an explicit compatible license must remain reference-only. No source text is copied unless permission/license is established.

## 5. Future incorporated-code entry format

When source is actually incorporated, add an entry like:

```text
Component: <local package/path>
Upstream: <repository URL>
Upstream path: <path>
Upstream commit/tag: <sha/tag>
License: <SPDX>
Copyright: <upstream notice>
Modifications: <summary>
License notice retained at: <path>
```
