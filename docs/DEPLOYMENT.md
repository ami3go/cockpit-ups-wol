# Deployment and Power Topology

**Status:** Normative v0.1 deployment requirements

## 1. Controller power source

The controller SBC SHALL be powered from a **battery-backed UPS output** unless it has an independently backed source with equivalent reliability. A surge-only UPS outlet is not sufficient.

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

The controller needs both UPS-backed electrical power and a trustworthy UPS status/control data path.

## 2. Why controller/network backing is mandatory

During an outage the controller must remain available while it records pre-outage state, evaluates policy, performs ordered direct shutdown, coordinates NUT FSD ownership, persists the transaction and reconciles faults/reboots.

Powering only the SBC is insufficient when its required LAN path disappears. At minimum, the switching/routing infrastructure needed to reach protected devices during shutdown SHALL remain powered long enough for coordination.

Typical deployment:

```text
controller SBC   UPS-backed
core switch      UPS-backed
router/firewall  UPS-backed when routing/VLAN services are needed locally
Wi-Fi AP         UPS-backed only when protected clients depend on Wi-Fi control
```

Pure same-VLAN Layer-2 operation may not require a router when the switch path remains available.

## 3. Shutdown ordering

Typical order:

```text
workstations / application hosts
NAS / storage
hypervisors
NUT secondary group
controller SBC / NUT primary LAST
```

Exact NUT secondary semantics are defined in `docs/NUT_SHUTDOWN_MODEL.md`. The controller must not present a false exact ordering among secondaries that respond to the same FSD wave.

## 4. Controller automatic power-on

For unattended recovery, controller hardware SHALL boot automatically whenever backed output power is re-applied.

SBCs normally boot on applied power. PC-class systems should use firmware behavior equivalent to:

```text
Restore on AC Power Loss = Power On
```

If automatic restart cannot be verified, the deployment is not fully unattended-recovery capable.

## 5. UPS output-return capability

The deployment records one of:

```text
POWER_CYCLE_VERIFIED
POWER_CYCLE_UNVERIFIED
MONITOR_ONLY
```

A fully automatic controller power-off -> controller boot path requires either verified UPS output shutdown/return behavior or another verified automatic restart mechanism.

## 6. Local USB UPS deployment

Preferred v0.1 topology:

```text
UPS battery-backed AC outlet -> SBC power supply
UPS USB/serial              -> SBC
SBC                         -> Ethernet switch
SBC NUT                     -> Synology / Linux secondaries
```

The UPS data connection must remain physically stable and accessible to the NUT driver.

## 7. Remote NUT deployment

When UPS state comes from another server:

- the remote NUT server and network path are dependencies;
- this controller is not automatically NUT primary;
- loss of remote NUT reachability becomes `UNKNOWN`, not `OL`;
- FSD/final output ownership remains with the validated remote primary unless deliberately redesigned.

## 8. Network dependency recovery

Accepted armed-v0.1 dependency startup modes are:

```text
auto-power
wait-only
```

Examples:

- unmanaged switch: `startup: auto-power`;
- slow router/firewall: `startup: wait-only` with TCP/ping readiness.

The schema also contains dependency `startup: wol`, but **dependency Wake-on-LAN is not an accepted armed-v0.1 behavior**. It fails closed until dependency actions have durable request/retry/reconciliation state. Managed-host WoL remains supported separately.

The agent waits for required dependency readiness before restoring dependent managed hosts.

## 9. Static addressing and VLANs

The controller SHOULD use stable addressing through DHCP reservation or a deliberately configured static address. The installer recommends this but does not rewrite working network configuration without explicit authorization.

Managed-host WoL is normally local-broadcast based. Per-host configuration supports interface, IPv4 broadcast address and UDP port. Directed/routed broadcast is not assumed; cross-VLAN deployments must verify network-device support and security policy.

## 10. Physical deployment checklist

Before `armed` mode:

```text
[ ] controller uses a UPS battery-backed output
[ ] controller automatically boots when backed power returns
[ ] required core switch remains backed
[ ] router/VLAN path remains backed where required
[ ] UPS data link is functional
[ ] NUT role/authority is validated
[ ] UPS output-cycle classification is known
[ ] Synology/NUT clients can reach TCP 3493 where used
[ ] every managed-host WoL subnet/broadcast is tested
[ ] shutdown/recovery plan reviewed in dry-run
```

Hardware facts that cannot be detected automatically require administrator confirmation and, for the release gate, retained test evidence.

## 11. Recommended controller characteristics

```text
Linux/systemd capable
Ethernet preferred
USB host for local USB UPS
persistent storage with reliable fsync semantics
amd64, arm64 or riscv64
512 MiB RAM minimum target for full stack
1 GiB RAM recommended
```

Real installation requires systemd to be the active PID-1 system manager. Container/chroot environments without a functional systemd manager are rejected before installation transaction state is created.

## 12. Failure examples

### SBC backed, switch not backed

The SBC survives but cannot reach managed hosts. This is not suitable for network-coordinated shutdown unless the targets are independently protected.

### Monitor-only UPS, controller powers off

If UPS output never cycles, a powered-off controller may remain off after utility returns. Automatic controller shutdown therefore requires verified output return or another verified restart mechanism.

### Router unavailable, local VLAN still usable

Recovery readiness should test the dependencies actually required by targets, not Internet connectivity by default.

## 13. Acceptance

Physical acceptance should include utility removal with controller/network on backed outputs, real OB observation, shutdown coordination, controller-last/FSD behavior, output-return/automatic-boot behavior where applicable, repeated power bounce, dependency readiness gating, managed-host WoL and retained evidence.

The authoritative physical procedure is `docs/HARDWARE_ACCEPTANCE.md`.
