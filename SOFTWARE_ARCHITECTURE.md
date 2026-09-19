# cockpit-ups-wol — Software Architecture

**Architecture version:** 0.2  
**Status:** Implementation baseline

## 1. Purpose

`cockpit-ups-wol` is a lightweight homelab power-management system built around Network UPS Tools (NUT), Cockpit and Wake-on-LAN.

Its core functions are:

- monitor a locally or remotely connected UPS
- provide NUT service to network devices
- support Synology NAS as a NUT client
- safely shut down network devices during an extended power outage
- shut down the controller itself last when required
- persist outage/recovery state
- recover automatically after utility power returns
- wait until the UPS has recharged to a configurable level, default **80%**
- wake devices in a configurable order using Wake-on-LAN
- provide management and configuration through Cockpit

The system is intended primarily for protected home and small-lab networks.

## 2. Core Design Principle

Cockpit is the management interface, not the critical power-control engine.

Critical power protection SHALL continue to function when:

- no browser is open
- Cockpit is stopped
- the Cockpit extension fails
- the controller reboots during an outage
- the Cockpit UI is being upgraded

The critical path is:

```text
UPS
 │
 ▼
NUT
 │
 ▼
cockpit-ups-wol-agent
 │
 ├── shutdown policy
 ├── recovery policy
 ├── persistent state
 └── Wake-on-LAN
```

Cockpit configures and observes this system.

## 3. High-Level Architecture

```text
                         Browser
                            │
                            ▼
                         Cockpit
                            │
                            ▼
                  cockpit-ups-wol UI
                            │
                 configuration/status
                            │
                            ▼
              cockpit-ups-wol-agent
                 persistent service
                 │       │        │
                 │       │        └──── wolctl
                 │       │                 │
                 │       │                 ▼
                 │       │            LAN devices
                 │       │
                 │       └──── persistent state
                 │
                 ▼
                NUT
                 │
        ┌────────┴─────────┐
        │                  │
        ▼                  ▼
     Local UPS        Remote NUT server
        │
        USB


Network NUT clients
        │
        ├── Synology NAS
        ├── Proxmox
        ├── Linux servers
        └── other NUT clients
```

## 4. Main Components

### 4.1 NUT

NUT remains responsible for:

- UPS hardware communication
- UPS variables
- UPS status
- client/server UPS communication
- standard NUT shutdown coordination
- UPS instant commands
- writable UPS variables

Version 0.x SHALL use standard NUT tools where practical:

```text
upsc
upsrw
upscmd
upsmon
upsd
```

The project SHALL NOT reimplement UPS drivers.

### 4.2 cockpit-ups-wol-agent

The agent is a core component.

Systemd service:

```text
cockpit-ups-wol-agent.service
```

Responsibilities:

- observe NUT status
- maintain outage state machine
- snapshot host state before shutdown
- initiate managed host shutdown
- coordinate shutdown ordering
- persist recovery information
- resume after reboot
- detect stable utility power
- wait for UPS recharge threshold
- wait for network readiness
- wake hosts in configured order
- record all decisions to journald

The agent SHALL remain lightweight and suitable for ARM64, RISC-V64 and AMD64.

### 4.3 wolctl

`wolctl` is a small helper responsible for:

- building Wake-on-LAN magic packets
- validating MAC addresses
- selecting interface/broadcast address
- sending WoL packets
- optionally checking host state

Example:

```bash
wolctl wake server
wolctl status server
wolctl status --all
```

Implementation SHOULD use Go so release binaries can be provided without runtime dependencies.

### 4.4 Cockpit extension

Cockpit provides:

- UPS status
- service status
- outage/recovery state
- battery level
- runtime
- UPS measurements
- host management
- Wake buttons
- shutdown/recovery configuration
- event/log viewer
- NUT configuration
- Synology compatibility configuration
- battery test and permitted UPS commands

Cockpit SHALL NOT be required for automatic recovery.

## 5. Power State Machine

The agent SHALL implement an explicit state machine.

