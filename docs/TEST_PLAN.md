# v0.1 Test and Acceptance Plan

**Status:** Normative release-gate plan for the accepted v0.1 feature set

## 1. Test layers

```text
unit
component/integration
state-machine/fault injection
clean-OS/systemd installer E2E
multi-architecture runtime smoke
optional USB-UPS-Simulator hardware-in-loop
real UPS / DSM physical acceptance
release/governance checks
```

A v0.1 release must pass all mandatory automated tests plus the defined real-hardware acceptance and repository-governance gates.

The schema includes some future-facing values. Tests must distinguish **accepted v0.1 functionality** from **fail-closed reserved functionality**.

## 2. Unit/component tests

### NUT parser/normalizer

- `OL`, `OB`, `LB`, `FSD` and compound states;
- malformed/missing status -> `UNKNOWN`;
- failed `upsc` -> `UNKNOWN`;
- missing charge/runtime remains unavailable, never assumed healthy/full.

### Managed-host WoL

- exact magic packet construction;
- MAC/interface/broadcast/port validation;
- bounded retry state;
- durable `wol_sent` before side effect;
- reboot reconciliation;
- inter-host delay survives/restarts conservatively.

### Configuration

- canonical example parses;
- unknown/invalid fields/ranges rejected where required;
- duplicate host/dependency IDs rejected;
- missing dependency references rejected;
- dependency cycles rejected;
- unsafe timing/range values rejected;
- SSH shutdown missing required credentials rejected;
- managed-host WoL missing MAC/broadcast rejected;
- armed `shutdown.method: command` rejected;
- armed ARP-only verification rejected;
- armed dependency WoL rejected.

### State store

- deterministic checksum;
- sequence increments;
- valid current read;
- corrupt/torn current + valid previous fallback;
- previous generation is not destroyed before new current is durable;
- both invalid -> `FAILED_SAFE`;
- unsupported newer state version not overwritten.

## 3. Power policy simulation

Required transitions include:

```text
BOOT_RECONCILE -> NORMAL
NORMAL -> ON_BATTERY
ON_BATTERY -> NORMAL before commit after debounced OL
ON_BATTERY -> SHUTDOWN_COMMITTED
SHUTDOWN_COMMITTED -> SHUTDOWN_IN_PROGRESS
SHUTDOWN_IN_PROGRESS -> WAITING_FOR_AC
WAITING_FOR_AC -> RECOVERY_WAIT
RECOVERY_WAIT -> RECOVERY_STARTED
RECOVERY_STARTED -> RESTORE_HOSTS
RESTORE_HOSTS -> NORMAL
unrecoverable uncertainty -> FAILED_SAFE
```

Also test a renewed outage during `RESTORE_HOSTS`: it creates a fresh outage epoch, stops further wake actions and makes already-restored online hosts eligible for shutdown again according to policy.

## 4. Trigger and communication-loss behavior

Test:

- ordinary OB + grace;
- critical runtime/charge/max-on-battery triggers;
- OB+LB and FSD commit behavior;
- communication loss while normal and on battery;
- configured communication-loss grace exhaustion;
- reboot during persisted on-battery state does not grant fresh grace;
- two consecutive online samples required to cancel an uncommitted outage;
- online state after shutdown commit does not erase the committed transaction.

## 5. Recovery gates

Test:

- online for less than stable interval: no wake;
- continuous online through configured interval: stability passes;
- OB/communication loss/reboot resets stability proof;
- `recovery.enabled: false` blocks automatic recovery including boot reconciliation;
- charge below/at default 80% boundary;
- runtime fallback;
- configured recharge-time fallback;
- missing usable evidence -> manual recovery;
- network timeout and critical-health failure inhibit further restore;
- post-`RECOVERY_STARTED` small charge drop alone does not reverse recovery while utility remains safe.

## 6. Interrupted boot/shutdown/recovery

Fault-injection coverage includes:

```text
power/service loss before agent startup
NUT unavailable during boot
repeated controller reboot during outage
reboot during stable-AC timer
reboot during candidate configuration validation/probation
loss immediately after SHUTDOWN_COMMITTED persist
loss after shutdown requested but before verification
loss after FSD request
loss immediately after RECOVERY_STARTED persist
reboot after wol_sent
power failure after one or more hosts restored
```

Restart must reconcile real state rather than blindly repeat non-idempotent actions.

## 7. Direct host adapters

### Accepted SSH path

Test:

- dedicated key/known host accepted;
- wrong key or changed host key rejected;
- fixed argv/no shell interpolation;
- timeout bounded;
- consecutive offline verification;
- transient failure -> reconcile + bounded retry;
- retry exhaustion -> durable failure/`FAILED_SAFE` and no premature FSD.

### NUT secondary path

Test:

- `shutdown.method: nut` receives no duplicate SSH/direct request;
- secondary participates in FSD group;
- Synology compatibility account remains monitor-only.

### Reserved/fail-closed paths

Do **not** test successful command shutdown, ARP-only armed verification or dependency WoL as v0.1 features. Instead test that armed validation rejects them predictably.

## 8. NUT primary/secondary integration

Verify generated/project-managed configuration for:

