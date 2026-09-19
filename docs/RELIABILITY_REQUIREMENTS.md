# cockpit-ups-wol — Reliability, Health and Configuration Rollback Requirements

**Requirements version:** 1.0  
**Status:** Normative implementation requirement  
**Applies to:** installer, runtime services, Cockpit configuration changes, CLI/TUI changes, upgrades and automatic recovery

## 1. Purpose

`cockpit-ups-wol` is a power-management system and must remain predictable after reboots, process crashes, invalid configuration changes and power events.

Reliability is therefore a core requirement rather than an optional feature.

The implementation SHALL provide:

- automatic startup of all required runtime services
- automatic recovery of failed services where safe
- automatic continuation of power-recovery state after reboot
- continuous health checking
- bounded automatic repair of recoverable faults
- transactional configuration changes
- automatic rollback when a new configuration fails validation or runtime health checks
- immutable revision tagging of every successfully validated configuration
- a persistent **last-known-good** configuration used as the default fallback

The system SHALL prefer a previously verified configuration over attempting to operate with an unknown or partially applied configuration.

---

## 2. Reliability Principles

The runtime SHALL follow these principles:

1. **No manual start after reboot.** Required services start or become available automatically.
2. **No blind restart loops.** Restart attempts are bounded and back off after repeated failure.
3. **Unknown is not healthy.** Missing NUT data, failed health checks or unvalidated configuration SHALL NOT be interpreted as successful operation.
4. **Configuration changes are transactions.** A configuration either becomes verified known-good or is rolled back.
5. **The last-known-good revision is persistent.** It survives reboot, package upgrade and power loss.
6. **Automatic repair must be conservative.** It may restart services or restore project-owned known-good files, but SHALL NOT rewrite unknown user-managed configuration destructively.
7. **Power recovery is restart-safe.** Rebooting the controller SHALL NOT lose an active outage/recovery transaction.
8. **Every automatic repair is observable.** Actions and reasons are recorded in journald and exposed in Cockpit.

---

## 3. Required Automatic Startup

All installed runtime components required by the selected installation profile SHALL be enabled automatically during installation.

A normal reboot SHALL require no manual commands.

For a local NUT server installation this normally includes:

```text
cockpit.socket
nut driver service/unit(s)
nut-server.service
nut-monitor.service
cockpit-ups-wol-agent.service
cockpit-ups-wol-health.timer
```

Exact NUT service names MAY vary by distribution.

`wolctl` is an on-demand helper and is therefore not a persistent service.

The installer SHALL determine the correct distro-specific units, enable them, start them and verify that their enabled/active state matches the selected profile.

The installer SHALL fail validation if a required unit is installed but unintentionally disabled.

---

## 4. Service Recovery

Required persistent services SHALL use systemd recovery settings appropriate to their role.

The implementation SHOULD use concepts equivalent to:

```ini
Restart=on-failure
RestartSec=5s
StartLimitIntervalSec=300
StartLimitBurst=5
```

Values MAY differ by component.

The system SHALL avoid an unlimited tight restart loop.

Where the agent supports watchdog notification, the project SHOULD use:

```ini
WatchdogSec=<configured interval>
```

so systemd can restart a process that remains alive but stops making progress.

Service recovery SHALL distinguish at least:

```text
process crash
startup failure
hung process / missed watchdog
configuration failure
dependency unavailable
network temporarily unavailable
UPS/NUT temporarily unavailable
```

A dependency outage such as unavailable network or UPS SHALL normally cause retry/backoff rather than repeated full service restarts.

---

## 5. Power Recovery

Power recovery is separate from service recovery.

Automatic power recovery SHALL be enabled by default on a normal installation.

Default recovery policy remains:

```text
utility stable period     120 seconds
minimum UPS charge        80%
network readiness wait    300 seconds
```

After controller reboot the agent SHALL load persisted outage state and continue from the last safe state.

Example:

```text
controller boots
      ↓
services start automatically
      ↓
agent loads persistent outage state
      ↓
NUT data becomes valid
      ↓
AC confirmed stable
      ↓
UPS battery >= 80%
      ↓
network confirmed ready
      ↓
restore previously-running hosts in configured order
```

