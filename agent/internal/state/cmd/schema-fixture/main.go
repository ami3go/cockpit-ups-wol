package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

func main() {
	wasOnline := true
	lowBattery := false
	fsd := false
	charge := 95.0
	runtime := int64(1800)
	st := state.State{
		StateVersion:         state.Version,
		TransactionID:        "schema-fixture",
		ParentTransactionID:  "previous",
		Sequence:             1,
		PowerState:           state.RecoveryWait,
		ShutdownCommitted:    true,
		RecoveryStarted:      false,
		ActiveConfigRevision: "fixture-revision",
		LastUPS: &state.UPSObservation{
			Utility:               "ONLINE",
			LowBattery:            &lowBattery,
			FSD:                   &fsd,
			BatteryCharge:         &charge,
			BatteryRuntimeSeconds: &runtime,
			RawStatus:             "OL",
			ObservedAtWallclock:   "2026-01-01T00:00:00Z",
		},
		OutageElapsedSeconds: 42,
		Hosts: map[string]state.HostState{
			"node": {
				WasOnline:        &wasOnline,
				ShutdownState:    state.ShutdownCompleted,
				RecoveryState:    state.RecoveryOnline,
				ShutdownAttempts: 1,
				WakeAttempts:     1,
				LastActionID:     "wake:schema-fixture:node:1",
				LastVerification: "online",
			},
		},
		FailedSafeReason: "fixture",
		Checksum:         "0000000000000000000000000000000000000000000000000000000000000000",
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(st); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
