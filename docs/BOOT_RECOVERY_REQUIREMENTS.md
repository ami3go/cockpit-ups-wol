# cockpit-ups-wol — Interrupted Boot and Power-Recovery Requirements

**Requirements version:** 1.0  
**Status:** Normative implementation requirement  
**Applies to:** controller boot, repeated power interruption, outage-state persistence, host restoration, configuration rollback and health recovery

## 1. Purpose

The controller may lose power while Linux, NUT, Cockpit, the power agent or the network are still starting. Utility power may also return briefly and fail again several times before becoming stable.

`cockpit-ups-wol` SHALL therefore treat boot as an unsafe transitional condition rather than proof that utility power has recovered.

A boot SHALL NEVER by itself trigger host restoration.

The system SHALL remain safe through repeated sequences such as:

```text
power returns
    ↓
controller begins boot
    ↓
power fails again
    ↓
controller loses power
    ↓
power returns
    ↓
controller boots again
    ↓
UPS/NUT becomes available
    ↓
AC remains stable
    ↓
UPS reaches recovery threshold
    ↓
managed hosts are restored
```

---

## 2. Boot-Reconciliation State

The power agent SHALL have an explicit startup state:

```text
BOOT_RECONCILE
```

Every agent start, including process restart and full controller reboot, SHALL enter `BOOT_RECONCILE` before entering any normal power state.

During `BOOT_RECONCILE`:

- no managed host SHALL be awakened automatically
- no power-recovery transaction SHALL be considered complete
- persisted outage/recovery state SHALL be loaded
- the active configuration revision SHALL be reconciled against `last-known-good`
- NUT status SHALL be considered `UNKNOWN` until a valid query succeeds
- service and configuration health SHALL be checked
- incomplete per-host actions SHALL be reconciled
- the agent SHALL determine the last safe power state before continuing

The agent SHALL leave `BOOT_RECONCILE` only after enough information exists to make a safe state transition.

---

## 3. Boot Is Not Evidence of Restored Utility Power

The fact that the SBC is powered and booting SHALL NOT be interpreted as:

```text
UPS = OL
utility stable = true
battery recovered = true
network ready = true
recovery allowed = true
```

The agent SHALL obtain valid UPS/NUT data before making these decisions.

If NUT cannot yet be queried, the effective UPS state SHALL be:

```text
UNKNOWN
```

`UNKNOWN` SHALL block automatic host restoration.

A failed NUT query SHALL NEVER fall back to an assumed `OL` state or an assumed battery percentage.

---

## 4. Boot During an Existing Outage

If the controller boots and valid NUT data reports:

```text
OB
OB LB
LB
FSD
```

or another status that indicates an active/unsafe outage, the agent SHALL resume the persisted outage transaction rather than beginning recovery.

Example:

```text
controller boots
      ↓
BOOT_RECONCILE
      ↓
NUT = OB
      ↓
load outage transaction
      ↓
resume ON_BATTERY / WAITING_FOR_AC
      ↓
DO NOT wake hosts
```

If battery/runtime conditions require the controller to shut down again, it MAY do so after persisting state and completing any safe required reconciliation.

---

## 5. Boot After Utility Returns

If the controller boots and NUT reports `OL`, that single observation SHALL NOT be sufficient to start recovery.

The normal recovery gates SHALL still apply:

```text
valid NUT data
      ↓
OL continuously stable
      ↓
configured utility-stable period
      ↓
UPS recovery threshold satisfied
      ↓
network ready
      ↓
configuration/health state safe
      ↓
RESTORE_HOSTS
```

Default recovery policy remains:

```text
utility stable period     120 seconds
minimum UPS charge        80%
network readiness wait    300 seconds
```

---

## 6. Stability Timer After Boot

The utility-stability timer SHALL restart after every controller boot unless continuous stability can be proven by a trusted source.

Default behavior SHALL be conservative:

```text
boot
  ↓
first valid OL observation
  ↓
start 120-second stability timer from zero
```

If any of the following occurs during that interval:

- `OB` is observed
- `LB` is observed
- NUT communication is lost long enough that stability cannot be proven
- the controller reboots again
- the UPS becomes unavailable

