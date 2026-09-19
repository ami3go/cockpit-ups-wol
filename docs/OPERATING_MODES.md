# Operating Modes

**Status:** Normative v0.1 behavior

## monitor

Allowed:

- UPS/NUT monitoring
- host status checks
- health checks
- logs/status/config viewing

Blocked:

- automatic managed shutdown
- automatic FSD request
- automatic WoL/recovery
- manual destructive actions unless mode is deliberately changed by an authorized admin

## dry-run

Default for new installations.

The full policy/state machine evaluates conditions and records the action plan, but external power actions are simulated.

Allowed:

- UPS/host monitoring
- calculate shutdown/recovery plan
- simulated state transitions clearly marked as simulation
- configuration validation/probation
- health/autofix that does not perform destructive power actions

Blocked:

- real host shutdown
- real FSD
- real WoL
- UPS output commands

## armed

Normal automatic production mode.

Entering `armed` requires mandatory arming checks:

```text
known-good active config
agent/health stack acceptable
NUT role/profile valid
UPS state available or safely classified
controller backed-power confirmation
controller automatic power-on confirmation
network dependency plan valid
UPS output-cycle classification understood
no unresolved FAILED_SAFE/config transaction
```

`armed` permits configured automatic shutdown and recovery actions.

## maintenance

Used while servicing UPS/network/hosts.

Automatic shutdown/recovery actions are inhibited to avoid unexpected orchestration during maintenance.

Monitoring, health visibility, configuration work and explicit authorized diagnostics remain available.

Manual power actions require separate explicit confirmation and current-state safety validation.

## Transition rules

```text
monitor → dry-run       allowed with valid config
dry-run → armed         requires arming checks
armed → dry-run         allowed unless doing so would interfere with committed critical shutdown
armed → maintenance     restricted during committed power transaction
maintenance → armed     requires arming checks again
FAILED_SAFE             cannot be bypassed merely by mode change
```

During these committed states:

```text
SHUTDOWN_COMMITTED
SHUTDOWN_IN_PROGRESS
WAITING_FOR_AC
RECOVERY_STARTED
RESTORE_HOSTS
```

unsafe mode changes are rejected or staged until the active transaction reaches a safe boundary.

## UI requirement

Cockpit SHALL always show the current operating mode prominently.

`dry-run` actions must never be visually indistinguishable from real armed actions.
