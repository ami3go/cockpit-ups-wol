package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/host"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/policy"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

type wakeDelayStore struct{ st state.State }

func (s *wakeDelayStore) Write(in state.State) (state.State, error) {
	s.st = in
	return in, nil
}

type wakeDelayRunner struct {
	coord *policy.Coordinator
	calls []string
}

func (r *wakeDelayRunner) RunNext(_ context.Context, st state.State, hosts []host.Config) (state.State, string, error) {
	next, ok, err := host.NextRecoveryHost(hosts, st.Hosts)
	if err != nil || !ok {
		return st, "", err
	}
	hs := st.Hosts[next.ID]
	hs.WakeAttempts++
	hs.RecoveryState = state.RecoveryOnline
	st.Hosts[next.ID] = hs
	if _, err := r.coord.Write(st); err != nil {
		return st, next.ID, err
	}
	r.calls = append(r.calls, next.ID)
	return st, next.ID, nil
}

func wakeDelayController(t *testing.T, aState, bState state.HostState) (*Controller, *wakeDelayRunner) {
	t.Helper()
	st := state.New("recovery-delay", "cfg")
	st.PowerState = state.RestoreHosts
	st.ShutdownCommitted = true
	st.RecoveryStarted = true
	st.Hosts = map[string]state.HostState{
		"host-a": aState,
		"host-b": bState,
	}
	store := &wakeDelayStore{st: st}
	engine := policy.New(policy.Config{}, st)
	coord := policy.NewCoordinator(engine, store)
	if _, err := coord.Write(st); err != nil {
		t.Fatal(err)
	}
	runner := &wakeDelayRunner{coord: coord}
	ctl := &Controller{
		Policy:   coord,
		Recovery: runner,
		Config: config.Config{Hosts: []config.HostConfig{
			{
				ID:            "host-a",
				Name:          "host-a",
				RestorePolicy: "always",
				Wake: config.WakeConfig{
					Enabled:                   true,
					Priority:                  10,
					DelayAfterPreviousSeconds: 30,
					MaxAttempts:               3,
				},
			},
			{
				ID:            "host-b",
				Name:          "host-b",
				RestorePolicy: "always",
				Wake: config.WakeConfig{
					Enabled:                   true,
					Priority:                  20,
					DelayAfterPreviousSeconds: 30,
					MaxAttempts:               3,
				},
			},
		}},
	}
	return ctl, runner
}

func TestWakeDelayBetweenRecoveredHostsIsNonBlocking(t *testing.T) {
	waiting := state.HostState{RecoveryState: state.RecoveryWaiting}
	ctl, runner := wakeDelayController(t, waiting, waiting)
	t0 := time.Unix(1000, 0)

	if err := ctl.executeRecoveryStep(context.Background(), t0); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "host-a" {
		t.Fatalf("first recovery calls=%v", runner.calls)
	}

	if err := ctl.executeRecoveryStep(context.Background(), t0.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("second host woke before delay expired: %v", runner.calls)
	}

	if err := ctl.executeRecoveryStep(context.Background(), t0.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 || runner.calls[1] != "host-b" {
		t.Fatalf("second host not restored after delay: %v", runner.calls)
	}
}

func TestWakeDelayIsConservativelyReappliedAfterControllerRestart(t *testing.T) {
	a := state.HostState{RecoveryState: state.RecoveryOnline, WakeAttempts: 1}
	b := state.HostState{RecoveryState: state.RecoveryWaiting}
	ctl, runner := wakeDelayController(t, a, b)
	t0 := time.Unix(2000, 0)

	// The pre-reboot monotonic wake timestamp is unknowable. The controller must
	// apply the complete configured delay once rather than waking immediately.
	if err := ctl.executeRecoveryStep(context.Background(), t0); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("restart bypassed wake delay: %v", runner.calls)
	}
	if err := ctl.executeRecoveryStep(context.Background(), t0.Add(29*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("restart wake delay expired early: %v", runner.calls)
	}
	if err := ctl.executeRecoveryStep(context.Background(), t0.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "host-b" {
		t.Fatalf("host did not restore after conservative delay: %v", runner.calls)
	}
}