The controller SHALL NOT treat startup itself as proof that utility power has recovered.

Missing/invalid UPS status SHALL be represented as `UNKNOWN` or equivalent and SHALL block automatic host restoration until valid data is available or an explicitly configured fallback policy is satisfied.

---

## 6. Health Supervisor

The project SHALL include an independent lightweight health-check mechanism.

Recommended systemd units:

```text
cockpit-ups-wol-health.service
cockpit-ups-wol-health.timer
```

The health service SHOULD be a short-lived oneshot process rather than another permanent daemon.

Recommended initial cadence:

```text
OnBootSec=60s
OnUnitActiveSec=60s
```

The interval SHALL be configurable.

The health supervisor is complementary to the agent:

```text
systemd
  └─ restarts crashed/hung services

cockpit-ups-wol-health
  └─ validates the complete application stack and attempts bounded repair

cockpit-ups-wol-agent
  └─ handles UPS outage/recovery state and managed hosts
```

---

## 7. Health Checks

The health supervisor SHALL check, as applicable to the selected installation profile:

### Core services

- required units are enabled
- required units are active or in an expected transitional state
- agent heartbeat/watchdog is current
- Cockpit socket is available

### NUT

- NUT binaries exist
- selected NUT configuration is syntactically/semantically acceptable where validation is available
- expected driver service exists
- `upsd` is reachable locally when local-server mode is selected
- `upsc <ups>` succeeds when the UPS is expected to be available
- TCP 3493 listener exists when LAN NUT service is enabled
- Synology monitor account exists when Synology compatibility is enabled

### Configuration

- active configuration revision exists
- configuration hashes match its revision manifest
- schema version is supported
- references between host/config files are valid
- active revision is either known-good or is currently inside a controlled validation transaction

### Persistent state

- `/var/lib/cockpit-ups-wol/` exists
- required files/directories have safe ownership and permissions
- outage state can be read
- state storage is writable when the filesystem is expected to be writable

### Runtime dependencies

- selected network interface exists
- network readiness is reported accurately
- `wolctl` is executable
- required SSH keys/helpers exist for hosts configured to use them

A temporarily disconnected UPS SHALL be reported distinctly from a broken NUT installation.

---

## 8. Health States

The stack SHOULD expose a single summarized health state in Cockpit:

```text
HEALTHY
DEGRADED
RECOVERING
CONFIG_VALIDATING
ROLLING_BACK
FAILED_SAFE
```

Suggested meaning:

- `HEALTHY` — all required checks pass.
- `DEGRADED` — service is usable but one or more non-critical checks fail.
- `RECOVERING` — automatic service repair is in progress.
- `CONFIG_VALIDATING` — a candidate configuration is in probation.
- `ROLLING_BACK` — candidate configuration failed and known-good state is being restored.
- `FAILED_SAFE` — automatic repair/rollback could not restore trustworthy operation; destructive automation is inhibited.

Cockpit SHALL show the failing check and most recent repair action.

---

## 9. Automatic Repair

The health supervisor SHALL be allowed to perform only bounded, well-defined repairs.

Safe automatic repairs MAY include:

- restart a failed project-managed service
- re-enable a required project-managed unit that was unintentionally disabled
- restart a NUT service/driver after a confirmed transient failure
- recreate missing project-owned runtime directories
- restore expected permissions on project-owned files/directories
- restore the last-known-good project configuration after an active configuration failure
- resume the persisted recovery state after agent restart

Automatic repair SHALL NOT, by default:

- overwrite unknown pre-existing NUT configuration
- replace an unknown/custom firewall configuration
- change system networking
- regenerate credentials without an explicit recovery procedure
- send destructive UPS instant commands
- shut down or wake managed hosts merely to make a health check pass

Each repair SHALL be logged with:

```text
time
check that failed
repair attempted
result
retry count
active configuration revision
last-known-good revision
```

---

## 10. Repair Backoff and Circuit Breaker

Repeated repair failures SHALL use backoff and a circuit-breaker behavior.

Example policy:

```text
attempt 1     immediate
attempt 2     +10 seconds
attempt 3     +30 seconds
attempt 4     +60 seconds
attempt 5     +300 seconds
```

