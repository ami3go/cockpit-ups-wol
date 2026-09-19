# Configuration Model

**Status:** Normative v0.1 user configuration

The authoritative schema is `schemas/config.schema.json`. The reference configuration is `config/config.yaml.example`.

## 1. File location

The active configuration is:

```text
/etc/cockpit-ups-wol/config.yaml
```

It SHALL be changed through the project's transaction manager when using Cockpit, CLI/TUI, installer, upgrade or autofix.

Manual editing is permitted for administrators, but it creates configuration drift until the transaction/health manager imports or validates the change.

## 2. Schema version

```yaml
config_version: 1
```

Unknown newer schema versions SHALL be rejected rather than overwritten.

## 3. Operating mode

```yaml
mode: dry-run
```

Allowed values:

```text
monitor
 dry-run
 armed
 maintenance
```

New installations default to `dry-run`.

## 4. NUT profile

```yaml
nut:
  profile: local-server
  ups_name: ups
  host: localhost
  port: 3493
```

Profiles:

- `local-server` — controller owns locally attached UPS, runs driver/upsd/primary upsmon.
- `remote-client` — reads a remote NUT server and does not automatically assume primary/FSD authority.
- `existing` — integrates with an existing local NUT deployment without blindly rewriting it.

`power_cycle_capability` is one of:

```text
POWER_CYCLE_VERIFIED
POWER_CYCLE_UNVERIFIED
MONITOR_ONLY
```

This classification gates unattended controller power-off/restart behavior.

## 5. Synology compatibility

When enabled:

```yaml
nut:
  synology_compatibility:
    enabled: true
    username: monuser
    password: secret
```

The compatibility account remains monitor-only and uses NUT secondary semantics.

## 6. Outage policy

```yaml
outage:
  grace_period_seconds: 120
  critical_battery_percent: 30
  critical_runtime_seconds: 600
  max_on_battery_seconds: null
  communication_loss_grace_seconds: 30
```

Trigger precedence is defined in `docs/POWER_POLICY.md`.

A `null` optional trigger means it is disabled. `FSD` and `LB` remain safety-critical regardless of optional thresholds.

## 7. Controller requirements

```yaml
controller:
  require_ups_backed_power: true
  require_auto_power_on: true
```

Both are mandatory v0.1 safety requirements. They are represented in configuration so installer/TUI/Cockpit can show compliance status, not so users can disable them.

## 8. Recovery policy

```yaml
recovery:
  enabled: true
  utility_stable_seconds: 120
  battery_charge_min: 80
  runtime_min_seconds: null
  recharge_time_seconds: null
  network_wait_seconds: 300
```

Fallback order:

```text
battery charge → runtime → configured recharge time → manual
```

Missing data never silently satisfies a recovery gate.

## 9. Health policy

```yaml
health:
  enabled: true
  interval_seconds: 60
  probation_seconds: 60
  autofix: true
  max_repair_attempts: 5
```

Health autofix remains bounded by `docs/RELIABILITY_REQUIREMENTS.md` and must not perform destructive power actions simply to make a health check pass.

## 10. Network dependencies

Use `network_dependencies` for switches, routers or other infrastructure that must become reachable before managed hosts are restored.

Example:

```yaml
network_dependencies:
  - id: core-switch
    startup: auto-power
    priority: 1
    status:
      method: tcp
      port: 22
    wake:
      enabled: false
```

Startup modes:

- `auto-power` — expected to start automatically when power is available.
- `wait-only` — external/manual power; the agent waits for readiness.
- `wol` — dependency may be awakened with WoL.

## 11. Managed hosts

Each host has:

```text
id
name
address
depends_on
status
shutdown
wake
restore_policy
```

### Status methods

```text
auto
ping
tcp
arp
none
```

A positive/negative state normally requires multiple consecutive observations rather than one probe.

### Shutdown methods

```text
nut
ssh
command
none
```

`command` references an allowlisted `command_id`; arbitrary UI-supplied shell text is not accepted.

Lower shutdown priority executes first for pre-FSD managed hosts. NUT secondaries form a synchronized FSD group as described in `docs/NUT_SHUTDOWN_MODEL.md`.

### Restore policy

```text
previous-state
always
never
```

Default: `previous-state`.

### Wake settings

Wake supports MAC, interface, broadcast, UDP port, priority, inter-host delay and bounded retry count.

## 12. Secrets

Long-lived secrets SHOULD NOT be embedded directly in ordinary configuration except where compatibility requires a known credential (notably optional DSM compatibility).

SSH private keys and future privileged NUT/API credentials live under a protected secrets directory, for example:

```text
/etc/cockpit-ups-wol/secrets/
```

Expected permissions:

```text
root-owned or dedicated service-owned
0600 private files
0700 secrets directory where practical
```

Secrets SHALL be redacted from logs, configuration diagnostics and revision diffs shown in Cockpit.

## 13. Transactional activation

Configuration changes follow:

```text
candidate
→ schema validation
→ cross-reference validation
→ component preflight
→ atomic activation
→ service reload/restart
→ immediate health checks
→ probation
→ known-good OR rollback
```

A candidate is never promoted solely because the machine rebooted successfully.

## 14. Cross-reference validation

The transaction manager SHALL reject at least:

- duplicate host/dependency IDs
- references to unknown dependencies
- invalid MAC addresses
- TCP status without a valid port
- WoL enabled without MAC
- SSH shutdown without required user/key reference
- command shutdown with unknown command ID
- restricted NUT mode that leaves an enabled address family unintentionally unrestricted
- local automatic controller shutdown when UPS power-cycle capability is not verified and no alternate restart mechanism exists

## 15. Defaults

Important defaults:

```text
mode                         dry-run
UPS name                     ups
NUT port                     3493
NUT network                  trusted-lan
automatic recovery           enabled
utility stability            120 s
battery recovery threshold   80%
network wait                 300 s
outage grace                 120 s
health interval              60 s
config probation             60 s
restore policy               previous-state
```

## 16. Compatibility and migration

Configuration migration SHALL create a candidate revision rather than rewriting the only active copy in place.

Migration is committed only after the migrated stack passes validation/probation. Otherwise the prior known-good revision remains active.
