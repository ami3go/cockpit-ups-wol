# Security Model

**Status:** Normative v0.1 security baseline

## 1. Scope

`cockpit-ups-wol` can shut down systems, wake devices, modify NUT configuration and potentially participate in UPS shutdown. Treat it as management-plane software.

Primary deployment assumption:

```text
trusted home/lab management LAN
not exposed directly to the public Internet
```

## 2. Security principles

- least privilege
- no arbitrary browser-supplied shell commands
- no unauthenticated TCP management API
- NUT clients use monitor-only credentials
- destructive UPS operations are separated from ordinary monitoring
- secrets are stored in protected files and redacted from logs
- configuration rollback never exposes previous secret values in diagnostics
- autofix may repair project-owned state but does not rewrite unknown system/network policy destructively

## 3. Trust boundaries

```text
Browser
  ↓ authenticated Cockpit session
Cockpit bridge / local CLI
  ↓ local Unix socket + peer credentials
Agent
  ↓ controlled adapters
NUT / SSH / WoL / systemd
  ↓
Managed infrastructure
```

The browser never receives direct filesystem or raw privileged shell access from this project.

## 4. Agent IPC

Management IPC uses the Unix-domain socket defined in `docs/IPC.md`.

No TCP management API is enabled in v0.1.

Privileged operations require a privileged local peer; Cockpit uses its superuser path for those operations.

## 5. NUT credentials

Network clients such as Synology receive monitor-only credentials.

They SHALL NOT receive:

```text
SET
FSD
instcmds = ALL
```

Administrative UPS permissions, if required, are separate and accessible only through the local privileged path.

## 6. Synology compatibility credential

DSM compatibility may require the known `monuser` / `secret` convention.

This credential is created only when Synology compatibility is enabled.

Because it is not a strong secret, security relies on:

- trusted/restricted LAN placement
- no Internet exposure of TCP 3493
- monitor-only privileges
- optional firewall restriction for environments requiring tighter access

## 7. SSH secrets

SSH private keys used for managed shutdown live under:

```text
/etc/cockpit-ups-wol/secrets/
```

Recommended permissions:

```text
secrets directory 0700
private keys       0600
```

Keys should be dedicated to power management rather than reused administrator keys.

Remote accounts SHOULD have narrowly scoped privileges, e.g. permission to execute only the required shutdown command where practical.

## 8. Host key verification

SSH adapters SHALL verify host keys.

They SHALL NOT silently use insecure host-key bypass options for normal operation.

Installer/TUI may provide a deliberate trust-on-first-use enrollment step that records the observed fingerprint for administrator review.

A changed host key is a security error, not an automatic acceptance event.

## 9. Configuration secrets

Ordinary configuration revision manifests/diffs SHALL redact secret fields.

Preferred model:

```text
config contains path/reference
secret exists in protected file
```

rather than embedding credentials directly in YAML.

Compatibility values that must exist in config are treated as sensitive in UI/log rendering even if widely known.

## 10. Secret logging

Never log:

```text
private key content
passwords
API tokens
full Authorization headers
raw secret file content
```

Error messages should identify the secret reference/path without echoing its value.

## 11. Configuration revision store

Known-good configuration revisions are protected from ordinary users.

The revision store SHALL preserve safe ownership/mode and SHALL NOT make secret-bearing snapshots world-readable.

Failed candidates may be retained for diagnostics, but secret values remain protected/redacted in UI output.

## 12. Command execution

The project SHALL use argv-based process execution without shell concatenation where practical.

`shutdown.method: command` refers to an allowlisted command ID defined by project/admin configuration.

The UI SHALL NOT submit arbitrary shell source for execution.

## 13. Dangerous UPS actions

Operations such as:

```text
FSD
load.off
shutdown.return
writable UPS variables
battery tests with operational impact
```

require:

- privileged authorization
- explicit confirmation
- current-state safety validation
- journald audit event

