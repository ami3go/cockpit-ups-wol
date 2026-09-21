package policy

import (
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

func TestUnsafeUPSAfterRecoveryCommitClearsDurableRecoveryFlag(t *testing.T) {
	for _, powerState := range []state.PowerState{state.RecoveryStarted, state.RestoreHosts} {
		t.Run(string(powerState), func(t *testing.T) {
			st := state.New("outage-1", "cfg")
			st.PowerState = powerState
			st.ShutdownCommitted = true
			st.RecoveryStarted = true

			e := &Engine{cfg: Config{}, st: st}
			decision, err := e.Step(time.Now(), Inputs{
				UPS:          nut.Status{Utility: nut.UtilityOnBattery},
				NetworkReady: true,
				HealthSafe:   true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if decision.Action != ActionStopRecovery {
				t.Fatalf("action=%s want %s", decision.Action, ActionStopRecovery)
			}
			if e.State().RecoveryStarted {
				t.Fatal("recovery_started remained true after recovery was aborted by unsafe UPS state")
			}
		})
	}
}

func TestBootWithStaleRecoveryStartedReentersAllRecoveryGates(t *testing.T) {
	chargeMin := 80.0
	st := state.New("outage-1", "cfg")
	st.PowerState = state.RestoreHosts
	st.ShutdownCommitted = true
	st.RecoveryStarted = true

	e := New(Config{
		UtilityStable:     120 * time.Second,
		RecoveryChargeMin: &chargeMin,
	}, st)

	now := time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC)
	highCharge := 90.0
	inputs := Inputs{
		UPS:          nut.Status{Utility: nut.UtilityOnline, ChargePercent: &highCharge},
		NetworkReady: true,
		HealthSafe:   true,
	}

	decision, err := e.Step(now, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionNone || e.State().PowerState != state.RecoveryWait {
		t.Fatalf("boot resume bypassed recovery wait: decision=%+v state=%+v", decision, e.State())
	}
	if e.State().RecoveryStarted {
		t.Fatal("stale recovery_started flag was retained after boot reconciliation")
	}

	decision, err = e.Step(now.Add(119*time.Second), inputs)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionNone || e.State().PowerState != state.RecoveryWait {
		t.Fatalf("utility stability gate was bypassed: decision=%+v state=%+v", decision, e.State())
	}

	lowCharge := 20.0
	inputs.UPS.ChargePercent = &lowCharge
	decision, err = e.Step(now.Add(121*time.Second), inputs)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionNone || e.State().PowerState != state.RecoveryWait {
		t.Fatalf("battery recharge gate was bypassed: decision=%+v state=%+v", decision, e.State())
	}

	inputs.UPS.ChargePercent = &highCharge
	decision, err = e.Step(now.Add(122*time.Second), inputs)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionCommitRecovery || e.State().PowerState != state.RecoveryStarted || !e.State().RecoveryStarted {
		t.Fatalf("recovery did not recommit after all gates passed: decision=%+v state=%+v", decision, e.State())
	}
}
