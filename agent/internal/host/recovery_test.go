package host

import (
	"context"
	"errors"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

type fakeWake struct {
	calls []string
	err   error
}

func (f *fakeWake) Wake(_ context.Context, id string) error {
	f.calls = append(f.calls, id)
	return f.err
}

type fakeCheck struct {
	online map[string]bool
	err    error
}

func (f fakeCheck) Online(_ context.Context, id string) (bool, error) { return f.online[id], f.err }

type fakeRecoveryStore struct {
	writes []state.State
	err    error
}

func (f *fakeRecoveryStore) Write(st state.State) (state.State, error) {
	if f.err != nil {
		return state.State{}, f.err
	}
	st.Sequence++
	f.writes = append(f.writes, st)
	return st, nil
}

func TestNextRecoveryHostWaitsForManagedDependency(t *testing.T) {
	hosts := []Config{
		{ID: "db", WakeEnabled: true, RestorePolicy: RestoreAlways, WakePriority: 20},
		{ID: "app", WakeEnabled: true, RestorePolicy: RestoreAlways, WakePriority: 10, DependsOn: []string{"db"}},
	}
	states := map[string]state.HostState{
		"db":  {RecoveryState: state.RecoveryWaiting},
		"app": {RecoveryState: state.RecoveryWaiting},
	}
	got, ok, err := NextRecoveryHost(hosts, states)
	if err != nil || !ok || got.ID != "db" {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}
	db := states["db"]
	db.RecoveryState = state.RecoveryOnline
	states["db"] = db
	got, ok, err = NextRecoveryHost(hosts, states)
	if err != nil || !ok || got.ID != "app" {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}
}

func TestRunNextPersistsAttemptBeforeWake(t *testing.T) {
	was := true
	st := state.New("outage-x", "cfg-x")
	st.ShutdownCommitted = true
	st.RecoveryStarted = true
	st.PowerState = state.RestoreHosts
	st.Hosts["nas"] = state.HostState{WasOnline: &was, RecoveryState: state.RecoveryWaiting}
	w := &fakeWake{}
	store := &fakeRecoveryStore{}
	ex := RecoveryExecutor{Waker: w, Checker: fakeCheck{online: map[string]bool{"nas": false}}, Store: store}
	out, id, err := ex.RunNext(context.Background(), st, []Config{{ID: "nas", WakeEnabled: true, RestorePolicy: RestorePrevious, WakeMaxAttempts: 5}})
	if err != nil || id != "nas" {
		t.Fatalf("id=%s err=%v", id, err)
	}
	if len(w.calls) != 1 || w.calls[0] != "nas" {
		t.Fatalf("wake calls=%v", w.calls)
	}
	if len(store.writes) == 0 {
		t.Fatal("wake occurred without persisted attempt")
	}
	persisted := store.writes[0].Hosts["nas"]
	if persisted.WakeAttempts != 1 || persisted.RecoveryState != state.RecoveryWOLSent {
		t.Fatalf("persisted=%+v", persisted)
	}
	if out.Hosts["nas"].WakeAttempts != 1 {
		t.Fatalf("out=%+v", out.Hosts["nas"])
	}
}

func TestRunNextReconcilesOnlineWithoutDuplicateWake(t *testing.T) {
	was := true
	st := state.New("outage-x", "cfg-x")
	st.ShutdownCommitted = true
	st.RecoveryStarted = true
	st.PowerState = state.RestoreHosts
	st.Hosts["nas"] = state.HostState{WasOnline: &was, RecoveryState: state.RecoveryWOLSent, WakeAttempts: 1}
	w := &fakeWake{}
	store := &fakeRecoveryStore{}
	ex := RecoveryExecutor{Waker: w, Checker: fakeCheck{online: map[string]bool{"nas": true}}, Store: store}
	out, _, err := ex.RunNext(context.Background(), st, []Config{{ID: "nas", WakeEnabled: true, RestorePolicy: RestorePrevious}})
	if err != nil {
		t.Fatal(err)
	}
	if len(w.calls) != 0 {
		t.Fatalf("duplicate wake calls=%v", w.calls)
	}
	if out.Hosts["nas"].RecoveryState != state.RecoveryOnline {
		t.Fatalf("state=%+v", out.Hosts["nas"])
	}
}

func TestRunNextMaxAttemptsFailsWithoutWake(t *testing.T) {
	st := state.New("outage-x", "cfg-x")
	st.Hosts["nas"] = state.HostState{RecoveryState: state.RecoveryWaiting, WakeAttempts: 2}
	w := &fakeWake{}
	store := &fakeRecoveryStore{}
	ex := RecoveryExecutor{Waker: w, Checker: fakeCheck{online: map[string]bool{"nas": false}}, Store: store}
	out, _, err := ex.RunNext(context.Background(), st, []Config{{ID: "nas", WakeEnabled: true, RestorePolicy: RestoreAlways, WakeMaxAttempts: 2}})
	if err == nil {
		t.Fatal("expected max attempt error")
	}
	if len(w.calls) != 0 {
		t.Fatalf("wake calls=%v", w.calls)
	}
	if out.Hosts["nas"].RecoveryState != state.RecoveryFailed {
		t.Fatalf("state=%+v", out.Hosts["nas"])
	}
}

func TestRunNextPersistenceFailureSuppressesWake(t *testing.T) {
	st := state.New("outage-x", "cfg-x")
	st.Hosts["nas"] = state.HostState{RecoveryState: state.RecoveryWaiting}
	w := &fakeWake{}
	ex := RecoveryExecutor{Waker: w, Checker: fakeCheck{online: map[string]bool{"nas": false}}, Store: &fakeRecoveryStore{err: errors.New("disk")}}
	_, _, err := ex.RunNext(context.Background(), st, []Config{{ID: "nas", WakeEnabled: true, RestorePolicy: RestoreAlways}})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(w.calls) != 0 {
		t.Fatalf("wake occurred despite persistence failure: %v", w.calls)
	}
}