Exact values MAY be configurable.

After the configured failure limit, the subsystem SHALL enter `FAILED_SAFE` instead of restarting indefinitely.

In `FAILED_SAFE`:

- monitoring SHOULD remain available where possible
- Cockpit SHALL clearly display the failure
- destructive automatic shutdown/wake actions that depend on uncertain state SHALL be inhibited
- journald SHALL contain the exact reason and recovery instructions

---

# 11. Transactional Configuration Management

Every configuration modification performed by this project SHALL use the same transactional configuration mechanism.

This includes changes from:

```text
Cockpit UI
installer
TUI
CLI
upgrade/migration
automatic migration
authorized autofix
```

Direct manual editing by the administrator cannot be made transactional automatically, but the health system SHALL detect that the active configuration differs from the recorded revision and report configuration drift.

---

## 12. Configuration Revision Storage

Active user configuration remains under:

```text
/etc/cockpit-ups-wol/
```

Revision history SHALL be stored separately, for example:

```text
/var/lib/cockpit-ups-wol/config-history/
```

Recommended layout:

```text
/var/lib/cockpit-ups-wol/config-history/
├── revisions/
│   ├── cfg-20260919T105201Z-a13f92cd/
│   │   ├── config.yaml
│   │   ├── hosts.yaml
│   │   ├── managed-nut/
│   │   └── manifest.yaml
│   └── ...
├── active
├── last-known-good
└── previous-known-good
```

The pointers MAY be symlinks, small metadata files or another atomic representation.

The revision store SHALL survive reboot and normal package upgrades.

---

## 13. Revision Manifest

Every revision SHALL have immutable metadata similar to:

```yaml
revision_id: cfg-20260919T105201Z-a13f92cd
created_at: 2026-09-19T10:52:01Z
source: cockpit
schema_version: 1
parent_revision: cfg-20260919T102400Z-7a31d912
status: known-good
content_sha256: a13f92cd...
```

Recommended `source` values:

```text
installer
cockpit
cli
tui
upgrade
autofix
rollback
```

Recommended lifecycle states:

```text
candidate
validating
known-good
failed
rolled-back
```

A revision marked `known-good` SHALL be immutable.

---

## 14. Automatic Configuration Tags

Every configuration that successfully completes validation SHALL receive an immutable revision ID/tag.

Recommended format:

```text
cfg-<UTC timestamp>-<content hash prefix>
```

Example:

```text
cfg-20260919T105201Z-a13f92cd
```

The project SHALL maintain logical tags/pointers for at least:

```text
active
last-known-good
previous-known-good
```

`last-known-good` SHALL always point to the newest configuration that has completed full validation and probation successfully.

A newly written candidate SHALL NOT replace `last-known-good` until runtime validation completes.

---

## 15. Configuration Transaction Flow

All project-controlled configuration changes SHALL follow this flow:

```text
request change
      ↓
acquire configuration lock
      ↓
identify current last-known-good revision
      ↓
create candidate revision in staging area
      ↓
validate schema and syntax
      ↓
validate cross-file references
      ↓
run component-specific preflight checks
      ↓
atomically activate candidate
      ↓
reload/restart only affected services
      ↓
run immediate health checks
      ↓
probation period
      ↓
run complete health checks
      │
      ├── PASS → tag revision KNOWN-GOOD
      │          update last-known-good pointer
      │
      └── FAIL → automatic rollback
```

Configuration writes SHALL use temporary files and atomic rename wherever the filesystem permits.

For critical revision metadata the implementation SHOULD use:

```text
write temporary file
fsync temporary file
atomic rename
fsync parent directory
```

---

## 16. Validation Stages

A candidate configuration SHALL pass multiple stages.

### Stage A — static validation

- YAML/JSON syntax
- schema version
- required keys
- value ranges
- MAC/IP/port validation
- duplicate IDs
- invalid references

### Stage B — component preflight

Where available:

- NUT configuration validation
- executable/helper availability
- referenced interfaces exist
- configured files/credentials are readable by the expected service
- systemd unit validity

### Stage C — activation

- atomically install candidate
- reload/restart affected components only
- verify processes start successfully

### Stage D — runtime probation

