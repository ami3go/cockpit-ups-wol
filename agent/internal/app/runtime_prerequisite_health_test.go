package app

import (
	"context"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
)

func TestRuntimePrerequisiteCheckIsVisibleButNoncritical(t *testing.T) {
	check := runtimePrerequisiteCheck{hostID: "pc", reason: "missing key"}
	result := check.Run(context.Background())
	if result.OK || result.Critical || result.Name != "host-prerequisite:pc" || result.Message != "missing key" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCriticalAgentHealthSafeAllowsNoncriticalDegradation(t *testing.T) {
	snap := health.NewSnapshot()
	snap.State = health.Degraded
	snap.Results = []health.Result{
		{Name: "nut", OK: true, Critical: true},
		{Name: "host-prerequisite:pc", OK: false, Critical: false},
	}
	if !criticalAgentHealthSafe(snap) {
		t.Fatal("noncritical host prerequisite warning blocked recovery")
	}
	snap.Results[0].OK = false
	if criticalAgentHealthSafe(snap) {
		t.Fatal("critical health failure was accepted")
	}
}
