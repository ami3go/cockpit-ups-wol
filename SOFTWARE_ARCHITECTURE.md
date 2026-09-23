# cockpit-ups-wol — Software Architecture

**Architecture version:** 0.4  
**Status:** Canonical v0.1 implementation baseline  
**Normative companions:** `docs/CONFIGURATION.md`, `docs/STATE_MODEL.md`, `docs/NUT_SHUTDOWN_MODEL.md`, `docs/POWER_POLICY.md`, `docs/RELIABILITY_REQUIREMENTS.md`, `docs/BOOT_RECOVERY_REQUIREMENTS.md`

## 1. Purpose

`cockpit-ups-wol` is a lightweight homelab/small-network power-management appliance built around Network UPS Tools (NUT), a persistent Go safety agent, Wake-on-LAN, systemd and Cockpit.

The system is designed to:

- monitor a local or remote UPS through NUT;
- provide NUT service to protected LAN clients;
- support Synology DSM as a NUT secondary/client;
- shut down managed systems safely during extended outage conditions;
- keep the controller available longest and let the validated NUT-primary path shut it down last when required;
- persist complete outage/recovery state across crash/reboot/power loss;
- recover only after utility, UPS recharge, network and health gates pass;
- restore only eligible previously-running hosts in deterministic order;
- remain safe if Cockpit/the browser is unavailable;
- apply configuration/installation changes transactionally with rollback.

## 2. Core safety principles

1. **Cockpit is not in the critical path.** Automatic outage/recovery continues without a browser session.
2. **NUT is authoritative for UPS communication.** The project does not reimplement UPS drivers.
3. **Unknown is not healthy.** Missing/failed UPS communication becomes `UNKNOWN`, never implicit `OL`/100% charge.
4. **Boot is not recovery.** Every start reconciles persisted state and requires fresh trustworthy evidence.
5. **Destructive intent is durable first.** Commit/action state is fsynced before external shutdown/FSD/WoL side effects.
6. **Configuration is transactional.** A candidate becomes known-good only after validation and probation.
7. **Failure converges safely.** Unrecoverable uncertainty enters durable `FAILED_SAFE`.
8. **The controller/network path is UPS-backed.** The coordinator must survive long enough to manage the event.
9. **Unsupported action types fail closed.** Schema representation does not imply accepted armed behavior.

## 3. High-level architecture

```text
                         Browser
                            │
                            ▼
                         Cockpit
                            │
                    cockpit-ups-wol UI
                            │
             privileged local CLI when needed
                            │
                            ▼
                  cockpit-ups-wolctl
                   config/report/log path
                            │
             ┌──────────────┴───────────────┐
             │                              │
             ▼                              ▼
   transactional config manager     cockpit-ups-wol-agent
             │                       persistent safety engine
             │                              │
             │                   ┌──────────┼──────────┐
             │                   ▼          ▼          ▼
             │                  NUT      state store   host/WoL
             │                   │          │          adapters
             │             local/remote     │
             └──────────► config history    │

Independent supervision:

systemd -> service restart/watchdog
cockpit-ups-wol-health.timer -> stack checks + bounded project repair
```

A small Unix-domain socket is used by the running agent for the current `GetHealth` IPC method. Broader configuration/reporting operations are intentionally handled by `cockpit-ups-wolctl`/the revision manager rather than pretending they are implemented agent IPC methods.

## 4. Implementation stack

```text
Agent / config / health helpers     Go
wolctl                              Go
Cockpit frontend                    TypeScript + React 19.3.0 + PatternFly 6.6.1
Installer                           Bash + systemd integration
UPS backend                         distribution NUT packages/services
Runtime service manager             systemd
Runtime logs                        journald
Configuration                       YAML + versioned schema/semantic validation
Agent IPC                            local AF_UNIX JSON request/response (current method: GetHealth)
Project license                     AGPL-3.0-or-later
```

Target release architectures:

```text
amd64
arm64
riscv64
```

## 5. NUT subsystem

NUT owns:

- hardware drivers;
- UPS variables/status/protocol;
- upsd client/server transport;
- standard primary/secondary `upsmon` synchronization;
- final system-shutdown/driver output handling.