- validated local primary ownership;
- remote-client does not issue FSD;
- existing-NUT multi-primary ambiguity rejects process-wide FSD;
- canonical `HOSTSYNC`/`FINALDELAY` values are rendered from project config;
- secondaries receive the NUT shutdown wave;
- controller/primary shutdown remains last in the NUT path;
- long-running agent does not substitute direct `load.off`/driver shutdown.

Real UPS output shutdown/return is a physical test, not a `dummy-ups` assertion.

## 9. Synology software and physical acceptance

Automated integration validates generated DSM-compatible identity/credentials and no administrative NUT permissions.

Real DSM acceptance must additionally prove on actual DSM hardware/version:

- TCP 3493 monitoring works;
- OL/OB/FSD behavior is observed as expected;
- DSM performs its own safe shutdown through NUT;
- no duplicate direct shutdown is sent;
- managed-host WoL recovery works when supported/configured;
- repeated controller reboot/power bounce does not wake the NAS prematurely.

## 10. Network dependencies

Accepted v0.1 dependency tests cover `auto-power` and `wait-only` readiness:

- dependency unavailable delays dependent-host wake;
- dependency later becomes ready and recovery continues;
- network wait timeout surfaces failure/fail-safe rather than false success;
- no Internet dependency is required unless explicitly modeled.

Dependency `startup: wol` is a negative validation test in armed v0.1.

## 11. Configuration transactions

Test:

- invalid candidate never activates/promotes;
- semantic/safety-invalid candidate rejected;
- component/service startup failure triggers rollback;
- probation failure triggers rollback;
- power loss/reboot during activation does not promote candidate;
- active config bytes/manifest mismatch restores known-good;
- manual rollback is transactional and health-validated;
- failed rollback -> `FAILED_SAFE`;
- browser/channel loss after `config-apply` does not kill systemd-owned probation.

## 12. Health/autofix

Test:

- agent crash/watchdog recovery;
- required project service disabled -> detection and safe bounded repair;
- project-owned permissions/path repair;
- UPS disconnect -> unavailable/unknown, not online;
- repeated repair failure reaches circuit breaker/`FAILED_SAFE`;
- health repair never sends shutdown/WoL/FSD merely to make health pass;
- system-service health state does not race/corrupt agent UPS health state.

## 13. IPC/control security tests

Current agent IPC tests:

```text
GetHealth success/provider failure
unknown method rejected
malformed JSON rejected
missing id/method rejected
oversized request rejected
one request/response per connection
response ID correlation
```

Current CLI/control tests:

```text
config validate without mutation
privileged config apply/rollback require root
candidate size bounded
systemd-owned activation/probation
plan/log/status commands do not invent IPC methods
```

Do not claim generic mutating IPC peer-credential authorization until such methods are implemented.

## 14. Installer/upgrade acceptance

Automated installer coverage includes:

- dependency/platform/architecture checks;
- systemd PID-1 requirement before transaction creation;
- local/remote/existing NUT profiles;
- UPS discovery ambiguity handling;
- trusted/restricted network policy;
- generated empty live inventory and dry-run default;
- service enable/start and health probation;
- known-good creation;
- idempotent reinstall;
- normal broken-upgrade rollback;
- interrupted-install durable marker and boot rollback;
- package/checksum verification.

The existing Ubuntu 24.04 E2E uses NUT `dummy-ups`, real systemd and Cockpit. Additional distro/package matrices may extend this without changing the real-hardware gate.

## 15. Multi-architecture gates

```text
amd64: native build/runtime smoke + systemd installer E2E where defined
arm64: build + QEMU runtime smoke
riscv64: build + QEMU runtime smoke
```

Real UPS v0.1 physical acceptance is required on representative amd64 and arm64 controllers. Physical riscv64 UPS testing is optional for v0.1 unless the release scope changes.

## 16. USB-UPS-Simulator hardware-in-loop

`ami3go/USB-UPS-Simulator` may be used as an additional test layer to validate real USB HID/NUT enumeration and controllable UPS-state transitions.

Useful scenarios:

```text
USB attach/discovery
NUT usbhid-ups integration
OL/OB/LB-style transition handling
communication interruption/reconnect
controller/service restart during simulated outage
```

A simulator PASS is valuable but is **not** a substitute for production UPS electrical output/battery/charger behavior.

## 17. Real hardware matrix

Required before public v0.1:

```text
amd64 + real supported UPS full outage/recovery
arm64 + real supported UPS full outage/recovery
real Synology DSM NUT-secondary shutdown/recovery
```

Use `docs/HARDWARE_ACCEPTANCE.md` and retain exact controller, OS, NUT, UPS, firmware, connection, config revision, software commit and journal/state evidence.

## 18. Release/governance gate

A public v0.1 requires:

```text
[x] project license selected: AGPL-3.0-or-later
[x] mandatory software/unit/integration/fault tests green
[x] package/checksum pipeline green
[ ] amd64 real UPS acceptance retained
[ ] arm64 real UPS acceptance retained
[ ] real DSM acceptance retained
[ ] main branch protected to require PR + green CI and block force-push/deletion (issue #39)
```

No automated/simulated result may be relabeled as a physical PASS.
