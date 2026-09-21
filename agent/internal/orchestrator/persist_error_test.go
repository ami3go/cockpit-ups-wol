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

var errPersistTest = errors.New("persist failed")

type alwaysFailWriter struct{}

func (alwaysFailWriter) Write(state.State) (state.State, error) {
	return state.State{}, errPersistTest
}

type verifyProber struct{}

func (verifyProber) Check(context.Context, config.HostConfig) (host.ProbeResult, error) {
	return host.ProbeResult{Known: true, Online: true}, nil
}

func (verifyProber) Wait(context.Context, config.HostConfig, bool) error { return nil }

func controllerWithFailingStore(shutdownState state.ShutdownState) *Controller {
	st := state.New("outage-1", "cfg")
	st.Hosts["nas"] = state.HostState{ShutdownState: shutdownState}
	engine := policy.New(policy.Config{}, st)
	return &Controller{Policy: policy.NewCoordinator(engine, alwaysFailWriter{})}
}

func TestMarkNUTAcknowledgedReturnsPersistenceFailure(t *testing.T) {
	c := controllerWithFailingStore(state.ShutdownRequested)
	if err := c.markNUTAcknowledged([]string{"nas"}); !errors.Is(err, errPersistTest) {
		t.Fatalf("err=%v want persist failure", err)
	}
}

func TestMarkNUTErrorReturnsPersistenceFailure(t *testing.T) {
	c := controllerWithFailingStore(state.ShutdownRequested)
	if err := c.markNUTError([]string{"nas"}, errors.New("FSD failed")); !errors.Is(err, errPersistTest) {
		t.Fatalf("err=%v want persist failure", err)
	}
}

func TestVerifyNUTGroupReturnsPersistenceFailure(t *testing.T) {
	c := controllerWithFailingStore(state.ShutdownAcknowledged)
	c.Probe = verifyProber{}
	address := "nas.lan"
	byID := map[string]config.HostConfig{
		"nas": {
			ID:      "nas",
			Address: &address,
			Status:  config.StatusConfig{Method: "ping"},
		},
	}
	if err := c.verifyNUTGroup(context.Background(), []string{"nas"}, byID); !errors.Is(err, errPersistTest) {
		t.Fatalf("err=%v want persist failure", err)
	}
}