The project uses installed NUT tools/services, including argv-safe reads such as `upsc`, and requests FSD only through the validated local primary path.

The long-running agent does not replace NUT's final shutdown sequence with direct `load.off`/driver-shutdown commands.

`docs/NUT_SHUTDOWN_MODEL.md` is authoritative for shutdown ownership.

## 6. Agent

`cockpit-ups-wol-agent.service` is the persistent safety engine.

Responsibilities:

- enter boot reconciliation on every start;
- load/reconcile last-known-good configuration and durable state;
- normalize NUT observations;
- execute the power policy/state machine;
- snapshot pre-outage host state;
- perform accepted pre-FSD direct shutdown;
- request FSD through validated ownership;
- persist transaction/per-host progress;
- evaluate recovery gates;
- stop/restart outage handling on power bounce;
- restore eligible managed hosts through durable WoL sequencing;
- publish runtime health through its current local health IPC;
- notify systemd watchdog/readiness state as implemented;
- journal important decisions/failures.

## 7. Canonical power state

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

Important transitions that change external-action eligibility are persisted.

`SHUTDOWN_COMMITTED` is durable before the first destructive shutdown/FSD side effect.

`RECOVERY_STARTED` is durable before the first managed-host wake side effect.

A power failure during `RESTORE_HOSTS` creates a new linked outage epoch and re-snapshots/re-evaluates already-restored hosts rather than reusing stale completion state.

## 8. Outage policy

Safety trigger order while on battery:

```text
FSD
low battery
critical runtime
critical battery charge
maximum time on battery
ordinary outage grace
```

Communication loss is bounded by configured grace and retains outage context. It never proves online utility.

Before shutdown commit, cancellation requires at least two consecutive trustworthy online observations and no latched safety condition.

A reboot during an existing outage does not provide a new grace period.

## 9. Recovery policy

Default:

```yaml
recovery:
  enabled: true
  utility_stable_seconds: 120
  battery_charge_min: 80
  network_wait_seconds: 300
```

Before automatic recovery starts, require:

1. `recovery.enabled`;
2. valid online NUT state;
3. continuous utility stability;
4. configured charge/runtime/recharge proof;
5. required network/dependencies ready;
6. known-good configuration;
7. acceptable runtime health;
8. no unresolved critical transaction state.

Fallback for unavailable charge data:

```text
battery charge -> runtime -> configured recharge time -> manual
```

Network/health remain gates for subsequent host restoration even after `RECOVERY_STARTED`.

## 10. Persistent state

State is stored as checksummed JSON generations with sequence numbers and current/previous fallback.

Critical write pattern:

```text
lock
-> validate current/previous
-> write temp
-> fsync temp
-> atomic rotation/rename
-> fsync directory
-> unlock
```

The implementation preserves the only valid previous generation until the new current is durable.

Per-host state tracks pre-outage eligibility, shutdown/recovery status and retry counts. Ambiguous interrupted actions are reconciled against real host state before retry.

## 11. Configuration manager

Main active config:

```text
/etc/cockpit-ups-wol/config.yaml
```

Revision history:

```text
/var/lib/cockpit-ups-wol/config-history/
```

Project-mediated changes follow:

```text
candidate
-> syntax/schema/semantic/safety validation
-> component preflight
-> atomic activation
-> affected service reload/restart
-> immediate health
-> probation
-> known-good OR rollback
```

`cockpit-ups-wolctl config-apply` starts activation/probation inside a transient systemd unit so browser/channel loss cannot abort it.

Startup reconciles interrupted config activation and active content/manifest mismatches before normal automation.

## 12. Host and dependency model

Accepted armed-v0.1 shutdown behavior:

```text
ssh
nut
none
```

- `ssh`: fixed argv, verified host key, bounded timeout/retry/reconciliation;
- `nut`: host participates in the NUT-secondary FSD group;
- `none`: no controller-issued direct shutdown.

The schema contains `command`, but armed v0.1 rejects it until a durable allowlisted command model exists.

Accepted deterministic armed host verification is TCP/ping (or an implemented adapter-specific check). ARP-only verification fails closed.

