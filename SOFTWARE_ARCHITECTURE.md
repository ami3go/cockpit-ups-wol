# cockpit-ups-wol — Software Architecture

**Architecture version:** 0.3  
**Status:** Canonical implementation baseline  
**Normative companions:** `docs/INSTALLATION_REQUIREMENTS.md`, `docs/RELIABILITY_REQUIREMENTS.md`, `docs/BOOT_RECOVERY_REQUIREMENTS.md`

## 1. Purpose

`cockpit-ups-wol` is a lightweight homelab and small-network power-management system built around Network UPS Tools (NUT), Cockpit and Wake-on-LAN.

The system SHALL:

- monitor a locally or remotely connected UPS through NUT
- provide NUT service to protected network devices
- support Synology DSM as a first-class NUT client
- safely shut down managed devices during an extended outage
- shut down the controller last when controller shutdown is required
- persist the complete outage/recovery transaction
- survive service crashes, controller reboot and repeated interrupted boots
- automatically recover after utility power is proven stable
- wait for UPS recharge, defaulting to **80%** when `battery.charge` is available
- restore only eligible devices in configured order
- provide management through Cockpit without depending on Cockpit for safety-critical behavior
- automatically start, health-check and recover required runtime services
- apply configuration transactionally and automatically roll back failed changes

## 2. Core safety principles

1. **Cockpit is not in the critical path.** The browser and Cockpit extension may be unavailable without disabling outage protection.
2. **NUT is authoritative for UPS communication.** This project does not reimplement UPS drivers.
3. **Unknown is not healthy.** Missing/invalid UPS data becomes `UNKNOWN`; it never silently becomes `OL` or 100% battery.
4. **Boot is not recovery.** Every agent start begins in `BOOT_RECONCILE` and a boot never proves utility stability.
5. **Destructive actions have durable commit points.** Shutdown/recovery intent is persisted and fsynced before the first external action.
6. **Configuration is transactional.** A candidate becomes known-good only after validation and runtime probation.
7. **Failure must converge safely.** Unrecoverable uncertainty enters `FAILED_SAFE`, inhibiting destructive automation.
8. **The controller remains available longest.** It is powered from UPS-backed power and shuts down after managed loads.

## 3. High-level architecture

```text
                         Browser
                            │
                            ▼
                         Cockpit
                            │
                            ▼
                  cockpit-ups-wol UI
                            │
                  local authenticated IPC
                            │
                            ▼
              cockpit-ups-wol-agent
                 persistent service
        ┌──────────────┼──────────────┐
        │              │              │
        ▼              ▼              ▼
      NUT        persistent state    wolctl
        │              │              │
   ┌────┴────┐         │              ▼
   │         │         │          LAN devices
Local UPS  Remote NUT  │
   │                   │
 USB/serial            │
                       ▼
                config revision store

Independent supervision:

systemd ──► process restart/watchdog
health timer ──► stack validation / bounded autofix / rollback
```

## 4. Implementation baseline

The v0.1 implementation SHALL use:

```text
Agent / config manager / health helper   Go
wolctl                                   Go
Cockpit frontend                         TypeScript + React + PatternFly
Installer                                Bash with distro modules
UPS backend                              distribution NUT packages/services
Runtime service manager                  systemd
Runtime logs                             journald
Configuration                            YAML, validated against a versioned schema
Agent IPC                                Unix-domain socket with local authorization boundary
```

Target release architectures:

```text
amd64
arm64
riscv64
```

Optional later target: `armhf`.

## 5. Main components

### 5.1 NUT

NUT owns:

- UPS hardware drivers
- UPS variables and status
- NUT client/server protocol
- standard `upsmon` primary/secondary synchronization
- UPS shutdown handoff at the end of a critical shutdown

Version 0.x uses installed NUT tools/services where practical, including:

```text
upsc
upsrw
upscmd
upsmon
upsd
upsdrvctl / distro service-aware equivalents
```

Exact shutdown ownership is defined in `docs/NUT_SHUTDOWN_MODEL.md`.

### 5.2 cockpit-ups-wol-agent

Systemd service:

```text
cockpit-ups-wol-agent.service
```

Responsibilities:

- enter `BOOT_RECONCILE` on every process start
- observe and normalize NUT state
- execute the power state machine
- snapshot pre-outage host state
- coordinate managed shutdown policy
- persist transaction and per-host action state
- resume safely after reboot
- evaluate recovery gates
- restore eligible hosts in configured order
- expose status/control over authenticated local IPC
- write decisions and failures to journald

The agent SHALL NOT infer successful UPS state from communication failure.

### 5.3 Configuration manager

