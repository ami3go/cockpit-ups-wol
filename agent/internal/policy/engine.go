package policy

import (
	"errors"
	"fmt"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

type Config struct {
	GracePeriod          time.Duration
	MaxOnBattery         time.Duration
	CriticalCharge       *float64
	CriticalRuntime      time.Duration
	UtilityStable        time.Duration
	RecoveryChargeMin    *float64
	RecoveryRuntimeMin   time.Duration
	RecoveryRechargeTime time.Duration
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
	cfg            Config
	st             state.State
	resumeState    state.PowerState
	onBatterySince time.Time
	onlineSince    time.Time
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
			e.onlineSince = now
			e.st.PowerState = state.RecoveryWait
			return Decision{Changed: true, Action: ActionNone, Reason: "utility restored; recovery gates pending"}, nil
		}
		return Decision{Action: ActionNone}, nil

	case state.RecoveryWait:
		return e.stepRecoveryWait(now, in)

	case state.RecoveryStarted:
		if unsafeUPS(in.UPS) {
			e.onlineSince = time.Time{}
			e.st.PowerState = state.OnBattery
			return Decision{Changed: true, Action: ActionStopRecovery, Reason: "unsafe UPS state after recovery commit"}, nil
		}
		e.st.PowerState = state.RestoreHosts
		return Decision{Changed: true, Action: ActionStartRestore, Reason: "recovery commit durable"}, nil

	case state.RestoreHosts:
		if unsafeUPS(in.UPS) {
			e.onlineSince = time.Time{}
			e.st.PowerState = state.OnBattery
			return Decision{Changed: true, Action: ActionStopRecovery, Reason: "power failed during host restoration"}, nil
		}
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
	e.resumeState = state.Normal
	e.onBatterySince = time.Time{}
	e.onlineSince = time.Time{}
	return Decision{Changed: true, Action: ActionNone, Reason: "recovery complete"}
}

func (e *Engine) reconcileBoot(now time.Time, in Inputs) (Decision, error) {
	if in.UPS.Utility == nut.UtilityUnknown {
		return Decision{Action: ActionNone, Reason: "waiting for trustworthy UPS state"}, nil
	}

	if e.st.RecoveryStarted {
		if in.UPS.Utility == nut.UtilityOnBattery || in.UPS.LowBattery || in.UPS.FSD {
			e.st.PowerState = state.OnBattery
			return Decision{Changed: true, Action: ActionStopRecovery, Reason: "booted into unsafe power after recovery started"}, nil
		}
		e.st.PowerState = state.RestoreHosts
		return Decision{Changed: true, Action: ActionStartRestore, Reason: "resume committed recovery"}, nil
	}

	if e.st.ShutdownCommitted {
		if in.UPS.Utility == nut.UtilityOnline {
			e.onlineSince = now
			e.st.PowerState = state.RecoveryWait
			return Decision{Changed: true, Action: ActionNone, Reason: "committed shutdown found; utility online; wait recovery gates"}, nil
		}
		e.st.PowerState = state.WaitingForAC
		return Decision{Changed: true, Action: ActionNone, Reason: "committed shutdown found; wait for utility"}, nil
	}

	if in.UPS.Utility == nut.UtilityOnBattery {
		e.onBatterySince = now
		e.st.PowerState = state.OnBattery
		return Decision{Changed: true, Action: ActionNone, Reason: "booted while on battery"}, nil
	}

	if in.UPS.Utility == nut.UtilityOnline {
		e.st.PowerState = state.Normal
		return Decision{Changed: true, Action: ActionNone, Reason: "boot reconciliation complete"}, nil
	}
	return Decision{}, errors.New("unhandled boot reconciliation input")
}

func (e *Engine) stepOnBattery(now time.Time, in Inputs) (Decision, error) {
	if in.UPS.Utility == nut.UtilityOnline && !e.st.ShutdownCommitted {
		e.onBatterySince = time.Time{}
		e.st.PowerState = state.Normal
		return Decision{Changed: true, Action: ActionNone, Reason: "utility restored before shutdown commit"}, nil
	}
	if in.UPS.Utility == nut.UtilityUnknown {
		return Decision{Action: ActionNone, Reason: "UPS state unknown; outage context retained"}, nil
	}
	if in.UPS.Utility != nut.UtilityOnBattery {
		return Decision{Action: ActionNone}, nil
	}
	if in.UPS.FSD {
		return e.commitShutdown("FSD observed"), nil
	}
	if in.UPS.LowBattery {
		return e.commitShutdown("low battery observed"), nil
	}

	if e.onBatterySince.IsZero() {
		e.onBatterySince = now
	}
	elapsed := now.Sub(e.onBatterySince)
	if elapsed < e.cfg.GracePeriod {
		return Decision{Action: ActionNone, Reason: "outage grace period"}, nil
	}
	if e.cfg.CriticalRuntime > 0 && in.UPS.RuntimeSeconds != nil && time.Duration(*in.UPS.RuntimeSeconds)*time.Second <= e.cfg.CriticalRuntime {
		return e.commitShutdown("critical runtime threshold reached"), nil
	}
	if e.cfg.CriticalCharge != nil && in.UPS.ChargePercent != nil && *in.UPS.ChargePercent <= *e.cfg.CriticalCharge {
		return e.commitShutdown("critical battery threshold reached"), nil
	}
	if e.cfg.MaxOnBattery > 0 && elapsed >= e.cfg.MaxOnBattery {
		return e.commitShutdown("maximum time on battery reached"), nil
	}
	return Decision{Action: ActionNone, Reason: "on battery; no shutdown trigger reached"}, nil
}

func (e *Engine) commitShutdown(reason string) Decision {
	e.st.ShutdownCommitted = true
	e.st.PowerState = state.ShutdownCommitted
	return Decision{Changed: true, Action: ActionCommitShutdown, Reason: reason}
}

func (e *Engine) stepRecoveryWait(now time.Time, in Inputs) (Decision, error) {
	if in.UPS.Utility != nut.UtilityOnline || in.UPS.LowBattery || in.UPS.FSD {
		e.onlineSince = time.Time{}
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
		return Decision{Action: ActionNone, Reason: "utility stability timer"}, nil
	}
	if !rechargeGate(e.cfg, in.UPS, stableFor) {
		return Decision{Action: ActionNone, Reason: "UPS recharge gate not satisfied"}, nil
	}
	if !in.NetworkReady {
		return Decision{Action: ActionNone, Reason: "network not ready"}, nil
	}
	if !in.HealthSafe {
		return Decision{Action: ActionNone, Reason: "control stack health not safe"}, nil
	}
	e.st.RecoveryStarted = true
	e.st.PowerState = state.RecoveryStarted
	return Decision{Changed: true, Action: ActionCommitRecovery, Reason: "all recovery gates satisfied"}, nil
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
