package health

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type SystemdUnitCheck struct {
	Runner   CommandRunner
	Unit     string
	Critical bool
	Timeout  time.Duration
}

func (c SystemdUnitCheck) Name() string { return "systemd:" + c.Unit }

func (c SystemdUnitCheck) Run(ctx context.Context) Result {
	r := Result{Name: c.Name(), Critical: c.Critical, Repairable: true}
	if c.Runner == nil {
		c.Runner = ExecRunner{}
	}
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	out, err := c.Runner.Run(ctx, "systemctl", "is-enabled", c.Unit)
	if err != nil || strings.TrimSpace(string(out)) != "enabled" {
		r.Message = "service is not enabled"
		return r
	}
	out, err = c.Runner.Run(ctx, "systemctl", "is-active", c.Unit)
	if err != nil || strings.TrimSpace(string(out)) != "active" {
		r.Message = "service is not active"
		return r
	}
	r.OK = true
	return r
}

type SystemdUnitRepair struct {
	Runner  CommandRunner
	Unit    string
	Timeout time.Duration
}

func (r SystemdUnitRepair) Name() string { return "systemd:" + r.Unit }

func (r SystemdUnitRepair) Repair(ctx context.Context) error {
	if r.Runner == nil {
		r.Runner = ExecRunner{}
	}
	if r.Timeout <= 0 {
		r.Timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	out, err := r.Runner.Run(ctx, "systemctl", "enable", "--now", r.Unit)
	if err != nil {
		return fmt.Errorf("systemctl enable --now %s: %w: %s", r.Unit, err, strings.TrimSpace(string(out)))
	}
	return nil
}
