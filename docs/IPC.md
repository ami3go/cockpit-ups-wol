# Local Agent IPC Contract

**Status:** Normative v0.1 control interface

## 1. Decision

Cockpit, CLI/TUI and local helpers SHALL communicate with `cockpit-ups-wol-agent` through a Unix-domain socket.

Default path:

```text
/run/cockpit-ups-wol/agent.sock
```

The UI SHALL NOT directly modify runtime state or active configuration files.

## 2. Why Unix-domain socket

For v0.1 it provides:

- no TCP listening port
- Linux peer credentials (`SO_PEERCRED`)
- simple Go implementation
- easy CLI integration
- clean separation between Cockpit and the power engine
- no dependency on D-Bus schema/tooling for the first release

A D-Bus adapter may be added later without changing the internal service API.

## 3. Transport

Protocol: newline-delimited JSON request/response over `AF_UNIX` stream socket.

Each connection may send multiple requests sequentially.

Request example:

```json
{"id":"1","method":"GetStatus","params":{}}
```

Response:

```json
{"id":"1","ok":true,"result":{"power_state":"NORMAL","health":"HEALTHY"}}
```

Error:

```json
{"id":"1","ok":false,"error":{"code":"AUTH_REQUIRED","message":"root authorization required"}}
```

Maximum request size SHALL be bounded; recommended default: 1 MiB.

## 4. Peer identity

The agent SHALL inspect Unix peer credentials and record at least:

```text
uid
gid
pid
```

for mutating requests.

No user-provided UID field is trusted.

## 5. Socket ownership

Recommended runtime ownership:

```text
owner: cockpit-ups-wol
group: cockpit-ups-wol
mode: 0660
```

The exact service user/group is created by the installer.

Read access for ordinary Cockpit sessions is normally mediated through the local CLI/helper rather than exposing the socket to all users.

## 6. Authorization classes

Methods are divided into:

```text
READ
CONFIG_WRITE
POWER_CONTROL
ADMIN
```

### READ

May be allowed to authenticated local users according to installer policy.

Examples:

```text
GetStatus
GetHealth
GetConfigSummary
ListConfigRevisions
GetPowerPlan
GetHostStatus
```

### CONFIG_WRITE

Requires privileged authorization.

Examples:

```text
BeginConfigTransaction
ValidateConfigCandidate
CommitConfigCandidate
RollbackConfig
SetOperatingMode
```

### POWER_CONTROL

Requires privileged authorization and explicit user confirmation at the UI/CLI layer.

Examples:

```text
TriggerWake
TriggerShutdown
TriggerFSD
CancelPendingRecovery
```

### ADMIN

Requires privileged authorization.

Examples:

```text
ReloadConfig
RunHealthCheck
AttemptRepair
AcknowledgeFailedSafe
ResetTransactionRetries
```

## 7. Cockpit authorization path

The Cockpit extension SHALL use a small local CLI client, tentatively:

```text
cockpit-ups-wolctl
```

Read operations can be executed without privilege where permitted.

For privileged operations, Cockpit uses its authenticated superuser mechanism to execute the CLI as root. The agent verifies root peer credentials on the Unix socket.

This keeps browser input separated from direct privileged file/system operations.

Future versions may replace/root-split individual operations with polkit actions, but v0.1 SHALL preserve the same operation-level authorization semantics.

## 8. Required methods

### GetStatus

Returns:

```text
power state
normalized/raw UPS state
operating mode
active outage transaction
active config revision
last-known-good revision
managed-host summary
recovery-gate summary
```

Authorization: READ.

### GetHealth

Returns overall health and individual check results.

Authorization: READ.

### GetPowerPlan

Returns the exact calculated dry-run shutdown/recovery sequence with priorities, NUT-secondary grouping and dependencies.

Authorization: READ.

### GetHostStatus

Returns current configured/observed host state.

Authorization: READ.

### ListConfigRevisions

Returns revision metadata, never unredacted secrets.

Authorization: READ.

### BeginConfigTransaction

Creates a candidate revision from proposed configuration content and returns a transaction/revision ID.

Authorization: CONFIG_WRITE.

### ValidateConfigCandidate

Runs schema, cross-reference and component preflight validation.

Authorization: CONFIG_WRITE.

### CommitConfigCandidate

