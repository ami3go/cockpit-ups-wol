# NUT Shutdown Ownership Model

**Status:** Normative v0.1 design  
**Applies to:** local UPS server mode, Synology/Linux NUT clients, FSD, controller shutdown and UPS output power-off

## 1. Goal

`cockpit-ups-wol` must have exactly one authority for each stage of a critical shutdown. The agent may decide **when** shutdown is required, but it must not compete with NUT's primary/secondary shutdown protocol or independently cut UPS output while clients may still be running.

This document defines ownership for v0.1.

## 2. Authoritative references

Current NUT documentation used for this design:

- `upsmon(8)`: https://networkupstools.org/docs/man/upsmon.html
- `upsmon.conf(5)`: https://networkupstools.org/docs/man/upsmon.conf.html
- `upsdrvctl(8)`: https://networkupstools.org/docs/man/upsdrvctl.html
- `upsdrvsvcctl(8)`: https://networkupstools.org/docs/man/upsdrvsvcctl.html
- `ups.conf(5)`: https://networkupstools.org/docs/man/ups.conf.html
- `upssched(8)`: https://networkupstools.org/docs/man/upssched.html

NUT documents that the primary `upsmon` sets FSD, waits for secondaries (bounded by `HOSTSYNC`), runs the local `SHUTDOWNCMD`, and leaves final UPS power-off to the system shutdown path/driver shutdown handling.

## 3. v0.1 local-UPS role assignment

For a UPS physically attached to the controller SBC:

```text
Controller SBC
  ├─ NUT driver
  ├─ upsd
  ├─ upsmon PRIMARY
  └─ cockpit-ups-wol-agent

Synology / Linux NUT clients
  └─ upsmon SECONDARY
```

The controller SBC SHALL be the only NUT primary for that locally attached UPS.

Synology compatibility remains monitor-only:

```ini
[monuser]
    password = secret
    upsmon secondary
```

Legacy installations may use the historical `slave` keyword when required by their NUT version.

## 4. Ownership matrix

| Operation | Owner | Agent allowed? |
|---|---|---:|
| read UPS variables/status | NUT driver/upsd | yes, through NUT |
| decide project shutdown policy | `cockpit-ups-wol-agent` | yes |
| pre-FSD shutdown of non-NUT managed hosts | agent host adapters | yes |
| set NUT FSD | primary `upsmon` | agent may request via `upsmon -c fsd` |
| notify NUT secondaries of FSD | `upsd` / primary `upsmon` protocol | no direct replacement |
| secondary local OS shutdown | each secondary `upsmon`/OS | no |
| primary/controller OS shutdown | primary `upsmon` via configured `SHUTDOWNCMD` | no direct bypass in normal flow |
| final UPS driver shutdown/power-cycle command | OS/NUT shutdown integration | no direct runtime call |
| arbitrary `load.off` / vendor instant commands | manual privileged admin path only | never automatic in v0.1 |

## 5. Critical shutdown sequence

For local NUT-primary mode the normal v0.1 sequence is:

```text
policy trigger reached
      ↓
agent persists SHUTDOWN_COMMITTED + fsync
      ↓
agent finishes required pre-FSD actions
(non-NUT SSH/command-managed hosts)
      ↓
agent asks PRIMARY upsmon to enter FSD
      ↓
primary upsmon sets FSD in upsd
      ↓
NUT secondaries observe critical/FSD and shut down
      ↓
primary upsmon waits for secondaries / HOSTSYNC bound
      ↓
primary upsmon runs controller SHUTDOWNCMD
      ↓
controller OS enters shutdown
      ↓
late NUT/OS shutdown integration issues driver shutdown
      ↓
UPS turns load off / schedules return if supported
```

The agent SHALL NOT call `upsdrvctl shutdown` while the normal writable filesystem/runtime stack is still active.

NUT documents `upsdrvctl shutdown` as a final shutdown-stage operation intended after the system is prepared to lose power.

## 6. FSD is the point of no return

NUT explicitly treats FSD as latched shutdown intent. Therefore:

- before the project calls `upsmon -c fsd`, restored utility may cancel an uncommitted outage after reconciliation
- once FSD has been requested, the project SHALL NOT attempt to "undo" FSD
- after FSD, the system proceeds through the committed shutdown/power-cycle path

This aligns project `SHUTDOWN_COMMITTED` with a one-way shutdown transaction.

## 7. Pre-FSD host ordering

NUT secondaries normally react to FSD as a group. Therefore exact individual shutdown ordering among NUT secondaries is not guaranteed by the controller.

v0.1 rules:

1. Hosts using `shutdown.method: ssh` or controlled `command` adapters that must stop before the NUT shutdown wave are handled before FSD.
2. Hosts using `shutdown.method: nut` join the NUT secondary shutdown wave when FSD is set.
3. The controller is the NUT primary and shuts down after secondary synchronization.

The UI SHALL clearly distinguish:

