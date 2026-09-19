package policy

import (
	"errors"
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

type fakeStateWriter struct {
	writes []state.State
	err    error
}

func (f *fakeStateWriter) Write(st state.State) (state.State, error) {
	if f.err != nil {
		return state.State{}, f.err
	}
	st.Sequence++
	f.writes = append(f.writes, st)
	return st, nil
}

func TestCoordinatorPersistsShutdownCommitBeforeReturningAction(t *testing.T) {
	st := baseState()
	engine := New(Config{}, st)
	writer := &fakeStateWriter{}
	c := NewCoordinator(engine, writer)
	now := time.Unix(100, 0)

	// First observation reconciles boot and must itself be durable.
	if _, err := c.Step(now, Inputs{UPS: nut.Status{Utility: nut.UtilityOnline}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Step(now.Add(time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery, LowBattery: true}}); err != nil {
		t.Fatal(err)
	}
	decision, err := c.Step(now.Add(2*time.Second), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery, LowBattery: true}})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionCommitShutdown {
		t.Fatalf("action=%s want %s", decision.Action, ActionCommitShutdown)
	}
	if len(writer.writes) == 0 {
		t.Fatal("commit decision returned without durable state write")
	}
	last := writer.writes[len(writer.writes)-1]
	if !last.ShutdownCommitted || last.PowerState != state.ShutdownCommitted {
		t.Fatalf("last durable state is not shutdown commit: %+v", last)
	}
}

func TestCoordinatorSuppressesActionWhenPersistenceFails(t *testing.T) {
	st := baseState()
	st.PowerState = state.OnBattery
	engine := New(Config{}, st)
	// Re-enter a direct active state after constructor BOOT_RECONCILE for this test.
	engine.st.PowerState = state.OnBattery
	writer := &fakeStateWriter{err: errors.New("disk failed")}
	c := NewCoordinator(engine, writer)

	decision, err := c.Step(time.Unix(100, 0), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery, LowBattery: true}})
	if err == nil {
		t.Fatal("expected persistence error")
	}
	if decision.Action != "" {
		t.Fatalf("external action leaked despite persistence failure: %+v", decision)
	}
	if c.State().ShutdownCommitted {
		t.Fatal("in-memory state must roll back when durable write fails")
	}
}