Configuration changes from Cockpit, installer, CLI/TUI, migration or autofix SHALL pass through one transaction manager.

Required revision pointers:

```text
active
last-known-good
previous-known-good
```

Candidate lifecycle:

```text
candidate → validating → known-good
                    └──→ failed → rolled-back
```

The canonical user schema is defined in `docs/CONFIGURATION.md` and `schemas/config.schema.json`.

### 5.4 Health supervisor

Recommended units:

```text
cockpit-ups-wol-health.service
cockpit-ups-wol-health.timer
```

The health service is a short-lived check/repair process. It validates:

- required service enabled/active state
- agent heartbeat
- NUT availability appropriate to the selected profile
- active configuration integrity
- state-store integrity
- required runtime directories/permissions
- required network/helper dependencies

Safe repairs are bounded and observable. Repeated failure enters `FAILED_SAFE` rather than an endless restart loop.

### 5.5 wolctl

`wolctl` validates and sends Wake-on-LAN packets and may perform host status checks.

Per-host wake configuration supports:

```yaml
wake:
  enabled: true
  mac: "AA:BB:CC:DD:EE:FF"
  interface: eth0
  broadcast: 192.168.1.255
  port: 9
  priority: 20
  delay_after_previous_seconds: 30
```

### 5.6 Cockpit extension

Primary pages:

```text
Overview
UPS
Devices
Automation
Reliability
Settings
Logs
```

Cockpit SHALL expose status and configuration but SHALL NOT directly edit runtime state files or bypass the transaction manager.

## 6. Operating modes

The application SHALL support:

```text
monitor
 dry-run
 armed
 maintenance
```

Semantics:

- `monitor`: observe only; no managed shutdown or wake actions.
- `dry-run`: evaluate and record planned actions but do not execute external destructive/recovery actions.
- `armed`: normal automatic shutdown/recovery behavior.
- `maintenance`: inhibit automatic power actions while permitting explicit authorized diagnostics/manual operations.

A new installation SHALL default to `dry-run` until the administrator explicitly arms automation.

## 7. Canonical power state machine

```text
BOOT_RECONCILE
      │
      ├── unsafe/unknown ───────────────► FAILED_SAFE (when unrecoverable)
      │
      ▼
    NORMAL
      │ OB
      ▼
 ON_BATTERY
      │ shutdown trigger
      ▼
SHUTDOWN_COMMITTED
      │ persisted + fsynced
      ▼
SHUTDOWN_IN_PROGRESS
      │
      ▼
 WAITING_FOR_AC
      │ valid OL
      ▼
 RECOVERY_WAIT
      │ all recovery gates pass
      ▼
 RECOVERY_STARTED
      │ persisted + fsynced
      ▼
 RESTORE_HOSTS
      │ completed
      ▼
    NORMAL
```

Every state transition that changes external-action eligibility SHALL be persisted.

`SHUTDOWN_COMMITTED` is the point of no return for the current outage transaction. Before it, restored stable utility may cancel the pending outage. After it, the system SHALL reconcile/complete the committed shutdown transaction rather than pretending no shutdown started.

`RECOVERY_STARTED` is persisted before the first wake/recovery action.

## 8. UPS normalized states

At minimum the agent SHALL distinguish:

```text
ONLINE
ON_BATTERY
LOW_BATTERY
FORCED_SHUTDOWN
UNKNOWN
```

Raw NUT tokens including `OL`, `OB`, `LB`, `FSD`, `CHRG`, `DISCHRG`, `BYPASS`, `OVER`, `OFF` SHALL be retained for diagnostics.

Communication failure and missing status SHALL normalize to `UNKNOWN`.

## 9. Outage triggers and precedence

The canonical trigger policy is defined in `docs/CONFIGURATION.md`; the following safety precedence applies:

1. explicit/observed `FSD` → shutdown is committed immediately
2. `OB` + `LB` → shutdown is committed immediately unless already in a later state
3. configured critical runtime threshold while `OB` → commit shutdown
4. configured critical battery threshold while `OB` → commit shutdown
5. configured maximum time-on-battery while `OB` → commit shutdown
6. ordinary `OB` → remain in grace/monitoring until a trigger is reached
7. communication loss → `UNKNOWN`; never assume restored power

If utility returns before `SHUTDOWN_COMMITTED`, the outage may be cancelled after valid state reconciliation. If utility returns after commit, the current shutdown transaction remains committed.

## 10. Host state snapshot and per-host progress

Before managed shutdown begins, record each host's pre-outage eligibility and progress.

Typical durable fields:

