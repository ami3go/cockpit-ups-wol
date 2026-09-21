package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/policy"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

func TestRecoveryPowerBounceStartsNewOutageAndReshutsRecoveredHost(t *testing.T) {
	events := []string{}
	cfg := testConfig()
	st := state.New("outage-1", "cfg")
	st.PowerState = state.RestoreHosts
	st.ShutdownCommitted = true
	st.RecoveryStarted = true
	wasOnline := true
	st.Hosts["pc"] = state.HostState{
		WasOnline:        &wasOnline,
		ShutdownState:    state.ShutdownCompleted,
		RecoveryState:    state.RecoveryOnline,
		ShutdownAttempts: 1,
	}
	st.Hosts["nas"] = state.HostState{
		WasOnline:        &wasOnline,
		ShutdownState:    state.ShutdownCompleted,
		RecoveryState:    state.RecoveryWaiting,
		ShutdownAttempts: 1,
	}

	store := &memoryStore{st: st, events: &events}
	coord := policy.NewCoordinator(policy.New(policyConfig(cfg), st), store)
	probe := &fakeProbe{online: map[string]bool{"pc": true, "nas": false}}
	ctl := &Controller{
		Config:           cfg,
		Policy:           coord,
		Probe:            probe,
		Shutdown:         fakeShutdown{&events},
		FSD:              fakeFSD{&events},
		NewTransactionID: func() string { return "outage-2" },
	}
	ctl.Recovery = fakeRecovery{events: &events, coord: coord}

	now := time.Unix(300, 0)
	decision, err := ctl.Tick(context.Background(), now, policy.Inputs{
		UPS:          nut.Status{Utility: nut.UtilityOnBattery},
		NetworkReady: true,
		HealthSafe:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != policy.ActionStopRecovery {
		t.Fatalf("expected stop recovery, got %+v", decision)
	}

	afterBounce := coord.State()
	if afterBounce.PowerState != state.OnBattery {
		t.Fatalf("expected ON_BATTERY after bounce, got %s", afterBounce.PowerState)
	}
	if afterBounce.TransactionID != "outage-2" || afterBounce.ParentTransactionID != "outage-1" {
		t.Fatalf("new outage epoch not created: tx=%q parent=%q", afterBounce.TransactionID, afterBounce.ParentTransactionID)
	}
	pc := afterBounce.Hosts["pc"]
	if pc.WasOnline == nil || !*pc.WasOnline {
		t.Fatalf("recovered PC was not re-snapshotted online: %+v", pc)
	}
	if pc.ShutdownState != state.ShutdownPlanned {
		t.Fatalf("stale completed shutdown survived bounce: %+v", pc)
	}
	if pc.RecoveryState != state.RecoveryWaiting {
		t.Fatalf("recovery state was not reset for new outage: %+v", pc)
	}

	low := nut.Status{Utility: nut.UtilityOnBattery, LowBattery: true}
	if _, err := ctl.Tick(context.Background(), now.Add(time.Second), policy.Inputs{
		UPS:          low,
		NetworkReady: true,
		HealthSafe:   true,
	}); err != nil {
		t.Fatal(err)
	}
	if index(events, "shutdown:pc") < 0 {
		t.Fatalf("recovered PC was not shut down on second outage: %#v", events)
	}
}
