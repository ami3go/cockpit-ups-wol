# cockpit-ups-wol

Cockpit-managed UPS shutdown and recovery orchestration for homelabs and small protected networks.

## Status

**Pre-alpha / implementation starting.**

The safety architecture and requirements are defined; the runtime implementation is being built. Do not rely on the repository yet for production shutdown protection.

## Goals

`cockpit-ups-wol` is designed to:

- use Network UPS Tools (NUT) for UPS communication
- expose management through Cockpit
- support Synology DSM as a first-class NUT client
- shut managed hosts down safely during a power outage
- shut the controller/NUT primary down last
- survive service crashes, controller reboots and interrupted boots
- automatically recover after utility is proven stable
- wait until the UPS has recharged to a configurable threshold, default **80%**
- restore only eligible hosts in controlled order using Wake-on-LAN
- automatically start and health-check required services
- transactionally validate configuration and roll back failed changes

## Core architecture

```text
UPS
 │
 ▼
NUT
 │
 ▼
cockpit-ups-wol-agent
 ├─ outage/recovery state machine
 ├─ host orchestration
 ├─ persistent crash-safe state
 ├─ config revision + rollback
 ├─ health supervision
 └─ WoL

Cockpit ── local authenticated IPC ──► agent
```

Cockpit is management-only. The automatic safety lifecycle does not require a browser or a functioning Cockpit UI.

## Controller deployment

The controller SBC must normally be powered from a **battery-backed UPS output**.

The required LAN path—especially the switch, and router/VLAN infrastructure when needed—must remain available long enough for shutdown coordination.

The controller must auto-boot when backed output power returns.

See `docs/DEPLOYMENT.md`.

## Recovery defaults

```text
Auto recovery        enabled
Stable utility       120 s
Minimum UPS charge   80%
Network wait         300 s
```

A boot is never treated as proof that utility has recovered. Every agent start enters `BOOT_RECONCILE` and waits for trustworthy NUT state.

## Operating modes

```text
monitor
 dry-run
 armed
 maintenance
```

New installations default to **dry-run**. The calculated shutdown/recovery plan must be reviewed before automation is armed.

## Synology

Compatibility baseline:

```text
UPS name       ups
NUT port       3493
monitor user   monuser
password       secret
role           secondary
```

The compatibility user is monitor-only. DSM normally performs its own shutdown through its native NUT integration.

See `docs/SYNOLOGY.md`.

## Safety model

Important rules:

- NUT communication failure becomes `UNKNOWN`, never `OL`.
- Missing battery charge never becomes 100%.
- Shutdown and recovery have durable commit points.
- FSD is owned by the validated NUT primary path.
- The long-running agent does not directly perform late UPS output power-off.
- Config changes become known-good only after runtime health probation.
- Failed config changes roll back automatically.
- Unresolved safety uncertainty enters `FAILED_SAFE` and inhibits destructive automatic actions.

## Implementation stack

```text
Agent / CLI      Go
WoL helper       Go
Cockpit UI       TypeScript + React + PatternFly
Installer        Bash
UPS backend      NUT
Service manager  systemd
Logs             journald
```

Target architectures:

```text
amd64
arm64
riscv64
```

## Documentation

- `SOFTWARE_ARCHITECTURE.md` — canonical architecture
- `docs/INSTALLATION_REQUIREMENTS.md` — installer/autostart/upgrade requirements
- `docs/RELIABILITY_REQUIREMENTS.md` — health/autofix/config rollback
- `docs/BOOT_RECOVERY_REQUIREMENTS.md` — interrupted-boot/brownout behavior
- `docs/NUT_SHUTDOWN_MODEL.md` — FSD/primary/secondary/output-off ownership
- `docs/POWER_POLICY.md` — trigger precedence and hysteresis
- `docs/CONFIGURATION.md` — canonical user configuration
- `docs/STATE_MODEL.md` — durable outage/recovery state
- `docs/IPC.md` — Cockpit/CLI ↔ agent control interface
- `docs/DEPLOYMENT.md` — UPS/network topology
- `docs/SECURITY.md` — security and secret handling
- `docs/SYNOLOGY.md` — DSM integration
- `docs/NUT.md` — NUT profiles and behavior
- `docs/TEST_PLAN.md` — release acceptance matrix
- `docs/RELATED_PROJECTS.md` — upstream reuse research
- `docs/READINESS_AUDIT.md` — implementation readiness review
- `ROADMAP.md` — implementation checklist

## Canonical schemas

```text
schemas/config.schema.json
schemas/state.schema.json
```

Reference config:

```text
config/config.yaml.example
```

## Planned install interface

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

The installer is not implemented yet.

## Project roadmap

See `ROADMAP.md` and the GitHub issues for v0.1 implementation epics.

## Contributing / code reuse

The project is evaluating selected patterns/code from NUT ecosystem, WOLNUT, Nutcracker, Eneru, Cockpit UPSide, Trugamr/wol and other projects documented in `docs/RELATED_PROJECTS.md`.

No upstream source should be copied until license compatibility and attribution are recorded in `THIRD_PARTY_NOTICES.md`.

## License

**Project license has not yet been selected.**

This is an explicit pre-release blocker. Do not assume that repository contents have a permissive license until a root `LICENSE` file is added.
