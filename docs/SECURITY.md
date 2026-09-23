# Security Model

**Status:** Normative v0.1 security baseline

## 1. Scope

`cockpit-ups-wol` can participate in system shutdown, Wake-on-LAN recovery, NUT configuration and NUT-primary FSD. Treat it as management-plane software.

Primary deployment assumption:

```text
trusted home/lab management LAN
not exposed directly to the public Internet
```

## 2. Security principles

- least privilege;
- no arbitrary browser-supplied shell commands;
- no project TCP management API;
- NUT network clients use monitor-only credentials;
- destructive UPS behavior is separated from ordinary monitoring;
- secrets are protected/redacted;
- rollback does not expose prior secret values;
- autofix is bounded to project-owned safe repair;
- unsupported action types fail closed.

## 3. Trust boundaries

```text
Browser
  ↓ authenticated Cockpit session
Cockpit bridge / local CLI
  ↓ privilege escalation only for explicit admin operations
cockpit-ups-wolctl / project files / agent health socket
  ↓ controlled argv-based adapters
NUT / SSH / WoL / systemd
  ↓
Managed infrastructure
```

The browser does not receive a project-provided raw privileged shell or direct write access to durable power-state files.

## 4. Current local control model

### Agent Unix socket

The agent listens on a local Unix-domain socket for the current minimal IPC surface. The implemented runtime method is `GetHealth` and the server handles one bounded request/response per connection.

The current v0.1 implementation does **not** claim a completed generic `SO_PEERCRED` authorization framework for mutating IPC methods, because mutating IPC methods are not currently exposed.

If future mutating IPC is added, explicit peer-credential authorization and regression tests are required before it becomes accepted functionality.

### Privileged configuration control

Configuration mutation is performed through `cockpit-ups-wolctl` and the transactional revision manager. Commands such as config apply/activate/rollback are root-gated; Cockpit uses its authenticated superuser path when privilege is required.

This is the actual v0.1 privilege boundary and must not be confused with historical draft-only IPC methods.

## 5. NUT credentials and FSD authority

Synology/Linux network clients receive monitoring/secondary-only credentials. They do not receive:

```text
SET
FSD
instcmds = ALL
```

Automatic FSD is available only through the validated local NUT-primary ownership path. Remote-client mode does not inherit FSD/output authority merely because a remote `upsd` is reachable.

Existing-NUT mode rejects automatic process-wide FSD when multiple primary/master monitor entries make ownership ambiguous.

## 6. Synology compatibility credential

DSM compatibility may require the conventional `monuser` / `secret` account. It is created only when Synology compatibility is explicitly enabled and remains monitor-only.

Because the credential is a compatibility convention rather than a strong secret, protection depends on LAN placement, no Internet exposure of TCP 3493, minimum privileges and optional restricted NUT firewall policy.

## 7. SSH security

Shutdown SSH keys live under a protected secrets directory, for example:

```text
/etc/cockpit-ups-wol/secrets/
```

Recommended permissions:

```text
secrets directory 0700
private keys       0600
```

Keys should be dedicated to power management. Remote accounts should have narrowly scoped shutdown privileges where practical.

SSH host keys are verified. Changed host keys are a security error, not automatically accepted.

The shutdown adapter uses fixed argv construction rather than concatenating browser/config text into a shell command.

## 8. Command execution boundary

Schema value `shutdown.method: command` is reserved for future work but **is not an accepted armed-v0.1 action**.

Armed validation fails closed because no durable allowlisted command registry has yet passed acceptance. The project therefore makes no claim that arbitrary/custom command shutdown is currently available.

The UI never submits arbitrary shell source for execution.

## 9. Host verification boundary

TCP/ping verification paths are accepted where implemented. ARP-only verification is not accepted in armed v0.1 and fails closed rather than being treated as trustworthy shutdown/recovery evidence.

## 10. Dependency actions

Managed-host Wake-on-LAN is implemented with durable action state. Dependency Wake-on-LAN is not accepted in armed v0.1 because dependency actions do not yet have equivalent durable request/retry/reconciliation semantics.

## 11. Dangerous UPS operations

Operations capable of dropping load or changing UPS shutdown state, such as writable variables or vendor instant commands, are not used automatically by health/autofix.

The normal project shutdown path requests FSD through validated primary `upsmon` and leaves final UPS output shutdown/return to the late NUT/system shutdown integration.

Future manual dangerous UPS controls require explicit authorization, confirmation, current-state validation and audit logging before they may be exposed.

## 12. NUT network exposure

Default: `trusted-lan`, relying on the administrator's upstream LAN boundary.

Optional: `restricted`, using an additive project-owned nftables table for NUT TCP/3493.

Requirements:

- no deliberate public Internet exposure;
- every enabled address family is covered;
- no `flush ruleset` or wholesale replacement of unknown firewall state;
- rollback removes only project-owned firewall state.

## 13. Cockpit

Cockpit remains the primary user-facing authentication/authorization layer. The project uses Cockpit's superuser mechanism for privileged CLI operations rather than adding a separate project password database.

Closing a browser session does not terminate an in-flight configuration probation transaction after `systemd-run` has started it.

## 14. Secrets and revision history

Preferred model:

```text
configuration references protected secret file
secret value remains outside normal UI/log output
```

Never log private keys, passwords, tokens, full authorization headers or raw secret file content.

Revision history and failed candidates must retain safe ownership/modes; Cockpit diagnostics/revision summaries redact secret values.

## 15. Persistent-state integrity

Power state uses SHA-256 integrity validation plus valid previous-generation fallback. This detects accidental corruption/torn writes; it is not an anti-tamper signature.

Root filesystem compromise is outside this checksum's protection boundary.

## 16. Supply-chain/update integrity

- project license: AGPL-3.0-or-later;
- first-party GitHub Actions are pinned to immutable commit SHAs;
- Dependabot monitors Action, Go and npm updates;
- Cockpit dependencies are locked and installed with `npm ci`;
- release artifacts carry SHA256 checksums;
- copied/adapted upstream source requires exact attribution/license review in `THIRD_PARTY_NOTICES.md`.

## 17. Autofix boundary

Safe automatic repair may restart/re-enable project services, recreate/repair project-owned paths and restore known-good project configuration.

It does not automatically change system networking, replace unknown firewalls, regenerate credentials, accept changed SSH host keys, issue destructive UPS commands, or wake/shut down hosts merely to make health green.

Repeated repair failure reaches the circuit breaker/`FAILED_SAFE` state.

## 18. Threats considered

v0.1 explicitly considers:

```text
unauthorized LAN NUT access
malicious/accidental configuration change
browser/config injection into process execution
credential leakage in logs/revision output
compromised low-privilege NUT client
changed SSH host key
malformed/oversized local IPC request
unsafe rollback/repair
network/firewall misconfiguration
ambiguous NUT primary ownership
```

## 19. Out of scope

v0.1 does not claim protection against root compromise, physical compromise of equipment, malicious UPS firmware, or a fully compromised Cockpit/system administrator account.

## 20. Security acceptance

Required automated acceptance includes:

```text
root gate for privileged config mutation
malformed/oversized IPC rejected
unknown IPC methods fail closed
no arbitrary shell execution path
changed SSH host key blocks action
Synology account cannot SET/FSD/instcmd
restricted mode covers enabled address families
health autofix cannot issue destructive UPS commands
remote NUT profile cannot issue FSD by default
multi-primary existing-NUT FSD is rejected
state/config/secrets ownership and modes are appropriate
```
