# cockpit-ups-wol — Installation Requirements

**Requirements version:** 0.3  
**Status:** Canonical v0.1 installer baseline

## 1. Single entry point

Exactly one installer entry point:

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

All modes use the same backend. `--silent` means unattended, not quiet.

Invalid combinations such as `--silent --tui` fail before modifying the system.

## 2. Installer logging

All modes write persistent logs under:

```text
/var/log/cockpit-ups-wol/
```

Suggested current log:

```text
/var/log/cockpit-ups-wol/install.log
```

Logs include timestamps/severity but never secrets.

## 3. Clean-OS installation

The installer SHALL not assume these are installed:

```text
Cockpit
NUT
Go
Node.js/npm
git
make
compiler
dialog/whiptail
```

Normal release installation uses prebuilt project artifacts.

## 4. Supported platforms

Tier 1:

```text
Debian
Ubuntu
Arch Linux
```

Tier 2 target:

```text
Fedora
Rocky Linux
AlmaLinux
Raspberry Pi OS
Armbian
```

Required release architectures:

```text
amd64
arm64
riscv64
```

Optional later: `armhf`.

Unsupported architecture is rejected before system mutation.

## 5. Target resource profile

Full Cockpit appliance target:

```text
RAM: 512 MiB minimum target, 1 GiB recommended
storage: at least 4 GiB available system storage recommended
Ethernet: preferred
USB host: required for local USB UPS
```

Milk-V Duo 256M / Duo S remain validation targets where the OS/Cockpit footprint permits. The original 64 MiB Duo is not a full-stack target.

## 6. Core installer responsibilities

The installer SHALL:

1. parse options
2. obtain/check root privileges
3. initialize logging
4. detect OS/package manager/architecture/platform
5. check resource prerequisites
6. install Cockpit
7. install NUT
8. install `cockpit-ups-wol-agent`
9. install `cockpit-ups-wolctl` / `wolctl`
10. install Cockpit extension
11. configure selected NUT profile
12. configure optional Synology compatibility
13. create config/state/history/secrets/runtime directories
14. install systemd units and timer
15. enable required services for automatic startup
16. create initial candidate configuration
17. validate and activate candidate
18. start/reload services
19. run immediate health validation
20. run configuration probation
21. mark initial revision known-good only after success
22. initialize `active`, `last-known-good`, `previous-known-good` metadata
23. produce final validation/report

Installation is not successful until a known-good baseline exists.

## 7. Required runtime services

For a local-server profile, expected components include distro-equivalent units for:

```text
cockpit.socket
NUT driver service instance(s)
nut-server.service
nut-monitor.service
cockpit-ups-wol-agent.service
cockpit-ups-wol-health.timer
```

Exact NUT unit names vary by distribution and SHALL be detected rather than globally hard-coded.

Every required selected unit must be enabled for reboot/autostart.

## 8. systemd recovery

Installed persistent project services SHALL use bounded failure recovery appropriate to their role, e.g. concepts equivalent to:

```text
Restart=on-failure
bounded StartLimit
watchdog for agent progress where supported
```

Dependency unavailability (network, USB, NUT) should use retry/backoff rather than tight process restart loops.

## 9. TUI

`--tui` uses `dialog`, with `whiptail` fallback where practical.

The TUI should cover:

```text
system summary
package plan
UPS/NUT profile
UPS device selection
Synology compatibility
network security mode
outage/recovery policy
controller power topology confirmation
network dependencies
initial hosts
operating mode
review
installation progress
health/probation result
final report
```

## 10. Silent mode

`--silent`:

- never prompts
- uses documented safe defaults
- remains verbose
- fails instead of guessing when a safe choice cannot be made
- returns zero only after full validation/probation succeeds

Ambiguous UPS selection requires explicit CLI parameters.

## 11. UPS detection

The installer SHOULD use supported NUT discovery mechanisms where available.

If exactly one suitable device is found, interactive/TUI may propose it.

If ambiguous:

- interactive asks
- TUI lists choices
- silent mode requires explicit selection

Supported options SHOULD include:

```text
--ups-name
--ups-driver
--ups-port
```

Default UPS name: `ups`.

## 12. NUT profiles

Required:

```text
local-server
remote-client
existing
```

### local-server

Controller hosts driver/upsd and is intended to become NUT primary after validation.

### remote-client

Controller reads a remote NUT server; it does not automatically receive primary/FSD/output-control authority.

### existing

Project integrates with existing NUT configuration conservatively and does not blindly replace it.

Ownership details: `docs/NUT_SHUTDOWN_MODEL.md`.

## 13. Synology compatibility

CLI/TUI option:

```text
--synology
```

Preset:

```text
UPS name       ups
NUT port       3493
network mode   trusted-lan
monitor user   monuser
password       secret
role           upsmon secondary
```

Legacy `slave` syntax may be used only where required by installed NUT version.

Compatibility account is monitor-only and SHALL NOT receive SET/FSD/unrestricted instant-command permissions.

## 14. NUT network policy

Default:

```text
trusted-lan
```

Optional:

```text
restricted
```

Restricted mode may use distro-appropriate nftables/ufw/firewalld integration but SHALL not destructively replace unknown firewall rules.

If IPv6 listening is enabled, restrictions must cover IPv6 too; IPv4-only protection cannot leave an unintentionally open IPv6 service.

NUT SHALL never be intentionally exposed to the public Internet by the installer.

## 15. Controller deployment checks

Before arming automatic power behavior, installer/TUI SHALL check or request confirmation that:

```text
controller uses UPS battery-backed output
controller auto-boots when backed power returns
required switch/router/VLAN path remains powered long enough
UPS data link works
```

Hardware facts that cannot be detected automatically are marked as administrator-confirmed.

The installer SHALL warn prominently if the controller is known to be on surge-only/non-backed power.

See `docs/DEPLOYMENT.md`.

## 16. UPS output-cycle capability

The installation records:

```text
POWER_CYCLE_VERIFIED
POWER_CYCLE_UNVERIFIED
MONITOR_ONLY
```

New hardware starts `POWER_CYCLE_UNVERIFIED` unless validated.

Full unattended controller poweroff/reboot behavior SHALL not rely on an unverified output-return sequence without explicit acknowledgement/alternate verified restart mechanism.

## 17. Operating mode default

New installations default to:

```text
dry-run
```

The final report explains how to review the calculated power plan and explicitly arm automation.

`armed` mode requires mandatory safety checks to pass.

## 18. Recovery defaults

```text
automatic recovery       enabled
minimum charge           80%
utility stable period    120 s
network wait             300 s
```

CLI overrides SHOULD include:

```text
--recovery-charge
--utility-stable-seconds
--network-wait-seconds
```

## 19. Outage defaults

Suggested default:

```text
grace period 120 s
```

Optional thresholds for critical charge/runtime/time-on-battery follow the canonical configuration schema.

## 20. Canonical configuration

Generated config SHALL validate against:

```text
schemas/config.schema.json
```

and follow `docs/CONFIGURATION.md`.

Schema version:

```yaml
config_version: 1
```

Unknown newer schemas are never overwritten.

## 21. Existing NUT configuration

Detect existing NUT files/services.

Before authorized mutation:

- preserve exact original content/metadata
- avoid replacing unrelated admin settings
- create project-managed fragments where supported
- fail in silent mode if safe migration cannot be determined

## 22. Initial configuration transaction

Fresh install flow:

```text
create candidate
→ static/schema validation
→ semantic/cross-reference validation
→ component preflight
→ atomic activation
→ start/reload services
→ immediate health check
→ probation (default 60 s)
→ known-good
```

If any mandatory stage fails:

- mark candidate failed
- restore prior project-owned state when applicable
- leave a clear install failure
- never claim success without known-good baseline

## 23. Config revision storage

Installer creates:

```text
/var/lib/cockpit-ups-wol/config-history/
```