Health autofix SHALL NOT invoke destructive UPS instant commands.

## 14. FSD authority

Only the validated local NUT primary path may request automatic FSD in local-server mode.

Remote-client mode does not inherit FSD/output-control authority merely because the remote server is reachable.

See `docs/NUT_SHUTDOWN_MODEL.md`.

## 15. NUT network exposure

Default `trusted-lan` mode assumes upstream router/firewall protection.

NUT TCP 3493 must not be intentionally Internet-exposed.

Optional `restricted` mode applies explicit host/subnet rules.

If IPv6 is enabled, protection SHALL cover IPv6 as well as IPv4.

Unknown existing firewall rules are preserved rather than replaced wholesale.

## 16. Cockpit

Cockpit authentication/authorization remains the primary user-facing access control layer.

The project extension SHALL use Cockpit APIs and superuser mechanisms rather than implementing a separate password database.

## 17. Service identity and sandboxing

The v0.1 agent intentionally runs as root because the controller must request local NUT FSD, perform network/Wake-on-LAN operations, and maintain protected transaction state. A non-root service identity is therefore not claimed for this release.

Privilege is constrained with systemd sandboxing instead: `NoNewPrivileges`, strict filesystem protection, explicit writable project paths, restricted address families, kernel/control-group protections, namespace/SUID restrictions, a system-service syscall allowlist, and a bounded capability set. SSH keys and `known_hosts` are pinned under `/etc/cockpit-ups-wol/` so `ProtectHome=yes` cannot hide them.

Any future split into an unprivileged policy process plus a narrow privileged helper must preserve the same fail-closed behavior and durable transaction semantics.

## 18. State integrity

Persistent transaction state includes SHA-256 integrity validation and previous-generation fallback.

The checksum detects accidental corruption/torn writes; it is not an anti-tamper signature.

An attacker with root filesystem write access is outside the protection boundary of this checksum.

## 19. Update/release integrity

Installer verifies release artifact SHA-256 checksums before installation.

Future signing/provenance verification may strengthen this, but checksum failure already aborts replacement.

## 20. Supply-chain reuse

Before copying upstream source:

- confirm license
- record repository, path and commit
- retain required copyright/license notice
- document modifications in `THIRD_PARTY_NOTICES.md`

No-license source is reference-only.

## 21. Autofix boundary

Safe automatic repair may:

- restart/re-enable project-managed services
- restore project-owned permissions/directories
- restore last-known-good project config

It SHALL NOT automatically:

- change system networking
- replace unknown firewall rules
- regenerate credentials
- accept changed SSH host keys
- send destructive UPS instant commands
- wake/shut down hosts only to make health green

## 22. FAILED_SAFE

When trustworthy control cannot be restored, `FAILED_SAFE` blocks destructive automatic actions while retaining monitoring where possible.

Clearing `FAILED_SAFE` requires correction/reconciliation; an acknowledgement alone does not erase the underlying condition.

## 23. Threats considered

v0.1 explicitly considers:

```text
unauthorized LAN NUT access
malicious/accidental config change
browser injection into command execution
credential leakage in logs
compromised low-privilege NUT client
stale/changing SSH host key
malformed IPC request
config rollback exposing secrets
network/firewall misconfiguration
unsafe automatic repair
```

## 24. Out of scope

v0.1 does not claim protection against:

- root compromise of the controller
- physical compromise of UPS/controller/network equipment
- malicious UPS firmware
- a fully compromised Cockpit/system administrator account

## 25. Security acceptance tests

Required:

```text
unprivileged peer cannot call privileged IPC methods
malformed/oversized IPC requests are rejected
shell metacharacters in config cannot become arbitrary execution
secrets absent from logs/error JSON/revision summaries
SSH changed host key blocks action
Synology account cannot SET/FSD
restricted mode covers every enabled address family
health autofix cannot issue destructive UPS command
remote NUT profile cannot issue FSD by default
state/config files have expected ownership/modes
```
