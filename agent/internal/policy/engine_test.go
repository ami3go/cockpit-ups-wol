package policy

import (
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

func f64(v float64) *float64 { return &v }
func i64(v int64) *int64     { return &v }

func baseState() state.State {
	st := state.New("outage-test", "cfg-test")
	st.PowerState = state.Normal
	return st
}

func TestBootUnknownBlocksRecovery(t *testing.T) {
	st := baseState()
	st.ShutdownCommitted = true
	st.PowerState = state.WaitingForAC
	e := New(Config{}, st)
	d, err := e.Step(time.Now(), Inputs{UPS: nut.Status{Utility: nut.UtilityUnknown}})
	if err != nil || d.Changed || e.State().PowerState != state.BootReconcile {
		t.Fatalf("decision=%+v state=%s err=%v", d, e.State().PowerState, err)
	}
}

func TestOnBatteryLowCommitsShutdown(t *testing.T) {
	now := time.Now()
	e := New(Config{GracePeriod: time.Hour}, baseState())
	if _, err := e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Step(now.Add(time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}}); err != nil {
		t.Fatal(err)
	}
	d, err := e.Step(now.Add(2*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery, LowBattery: true}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionCommitShutdown || !e.State().ShutdownCommitted || e.State().PowerState != state.ShutdownCommitted {
		t.Fatalf("decision=%+v state=%+v", d, e.State())
	}
}

func TestUtilityRestoredBeforeCommitCancels(t *testing.T) {
	now := time.Now()
	e := New(Config{GracePeriod: 2 * time.Minute}, baseState())
	_, _ = e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}})
	_, _ = e.Step(now.Add(time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}})
	d, err := e.Step(now.Add(10*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}})
	if err != nil || !d.Changed || e.State().PowerState != state.Normal || e.State().ShutdownCommitted {
		t.Fatalf("decision=%+v state=%+v err=%v", d, e.State(), err)
	}
}

func TestCriticalChargeAfterGraceCommits(t *testing.T) {
	now := time.Now()
	crit := 30.0
	e := New(Config{GracePeriod: 2 * time.Second, CriticalCharge: &crit}, baseState())
	_, _ = e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}})
	_, _ = e.Step(now.Add(time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery, ChargePercent: f64(50)}})
	d, err := e.Step(now.Add(4*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery, ChargePercent: f64(30)}})
	if err != nil || d.Action != ActionCommitShutdown {
		t.Fatalf("decision=%+v err=%v", d, err)
	}
}

func TestRecoveryRequiresStableChargeNetworkHealth(t *testing.T) {
	now := time.Now()
	min := 80.0
	st := baseState()
	st.PowerState = state.WaitingForAC
	st.ShutdownCommitted = true
	e := New(Config{UtilityStable: 120 * time.Second, RecoveryChargeMin: &min}, st)
	_, _ = e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: f64(79)}})
	if e.State().PowerState != state.RecoveryWait {
		t.Fatalf("state=%s", e.State().PowerState)
	}
	d, _ := e.Step(now.Add(121*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: f64(79)}, NetworkReady: true, HealthSafe: true})
	if d.Action != ActionNone {
		t.Fatalf("79%% must not start recovery: %+v", d)
	}
	d, _ = e.Step(now.Add(122*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: f64(80)}, NetworkReady: false, HealthSafe: true})
	if d.Action != ActionNone {
		t.Fatal("network gate must block recovery")
	}
	d, _ = e.Step(now.Add(123*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: f64(80)}, NetworkReady: true, HealthSafe: true})
	if d.Action != ActionCommitRecovery || !e.State().RecoveryStarted {
		t.Fatalf("decision=%+v state=%+v", d, e.State())
	}
}

func TestChargeDropAfterRecoveryCommitDoesNotReverse(t *testing.T) {
	min := 80.0
	st := baseState()
	st.PowerState = state.RecoveryStarted
	st.ShutdownCommitted = true
	st.RecoveryStarted = true
	e := New(Config{RecoveryChargeMin: &min}, st)
	d, err := e.Step(time.Now(), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: f64(79)}, NetworkReady: true, HealthSafe: true})
	if err != nil || d.Action != ActionStartRestore || e.State().PowerState != state.RestoreHosts {
		t.Fatalf("decision=%+v state=%s err=%v", d, e.State().PowerState, err)
	}
}

func TestPowerFailDuringRestoreStopsRecovery(t *testing.T) {
	st := baseState()
	st.PowerState = state.RestoreHosts
	st.ShutdownCommitted = true
	st.RecoveryStarted = true
	e := New(Config{}, st)
	// First call reconciles boot and resumes restore.
	_, _ = e.Step(time.Now(), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}})
	d, err := e.Step(time.Now().Add(time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}})
	if err != nil || d.Action != ActionStopRecovery || e.State().PowerState != state.OnBattery {
		t.Fatalf("decision=%+v state=%s err=%v", d, e.State().PowerState, err)
	}
}

func TestRuntimeFallbackRecovery(t *testing.T) {
	now := time.Now()
	minCharge := 80.0
	st := baseState()
	st.PowerState = state.WaitingForAC
	st.ShutdownCommitted = true
	e := New(Config{UtilityStable: time.Second, RecoveryChargeMin: &minCharge, RecoveryRuntimeMin: 600 * time.Second}, st)
	_, _ = e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, RuntimeSeconds: i64(700)}})
	d, _ := e.Step(now.Add(2*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, RuntimeSeconds: i64(700)}, NetworkReady: true, HealthSafe: true})
	if d.Action != ActionCommitRecovery {
		t.Fatalf("runtime fallback did not pass: %+v", d)
	}
}