Candidate remains `validating` for a configurable probation period.

Recommended default:

```text
60 seconds
```

During probation the system SHALL verify at least:

- agent remains healthy
- required services remain active
- local NUT query works when expected
- no immediate service restart loop occurs
- configuration hash still matches the candidate

Only after probation succeeds may the candidate become `known-good`.

---

## 17. Automatic Rollback

Rollback SHALL occur automatically when a candidate configuration causes any mandatory validation stage to fail.

Rollback flow:

```text
candidate fails
      ↓
mark candidate FAILED
      ↓
activate last-known-good revision atomically
      ↓
reload/restart affected services
      ↓
run health validation
      │
      ├── PASS → mark system HEALTHY and candidate ROLLED-BACK
      │
      └── FAIL → try previous-known-good if policy permits
                     ↓
                 FAILED_SAFE if no verified revision works
```

The system SHALL NOT delete the failed candidate immediately. It SHOULD retain metadata and diagnostic reason so the administrator can understand why it failed.

Secrets in diagnostics SHALL remain redacted.

---

## 18. Rollback After Reboot

Configuration validation state SHALL survive reboot.

If the controller reboots while a candidate configuration is still in `validating` state, startup SHALL NOT silently promote that configuration to known-good.

Startup logic SHALL:

```text
load active revision
      ↓
load last-known-good revision
      ↓
if active == known-good
    validate and continue
else
    resume candidate validation when safe
    OR rollback to last-known-good according to transaction state
```

If the active candidate prevents core services from becoming healthy, automatic rollback to `last-known-good` SHALL occur.

---

## 19. Rollback Scope

A configuration transaction SHALL include every project-managed file required for that change to be internally consistent.

This MAY include:

```text
/etc/cockpit-ups-wol/config.yaml
/etc/cockpit-ups-wol/hosts.yaml
project-owned systemd drop-ins
project-managed NUT configuration fragments
project-owned firewall fragments when restricted mode is explicitly enabled
```

For existing administrator-owned NUT files, the project SHALL preserve an exact pre-change snapshot before making an authorized modification.

Rollback SHALL restore:

- file contents
- owner/group
- permissions
- project-managed service state where applicable

The project SHALL NOT claim rollback succeeded until the restored configuration passes health validation.

---

## 20. Configuration History Retention

The project SHALL retain multiple known-good revisions.

Recommended defaults:

```text
known-good revisions retained    10
failed revisions retained         5
```

Retention SHALL never remove:

```text
active
last-known-good
previous-known-good
```

Pinned revisions MAY be supported later and SHALL not be automatically pruned.

---

## 21. Manual Rollback

Cockpit and CLI SHOULD allow an administrator to view and restore known-good revisions.

Example CLI concept:

```bash
cockpit-ups-wol config list
cockpit-ups-wol config show cfg-20260919T105201Z-a13f92cd
cockpit-ups-wol config rollback cfg-20260919T105201Z-a13f92cd
```

A manual rollback SHALL itself be transactional and health-validated.

Cockpit SHOULD show:

```text
Revision
Created
Source
Status
Health result
Parent
Active
Known-good
Rollback action
```

---

## 22. Configuration Change Locking

Only one configuration transaction SHALL run at a time.

A lock SHALL protect against concurrent changes from:

- Cockpit
- installer
- CLI/TUI
- upgrade process
- autofix

A process crash SHALL not leave a permanent unusable lock. The locking mechanism SHALL support safe stale-lock recovery.

---

## 23. Interaction With Active Power Events

Configuration changes affecting shutdown/recovery policy SHALL be restricted during a committed outage transaction.

When the system is in a safety-critical state such as:

```text
SHUTDOWN_IN_PROGRESS
WAITING_FOR_AC
RECOVERY_WAIT
RESTORE_HOSTS
```

Cockpit SHOULD either:

- reject unsafe policy changes, or
- stage them for activation after the current power transaction completes.

A configuration rollback required to restore the health of the power-management system MAY still proceed.

---

## 24. Health and Configuration UI

Cockpit SHALL expose a reliability view or equivalent information showing:

```text
Overall health
Active configuration revision
Last-known-good revision
Last health check time
Failed checks
Automatic repair history
Service restart counts
Configuration probation state
Rollback events
```

