# Power Policy and Hysteresis

**Status:** Normative v0.1 behavior

## 1. Purpose

This document defines exact precedence for UPS events, shutdown triggers, cancellation, recovery gating and power-bounce handling.

## 2. Normalized UPS inputs

The agent SHALL normalize NUT observations into these safety inputs:

```text
utility = ONLINE | ON_BATTERY | UNKNOWN
low_battery = true | false | unknown
fsd = true | false | unknown
battery_charge = percentage | unavailable
battery_runtime = seconds | unavailable
```

Missing or failed NUT queries become `UNKNOWN`/`unavailable`. They never imply `ONLINE` or full battery.

## 3. Shutdown trigger precedence

When `utility = ON_BATTERY`, evaluate triggers in this order:

1. `fsd == true`
2. `low_battery == true`
3. runtime <= configured critical runtime
4. battery charge <= configured critical charge
5. time-on-battery >= configured maximum
6. grace timer / continued monitoring

The first satisfied critical trigger commits shutdown.

A project-managed policy SHALL NOT override an observed FSD by deciding to keep systems running.

## 4. Communication failure

If UPS communication becomes unavailable:

```text
ONLINE      + comm loss → UNKNOWN
ON_BATTERY  + comm loss → UNKNOWN with prior-outage context retained
```

Communication loss SHALL NOT reset an outage transaction.

Before shutdown commit, the policy may wait/retry for a bounded period according to configuration. After shutdown commit, communication loss never uncommits shutdown.

## 5. Pre-commit cancellation

Before `SHUTDOWN_COMMITTED`, a pending outage can be cancelled only when:

```text
valid ONLINE state returns
AND
no FSD is latched
AND
no safety-critical condition remains
```

The agent then returns to `NORMAL` after reconciliation.

A single transient `OL` sample is sufficient to cancel only if the system has not crossed the commit point and the configured debounce policy permits it. For v0.1, use at least two consecutive valid ONLINE samples separated by one normal polling interval.

## 6. Shutdown commit semantics

Before the first destructive action:

```text
persist SHUTDOWN_COMMITTED
fsync state
then execute external shutdown action
```

Once committed:

- restored utility does not roll back the transaction
- FSD, once requested, is not undone
- incomplete host actions are reconciled on restart
- the project proceeds to the safe shutdown/recovery cycle

## 7. Recovery entry

Recovery can begin only from a committed or completed outage transaction when all required gates pass:

```text
valid ONLINE UPS state
continuous utility stability
battery/recharge gate
network/dependency readiness
known-good configuration
healthy enough control stack
no unresolved critical transaction error
```

Default utility-stability interval is 120 seconds.

## 8. Utility-stability hysteresis

The recovery stability timer starts from zero after the first trustworthy ONLINE observation.

It is invalidated by:

```text
ON_BATTERY
LOW_BATTERY
FSD
UPS communication loss long enough to lose proof of continuity
controller reboot
agent restart that loses monotonic continuity
```

After invalidation, the timer starts again only after trustworthy ONLINE data resumes.

## 9. Battery recovery gate

Default:

```text
battery_charge_min = 80%
```

Fallback hierarchy:

```text
battery.charge
→ battery.runtime
→ configured recharge time
→ manual recovery
```

No missing value may silently pass the gate.

## 10. Recovery commit and charge hysteresis

Before waking the first managed host:

```text
persist RECOVERY_STARTED
fsync state
then execute first recovery action
```

The battery threshold gates **entry** into recovery.

After `RECOVERY_STARTED`, a small battery drop caused by restored load does not reverse recovery by itself.

Example:

```text
battery reaches 80%
RECOVERY_STARTED committed
NAS wakes
battery falls to 79%
utility still ONLINE and otherwise healthy
→ continue recovery
```

Recovery is interrupted/reconciled only by a real unsafe condition, including:

```text
ON_BATTERY
LOW_BATTERY
FSD
loss of trustworthy UPS state
critical controller health failure
```

## 11. Power failure during RECOVERY_WAIT

If utility fails before `RECOVERY_STARTED`:

```text
invalidate recovery gates
return to ON_BATTERY / outage handling
send no wake actions
```

The original outage transaction remains active.

## 12. Power failure during RESTORE_HOSTS

If utility fails after recovery has started:

1. persist the new unsafe UPS observation
2. stop issuing new wake actions
3. re-evaluate hosts already recovered
4. re-enter outage handling using the same transaction lineage or a linked new outage epoch
5. never assume a `wol_sent` host is either online or offline without checking

Hosts already online may require another shutdown according to policy.

## 13. Repeated power bounce

Any number of sequences like:

```text
OL → OB → OL → OB → OL
```

must not produce duplicate transaction completion or premature wake.

Before recovery commit, every loss of trustworthy ONLINE resets the stable-utility gate.

## 14. Controller-specific shutdown

The controller is last.

For the v0.1 NUT-primary model, once the full critical shutdown transaction reaches FSD, primary `upsmon` owns controller shutdown timing.

For profiles where no validated UPS output power-cycle exists, the controller SHALL NOT power off automatically unless another verified restart mechanism exists.

## 15. Host verification hysteresis

A host SHALL NOT be declared shut down or recovered from one transient probe.

Default verification concept:

```text
success_consecutive = 3
probe_interval = 5 s
```

Shutdown success requires 3 consecutive offline results after a shutdown request.
Wake success requires 3 consecutive online results after a wake request.

Methods may be `tcp`, `ping`, `arp`, or an adapter-specific check. `tcp`/adapter-specific checks are preferred where available.

## 16. Priority interaction

Shutdown:

```text
lower numeric priority first
pre-FSD ordered adapters first
NUT secondaries as FSD group
controller primary last
```

Recovery:

```text
lower numeric wake priority first
wait for configured verification/delay before next dependency group
```

## 17. Policy acceptance tests

Required cases:

```text
short OB then OL before commit
OB until time threshold
OB + LB
FSD while OL/other mixed tokens
runtime threshold before charge threshold
charge threshold before runtime threshold
comm loss while NORMAL
comm loss while ON_BATTERY
OL before commit cancels
OL after commit does not cancel
power bounce during RECOVERY_WAIT
80% gate then drop to 79% after recovery commit
power fail during RESTORE_HOSTS
reboot during each major state
```
