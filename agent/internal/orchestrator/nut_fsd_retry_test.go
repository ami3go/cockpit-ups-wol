package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/policy"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

type failingFSD struct{ calls int }

func (f *failingFSD) RequestFSD(context.Context, string, string) error {
	f.calls++
	return errors.New("fsd unavailable")
}

func TestNUTFSDFailureRetriesWithoutKillingController(t *testing.T) {
	events := []string{}
	cfg := testConfig()
	cfg.Hosts = cfg.Hosts[1:]
	st := state.New("outage-1", "cfg")
	st.PowerState = state.ShutdownInProgress
	st.ShutdownCommitted = true
	wasOnline := true
	st.Hosts["nas"] = state.HostState{WasOnline: &wasOnline, ShutdownState: state.ShutdownPlanned, RecoveryState: state.RecoveryWaiting}
	store := &memoryStore{st: st, events: &events}
	coord := policy.NewCoordinator(policy.New(policyConfig(cfg), st), store)
	if _, err := coord.Write(st); err != nil {
		t.Fatal(err)
	}
	probe := &fakeProbe{online: map[string]bool{"nas": true}}
	fsd := &failingFSD{}
	ctl := &Controller{Config: cfg, Policy: coord, Probe: probe, Shutdown: fakeShutdown{&events}, FSD: fsd}
	now := time.Unix(1000, 0)
	in := policy.Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}, NetworkReady: true, HealthSafe: true}

	if _, err := ctl.Tick(context.Background(), now, in); err != nil {
		t.Fatalf("FSD transport failure must be retryable, got %v", err)
	}
	if fsd.calls != 1 || coord.State().Hosts["nas"].ShutdownAttempts != 1 {
		t.Fatalf("after first attempt calls=%d attempts=%d", fsd.calls, coord.State().Hosts["nas"].ShutdownAttempts)
	}
	if _, err := ctl.Tick(context.Background(), now.Add(time.Second), in); err != nil {
		t.Fatal(err)
	}
	if fsd.calls != 1 || coord.State().Hosts["nas"].ShutdownAttempts != 1 {
		t.Fatalf("retry backoff ignored: calls=%d attempts=%d", fsd.calls, coord.State().Hosts["nas"].ShutdownAttempts)
	}
	if _, err := ctl.Tick(context.Background(), now.Add(6*time.Second), in); err != nil {
		t.Fatal(err)
	}
	if fsd.calls != 2 || coord.State().Hosts["nas"].ShutdownAttempts != 2 {
		t.Fatalf("scheduled retry missing: calls=%d attempts=%d", fsd.calls, coord.State().Hosts["nas"].ShutdownAttempts)
	}
}

func TestNUTFSDRetriesAreBoundedAcrossDurableAttempts(t *testing.T) {
	events := []string{}
	cfg := testConfig()
	cfg.Hosts = cfg.Hosts[1:]
	st := state.New("outage-1", "cfg")
	st.PowerState = state.ShutdownInProgress
	st.ShutdownCommitted = true
	wasOnline := true
	st.Hosts["nas"] = state.HostState{WasOnline: &wasOnline, ShutdownState: state.ShutdownUnknown, RecoveryState: state.RecoveryWaiting, ShutdownAttempts: maxNUTFSDShutdownAttempts}
	store := &memoryStore{st: st, events: &events}
	coord := policy.NewCoordinator(policy.New(policyConfig(cfg), st), store)
	if _, err := coord.Write(st); err != nil {
		t.Fatal(err)
	}
	fsd := &failingFSD{}
	ctl := &Controller{Config: cfg, Policy: coord, Probe: &fakeProbe{online: map[string]bool{"nas": true}}, Shutdown: fakeShutdown{&events}, FSD: fsd}
	_, err := ctl.Tick(context.Background(), time.Unix(2000, 0), policy.Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}, NetworkReady: true, HealthSafe: true})
	if err != nil {
		t.Fatal(err)
	}
	if fsd.calls != 0 {
		t.Fatalf("FSD retried after durable cap: %d calls", fsd.calls)
	}
	if coord.State().PowerState != state.FailedSafe || coord.State().Hosts["nas"].ShutdownState != state.ShutdownFailed {
		t.Fatalf("expected durable failed-safe after bounded NUT retries: power=%s host=%s", coord.State().PowerState, coord.State().Hosts["nas"].ShutdownState)
	}
}
