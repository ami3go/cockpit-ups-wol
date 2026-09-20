package policy

import (
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

func TestRecoveryDisabledNeverStarts(t *testing.T) {
	now := time.Unix(10_000, 0)
	disabled := false
	minCharge := 80.0

	st := baseState()
	st.PowerState = state.WaitingForAC
	st.ShutdownCommitted = true

	cfg := Config{
		RecoveryEnabled:   &disabled,
		UtilityStable:     time.Second,
		RecoveryChargeMin: &minCharge,
	}

	e := New(cfg, st)
	d, err := e.Step(now, Inputs{
		UPS:          nut.Status{Utility: nut.UtilityOnline, ChargePercent: f64(100)},
		NetworkReady: true,
		HealthSafe:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionNone || e.State().PowerState != state.WaitingForAC || e.State().RecoveryStarted {
		t.Fatalf("disabled recovery advanced: decision=%+v state=%+v", d, e.State())
	}

	d, err = e.Step(now.Add(time.Hour), Inputs{
		UPS:          nut.Status{Utility: nut.UtilityOnline, ChargePercent: f64(100)},
		NetworkReady: true,
		HealthSafe:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionNone || e.State().PowerState != state.WaitingForAC || e.State().RecoveryStarted {
		t.Fatalf("disabled recovery advanced after wait: decision=%+v state=%+v", d, e.State())
	}

	// Reboot/recreate the policy engine from durable state. Disabled recovery
	// must remain disabled and must not be re-enabled by BOOT_RECONCILE.
	e = New(cfg, e.State())
	d, err = e.Step(now.Add(2*time.Hour), Inputs{
		UPS:          nut.Status{Utility: nut.UtilityOnline, ChargePercent: f64(100)},
		NetworkReady: true,
		HealthSafe:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionNone || e.State().PowerState != state.WaitingForAC || e.State().RecoveryStarted {
		t.Fatalf("boot reconciliation re-enabled recovery: decision=%+v state=%+v", d, e.State())
	}
}

func TestRecoveryDisableStopsLegacyCommittedRecovery(t *testing.T) {
	disabled := false
	st := baseState()
	st.PowerState = state.RecoveryStarted
	st.ShutdownCommitted = true
	st.RecoveryStarted = true

	e := New(Config{RecoveryEnabled: &disabled}, st)
	d, err := e.Step(time.Unix(20_000, 0), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionStopRecovery || e.State().PowerState != state.WaitingForAC || e.State().RecoveryStarted {
		t.Fatalf("legacy committed recovery not stopped: decision=%+v state=%+v", d, e.State())
	}
}
