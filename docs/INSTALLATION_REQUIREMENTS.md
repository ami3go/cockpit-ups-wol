# cockpit-ups-wol — Installation Requirements

**Requirements version:** 0.2  
**Status:** Implementation baseline

## 1. Single Installation Entry Point

The project SHALL provide exactly one installation entry point:

```bash
sudo ./install.sh
```

The project SHALL NOT provide separate TUI or silent installers.

All installation modes use the same installer backend.

## 2. Installation Modes

Required modes:

```bash
sudo ./install.sh
sudo ./install.sh --tui
sudo ./install.sh --silent
```

Meaning:

| Mode | Interaction | Output | Log |
|---|---|---|---|
| default | minimal prompts | verbose | full |
| `--tui` | guided | TUI + progress | full |
| `--silent` | none | verbose | full |

Optional:

```text
--verbose
--debug
--quiet
```

`--silent` means **unattended**, not quiet.

This is invalid:

```bash
sudo ./install.sh --silent --tui
```

and SHALL fail immediately.

## 3. Persistent Installer Logging

All modes SHALL write:

```text
/var/log/cockpit-ups-wol/install.log
```

Historical logs SHOULD be retained using timestamps.

Console and log output SHALL include timestamps and severity:

```text
[11:21:02] [INFO] Installing Cockpit...
[11:21:14] [OK]   Cockpit installed
[11:21:15] [WARN] Existing NUT configuration found
[11:21:15] [INFO] Configuration preserved
```

Secrets SHALL NOT be logged.

## 4. Clean-OS Requirement

The installer SHALL support installation on a clean supported OS.

It SHALL NOT assume the presence of:

```text
Cockpit
NUT
Go
Node.js
npm
git
make
compiler
dialog
```

Runtime systems SHOULD receive prebuilt release artifacts.

Go and frontend build tools belong in CI/release infrastructure.

## 5. Supported Distribution Families

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

Distribution logic SHALL be internal modules, not separate installers.

Example:

```text
scripts/lib/distro/debian.sh
scripts/lib/distro/arch.sh
scripts/lib/distro/fedora.sh
```

## 6. CPU Architectures

Required release architectures:

```text
amd64
arm64
riscv64
```

Optional:

```text
armhf
```

Normalization:

```text
x86_64  → amd64
aarch64 → arm64
riscv64 → riscv64
armv7l  → armhf
```

Unsupported architectures SHALL be rejected before modifying the system.

## 7. Milk-V

Explicit validation targets SHOULD include:

```text
Milk-V Duo 256M
Milk-V Duo S
```

Preferred RISC-V test configuration:

```text
Milk-V Duo 256M
+
Debian riscv64
```

The original Milk-V Duo 64M is not guaranteed to support the complete Cockpit appliance.

For highly constrained systems, a future headless mode MAY provide:

```text
NUT + agent + wolctl
```

without Cockpit.

## 8. Installer Responsibilities

`install.sh` SHALL:

1. parse options
2. obtain/check root privileges
3. initialize logging
4. detect OS
5. detect package manager
6. detect CPU architecture
7. detect SBC/platform where possible
8. check RAM/storage
9. install Cockpit
10. install NUT
11. install `cockpit-ups-wol-agent`
12. install `wolctl`
13. install Cockpit extension
14. configure NUT
15. configure Synology compatibility when enabled
16. create configuration directories
17. create persistent state directory
18. install systemd services
19. configure permissions
20. preserve existing user configuration
21. enable required services
22. validate complete installation
23. display final report

## 9. TUI

TUI mode SHALL be selected only with:

```bash
sudo ./install.sh --tui
```

Preferred implementation:

```text
dialog
```

Fallback:

```text
whiptail
```

The TUI SHOULD provide:

- detected system
- package plan
- UPS selection
- NUT mode
- Synology compatibility
- network security mode
- recovery threshold
- outage delay
- initial managed hosts
- review screen
- installation progress
- log viewer
- final validation

## 10. Silent Mode

```bash
sudo ./install.sh --silent
```

SHALL:

- never prompt
- use documented safe defaults
- remain verbose
- write complete logs
- fail instead of guessing when a safe automatic decision cannot be made
- return zero only after validation succeeds

Example CI/provisioning usage:

```bash
curl -fsSL <installer-location> | sudo bash -s -- --silent
```

## 11. UPS Detection

The installer SHOULD attempt automatic UPS detection using available NUT mechanisms.

If exactly one suitable UPS is found, the installer MAY propose/use it.

If selection is ambiguous:

- interactive mode asks
- TUI provides selection
- silent mode requires explicit parameters