then the stability timer SHALL be invalidated and restarted only after valid stable `OL` data returns.

The system SHALL NOT persist a partially completed stability delay and blindly continue it after an uncontrolled reboot.

---

## 7. Repeated Power Interruptions During Boot

The design SHALL tolerate an unlimited number of interrupted boots without incorrectly restoring managed hosts.

Example:

```text
AC ON  → boot attempt 1 → AC OFF
AC ON  → boot attempt 2 → AC OFF
AC ON  → boot attempt 3 → NUT starts → OL
                           ↓
                    stability timer
                           ↓
                       AC remains ON
                           ↓
                     battery >= 80%
                           ↓
                        recovery
```

Every new boot SHALL re-enter `BOOT_RECONCILE`.

No count of successful Linux boots SHALL substitute for UPS state validation.

---

## 8. Durable Power Transaction State

An active outage/recovery transaction SHALL be stored persistently under:

```text
/var/lib/cockpit-ups-wol/
```

The persisted transaction SHOULD contain at least:

```yaml
transaction_id: outage-<unique-id>
state: RECOVERY_WAIT
sequence: 17
shutdown_committed: true
recovery_started: false
active_config_revision: cfg-...
hosts:
  nas:
    was_online: true
    shutdown_state: completed
    recovery_state: waiting
  proxmox:
    was_online: true
    shutdown_state: completed
    recovery_state: waiting
  desktop:
    was_online: false
    shutdown_state: not_required
    recovery_state: not_required
```

The transaction ID SHALL remain unchanged across controller reboots for the same outage/recovery cycle.

The `sequence` or equivalent monotonically increasing transaction field SHALL allow the implementation to distinguish newer state from stale state without relying only on wall-clock timestamps.

---

## 9. Atomic and Power-Loss-Safe State Writes

Critical state writes SHALL use a crash/power-loss resistant sequence where supported:

```text
write temporary file
      ↓
flush file contents
      ↓
fsync temporary file
      ↓
atomic rename
      ↓
fsync parent directory
```

The project SHALL NOT overwrite its only valid state copy in-place.

At least one previously valid state generation SHOULD be retained so a torn/corrupt newest state file can be detected and recovered.

State records SHOULD contain a checksum/hash.

If the newest state is invalid but the previous state is valid, the system MAY restore the previous generation and SHALL log that recovery.

If no trustworthy persistent state remains, the system SHALL enter a fail-safe state and SHALL NOT automatically wake managed hosts.

---

## 10. Configuration Transaction Interrupted by Power Loss

Power loss may occur while configuration is being changed or validated.

At boot the configuration manager SHALL inspect transaction metadata before starting normal automation.

Cases:

```text
active revision = known-good
    → validate and continue

active revision = candidate/validating
    → do NOT promote because of reboot
    → resume validation only when safe, or rollback to last-known-good

active revision missing/corrupt
    → restore last-known-good

last-known-good also invalid
    → FAILED_SAFE
```

A reboot or power interruption SHALL NEVER convert a candidate revision into `known-good` merely because the system subsequently booted.

If a configuration transaction was interrupted during an active UPS outage, restoring reliable power-management operation SHALL take precedence over completing the configuration change. The preferred default is rollback to the last-known-good revision.

---

## 11. Service Startup Ordering

Required runtime services SHALL be enabled for automatic startup.

Boot dependencies SHALL avoid assuming that the network or UPS is immediately ready.

Conceptually:

```text
local filesystems available
       ↓
NUT driver/server begin startup
       ↓
cockpit-ups-wol-agent starts in BOOT_RECONCILE
       ↓
network may become ready asynchronously
       ↓
NUT may become ready asynchronously
       ↓
agent reconciles and transitions when evidence is valid
```

The agent SHALL retry unavailable dependencies with bounded backoff rather than failing permanently because NUT, USB enumeration, DHCP or Ethernet is delayed during boot.

Cockpit availability SHALL NOT be a dependency for the power agent.

---

## 12. Boot Health Gate

