package host

import (
	"reflect"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

func boolp(v bool) *bool { return &v }

func TestBuildShutdownPlan(t *testing.T) {
	hosts := []Config{
		{ID: "nas", ShutdownMethod: "nut", ShutdownPriority: 20},
		{ID: "desktop", ShutdownMethod: "ssh", ShutdownPriority: 10},
		{ID: "server", ShutdownMethod: "command", ShutdownPriority: 30},
		{ID: "observe", ShutdownMethod: "none"},
	}
	p := BuildShutdownPlan(hosts)
	if !reflect.DeepEqual(p.PreFSD, []string{"desktop", "server"}) {
		t.Fatalf("pre=%v", p.PreFSD)
	}
	if !reflect.DeepEqual(p.NUTGroup, []string{"nas"}) {
		t.Fatalf("nut=%v", p.NUTGroup)
	}
}

func TestEligiblePreviousUnknownDoesNotWake(t *testing.T) {
	cfg := Config{ID: "nas", WakeEnabled: true, RestorePolicy: RestorePrevious}
	if EligibleForRestore(cfg, state.HostState{WasOnline: nil}) {
		t.Fatal("unknown previous state must not wake")
	}
	if !EligibleForRestore(cfg, state.HostState{WasOnline: boolp(true)}) {
		t.Fatal("previously-online host should be eligible")
	}
}

func TestRecoveryPlanRespectsDependenciesAndPriority(t *testing.T) {
	hosts := []Config{
		{ID: "app", WakeEnabled: true, WakePriority: 5, RestorePolicy: RestoreAlways, DependsOn: []string{"db"}},
		{ID: "db", WakeEnabled: true, WakePriority: 20, RestorePolicy: RestoreAlways},
		{ID: "nas", WakeEnabled: true, WakePriority: 10, RestorePolicy: RestoreAlways},
	}
	states := map[string]state.HostState{
		"app": {}, "db": {}, "nas": {},
	}
	got, err := BuildRecoveryPlan(hosts, states)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"nas", "db", "app"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}

func TestRecoveryDependencyCycle(t *testing.T) {
	hosts := []Config{
		{ID: "a", WakeEnabled: true, RestorePolicy: RestoreAlways, DependsOn: []string{"b"}},
		{ID: "b", WakeEnabled: true, RestorePolicy: RestoreAlways, DependsOn: []string{"a"}},
	}
	states := map[string]state.HostState{"a": {}, "b": {}}
	if _, err := BuildRecoveryPlan(hosts, states); err == nil {
		t.Fatal("expected cycle error")
	}
}
