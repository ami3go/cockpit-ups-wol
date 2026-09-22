package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
)

func TestSystemHealthSafeRequiresFreshCriticalHealth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "system-health.json")
	now := time.Date(2026, 9, 21, 1, 30, 0, 0, time.UTC)

	if ok, reason := systemHealthSafe(path, 3*time.Minute, now); ok || !strings.Contains(reason, "unavailable") {
		t.Fatalf("missing snapshot: ok=%v reason=%q", ok, reason)
	}

	snap := health.NewSnapshot()
	snap.CheckedAt = now.Add(-time.Minute)
	snap.Results = []health.Result{{Name: "cockpit-ups-wol-agent.service", OK: true, Critical: true}}
	writeHealthSnapshot(t, path, snap)
	if ok, reason := systemHealthSafe(path, 3*time.Minute, now); !ok {
		t.Fatalf("fresh critical-health snapshot rejected: %s", reason)
	}

	snap.CheckedAt = now.Add(-4 * time.Minute)
	writeHealthSnapshot(t, path, snap)
	if ok, reason := systemHealthSafe(path, 3*time.Minute, now); ok || !strings.Contains(reason, "stale") {
		t.Fatalf("stale snapshot accepted: ok=%v reason=%q", ok, reason)
	}

	snap.CheckedAt = now.Add(-time.Minute)
	snap.State = health.FailedSafe
	snap.Circuit.FailedReason = "nut-server.service repair attempts exhausted"
	writeHealthSnapshot(t, path, snap)
	if ok, reason := systemHealthSafe(path, 3*time.Minute, now); ok || !strings.Contains(reason, "FAILED_SAFE") {
		t.Fatalf("failed-safe snapshot accepted: ok=%v reason=%q", ok, reason)
	}
}

func TestSystemHealthSafeIgnoresNoncriticalCockpitFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "system-health.json")
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	snap := health.NewSnapshot()
	snap.CheckedAt = now.Add(-time.Minute)
	snap.State = health.Degraded
	snap.Results = []health.Result{
		{Name: "cockpit-ups-wol-agent.service", OK: true, Critical: true},
		{Name: "cockpit.socket", OK: false, Critical: false, Repairable: true, Message: "inactive"},
		{Name: "nut-monitor.service", OK: true, Critical: true},
	}
	writeHealthSnapshot(t, path, snap)

	if ok, reason := systemHealthSafe(path, 3*time.Minute, now); !ok {
		t.Fatalf("noncritical Cockpit failure blocked recovery: %s", reason)
	}
}

func TestSystemHealthSafeRejectsFailedCriticalCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "system-health.json")
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	snap := health.NewSnapshot()
	snap.CheckedAt = now.Add(-time.Minute)
	snap.State = health.Degraded
	snap.Results = []health.Result{
		{Name: "cockpit.socket", OK: false, Critical: false, Message: "inactive"},
		{Name: "nut-monitor.service", OK: false, Critical: true, Message: "failed"},
	}
	writeHealthSnapshot(t, path, snap)

	if ok, reason := systemHealthSafe(path, 3*time.Minute, now); ok || !strings.Contains(reason, "nut-monitor.service") {
		t.Fatalf("critical failure accepted: ok=%v reason=%q", ok, reason)
	}
}

func writeHealthSnapshot(t *testing.T, path string, snap health.Snapshot) {
	t.Helper()
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}
