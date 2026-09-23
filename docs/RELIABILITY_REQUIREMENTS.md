# cockpit-ups-wol — Reliability, Health and Configuration Rollback Requirements

**Status:** Normative v0.1 implementation requirement

## 1. Reliability objectives

The stack must remain predictable after reboot, process crash, invalid configuration, dependency loss and power interruption.

Required behavior:

- required runtime services start automatically;
- process/service recovery is bounded and observable;
- power/outage state survives restart;
- health is checked continuously;
- automatic repair is conservative and bounded;
- project-managed configuration changes are transactional;
- failed configuration/upgrade activation rolls back to a verified state;
- successfully validated configuration is tagged durably as known-good;
- the last-known-good revision survives reboot, power loss and normal upgrade.

Unknown state is never treated as healthy merely to keep automation moving.

## 2. Required services and systemd

For a local-server profile the selected distro-equivalent units normally include:

```text
cockpit.socket
NUT driver unit(s)
nut-server.service
nut-monitor.service
cockpit-ups-wol-agent.service
cockpit-ups-wol-health.timer
```

Restricted NUT mode additionally uses the project firewall service. Interrupted-install recovery uses its dedicated boot recovery unit/marker path.

Exact NUT unit names are distro-aware.

Persistent services use bounded systemd restart/watchdog policy appropriate to their role. Dependency delay (USB/network/NUT readiness) is handled with retry/backoff rather than an unlimited tight restart loop.

Real installation requires a functional systemd system manager with systemd as PID 1. Unsupported container/chroot contexts are rejected before install transaction state is created.

## 3. Power recovery after reboot

Automatic power recovery is enabled by default, subject to configuration and safety gates.

Default policy:

```text
utility stable period     120 seconds
minimum UPS charge        80%
network readiness wait    300 seconds
```

Every agent start enters boot reconciliation. Boot itself is never proof of restored utility.

A persisted active outage resumes without a fresh grace period. An unfinished stable-utility timer is re-proven conservatively after uncontrolled restart. A power failure during partial host restoration creates a new outage epoch and may require already-restored hosts to shut down again.

## 4. Health supervision

`cockpit-ups-wol-health` is an independent short-lived checker/repair helper driven by a systemd timer.

Checks include, as applicable:

- required project/NUT/Cockpit services enabled/active;
- agent watchdog/IPC health;
- expected NUT availability/profile behavior;
- active configuration integrity and revision metadata;
- durable state readability/writability;
- project directories/permissions;
- required network/helper/SSH-key dependencies.

A disconnected UPS is distinguished from a broken NUT installation.

Overall health states include:

```text
HEALTHY
DEGRADED
RECOVERING
CONFIG_VALIDATING
ROLLING_BACK
FAILED_SAFE
```

The agent's UPS/runtime health and the external system-service health helper use separate durable health-state files so independent writers do not corrupt each other's state.

## 5. Automatic repair boundary

Safe automatic repair may:

- restart/re-enable required project-managed services;
- restart selected NUT services after confirmed transient failure;
- recreate/repair project-owned runtime directories/permissions;
- restore a project-owned last-known-good configuration;
- resume/reconcile persisted runtime state.

It does not automatically:

- replace unknown administrator NUT/firewall/network configuration;
- regenerate credentials;
- accept changed SSH host keys;
- issue destructive UPS instant commands;
- wake/shut down hosts merely to make health pass.

Repair attempts are bounded by the configured circuit breaker. Exhaustion enters durable `FAILED_SAFE` rather than an endless restart loop.

## 6. Transactional configuration

All project-mediated configuration changes use one revision manager:

```text
request
-> lock
-> create candidate
-> syntax/schema/semantic validation
-> armed capability/safety validation
-> component preflight
-> atomic activation
-> reload/restart affected components
-> immediate health check
-> probation
-> KNOWN-GOOD or ROLLBACK
```

Known-good revisions are immutable.

Power interruption during candidate activation/probation never promotes the candidate merely because the machine later boots.

Startup reconciles the active configuration bytes/manifest and unfinished activation metadata before normal power automation proceeds. If necessary it restores last-known-good.

## 7. Current CLI/control commands

The supported v0.1 management CLI is `cockpit-ups-wolctl`.

Relevant commands:

```bash
cockpit-ups-wolctl health
cockpit-ups-wolctl config-get
cockpit-ups-wolctl config-validate       # candidate YAML on stdin
sudo cockpit-ups-wolctl config-apply     # candidate YAML on stdin
cockpit-ups-wolctl config-status
sudo cockpit-ups-wolctl config-rollback <revision>
cockpit-ups-wolctl plan
cockpit-ups-wolctl logs
```

Older conceptual examples such as `cockpit-ups-wol config list/show/rollback` are not current command syntax.

`config-apply` hands activation/probation to a transient systemd service so a closed browser/Cockpit channel does not terminate the transaction.

## 8. Revision storage

Active configuration remains under:

```text
/etc/cockpit-ups-wol/config.yaml
```

Revision history lives under:

```text
/var/lib/cockpit-ups-wol/config-history/
```

The manager maintains logical active/last-known-good/previous-known-good metadata and content hashes/manifests. Critical revision metadata is written with power-loss-resistant temp-write/fsync/rename/parent-fsync semantics.

## 9. Upgrade/installation transaction reliability

Installer/upgrade transactions preserve prior project-owned artifacts/config/service state before replacement.

Normal failure restores the previous coherent state and validates it.

Sudden power loss is covered by a durable pending-install marker and boot-time recovery guard. On the next boot, the pre-install snapshot is restored/reconciled before ordinary automation starts.

No install/upgrade is considered successful until service health/probation succeeds and the corresponding configuration/application state is known-good.

## 10. FAILED_SAFE

`FAILED_SAFE` is a durable inhibited state for conditions where trustworthy control cannot be restored automatically, for example:

```text
no valid persistent state generation
no usable known-good config
ambiguous/unavailable primary FSD ownership
unresolved direct-host action after bounded retries
repeated repair exhaustion
```

Monitoring remains available where practical. Acknowledgement alone does not clear the underlying condition.

## 11. Accepted/future capability boundary

Reliability guarantees apply to the accepted v0.1 action set. Armed v0.1 intentionally rejects:

```text
shutdown.method: command
ARP-only verification
dependency Wake-on-LAN
```

Those capabilities do not gain reliability guarantees merely because the schema can represent them.

## 12. Acceptance

Required acceptance covers service crash/watchdog recovery, disabled required services, NUT loss, health repair circuit breaking, state/config corruption fallback, candidate rollback, rollback after reboot, interrupted installer recovery, idempotent reinstall, browser-independent config activation and preservation of the active outage/recovery transaction across restart.

Physical electrical behavior is covered separately by `docs/HARDWARE_ACCEPTANCE.md`.