```text
pre-FSD ordered hosts
NUT-secondary shutdown group
controller/primary last
```

It SHALL NOT present a false exact ordering among NUT secondaries.

## 8. HOSTSYNC and FINALDELAY

`HOSTSYNC` and `FINALDELAY` are safety parameters, not arbitrary cosmetic delays.

- `HOSTSYNC` bounds how long the primary waits for secondaries to disconnect during critical shutdown.
- `FINALDELAY` delays the primary local shutdown command after synchronization.

The installer SHOULD use conservative values appropriate to the selected profile and SHALL expose them in advanced configuration.

The project SHALL NOT set excessively large values that risk battery exhaustion.

For devices with long shutdown procedures, use device/NUT-supported mechanisms where possible rather than assuming a larger primary delay always proves the remote filesystem is safe.

## 9. Final UPS output shutdown

The normal automatic project path SHALL NOT directly issue:

```text
upscmd ... load.off
upscmd ... shutdown.return
upsdrvctl shutdown
```

from the long-running agent.

Instead, the final UPS shutdown is delegated to the distribution's supported NUT/system shutdown integration.

Where systemd NUT driver instances are in use, the implementation SHALL follow current distro/NUT service integration rather than starting conflicting manual driver processes.

## 10. UPS capability validation

Automatic unattended controller power-cycle recovery depends on the UPS/driver being able to perform a safe shutdown/return sequence.

Installation/arming validation SHALL classify the local UPS profile as one of:

```text
POWER_CYCLE_VERIFIED
POWER_CYCLE_UNVERIFIED
MONITOR_ONLY
```

### POWER_CYCLE_VERIFIED

The UPS/driver shutdown-return behavior has been validated for the deployment. Full controller shutdown + automatic reboot recovery may be armed.

### POWER_CYCLE_UNVERIFIED

NUT monitoring works but UPS output-cycle behavior has not been confirmed. The system SHALL remain in dry-run or require an explicit administrator acknowledgement before relying on controller power-off for unattended recovery.

### MONITOR_ONLY

The UPS cannot reliably power-cycle the protected output through NUT. Automatic behavior SHALL NOT assume that a powered-off controller will later reboot merely because utility returns.

In this profile, controller shutdown/recovery policy must use an alternate verified mechanism or remain running.

## 11. Remote NUT server mode

When the controller monitors a UPS served by another NUT system:

- this project is not automatically the NUT primary
- it SHALL NOT issue FSD to the remote UPS unless explicitly configured and validated as the designated shutdown authority
- it SHALL NOT issue driver shutdown/output-off commands on a remote server in v0.1
- the remote NUT primary retains final UPS shutdown ownership

The agent may still use remote NUT status as input for local managed-host policy.

## 12. Synology behavior

For `shutdown.method: nut`:

- Synology receives UPS state from the controller's `upsd`
- Synology shuts down through DSM's native NUT client behavior
- the agent SHALL NOT send a duplicate SSH shutdown to the NAS
- Synology is considered part of the NUT-secondary shutdown group

The Synology monitor account must not receive SET, FSD, or unrestricted instant-command privileges.

## 13. Manual dangerous commands

Commands that can drop load or alter UPS shutdown state require:

- Cockpit authorization
- explicit user confirmation
- local privileged helper/agent authorization
- journald audit event
- rejection while they would conflict with an active power transaction unless the command is an explicit emergency action

They are never used by health autofix.

## 14. Failure handling

### FSD request fails

If `upsmon -c fsd` fails after `SHUTDOWN_COMMITTED`:

- persist the failure
- retry only according to bounded policy
- continue protecting already-requested host shutdown state
- surface a safety-critical error
- do not substitute a direct `load.off`

### Primary upsmon unavailable

Attempt bounded service recovery. If trusted primary shutdown ownership cannot be restored, enter `FAILED_SAFE` for automatic UPS-output actions.

### NUT communications lost before FSD

Normalize UPS state to `UNKNOWN`. Do not assume utility restoration.

### NUT communications lost after shutdown commit

Do not clear the committed transaction. Reconcile services/state and continue only through verified safe ownership paths.

## 15. Acceptance tests

At minimum test:

```text
primary + one secondary normal FSD
primary + multiple secondaries
secondary fails to disconnect before HOSTSYNC
power returns before FSD
power returns after FSD
agent restart before FSD
agent/controller interruption after FSD
primary upsmon unavailable
UPS driver unavailable
POWER_CYCLE_VERIFIED shutdown-return hardware test
remote NUT mode does not issue unauthorized FSD/output-off
Synology receives secondary shutdown without duplicate SSH action
```

## 16. v0.1 decision

The v0.1 safety model is deliberately conservative:

> **The agent owns policy and pre-FSD orchestration; NUT primary/secondary semantics own FSD propagation and operating-system shutdown; the late NUT/system shutdown path owns final UPS output power-off.**

This avoids competing shutdown authorities and preserves standard NUT behavior.