Atomically activates a valid candidate and begins runtime probation. Success response means activation started, not necessarily that the candidate is already known-good.

Authorization: CONFIG_WRITE.

### RollbackConfig

Restores a selected known-good revision transactionally.

Authorization: CONFIG_WRITE.

### ReloadConfig

Reloads the current known-good active revision when supported.

Authorization: ADMIN.

### SetOperatingMode

Allowed transitions among:

```text
monitor
dry-run
armed
maintenance
```

Entering `armed` SHALL require validation that mandatory safety prerequisites pass.

Authorization: CONFIG_WRITE.

### TriggerWake

Explicit manual wake for one host.

Authorization: POWER_CONTROL.

### TriggerShutdown

Explicit managed shutdown for one configured host, using its allowlisted adapter.

Authorization: POWER_CONTROL.

### TriggerFSD

Requests the local NUT primary FSD path. This is a dangerous operation and only exists when the deployment is the validated NUT primary.

Authorization: POWER_CONTROL.

### CancelPendingRecovery

Cancels an uncommitted pending automatic recovery. It SHALL NOT undo `RECOVERY_STARTED` after recovery commit.

Authorization: POWER_CONTROL.

### RunHealthCheck

Runs checks immediately and returns details.

Authorization: ADMIN.

### AttemptRepair

Runs bounded safe autofix for specified failed checks.

Authorization: ADMIN.

### AcknowledgeFailedSafe

Records administrator acknowledgement and attempts re-reconciliation; it does not blindly clear the underlying failure.

Authorization: ADMIN.

## 9. Request envelope

Required request fields:

```json
{
  "id": "client-generated-string",
  "method": "GetStatus",
  "params": {}
}
```

Optional:

```text
client_version
request_nonce
```

The server responds with the same `id`.

## 10. Error codes

Canonical v0.1 codes:

```text
INVALID_REQUEST
UNKNOWN_METHOD
INVALID_PARAMS
AUTH_REQUIRED
FORBIDDEN
NOT_FOUND
CONFLICT
UNSAFE_STATE
NOT_SUPPORTED
VALIDATION_FAILED
DEPENDENCY_UNAVAILABLE
TIMEOUT
FAILED_SAFE
INTERNAL_ERROR
```

A human-readable `message` accompanies the stable code.

## 11. Concurrency

Read methods may run concurrently.

Mutating operations use internal locks:

```text
config transaction lock
power transaction lock
state write lock
```

Only one configuration transaction may be active at a time.

Power-control methods SHALL return `CONFLICT`/`UNSAFE_STATE` instead of racing an incompatible active automatic transaction.

## 12. Idempotency

Mutating calls SHOULD accept an optional `request_nonce`.

The agent may retain recent nonces to avoid duplicate actions caused by UI retry/network interruption.

Critical external actions also use durable internal action IDs tied to the power transaction.

## 13. Events / live updates

v0.1 may use polling for Cockpit status.

Recommended intervals:

```text
UPS/power state: 5 s
host/service summary: 10 s
logs: journald stream through Cockpit APIs
```

A future socket subscription/event stream may be added without changing the basic request/response methods.

## 14. Versioning

The IPC server reports:

```text
protocol_version: 1
agent_version: <release>
```

Unknown major protocol versions SHALL fail clearly rather than silently misinterpreting methods.

## 15. Audit logging

Every privileged mutating request logs:

```text
time
peer uid/pid
method
target
result
power transaction ID when applicable
active config revision
```

Secrets and raw private-key material are never logged.

## 16. Security boundaries

- no TCP management API in v0.1
- no arbitrary shell command method
- no direct path supplied by browser to execute arbitrary binaries
- host `command` shutdown uses predefined allowlisted command IDs
- dangerous UPS commands are separate from ordinary health/config methods
- agent validates state again at execution time; UI confirmation alone is not sufficient safety validation

## 17. Acceptance tests

Required tests:

```text
read method from permitted local client
privileged method rejected for unprivileged peer
root privileged method accepted
unknown method rejected
oversized request rejected
malformed JSON does not crash agent
concurrent reads succeed
concurrent config transactions conflict
power command conflicts with unsafe active transaction
duplicate request nonce does not duplicate destructive action
agent restart recreates socket with safe permissions
```
