# Local Agent IPC Contract

**Status:** Normative for the implemented v0.1 IPC surface  
**Important:** this document describes what is implemented now, not the broader historical design sketch.

## 1. Purpose

`cockpit-ups-wol-agent` exposes a small local Unix-domain socket for runtime queries that benefit from talking directly to the running agent.

Default path:

```text
/run/cockpit-ups-wol/agent.sock
```

Cockpit does **not** directly edit power-state files or active configuration. Privileged configuration management is performed through `cockpit-ups-wolctl` and the transactional configuration revision manager.

## 2. Current implementation boundary

The implemented v0.1 agent IPC method is:

```text
GetHealth
```

Other historical design methods such as `GetStatus`, `GetPowerPlan`, `BeginConfigTransaction`, `TriggerWake`, `TriggerShutdown`, and `TriggerFSD` are **not current agent IPC methods** and must not be documented or consumed as if they exist.

Current management paths are:

```text
health query          -> agent Unix socket / GetHealth
config read           -> cockpit-ups-wolctl config-get
config validation     -> cockpit-ups-wolctl config-validate
config apply          -> cockpit-ups-wolctl config-apply
config status         -> cockpit-ups-wolctl config-status
config rollback       -> cockpit-ups-wolctl config-rollback <revision>
power plan            -> cockpit-ups-wolctl plan
logs                  -> cockpit-ups-wolctl logs / journald
```

`config-apply` starts activation/probation in a transient systemd unit so the operation continues independently of the browser/Cockpit channel.

## 3. Transport

Protocol: one newline-delimited JSON request followed by one JSON response over an `AF_UNIX` stream connection.

The current server handles **one request per connection**.

Request:

```json
{"id":"1","method":"GetHealth","params":{}}
```

Successful response shape:

```json
{"id":"1","ok":true,"result":{}}
```

Error response shape:

```json
{"id":"1","ok":false,"error":{"code":"UNKNOWN_METHOD","message":"unknown method"}}
```

The exact health snapshot fields are defined by the health package and may evolve compatibly.

## 4. Limits and connection behavior

Current implementation properties:

```text
maximum request bytes: 1 MiB
socket mode:           0660 by default
connection deadline:   30 seconds
one request/response per connection
```

Malformed JSON, oversized requests and unknown methods fail cleanly instead of crashing the agent.

The server removes a stale socket path before binding and removes its socket when shutting down normally.

## 5. Request envelope

Required fields:

```json
{
  "id": "client-generated-string",
  "method": "GetHealth",
  "params": {}
}
```

`id` and `method` are mandatory. The response echoes the request ID when available.

The current transport does not implement a generic nonce/idempotency layer; durable idempotency for safety-critical side effects belongs to the power transaction/state machinery, not this health-query method.

## 6. Current error codes

The current server/handler may return at least:

```text
INVALID_REQUEST
UNKNOWN_METHOD
DEPENDENCY_UNAVAILABLE
INTERNAL_ERROR
```

Do not rely on older draft-only error codes until corresponding methods are actually implemented and tested.

## 7. Authorization and privilege boundary

The Unix socket is local-only; there is no project TCP management API in v0.1.

Current privileged configuration operations do **not** depend on an unimplemented generic IPC authorization layer. `cockpit-ups-wolctl` checks effective root privilege for operations such as apply/activate/rollback, while Cockpit uses its authenticated superuser path to invoke the CLI.

The current socket server does not claim a completed `SO_PEERCRED` authorization framework for mutating IPC operations because no mutating IPC methods are currently exposed.

If future mutating agent IPC methods are added, they must add and test an explicit peer-credential authorization boundary before being considered accepted functionality.

## 8. Cockpit usage

Cockpit uses the project control CLI as its stable local management boundary. This keeps browser input separated from direct privileged filesystem/system operations and allows configuration activation to be handed to systemd for browser-independent probation.

The UI may poll health/status data through the available CLI/control paths. Journald access uses Cockpit/system tooling rather than a custom log-streaming socket protocol.

## 9. Concurrency

The server accepts connections concurrently. Each connection is serviced independently, with one request per connection.

Configuration transaction concurrency is handled by the configuration manager/revision store, not by a generic IPC mutation lock. Power transaction/state locking remains internal to the runtime safety engine.

## 10. Security boundaries

v0.1 guarantees:

- no project TCP management listener;
- bounded local Unix-socket request size;
- malformed requests do not crash the agent;
- no arbitrary shell execution method on IPC;
- privileged config mutation is performed through the root-gated CLI/revision manager;
- the UI does not directly mutate durable power state;
- unsupported methods fail closed as `UNKNOWN_METHOD`.

## 11. Acceptance tests

Current IPC acceptance should cover:

```text
GetHealth success
GetHealth provider failure
unknown method rejected
missing id/method rejected
malformed JSON rejected
oversized request rejected
one request/response per connection
agent restart recreates the socket
client detects response-id mismatch
```

Future methods require their own authorization, concurrency, idempotency and safety tests before this document may list them as implemented.

## 12. Future extension rule

Potential future IPC methods may include richer status, event subscriptions or privileged operations, but documentation must follow implementation. A method becomes part of the normative IPC contract only after code, authorization behavior and regression tests exist.