Before automatic recovery may wake the first host, all mandatory boot health gates SHALL pass:

```text
✓ persistent state readable or safely reconstructed
✓ active configuration known-good
✓ agent healthy
✓ required NUT services healthy
✓ valid UPS status available
✓ utility stable for configured period
✓ recovery charge/runtime policy satisfied
✓ required network interface ready
✓ no critical configuration transaction unresolved
✓ no FAILED_SAFE condition
```

A non-critical Cockpit UI failure MAY leave the power engine operational, but SHALL be reported as degraded health.

---

## 13. Power Loss During Host Restoration

Power may fail after some hosts have already been restored.

Per-host recovery progress SHALL therefore be persisted after every significant action.

Example:

```text
NAS       online
Proxmox   WoL sent, not yet confirmed
Desktop   waiting
      ↓
power fails
      ↓
controller later reboots
      ↓
BOOT_RECONCILE
      ↓
NUT = OB
      ↓
re-enter outage handling
```

On a later stable recovery:

- a host already confirmed online SHALL be re-evaluated rather than blindly receiving another start command
- a host with only `wol_sent` SHALL be status-checked before retry
- WoL retries MAY occur because duplicate magic packets are normally harmless, but shall remain bounded
- a host whose original `was_online` value was false SHALL remain excluded unless `restore_policy: always` is configured

Recovery actions SHALL be idempotent wherever practical.

---

## 14. Power Loss During Managed Shutdown

Power may interrupt the controller while shutdown sequencing is active.

The agent SHALL persist per-host shutdown progress so the next boot can determine whether a shutdown request was:

```text
planned
requested
acknowledged
completed
unknown
```

After boot, an `unknown` action SHALL be reconciled by checking the target's real state before reissuing a potentially destructive command.

The implementation SHALL avoid blindly repeating non-idempotent commands solely because the previous acknowledgement was lost.

---

## 15. Shutdown Commit Point

Before destructive shutdown begins, the outage is reversible.

Once the system has crossed the shutdown commit point, that fact SHALL be persisted before the first destructive action.

Conceptually:

```text
ON_BATTERY
     ↓
shutdown trigger reached
     ↓
persist SHUTDOWN_COMMITTED
     ↓
fsync state
     ↓
first shutdown request
```

If power is lost immediately afterward, the next boot SHALL know that a shutdown transaction had begun.

This prevents a reboot from incorrectly returning the system to an ordinary `NORMAL` state while some managed systems may already be shut down.

---

## 16. Recovery Commit Point

Recovery SHALL also have a persisted transition before the first host is awakened:

```text
all recovery gates pass
     ↓
persist RECOVERY_STARTED
     ↓
fsync state
     ↓
wake first host
```

If the controller loses power after the first wake packet, the next boot can reconcile which recovery actions may already have happened.

---

## 17. Wall Clock Independence

Correctness SHALL NOT depend on wall-clock time being accurate immediately after boot.

Small SBCs may boot without RTC synchronization, and network time may not be available yet.

Therefore:

- wall-clock timestamps MAY be used for logs and human-readable revision IDs
- ordering of transaction state SHALL use transaction IDs, sequence numbers or equivalent durable metadata
- the post-boot utility-stability interval SHALL use a monotonic timer while the process is running
- an uncontrolled reboot SHALL invalidate any unfinished monotonic stability interval

An incorrect system clock SHALL NOT cause hosts to wake early.

---

## 18. Controller Hardware Power-On Requirement

For unattended recovery after the UPS has removed and later restored output power, the controller hardware SHALL be capable of starting automatically when power is applied.

Suitable SBCs normally satisfy this inherently.

Platforms requiring a manual power-button press after complete power loss SHALL require one of:

- firmware/BIOS `Restore on AC Power Loss = Power On`
- supported automatic power-on setting
- external hardware power-control solution

The installer/TUI SHOULD warn when automatic boot-after-power-return cannot be verified.

---

## 19. Filesystem and Storage Failure

If persistent state or configuration cannot be read reliably during boot, the system SHALL prefer safe inhibition over speculative recovery.

Examples:

