package app

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
)

// systemHealthSafe reports whether the separate cockpit-ups-wol-health timer
// has recently verified the safety-critical control-stack services. Missing,
// stale, malformed snapshots and failed critical checks fail closed for
// recovery. Non-critical management-plane failures remain visible and
// repairable without becoming a power-policy dependency.
func systemHealthSafe(path string, maxAge time.Duration, now time.Time) (bool, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Sprintf("system health snapshot unavailable: %v", err)
	}
	var snap health.Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return false, fmt.Sprintf("system health snapshot unreadable: %v", err)
	}
	if snap.Version != 1 {
		return false, fmt.Sprintf("unsupported system health snapshot version %d", snap.Version)
	}
	if snap.CheckedAt.IsZero() {
		return false, "system health snapshot has no checked_at timestamp"
	}
	if maxAge > 0 {
		age := now.Sub(snap.CheckedAt)
		if age < 0 {
			return false, "system health snapshot timestamp is in the future"
		}
		if age > maxAge {
			return false, fmt.Sprintf("system health snapshot stale (%s old)", age.Round(time.Second))
		}
	}
	if snap.State == health.FailedSafe {
		reason := string(snap.State)
		if snap.Circuit.FailedReason != "" {
			reason += ": " + snap.Circuit.FailedReason
		}
		return false, "system health " + reason
	}
	for _, result := range snap.Results {
		if result.Critical && !result.OK {
			reason := result.Name
			if result.Message != "" {
				reason += ": " + result.Message
			}
			return false, "critical control-stack check unhealthy: " + reason
		}
	}
	return true, "critical control-stack checks healthy"
}
