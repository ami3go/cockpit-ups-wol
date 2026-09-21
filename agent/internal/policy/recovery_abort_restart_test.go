package policy

import (
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

func TestBootCompletesInterruptedRecoveryAbortOnBattery(t *testing.T) {
	st := state.New("outage-1", "cfg")
	st.PowerState = state.OnBattery
	st.ShutdownCommitted = true
	st.RecoveryStarted = false
	engine := New(Config{RecoveryEnabled: boolPtr(true)}, st)

	decision, err := engine.Step(time.Unix(100, 0), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionStopRecovery {
		t.Fatalf("action=%s, want %s", decision.Action, ActionStopRecovery)
	}
	if engine.State().PowerState != state.OnBattery {
		t.Fatalf("power=%s, want ON_BATTERY", engine.State().PowerState)
	}
}

func boolPtr(v bool) *bool { return &v }
