package orchestrator

import (
	"context"
	"fmt"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/host"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
)

type HostProbe struct{ Checker host.StatusChecker }

func (p HostProbe) Check(ctx context.Context, h config.HostConfig) (host.ProbeResult, error) {
	if h.Address == nil {
		return host.ProbeResult{Known: false}, nil
	}
	return p.Checker.Check(ctx, *h.Address, h.Status)
}
func (p HostProbe) Wait(ctx context.Context, h config.HostConfig, want bool) error {
	if h.Address == nil {
		return fmt.Errorf("host %s has no address for verification", h.ID)
	}
	return p.Checker.WaitForConsecutive(ctx, *h.Address, h.Status, want)
}

type HostShutdown struct{ Executor host.ShutdownExecutor }

func (s HostShutdown) Shutdown(ctx context.Context, h config.HostConfig) (host.ShutdownResult, error) {
	return s.Executor.Shutdown(ctx, h)
}

type NUTFSD struct{ Client *nut.Client }

func (n NUTFSD) RequestFSD(ctx context.Context, upsName, path string) error {
	if n.Client == nil {
		return fmt.Errorf("NUT client is nil")
	}
	return n.Client.RequestFSD(ctx, upsName, path)
}
