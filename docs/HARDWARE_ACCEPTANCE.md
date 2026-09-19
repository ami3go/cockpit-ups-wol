# Hardware Acceptance Protocol

Status: **required before v0.1 release; not yet executed on physical UPS hardware**.

This document defines the physical release gate for `cockpit-ups-wol`. Cloud CI, QEMU and NUT `dummy-ups` testing are necessary but do **not** replace this procedure.

## 1. Required representative platforms

At minimum, v0.1 must be exercised on:

| Platform | Required result | Current status |
|---|---|---|
| linux/amd64 controller + supported USB/NUT UPS | Full outage/recovery acceptance | NOT RUN |
| linux/arm64 controller + supported USB/NUT UPS | Full outage/recovery acceptance | NOT RUN |
| linux/riscv64 | Runtime smoke; physical UPS optional for v0.1 | QEMU runtime smoke automated; physical NOT RUN |
| Synology DSM as NUT secondary | Real NAS shutdown/recovery compatibility | NOT RUN |

Record exact controller model, OS/version, NUT version, UPS make/model/firmware and connection type for every run.

## 2. Safety setup

The controller SBC must be connected to a **battery-backed UPS output**. Required local networking (switch/router/VLAN path) must remain powered long enough for shutdown coordination. The test operator must have local console access and a method to restore utility input without relying on the managed network.

Do not begin with an unreviewed `armed` configuration. Start in `dry-run`, run the preflight below, review the sanitized plan, and only then move to `armed` for the controlled destructive test.

```bash
sudo ./scripts/test/hardware-preflight.sh --ups-target ups@localhost
```

The preflight is deliberately non-destructive. It checks architecture, service enablement/activity, agent IPC health, last-known-good configuration, NUT status/battery variables and the rendered power plan. It cannot prove physical outlet wiring or firmware auto-power-on; those remain manual checks.

## 3. Evidence to capture

Before each test, save:

```bash
uname -a
cat /etc/os-release
upsc ups@localhost
systemctl status cockpit-ups-wol-agent.service cockpit-ups-wol-health.timer
cockpit-ups-wolctl health
cockpit-ups-wolctl config-status
cockpit-ups-wolctl plan
journalctl -u cockpit-ups-wol-agent.service --since '-10 min'
```

After the test, preserve the full agent/NUT/system journal covering the outage and recovery transaction plus the resulting durable state/config revision metadata. Redact credentials before publishing logs.

## 4. Dry-run acceptance

With utility power present:

1. Verify NUT reports a trustworthy online state.
2. Verify Cockpit and `cockpit-ups-wolctl health` report healthy operation.
3. Verify `active == last-known-good`.
4. Review the shutdown order, NUT-managed hosts and restore order in the power plan.
5. Remove utility input while keeping the UPS output active.
6. Confirm the agent enters the on-battery state and records the outage without issuing destructive actions in `dry-run` mode.
7. Restore utility and confirm recovery gates are represented correctly.

Pass criteria: no shutdown/WoL side effects, no false `OL` or 100% battery assumptions, no state corruption, and no unexpected service restart loop.

## 5. Armed outage/shutdown acceptance

Only after the dry-run plan is approved:

1. Put the tested configuration into `armed` mode through the transactional configuration path.
2. Confirm the new revision completes probation and becomes last-known-good.
3. Start all devices expected to be restored; intentionally leave at least one configured `previous-state` host off.
4. Remove utility input.
5. Verify the persisted pre-outage snapshot matches actual online/offline state.
6. Allow the configured shutdown trigger to be reached.
7. Confirm direct-shutdown hosts are processed in deterministic priority order.
8. Confirm a per-host `requested` state exists durably before each direct shutdown action.
9. Confirm NUT-secondary hosts receive no duplicate SSH/direct shutdown.
10. Confirm FSD is requested only after pre-FSD hosts are settled.
11. Confirm the controller is the final managed system to shut down when the real NUT shutdown path requires it.
12. If the UPS supports verified load-off/return, confirm the configured NUT/system shutdown path owns that action; the long-running agent must not issue an independent `load.off`.

