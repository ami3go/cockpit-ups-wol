# Persistent State Model

**Status:** Normative v0.1 internal state contract  
**Schema:** `schemas/state.schema.json`

## 1. Purpose

The persistent state records the safety-critical outage/recovery transaction so the controller can recover correctly after:

- process crash
- service restart
- controller reboot
- sudden power loss
- repeated interrupted boot
- partial shutdown
- partial restoration
- interrupted configuration validation

The state file is not ordinary configuration and SHALL NOT be edited directly from Cockpit.

## 2. Storage layout

Recommended layout:

```text
/var/lib/cockpit-ups-wol/state/
├── current.json
├── previous.json
└── lock
```

At least one previous valid generation is retained.

## 3. State version

```json
"state_version": 1
```

Unknown newer state versions SHALL not be modified by older software.

Migrations must be explicit, testable and transactional.

## 4. Transaction identity

Each outage/recovery cycle has a durable `transaction_id`.

Example:

```text
outage-01J7Y7PZJ7M4C6QF3J4D0ABC12
```

The exact unique-ID format is implementation-defined, but it SHALL NOT depend on a correct wall clock.

A `parent_transaction_id` may link a new outage that occurs during an incomplete recovery to the previous transaction lineage.

## 5. Sequence number

Every durable write increments:

```json
"sequence": 42
```

`sequence` is the primary ordering mechanism for generations. Wall-clock timestamps are diagnostic only.

## 6. Canonical power states

Allowed values:

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

The durable state should reflect the last fully committed transition, not a speculative next state.

## 7. Commit flags

The schema records both:

```text
shutdown_committed
recovery_started
```

These are intentionally redundant with high-level state so boot reconciliation can detect inconsistent/corrupt combinations.

Examples of invalid combinations that must trigger reconciliation/fail-safe handling:

```text
power_state = NORMAL but shutdown_committed = true
power_state = RESTORE_HOSTS but recovery_started = false
recovery_started = true but shutdown_committed = false for a normal outage transaction
```

## 8. Active configuration revision

Every state generation records:

```text
active_config_revision
```

This allows a rebooting agent to determine which known configuration produced the current transaction.

If that revision is no longer usable, reconciliation must deliberately map the transaction onto the restored last-known-good configuration rather than silently discarding state.

## 9. Last UPS observation

A limited last observation may be stored for diagnostics and reconciliation:

```text
utility
low_battery
fsd
battery_charge
battery_runtime_seconds
raw_status
observed_at_wallclock
```

Persisted UPS data is never proof of current UPS state after reboot. Fresh NUT data is required before recovery decisions.

## 10. Per-host shutdown state

Allowed values:

```text
not_required
planned
requested
acknowledged
completed
unknown
failed
```

Meaning:

- `not_required` — host excluded from this shutdown action.
- `planned` — eligible but no external command sent.
- `requested` — command/action sent; result not yet confirmed.
- `acknowledged` — remote adapter acknowledged receipt/processing where supported.
- `completed` — configured verification confirms shutdown.
- `unknown` — prior action may have happened but cannot be proven after interruption.
- `failed` — bounded attempts exhausted or a definitive failure occurred.

`unknown` SHALL cause real-world reconciliation before another non-idempotent command is issued.

## 11. Per-host recovery state

Allowed values:

```text
not_required
waiting
wol_sent
online
unknown
failed
```

Meaning:

- `not_required` — restore policy excludes host.
- `waiting` — eligible but not yet started.
- `wol_sent` — wake request sent; online state not yet confirmed.
- `online` — configured verification passed.
- `unknown` — interrupted action requires reconciliation.
- `failed` — bounded recovery attempts exhausted.

A reboot after `wol_sent` checks host state before deciding whether to retry WoL.

## 12. Pre-outage state

`was_online` is tri-state:

```text
true
false
null/unknown
```

The default `previous-state` restore policy wakes only hosts known to have been online before the committed outage.

