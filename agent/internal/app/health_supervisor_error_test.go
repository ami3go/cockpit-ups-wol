package app

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
)

func TestRunAgentHealthConvertsSupervisorPersistenceErrorToCriticalDegraded(t *testing.T) {
	root := t.TempDir()
	blocking := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blocking, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	supervisor := &health.Supervisor{StatePath: filepath.Join(blocking, "health.json")}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	snap := runAgentHealth(context.Background(), supervisor, false, logger)
	if snap.State != health.Degraded || len(snap.Results) != 1 || snap.Results[0].Name != "health.supervisor" || !snap.Results[0].Critical || snap.Results[0].OK {
		t.Fatalf("unexpected degraded fallback: %+v", snap)
	}
	if logs.Len() == 0 {
		t.Fatal("supervisor persistence error was not logged")
	}
}
