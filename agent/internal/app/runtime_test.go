package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/ipc"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
)

type fakeNUT struct {
	status nut.Status
	err    error
}

func (f fakeNUT) Query(context.Context, string) (nut.Status, error) { return f.status, f.err }

func baseConfig(mode string) config.Config {
	return config.Config{
		ConfigVersion: 1,
		Mode:          mode,
		NUT: config.NUTConfig{Profile: "local-server", UPSName: "ups", Host: "localhost", Port: 3493},
		Controller: config.ControllerConfig{RequireUPSBackedPower: true, RequireAutoPowerOn: true},
		Recovery: config.RecoveryConfig{Enabled: true, UtilityStableSeconds: 1, BatteryChargeMin: intPtr(80)},
		Health: config.HealthConfig{Enabled: true, IntervalSeconds: 60, MaxRepairAttempts: 5},
	}
}

func intPtr(v int) *int { return &v }

func closedWatchdog(context.Context) (<-chan error, error) {
	ch := make(chan error)
	close(ch)
	return ch, nil
}

func TestRunRefusesUnsupportedArmedCapability(t *testing.T) {
	cfg := baseConfig("armed")
	addr := "192.0.2.1"
	cfg.Dependencies = []config.DependencyConfig{{
		ID:      "switch",
		Address: &addr,
		Startup: "wol",
		Status:  config.StatusConfig{Method: "ping", TimeoutMS: 1000, SuccessConsecutive: 1, ProbeIntervalSeconds: 1},
		Wake:    config.WakeConfig{Enabled: true, MAC: strPtr("00:11:22:33:44:55"), Broadcast: strPtr("192.0.2.255"), Port: 9, MaxAttempts: 1},
	}}
	err := Run(context.Background(), cfg, Options{})
	if err == nil {
		t.Fatal("expected unsupported armed dependency wake to fail closed")
	}
}

func strPtr(v string) *string { return &v }

func TestRunServesHealthAndStopsCleanly(t *testing.T) {
	testRuntimeStartsAndStops(t, "dry-run")
}

func TestRunArmedServesHealthAndStopsCleanly(t *testing.T) {
	testRuntimeStartsAndStops(t, "armed")
}

func testRuntimeStartsAndStops(t *testing.T, mode string) {
	t.Helper()
	dir := t.TempDir()
	socket := filepath.Join(dir, "agent.sock")
	healthState := filepath.Join(dir, "health.json")
	stateDir := filepath.Join(dir, "state")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, baseConfig(mode), Options{
			SocketPath:      socket,
			HealthStatePath: healthState,
			StateDir:        stateDir,
			HealthInterval:  10 * time.Millisecond,
			PowerInterval:   10 * time.Millisecond,
			NUT:             fakeNUT{status: nut.Status{Utility: nut.UtilityOnline}},
			Ready:           func() error { return nil },
			Stopping:        func() error { return nil },
			StartWatchdog:   closedWatchdog,
		})
	}()

	var resp ipc.Response
	var err error
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		callCtx, callCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		resp, err = ipc.Call(callCtx, socket, ipc.Request{ID: "test", Method: "GetHealth"})
		callCancel()
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("health IPC did not become available: %v", err)
	}
	if !resp.OK {
		t.Fatalf("health response not OK: %+v", resp)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("runtime returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop after cancellation")
	}
}
