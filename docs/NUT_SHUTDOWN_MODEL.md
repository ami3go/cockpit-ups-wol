# NUT Shutdown Ownership Model

**Status:** Normative v0.1 behavior  
**Applies to:** local UPS server mode, Synology/Linux NUT clients, FSD, controller shutdown and final UPS output handling

## 1. Goal

`cockpit-ups-wol` uses exactly one authority for each critical-shutdown stage. The agent decides **when** the project policy requires shutdown and handles accepted pre-FSD direct hosts, while NUT owns primary/secondary FSD propagation and the late system-shutdown/output path.

The agent must not compete with NUT by independently cutting UPS output while secondaries may still be running.

## 2. Local-UPS role assignment

For a UPS physically attached to the controller:

```text
Controller SBC
  ├─ NUT driver
  ├─ upsd
  ├─ upsmon PRIMARY
  └─ cockpit-ups-wol-agent

Synology / Linux NUT clients
  └─ upsmon SECONDARY
```

The controller is the only validated NUT primary for that local UPS.

Synology compatibility remains monitor-only:

```ini
[monuser]
    password = secret
    upsmon secondary
```

Legacy `slave` syntax is used only when required by the installed NUT version.

## 3. Ownership matrix

| Operation | Owner | Project agent behavior |
|---|---|---|
| read UPS variables/status | NUT driver/upsd | reads through NUT |
| decide project shutdown policy | `cockpit-ups-wol-agent` | owns decision |
| pre-FSD direct shutdown | agent host adapter | accepted v0.1 path: fixed-argv SSH only where configured |
| NUT-secondary shutdown | each secondary `upsmon`/OS | agent does not duplicate direct shutdown |
| set FSD | validated primary `upsmon` | agent may request `upsmon -c fsd` |
| primary/controller OS shutdown | primary `upsmon` / configured `SHUTDOWNCMD` | no normal bypass |
| final UPS driver shutdown/return | distro NUT/system shutdown integration | no long-running-agent direct call |
| arbitrary load-off/vendor commands | privileged/manual future/admin path | never automatic v0.1 behavior |

`shutdown.method: command` is not an accepted armed-v0.1 pre-FSD adapter. The schema retains it for future work, but armed validation fails closed until a durable allowlisted command model is implemented and accepted.

## 4. Critical shutdown sequence

```text
shutdown policy trigger
      ↓
persist SHUTDOWN_COMMITTED + fsync
      ↓
finish accepted pre-FSD direct SSH hosts
      ↓
request FSD through validated PRIMARY upsmon
      ↓
primary upsmon sets FSD in upsd
      ↓
NUT secondaries observe FSD/critical state and shut down
      ↓
primary waits for secondaries / HOSTSYNC bound
      ↓
primary applies FINALDELAY then SHUTDOWNCMD
      ↓
controller OS shuts down
      ↓
late NUT/system integration performs driver shutdown/return if supported
```

The long-running agent does not call `upsdrvctl shutdown` or substitute `load.off`/`shutdown.return` during ordinary runtime.

## 5. Commit/FSD semantics

`SHUTDOWN_COMMITTED` is persisted before the first destructive project side effect.

Before project commit/FSD, sufficiently reconciled restored utility may cancel an outage according to `docs/POWER_POLICY.md`. Once the committed transaction advances to FSD, the project does not attempt to undo FSD; it completes/reconciles the shutdown transaction and later recovers through the normal recovery gates.

## 6. Pre-FSD host ordering

v0.1 rules:

1. accepted `shutdown.method: ssh` hosts that must stop before the NUT wave are processed in deterministic configured priority order;
2. `shutdown.method: nut` hosts join the NUT-secondary FSD group;
3. `shutdown.method: none` receives no controller-issued shutdown;
4. armed `shutdown.method: command` is rejected;
5. the controller/NUT primary shuts down after the secondary synchronization path.

The UI/plan should distinguish:

```text
pre-FSD ordered SSH hosts
NUT-secondary shutdown group
controller/primary last
```

It must not promise exact ordering among NUT secondaries inside the same FSD wave.

## 7. HOSTSYNC and FINALDELAY

The canonical project configuration contains:

```yaml
nut:
  hosts_sync_seconds: 60
  final_delay_seconds: 15
```

The installer renders these values into project-managed `upsmon.conf` as NUT `HOSTSYNC` and `FINALDELAY`. Integration tests verify that the NUT configuration and project configuration do not silently drift.

These values are safety timing controls:

- `HOSTSYNC` bounds primary waiting for secondaries during critical shutdown;
- `FINALDELAY` delays the primary local shutdown command after synchronization.

Excessively large values can consume UPS runtime and are therefore range-validated.

## 8. Final UPS output handling

The normal automatic project path does not directly issue from the running agent:

```text
upscmd ... load.off
upscmd ... shutdown.return
upsdrvctl shutdown
```

Final output shutdown/return is delegated to the distribution-supported NUT/system shutdown integration after the OS is committed to shutdown.

## 9. UPS capability classification

```text
POWER_CYCLE_VERIFIED
POWER_CYCLE_UNVERIFIED
MONITOR_ONLY
```

`POWER_CYCLE_VERIFIED` requires real deployment evidence that the UPS/driver/output-return path behaves correctly. QEMU, NUT `dummy-ups`, or USB simulation cannot establish the electrical output-return behavior of a real UPS.

If output return is unverified or unavailable, unattended controller power-off requires another verified automatic restart mechanism or the controller must remain running according to policy.

## 10. Existing-NUT and remote-NUT safety

### Existing local NUT

The project preserves existing configuration and validates primary ownership before FSD. Because `upsmon -c fsd` is process-wide, configurations with multiple primary/master monitor entries are rejected for automatic FSD when ownership is ambiguous.

### Remote NUT server

Remote-client mode does not automatically become primary. It does not issue remote FSD/output-off in v0.1 unless a future explicitly validated authority model is implemented. The remote primary retains final UPS shutdown ownership.

## 11. Synology

For `shutdown.method: nut`:

- DSM receives UPS state from controller `upsd`;
- DSM owns its native shutdown after FSD/critical state;
- the agent does not send duplicate SSH shutdown;
- the NAS is part of the NUT-secondary group;
- the compatibility account remains monitor-only.

Real DSM behavior is still a physical release gate.

## 12. Failure handling

If an accepted pre-FSD SSH host cannot be reconciled/shut down after bounded attempts, the project does not silently advance to FSD; the unresolved action can drive `FAILED_SAFE` according to policy.

If FSD request fails after project shutdown commit, persist the failure, apply bounded recovery/retry for the validated primary path, surface a safety-critical error, and do not substitute an immediate direct UPS load-off.

Loss of NUT communication never clears a committed transaction or proves utility restoration.

## 13. Acceptance

Required software/integration coverage includes primary + secondary FSD generation, Synology monitor-only privileges, canonical HOSTSYNC/FINALDELAY rendering, existing-NUT multi-primary rejection, restored-power commit semantics and remote-client no-FSD behavior.

Real output shutdown/return and real DSM behavior are validated through `docs/HARDWARE_ACCEPTANCE.md`.

## 14. v0.1 decision

> The agent owns policy and accepted pre-FSD SSH orchestration; NUT primary/secondary semantics own FSD propagation and operating-system shutdown; the late NUT/system path owns final UPS output power handling.
