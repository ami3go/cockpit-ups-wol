# cockpit-ups-wol — Readiness Audit

**Audit date:** 2026-09-20  
**Status:** Post-deep-review v0.1 release-gate review  
**Scope:** architecture, runtime safety, installation, interrupted-power recovery, NUT/Synology, Cockpit management, health/autofix, testing, packaging and release governance

## 1. Executive verdict

The **v0.1 software baseline is implemented and automated acceptance is green**. The deep code review identified several runtime and crash-consistency defects; those software findings have now been fixed through isolated pull requests with regression coverage and CI validation.

No unresolved software-runtime P0/P1 blocker from that review remains. The project is now at the **physical validation / release-governance** stage.

A tagged public v0.1 release remains blocked by:

1. **real hardware acceptance** — issue #10: amd64 + real supported UPS, arm64 + real supported UPS, and real Synology DSM NUT-secondary acceptance with retained evidence;
2. **main-branch protection** — issue #39: repository-admin configuration requiring PRs and green CI for `main`. The connected automation can create and merge PRs but cannot administer branch-protection settings.

## 2. Current readiness by area

| Area | Status | Evidence / note |
|---|---|---|
| Architecture / state model | READY | canonical power, health and operating-mode models implemented |
| Outage safety policy | READY | `recovery.enabled` enforced; OL cancellation debounce; bounded communication loss; durable max-on-battery elapsed time |
| Recovery safety policy | READY | stable-utility/recharge/network/health gates; network timeout; post-commit health/network re-checks |
| NUT ownership / FSD | READY | primary/master validation, multi-primary fail-closed behavior, generated integration tests |
| NUT secondary synchronization | READY | canonical `HOSTSYNC` and `FINALDELAY` generated from project config |
| Persistent state | READY | checksum, fsync, atomic rotation, validated previous-generation preservation |
| Interrupted boot / power bounce | READY | boot reconciliation, renewed-outage transaction, re-shutdown of already-restored hosts |
| Host shutdown | READY | constrained SSH/command execution, ambiguous-state reconciliation, bounded retries, `FAILED_SAFE` on exhaustion |
| WoL / recovery sequencing | READY | durable wake attempts, dependency ordering, configured inter-host wake delay, reboot-safe conservative delay |
| Configuration validation | READY | strict YAML schema/semantic validation plus dependency-cycle/timing checks |
| Configuration transactions | READY | candidate/probation/LKG/rollback, startup interrupted-transaction recovery and active-content verification |
| Health / autofix | READY | watchdog, bounded repair circuit, project-owned service supervision, separate durable health state files |
| Cockpit management | READY | status/plan/logs plus privileged transactional YAML validate/apply/rollback workflow |
| Installer | READY | default/silent/TUI share one transactional backend |
| Interrupted installation | READY | fsynced pending marker, boot-time rollback guard, retryable interrupted recovery |
| UPS discovery | READY | NUT scanner parsing + explicit driver/port + ambiguity failure |
| NUT network policy | READY | trusted-LAN default + additive restricted nftables mode |
| Service autostart | READY | systemd enable/start validation and health probation |
| Synology software integration | READY | monitor-only NUT-secondary account and ownership tests |
| amd64 software runtime | READY | native runtime + Ubuntu systemd/NUT E2E |
| arm64 software runtime | READY | build + QEMU runtime smoke |
| riscv64 software runtime | READY | build + QEMU runtime smoke |
| Build supply chain | READY | GitHub Actions pinned to immutable SHAs; Dependabot enabled |
| npm reproducibility | READY | committed `package-lock.json`; CI/package builds use `npm ci` |
| Packaging/checksums | READY | reproducible multi-arch bundles + SHA256SUMS |
| Real UPS amd64 | BLOCKED / NOT RUN | physical acceptance required |
| Real UPS arm64 | BLOCKED / NOT RUN | physical acceptance required |
| Real Synology DSM | BLOCKED / NOT RUN | physical DSM acceptance required |
| Project license | READY | GNU AGPL-3.0-or-later; canonical root `LICENSE` present |
| `main` protection | BLOCKED / ADMIN | issue #39; repository administration required |

## 3. Deep-review findings — closure

The 2026-09-20 deep review found the following implementation gaps. Their current status is:

- **Automatic recovery disable switch — COMPLETE.** `recovery.enabled: false` is a hard policy gate, including boot reconciliation and stale persisted recovery state.
- **Power failure during partial recovery — COMPLETE.** A new outage creates a fresh transaction and host snapshot, so an already-restored host is eligible for shutdown again.
- **State-generation crash consistency — COMPLETE.** A corrupt current generation is never rotated over the only valid previous generation; directory durability is preserved across rotation/activation steps.
- **Interrupted config activation — COMPLETE.** Agent startup reconciles config history before parsing active YAML and verifies active bytes against the revision manifest, restoring LKG when required.
- **Direct shutdown failure handling — COMPLETE.** Transient failures receive bounded retries after reconciliation; FSD cannot advance past unresolved direct hosts; exhausted attempts enter durable `FAILED_SAFE`.
- **Pre-commit utility hysteresis — COMPLETE.** One transient `OL` sample no longer cancels an outage; two consecutive valid online observations are required.
- **UPS communication loss during outage — COMPLETE.** `communication_loss_grace_seconds` is enforced rather than allowing indefinite uncertainty.
- **Maximum-on-battery continuity across reboot — COMPLETE.** Monotonic elapsed outage progress is durably checkpointed and resumed without relying on a trustworthy RTC.
- **Recovery network timeout — COMPLETE.** `network_wait_seconds` is enforced and unresolved critical recovery readiness fails safe.
- **Post-commit recovery gates — COMPLETE.** Network and critical controller health continue to gate host restoration after recovery commit.
- **Wake staggering — COMPLETE.** `delay_after_previous_seconds` is enforced without blocking the watchdog/event loop and is conservatively re-applied after reboot.
- **Dependency-cycle validation — COMPLETE.** Managed host recovery cycles are rejected before a configuration can become known-good.
- **NUT `HOSTSYNC` / `FINALDELAY` drift — COMPLETE.** Installer-generated `upsmon.conf` receives canonical project timing values.
- **Existing-NUT multi-primary FSD risk — COMPLETE.** Process-wide `upsmon -c fsd` is rejected when multiple primary/master monitor entries make ownership ambiguous.
- **Health-state writer race / incomplete service repair — COMPLETE.** Agent UPS health and external system-service health use separate state files; project-owned services are selected from canonical profile/network configuration.
- **Power loss during installer transaction — COMPLETE.** A durable install marker plus boot recovery restores the pre-install snapshot before automation starts.
- **Cockpit config management / privilege mismatch — COMPLETE.** Cockpit performs privileged management through the control CLI and transactional revision manager rather than directly editing protected runtime files.
- **CI supply-chain mutability — COMPLETE.** First-party Actions are pinned to immutable commit SHAs and dependency update monitoring is configured.
- **npm transitive dependency drift — COMPLETE.** Lockfile is committed and build workflows use `npm ci`.

## 4. Installer and interrupted-power readiness

The supported entry points remain:

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

Implemented installer properties include:

- supported platform/architecture detection;
- clean-OS dependency installation;
- prebuilt release artifacts with source-build fallback for development;
- local-server, remote-client and existing-NUT profiles;
- explicit UPS driver/port selection and NUT USB discovery;
- no silent guessing when discovery is missing or ambiguous;
- Synology compatibility opt-in;
- trusted-LAN default plus optional additive restricted nftables policy;
- no flushing/replacement of unrelated administrator firewall rules;
- IPv4/IPv6 NUT listeners tied to policy;
- guided `dialog` TUI with `whiptail` fallback;
- transactional application/config/service/NUT/Cockpit/firewall state;
- automatic required-service enablement;
- health/probation gate before known-good promotion;
- rollback of normal install/upgrade failures;
- durable boot recovery after sudden power loss during installation;
- idempotent reinstall acceptance.

Fresh installations intentionally contain empty live inventory:

```yaml
network_dependencies: []
hosts: []
```

Documentation examples are never silently copied into the active configuration.

## 5. Cockpit management readiness

Cockpit is now a management surface rather than a read-only dashboard.

Configuration changes follow this path:

```text
Cockpit editor
  -> privileged cockpit-ups-wolctl config-validate
  -> candidate revision
  -> transient systemd activation transaction
  -> immediate health check
  -> probation interval
  -> promote to last-known-good OR automatic rollback
```

The activation/probation transaction is independent of the browser session, so closing Cockpit does not terminate an in-flight validation. Startup reconciliation protects against controller power loss during candidate activation.

Sensitive reads and mutations require Cockpit superuser escalation; the UI does not gain direct write access to protected config/state files.

## 6. Automated acceptance status

Automated coverage includes:

- Go unit tests and `go vet`;
- canonical YAML and generated NUT integration tests;
- config candidate/probation/rollback/reboot reconciliation;
- active-config content/manifest mismatch recovery;
- corrupt/torn state fallback and valid-generation preservation;
- outage/recovery lifecycle simulation;
- two-sample AC cancellation debounce;
- bounded UPS communication loss;
- reboot-safe maximum-on-battery timing;
- power bounce during partial recovery and second-shutdown behavior;
- post-reboot AC-stability restart;
- recovery network timeout and critical-health gating;
- durable action ordering before SSH/FSD/WoL side effects;
- bounded direct shutdown retries and fail-safe exhaustion;
- wake-delay sequencing and reboot resume behavior;
- recovery dependency-cycle rejection;
- generated NUT primary/secondary + Synology privilege tests;
- canonical `HOSTSYNC` / `FINALDELAY` generation;
- existing-NUT multi-primary FSD rejection;
- installer discovery parsing and ambiguity rules;
- restricted firewall plan generation and no-firewall-flush assertion;
- interrupted-install boot rollback;
- installer option/self-check matrix including TUI and restricted mode;
- real Ubuntu 24.04 NUT `dummy-ups` + systemd + Cockpit install/probation/idempotency/rollback acceptance;
- amd64 native runtime smoke;
- arm64 QEMU runtime smoke;
- riscv64 QEMU runtime smoke;
- Cockpit strict TypeScript/build validation using locked npm dependencies;
- reproducible package/checksum verification.

## 7. Physical release gate

Software simulation cannot establish electrical behavior. Issue #10 remains open until retained evidence exists for:

```text
[ ] amd64 controller + supported real UPS
[ ] arm64 controller + supported real UPS
[ ] real Synology DSM configured as NUT secondary
```

Each UPS run must cover at least:

- controller and required network devices on battery-backed outputs;
- real OB detection;
- shutdown ordering and NUT-primary behavior;
- interrupted/repeated boot behavior where practical;
- utility restoration and stable-AC gate;
- battery recovery gate (default 80%);
- ordered WoL restoration;
- controller automatic boot when UPS output returns;
- retained logs/config/state evidence.

Use `scripts/test/hardware-preflight.sh` before the destructive test. Software/QEMU results must never be recorded as physical PASS.

## 8. Synology release gate

Software configuration is ready, but real DSM behavior must still be demonstrated for release. Acceptance must confirm:

- DSM connects to the generated NUT service;
- the Synology account remains monitor-only;
- DSM shuts itself down through the NUT-secondary path;
- the agent does not duplicate that shutdown through SSH;
- recovery/WoL behavior works for the selected NAS/DSM configuration;
- actual DSM behavior matches the documented compatibility profile for the tested DSM version.

## 9. Release governance

### License

Issue #12 remains intentionally open. Selecting the project license is a project-owner/legal decision and is not automated. Tagged publication is already guarded by a workflow check requiring a root `LICENSE`.

### Main-branch protection

Issue #39 tracks the repository-admin step to require pull requests and successful CI before changes reach `main`, and to block force-push/deletion. The connected GitHub integration does not have administration permission to apply that setting directly.

### Dependency/update controls

GitHub Actions are pinned to immutable commit SHAs. Dependabot monitors GitHub Actions, Go modules and Cockpit npm dependencies. Cockpit transitive dependencies are fixed by the committed lockfile and installed with `npm ci` in CI and packaging.

## 10. Known non-blocking v0.1 limitations

These remain outside the defined v0.1 safety baseline:

- no multi-UPS policy;
- no Proxmox-specific API adapter yet;
- no arbitrary shell shutdown command execution;
- no dependency WoL until dependency actions receive the same durable semantics as managed hosts;
- no advanced writable UPS command UI;
- no native Go NUT protocol client requirement;
- no Prometheus/notification/history subsystem;
- device-specific form-based Cockpit CRUD can be expanded; the full transactional YAML configuration workflow already exists.

## 11. Final release checklist

A public v0.1 tag is allowed only when all boxes below are satisfied:

```text
[x] architecture and safety model implemented
[x] armed orchestration implemented
[x] outage communication-loss / hysteresis / reboot timing safeguards implemented
[x] recovery network / health / wake-delay safeguards implemented
[x] bounded direct-host shutdown retry + FAILED_SAFE implemented
[x] automatic service startup / health / autofix implemented
[x] configuration LKG / rollback / startup recovery implemented
[x] boot interruption / power-bounce handling implemented
[x] interrupted installer rollback on next boot implemented
[x] NUT HOSTSYNC / FSD ownership safeguards implemented
[x] Synology software integration implemented
[x] transactional installer + TUI implemented
[x] UPS discovery / explicit selection implemented
[x] optional restricted NUT networking implemented
[x] Cockpit transactional configuration management implemented
[x] immutable Actions + dependency monitoring implemented
[x] npm lockfile / npm ci reproducibility implemented
[x] multi-arch build/package/checksum pipeline implemented
[x] automated software acceptance green
[ ] physical amd64 UPS acceptance retained
[ ] physical arm64 UPS acceptance retained
[ ] real Synology DSM acceptance retained
[x] root LICENSE selected and added (`AGPL-3.0-or-later`)
[ ] main branch protection configured (issue #39)
```

**Current verdict:** software candidate is ready for physical acceptance. Public v0.1 release remains blocked only by physical validation and repository-owner governance decisions.