```text
                 ┌─────────────┐
                 │   NORMAL    │
                 └──────┬──────┘
                        │ OB
                        ▼
                ┌───────────────┐
                │  ON_BATTERY   │
                └───┬───────┬───┘
                    │       │
          power back│       │shutdown trigger
                    │       ▼
                    │  ┌──────────────────────┐
                    │  │ SHUTDOWN_IN_PROGRESS │
                    │  └──────────┬───────────┘
                    │             ▼
                    │    ┌─────────────────┐
                    │    │ WAITING_FOR_AC  │
                    │    └────────┬────────┘
                    │             │ AC restored
                    │             ▼
                    │    ┌─────────────────┐
                    └───►│  RECOVERY_WAIT  │
                         └────────┬────────┘
                                  │ requirements met
                                  ▼
                         ┌─────────────────┐
                         │ RESTORE_HOSTS   │
                         └────────┬────────┘
                                  ▼
                                NORMAL
```

State changes SHALL be persisted.

## 6. Outage Detection

The normal NUT status is:

```text
OL
```

Utility power failure typically produces:

```text
OB
```

The agent SHALL support a configurable grace period before beginning shutdown logic.

Example:

```yaml
outage:
  grace_period_seconds: 120
```

This prevents short power interruptions from causing unnecessary shutdowns.

Shutdown conditions MAY include:

- time on battery
- `LB` low-battery state
- battery percentage
- remaining runtime
- explicit NUT FSD condition

## 7. Host State Snapshot

Before managed shutdown begins, the agent SHALL record which managed hosts are online.

Example:

```json
{
  "outage_id": "2026-09-19T15:23:11Z",
  "hosts": {
    "nas": true,
    "proxmox": true,
    "desktop": false,
    "workstation": true
  }
}
```

Persistent state location:

```text
/var/lib/cockpit-ups-wol/state.json
```

Writes SHALL be atomic.

The recovery process SHOULD normally wake only hosts that were running before the outage.

Per-host override:

```yaml
restore_policy: previous-state
```

Other supported policies:

```text
previous-state
always
never
```

## 8. Shutdown Methods

Each host SHALL define how shutdown is managed.

Supported methods:

```text
nut
ssh
command
none
```

### nut

Preferred where the device supports NUT.

Examples:

- Synology NAS
- Linux servers
- Proxmox hosts using NUT

The NUT client handles its own safe shutdown.

### ssh

For hosts requiring a remote shutdown command.

Example:

```yaml
shutdown:
  method: ssh
  user: powerctl
  command: sudo systemctl poweroff
```

Authentication SHOULD use dedicated SSH keys.

### command

Allows a controlled local helper executable.

Arbitrary shell interpolation from Cockpit input SHALL NOT be permitted.

### none

The controller observes the device but does not shut it down.

## 9. Synology NAS

Synology NAS SHALL be a first-class supported NUT client.

The compatibility preset SHALL use:

```text
UPS name: ups
NUT port: 3493
monitor account: monuser
compatibility password: secret
```

The compatibility account SHALL be monitoring-only.

On current NUT versions:

```ini
[monuser]
    password = secret
    upsmon secondary
```

On older NUT versions the installer MAY use the legacy equivalent:

```ini
upsmon slave
```

The Synology compatibility account MUST NOT receive:

```text
actions = SET
actions = FSD
instcmds = ALL
```

or other administrative permissions.

Synology shutdown SHALL normally be performed by Synology's own NUT client.

The agent SHALL NOT independently SSH-shutdown the NAS when its shutdown method is:

```text
nut
```

## 10. Shutdown Ordering

Managed devices SHALL support shutdown priorities.

Example:

```yaml
shutdown:
  priority: 20
  timeout_seconds: 120
```

Lower priority values shut down first.

Example:

```text
Desktop              priority 10
NAS                   priority 20
Application server    priority 30
Proxmox               priority 40
UPS controller        priority 100
```

The controller SHALL shut down last.

The agent SHALL wait for configured timeout/confirmation before progressing where practical.

## 11. Controller Shutdown

The controller SBC SHALL remain operational as long as practical so it can coordinate other systems.

If the UPS reaches the controller shutdown condition:

```text
network clients shutdown
        ↓
managed hosts shutdown
        ↓
state persisted
        ↓
controller powers off LAST
```

The system SHALL persist enough state before shutdown to continue recovery after boot.

## 12. Recovery Conditions

Automatic recovery SHALL be configurable.

Default:

```yaml
recovery:
  enabled: true
  battery_charge_min: 80
  utility_stable_seconds: 120
  network_wait_seconds: 300
```

Recovery SHALL require:

1. utility power restored
2. UPS no longer reporting `OB`
3. utility power stable for configured period
4. network available
5. UPS battery sufficiently recovered
6. no unresolved safety/error state

Default minimum battery level:

```text
80%
```

## 13. Battery-Recovery Fallback

Not every UPS reports:

```text
battery.charge
```

The recovery policy SHALL therefore support:

```text
percentage
runtime
time
manual
```

Preferred hierarchy:

```text
battery.charge available
        │
        └── wait for >= configured percentage

otherwise battery.runtime available
        │
        └── use configured minimum runtime

otherwise
        │
        └── wait configured recharge time

unsupported/uncertain
        │
        └── manual recovery
```

The agent SHALL NOT silently guess an unsafe battery threshold.

Cockpit SHALL show which recovery method is active.

## 14. Network Readiness

Before Wake-on-LAN recovery starts, the system SHALL verify network readiness.

Possible checks:

- Ethernet carrier
- interface has address
- route available
- gateway reachable
- configured management target reachable

Failure SHALL cause retries until `network_wait_seconds` expires.

It SHALL NOT immediately abandon the recovery process because DHCP or a switch is still starting.

## 15. Wake Ordering

Wake configuration SHALL support:

```yaml
wake:
  enabled: true
  priority: 20
  delay_after_previous_seconds: 30
```

Lower priority wakes first.

Example:

```text
Network infrastructure
        ↓
NAS
        ↓
Proxmox
        ↓
Application servers
        ↓
Workstations
```

This avoids a simultaneous UPS load surge.

## 16. Multi-Network Wake-on-LAN

Per-host configuration SHALL support:

```yaml
wake:
  mac: "AA:BB:CC:DD:EE:FF"
  interface: eth0
  broadcast: 192.168.1.255
  port: 9
```

This enables:

- multiple Ethernet interfaces
- VLANs
- multiple subnets
- directed broadcasts where supported

WoL across routed networks SHALL not be assumed automatically.

## 17. Host Status

Host detection SHOULD support:

```text
auto
ping
tcp
arp
none
```

Example:

```yaml
status:
  method: tcp
  port: 22
  timeout_ms: 1000
```

TCP checking is useful where ICMP is blocked.

## 18. NUT Network Policy

Default mode:

```text
trusted-lan
```

The project assumes operation behind a home/router firewall.

NUT SHALL be reachable from the LAN on TCP 3493.

Default server listener MAY use:

```ini
LISTEN 0.0.0.0 3493
```

and optionally IPv6.

Optional security mode:

```text
restricted
```

may restrict NUT access by host/subnet using the system firewall.

Restriction SHALL NOT be enabled by default.

NUT MUST NOT be intentionally exposed to the public Internet.

## 19. NUT Privilege Separation

Network UPS clients SHALL use monitoring-only credentials.

Administrative commands SHALL be separate.

Example architecture:

```text
Synology / NUT clients
       │
       └── monitor-only account

Cockpit administrative operation
       │
       └── local privileged helper
```

UPS administrative commands such as:

```text
load.off
shutdown.return
FSD
upsrw SET
```

SHALL require Cockpit authorization and explicit confirmation.

## 20. Persistent Configuration

Main directory:

```text
/etc/cockpit-ups-wol/
```

Suggested files:

```text
config.yaml
hosts.yaml
```

Configuration SHALL contain a schema version:

```yaml
config_version: 1
```

Future upgrades SHALL provide migrations when the configuration schema changes.

## 21. Example Configuration

```yaml
config_version: 1

nut:
  server: localhost
  port: 3493

  network:
    mode: trusted-lan

  synology_compatibility: true

outage:
  grace_period_seconds: 120

recovery:
  enabled: true
  battery_charge_min: 80
  utility_stable_seconds: 120
  network_wait_seconds: 300

hosts:

  - id: synology
    name: Synology NAS
    ip: 192.168.1.20

    status:
      method: tcp
      port: 5000

    shutdown:
      method: nut
      priority: 20

    wake:
      enabled: true
      mac: "AA:BB:CC:DD:EE:FF"
      priority: 20
      delay_after_previous_seconds: 30

    restore_policy: previous-state

  - id: workstation
    name: Workstation
    ip: 192.168.1.30

    shutdown:
      method: ssh
      priority: 10
      timeout_seconds: 90

    wake:
      enabled: true
      mac: "11:22:33:44:55:66"
      broadcast: 192.168.1.255
      priority: 30

    restore_policy: previous-state
```

## 22. systemd Services

Expected runtime:

