package policy

import (
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

func TestPreCommitUtilityRestoreRequiresTwoConsecutiveSamples(t *testing.T) {
	now := time.Unix(30_000, 0)
	e := New(Config{GracePeriod: time.Minute}, baseState())
	if _, err := e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Step(now.Add(time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}}); err != nil {
		t.Fatal(err)
	}

	d, err := e.Step(now.Add(2*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Changed || e.State().PowerState != state.OnBattery {
		t.Fatalf("single OL sample cancelled outage: decision=%+v state=%s", d, e.State().PowerState)
	}

	d, err = e.Step(now.Add(3*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Changed || e.State().PowerState != state.Normal {
		t.Fatalf("second consecutive OL did not cancel outage: decision=%+v state=%s", d, e.State().PowerState)
	}
}

func TestCommunicationLossDuringActiveOutageCommitsAfterGrace(t *testing.T) {
	now := time.Unix(40_000, 0)
	e := New(Config{
		GracePeriod:            time.Minute,
		CommunicationLossGrace: 10 * time.Second,
	}, baseState())
	if _, err := e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Step(now.Add(time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}}); err != nil {
		t.Fatal(err)
	}
	if d, err := e.Step(now.Add(2*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityUnknown}}); err != nil || d.Action != ActionNone {
		t.Fatalf("first UNKNOWN should retain outage: decision=%+v err=%v", d, err)
	}
	d, err := e.Step(now.Add(12*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityUnknown}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionCommitShutdown || !e.State().ShutdownCommitted {
		t.Fatalf("communication-loss grace did not commit shutdown: decision=%+v state=%+v", d, e.State())
	}
}

func TestRecoveryNetworkWaitExpiresIntoFailedSafe(t *testing.T) {
	now := time.Unix(50_000, 0)
	charge := 100.0
	minCharge := 80.0
	st := baseState()
	st.PowerState = state.WaitingForAC
	st.ShutdownCommitted = true
	e := New(Config{
		UtilityStable:       time.Second,
		RecoveryChargeMin:   &minCharge,
		RecoveryNetworkWait: 5 * time.Second,
	}, st)

	// Reconcile the durable committed shutdown, then satisfy utility/recharge
	// gates while keeping the network unavailable.
	if _, err := e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: &charge}, NetworkReady: false, HealthSafe: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Step(now.Add(time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: &charge}, NetworkReady: false, HealthSafe: true}); err != nil {
		t.Fatal(err)
	}
	d, err := e.Step(now.Add(6*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: &charge}, NetworkReady: false, HealthSafe: true})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Changed || e.State().PowerState != state.FailedSafe || e.State().FailedSafeReason == "" {
		t.Fatalf("network timeout did not fail safe: decision=%+v state=%+v", d, e.State())
	}
}

func TestRestoreReentersGatesAfterBootAndPausesWhenNetworkDrops(t *testing.T) {
	now := time.Unix(60_000, 0)
	charge := 100.0
	minCharge := 80.0
	st := baseState()
	st.PowerState = state.RestoreHosts
	st.ShutdownCommitted = true
	st.RecoveryStarted = true
	e := New(Config{
		UtilityStable:       time.Second,
		RecoveryChargeMin:   &minCharge,
		RecoveryNetworkWait: time.Minute,
	}, st)

	// A reboot invalidates the old recovery permission. Even with utility
	// online, boot reconciliation must return behind every recovery gate.
	d, err := e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: &charge}, NetworkReady: false, HealthSafe: true})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionNone || e.State().PowerState != state.RecoveryWait || e.State().RecoveryStarted {
		t.Fatalf("restore did not re-enter gates at boot: decision=%+v state=%+v", d, e.State())
	}

	// After a fresh stable-utility interval, recharge gate, network readiness,
	// and health observation, recovery may be committed again.
	d, err = e.Step(now.Add(time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: &charge}, NetworkReady: true, HealthSafe: true})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionCommitRecovery || e.State().PowerState != state.RecoveryStarted {
		t.Fatalf("recovery did not recommit after gates passed: decision=%+v state=%s", d, e.State().PowerState)
	}

	d, err = e.Step(now.Add(2*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: &charge}, NetworkReady: true, HealthSafe: true})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionStartRestore || e.State().PowerState != state.RestoreHosts {
		t.Fatalf("restore did not start after durable recovery commit: decision=%+v state=%s", d, e.State().PowerState)
	}

	// Drop the network after host restoration has begun. Policy must move behind
	// the start gate so the orchestrator cannot issue another WoL on this tick.
	d, err = e.Step(now.Add(3*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnline, ChargePercent: &charge}, NetworkReady: false, HealthSafe: true})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Changed || e.State().PowerState != state.RecoveryStarted {
		t.Fatalf("restore did not pause on network loss: decision=%+v state=%s", d, e.State().PowerState)
	}
}

func TestCriticalHealthFailureAfterRecoveryCommitFailsSafe(t *testing.T) {
	now := time.Unix(70_000, 0)
	st := baseState()
	st.PowerState = state.RecoveryStarted
	st.ShutdownCommitted = true
	st.RecoveryStarted = true

	// Exercise the already-running committed-recovery state directly. Boot
	// reconciliation is tested separately and intentionally re-enters the gates.
	e := &Engine{cfg: Config{}, st: st}
	d, err := e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}, NetworkReady: true, HealthSafe: false})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Changed || e.State().PowerState != state.FailedSafe {
		t.Fatalf("unsafe health did not fail safe: decision=%+v state=%+v", d, e.State())
	}
}

func TestMaxOnBatteryUsesDurableElapsedAfterReboot(t *testing.T) {
	now := time.Unix(80_000, 0)
	st := baseState()
	st.PowerState = state.OnBattery
	st.OutageElapsedSeconds = 50
	e := New(Config{GracePeriod: 10 * time.Second, MaxOnBattery: 60 * time.Second}, st)

	if _, err := e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}}); err != nil {
		t.Fatal(err)
	}
	d, err := e.Step(now.Add(10*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != ActionCommitShutdown {
		t.Fatalf("durable outage elapsed was reset by reboot: decision=%+v state=%+v", d, e.State())
	}
}

func TestOutageElapsedIsCheckpointedWithoutWallClockPersistence(t *testing.T) {
	now := time.Unix(90_000, 0)
	e := New(Config{GracePeriod: time.Minute}, baseState())
	if _, err := e.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Step(now.Add(time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}}); err != nil {
		t.Fatal(err)
	}
	d, err := e.Step(now.Add(31*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Changed || e.State().OutageElapsedSeconds < 30 {
		t.Fatalf("outage elapsed checkpoint missing: decision=%+v elapsed=%d", d, e.State().OutageElapsedSeconds)
	}
}
