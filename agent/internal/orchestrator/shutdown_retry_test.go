package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/host"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/policy"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

type shutdownRetryStore struct{ st state.State }

func (s *shutdownRetryStore) Write(in state.State) (state.State, error) {
	s.st = in
	return in, nil
}

type shutdownRetryProbe struct{}

func (shutdownRetryProbe) Check(context.Context, config.HostConfig) (host.ProbeResult, error) {
	return host.ProbeResult{Known: true, Online: true}, nil
}
func (shutdownRetryProbe) Wait(context.Context, config.HostConfig, bool) error { return nil }

type flakyDirectShutdown struct {
	calls    int
	failFor  int
}

func (s *flakyDirectShutdown) Shutdown(context.Context, config.HostConfig) (host.ShutdownResult, error) {
	s.calls++
	if s.calls <= s.failFor {
		return host.ShutdownResult{}, errors.New("temporary ssh failure")
	}
	return host.ShutdownResult{Disposition: host.ShutdownDirectRequested}, nil
}

func directRetryController(failFor int) (*Controller, *flakyDirectShutdown) {
	addr := "192.0.2.10"
	st := state.New("outage-retry", "cfg")
	st.PowerState = state.ShutdownInProgress
	st.ShutdownCommitted = true
	st.Hosts["pc"] = state.HostState{ShutdownState: state.ShutdownPlanned}
	store := &shutdownRetryStore{st: st}
	coord := policy.NewCoordinator(policy.New(policy.Config{}, st), store)
	shutdown := &flakyDirectShutdown{failFor: failFor}
	ctl := &Controller{
		Policy:   coord,
		Probe:    shutdownRetryProbe{},
		Shutdown: shutdown,
	}
	ctl.Config.Hosts = []config.HostConfig{{
		ID:      "pc",
		Address: &addr,
		Status:  config.StatusConfig{Method: "tcp"},
		Shutdown: config.ShutdownConfig{
			Method:         "ssh",
			TimeoutSeconds: 1,
		},
	}}
	return ctl, shutdown
}

func TestDirectShutdownRetriesAfterReconciliation(t *testing.T) {
	ctl, shutdown := directRetryController(2)
	h := ctl.Config.Hosts[0]

	for attempt := 1; attempt <= 2; attempt++ {
		err := ctl.settleDirect(context.Background(), h)
		if !errors.Is(err, errDirectShutdownRetryPending) {
			t.Fatalf("attempt %d: err=%v want retry pending", attempt, err)
		}
		hs := ctl.Policy.State().Hosts["pc"]
		if hs.ShutdownState != state.ShutdownUnknown || hs.ShutdownAttempts != attempt {
			t.Fatalf("attempt %d state=%+v", attempt, hs)
		}
	}

	if err := ctl.settleDirect(context.Background(), h); err != nil {
		t.Fatalf("third attempt should succeed: %v", err)
	}
	if shutdown.calls != 3 {
		t.Fatalf("shutdown calls=%d want 3", shutdown.calls)
	}
	hs := ctl.Policy.State().Hosts["pc"]
	if hs.ShutdownState != state.ShutdownCompleted || hs.ShutdownAttempts != 3 {
		t.Fatalf("final state=%+v", hs)
	}
}

func TestDirectShutdownExhaustionEntersFailedSafe(t *testing.T) {
	ctl, shutdown := directRetryController(99)
	h := ctl.Config.Hosts[0]

	for attempt := 1; attempt <= maxDirectShutdownAttempts; attempt++ {
		err := ctl.settleDirect(context.Background(), h)
		if attempt < maxDirectShutdownAttempts {
			if !errors.Is(err, errDirectShutdownRetryPending) {
				t.Fatalf("attempt %d: err=%v want retry pending", attempt, err)
			}
		} else if !errors.Is(err, errDirectShutdownFailedSafe) {
			t.Fatalf("attempt %d: err=%v want failed-safe", attempt, err)
		}
	}

	if shutdown.calls != maxDirectShutdownAttempts {
		t.Fatalf("shutdown calls=%d want %d", shutdown.calls, maxDirectShutdownAttempts)
	}
	st := ctl.Policy.State()
	if st.PowerState != state.FailedSafe {
		t.Fatalf("power state=%s want FAILED_SAFE", st.PowerState)
	}
	if st.Hosts["pc"].ShutdownState != state.ShutdownFailed {
		t.Fatalf("host state=%+v", st.Hosts["pc"])
	}
	if st.FailedSafeReason == "" {
		t.Fatal("FAILED_SAFE reason missing")
	}
}
