package policy

import (
	"errors"
	"fmt"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

const outageCheckpointInterval = 30 * time.Second
const preCommitOnlineConfirmations = 2

type Config struct {
	GracePeriod            time.Duration
	MaxOnBattery           time.Duration
	CriticalCharge         *float64
	CriticalRuntime        time.Duration
	CommunicationLossGrace time.Duration
	RecoveryEnabled        *bool
	UtilityStable          time.Duration
	RecoveryChargeMin      *float64
	RecoveryRuntimeMin     time.Duration
	RecoveryRechargeTime   time.Duration
	RecoveryNetworkWait    time.Duration
}

type Inputs struct {
	UPS          nut.Status
	NetworkReady bool
	HealthSafe   bool
}

type Action string

const (
	ActionNone           Action = "none"
	ActionCommitShutdown Action = "commit_shutdown"
	ActionStopRecovery   Action = "stop_recovery"
	ActionCommitRecovery Action = "commit_recovery"
	ActionStartRestore   Action = "start_restore"
)

type Decision struct {
	Changed bool
	Action  Action
	Reason  string
}

type Engine struct {
	cfg                  Config
	st                   state.State
	resumeState          state.PowerState
	onBatterySince       time.Time
	onlineSince          time.Time
	unknownSince         time.Time
	networkWaitSince     time.Time
	lastOutageCheckpoint time.Time
	onlineConfirmations  int
}

func New(cfg Config, persisted state.State) *Engine {
	if cfg.UtilityStable <= 0 {
		cfg.UtilityStable = 120 * time.Second
	}
	if persisted.StateVersion == 0 {
		persisted.StateVersion = state.Version
	}
	resume := persisted.PowerState
	persisted.PowerState = state.BootReconcile
	return &Engine{cfg: cfg, st: persisted, resumeState: resume}
}

func (e *Engine) State() state.State { return e.st }

func (e *Engine) Step(now time.Time, in Inputs) (Decision, error) {
	if e.st.PowerState == state.FailedSafe {
		return Decision{Action: ActionNone, Reason: "FAILED_SAFE"}, nil
	}

	if e.st.PowerState == state.BootReconcile {
		return e.reconcileBoot(now, in)
	}

	if in.UPS.FSD && !e.st.ShutdownCommitted {
		return e.commitShutdown("FSD observed"), nil
	}

	switch e.st.PowerState {
	case state.Normal:
		if in.UPS.Utility == nut.UtilityOnBattery {
			e.onBatterySince = now
			e.lastOutageCheckpoint = now
			e.unknownSince = time.Time{}
			e.onlineConfirmations = 0
			e.st.OutageElapsedSeconds = 0
			e.st.PowerState = state.OnBattery
			return Decision{Changed: true, Action: ActionNone, Reason: "UPS on battery"}, nil
		}
		return Decision{Action: ActionNone}, nil

	case state.OnBattery:
		return e.stepOnBattery(now, in)

	case state.ShutdownCommitted:
		e.st.PowerState = state.ShutdownInProgress
		return Decision{Changed: true, Action: ActionNone, Reason: "shutdown transaction committed"}, nil

	case state.ShutdownInProgress:
		return Decision{Action: ActionNone}, nil

	case state.WaitingForAC:
		if in.UPS.Utility == nut.UtilityOnline {
			if !recoveryEnabled(e.cfg) {
				return Decision{Action: ActionNone, Reason: "automatic recovery disabled"}, nil
			}
			e.onlineSince = now
			e.networkWaitSince = time.Time{}
			e.st.PowerState = state.RecoveryWait
			return Decision{Changed: true, Action: ActionNone, Reason: "utility restored; recovery gates pending"}, nil
		}
		return Decision{Action: ActionNone}, nil

	case state.RecoveryWait:
		return e.stepRecoveryWait(now, in)

	case state.RecoveryStarted:
		if !recoveryEnabled(e.cfg) {
			e.st.RecoveryStarted = false
			e.st.PowerState = state.WaitingForAC
			return Decision{Changed: true, Action: ActionStopRecovery, Reason: "automatic recovery disabled"}, nil
		}
		if unsafeUPS(in.UPS) {
			e.onlineSince = time.Time{}
			e.networkWaitSince = time.Time{}
			e.st.RecoveryStarted = false
			e.st.PowerState = state.OnBattery
			return Decision{Changed: true, Action: ActionStopRecovery, Reason: "unsafe UPS state after recovery commit"}, nil
		}
		if !in.HealthSafe {
			return e.failSafe("critical control-stack health became unsafe during recovery"), nil
		}
		if !in.NetworkReady {
			if e.networkWaitSince.IsZero() {
				e.networkWaitSince = now
			}
			if e.cfg.RecoveryNetworkWait > 0 && now.Sub(e.networkWaitSince) >= e.cfg.RecoveryNetworkWait {
				return e.failSafe("network readiness timeout during committed recovery"), nil
			}
			return Decision{Action: ActionNone, Reason: "recovery paused: network not ready"}, nil
		}
		e.networkWaitSince = time.Time{}
		e.st.PowerState = state.RestoreHosts
		return Decision{Changed: true, Action: ActionStartRestore, Reason: "recovery commit durable"}, nil

	case state.RestoreHosts:
		if !recoveryEnabled(e.cfg) {
			e.st.RecoveryStarted = false
			e.st.PowerState = state.WaitingForAC
			return Decision{Changed: true, Action: ActionStopRecovery, Reason: "automatic recovery disabled"}, nil
		}
		if unsafeUPS(in.UPS) {
			e.onlineSince = time.Time{}
			e.networkWaitSince = time.Time{}
			e.st.RecoveryStarted = false
			e.st.PowerState = state.OnBattery
			return Decision{Changed: true, Action: ActionStopRecovery, Reason: "power failed during host restoration"}, nil
		}
		if !in.HealthSafe {
			return e.failSafe("critical control-stack health became unsafe during host restoration"), nil
		}
		if !in.NetworkReady {
			if e.networkWaitSince.IsZero() {
				e.networkWaitSince = now
			}
			// Move back behind the recovery-start gate. The orchestrator only
			// runs host restoration in RESTORE_HOSTS, so no additional WoL is
			// emitted while the network is unavailable.
			e.st.PowerState = state.RecoveryStarted
			return Decision{Changed: true, Action: ActionNone, Reason: "host restoration paused: network not ready"}, nil
		}
		e.networkWaitSince = time.Time{}
		return Decision{Action: ActionNone}, nil

	default:
		return Decision{}, fmt.Errorf("unsupported power state %q", e.st.PowerState)
	}
}

func (e *Engine) MarkShutdownPhaseComplete() Decision {
	if e.st.PowerState != state.ShutdownInProgress && e.st.PowerState != state.ShutdownCommitted {
		return Decision{Action: ActionNone, Reason: "shutdown completion ignored outside shutdown phase"}
	}
	e.st.PowerState = state.WaitingForAC
	return Decision{Changed: true, Action: ActionNone, Reason: "managed shutdown phase complete"}
}

func (e *Engine) MarkRecoveryComplete() Decision {
	if e.st.PowerState != state.RestoreHosts {
		return Decision{Action: ActionNone, Reason: "recovery completion ignored outside restore phase"}
	}
	e.st.PowerState = state.Normal
	e.st.ShutdownCommitted = false
	e.st.RecoveryStarted = false
	e.st.OutageElapsedSeconds = 0
	e.resumeState = state.Normal
	e.onBatterySince = time.Time{}
	e.onlineSince = time.Time{}
	e.unknownSince = time.Time{}
	e.networkWaitSince = time.Time{}
	e.lastOutageCheckpoint = time.Time{}
	e.onlineConfirmations = 0
	return Decision{Changed: true, Action: ActionNone, Reason: "recovery complete"}
}

func (e *Engine) reconcileBoot(now time.Time, in Inputs) (Decision, error) {
	if in.UPS.Utility == nut.UtilityUnknown {
		return Decision{Action: ActionNone, Reason: "waiting for trustworthy UPS state"}, nil
	}

	if e.st.RecoveryStarted {
		if !recoveryEnabled(e.cfg) {
			e.st.RecoveryStarted = false
			e.st.PowerState = state.WaitingForAC
			return Decision{Changed: true, Action: ActionStopRecovery, Reason: "automatic recovery disabled during boot reconciliation"}, nil
		}
		if in.UPS.Utility == nut.UtilityOnBattery || in.UPS.LowBattery || in.UPS.FSD {
			e.st.RecoveryStarted = false
			e.st.PowerState = state.OnBattery
			return Decision{Changed: true, Action: ActionStopRecovery, Reason: "booted into unsafe power after recovery started"}, nil
		}

		// A persisted RecoveryStarted flag records that recovery had been
		// committed before the interruption; it is not durable permission to
		// bypass the recovery safety gates after reboot. Re-enter RecoveryWait
		// and require a fresh stable-utility interval, recharge/runtime gate,
		// network readiness, and health-safe observation before restoring more
		// hosts.
		e.st.RecoveryStarted = false
		e.onlineSince = now
		e.networkWaitSince = time.Time{}
		e.st.PowerState = state.RecoveryWait
		return Decision{Changed: true, Action: ActionNone, Reason: "resumed recovery re-gated after boot"}, nil
	}

	if e.st.ShutdownCommitted && e.resumeState == state.OnBattery && in.UPS.Utility == nut.UtilityOnBattery {
		// Recovery abort persists ON_BATTERY before the orchestrator opens the
		// fresh outage transaction. If power is lost between those two durable
		// writes, complete that transition now so restored hosts become shutdown
		// targets again rather than parking forever in WAITING_FOR_AC.
		e.st.PowerState = state.OnBattery
		return Decision{Changed: true, Action: ActionStopRecovery, Reason: "completing interrupted recovery abort"}, nil
	}

	if e.st.ShutdownCommitted {
		if in.UPS.Utility == nut.UtilityOnline {
			if !recoveryEnabled(e.cfg) {
				e.st.PowerState = state.WaitingForAC
				return Decision{Changed: e.resumeState != state.WaitingForAC, Action: ActionNone, Reason: "committed shutdown found; automatic recovery disabled"}, nil
			}
			e.onlineSince = now
			e.networkWaitSince = time.Time{}
			e.st.PowerState = state.RecoveryWait
			return Decision{Changed: true, Action: ActionNone, Reason: "committed shutdown found; utility online; wait recovery gates"}, nil
		}
		e.st.PowerState = state.WaitingForAC
		return Decision{Changed: true, Action: ActionNone, Reason: "committed shutdown found; wait for utility"}, nil
	}

	if in.UPS.Utility == nut.UtilityOnBattery {
		// Preserve a monotonic lower bound of elapsed outage time across reboot.
		// The durable value is checkpointed while running and never depends on
		// wall-clock correctness. Keep the previous grace protection as a lower
		// bound for older state generations that predate elapsed checkpoints.
		elapsed := time.Duration(e.st.OutageElapsedSeconds) * time.Second
		if e.resumeState == state.OnBattery && elapsed < e.cfg.GracePeriod {
			elapsed = e.cfg.GracePeriod
		}
		e.onBatterySince = now.Add(-elapsed)
		e.lastOutageCheckpoint = now
		e.onlineConfirmations = 0
		e.st.PowerState = state.OnBattery
		return Decision{Changed: true, Action: ActionNone, Reason: "booted while on battery"}, nil
	}

	if in.UPS.Utility == nut.UtilityOnline {
		e.st.OutageElapsedSeconds = 0
		e.st.PowerState = state.Normal
		return Decision{Changed: true, Action: ActionNone, Reason: "boot reconciliation complete"}, nil
	}
	return Decision{}, errors.New("unhandled boot reconciliation input")
}

func (e *Engine) stepOnBattery(now time.Time, in Inputs) (Decision, error) {
	if in.UPS.Utility == nut.UtilityOnline && !e.st.ShutdownCommitted {
		e.unknownSince = time.Time{}
		e.onlineConfirmations++
		if e.onlineConfirmations < preCommitOnlineConfirmations {
			return Decision{Action: ActionNone, Reason: "utility online sample observed; awaiting confirmation"}, nil
		}
		e.onlineConfirmations = 0
		e.onBatterySince = time.Time{}
		e.lastOutageCheckpoint = time.Time{}
		e.st.OutageElapsedSeconds = 0
		e.st.PowerState = state.Normal
		return Decision{Changed: true, Action: ActionNone, Reason: "utility restored before shutdown commit after consecutive confirmation"}, nil
	}
	e.onlineConfirmations = 0

	if in.UPS.LowBattery && in.UPS.Utility != nut.UtilityOnline {
		return e.commitShutdown("low battery observed"), nil
	}
	if in.UPS.Utility == nut.UtilityUnknown {
		if e.unknownSince.IsZero() {
			e.unknownSince = now
		}
		if e.cfg.CommunicationLossGrace > 0 && now.Sub(e.unknownSince) >= e.cfg.CommunicationLossGrace {
			return e.commitShutdown("UPS communication loss grace expired during active outage"), nil
		}
		return Decision{Action: ActionNone, Reason: "UPS state unknown; outage context retained within communication-loss grace"}, nil
	}
	e.unknownSince = time.Time{}
	if in.UPS.Utility != nut.UtilityOnBattery {
		return Decision{Action: ActionNone}, nil
	}

	if e.onBatterySince.IsZero() {
		e.onBatterySince = now
		e.lastOutageCheckpoint = now
	}
	elapsed := now.Sub(e.onBatterySince)
	if elapsed < e.cfg.GracePeriod {
		return e.checkpointOutage(now, elapsed, "outage grace period"), nil
	}
	if e.cfg.CriticalRuntime > 0 && in.UPS.RuntimeSeconds != nil && time.Duration(*in.UPS.RuntimeSeconds)*time.Second <= e.cfg.CriticalRuntime {
		e.recordOutageElapsed(elapsed)
		return e.commitShutdown("critical runtime threshold reached"), nil
	}
	if e.cfg.CriticalCharge != nil && in.UPS.ChargePercent != nil && *in.UPS.ChargePercent <= *e.cfg.CriticalCharge {
		e.recordOutageElapsed(elapsed)
		return e.commitShutdown("critical battery threshold reached"), nil
	}
	if e.cfg.MaxOnBattery > 0 && elapsed >= e.cfg.MaxOnBattery {
		e.recordOutageElapsed(elapsed)
		return e.commitShutdown("maximum time on battery reached"), nil
	}
	return e.checkpointOutage(now, elapsed, "on battery; no shutdown trigger reached"), nil
}

func (e *Engine) checkpointOutage(now time.Time, elapsed time.Duration, reason string) Decision {
	if e.lastOutageCheckpoint.IsZero() {
		e.lastOutageCheckpoint = now
		return Decision{Action: ActionNone, Reason: reason}
	}
	if now.Sub(e.lastOutageCheckpoint) < outageCheckpointInterval {
		return Decision{Action: ActionNone, Reason: reason}
	}
	before := e.st.OutageElapsedSeconds
	e.recordOutageElapsed(elapsed)
	e.lastOutageCheckpoint = now
	if e.st.OutageElapsedSeconds != before {
		return Decision{Changed: true, Action: ActionNone, Reason: reason + "; outage elapsed checkpointed"}
	}
	return Decision{Action: ActionNone, Reason: reason}
}

func (e *Engine) recordOutageElapsed(elapsed time.Duration) {
	seconds := int64(elapsed / time.Second)
	if seconds > e.st.OutageElapsedSeconds {
		e.st.OutageElapsedSeconds = seconds
	}
}

func (e *Engine) commitShutdown(reason string) Decision {
	e.st.ShutdownCommitted = true
	e.st.PowerState = state.ShutdownCommitted
	return Decision{Changed: true, Action: ActionCommitShutdown, Reason: reason}
}

func (e *Engine) stepRecoveryWait(now time.Time, in Inputs) (Decision, error) {
	if !recoveryEnabled(e.cfg) {
		e.onlineSince = time.Time{}
		e.networkWaitSince = time.Time{}
		e.st.PowerState = state.WaitingForAC
		return Decision{Changed: true, Action: ActionNone, Reason: "automatic recovery disabled"}, nil
	}
	if in.UPS.Utility != nut.UtilityOnline || in.UPS.LowBattery || in.UPS.FSD {
		e.onlineSince = time.Time{}
		e.networkWaitSince = time.Time{}
		if in.UPS.Utility == nut.UtilityOnBattery || in.UPS.LowBattery || in.UPS.FSD {
			e.st.PowerState = state.WaitingForAC
			return Decision{Changed: true, Action: ActionNone, Reason: "recovery gate reset by unsafe power"}, nil
		}
		return Decision{Action: ActionNone, Reason: "recovery blocked: UPS state unknown"}, nil
	}
	if e.onlineSince.IsZero() {
		e.onlineSince = now
	}
	stableFor := now.Sub(e.onlineSince)
	if stableFor < e.cfg.UtilityStable {
		e.networkWaitSince = time.Time{}
		return Decision{Action: ActionNone, Reason: "utility stability timer"}, nil
	}
	if !rechargeGate(e.cfg, in.UPS, stableFor) {
		e.networkWaitSince = time.Time{}
		return Decision{Action: ActionNone, Reason: "UPS recharge gate not satisfied"}, nil
	}
	if !in.NetworkReady {
		if e.networkWaitSince.IsZero() {
			e.networkWaitSince = now
		}
		if e.cfg.RecoveryNetworkWait > 0 && now.Sub(e.networkWaitSince) >= e.cfg.RecoveryNetworkWait {
			return e.failSafe("network dependencies did not become ready within configured recovery wait"), nil
		}
		return Decision{Action: ActionNone, Reason: "network not ready"}, nil
	}
	e.networkWaitSince = time.Time{}
	if !in.HealthSafe {
		return Decision{Action: ActionNone, Reason: "control stack health not safe"}, nil
	}
	e.st.RecoveryStarted = true
	e.st.PowerState = state.RecoveryStarted
	return Decision{Changed: true, Action: ActionCommitRecovery, Reason: "all recovery gates satisfied"}, nil
}

func (e *Engine) failSafe(reason string) Decision {
	e.st.PowerState = state.FailedSafe
	e.st.FailedSafeReason = reason
	return Decision{Changed: true, Action: ActionNone, Reason: reason}
}

func recoveryEnabled(cfg Config) bool {
	return cfg.RecoveryEnabled == nil || *cfg.RecoveryEnabled
}

func rechargeGate(cfg Config, st nut.Status, stableFor time.Duration) bool {
	if cfg.RecoveryChargeMin != nil {
		if st.ChargePercent != nil {
			return *st.ChargePercent >= *cfg.RecoveryChargeMin
		}
		if cfg.RecoveryRuntimeMin > 0 && st.RuntimeSeconds != nil {
			return time.Duration(*st.RuntimeSeconds)*time.Second >= cfg.RecoveryRuntimeMin
		}
		if cfg.RecoveryRechargeTime > 0 {
			return stableFor >= cfg.RecoveryRechargeTime
		}
		return false
	}
	if cfg.RecoveryRuntimeMin > 0 {
		if st.RuntimeSeconds != nil {
			return time.Duration(*st.RuntimeSeconds)*time.Second >= cfg.RecoveryRuntimeMin
		}
		if cfg.RecoveryRechargeTime > 0 {
			return stableFor >= cfg.RecoveryRechargeTime
		}
		return false
	}
	if cfg.RecoveryRechargeTime > 0 {
		return stableFor >= cfg.RecoveryRechargeTime
	}
	return false
}

func unsafeUPS(st nut.Status) bool {
	return st.Utility == nut.UtilityOnBattery || st.Utility == nut.UtilityUnknown || st.LowBattery || st.FSD
}
