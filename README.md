# cockpit-ups-wol

`cockpit-ups-wol` is a small homelab UPS-management appliance built around Network UPS Tools (NUT), a persistent safety agent, Wake-on-LAN, and Cockpit.

The project is currently **pre-release**. The v0.1 software baseline is implemented and continuously tested, including armed orchestration, Cockpit management, transactional installation/rollback and multi-architecture packaging. It is **not yet release-ready** because representative physical UPS/DSM acceptance and the project-license decision are still open.

## Goals

- share a UPS safely with Linux, Synology DSM and other network clients
- shut managed systems down in deterministic order during an outage
- shut the controller down last when required
- remember what was running before the outage
- resume safely after controller reboot or interrupted boot
- restore only eligible systems after utility power is stable and the UPS has recovered
- default recovery gate: **80% battery charge**
- provide Cockpit-based management without making Cockpit part of the safety-critical path
- automatically start required services after reboot
- continuously health-check the stack and perform bounded safe repairs
- make project-managed configuration changes transactional and automatically roll back failures

## Safety model

The controller SBC must be powered from a **battery-backed UPS output** unless it has an equivalently reliable independent backed supply, and it must boot automatically whenever UPS output returns. Required Ethernet switching/routing must remain powered long enough for shutdown coordination.

A boot is never treated as proof that utility power has recovered. Recovery requires valid NUT status, a stable-utility interval, the configured battery/runtime/recharge gate, network readiness, a known-good configuration and healthy runtime state.

New installations start in **dry-run** mode. Fresh installs intentionally contain **no managed hosts or network dependencies**; sample addresses and MACs from the example configuration are never copied into the live configuration. Enroll real devices, verify the rendered shutdown/recovery plan and physical topology, then arm automation.

## Current implementation

The v0.1 software baseline includes:

- Go safety agent plus `cockpit-ups-wolctl`, health helper and `wolctl`
- normalized NUT parsing with explicit `UNKNOWN` on communication failure
- primary/master ownership validation before FSD
- crash-safe state persistence with checksum and previous-generation fallback
- deterministic power-state machine and durable shutdown/recovery commit points
- pre-outage online snapshot and per-host durable action state
- ordered direct-host shutdown with fixed-argv SSH and consecutive state verification
- NUT-managed secondary separation so hosts such as Synology are not shut down twice
- controller/primary FSD ownership after pre-FSD hosts are settled
- Wake-on-LAN sender with durable bounded retry state and dependency-aware ordered recovery
- power-bounce/reboot reconciliation, including no fresh outage grace after a persisted `ON_BATTERY` reboot
- recovery gating on valid utility, stable AC, UPS charge/runtime/recharge policy, network readiness and health
- transactional configuration revisions with probation and last-known-good rollback
- durable health circuit breaker, conservative autofix and `FAILED_SAFE`
- Unix-socket IPC and systemd READY/watchdog integration
- Cockpit TypeScript/React/PatternFly UI with Overview, UPS, Devices, Automation, Reliability, Settings and Logs
- sanitized dry-run power-plan/status reporting and privileged confirmed configuration rollback
- transactional installer with service autostart, initial known-good bootstrap and application/config/Cockpit/firewall rollback
- guided TUI using `dialog` with `whiptail` fallback, sharing the same backend as default and silent installation
- local UPS discovery through NUT scanner output where available, plus explicit `--ups-driver` / `--ups-port` overrides
- trusted-LAN default and optional restricted NUT mode using an additive project-owned nftables table/service without flushing the administrator's firewall
- IPv4/IPv6 NUT listener generation tied to the selected network policy
- real Ubuntu 24.04 CI acceptance using NUT `dummy-ups`, systemd and Cockpit, including deliberately broken-upgrade rollback
- generated NUT primary/secondary + Synology monitor-only integration tests
- amd64 native runtime smoke plus arm64/riscv64 QEMU runtime smoke
- reproducible amd64/arm64/riscv64 appliance archives with SHA256 checksums and prebuilt Cockpit assets

Remaining v0.1 release gates:

- physical full outage/recovery acceptance on a representative amd64 controller + real UPS
- physical full outage/recovery acceptance on a representative arm64 controller + real UPS
- real Synology DSM NUT-secondary shutdown/recovery acceptance
- project license selection and root `LICENSE`

The exact physical protocol is in `docs/HARDWARE_ACCEPTANCE.md`; software/QEMU simulation is not presented as a substitute for those tests.

## Installation

The project uses one installer entry point:

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

