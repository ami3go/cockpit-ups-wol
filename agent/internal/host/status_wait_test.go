package host

import (
	"context"
	"errors"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

type waitRunner struct{ err error }

func (r waitRunner) Run(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, r.err
}

func TestWaitForConsecutiveReturnsAfterRequiredKnownSample(t *testing.T) {
	checker := StatusChecker{Runner: waitRunner{}}
	cfg := config.StatusConfig{Method: "ping", TimeoutMS: 10, SuccessConsecutive: 1}
	if err := checker.WaitForConsecutive(context.Background(), "127.0.0.1", cfg, true); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForConsecutivePropagatesParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checker := StatusChecker{Runner: waitRunner{}}
	cfg := config.StatusConfig{Method: "ping", TimeoutMS: 10, SuccessConsecutive: 1}
	if err := checker.WaitForConsecutive(ctx, "127.0.0.1", cfg, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