```text
was_online
shutdown_state: planned/requested/acknowledged/completed/unknown/not_required
recovery_state: waiting/wol_sent/online/failed/not_required
retry counters
last verification result
```

Default restore policy:

```text
previous-state
```

Additional policies:

```text
always
never
```

## 11. Shutdown methods and ordering

Supported methods:

```text
nut
ssh
command
none
```

Lower numerical shutdown priority runs first. The controller uses the final/highest priority and remains operational as long as practical.

Arbitrary shell interpolation from Cockpit input is prohibited. `command` actions must resolve to configured/allowlisted helpers and arguments.

Host shutdown success SHOULD require multiple consistent observations rather than a single ping failure.

## 12. Synology DSM

Synology is a first-class NUT secondary/client profile.

Compatibility preset:

```text
UPS name: ups
NUT TCP port: 3493
monitor account: monuser
compatibility password: secret
role: upsmon secondary
```

Legacy NUT syntax may use `slave` where required.

The compatibility account SHALL NOT receive administrative `SET`, `FSD` or unrestricted instant-command permissions.

When a Synology host uses `shutdown.method: nut`, the agent SHALL NOT also SSH-shutdown it.

## 13. Controller power topology

The controller SBC SHALL be connected to a **battery-backed UPS output**, unless it has an equivalent independent backed power source.

It SHALL NOT be connected only to a surge-only/non-backed outlet.

The UPS data link may simultaneously be USB/serial/network:

```text
UPS backed output ──► SBC PSU
UPS USB/serial    ──► SBC/NUT driver
```

Network infrastructure needed for shutdown coordination (at minimum the required switch, and router/VLAN infrastructure where needed for local reachability) SHALL remain powered long enough for shutdown coordination.

Controller hardware SHALL automatically boot when backed output power returns. SBCs normally satisfy this; PC-class hardware must use firmware settings such as `Restore on AC Power Loss = Power On`.

## 14. Controller shutdown policy

Managed heavy loads shut down before the controller.

The controller SHOULD remain running after other hosts are down while battery/runtime remains sufficient because its load is normally small and it is the recovery coordinator.

Controller shutdown may be triggered by a dedicated late threshold such as:

```text
NUT LB/FSD
critical runtime
controller-specific battery threshold
```

The exact policy is configurable. Before controller poweroff, state and configuration transaction metadata SHALL be durably synchronized.

## 15. Recovery gates

Default recovery policy:

```yaml
recovery:
  enabled: true
  utility_stable_seconds: 120
  battery_charge_min: 80
  network_wait_seconds: 300
```

Before `RECOVERY_STARTED`, require:

1. valid NUT data
2. utility state proven `OL`
3. continuous utility stability for the configured interval
4. battery/recovery policy satisfied
5. required network ready
6. active configuration known-good
7. stack health not safety-critical
8. no unresolved power/config transaction

The stability timer uses monotonic time and restarts after every uncontrolled reboot or loss of trustworthy `OL` evidence.

## 16. Battery recovery fallback

Preferred hierarchy:

```text
battery.charge available → require configured percentage (default 80%)
else battery.runtime available → require configured runtime threshold
else configured recharge time → wait configured interval while valid OL persists
else → manual recovery
```

The agent SHALL never substitute an assumed 100% value.

Once recovery is committed, a minor charge decrease (for example 80% → 79% after loads start) does not itself reverse recovery. A real unsafe condition such as `OB`, `LB`, `FSD`, or loss of trustworthy UPS state causes reconciliation/outage handling.

## 17. Network readiness and dependencies

Recovery readiness may check:

- link carrier
- address assignment
- required route
- gateway or configured management target reachability
- required switch/router dependency readiness

A dependency can be marked `auto-power`/`wait-only` when it cannot be awakened by WoL. Recovery SHALL wait for dependencies in configured order rather than assuming every device has WoL.

## 18. Wake ordering

Lower wake priority starts first. Recovery should avoid simultaneous inrush/load surge.

Example:

```text
network dependencies ready
        ↓
NAS
        ↓
Proxmox
        ↓
application servers
        ↓
workstations
```

Per-host wake attempts are bounded and persisted.

## 19. NUT network policy

Default:

```text
trusted-lan
```

NUT may listen on TCP 3493 for protected LAN clients. It SHALL NOT intentionally be exposed to the public Internet.

Optional:

```text
restricted
```

Restricted mode must handle every enabled address family. An IPv4-only restriction SHALL NOT leave an unrestricted IPv6 listener.

Unknown existing firewall configuration SHALL not be destructively replaced.

## 20. Privilege separation and secrets

Network NUT clients use monitor-only credentials. Administrative UPS actions require a separate local privileged path and explicit authorization/confirmation.

