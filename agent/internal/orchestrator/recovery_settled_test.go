package orchestrator

import (
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/host"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

func TestRecoverySettledEmptyFleet(t *testing.T) {
	if !recoverySettled(nil, nil) {
		t.Fatal("empty fleet must be settled")
	}
}

func TestRecoverySettledEligibleWaitingHost(t *testing.T) {
	hosts := []host.Config{{ID: "node", WakeEnabled: true, RestorePolicy: host.RestoreAlways}}
	states := map[string]state.HostState{"node": {RecoveryState: state.RecoveryWaiting}}
	if recoverySettled(hosts, states) {
		t.Fatal("waiting restore-eligible host must keep recovery unsettled")
	}
}

func TestRecoverySettledTerminalEligibleHosts(t *testing.T) {
	hosts := []host.Config{
		{ID: "online", WakeEnabled: true, RestorePolicy: host.RestoreAlways},
		{ID: "failed", WakeEnabled: true, RestorePolicy: host.RestoreAlways},
	}
	states := map[string]state.HostState{
		"online": {RecoveryState: state.RecoveryOnline},
		"failed": {RecoveryState: state.RecoveryFailed},
	}
	if !recoverySettled(hosts, states) {
		t.Fatal("terminal restore-eligible hosts must be settled")
	}
}