```text
cockpit.socket
nut-driver@*.service
nut-server.service
nut-monitor.service
cockpit-ups-wol-agent.service
```

The agent SHALL declare appropriate dependencies on network and NUT services.

Restart policy SHOULD allow recovery from transient failures without creating restart loops.

## 23. Logging

Runtime logging SHALL use journald.

Example:

```bash
journalctl -u cockpit-ups-wol-agent
```

Events SHALL include:

- utility power lost
- utility power restored
- host state snapshot
- shutdown initiated
- shutdown success/failure
- controller shutdown
- recovery resumed after boot
- battery recovery progress
- network readiness
- WoL packet sent
- host recovered
- timeout/failure
- configuration error

A separate database is not required.

## 24. Failure Handling

The system SHALL distinguish:

```text
UPS disconnected
NUT driver unavailable
upsd unavailable
network unavailable
host unreachable
shutdown command failed
battery percentage unavailable
WoL failed
invalid configuration
```

Failure of one host SHALL not necessarily prevent shutdown/recovery of other hosts.

Safety-critical errors SHALL be visible in Cockpit and journald.

## 25. Cockpit UI

Primary pages:

```text
Overview
UPS
Devices
Automation
Settings
Logs
```

### Overview

Display:

- UPS state
- battery percentage
- runtime
- load
- utility status
- agent state
- recovery threshold
- managed hosts

### Automation

Configure:

- outage grace period
- shutdown conditions
- shutdown priority
- recovery enabled
- minimum battery charge
- stable utility delay
- recovery order
- wake delay

### UPS

Provide:

- measurements
- NUT services
- supported instant commands
- writable variables
- battery tests

Dangerous commands require confirmation.

## 26. Simulation and Testing

The agent SHALL provide a safe simulation/test mechanism.

Examples:

```bash
cockpit-ups-wol-agent --simulate on-battery
cockpit-ups-wol-agent --simulate low-battery
cockpit-ups-wol-agent --simulate power-restored
```

Simulation SHALL NOT invoke real shutdown or WoL operations unless explicitly requested.

Automated tests SHALL cover:

- OL → OB
- short outage
- prolonged outage
- low battery
- failed host shutdown
- controller reboot
- AC restore
- unstable AC
- battery below threshold
- battery threshold reached
- missing battery.charge
- delayed network startup
- ordered WoL
- previously-off host remains off
- Synology NUT client behavior

## 27. Resource Targets

Primary CPU architectures:

```text
amd64
arm64
riscv64
```

Target systems include:

- Raspberry Pi
- NanoPi
- Radxa
- Orange Pi
- Milk-V Duo 256M / Duo S
- x86 mini PCs

The original 64 MB Milk-V Duo is not a target for the full Cockpit stack.

## 28. Repository Layout

```text
cockpit-ups-wol/
├── README.md
├── SOFTWARE_ARCHITECTURE.md
├── LICENSE
├── CHANGELOG.md
│
├── src/
│   ├── manifest.json
│   ├── app/
│   ├── components/
│   └── api/
│
├── agent/
│   ├── state/
│   ├── policy/
│   ├── nut/
│   └── host/
│
├── wolctl/
│   ├── cmd/
│   └── magicpacket/
│
├── config/
│   ├── config.yaml.example
│   └── hosts.yaml.example
│
├── scripts/
│   ├── install.sh
│   └── lib/
│
├── docs/
│   ├── INSTALLATION_REQUIREMENTS.md
│   ├── RELATED_PROJECTS.md
│   ├── NUT.md
│   ├── SYNOLOGY.md
│   └── SECURITY.md
│
├── tests/
│   ├── unit/
│   ├── integration/
│   └── simulation/
│
└── .github/
    └── workflows/
```

## 29. Architectural Acceptance Criteria

The architecture is considered functional when the following scenario succeeds without a browser open:

```text
1. UPS reports utility failure.
2. Short-outage grace timer expires.
3. Running host state is recorded.
4. Managed hosts shut down in configured order.
5. Synology safely shuts down through NUT.
6. Controller shuts down last if required.
7. Utility power returns.
8. Controller boots.
9. Agent restores persisted outage state.
10. Utility power remains stable.
11. Network becomes available.
12. UPS battery reaches at least 80%.
13. Previously-running hosts are awakened in configured order.
14. Previously-off hosts remain off.
15. System returns to NORMAL.
```

Cockpit SHALL be able to inspect the entire process, but SHALL not be required for it to complete.
