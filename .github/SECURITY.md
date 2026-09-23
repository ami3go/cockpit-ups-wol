# Security Policy

`cockpit-ups-wol` is management-plane software that can coordinate system shutdown, NUT/FSD behavior and Wake-on-LAN recovery. Security-sensitive reports should therefore avoid public disclosure of credentials, exploit details or production infrastructure data.

## Reporting a vulnerability

If GitHub shows **Report a vulnerability** for this repository, use that private reporting path.

If private vulnerability reporting is not available, open a public issue containing only a minimal non-sensitive description and ask the maintainer for a private reporting channel. Do not include proof-of-concept exploit code, secrets, private keys, credentials, production addresses, or other details that would materially increase exploitation risk.

## Supported versions

The project is currently pre-release. Security fixes are made against the current `main` development line until a versioned support policy is published.

## Security model

The normative v0.1 security assumptions, trust boundaries, credential handling, NUT authority model, privileged execution requirements, update integrity and security acceptance tests are documented in [`docs/SECURITY.md`](../docs/SECURITY.md).

Deployments are intended for a trusted or explicitly restricted management LAN and should not expose Cockpit or NUT directly to the public Internet.
