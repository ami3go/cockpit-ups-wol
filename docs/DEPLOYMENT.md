# Deployment and Power Topology

**Status:** Normative v0.1 deployment requirements

## 1. Controller power source

The controller SBC SHALL be powered from a **battery-backed UPS output** unless it has another independently backed power source with equivalent reliability.

A surge-only UPS outlet is not sufficient.

Preferred local-UPS topology:

```text
                    UPS
          ┌──────────┼──────────────┐
          │          │              │
          ▼          ▼              ▼
     Controller    Switch/       Managed
        SBC        router         devices
          │
          └──── USB/serial/network UPS data
```

The controller needs both:

```text
UPS-backed electrical power
UPS status/control data path
```

## 2. Why this is mandatory

The controller must remain available while it:

- observes outage progression
- records pre-outage host state
- performs ordered pre-FSD shutdown
- coordinates NUT shutdown ownership
- persists transaction state
- performs health/reconciliation logic

A controller powered only from utility mains would disappear at the start of the event it is supposed to manage.

## 3. Network path must remain available

Powering only the SBC is insufficient if its required LAN path fails.

At least the infrastructure required to reach protected devices during shutdown SHALL remain available long enough for shutdown coordination.

Typical requirement:

```text
controller SBC   UPS-backed
core switch      UPS-backed
router/firewall  UPS-backed when routing/VLAN services are required locally
Wi-Fi AP         UPS-backed only if protected clients depend on Wi-Fi control path
```

Pure Layer-2 devices on the same VLAN may not require a router if the switch remains operational.

## 4. Managed loads

Heavy loads should shut down before infrastructure/controller loads.

Typical order:

```text
workstations / application hosts
NAS / storage
hypervisors
NUT secondaries as required
controller SBC / NUT primary LAST
```

Exact behavior for NUT secondaries is defined in `docs/NUT_SHUTDOWN_MODEL.md`.

## 5. Controller automatic power-on

For unattended recovery, controller hardware SHALL boot automatically whenever backed output power is re-applied.

SBCs typically boot automatically on power application.

PC-class hardware SHALL use firmware behavior equivalent to:

```text
Restore on AC Power Loss = Power On
```

If automatic restart cannot be verified, the deployment SHALL NOT be considered fully unattended-recovery capable.

## 6. UPS output-return capability

The installer/arming flow classifies UPS output behavior as:

```text
POWER_CYCLE_VERIFIED
POWER_CYCLE_UNVERIFIED
MONITOR_ONLY
```

A fully automatic controller power-off → controller boot recovery path requires either:

- verified UPS output shutdown/return behavior, or
- another verified automatic controller restart mechanism.

## 7. Local USB UPS deployment

Preferred first-release topology:

```text
UPS battery-backed AC outlet → SBC power supply
UPS USB/serial              → SBC
SBC                         → Ethernet switch
SBC NUT                     → Synology / Linux secondaries
```

The UPS USB connection must remain physically stable and accessible to the NUT driver.

## 8. Remote NUT deployment

When UPS status comes from another server:

- the remote NUT server and network path become dependencies
- the local controller is not automatically NUT primary
- loss of remote NUT reachability becomes `UNKNOWN`, not `OL`
- final UPS output-control ownership remains with the designated remote primary unless explicitly redesigned

## 9. Switch/router recovery behavior

Network infrastructure may be modeled as:

```text
auto-power
wait-only
wol
```

Typical unmanaged switch:

```text
startup: auto-power
```

The controller waits until the switch path is usable before waking dependent hosts.

A router that boots slowly may be `wait-only` with a TCP/ping readiness check.

## 10. Static addressing

The controller SHOULD use a stable LAN address through:

```text
DHCP reservation
or
static IP configuration
```

The installer SHALL recommend this but SHALL NOT rewrite working network configuration without explicit authorization.

## 11. VLANs and routed WoL

WoL is normally local-broadcast based.

Per-host settings support:

```text
interface
broadcast address
UDP port
```

Routed/directed broadcast WoL SHALL not be assumed. Deployments that require cross-VLAN WoL must verify network-device support and security policy.

## 12. Physical deployment checklist

Before `armed` mode is allowed, the UI/TUI should confirm or warn on:

```text
[ ] controller plugged into UPS battery-backed outlet
[ ] controller auto-boots when power is applied
[ ] required network switch remains UPS-backed
[ ] router/VLAN path remains available where required
[ ] UPS data link is functional
[ ] NUT role/authority is validated
[ ] output power-cycle capability classification is known
[ ] Synology/NUT clients can reach TCP 3493
[ ] WoL broadcast/interface configuration tested
[ ] dry-run shutdown/recovery plan reviewed
```

Hardware facts that cannot be detected automatically may require administrator confirmation.

## 13. Recommended controller characteristics

For v0.1:

```text
Linux/systemd capable
Ethernet preferred
USB host when local USB UPS is used
persistent storage with reliable fsync semantics
amd64, arm64 or riscv64
512 MiB RAM minimum target for full stack
1 GiB RAM recommended
```

Very constrained platforms may later use headless mode without Cockpit.

## 14. Failure examples

### SBC backed, switch not backed

```text
utility fails
SBC remains alive
switch dies
SBC cannot reach NAS/Proxmox
```

This deployment is not suitable for network-coordinated shutdown unless the devices are independently protected through NUT/local policy.

### UPS monitor-only, SBC shuts down

If the UPS never removes/re-applies its output, a powered-off SBC may remain off after utility returns. Therefore controller shutdown is prohibited unless an alternate verified wake mechanism exists.

### Router unavailable but local VLAN still works

The recovery network gate should test the dependencies actually required for target hosts instead of blindly requiring Internet connectivity.

## 15. Acceptance tests

Deployment acceptance should include:

```text
pull utility power with SBC/switch on UPS
verify management path survives
verify local NUT secondaries remain reachable
restore utility and verify controller boot behavior after a full output cycle
verify repeated power bounce during controller boot
verify network dependency readiness delays host recovery
verify WoL on every required subnet/VLAN
```