Secrets such as SSH keys and administrative NUT credentials SHALL use dedicated root/service-readable files with minimal permissions and SHALL be redacted from logs and revision diagnostics.

The Synology compatibility credential is only created when that compatibility mode is explicitly selected.

## 21. Persistent configuration

Main user configuration:

```text
/etc/cockpit-ups-wol/config.yaml
```

State and config history:

```text
/var/lib/cockpit-ups-wol/
├── state/
└── config-history/
```

Configuration contains a schema version and is validated before activation.

Critical writes use the power-loss-resistant sequence:

```text
write temporary file
→ flush
→ fsync file
→ atomic rename
→ fsync parent directory
```

## 22. Config transaction and rollback

Every project-mediated change:

```text
lock
→ create candidate revision
→ static schema validation
→ component preflight
→ atomic activation
→ reload/restart affected services
→ immediate health validation
→ probation
→ promote to known-good OR rollback
```

A reboot never promotes a candidate merely because the machine booted successfully.

## 23. systemd runtime

Typical local-server profile:

```text
cockpit.socket
NUT driver service instance(s)
nut-server.service
nut-monitor.service
cockpit-ups-wol-agent.service
cockpit-ups-wol-health.timer
```

Exact NUT service names vary by distribution and are detected by installer modules.

Persistent services SHALL be automatically enabled and configured for bounded recovery. Delayed USB/network/NUT readiness is handled with retry/backoff, not a fragile assumption that all dependencies are immediately ready at boot.

## 24. Health states

Cockpit/CLI SHALL expose:

```text
HEALTHY
DEGRADED
RECOVERING
CONFIG_VALIDATING
ROLLING_BACK
FAILED_SAFE
```

`FAILED_SAFE` inhibits destructive automatic actions that depend on uncertain state while preserving monitoring where practical.

## 25. Local IPC

Cockpit communicates with the agent through a root-controlled Unix-domain socket, for example:

```text
/run/cockpit-ups-wol/agent.sock
```

Detailed request/authorization semantics are defined in `docs/IPC.md`.

The UI SHALL NOT mutate state or configuration files directly.

## 26. Logging and observability

Runtime logging uses journald. Important events include:

- service startup/restart/autofix
- config candidate/validation/rollback
- utility loss/restore
- UPS communication failure/recovery
- power-state transitions
- host snapshot
- shutdown/recovery commands and verification
- controller shutdown
- interrupted-boot reconciliation
- state corruption/fallback
- entry into `FAILED_SAFE`

## 27. Simulation and acceptance testing

The implementation SHALL support non-destructive simulation for at least:

```text
on-battery
low-battery
FSD
communication loss
power restored
power bounce
battery threshold
network delay
controller reboot/interrupted boot
interrupted shutdown
interrupted recovery
config validation failure/rollback
```

New installations default to `dry-run`; simulation never performs real destructive operations unless explicitly authorized by a dedicated test path.

## 28. Repository layout target

```text
cockpit-ups-wol/
├── README.md
├── SOFTWARE_ARCHITECTURE.md
├── agent/
│   ├── cmd/
│   └── internal/
│       ├── config/
│       ├── health/
│       ├── host/
│       ├── ipc/
│       ├── nut/
│       ├── policy/
│       └── state/
├── wolctl/
├── cockpit/
├── config/
├── schemas/
├── packaging/systemd/
├── scripts/
├── docs/
├── tests/
└── .github/workflows/
```

## 29. Architectural acceptance scenario

A functional v0.1 SHALL complete this without a browser open:

```text
1. Controller and required network path are UPS-backed.
2. Utility fails; NUT reports OB.
3. Grace/trigger policy is evaluated.
4. Pre-outage host state is durably recorded.
5. SHUTDOWN_COMMITTED is persisted before the first destructive action.
6. Managed hosts shut down in configured order; Synology uses NUT.
7. Controller remains last and may shut down only at its late threshold.
8. Utility returns and controller auto-boots if it had powered off.
9. Agent enters BOOT_RECONCILE and loads persisted transaction/config state.
10. Valid OL is observed continuously for the stability interval.
11. Network/dependency readiness is proven.
12. UPS battery reaches the recovery gate (80% by default).
13. RECOVERY_STARTED is persisted before the first wake action.
14. Previously eligible hosts are restored in configured order.
15. Previously-off hosts remain off unless configured `always`.
16. Per-host progress survives any reboot/power interruption.
17. Successful completion returns the system to NORMAL/HEALTHY.
```

Cockpit SHALL be able to inspect and manage this process but is never required for its safety-critical completion.
