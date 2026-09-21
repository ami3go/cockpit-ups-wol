package ipc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
)

func TestUnixSocketGetHealth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	path := filepath.Join(t.TempDir(), "agent.sock")
	snap := health.NewSnapshot()
	snap.State = health.Healthy
	s := &Server{SocketPath: path, Handler: Handler{Health: fakeHealthProvider{snap: snap}}}
	done := make(chan error, 1)
	go func() { done <- s.ListenAndServe(ctx) }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		resp, err := Call(context.Background(), path, Request{ID: "health-1", Method: "GetHealth"})
		if err == nil {
			if !resp.OK {
				t.Fatalf("resp=%+v", resp)
			}
			info, statErr := os.Stat(path)
			if statErr != nil {
				t.Fatalf("stat socket: %v", statErr)
			}
			if got := info.Mode().Perm(); got != 0o600 {
				t.Fatalf("socket mode=%#o want 0600", got)
			}
			cancel()
			<-done
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("socket server did not become ready")
}
