package host

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

// ProbeResult separates a trustworthy offline observation from a status method
// that intentionally provides no information.
type ProbeResult struct {
	Known  bool
	Online bool
}

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type StatusChecker struct {
	Runner CommandRunner
}

func (c StatusChecker) Check(ctx context.Context, address string, cfg config.StatusConfig) (ProbeResult, error) {
	method := cfg.Method
	if method == "auto" {
		if cfg.Port != nil {
			method = "tcp"
		} else {
			method = "ping"
		}
	}
	if method == "none" {
		return ProbeResult{Known: false}, nil
	}
	if err := validateEndpoint(address); err != nil {
		return ProbeResult{}, err
	}
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Second
	}
	switch method {
	case "tcp":
		if cfg.Port == nil || *cfg.Port < 1 || *cfg.Port > 65535 {
			return ProbeResult{}, errors.New("TCP status requires a valid port")
		}
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(address, strconv.Itoa(*cfg.Port)))
		if err != nil {
			if ctx.Err() != nil {
				return ProbeResult{}, ctx.Err()
			}
			return ProbeResult{Known: true, Online: false}, nil
		}
		_ = conn.Close()
		return ProbeResult{Known: true, Online: true}, nil
	case "ping":
		runner := c.Runner
		if runner == nil {
			runner = ExecRunner{}
		}
		probeCtx, cancel := context.WithTimeout(ctx, timeout+time.Second)
		defer cancel()
		seconds := int(math.Ceil(timeout.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		_, err := runner.Run(probeCtx, "ping", "-n", "-c", "1", "-W", strconv.Itoa(seconds), address)
		if err == nil {
			return ProbeResult{Known: true, Online: true}, nil
		}
		// Cancellation of the caller is not an observation about the host.
		// Check it before ExitError because exec.CommandContext may report a
		// killed child as an ExitError when the parent context is cancelled.
		if ctx.Err() != nil {
			return ProbeResult{}, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return ProbeResult{Known: true, Online: false}, nil
		}
		if errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
			return ProbeResult{Known: true, Online: false}, nil
		}
		return ProbeResult{}, fmt.Errorf("ping probe: %w", err)
	case "arp":
		return ProbeResult{}, errors.New("ARP status is not implemented for armed mode")
	default:
		return ProbeResult{}, fmt.Errorf("unsupported status method %q", method)
	}
}

// WaitForConsecutive requires the configured number of consistent known
// observations before declaring a host online/offline.
func (c StatusChecker) WaitForConsecutive(ctx context.Context, address string, cfg config.StatusConfig, wantOnline bool) error {
	need := cfg.SuccessConsecutive
	if need <= 0 {
		need = 3
	}
	interval := time.Duration(cfg.ProbeIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	streak := 0
	for {
		result, err := c.Check(ctx, address, cfg)
		if err != nil {
			return err
		}
		if result.Known && result.Online == wantOnline {
			streak++
			if streak >= need {
				return nil
			}
		} else {
			streak = 0
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func validateEndpoint(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return errors.New("host address is required")
	}
	if strings.HasPrefix(v, "-") || strings.ContainsAny(v, " \t\r\n") {
		return errors.New("host address contains unsafe characters")
	}
	return nil
}
