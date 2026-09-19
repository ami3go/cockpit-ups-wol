# v0.1 Test and Acceptance Plan

**Status:** Normative release-gate plan

## 1. Test layers

```text
unit
component/integration
simulation/fault injection
clean-OS installer
hardware-in-loop
release/upgrade
```

A v0.1 release SHALL pass all mandatory non-hardware tests and the defined hardware acceptance matrix.

## 2. Unit tests

### NUT parser/normalizer

- parse `OL`
- parse `OB`
- parse compound status such as `OL CHRG`, `OB LB`
- parse `FSD`
- malformed/missing status → UNKNOWN
- failed `upsc` → UNKNOWN
- missing charge remains unavailable, never 100%

### WoL

- exact 102-byte magic packet
- six `FF` sync bytes
- MAC repeated 16 times
- invalid MAC rejected
- interface/broadcast/port validation

### Configuration

- valid reference example passes
- unknown field rejected where schema forbids it
- invalid ranges rejected
- duplicate host/dependency ID semantic error
- missing dependency semantic error
- WoL enabled without MAC rejected
- SSH method without required credential reference rejected
- unknown command ID rejected

### State store

- deterministic checksum
- sequence increments
- current valid read
- current corrupt + previous valid fallback
- both corrupt → FAILED_SAFE
- unsupported newer state version is not overwritten

## 3. Power policy simulation

Required transitions:

```text
BOOT_RECONCILE → NORMAL
NORMAL → ON_BATTERY
ON_BATTERY → NORMAL before commit
ON_BATTERY → SHUTDOWN_COMMITTED
SHUTDOWN_COMMITTED → SHUTDOWN_IN_PROGRESS
SHUTDOWN_IN_PROGRESS → WAITING_FOR_AC
WAITING_FOR_AC → RECOVERY_WAIT
RECOVERY_WAIT → RECOVERY_STARTED
RECOVERY_STARTED → RESTORE_HOSTS
RESTORE_HOSTS → NORMAL
any unrecoverable safety state → FAILED_SAFE
```

## 4. Trigger precedence

Test:

- ordinary OB + grace
- max time-on-battery trigger
- critical charge trigger
- critical runtime trigger
- OB+LB immediate commit
- FSD immediate commit
- communication loss during NORMAL
- communication loss during ON_BATTERY
- power restoration before commit
- power restoration after commit

## 5. Recovery gates

Test:

- ONLINE for < stable interval: no wake
- 120 s continuous ONLINE: stability gate passes
- OB inside stability interval resets timer
- communication loss inside stability interval resets proof
- reboot invalidates unfinished stability timer
- charge 79%: no wake
- charge 80%: entry gate passes
- charge 80% then 79% after RECOVERY_STARTED: continue if utility remains safe
- missing charge uses runtime fallback
- missing charge/runtime uses configured recharge-time fallback
- no usable fallback requires manual recovery

## 6. Interrupted boot

Fault-injection cases:

```text
power loss before agent starts
power loss during NUT startup
power loss during BOOT_RECONCILE
repeated boot interruptions
boot while UPS still OB
boot with NUT unavailable
boot with OL but battery below recovery threshold
boot during active candidate config validation
```

No case may wake hosts solely because Linux booted.

## 7. Interrupted shutdown

For each interruption point:

```text
before SHUTDOWN_COMMITTED persist
after commit persist but before first command
after shutdown request before acknowledgement
after acknowledgement before verification
after some hosts complete
immediately before FSD request
after FSD request
```

Restart must reconcile instead of blindly repeating unsafe actions.

## 8. Interrupted recovery

Cases:

```text
before RECOVERY_STARTED persist
after persist before first WoL
after first WoL before verification
after one host online
power fails during RESTORE_HOSTS
agent/controller reboot during RESTORE_HOSTS
```

Previously-off hosts remain off under `previous-state`.

## 9. NUT primary/secondary integration

Use a test NUT environment with primary and one/multiple secondaries.

Verify:

