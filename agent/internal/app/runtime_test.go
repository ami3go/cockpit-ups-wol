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
		NUT: config.NUTConfig{UPSName: "ups", Host: "localhost", Port: 3493},
		Health: config.HealthConfig{Enabled: true, IntervalSeconds: 60, MaxRepairAttempts: 5},
	}
}

func TestRunRefusesArmedMode(t *testing.T) {
	err := Run(context.Background(), baseConfig("armed"), Options{})
	if err == nil {
		t.Fatal("expected armed mode to fail closed")
	}
}

func TestRunServesHealthAndStopsCleanly(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "agent.sock")
	healthState := filepath.Join(dir, "health.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	closedWatchdog := func(context.Context) (<-chan error, error) {
		ch := make(chan error)
		close(ch)
		return ch, nil
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, baseConfig("dry-run"), Options{
			SocketPath:      socket,
			HealthStatePath: healthState,
			HealthInterval:  10 * time.Millisecond,
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
