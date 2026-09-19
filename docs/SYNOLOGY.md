# Synology DSM Integration

**Status:** v0.1 compatibility baseline

## 1. Design goal

Synology NAS is a first-class NUT secondary/client target.

The preferred shutdown path is DSM's native network UPS/NUT integration rather than duplicate SSH shutdown commands.

## 2. Compatibility preset

The project preset uses:

```text
UPS name       ups
NUT port       3493
monitor user   monuser
password       secret
role           upsmon secondary
```

Older NUT syntax may use `slave` where required by the installed version.

## 3. Security model

The compatibility account is monitor-only.

It SHALL NOT receive:

```text
SET
FSD
instcmds = ALL
```

or other administrative rights.

Because `monuser/secret` is a compatibility convention rather than a strong secret, deployments rely on trusted/restricted LAN controls and no public Internet exposure of NUT TCP 3493.

## 4. Shutdown ownership

For a host configured:

```yaml
shutdown:
  method: nut
```

DSM owns its local shutdown behavior after receiving critical/FSD state from NUT.

The agent SHALL NOT also issue a separate SSH shutdown to the same NAS.

## 5. Controller relationship

Typical topology:

```text
UPS USB → controller SBC
controller NUT primary / upsd
        ↓ TCP 3493
Synology DSM NUT secondary
```

During committed shutdown:

```text
agent performs any required pre-FSD actions
→ primary upsmon enters FSD
→ DSM secondary receives FSD/critical state
→ DSM begins shutdown
→ controller primary shuts down last
```

## 6. Recovery

DSM recovery is normally Wake-on-LAN if the NAS model/firmware/network configuration supports it.

Recommended host config:

```yaml
restore_policy: previous-state
wake:
  enabled: true
  mac: "AA:BB:CC:DD:EE:FF"
```

The controller waits for utility stability and the configured UPS recharge gate before sending WoL.

## 7. DSM configuration expectations

The user must configure DSM to use the controller as its network UPS server and, where DSM requires it, allow the controller/server relationship in DSM's UPS settings.

Exact DSM UI wording may vary by release/model, so the project UI should present compatibility values and diagnostics rather than hard-code assumptions about a specific DSM screen layout.

## 8. Readiness checks

The project should verify:

```text
controller NUT server listening on TCP 3493
UPS name = ups
compatibility user exists when enabled
user is monitor-only
NAS address is reachable when expected
NAS status check configured
WoL MAC/broadcast validated when automatic restore enabled
```

It cannot reliably prove every DSM-side setting remotely, so the TUI/Cockpit wizard should include a user-confirmed DSM setup step.

## 9. Shutdown verification

A NAS is not considered safely shut down based on one failed ping.

Use a configured verification method, preferably an application/management TCP port plus repeated checks.

Default concept:

```text
3 consecutive offline observations
5 s interval
```

The NUT-secondary protocol remains the shutdown trigger; status checks are verification only.

## 10. WoL verification

After WoL:

- retry is bounded
- check real NAS readiness
- require consecutive online observations
- persist `wol_sent` before/after retries as defined by the state model

If the NAS was known to be off before the outage and restore policy is `previous-state`, it is not automatically awakened.

## 11. Failure cases

### NAS cannot reach NUT server

Report degraded/unsafe Synology integration; do not pretend DSM is protected.

### NAS does not shut down after FSD

Primary NUT synchronization remains bounded by `HOSTSYNC`; the project records the failure prominently. Do not bypass the situation by immediately cutting UPS output from the agent.

### WoL unsupported

Set:

```yaml
wake:
  enabled: false
```

Recovery becomes manual or relies on another verified NAS power-on mechanism.

## 12. Acceptance test

Before marking Synology integration supported for a tested DSM model/version:

1. DSM reads UPS status from controller.
2. OL/OB changes are visible as expected.
3. FSD/critical shutdown causes safe DSM shutdown.
4. compatibility account cannot perform admin NUT actions.
5. controller remains primary/last.
6. after valid recovery gate, WoL restores a previously-running NAS when supported.
7. repeated controller reboot/power bounce does not wake NAS prematurely.

Record DSM version and NAS model in hardware acceptance results.
