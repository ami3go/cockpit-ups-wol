package host

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

type ShutdownDisposition string

const (
	ShutdownDirectRequested ShutdownDisposition = "direct-requested"
	ShutdownManagedByNUT    ShutdownDisposition = "managed-by-nut"
	ShutdownNotRequired     ShutdownDisposition = "not-required"
)

type ShutdownResult struct {
	Disposition ShutdownDisposition
}

type ShutdownExecutor struct {
	Runner CommandRunner
}

func (e ShutdownExecutor) Shutdown(ctx context.Context, h config.HostConfig) (ShutdownResult, error) {
	switch h.Shutdown.Method {
	case "none":
		return ShutdownResult{Disposition: ShutdownNotRequired}, nil
	case "nut":
		// NUT secondaries are deliberately not sent a duplicate direct shutdown.
		return ShutdownResult{Disposition: ShutdownManagedByNUT}, nil
	case "command":
		return ShutdownResult{}, errors.New("command shutdown is disabled until an explicit command registry is implemented")
	case "ssh":
		return e.shutdownSSH(ctx, h)
	default:
		return ShutdownResult{}, fmt.Errorf("unsupported shutdown method %q", h.Shutdown.Method)
	}
}

func (e ShutdownExecutor) shutdownSSH(ctx context.Context, h config.HostConfig) (ShutdownResult, error) {
	if h.Address == nil {
		return ShutdownResult{}, errors.New("SSH shutdown requires host address")
	}
	if err := validateEndpoint(*h.Address); err != nil {
		return ShutdownResult{}, err
	}
	if h.Shutdown.SSHUser == nil || h.Shutdown.SSHKeyFile == nil {
		return ShutdownResult{}, errors.New("SSH shutdown requires ssh_user and ssh_key_file")
	}
	user := strings.TrimSpace(*h.Shutdown.SSHUser)
	key := strings.TrimSpace(*h.Shutdown.SSHKeyFile)
	if user == "" || strings.HasPrefix(user, "-") || strings.ContainsAny(user, "@ \t\r\n") {
		return ShutdownResult{}, errors.New("unsafe SSH user")
	}
	if key == "" || strings.HasPrefix(key, "-") || strings.ContainsAny(key, "\r\n") {
		return ShutdownResult{}, errors.New("unsafe SSH key path")
	}
	runner := e.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	timeout := time.Duration(h.Shutdown.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	destination := user + "@" + *h.Address
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=yes",
		"-o", "ConnectTimeout=10",
		"-i", key,
		destination,
		"sudo", "-n", "/sbin/shutdown", "-h", "now",
	}
	out, err := runner.Run(runCtx, "ssh", args...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return ShutdownResult{}, fmt.Errorf("SSH shutdown failed: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return ShutdownResult{}, fmt.Errorf("SSH shutdown failed: %w", err)
	}
	return ShutdownResult{Disposition: ShutdownDirectRequested}, nil
}

// ValidateArmedCapabilities rejects configuration features that do not yet have
// a safe, tested v0.1 execution adapter.
func ValidateArmedCapabilities(cfg config.Config) error {
	for _, dep := range cfg.Dependencies {
		if dep.Status.Method == "arp" {
			return fmt.Errorf("network dependency %q uses unsupported armed status method arp", dep.ID)
		}
	}
	for _, h := range cfg.Hosts {
		if h.Status.Method == "arp" {
			return fmt.Errorf("host %q uses unsupported armed status method arp", h.ID)
		}
		switch h.Shutdown.Method {
		case "none", "nut":
		case "ssh":
			if h.Address == nil {
				return fmt.Errorf("host %q SSH shutdown has no address", h.ID)
			}
			if err := validateEndpoint(*h.Address); err != nil {
				return fmt.Errorf("host %q: %w", h.ID, err)
			}
			if h.Shutdown.SSHUser == nil || h.Shutdown.SSHKeyFile == nil {
				return fmt.Errorf("host %q SSH shutdown is incomplete", h.ID)
			}
		case "command":
			return fmt.Errorf("host %q uses command shutdown, which is not available until the command registry is implemented", h.ID)
		default:
			return fmt.Errorf("host %q uses unsupported shutdown method %q", h.ID, h.Shutdown.Method)
		}
	}
	return nil
}