and initializes atomic metadata for:

```text
active
last-known-good
previous-known-good
```

Known-good revisions are immutable.

## 24. Project runtime/state directories

Create at least:

```text
/etc/cockpit-ups-wol/
/etc/cockpit-ups-wol/secrets/
/var/lib/cockpit-ups-wol/state/
/var/lib/cockpit-ups-wol/config-history/
/var/log/cockpit-ups-wol/
/run/cockpit-ups-wol/
```

Apply minimal ownership/permissions for each component.

## 25. Interrupted installation / power loss

Installation/config transactions SHALL leave enough durable metadata that reboot can distinguish:

```text
known-good active config
candidate/validating config
incomplete rollback
```

A power interruption during probation never promotes the candidate.

On next boot, reliability logic restores trusted power-management operation before attempting to continue a non-essential interrupted config change.

## 26. Upgrade detection

Installer distinguishes:

```text
fresh install
same-version reinstall
upgrade
```

Rerunning is idempotent.

## 27. Upgrade transaction

Before upgrade:

```text
record application version
record active/LKG config revisions
backup project-owned binaries/bundle/units needed for rollback
preserve user config
```

Then:

```text
install new artifacts
migrate config as candidate
restart/reload
health check
probation
```

On failure:

```text
restore previous application artifacts
restore previous known-good config
restart
validate
```

An upgrade is committed only after the upgraded application/configuration is healthy.

## 28. Binary rollback retention

v0.1 installer SHALL retain at least the immediately previous installed project application version until the new version passes probation.

This includes, as applicable:

```text
agent/CLI binaries
Cockpit bundle
project-owned systemd units/helpers
version manifest
```

Config revision retention is governed separately by reliability requirements.

## 29. Artifact integrity

Release downloads include and verify:

```text
SHA256SUMS
```

Checksum mismatch aborts installation before replacing active artifacts.

Future signature verification may supplement checksums.

## 30. Network configuration

Installer SHALL recommend stable controller addressing using DHCP reservation or static configuration.

It SHALL not rewrite existing network configuration without explicit request.

Network/DHCP may become ready after the agent starts; boot behavior must tolerate this.

## 31. Installation validation

Required checks include:

```text
supported OS/CPU
Cockpit installed/socket enabled
NUT commands/services present
selected NUT profile coherent
NUT config accepted where validators exist
agent/CLI binaries executable
agent config valid
agent service enabled/healthy
health timer enabled
Cockpit extension installed/manifest valid
state/history directories safe/writable
IPC socket created with safe permissions
active config revision exists
last-known-good exists after probation
wolctl basic self-test passes
```

Synology mode also validates:

```text
UPS name = ups
TCP 3493 service configured
monuser exists
monitor-only privilege
```

Physical UPS communication status is reported distinctly from software installation health.

## 32. Non-destructive simulation validation

Installer/release validation SHOULD exercise simulation for:

```text
on-battery
low-battery
power-restored
communication failure
battery recovery gate
network delay
host restore plan
config validation rollback
```

No real shutdown/WoL occurs during installation validation.

## 33. Final report

The final report SHALL include:

```text
OS / architecture
installed version
services enabled/healthy
NUT profile / UPS
Synology compatibility
operating mode (dry-run by default)
outage/recovery policy
UPS power-cycle capability classification
controller backed-power confirmation status
controller auto-power-on confirmation status
active config revision
last-known-good revision
Cockpit address
installer log path
warnings blocking armed mode
```

## 34. Installation acceptance criteria

A release is installation-ready only when a clean supported OS can run unattended installation with required explicit hardware parameters and achieve:

```text
✓ all dependencies installed
✓ all selected services auto-start
✓ agent/health supervision healthy
✓ NUT configured
✓ Synology preset available
✓ config schema validated
✓ initial config promoted to known-good after probation
✓ rollback metadata initialized
✓ default operating mode is dry-run
✓ 80% recovery default present
✓ repeated installer execution safe
✓ reboot returns complete selected service stack automatically
```