Supported explicit options SHOULD include:

```text
--ups-name
--ups-driver
--ups-port
```

Example:

```bash
sudo ./install.sh \
  --silent \
  --ups-name ups \
  --ups-driver usbhid-ups \
  --ups-port auto
```

Silent mode SHALL not arbitrarily select between multiple detected UPS devices.

## 12. NUT Profiles

Required NUT profiles:

```text
Local UPS server
Remote NUT client
Existing NUT installation
```

Local server configuration SHOULD be compatible with Synology NAS by default where practical.

Default UPS identifier:

```text
ups
```

## 13. Synology Compatibility

The installer SHALL provide:

```text
--synology
```

and a corresponding TUI option.

Example:

```bash
sudo ./install.sh --synology
```

Silent:

```bash
sudo ./install.sh --silent --synology
```

Synology compatibility SHALL configure:

```text
UPS name       ups
NUT port       3493
network mode   trusted-lan
```

Compatibility monitoring account:

```ini
[monuser]
    password = secret
    upsmon secondary
```

For older NUT versions the installer MAY use the legacy equivalent:

```ini
upsmon slave
```

The compatibility account SHALL remain monitor-only.

It MUST NOT receive:

```text
SET
FSD
instcmds = ALL
```

The known compatibility credentials SHALL only be created when Synology compatibility is selected or required by the selected profile.

## 14. NUT Network Access

Default:

```text
trusted-lan
```

The project assumes deployment behind a trusted home/router firewall.

NUT SHALL be reachable on:

```text
TCP 3493
```

The installer SHALL NOT require individual NAS/client IP addresses in default mode.

Example:

```bash
sudo ./install.sh --silent --synology
```

is valid.

## 15. Optional Restricted NUT Access

Optional security mode:

```text
restricted
```

CLI:

```bash
sudo ./install.sh \
  --synology \
  --restrict-nut-access \
  --nut-client 192.168.1.20
```

Subnet:

```bash
sudo ./install.sh \
  --restrict-nut-access \
  --nut-subnet 192.168.1.0/24
```

Possible firewall backends:

```text
nftables
ufw
firewalld
```

Restriction SHALL NOT be enabled by default.

An unknown existing firewall configuration SHALL not be destructively rewritten.

## 16. Recovery Defaults

New installations SHALL use documented recovery defaults:

```text
automatic recovery       enabled
minimum UPS charge       80%
utility stable period    120 seconds
network wait             300 seconds
```

TUI SHALL expose these values before installation completes.

CLI overrides SHOULD include:

```text
--recovery-charge
--utility-stable-seconds
--network-wait-seconds
```

Example:

```bash
sudo ./install.sh \
  --silent \
  --recovery-charge 80 \
  --utility-stable-seconds 120
```

## 17. Outage Defaults

Suggested default:

```text
outage grace period 120 seconds
```

CLI:

```text
--outage-grace-seconds
```

This value SHALL remain configurable later through Cockpit.

## 18. Configuration Versioning

Generated configuration SHALL include:

```yaml
config_version: 1
```

Installer upgrades SHALL migrate older supported configuration versions.

Unknown newer schemas SHALL not be overwritten.

## 19. Existing NUT Configuration

Existing NUT files SHALL be detected:

```text
/etc/nut/ups.conf
/etc/nut/upsd.conf
/etc/nut/upsd.users
/etc/nut/upsmon.conf
```

They SHALL NOT be overwritten automatically.

Example:

```text
[INFO] Existing NUT configuration detected
[INFO] Preserving /etc/nut/ups.conf
```

TUI MAY offer a reviewed migration.

Silent mode SHALL preserve the configuration and fail if migration is required but cannot be performed safely.

## 20. Project Configuration Safety

Before modifying an existing installation, the installer SHALL back up project-controlled configuration.

Suggested location:

```text
/var/backups/cockpit-ups-wol/
```

Example:

```text
2026-09-19_115201/
├── config.yaml
├── hosts.yaml
└── service-state.txt
```

## 21. Upgrade and Rollback

The installer SHALL detect:

```text
fresh install
same-version reinstall
upgrade
```

Upgrade flow:

```text
validate current installation
        ↓
backup configuration
        ↓
install new files
        ↓
run schema migration
        ↓
restart/reload services
        ↓
validate
        │
        ├── success → keep upgrade
        └── failure → restore previous configuration/files
```

User configuration SHALL be preserved.

## 22. Idempotency

Repeated execution SHALL be safe:

```bash
sudo ./install.sh --silent
sudo ./install.sh --silent
```

Expected behavior:

```text
Cockpit                 already installed
NUT                     already installed
cockpit-ups-wol-agent   already installed
wolctl                  already installed
configuration           preserved
validation              successful
```

## 23. Release Artifact Integrity

Downloaded project release binaries SHALL be integrity checked.

Release assets SHOULD include:

```text
SHA256SUMS
```

The installer SHALL verify checksums before installing downloaded binaries.

Checksum failure SHALL terminate installation.

## 24. Runtime Directories

Installer SHALL create:

```text
/etc/cockpit-ups-wol/
/var/lib/cockpit-ups-wol/
/var/log/cockpit-ups-wol/
```

Responsibilities:

```text
/etc     persistent user configuration
/var/lib persistent runtime/recovery state
/var/log installer logs
```

Runtime agent logs SHOULD primarily use journald.

## 25. systemd

The installer SHALL configure:

```text
cockpit.socket
nut-server.service
nut-monitor.service
cockpit-ups-wol-agent.service
```

Exact NUT unit names may vary by distribution.

The installer SHALL use distro-aware service detection rather than assuming identical names everywhere.

## 26. Network Readiness

The agent SHALL be enabled to start automatically after reboot.

It SHALL tolerate the network coming up later than the service.

The installer SHALL NOT encode a fragile assumption that Ethernet/DHCP is ready immediately when the agent starts.

## 27. Stable Address Recommendation

Because network clients such as Synology must locate the NUT server, installation SHOULD display a recommendation to provide the controller with a stable address using either:

```text
DHCP reservation
```

or:

```text
static IP configuration
```

The installer SHALL not automatically replace existing network configuration unless explicitly requested.

## 28. Installation Validation

Installation SHALL not report success before verifying:

```text
✓ supported OS
✓ supported CPU
✓ Cockpit installed
✓ Cockpit available
✓ NUT commands installed
✓ NUT configuration valid
✓ NUT service state acceptable
✓ agent executable available
✓ agent configuration valid
✓ agent service enabled
✓ wolctl executable works
✓ Cockpit plugin installed
✓ Cockpit manifest valid
✓ state directory writable
✓ configuration permissions valid
```

Synology mode additionally verifies:

```text
✓ UPS identifier = ups
✓ TCP 3493 listener configured
✓ monuser compatibility account exists
✓ account is monitor-only
```

Physical UPS communication MAY be reported separately so software installation can still complete when hardware is temporarily disconnected.

## 29. Simulation Validation

Installer SHOULD perform non-destructive checks for the agent.

The release test suite SHALL exercise:

```text
on-battery
low-battery
power-restored
battery-threshold
network-delay
host-restore
```

A real machine SHALL never be shut down during installation validation.

## 30. TUI Installation Review

Before modifying the system, TUI SHALL display a review screen similar to:

```text
System
  Debian 13
  riscv64
  Milk-V Duo 256M

Components
  Cockpit                 Install
  NUT                     Install
  Power agent             Install
  wolctl                  Install
  Cockpit plugin          Install

UPS
  Mode                    Local server
  Name                    ups
  Driver                  usbhid-ups

Synology
  Compatibility           Enabled
  Network                 Trusted LAN

Recovery
  Auto restore            Enabled
  Battery threshold       80%
  Utility stable          120 s

[ Back ]                 [ Install ]
```

## 31. Final Installation Report

Example:

```text
============================================================
 cockpit-ups-wol installation summary
============================================================

System
  OS               Debian 13
  Architecture     riscv64
  Platform         Milk-V Duo 256M

Services
  Cockpit          OK
  NUT              OK
  Power agent      OK
  wolctl           OK
  Cockpit plugin   OK

UPS
  Name             ups
  Status           Online

Synology
  Compatibility    Enabled
  Port             3493
  Access           Trusted LAN
  Monitor account  OK

Automation
  Outage grace     120 s
  Recovery         Enabled
  Battery minimum  80%
  Stable AC        120 s

Cockpit
  https://<controller-address>:9090/

Installer log
  /var/log/cockpit-ups-wol/install.log

Result: SUCCESS
============================================================
```

## 32. Installation Acceptance Criteria

A release SHALL be considered installation-ready only when a clean supported OS can run:

```bash
sudo ./install.sh --silent
```

and, without additional manual package installation:

1. Cockpit is accessible.
2. `cockpit-ups-wol` appears in Cockpit.
3. NUT is installed and configured.
4. Synology compatibility can be enabled.
5. `cockpit-ups-wol-agent` starts automatically.
6. `wolctl` works.
7. recovery defaults to 80%.
8. persistent state directories exist.
9. all services survive reboot.
10. rerunning the installer is safe.
11. the installation log clearly records all operations.
