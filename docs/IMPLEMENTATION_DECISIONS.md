# v0.1 Implementation Decisions

**Status:** Implemented/frozen v0.1 software baseline  
**Release state:** software acceptance green; physical UPS/DSM acceptance and root license remain open

## 1. Agent

Language: **Go**.

Reasons:

- small self-contained release binaries
- straightforward amd64/arm64/riscv64 cross-builds
- low runtime dependency burden on small SBCs
- strong standard-library support for system programming, Unix sockets, JSON, networking and concurrency
- suitable for systemd watchdog/daemon behavior

Normal target systems do not require a Go compiler; release bundles contain prebuilt binaries. Source-tree development installation may build from source when prebuilt binaries are absent.

## 2. WoL helper

`wolctl` is implemented in Go.

The current project implementation is independently maintained in this repository. Any future copied/adapted upstream code still requires exact attribution in `THIRD_PARTY_NOTICES.md` before inclusion.

## 3. Cockpit frontend

Stack:

```text
TypeScript
React
PatternFly
Cockpit JavaScript API
```

The v0.1 frontend is implemented as a prebuilt Cockpit bundle. Node/npm are build-time dependencies only; production SBC installation consumes the built assets.

Cockpit remains management-only and is not required for automatic outage/recovery processing.

## 4. Installer

Implementation:

```text
Bash entry point
modular distro-specific shell libraries
```

One entry point:

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

All three modes use one transactional backend. The TUI uses `dialog` with `whiptail` fallback where practical.

Normal release installation requires no Go/Node build toolchain on the target.

## 5. Configuration

Format: YAML.

Validation source of truth: JSON Schema (`schemas/config.schema.json`) plus semantic/cross-reference validators in Go.

The maintained YAML parser is pinned through `go.mod`/`go.sum`.

Fresh live installations deliberately start with empty `hosts` and `network_dependencies`; example targets are documentation/examples only.

## 6. Persistent state

Format: JSON with deterministic canonical serialization for checksum calculation.

Implemented model:

- `current` and `previous` durable generations
- sequence number and checksum verification
- temp write + fsync + atomic rename + parent-directory fsync
- outage/action identity
- per-host shutdown/recovery state
- fail-safe fallback when the newest generation is corrupt

No SQLite/database is required for v0.1 safety state.

## 7. Local IPC

Unix-domain stream socket with newline-delimited JSON as defined in `docs/IPC.md`.

v0.1 has no TCP management API. Cockpit uses the local CLI/IPC boundary rather than editing agent power-state files directly.

## 8. Service manager

systemd is the v0.1 runtime service model.

The full appliance targets systemd-based Linux distributions. Runtime integration includes service autostart, watchdog notification, the health timer, and optional project firewall service.

NUT unit naming remains distro-aware.

## 9. Logging

Runtime logs use journald.

The installer additionally writes:

```text
/var/log/cockpit-ups-wol/install.log
```

No separate runtime logging database is required for v0.1.

## 10. NUT integration

v0.1 uses installed NUT tools/services rather than embedding a native NUT protocol implementation.

The Go agent invokes safe argv-based tools such as `upsc` and the validated primary FSD path without shell string concatenation.

Communication failure is `UNKNOWN`, never inferred as `OL` or full battery.

A native Go NUT client remains a later optimization.

## 11. Host adapters

Implemented/accepted v0.1 shutdown methods:

```text
nut   — host participates in the NUT/FSD secondary path
ssh   — fixed argv / constrained remote shutdown
none  — observe/manage state without controller-issued shutdown
```

`command` exists in the schema/design space but **fails closed for armed v0.1** because no durable allowlisted command registry has yet passed acceptance. Arbitrary shell strings are not executed.

Status checking implemented for safe TCP/ping paths with consecutive verification. Armed ARP-only verification remains unsupported.

Proxmox native API integration is deferred to v0.2.

## 12. Build layout

The repository uses one Go module under `agent/`, with executable entry points under `agent/cmd/` and shared safety packages under `agent/internal/`.

Current major internal areas include configuration, control/runtime, health, host adapters, IPC, NUT, orchestration/policy, reporting, durable state, system integration and WoL.

Keeping agent/CLI tools in one module synchronizes shared state/config/protocol types.

## 13. Tests

Current automated acceptance includes:

### Go / runtime

```text
unit tests
go vet
canonical YAML integration
NUT primary/secondary + Synology generated-config integration
fault/reboot/power-state tests
amd64/arm64/riscv64 builds
amd64 native runtime smoke
arm64/riscv64 QEMU runtime smoke
```

### Installer/system

```text
Bash syntax
installer backend unit tests
UPS discovery/config rendering tests
restricted-firewall plan tests
option/self-check matrix
Ubuntu 24.04 + real systemd + NUT dummy-ups + Cockpit E2E
idempotent reinstall
intentionally broken upgrade rollback
```

### Frontend

```text
strict TypeScript typecheck
production bundle build
bundle installation/rollback through installer E2E
```

Physical real-UPS and DSM validation is explicitly separate from CI simulation.

## 14. Release artifacts

The packaging pipeline produces:

```text
linux-amd64 appliance archive
linux-arm64 appliance archive
linux-riscv64 appliance archive
SHA256SUMS
prebuilt Cockpit bundle
installer/systemd/config/docs payload
```

Archives are normalized for reproducibility. Tagged publication remains blocked until a root project `LICENSE` exists.

## 15. NUT network security

Default: `trusted-lan`.

Optional `restricted` mode uses a dedicated project-owned nftables table for NUT TCP/3493. It does not flush/replace unrelated firewall state. IPv6 NUT listening is protected when enabled.

## 16. Dependency policy

Prefer the Go standard library and small focused dependencies.

Every copied/adapted upstream source file/function requires:

- license compatibility review
- exact source project/path/commit recorded
- attribution in `THIRD_PARTY_NOTICES.md`
- clear indication of local modifications

The project license decision remains a release blocker.

## 17. Deferred choices

Not required for v0.1:

```text
native Go NUT protocol client
D-Bus agent API
embedded database
arbitrary command shutdown adapter
dependency WoL without durable dependency-action state
multi-UPS policy
Proxmox API adapter
containerized primary deployment
Kubernetes
multi-controller HA
cloud service
```

These may be reconsidered after the core safety lifecycle is physically hardware-tested.