An unknown pre-outage state SHALL NOT be treated as `true`.

## 13. Action IDs and idempotency

External actions SHOULD have durable IDs such as:

```text
shutdown:<transaction-id>:<host-id>:<attempt>
wake:<transaction-id>:<host-id>:<attempt>
```

Adapters that support idempotency keys/acknowledgements should use them.

For adapters without idempotency support, state verification occurs before repeating an interrupted action.

## 14. Attempt counters

Durably record at least:

```text
shutdown_attempts
wake_attempts
```

A controller reboot SHALL NOT reset bounded retry counts for the active transaction.

Administrative reset starts a deliberate new retry epoch and is logged.

## 15. Configuration transaction marker

Persistent power state may also carry a compact reference to an active config transaction:

```text
candidate_revision
status
last_known_good
```

The authoritative revision metadata remains in the config-history store.

If power fails during config validation while an outage is active, boot reconciliation prioritizes restoring a trusted power-management configuration.

## 16. FAILED_SAFE reason

When entering `FAILED_SAFE`, persist a human-readable reason code/message, for example:

```text
STATE_CORRUPT_NO_VALID_GENERATION
NO_KNOWN_GOOD_CONFIG
NUT_PRIMARY_OWNERSHIP_UNAVAILABLE
UNRESOLVED_ACTION_STATE
```

The reason must be surfaced in Cockpit and journald.

## 17. Checksum

Each generation stores a SHA-256 integrity checksum.

Implementation rule:

1. serialize the canonical state payload excluding the checksum field
2. compute SHA-256 over canonical bytes
3. store lowercase hexadecimal digest
4. verify digest before accepting a generation

Canonical serialization details SHALL be deterministic in implementation/tests.

The checksum detects corruption/torn files; it is not an authentication mechanism.

## 18. Power-loss-safe write algorithm

Required algorithm where supported:

```text
acquire state lock
read/validate current generation
increment sequence
serialize new generation
write temporary file
flush buffered writes
fsync temporary file
atomically rotate current → previous
atomically rename temporary → current
fsync state directory
release lock
```

The implementation SHALL never intentionally destroy the only valid generation before the new one is durable.

## 19. Read/recovery algorithm

On startup:

```text
read current
validate schema + checksum
        │
        ├─ valid → use current
        │
        └─ invalid → read previous
                       │
                       ├─ valid → restore previous as recovery source + log
                       └─ invalid → FAILED_SAFE
```

After loading a valid generation, the agent still enters `BOOT_RECONCILE` and obtains fresh external evidence.

## 20. State lock

v0.1 uses an OS advisory file lock (`flock` semantics on Linux) around write/rotation operations.

The long-running agent is the normal state writer. Short-lived helpers should request state changes over IPC rather than writing files directly.

A process crash automatically releases the OS lock; no permanent stale PID lock is required.

## 21. Reconciliation examples

### Requested shutdown, controller lost power

```text
host.shutdown_state = requested
boot
host currently offline
→ verify consecutive offline state
→ mark completed
```

If currently online:

```text
check whether adapter action is safely repeatable
bounded retry or mark failure according to policy
```

### WoL sent, controller reboots

```text
host.recovery_state = wol_sent
boot
host online
→ mark online
```

If offline, bounded retry may continue.

### Unknown pre-outage host state

```text
was_online = null
restore_policy = previous-state
→ do not wake automatically
```

## 22. State retention

For v0.1 retain at least:

```text
current generation
previous valid generation
```

Optional diagnostic snapshots may be retained separately but must not be confused with authoritative current state.

## 23. Acceptance tests

Required tests include:

```text
normal state write/read
sequence increments
checksum mismatch in current, valid previous fallback
both generations invalid → FAILED_SAFE
power loss before rename
power loss after current→previous rotation but before new current rename
reboot with requested shutdown
reboot with wol_sent
reboot in every canonical power state
retry counts survive reboot
unknown was_online does not wake
state produced by unsupported newer version is not overwritten
```
