# Contributing to cockpit-ups-wol

Thanks for helping improve `cockpit-ups-wol`.

This project controls shutdown, UPS/NUT coordination, Wake-on-LAN and recovery behavior. Changes that affect power-state transitions, NUT authority, configuration rollback, service recovery or privileged execution must preserve the project's fail-safe behavior.

## Before opening a change

1. Search existing issues and pull requests.
2. For behavior changes, describe the failure mode or operational need first.
3. Keep changes scoped. Avoid mixing refactors with safety-policy changes unless required.
4. Do not include real credentials, SSH private keys, production MAC/IP inventories or private UPS data in commits, fixtures or logs.

## Development checks

Run the checks relevant to your change before opening a pull request.

### Go agent

```bash
cd agent
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
```

### Cockpit UI

```bash
cd cockpit
npm ci --ignore-scripts --no-audit --no-fund
npm run typecheck
npm run lint
npm run format:check
npm test
npm run build
```

### Shell and installer

```bash
find . -type f -name '*.sh' -not -path './.git/*' -print0 | xargs -0 -n1 bash -n
./scripts/test/installer-unit.sh
./install.sh --check --profile local-server
./install.sh --check --profile remote-client
./install.sh --check --profile existing
```

The GitHub Actions workflows remain the authoritative merge checks.

## Safety-sensitive changes

Changes in the following areas should include regression tests for both the expected path and the failure/recovery path:

- outage detection and grace timing
- shutdown ordering and NUT/FSD ownership
- controller shutdown
- interrupted boot and power-bounce recovery
- Wake-on-LAN retry/recovery ordering
- battery/recharge/stable-AC gates
- configuration transactions and rollback
- health autofix / `FAILED_SAFE`
- privileged IPC, SSH execution and credential handling

Do not weaken dry-run defaults or silently convert unknown/ambiguous state into a destructive action.

## Pull requests

A pull request should explain:

- what changed
- why it changed
- safety/recovery implications
- tests run
- documentation changes, if any

Operational behavior changes should update the corresponding document under `docs/` and, where appropriate, `CHANGELOG.md`.

## Hardware acceptance

Simulation, QEMU and NUT `dummy-ups` coverage do not replace the physical release gates documented in `docs/HARDWARE_ACCEPTANCE.md`.

## Licensing

By submitting a contribution, you agree that it may be distributed under the project's `AGPL-3.0-or-later` license. Third-party code must retain required attribution and must be documented in `THIRD_PARTY_NOTICES.md` when applicable.