```text
state checksum invalid
filesystem read-only unexpectedly
last-known-good revision unavailable
transaction metadata inconsistent
```

The resulting behavior SHALL be:

```text
monitor where possible
attempt bounded safe repair
restore previous verified state if available
otherwise enter FAILED_SAFE
DO NOT automatically wake hosts from uncertain state
```

The health supervisor SHALL surface the exact failure.

---

## 20. Boot Recovery State Flow

Recommended combined flow:

```text
POWER APPLIED
     ↓
Linux boot
     ↓
required services auto-start
     ↓
BOOT_RECONCILE
     ↓
load last-known-good configuration
     ↓
load/recover persistent power transaction
     ↓
validate NUT/service health
     ↓
             ┌──────────────────────┐
             │ valid UPS data?      │
             └──────┬───────────────┘
                    │ no
                    ▼
              UNKNOWN / wait
                    │
                    └──────────────┐
                                   │
                    yes            │
                    ▼              │
             ┌───────────────┐     │
             │ OB/LB/FSD ?   │     │
             └───┬───────┬───┘     │
                 │ yes   │ no       │
                 ▼       ▼          │
         resume outage   OL observed│
                         │          │
                         ▼          │
                   stable-AC timer  │
                         │          │
               interruption? ──────┘
                         │ no
                         ▼
                  battery >= 80%
                         ↓
                    network ready
                         ↓
                     health safe
                         ↓
               persist RECOVERY_STARTED
                         ↓
                 ordered host restore
                         ↓
                       NORMAL
```

---

## 21. Health Supervisor Behavior During Boot

The health supervisor SHALL understand startup grace periods and SHALL NOT treat expected boot-time dependency delays as corruption.

It SHALL distinguish:

```text
STARTING
WAITING_FOR_NUT
WAITING_FOR_UPS
WAITING_FOR_NETWORK
BOOT_RECONCILE
RECOVERING
DEGRADED
FAILED_SAFE
```

Autofix MAY restart a genuinely failed service, but SHALL not create a restart storm merely because USB enumeration, DHCP or NUT driver initialization is still progressing.

---

## 22. Acceptance Tests

Release testing SHALL include at least:

1. power loss while Linux is booting
2. power loss while NUT driver is starting
3. power loss before network becomes ready
4. repeated AC on/off cycles during boot
5. boot while UPS remains `OB`
6. boot after AC restore but before battery reaches 80%
7. AC failure during the 120-second stable-power timer
8. controller reboot during the stable-power timer
9. power loss during configuration candidate activation
10. power loss during configuration probation
11. power loss immediately after `SHUTDOWN_COMMITTED` is persisted
12. power loss during managed host shutdown
13. power loss immediately after `RECOVERY_STARTED` is persisted
14. power loss after one host is restored but before the next
15. corrupt newest state generation with valid previous generation
16. all state generations corrupt or missing
17. no RTC/network time available during boot
18. NUT communication unavailable during boot

In all cases the system SHALL either resume the correct persisted transaction or enter a safe inhibited state. It SHALL NOT incorrectly wake previously-off systems or treat unknown power state as healthy.

---

## 23. Acceptance Criteria

Interrupted-boot handling is complete only when:

```text
✓ boot never implies utility recovery
✓ every boot enters BOOT_RECONCILE
✓ UNKNOWN UPS state blocks automatic wake
✓ stable-power timer restarts after uncontrolled reboot
✓ repeated interrupted boots remain safe
✓ active outage/recovery transaction survives reboot
✓ shutdown commit is persisted before destructive shutdown
✓ recovery commit is persisted before first wake
✓ per-host action state is durable and reconciled after boot
✓ configuration candidate is never promoted merely because boot succeeded
✓ last-known-good configuration remains available after interrupted config changes
✓ corrupt newest state can fall back to a previous valid generation
✓ absence of trustworthy state results in FAILED_SAFE, not guessed recovery
✓ correctness does not depend on RTC/NTP availability
✓ automatic service startup and health supervision continue after every boot
✓ recovery still requires stable AC + configured battery threshold (default 80%) + network readiness
```
