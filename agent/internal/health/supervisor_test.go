package health

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

type fakeCheck struct {
	name   string
	result Result
}

func (f fakeCheck) Name() string { return f.name }
func (f fakeCheck) Run(context.Context) Result {
	r := f.result
	if r.Name == "" {
		r.Name = f.name
	}
	return r
}

type fakeRepair struct {
	name  string
	calls int
	err   error
}

func (f *fakeRepair) Name() string                 { return f.name }
func (f *fakeRepair) Repair(context.Context) error { f.calls++; return f.err }

func TestHealthySnapshot(t *testing.T) {
	s := &Supervisor{Checks: []Check{fakeCheck{name: "ok", result: Result{OK: true}}}, StatePath: filepath.Join(t.TempDir(), "health.json")}
	snap, err := s.Run(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if snap.State != Healthy {
		t.Fatalf("state=%s", snap.State)
	}
}

func TestRepairCircuitPersistsAcrossRuns(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	repair := &fakeRepair{name: "systemd:agent", err: errors.New("still broken")}
	path := filepath.Join(t.TempDir(), "health.json")
	s := &Supervisor{Checks: []Check{fakeCheck{name: "systemd:agent", result: Result{OK: false, Critical: true, Repairable: true}}}, Repairers: map[string]Repairer{"systemd:agent": repair}, StatePath: path, MaxRepairAttempts: 2, BaseBackoff: time.Second, MaxBackoff: time.Second, Now: func() time.Time { return now }}
	first, err := s.Run(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if first.Circuit.Attempts["systemd:agent"] != 1 || repair.calls != 1 {
		t.Fatalf("first=%+v calls=%d", first, repair.calls)
	}
	now = now.Add(2 * time.Second)
	// New Supervisor simulates reboot/process restart and must retain attempts.
	s2 := &Supervisor{Checks: s.Checks, Repairers: s.Repairers, StatePath: path, MaxRepairAttempts: 2, BaseBackoff: time.Second, MaxBackoff: time.Second, Now: func() time.Time { return now }}
	second, err := s2.Run(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if second.State != FailedSafe {
		t.Fatalf("state=%s", second.State)
	}
	if second.Circuit.Attempts["systemd:agent"] != 2 {
		t.Fatalf("attempts=%d", second.Circuit.Attempts["systemd:agent"])
	}
}

func TestUnregisteredRepairNeverRunsArbitraryAction(t *testing.T) {
	s := &Supervisor{Checks: []Check{fakeCheck{name: "unsafe", result: Result{OK: false, Critical: true, Repairable: true}}}, StatePath: filepath.Join(t.TempDir(), "health.json")}
	snap, err := s.Run(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Repairs) != 0 {
		t.Fatalf("repairs=%v", snap.Repairs)
	}
	if snap.State != Degraded {
		t.Fatalf("state=%s", snap.State)
	}
}
