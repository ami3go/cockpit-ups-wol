# Related GitHub Projects and Code-Reuse Assessment

**Original research:** 2026-09-19  
**Reconciled with implementation:** 2026-09-23  
**Project license:** AGPL-3.0-or-later

## Purpose

This document is a reference/reuse catalog for `cockpit-ups-wol`. The original research was performed before the v0.1 software baseline was fully implemented; many ideas that were previously recommendations are now implemented locally.

The current project already has its own Go safety agent, Cockpit UI, transactional installer/config manager, NUT/FSD ownership model, durable outage/recovery state and managed-host WoL recovery. Related projects are therefore used primarily for:

- behavioral cross-checking;
- test-case inspiration;
- future adapters/integrations;
- UX/operational patterns;
- carefully attributed source reuse if a later change genuinely benefits from it.

Reviewing a project does **not** mean its source code was copied. Exact source reuse must be recorded in `THIRD_PARTY_NOTICES.md` before merge.

## Current project baseline

Already implemented in `cockpit-ups-wol`:

```text
NUT local/remote/existing profiles
Synology monitor-only NUT-secondary integration
deterministic durable outage/recovery state machine
shutdown/recovery commit points
reboot/power-bounce reconciliation
communication-loss fail-safe handling
stable-AC + default 80% recovery gate
fixed-argv SSH shutdown
NUT-secondary FSD grouping/controller-last path
managed-host previous-state restore + WoL
transactional config + LKG rollback
bounded health/autofix + FAILED_SAFE
Cockpit management surface
transactional default/silent/TUI installer
multi-arch archive + Debian packaging
CI/QEMU/dummy-ups acceptance
```

Future-facing schema values such as command shutdown, ARP-only armed verification and dependency WoL remain intentionally fail-closed.

## Reference matrix

| Project | Main value | Language | License/reference status | Current use |
|---|---|---|---|---|
| `networkupstools/nut` | authoritative UPS/NUT backend | C/mixed | project/file specific | runtime dependency + authoritative behavior reference |
| `cockpit-project/cockpit` | Cockpit platform/security/API | mixed | component specific | runtime platform/API reference |
| `cockpit-project/starter-kit` | Cockpit frontend/build patterns | JS/TS | LGPL-2.1 | frontend/build reference |
| `hardwarehaven/wolnut` | previous-online snapshot + recovery/WoL | Python | MIT | recovery/state test/reference only |
| `world-wide-dev/nutcracker` | deterministic restart-safe shutdown state machine | Shell | MIT | state-machine/failure-handling reference |
| `m4r1k/Eneru` | broad shutdown orchestration/adapters/testing | Python | MIT | architecture/future adapter reference |
| `wijits36/hypercore-power-manager` | shutdown + recovery lifecycle | Python | MIT | commit/lifecycle reference |
| `ffind-dev/pve-ups` | Proxmox appliance/dry-run/ordering | Python | MIT | future Proxmox adapter reference |
| `deviationist/cockpit-upside` | Cockpit + NUT UI/config | TypeScript | LGPL-2.1 | specialized Cockpit/NUT UX reference |
| `JuanCF/nutwatch` | NUT config, USB detection, WoL/admin patterns | TS/Python/Shell | MIT | configuration/discovery/UX reference |
| `rtorcato/homelab-nut` | Go TUI/install/fleet workflow | Go | MIT | provisioning/TUI reference |
| `riofutab/nut-server` | durable Go orchestration/idempotency/systemd hardening | Go | MIT | implementation-pattern reference |
| `Trugamr/wol` | compact WoL packet implementation | Go | MIT | packet/test reference; project implementation remains local |
| `exelban/nutshell` | native Go NUT protocol client | Go | MIT | possible future optimization |
| `ScottPierce/synology-ecoflow-nut` | DSM/NUT compatibility | Shell/Docker | GPL-2.0-or-later project files | Synology behavior/security reference only |
| `Brandawg93/PeaNUT` | mature NUT dashboard/API | TypeScript | Apache-2.0 | secondary NUT UX/capability reference |
| `SuperioOne/nut_webgui` | NUT dashboard/control UX | Rust | Apache-2.0 | secondary UX reference |
| `seriousm4x/UpSnap` | WoL host-management UX | Go/SvelteKit | MIT | UX reference |
| `geerlingguy/pi-nut` | practical SBC NUT appliance deployment | Shell/config | GPL-3.0 | deployment reference only |
| `MarekWo/UPS_Server_Docker` | virtual/simulated NUT and recovery concepts | Python | MIT | simulation/client-status reference |