Managed-host WoL is implemented with durable attempt state, priority, retry and inter-host delay.

Network dependencies support `auto-power`/`wait-only` readiness in accepted v0.1. Dependency `wol` is schema-reserved but fails closed in armed mode until dependency actions gain durable state semantics.

## 13. NUT primary/secondary model

For a local UPS:

```text
controller = NUT primary
Synology/Linux NUT clients = secondaries
```

Accepted pre-FSD SSH hosts settle first. Then the agent requests FSD through validated primary `upsmon`. NUT owns secondary propagation, `HOSTSYNC`, `FINALDELAY`, controller `SHUTDOWNCMD`, and late driver shutdown/return.

Canonical project `hosts_sync_seconds`/`final_delay_seconds` are rendered into managed NUT configuration and regression-tested.

Existing-NUT mode rejects automatic FSD when multiple primary/master entries make process-wide ownership ambiguous.

## 14. Synology

Synology DSM is a first-class NUT-secondary profile.

Compatibility preset uses UPS name `ups`, TCP 3493 and monitor-only `monuser`/`secret` where required by DSM behavior. The account receives no actions/instcmds/FSD authority.

A Synology host using `shutdown.method: nut` is not also SSH-shut down.

Software integration is accepted; real DSM shutdown/recovery remains a physical release gate.

## 15. Health and autofix

Systemd restarts crashed/hung persistent services according to bounded policy. `cockpit-ups-wol-health` performs independent stack checks and conservative repair.

Safe repair may restart/re-enable project services, repair project-owned paths and restore known-good project config.

It does not rewrite unknown network/firewall policy, accept changed SSH host keys, issue destructive UPS commands, or wake/shut down hosts merely to improve health.

Repeated repair exhaustion enters durable `FAILED_SAFE`.

## 16. Local IPC/control

Current agent Unix-socket behavior:

```text
AF_UNIX
JSON request/response
1 MiB request bound
one request per connection
current method: GetHealth
```

The server does not currently expose generic mutating power/config methods and does not claim a completed peer-credential authorization framework for such methods.

Cockpit configuration management uses the root-gated `cockpit-ups-wolctl` path instead.

## 17. Installer and interrupted installation

One backend supports default, silent and TUI installation.

The installer:

- validates platform/architecture/systemd environment;
- installs required dependencies/artifacts;
- configures selected NUT profile/network policy;
- preserves authorized existing NUT state;
- enables required services;
- creates/validates initial candidate configuration;
- runs health probation;
- promotes known-good only after success;
- rolls back failed application/config/service/NUT/firewall changes.

A durable pending-install marker plus boot recovery guard protects against sudden power loss during installation/upgrade.

Real installation rejects environments where systemd is not PID 1 before transaction state is created.

## 18. Packaging and supply chain

Release outputs include multi-architecture appliance archives, Debian packages, prebuilt Cockpit assets, project license and SHA256 checksums.

GitHub Actions are pinned to immutable commit SHAs and monitored by Dependabot. npm dependencies are locked and installed with `npm ci`.

## 19. Physical topology

Controller and required LAN path are UPS-backed. Controller hardware must automatically boot when backed output returns. UPS output-return capability is classified:

```text
POWER_CYCLE_VERIFIED
POWER_CYCLE_UNVERIFIED
MONITOR_ONLY
```

Only real hardware testing can establish production UPS electrical shutdown/return behavior.

## 20. Accepted v0.1 boundary

The following are deliberately deferred rather than half-implemented:

```text
command shutdown adapter
ARP-only armed verification
dependency WoL
multi-UPS policy
Proxmox API adapter
native Go NUT protocol client
advanced writable UPS command UI
metrics/history/notification subsystem
```

## 21. Release gates

Software implementation and licensing are complete for the accepted feature set.

Public v0.1 still requires:

```text
real amd64 UPS acceptance
real arm64 UPS acceptance
real Synology DSM acceptance
main branch PR/green-CI protection (issue #39)
```

Use `docs/READINESS_AUDIT.md` for current release status and `docs/HARDWARE_ACCEPTANCE.md` for retained physical evidence requirements.
