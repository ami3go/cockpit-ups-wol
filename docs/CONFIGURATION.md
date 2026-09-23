# Configuration Model

**Status:** Normative v0.1 user configuration  
**Schema:** `schemas/config.schema.json`  
**Example:** `config/config.yaml.example`

The schema describes parseable configuration. Some values are reserved for future features and are intentionally rejected by armed-v0.1 safety validation; schema presence alone does not mean a capability is accepted for automatic execution.

## 1. Active configuration

```text
/etc/cockpit-ups-wol/config.yaml
```

Project-mediated changes from Cockpit, installer, CLI/TUI, upgrade or autofix use the transactional revision manager. Manual administrator edits create drift until validated/reconciled.

## 2. Schema version

```yaml
config_version: 1
```

Unknown newer versions are rejected rather than overwritten.

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

Example:

```yaml
nut:
  profile: local-server
  ups_name: ups
  host: localhost
  port: 3493
  hosts_sync_seconds: 60
  final_delay_seconds: 15
```

Profiles:

- `local-server` — controller owns the locally attached UPS and validated primary/FSD path;
- `remote-client` — reads a remote NUT server and does not automatically inherit primary/FSD authority;
- `existing` — integrates conservatively with existing local NUT configuration.

`hosts_sync_seconds` and `final_delay_seconds` are canonical project settings used when rendering managed `upsmon.conf`; they are not cosmetic UI-only fields.

`power_cycle_capability` is one of:

```text
POWER_CYCLE_VERIFIED
POWER_CYCLE_UNVERIFIED
MONITOR_ONLY
```

This classification gates unattended controller power-off/restart assumptions.

## 5. Synology compatibility

When enabled:

```yaml
nut:
  synology_compatibility:
    enabled: true
    username: monuser
    password: secret
```

The compatibility account remains monitor-only and uses NUT secondary semantics. It must not receive actions/instant-command/FSD authority.

## 6. Outage policy

```yaml
outage:
  grace_period_seconds: 120
  critical_battery_percent: 30
  critical_runtime_seconds: 600
  max_on_battery_seconds: null
  communication_loss_grace_seconds: 30
```

A `null` optional threshold disables that optional trigger. `FSD`/low-battery safety behavior remains authoritative regardless of optional thresholds.

Trigger precedence and debounce semantics are defined in `docs/POWER_POLICY.md`.

## 7. Controller requirements

```yaml
controller:
  require_ups_backed_power: true
  require_auto_power_on: true
```

These are v0.1 safety requirements. They exist in configuration so installer/TUI/Cockpit can represent compliance; they are not intended as switches to bypass the deployment model.

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

`recovery.enabled: false` is a hard automatic-recovery gate, including boot reconciliation.

Recovery fallback order:

```text
battery charge -> runtime -> configured recharge time -> manual
```

Missing data never silently satisfies a gate.

## 9. Health policy

```yaml
health:
  enabled: true
  interval_seconds: 60
  probation_seconds: 60
  autofix: true
  max_repair_attempts: 5
```

Autofix is bounded and may repair project-owned runtime/service/config state. It does not wake/shut down hosts or issue destructive UPS commands simply to make health green.

## 10. Network dependencies

Use `network_dependencies` for switches, routers or infrastructure that must be ready before managed-host recovery.

Accepted v0.1 startup behavior:

- `auto-power` — device is expected to start automatically when backed power is available;
- `wait-only` — power is external/manual and the agent waits for readiness.

The schema also contains `startup: wol`, but **dependency WoL is not an accepted armed-v0.1 capability**. Armed validation fails closed until dependency actions have durable request/retry/reconciliation semantics equivalent to managed-host actions.

Example accepted dependency:

```yaml
network_dependencies:
  - id: core-switch
    startup: auto-power
    priority: 1
    address: 192.168.1.2
    status:
      method: tcp
      port: 22
```

## 11. Managed hosts

Each host may define:

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

Schema values include:

```text
auto
ping
tcp
arp
none
```

For **armed v0.1**, accepted deterministic verification paths are TCP/ping (or adapter-specific logic where implemented). ARP-only verification intentionally fails closed in armed mode. A host is not considered offline/online from one transient observation; configured consecutive verification is required.

### Shutdown methods

Schema values include:

```text
nut
ssh
command
none
```

Accepted armed-v0.1 behavior:

- `nut` — host participates in the NUT secondary/FSD shutdown group;
- `ssh` — fixed-argv constrained remote shutdown with bounded retry/reconciliation;
- `none` — observe/manage state without controller-issued shutdown.

`command` is **reserved but not accepted in armed v0.1**. No arbitrary shell source is executed, and no command registry is treated as complete until it has its own durable/safe execution model and acceptance suite.

Lower shutdown priority executes first for pre-FSD direct hosts. NUT secondaries form a synchronized group as described in `docs/NUT_SHUTDOWN_MODEL.md`.

### Restore policy

```text
previous-state
always
never
```

Default: `previous-state`. Unknown pre-outage state is not treated as online.

### Managed-host Wake-on-LAN

Managed-host wake supports MAC, interface, broadcast, UDP port, priority, inter-host delay and bounded retry count. Wake intent/attempt state is durable and reconciled after restart.

This managed-host implementation must not be confused with the currently unsupported dependency-WoL path.

## 12. Secrets

Long-lived secrets should be referenced from protected files where practical, for example:

```text
/etc/cockpit-ups-wol/secrets/
```

Expected protection:

```text
0700 secrets directory where practical
0600 private secret/key files
root or dedicated service ownership
```

Secrets are redacted from logs, diagnostics and Cockpit revision summaries.

## 13. Transactional activation

```text
candidate
-> syntax/schema/semantic validation
-> cross-reference and armed-capability validation
-> component preflight
-> atomic activation
-> service reload/restart
-> immediate health checks
-> probation
-> known-good OR rollback
```

A candidate is never promoted merely because the controller rebooted successfully.

`cockpit-ups-wolctl config-apply` starts activation/probation in a transient systemd unit so browser/channel loss does not terminate the transaction.

## 14. Validation examples

Validation rejects or prevents arming for conditions including:

- duplicate host/dependency IDs;
- unknown dependency references or recovery cycles;
- invalid MAC/IP/port values;
- TCP verification without a valid port;
- managed-host WoL enabled without the required MAC/broadcast data;
- SSH shutdown missing required user/key information;
- armed `shutdown.method: command`;
- armed ARP-only host verification;
- armed dependency WoL;
- unsafe/ambiguous NUT FSD ownership;
- restricted NUT policy that leaves an enabled address family unintentionally exposed;
- automatic controller shutdown assumptions without a verified output-return or alternate restart mechanism.

## 15. Important defaults

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
HOSTSYNC                     60 s
FINALDELAY                   15 s
health interval              60 s
config probation             60 s
restore policy               previous-state
```

## 16. Compatibility and migration

Migration creates a candidate revision rather than rewriting the only active copy in place. The migrated configuration becomes known-good only after validation/probation; otherwise the prior known-good revision remains/restores active.
