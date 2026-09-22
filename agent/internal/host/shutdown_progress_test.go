package host

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

type blockingCommandRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r blockingCommandRunner) Run(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	close(r.started)
	select {
	case <-r.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestSSHShutdownReportsProgressWhileCommandIsRunning(t *testing.T) {
	address := "host.example"
	user := "ups-shutdown"
	key := "/tmp/test-key"
	started := make(chan struct{})
	release := make(chan struct{})
	var beats atomic.Int32
	exec := ShutdownExecutor{
		Runner: blockingCommandRunner{started: started, release: release},
		Progress: func() {
			beats.Add(1)
		},
		ProgressInterval: 20 * time.Millisecond,
	}
	h := config.HostConfig{
		ID:      "pc",
		Address: &address,
		Shutdown: config.ShutdownConfig{
			Method:         "ssh",
			SSHUser:        &user,
			SSHKeyFile:     &key,
			TimeoutSeconds: 2,
		},
	}
	done := make(chan error, 1)
	go func() {
		_, err := exec.Shutdown(context.Background(), h)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("SSH runner did not start")
	}
	time.Sleep(90 * time.Millisecond)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := beats.Load(); got < 4 {
		t.Fatalf("only %d progress beats while SSH command was live", got)
	}
}