Repository URLs are the corresponding public GitHub owner/project names, for example `https://github.com/hardwarehaven/wolnut`.

## Companion test project: ami3go/USB-UPS-Simulator

Repository: https://github.com/ami3go/USB-UPS-Simulator

This is especially relevant to `cockpit-ups-wol` because it provides a controllable **USB HID UPS** target for NUT testing. The current reference design uses Arduino Leonardo/ATmega32U4 + W5500 and presents a HID Power Device to the system under test while providing a separate TCP/UART control plane.

Useful capabilities include:

- NUT `usbhid-ups` compatibility testing;
- deterministic USB UPS status/fault scenarios;
- separate Ethernet TCP control on port 5000;
- UART fallback control;
- arming/safe-state behavior;
- Python API/CLI and host self-test support;
- hardware-in-loop validation of USB enumeration/driver behavior.

### Intended use with cockpit-ups-wol

The simulator is a strong intermediate test layer between `dummy-ups` CI and destructive real-UPS acceptance:

```text
unit/simulation
    ↓
NUT dummy-ups/systemd E2E
    ↓
USB-UPS-Simulator hardware-in-loop
    ↓
real UPS physical acceptance
```

It can validate USB/NUT discovery, OB/OL/LB/FSD-style transitions and reboot/fault handling using real USB hardware interaction. It **cannot** prove a production UPS's battery runtime, electrical output shutdown/return, charger behavior, or load response, so it does not close issue #10 by itself.

No simulator source is currently recorded as copied into this repository. Any future copied code must carry exact revision/license attribution.

## Most useful references by subsystem

### Recovery / prior-host state

`hardwarehaven/wolnut` and `wijits36/hypercore-power-manager` remain useful cross-checks for remembering pre-outage state and restoration behavior. `cockpit-ups-wol` deliberately uses stricter `UNKNOWN` semantics and durable commit/reconciliation rules.

### Shutdown state machine

`world-wide-dev/nutcracker`, `m4r1k/Eneru` and `ffind-dev/pve-ups` remain useful references for staged shutdown, failure handling, dry-run planning and future hypervisor-aware adapters. The v0.1 runtime itself is independently implemented in Go.

### Cockpit/NUT UX

`deviationist/cockpit-upside`, official Cockpit projects, `JuanCF/nutwatch`, PeaNUT and `nut_webgui` remain UX/configuration references. Safety-critical outage/recovery decisions stay in the agent, not the browser.

### Go implementation patterns

`riofutab/nut-server`, `rtorcato/homelab-nut`, `Trugamr/wol` and `exelban/nutshell` are useful references for idempotency/persistence, provisioning, packet handling and potential future native NUT access. Existing local implementations should not be replaced merely for similarity.

### Synology

Official NUT behavior plus `ScottPierce/synology-ecoflow-nut` remain useful for DSM compatibility expectations. Real DSM acceptance must still be performed on actual hardware/version combinations.

## Reuse policy

Preferred approach:

1. use standard NUT/Cockpit/Linux interfaces first;
2. preserve project-authored code where it already satisfies the safety model;
3. reuse small permissively licensed components only when there is a concrete maintenance/test benefit;
4. re-check the exact upstream file/license/commit before copying;
5. keep LGPL/GPL/no-license projects reference-only unless the exact reuse decision and obligations are deliberately accepted;
6. record every copied/adapted component in `THIRD_PARTY_NOTICES.md` before merge.

Project-level AGPL licensing does not eliminate upstream attribution or file-specific obligations.

## Future opportunities informed by references

Potential later additions include:

```text
Proxmox API adapter
multi-UPS policy
safe command registry
durable dependency power/WoL actions
native Go NUT client
richer UPS writable/instant-command UI
event history / metrics / notifications
```

These are roadmap items, not missing claims in the accepted v0.1 baseline.

## Conclusion

The related-project search remains valuable, but the project is no longer at the "choose an architecture" stage. The right use of upstream work now is targeted validation and selective, attributed reuse while keeping `cockpit-ups-wol-agent` as the small safety-critical integration layer.
