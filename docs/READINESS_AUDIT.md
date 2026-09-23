# cockpit-ups-wol — Readiness Audit

**Audit date:** 2026-09-23  
**Status:** v0.1 post-deep-review release-gate reconciliation  
**Scope:** architecture, runtime safety, installation, interrupted-power recovery, NUT/Synology, Cockpit management, health/autofix, testing, packaging, licensing and repository governance

## 1. Executive verdict

The **v0.1 software baseline is implemented and automated acceptance is green on the accepted feature set**. Previous runtime/crash-consistency findings were fixed with regression coverage.

No unresolved software-runtime P0/P1 blocker from the deep review remains. The project is at the **physical validation / release-governance** stage.

A public v0.1 remains blocked by:

1. **real hardware acceptance — issue #10**: amd64 + real supported UPS, arm64 + real supported UPS, and real Synology DSM NUT-secondary acceptance with retained evidence;
2. **main-branch protection — issue #39**: repository-admin configuration requiring pull requests and green CI for `main`, with force-push/deletion blocked.

The project license is already selected: **GNU AGPL-3.0-or-later** with a root `LICENSE`.

## 2. Current readiness by area

| Area | Status | Evidence / note |
|---|---|---|
| Architecture / state model | READY | canonical power, health and operating-mode models implemented |
| Outage safety policy | READY | `recovery.enabled`; online-cancellation debounce; bounded communication loss; durable max-on-battery progress |
| Recovery safety policy | READY | stable utility, UPS recharge, network and health gates; timeout and post-commit re-checks |
| NUT ownership / FSD | READY | primary/master validation; multi-primary fail closed; generated integration tests |
| NUT synchronization | READY | canonical `HOSTSYNC` and `FINALDELAY` rendered from project config |
| Persistent state | READY | checksum, fsync, atomic rotation and valid previous-generation preservation |
| Interrupted boot / power bounce | READY | boot reconciliation; renewed-outage epochs; re-shutdown of already-restored hosts when required |
| Direct host shutdown | READY | accepted v0.1 path is fixed-argv SSH plus NUT/none semantics; bounded retries and `FAILED_SAFE` exhaustion |
| Managed-host WoL | READY | durable attempts, dependency-aware host ordering, inter-host delay and reboot reconciliation |
| Configuration validation | READY | strict schema/semantic validation, dependency-cycle/timing checks and armed capability rejection |
| Configuration transactions | READY | candidate/probation/LKG/rollback plus interrupted-activation startup reconciliation |
| Health / autofix | READY | watchdog, bounded repair circuit and separate durable system/agent health state |
| Cockpit management | READY | status/plan/logs and privileged `cockpit-ups-wolctl` YAML validate/apply/rollback workflow |
| Agent IPC | READY FOR CURRENT SCOPE | bounded Unix-socket request/response; current implemented agent method is `GetHealth`; broader management remains CLI/revision-manager based |
| Installer | READY | default/silent/TUI share one transactional backend |
| Interrupted installation | READY | durable pending marker and boot-time rollback/recovery guard |
| systemd environment validation | READY | real install rejects environments where systemd is not PID 1 before transaction state is created |
| UPS discovery | READY | NUT scanner parsing, explicit driver/port and ambiguity failure |
| NUT network policy | READY | trusted-LAN default plus additive restricted nftables mode |
| Service autostart | READY | systemd enable/start validation and health probation |
| Synology software integration | READY | monitor-only NUT-secondary account and ownership tests |
| amd64 software runtime | READY | native runtime + Ubuntu systemd/NUT E2E |
| arm64 software runtime | READY | build + QEMU runtime smoke |
| riscv64 software runtime | READY | build + QEMU runtime smoke |
| Build supply chain | READY | first-party Actions pinned to immutable SHAs; Dependabot enabled |
| Cockpit dependency reproducibility | READY | committed lockfile, `npm ci`, React 19.3.0 and aligned PatternFly 6.6.1 |
| Packaging/checksums | READY | reproducible multi-arch archives + Debian packages + SHA256SUMS |
| Project license | READY | AGPL-3.0-or-later; root `LICENSE` present |
| Real UPS amd64 | BLOCKED / NOT RUN | physical acceptance required |
| Real UPS arm64 | BLOCKED / NOT RUN | physical acceptance required |
| Real Synology DSM | BLOCKED / NOT RUN | physical DSM acceptance required |
| `main` protection | BLOCKED / ADMIN | issue #39; repository administration required |

## 3. Explicit capability boundary

The canonical schema carries some values reserved for later work. Their presence in the schema does not make them accepted armed-v0.1 capabilities.

The following intentionally fail closed in armed v0.1:

```text
shutdown.method: command
ARP-only host verification
dependency Wake-on-LAN
```

The current accepted direct-shutdown set is:

```text
ssh
nut
none
```

Managed-host WoL is implemented; dependency WoL is not yet accepted because dependency actions do not yet have the same durable action-state semantics.

## 4. Deep-review findings — closure

All software findings from the 2026-09-20 deep review are closed, including:

- hard `recovery.enabled` gating;
- new-outage creation during partial recovery;
- valid previous-state preservation across torn/corrupt writes;
- startup restoration of LKG after interrupted config activation;
- bounded direct-shutdown retries with `FAILED_SAFE` exhaustion;
- two-sample online cancellation debounce;
- bounded UPS communication-loss grace;
- reboot-safe maximum-on-battery accounting;
- recovery network timeout;
- post-commit network/health gates;
- durable/reboot-safe wake staggering;
- dependency-cycle validation;
- canonical NUT `HOSTSYNC`/`FINALDELAY` rendering;
- fail-closed existing-NUT multi-primary ownership;
- separate health-state writers and bounded project-service repair;
- interrupted installer boot rollback;
- Cockpit privileged transactional config management;
- immutable Action pinning and dependency monitoring;
- locked npm dependency graph with `npm ci`.

