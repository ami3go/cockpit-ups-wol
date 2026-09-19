# v0.1 Implementation Decisions

**Status:** Frozen baseline for initial implementation

## 1. Agent

Language: **Go**.

Reasons:

- small self-contained release binaries
- straightforward amd64/arm64/riscv64 cross-builds
- low runtime dependency burden on small SBCs
- strong standard-library support for system programming, Unix sockets, JSON, networking and concurrency
- suitable for systemd watchdog/daemon behavior
- useful permissively licensed upstream references already identified

The target system SHALL NOT require a Go compiler at runtime.

## 2. WoL helper

`wolctl` is implemented in Go.

The low-level packet implementation may adapt permissively licensed patterns/code after exact attribution is added to `THIRD_PARTY_NOTICES.md`.

## 3. Cockpit frontend

Stack:

```text
TypeScript
React
PatternFly
Cockpit JavaScript API
```

Use `cockpit-project/starter-kit` as the build/UI foundation where licensing obligations are satisfied.

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

No build toolchain is required on the target for normal release installation.

## 5. Configuration

Format: YAML.

Validation source of truth: JSON Schema (`schemas/config.schema.json`) plus semantic/cross-reference validators implemented in Go.

The Go service may use a small maintained YAML parser dependency. Dependencies SHALL be pinned by `go.mod`/`go.sum`.

## 6. Persistent state

Format: JSON with deterministic canonical serialization for checksum calculation.

Rationale:

- simple recovery/debugging
- easy schema validation and fixture generation
- no database dependency
- atomic file generations are adequate for the expected transaction volume

No SQLite/database is required for v0.1 safety state.

## 7. Local IPC

Unix-domain stream socket with newline-delimited JSON as defined in `docs/IPC.md`.

No TCP management API in v0.1.

## 8. Service manager

systemd is the v0.1 runtime service model.

The project targets systemd-based Linux distributions for the full appliance.

NUT unit naming is distro-aware and SHALL not be hard-coded globally.

## 9. Logging

Runtime logs use journald.

The installer additionally writes a persistent installer log under:

```text
/var/log/cockpit-ups-wol/
```

No separate runtime logging database is required for v0.1.

## 10. NUT integration

v0.1 primarily uses installed NUT tools/services rather than embedding a native NUT protocol implementation.

The Go agent may execute safe argv-based commands such as `upsc` through a controlled adapter without shell string concatenation.

A native Go NUT client remains a later optimization after protocol/compatibility behavior is proven.

## 11. Host adapters

v0.1 adapters:

```text
nut
ssh
command
none
```

`command` uses allowlisted command IDs, not arbitrary shell strings from UI/config.

Proxmox native API integration is planned after the base v0.1 lifecycle unless implementation effort permits it without delaying the safety core.

## 12. Build layout

Planned Go module structure:

```text
agent/
  cmd/cockpit-ups-wol-agent/
  cmd/cockpit-ups-wolctl/
  internal/
    config/
    health/
    host/
    ipc/
    nut/
    policy/
    state/
    system/
    wol/
```

A single Go module for agent + CLI is preferred initially to keep shared types/protocols synchronized.

## 13. Tests

Go:

```text
unit tests
race detector where supported
integration tests with fake NUT/host adapters
fault-injection state-store tests
```

Shell installer:

```text
shellcheck
bats or equivalent integration harness where useful
clean-OS VM/container tests
```

Frontend:

```text
TypeScript typecheck
lint
component/unit tests
build validation
```

System acceptance testing uses simulation first, then real UPS hardware.

## 14. Release artifacts

Required per release:

```text
linux-amd64 binaries
linux-arm64 binaries
linux-riscv64 binaries
SHA256SUMS
Cockpit bundle
installer
source archive/release notes
```

Target machines consume prebuilt artifacts.

## 15. Dependency policy

Prefer the Go standard library and small focused dependencies.

Every copied/adapted upstream source file/function requires:

- license compatibility review
- exact source project/path/commit recorded
- attribution in `THIRD_PARTY_NOTICES.md`
- clear indication of local modifications

## 16. Deferred choices

Not required for v0.1:

```text
native Go NUT protocol client
D-Bus agent API
embedded database
containerized primary deployment
Kubernetes
multi-controller HA
cloud service
```

These may be reconsidered after the core safety lifecycle is hardware-tested.
