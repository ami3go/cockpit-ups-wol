# Power Policy and Hysteresis

**Status:** Normative v0.1 behavior

## 1. Safety inputs

The agent normalizes NUT observations into at least:

```text
utility = ONLINE | ON_BATTERY | UNKNOWN
low_battery = true | false | unknown
fsd = true | false | unknown
battery_charge = percentage | unavailable
battery_runtime = seconds | unavailable
```

Missing/failed NUT queries become `UNKNOWN`/`unavailable`. They never imply online utility or full battery.

## 2. Shutdown trigger precedence

While on battery, critical triggers are evaluated in safety order:

1. observed/latched FSD;
2. low battery;
3. configured critical runtime;
4. configured critical battery percentage;
5. configured maximum time on battery;
6. ordinary outage grace/continued monitoring.

The first satisfied critical trigger commits shutdown. Project policy never overrides an observed FSD by deciding to keep systems running.

## 3. Communication loss

Communication loss produces `UNKNOWN` and retains prior outage context.

Before shutdown commit, communication uncertainty is tolerated only for the configured bounded `communication_loss_grace_seconds`. It cannot leave an outage indefinitely unresolved.

After shutdown commit, communication loss never uncommits the transaction.

## 4. Pre-commit utility cancellation

Before `SHUTDOWN_COMMITTED`, a pending outage may be cancelled only after reconciled valid online utility returns and no latched safety-critical condition remains.

v0.1 requires at least **two consecutive valid ONLINE observations** separated by the normal polling interval. A single transient `OL` sample does not cancel an outage.

## 5. Shutdown commit

Before the first destructive side effect:

```text
persist SHUTDOWN_COMMITTED
fsync durable state
then execute accepted external shutdown action
```

Once committed:

- restored utility does not erase the shutdown transaction;
- incomplete host actions are reconciled after restart;
- FSD, once requested, is not undone;
- the system proceeds through shutdown/wait/recovery reconciliation.

## 6. Reboot-safe outage duration

`max_on_battery_seconds` must remain meaningful across controller/agent restart. The implementation durably checkpoints outage progress and resumes conservatively without relying on a trustworthy wall clock.

A reboot during an already-active outage does not grant a fresh grace period.

## 7. Recovery entry gates

Automatic recovery requires all configured mandatory gates:

```text
recovery.enabled = true
valid ONLINE NUT state
continuous utility stability
UPS charge/runtime/recharge gate
required network/dependency readiness
known-good configuration
acceptable control-stack health
no unresolved critical transaction failure
```

Default utility-stability interval: 120 seconds.

`recovery.enabled: false` is a hard gate, including boot reconciliation of stale recovery state.

## 8. Utility-stability hysteresis

The stable-utility timer starts from zero after trustworthy ONLINE state is observed.

It is invalidated by:

```text
ON_BATTERY
LOW_BATTERY
FSD
loss of trustworthy UPS communication
controller/agent restart that loses monotonic continuity
```

An uncontrolled restart therefore requires the stability interval to be proven again from zero.

## 9. UPS recovery gate

Default:

```text
battery_charge_min = 80%
```

Fallback order:

```text
battery.charge
-> battery.runtime
-> configured recharge time
-> manual recovery
```

Missing values never silently pass.

## 10. Recovery commit

Before the first managed-host wake side effect:

```text
persist RECOVERY_STARTED
fsync durable state
then execute first recovery action
```

The charge/runtime threshold is an **entry** gate. A minor charge decrease after recovery starts does not reverse recovery by itself while utility and all other safety evidence remain trustworthy.

Network and critical controller health continue to gate new host restoration after recovery commit; passing the initial gate does not disable those protections.

## 11. Power failure during RECOVERY_WAIT

If utility fails before `RECOVERY_STARTED`:

- invalidate stable-utility/recovery proof;
- return to outage handling;
- send no wake actions;
- retain the durable outage transaction context.

## 12. Power failure during RESTORE_HOSTS

A renewed outage after some hosts have been restored is treated as a **new durable outage epoch** linked to the prior recovery transaction.

The agent:

1. persists the unsafe UPS observation;
2. stops issuing new wake actions;
3. creates/records a fresh outage transaction identity/snapshot for the renewed outage;
4. re-evaluates hosts already restored;
5. permits those now-online hosts to become shutdown targets again according to policy;
6. never assumes a `wol_sent` host is online/offline without verification.

This avoids the old failure mode where a host restored during partial recovery could be skipped during the next outage because its prior shutdown was already marked complete.

## 13. Repeated power bounce

Sequences such as:

```text
OL -> OB -> OL -> OB -> OL
```

must not cause duplicate transaction completion or premature wake. Every loss of trustworthy online evidence before recovery commit resets the stability gate; every renewed outage during partial restoration starts a fresh outage epoch.

## 14. Controller shutdown policy

The controller is last. In local NUT-primary mode, after the project commits the critical shutdown and pre-FSD direct hosts settle, primary `upsmon` owns FSD/secondary synchronization/controller shutdown timing.

If verified UPS output return or another verified automatic restart mechanism does not exist, policy must not assume a powered-off controller will later reboot unattended.

## 15. Host verification

A host is not declared shut down or recovered from one transient probe. Configured consecutive observations are required.

Accepted armed-v0.1 deterministic verification paths are TCP/ping or implemented adapter-specific checks. ARP-only verification is intentionally unsupported in armed v0.1 and fails closed.

Typical concept:

```text
success_consecutive = 3
probe_interval = 5 s
```

## 16. Ordering

Shutdown:

```text
lower numeric priority first
accepted pre-FSD SSH hosts
NUT secondary FSD group
controller/NUT primary last
```

Recovery:

```text
required dependencies ready
lower numeric managed-host wake priority first
configured inter-host delay/verification
```

Managed-host Wake-on-LAN is implemented. Dependency WoL is not accepted in armed v0.1.

## 17. Acceptance cases

Mandatory policy coverage includes:

```text
short OB then debounced OL before commit
OB until grace/critical threshold
OB + LB
FSD observation
runtime/charge/max-on-battery precedence
communication loss while normal/on-battery
reboot during active outage without fresh grace
OL after commit does not erase shutdown
power bounce during RECOVERY_WAIT
80% entry gate then small post-commit charge drop
network/health failure after RECOVERY_STARTED
power fail during RESTORE_HOSTS creates new outage epoch
already-restored host is eligible for shutdown in renewed outage
reboot during each major durable state
```
