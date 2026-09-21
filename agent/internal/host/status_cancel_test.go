package host

import (
	"context"
	"errors"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

type contextRunner struct{}

func (contextRunner) Run(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestPingParentCancellationIsNotOfflineEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := (StatusChecker{Runner: contextRunner{}}).Check(ctx, "host.lan", config.StatusConfig{
		Method:    "ping",
		TimeoutMS: 500,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if result.Known {
		t.Fatalf("cancelled probe was reported as trustworthy evidence: %+v", result)
	}
}