The guided TUI covers system/package review, NUT profile, UPS selection, Synology compatibility, network security, outage/recovery policy, controller power topology, dependency/host enrollment guidance, initial operating mode, final review, progress and probation result.

Useful non-destructive checks:

```bash
./install.sh --check
./install.sh --check --profile remote-client
./install.sh --check --profile local-server --network-mode restricted --allow-client 192.168.1.0/24
sudo ./scripts/test/hardware-preflight.sh --ups-target ups@localhost
```

For unattended local-UPS installs, explicit hardware selection is available with:

```bash
sudo ./install.sh --silent --profile local-server \
  --ups-driver usbhid-ups --ups-port auto
```

If silent local discovery finds zero or multiple safe candidates and driver/port are not explicit, installation fails instead of guessing.

The installer is transactional for project-managed binaries, Cockpit assets, configuration, systemd units, project firewall state and NUT files. It snapshots the existing state, enables required services, performs runtime health probation, creates/promotes the initial known-good configuration only after success, and restores the prior application/config/service state when an installation or upgrade fails.

Existing non-empty NUT configuration is preserved rather than silently overwritten.

## NUT network modes

`trusted-lan` is the default and does not install project firewall restrictions.

`restricted` is optional for a local NUT server. It requires explicit allowed client CIDRs and uses a project-owned nftables table that only governs NUT TCP/3493. It does not flush or replace unrelated host firewall rules. If IPv6 NUT listening is enabled, IPv6 is restricted as well.

## Operating modes

- `monitor` — observe state without power actions
- `dry-run` — evaluate and display the plan without destructive/wake actions; default for a new installation
- `armed` — execute the validated persistent shutdown/recovery lifecycle
- `maintenance` — inhibit automatic power orchestration for maintenance workflows

The first-install TUI intentionally does not offer immediate `armed` mode. Arm only after real targets and physical topology have been validated.

Unsupported armed capabilities fail closed. In particular, arbitrary `command` shutdown and dependency Wake-on-LAN remain disabled until their own constrained/durable execution models are implemented.

## Synology

Synology DSM compatibility is built into the project but is **disabled by default**. The installer can explicitly enable the compatibility profile with:

```bash
sudo ./install.sh --synology
```

The preferred model is DSM as a NUT secondary/client. Where the tested DSM version requires the compatibility account, the generated profile uses UPS name `ups` and monitor-only `monuser` / `secret` credentials. The account receives `upsmon secondary` capability only; generated integration tests reject `actions`/`instcmds` privileges.

A Synology host configured with `shutdown.method: nut` is left to the NUT/FSD path; the agent does not send a duplicate direct SSH shutdown.

Real DSM hardware acceptance is still required before v0.1 release.

## Testing and release integrity

CI currently gates:

- Go unit tests
- strict canonical YAML integration
- installer-generated NUT/FSD/Synology integration
- `go vet`
- amd64/arm64/riscv64 builds
- native amd64 and QEMU arm64/riscv64 runtime smoke
- Bash installer/TUI/network/preflight syntax and installer backend tests
- discovery/config/firewall-plan regression tests
- real Ubuntu NUT `dummy-ups` + systemd + Cockpit installation/rollback acceptance
- Cockpit strict TypeScript/build validation
- reproducible multi-architecture package generation and SHA256 verification

Tagged release publication is intentionally blocked until a project license exists.

## Documentation

Key design and operating documents include:

- `SOFTWARE_ARCHITECTURE.md`
- `docs/INSTALLATION_REQUIREMENTS.md`
- `docs/RELIABILITY_REQUIREMENTS.md`
- `docs/BOOT_RECOVERY_REQUIREMENTS.md`
- `docs/CONFIGURATION.md`
- `docs/STATE_MODEL.md`
- `docs/NUT_SHUTDOWN_MODEL.md`
- `docs/POWER_POLICY.md`
- `docs/IPC.md`
- `docs/DEPLOYMENT.md`
- `docs/OPERATING_MODES.md`
- `docs/SECURITY.md`
- `docs/TEST_PLAN.md`
- `docs/HARDWARE_ACCEPTANCE.md`
- `docs/NUT.md`
- `docs/SYNOLOGY.md`
- `docs/RELATED_PROJECTS.md`
- `docs/READINESS_AUDIT.md`
- `ROADMAP.md`

## License

A project license has not yet been selected. Do not assume permission to reuse project source code until a root `LICENSE` file is added.

Upstream projects reviewed for possible reuse and their licenses are tracked in `THIRD_PARTY_NOTICES.md` and `docs/RELATED_PROJECTS.md`. No upstream source should be copied/adapted until the root license and the corresponding third-party obligations are resolved.
