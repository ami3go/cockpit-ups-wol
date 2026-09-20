# cockpit-ups-wol — Readiness Audit

**Audit date:** 2026-09-20  
**Status:** Post-implementation v0.1 release-gate review  
**Scope:** architecture, implementation, installation, reliability, interrupted-boot recovery, Cockpit, testing, packaging, hardware acceptance and release governance

## 1. Executive verdict

The **v0.1 software baseline is implemented and automated acceptance is green**. The project has moved beyond implementation readiness and is now at the **physical validation / release-governance** stage.

The repository should **not yet publish a tagged v0.1 release**. Two categories of release blockers remain:

1. **real hardware acceptance** — issue #10 remains open until amd64 + real UPS, arm64 + real UPS, and real Synology DSM NUT-secondary tests have retained evidence;
2. **project license selection** — issue #12 remains open until a root `LICENSE` is selected and reuse/distribution implications are reviewed.

No unresolved software P0 blocker from the original readiness audit remains.

## 2. Current readiness by area

| Area | Status | Evidence / note |
|---|---|---|
| Architecture / state model | READY | canonical power, health and operating-mode models implemented |
| NUT ownership / FSD | READY | primary/master validation and generated-config integration tests |
| Persistent state | READY | checksum, fsync, atomic rotation, previous-generation fallback |
| Interrupted boot / power bounce | READY | automated reconciliation and fault tests |
| Armed orchestration | READY | durable pre-side-effect ordering, restart reconciliation, recovery gating |
| Host shutdown / WoL | READY | constrained SSH, NUT separation, verification, durable WoL retries |
| Configuration transactions | READY | candidate/probation/LKG/rollback implemented and tested |
| Health / autofix | READY | watchdog, bounded repair circuit breaker, `FAILED_SAFE` |
| Cockpit management | READY | read/status/plan/log pages and privileged confirmed config rollback |
| Installer | READY | default/silent/TUI share one transactional backend |
| UPS discovery | READY | NUT scanner parsing + explicit driver/port + ambiguity failure |
| NUT network policy | READY | trusted-LAN default + additive restricted nftables mode |
| Service autostart | READY | systemd enable/start validation and health probation |
| Application rollback | READY | binaries/config/Cockpit/systemd/NUT/firewall rollback |
| Synology software integration | READY | monitor-only generated account and NUT-secondary ownership tests |
| amd64 software runtime | READY | native runtime + Ubuntu systemd/NUT E2E |
| arm64 software runtime | READY | build + QEMU runtime smoke |
| riscv64 software runtime | READY | build + QEMU runtime smoke |
| Packaging/checksums | READY | reproducible multi-arch bundles + SHA256SUMS |
| Real UPS amd64 | BLOCKED / NOT RUN | physical acceptance required |
| Real UPS arm64 | BLOCKED / NOT RUN | physical acceptance required |
| Real Synology DSM | BLOCKED / NOT RUN | physical DSM acceptance required |
| Project license | BLOCKED | human project decision required |

## 3. Original P0 audit blockers — closure

The original audit identified these blockers. Their current status is:

- **Canonical architecture synchronization — COMPLETE.** `BOOT_RECONCILE`, commit points, `FAILED_SAFE`, health supervision and operating modes are canonical.
- **NUT shutdown ownership — COMPLETE.** Agent owns policy/orchestration; NUT primary/upsmon owns FSD/controller shutdown; late NUT shutdown path owns UPS output behavior.
- **Trigger precedence/hysteresis — COMPLETE.** `FSD`, `LB`, charge/runtime/time triggers, `UNKNOWN`, power bounce and recovery-entry hysteresis are deterministic.
- **Canonical configuration schema — COMPLETE.** YAML + JSON Schema + semantic validation exist.
- **Canonical persistent state schema — COMPLETE.** Durable outage/action identity and per-host state are implemented.
- **Cockpit ↔ agent control boundary — COMPLETE.** Local Unix socket + CLI/control boundary; Cockpit does not manipulate power state files directly.
- **Implementation stack decision — COMPLETE.** Go agent/CLI, React/TypeScript/PatternFly Cockpit, Bash installer, systemd/journald runtime.
- **Controller UPS-backed topology — COMPLETE.** Normative deployment requirement and hardware preflight exist.
- **Implementation roadmap/backlog — COMPLETE.** GitHub issues and `ROADMAP.md` are active.
- **Installer synchronization — COMPLETE.** Service autostart, probation, LKG, interrupted upgrade rollback and TUI/discovery/network policy are implemented.

## 4. Installer readiness

The single entry point remains:

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

Implemented installer properties:

- supported platform/architecture detection;
- clean-OS dependency installation;
- prebuilt release artifacts with source-build fallback for development;
- local-server, remote-client and existing-NUT profiles;
- `--ups-driver` / `--ups-port` explicit selection;
- NUT USB discovery when available;
- no silent guessing when discovery is missing/ambiguous;
- Synology compatibility opt-in;
- trusted-LAN default;
- optional restricted NUT mode using a dedicated `inet cockpit_ups_wol` nftables table;
- no `flush ruleset` or replacement of unrelated administrator firewall state;
- IPv4/IPv6 NUT listeners tied to policy;
- guided `dialog` TUI with `whiptail` fallback;
- transactional application/config/service/NUT/Cockpit/firewall state;
- automatic service enablement;
- health/probation gate;
- initial known-good revision only after probation;
- failed upgrade restoration;
- idempotent reinstall acceptance.

Fresh installations intentionally contain:

```yaml
network_dependencies: []
hosts: []
```

Example IP/MAC targets are documentation only and are never copied into the live generated configuration.

## 5. Automated acceptance status

Automated coverage currently includes:

- Go unit tests and `go vet`;
- canonical YAML integration;
- config candidate/probation/rollback/reboot reconciliation;
- corrupt/torn state fallback;
- outage/recovery lifecycle simulation;
- power bounce and interrupted boot;
- reboot from persisted `ON_BATTERY` without a new grace period;
- recovery stability timer restart after reboot;
- durable action ordering before SSH/FSD/WoL side effects;
- ambiguous shutdown-request restart reconciliation;
- generated NUT primary/secondary + Synology privilege tests;
- installer discovery parsing and ambiguity rules;
- generated NUT/project driver+port consistency;
- restricted firewall plan generation and no-firewall-flush assertion;
- fresh-install no-sample-target regression test;
- installer option/self-check matrix including `--tui` and restricted mode;
- real Ubuntu 24.04 NUT `dummy-ups` + systemd + Cockpit install/probation/idempotency/broken-upgrade rollback;
- amd64 native runtime smoke;
- arm64 QEMU runtime smoke;
- riscv64 QEMU runtime smoke;
- Cockpit strict TypeScript/build validation;
- reproducible package/checksum verification.

## 6. Physical release gate

Software simulation cannot establish electrical behavior. The release candidate must run the protocol in `docs/HARDWARE_ACCEPTANCE.md`.

Required evidence:

```text
[ ] amd64 controller + supported real UPS
[ ] arm64 controller + supported real UPS
[ ] real Synology DSM configured as NUT secondary
```

Each UPS run must cover at least:

- controller and required network devices on battery-backed outputs;
- real OB detection;
- shutdown ordering;
- controller/NUT primary behavior;
- interrupted/repeated boot behavior where practical;
- utility restoration;
- stable-AC gate;
- battery recovery gate (default 80%);
- ordered WoL restoration;
- controller automatic boot when UPS output returns;
- retained logs/config/state evidence.

Use `scripts/test/hardware-preflight.sh` before the destructive test. A software/QEMU result must never be recorded as a physical PASS.

## 7. Synology release gate

Software configuration is ready, but real DSM behavior is still unverified for release.

The hardware acceptance must confirm:

- DSM connects to the generated NUT service;
- account remains monitor-only;
- DSM shuts itself down from the NUT-secondary path;
- agent does not duplicate shutdown with SSH;
- recovery/WoL behavior works for the selected NAS/DSM configuration;
- actual DSM credentials/behavior match the documented compatibility profile for the tested version.

## 8. Release governance / license

The repository currently has **no selected root project license**.

Until issue #12 is resolved:

- tagged public release publication remains blocked;
- README continues to state that reuse permission should not be assumed;
- upstream projects remain references unless their code has been explicitly reviewed for compatible reuse;
- `THIRD_PARTY_NOTICES.md` remains the reuse inventory.

Selecting the project license is intentionally not automated because it is a project-owner/legal decision.

## 9. Known non-blocking v0.1 limitations

These are not release-safety blockers for the defined v0.1 scope:

- no multi-UPS policy;
- no Proxmox-specific API adapter yet;
- no arbitrary shell shutdown command execution;
- no dependency WoL until dependency actions receive the same durable semantics as host actions;
- no advanced writable UPS command UI;
- no native Go NUT protocol client requirement;
- no Prometheus/notification/history subsystem;
- Cockpit device/config CRUD can be expanded after the safe baseline.

They are tracked as v0.2/v0.3 work rather than being silently treated as missing v0.1 safety features.

## 10. Final release checklist

A v0.1 tag is allowed only when all boxes below are satisfied:

```text
[x] architecture and safety model implemented
[x] armed orchestration implemented
[x] automatic service startup / health / autofix implemented
[x] configuration LKG / rollback implemented
[x] boot interruption / power-bounce handling implemented
[x] Synology software integration implemented
[x] transactional installer + TUI implemented
[x] UPS discovery / explicit selection implemented
[x] optional restricted NUT networking implemented
[x] Cockpit management implemented
[x] multi-arch build/package/checksum pipeline implemented
[x] automated software acceptance green
[ ] physical amd64 UPS acceptance retained
[ ] physical arm64 UPS acceptance retained
[ ] real Synology DSM acceptance retained
[ ] root LICENSE selected and added
```

**Current verdict:** software candidate ready for physical acceptance; public v0.1 release remains blocked by the four unchecked release gates above.
