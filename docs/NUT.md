# NUT Integration Guide

## Supported v0.1 profiles

```text
local-server
remote-client
existing
```

### local-server

The controller runs the UPS driver, `upsd`, and primary `upsmon` for a locally attached UPS.

The project agent reads NUT state and may request FSD through the validated primary `upsmon`, but does not replace NUT's shutdown protocol.

### remote-client

The controller reads UPS state from a remote `upsd` server.

It is not automatically primary and may not issue remote FSD/output-off in v0.1.

### existing

The installer integrates conservatively with a pre-existing NUT installation and avoids blind file replacement.

## Normal status interpretation

Common tokens include:

```text
OL
OB
LB
FSD
CHRG
DISCHRG
BYPASS
OVER
OFF
```

Compound strings are expected, e.g. `OL CHRG` and `OB LB`.

The project retains raw tokens but also normalizes them for the state machine.

Failed query or absent status becomes `UNKNOWN`.

## Network service

Default NUT port:

```text
TCP 3493
```

Default project security mode is `trusted-lan`, assuming an upstream home/lab firewall.

Optional `restricted` mode adds explicit host/subnet restrictions while preserving unknown existing firewall policy.

## Primary/secondary shutdown

For a locally attached UPS:

```text
controller = NUT primary
Synology/Linux network clients = NUT secondary
```

Detailed ownership and FSD behavior: `docs/NUT_SHUTDOWN_MODEL.md`.

## Administrative commands

Ordinary network clients receive monitoring-only access.

Destructive or writable NUT commands require the local privileged project path and explicit authorization.

The health supervisor never invokes destructive instant commands.

## Driver/service management

NUT service names vary by distribution and NUT packaging.

The installer must discover distro-appropriate units and avoid running manual driver instances that conflict with systemd-managed driver services.

## UPS output-cycle classification

Deployments record:

```text
POWER_CYCLE_VERIFIED
POWER_CYCLE_UNVERIFIED
MONITOR_ONLY
```

A real hardware shutdown-return test is required before a UPS profile should be considered verified for unattended controller power-off/reboot recovery.

## Remote NUT communication failure

A lost remote connection is:

```text
UNKNOWN
```

not `OL`.

The prior outage context remains durable while connectivity is uncertain.

## Configuration preservation

The installer detects existing NUT configuration and backs up any file it is explicitly authorized to modify.

Unknown/newer/custom configuration is not destructively replaced merely to make the project fit.

## Diagnostics

Typical manual read-only checks include:

```bash
upsc -l <server>
upsc ups@<server>
```

Exact diagnostic commands shown in Cockpit/installer should be argv-safe and redact credentials.

## References

Official NUT documentation remains authoritative for NUT semantics:

- https://networkupstools.org/
- https://networkupstools.org/docs/man/upsmon.html
- https://networkupstools.org/docs/man/upsmon.conf.html
- https://networkupstools.org/docs/man/upsdrvctl.html
- https://networkupstools.org/docs/man/ups.conf.html