A configuration-save operation SHOULD display progression such as:

```text
Saving candidate
  ✓ schema valid
  ✓ preflight valid
  ✓ activated
  ✓ services restarted
  ✓ immediate health checks
  … probation
  ✓ known-good: cfg-20260919T105201Z-a13f92cd
```

Failure example:

```text
Saving candidate
  ✓ schema valid
  ✓ preflight valid
  ✓ activated
  ✗ nut-server failed health check

Automatic rollback
  ✓ restored cfg-20260919T102400Z-7a31d912
  ✓ services healthy

New configuration was NOT accepted.
```

---

## 25. Installer Requirements

The installer SHALL:

1. install all selected required units
2. enable required units for automatic startup
3. install and enable the health timer
4. start/restart the selected stack
5. run full installation health validation
6. create the initial configuration revision
7. mark the initial revision `known-good` only after validation succeeds
8. initialize `active`, `last-known-good` and revision metadata
9. fail the installation if a valid known-good baseline cannot be established

For an upgrade, the pre-upgrade known-good revision SHALL remain available until the upgraded stack has completed validation.

---

## 26. Upgrade Transaction

Upgrade procedure SHALL be transactional at the application/configuration level:

```text
record current version
record last-known-good config
backup project-controlled files
install new artifacts
migrate config into candidate revision
restart services
validate
probation
    │
    ├── PASS → commit upgrade + tag config known-good
    └── FAIL → restore prior files/config + restart + validate
```

An upgrade SHALL NOT destroy the previous known-good configuration merely because package installation itself succeeded.

---

## 27. Observability

The following events SHALL be logged:

- required service started/stopped/restarted
- systemd restart-limit reached
- health check success/failure summary
- autofix attempt/result
- configuration transaction opened
- candidate revision created
- validation stage failure
- candidate activated
- candidate promoted to known-good
- automatic rollback started/completed/failed
- last-known-good pointer changed
- boot-time recovery/rollback decision
- entry into `FAILED_SAFE`

Important reliability events SHOULD be visible from Cockpit without requiring shell access.

---

## 28. Acceptance Tests

Release testing SHALL include at least:

### Service recovery

- reboot controller and confirm all required services return automatically
- kill agent and confirm systemd restarts it
- simulate hung agent/watchdog failure
- temporarily stop NUT and verify bounded recovery
- verify restart-loop circuit breaker

### Health/autofix

- disable a required project service and verify detection/repair
- corrupt project-owned runtime permissions and verify safe repair
- disconnect UPS and verify `UPS unavailable`, not false `OL`
- restore UPS and verify recovery without manual service restart

### Configuration transactions

- valid configuration becomes known-good
- syntax-invalid configuration never activates
- semantic-invalid configuration never activates
- configuration that passes syntax but causes service startup failure rolls back
- configuration that fails during probation rolls back
- controller reboot during candidate probation does not promote candidate automatically
- last-known-good survives reboot
- previous-known-good can be restored manually
- failed rollback enters `FAILED_SAFE`

### Power-state persistence

- reboot during `ON_BATTERY`
- reboot during shutdown transaction
- reboot during `WAITING_FOR_AC`
- reboot during `RECOVERY_WAIT`
- reboot during host restoration

In every case, the system SHALL resume from persisted state without losing the safety context.

---

## 29. Reliability Acceptance Criteria

The reliability requirements are satisfied only when all of the following are true:

```text
✓ selected runtime services automatically start after reboot
✓ failed persistent services recover automatically within bounded policy
✓ agent state survives process/controller restart
✓ health supervisor runs automatically
✓ safe recoverable failures can be automatically repaired
✓ unknown/invalid UPS state never becomes a false healthy state
✓ every project-mediated config change creates a revision
✓ candidate configurations are validated before becoming known-good
✓ every successful configuration receives an immutable revision tag
✓ last-known-good always identifies a validated working configuration
✓ failed configuration automatically rolls back
✓ rollback itself is health-validated
✓ last-known-good survives reboot and upgrade
✓ failed rollback enters a safe inhibited state instead of guessing
✓ Cockpit reports health, repair and rollback status
```
