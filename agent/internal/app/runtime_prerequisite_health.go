package app

import (
	"context"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
)

type runtimePrerequisiteCheck struct {
	hostID string
	reason string
}

func (c runtimePrerequisiteCheck) Name() string { return "host-prerequisite:" + c.hostID }

func (c runtimePrerequisiteCheck) Run(context.Context) health.Result {
	return health.Result{
		Name:       c.Name(),
		OK:         false,
		Critical:   false,
		Repairable: false,
		Message:    c.reason,
	}
}

func criticalAgentHealthSafe(snap health.Snapshot) bool {
	if snap.State == health.FailedSafe {
		return false
	}
	for _, result := range snap.Results {
		if result.Critical && !result.OK {
			return false
		}
	}
	return true
}