Pass criteria: no premature FSD, no duplicate shutdown, no lost durable transaction state and no unmanaged destructive command.

## 6. Interrupted shutdown / reboot acceptance

Repeat with controlled fault injection:

- Restart the agent while the UPS is on battery before shutdown commit.
- Restart the agent after `SHUTDOWN_COMMITTED`.
- Restart after a host has `shutdown=requested` but before verification completes.
- If safe on the chosen hardware, interrupt controller power during boot and restore it while the UPS remains in an outage/recovery context.

Pass criteria:

- a reboot never counts as proof of utility recovery;
- a previously persisted `ON_BATTERY` outage does not receive a fresh grace delay before threshold evaluation;
- a committed shutdown resumes rather than returning to normal;
- ambiguous requested actions are reconciled by status before retry;
- no host receives an unnecessary duplicate shutdown.

## 7. Recovery acceptance

Restore utility input after the shutdown transaction.

1. Confirm no wake occurs immediately merely because the controller boots.
2. Confirm NUT must report trustworthy online utility.
3. Confirm the full configured utility-stability timer completes without interruption.
4. Confirm the UPS recharge gate is met. Default release test: **battery charge >= 80%**.
5. Confirm required network dependencies pass consecutive readiness probes.
6. Confirm only hosts eligible under their restore policy are considered.
7. Confirm the first `wol_sent` state is durably persisted before the packet is sent.
8. Confirm hosts wake in dependency/priority order.
9. Confirm a host that was off before the outage stays off when using `previous-state`.
10. Confirm wake retries remain bounded and survive an agent restart.

Pass criteria: no early wake, no restoration of intentionally-off `previous-state` hosts, and transaction returns cleanly to `NORMAL` only after recovery is settled.

## 8. Power-bounce recovery acceptance

During `RECOVERY_WAIT`, briefly remove utility again and then restore it.

Pass criteria: the AC stability timer resets completely.

During `RESTORE_HOSTS`, remove utility again before all hosts have been restored.

Pass criteria: no further WoL packets are sent after unsafe power is observed; the persisted outage context remains recoverable on the next safe cycle.

## 9. Configuration rollback acceptance on hardware

With the stable hardware configuration tagged known-good:

1. Apply a deliberately invalid candidate through a safe test method that cannot damage the UPS/NUT host.
2. Verify it does not become known-good.
3. Verify the previous last-known-good configuration is restored automatically.
4. Verify services return healthy after rollback.
5. Reboot once and verify the rejected candidate is not promoted during startup.

Do not deliberately corrupt SSH/NUT credentials needed for emergency access during this test.

## 10. Synology DSM acceptance

Use a real DSM system configured as a NUT secondary/client of the controller.

Verify:

- UPS is exposed using the expected DSM-compatible identity/profile;
- DSM can monitor UPS status over TCP 3493;
- compatibility credentials are monitor-only (`monuser` / `secret` where required by the tested DSM version);
- DSM receives the NUT FSD path and performs its own graceful shutdown;
- the controller does not also send a duplicate direct SSH shutdown;
- after safe power recovery, WoL restores the NAS only when its restore policy requires it;
- DSM storage/services return cleanly after boot.

Record DSM version because compatibility behavior may vary by release.

## 11. Result record

Create one result block per physical run:

```text
Date/time UTC:
Tester:
Controller model:
Architecture:
OS/version:
Kernel:
NUT version:
UPS make/model:
UPS firmware:
UPS connection:
Battery age/type:
Managed switch/router:
Synology model/DSM version (if applicable):
Config revision:
Software commit/release:

Dry-run: PASS / FAIL
Armed shutdown: PASS / FAIL
Controller-last/FSD: PASS / FAIL / N/A
Interrupted shutdown/reboot: PASS / FAIL
Recovery >=80%: PASS / FAIL
Recovery power bounce: PASS / FAIL
Config rollback: PASS / FAIL
Synology secondary: PASS / FAIL / N/A

Evidence/log location:
Observed defects:
Final disposition: ACCEPTED / REJECTED
```

No platform should be marked accepted without retained evidence for the safety-critical sequence it exercised.