## 5. Installer and interrupted-power readiness

Supported source-tree entry points:

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

Implemented properties include clean-OS dependency installation, supported platform/architecture checks, local/remote/existing NUT profiles, UPS discovery/explicit selection, Synology opt-in, trusted/restricted networking, one transactional backend, service enablement, health probation, known-good promotion, normal rollback, durable sudden-power-loss recovery, idempotent reinstall and early rejection when systemd is not the active PID-1 system manager.

Fresh installations intentionally contain:

```yaml
network_dependencies: []
hosts: []
```

and start in `dry-run`.

## 6. Cockpit and control boundary

Cockpit is a management surface rather than part of the safety-critical engine.

Configuration mutation follows:

```text
Cockpit editor
  -> privileged cockpit-ups-wolctl config-validate
  -> candidate revision
  -> transient systemd activation transaction
  -> immediate health check
  -> probation
  -> promote last-known-good OR rollback
```

The current agent Unix socket is deliberately smaller than older design drafts implied. It implements a bounded request/response transport and the current runtime handler exposes `GetHealth`. Configuration, plan and log operations are provided through the local CLI/revision manager and Cockpit privilege boundary rather than pretending every operation is an agent IPC method.

## 7. Automated acceptance status

Automated coverage includes Go unit/vet tests, canonical configuration and NUT generation tests, config transaction reboot/rollback tests, torn/corrupt state fallback, outage/recovery fault simulation, communication loss, AC debounce, max-on-battery reboot continuity, partial-recovery power bounce, network/health recovery gates, durable SSH/FSD/WoL ordering, bounded shutdown retries, wake delay, dependency-cycle rejection, Synology privilege checks, `HOSTSYNC`/`FINALDELAY`, multi-primary rejection, installer discovery/network/rollback tests, Ubuntu 24.04 NUT `dummy-ups` + systemd + Cockpit E2E, amd64 native runtime smoke, arm64/riscv64 QEMU runtime smoke, locked Cockpit typecheck/build and package/checksum validation.

Unsupported future capabilities are covered by negative/fail-closed validation rather than positive armed-execution tests.

## 8. Physical release gate

Issue #10 remains open until retained evidence exists for:

```text
[ ] amd64 controller + supported real UPS
[ ] arm64 controller + supported real UPS
[ ] real Synology DSM configured as NUT secondary
```

Each real UPS run must demonstrate controller/network backed-power topology, real OB detection, shutdown ordering/FSD behavior, interruption/reboot safety, stable-AC recovery, default 80% recharge gating where available, managed-host recovery, controller automatic boot after output return and retained logs/config/state evidence.

Use `scripts/test/hardware-preflight.sh` before destructive acceptance. QEMU, `dummy-ups` and `ami3go/USB-UPS-Simulator` are valuable test tools but must never be recorded as a real-UPS physical PASS.

## 9. Release governance

### License

Complete. The project is `AGPL-3.0-or-later`, the root license is present, and release artifacts include the license. Future copied/adapted upstream source still requires exact per-component compatibility/attribution review in `THIRD_PARTY_NOTICES.md`.

### Main-branch protection

Issue #39 remains open. `main` should require pull requests and successful project CI, block force-push/deletion, and keep any emergency/admin bypass explicit and auditable. This is an admin setting outside the current connector permissions.

### Dependency/update controls

GitHub Actions are pinned to immutable commit SHAs. Dependabot monitors GitHub Actions, Go and Cockpit npm dependencies. The Cockpit dependency graph is locked and installed with `npm ci`.

## 10. Known non-blocking v0.1 limitations

- no multi-UPS policy;
- no Proxmox-specific API adapter;
- no armed arbitrary/allowlisted command shutdown adapter yet;
- no armed ARP-only verification;
- no dependency WoL until durable dependency action state exists;
- no advanced writable UPS command UI;
- no native Go NUT protocol requirement;
- no Prometheus/notification/history subsystem;
- form-based Cockpit CRUD can expand beyond the current transactional YAML workflow.

## 11. Final release checklist

```text
[x] architecture and safety model implemented
[x] armed accepted-feature orchestration implemented
[x] outage communication-loss / hysteresis / reboot timing safeguards implemented
[x] recovery network / health / wake-delay safeguards implemented
[x] bounded direct-host shutdown retry + FAILED_SAFE implemented
[x] automatic service startup / health / autofix implemented
[x] configuration LKG / rollback / startup recovery implemented
[x] interrupted installer rollback on next boot implemented
[x] NUT HOSTSYNC / FSD ownership safeguards implemented
[x] Synology software integration implemented
[x] transactional installer + TUI implemented
[x] UPS discovery / explicit selection implemented
[x] optional restricted NUT networking implemented
[x] Cockpit transactional configuration management implemented
[x] immutable Actions + dependency monitoring implemented
[x] npm lockfile / npm ci reproducibility implemented
[x] multi-arch archive + Debian package + checksum pipeline implemented
[x] AGPL-3.0-or-later root license present
[x] automated software acceptance green
[ ] physical amd64 UPS acceptance retained
[ ] physical arm64 UPS acceptance retained
[ ] real Synology DSM acceptance retained
[ ] main branch protection configured (issue #39)
```

**Current verdict:** the software candidate is ready for physical acceptance. Public v0.1 remains blocked only by real-hardware evidence and repository-owner branch governance.