- agent requests FSD only through primary `upsmon`
- secondaries observe FSD and enter shutdown path
- primary waits according to HOSTSYNC
- controller/primary is last
- agent never invokes late driver poweroff directly
- remote-client mode does not issue FSD

Hardware test final output-cycle behavior separately.

## 10. Synology acceptance

At minimum validate on supported DSM environment:

- NAS can connect to controller NUT TCP 3493
- UPS name `ups` is accepted
- `monuser` compatibility works where required
- account is monitor-only
- DSM observes expected OL/OB/FSD behavior
- safe DSM shutdown occurs through native NUT path
- agent does not send duplicate SSH shutdown for `method: nut`

## 11. Host adapters

### SSH

- dedicated key accepted
- wrong key rejected
- changed host key rejected
- timeout bounded
- no shell interpolation injection

### command

- known command ID executes expected argv
- unknown command ID rejected
- user-supplied shell text cannot execute

### status

For ping/tcp/arp/adapters:

- one transient failure does not mark offline
- 3 configured consecutive failures marks shutdown verification
- 3 consecutive successes marks recovered/online

## 12. Network dependencies

Test:

- auto-power switch unavailable delays dependent wake
- dependency becomes ready and recovery continues
- wait-only dependency timeout is surfaced without false success
- WoL-capable dependency can be ordered first
- no Internet dependency is required unless explicitly configured

## 13. Configuration transaction tests

- syntax-invalid candidate never activates
- schema-invalid candidate never activates
- semantic-invalid candidate never activates
- service-start failure triggers rollback
- probation failure triggers rollback
- power loss during probation does not promote candidate
- reboot resumes/rolls back transaction safely
- known-good immutable after promotion
- manual rollback is itself validated
- failed rollback → FAILED_SAFE

## 14. Health/autofix

- kill agent: systemd recovers it
- hang/missed watchdog: recovery path triggers
- disable required project service: health detects and safely re-enables
- break project-owned permissions: safe repair
- disconnect UPS: report unavailable, not OL
- repeated repair failure reaches circuit breaker/FAILED_SAFE
- health repair never sends WoL/shutdown/FSD merely to make health pass

## 15. IPC/security tests

- READ operation works for permitted client
- privileged operation rejected for unprivileged peer
- root/superuser path accepted
- malformed JSON rejected
- oversized request rejected
- duplicate nonce does not duplicate destructive action
- concurrent config transactions conflict
- secrets absent from response/log summaries

## 16. Installer tests

Clean systems for each supported family should validate:

```text
install dependencies
install prebuilt artifacts
generate candidate config
enable/start services
health probation
known-good creation
reboot/autostart
idempotent second install
```

Silent mode never prompts.

Ambiguous UPS selection fails safely without explicit parameters.

## 17. Upgrade/rollback

Test:

- healthy version A → healthy version B
- B binary fails startup → restore A
- B config migration fails → restore A + prior LKG
- power loss during upgrade → boot reconciliation restores coherent version/config
- checksum failure before install makes no active changes

## 18. Hardware matrix

Required before v0.1 stable:

```text
amd64 + real USB HID UPS
arm64 + real USB HID UPS
riscv64 smoke test of agent/config/state/IPC where practical
Synology DSM NUT client
Ethernet switch/router dependency test
```

Hardware acceptance records exact:

```text
UPS vendor/model
NUT driver
NUT version
OS/distribution
architecture
output-cycle capability result
observed shutdown-return behavior
```

## 19. UPS output-cycle hardware test

This test must be opt-in and performed only on disposable/test load.

Verify:

```text
FSD shutdown sequence
controller/secondary shutdown complete
UPS output actually turns off as expected
utility restoration causes expected output return
controller automatically boots
BOOT_RECONCILE blocks host wake until stability + 80% gate
```

Only after successful test may profile become `POWER_CYCLE_VERIFIED`.

## 20. Release gate

A stable v0.1 requires:

```text
all mandatory unit/integration tests pass
all safety fault-injection scenarios pass
clean installer test passes
upgrade rollback test passes
amd64 hardware test passes
arm64 hardware test passes
Synology acceptance passes
no unresolved P0 safety issue
```
