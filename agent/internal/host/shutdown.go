package host

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

const knownHostsPath = "/etc/cockpit-ups-wol/known_hosts"

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
	Log    *slog.Logger
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
	logger := e.Log
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
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
		"-o", "UserKnownHostsFile=" + knownHostsPath,
		"-o", "GlobalKnownHostsFile=/dev/null",
		"-o", "IdentitiesOnly=yes",
		"-o", "ConnectTimeout=10",
		"-i", key,
		destination,
		"sudo", "-n", "/sbin/shutdown", "-h", "now",
	}
	logger.Info("requesting SSH host shutdown", "host_id", h.ID, "timeout_seconds", int(timeout/time.Second))
	out, err := runner.Run(runCtx, "ssh", args...)
	if err != nil {
		logger.Error("SSH host shutdown failed", "host_id", h.ID, "error", err)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return ShutdownResult{}, fmt.Errorf("SSH shutdown failed: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return ShutdownResult{}, fmt.Errorf("SSH shutdown failed: %w", err)
	}
	logger.Info("SSH host shutdown requested", "host_id", h.ID)
	return ShutdownResult{Disposition: ShutdownDirectRequested}, nil
}

// ValidateArmedCapabilities rejects configuration features that do not yet have
// a safe, tested v0.1 execution adapter. For SSH shutdown it also verifies the
// local prerequisites that must exist before an outage: a readable private key
// and an explicitly pinned host key in the project-owned known_hosts file.
func ValidateArmedCapabilities(cfg config.Config) error {
	for _, dep := range cfg.Dependencies {
		if dep.Status.Method == "arp" {
			return fmt.Errorf("network dependency %q uses unsupported armed status method arp", dep.ID)
		}
		if dep.Status.Method == "none" || dep.Address == nil || strings.TrimSpace(*dep.Address) == "" {
			return fmt.Errorf("network dependency %q cannot be verified in armed mode", dep.ID)
		}
		if err := validateEndpoint(*dep.Address); err != nil {
			return fmt.Errorf("network dependency %q: %w", dep.ID, err)
		}
		if dep.Startup == "wol" {
			return fmt.Errorf("network dependency %q uses startup=wol, which is disabled until dependency wake state is durable", dep.ID)
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
			if err := validateSSHLocalPrerequisites(*h.Address, *h.Shutdown.SSHKeyFile); err != nil {
				return fmt.Errorf("host %q SSH shutdown prerequisite: %w", h.ID, err)
			}
		case "command":
			return fmt.Errorf("host %q uses command shutdown, which is not available until the command registry is implemented", h.ID)
		default:
			return fmt.Errorf("host %q uses unsupported shutdown method %q", h.ID, h.Shutdown.Method)
		}
		if cfg.Recovery.Enabled && h.Wake.Enabled {
			if h.Address == nil || h.Status.Method == "none" {
				return fmt.Errorf("host %q wake is enabled but online state cannot be verified", h.ID)
			}
			if h.Wake.MAC == nil || strings.TrimSpace(*h.Wake.MAC) == "" {
				return fmt.Errorf("host %q wake is enabled without a MAC address", h.ID)
			}
			if h.Wake.Broadcast == nil || strings.TrimSpace(*h.Wake.Broadcast) == "" {
				return fmt.Errorf("host %q wake is enabled without an IPv4 broadcast address", h.ID)
			}
		}
	}
	return nil
}

func validateSSHLocalPrerequisites(address, keyPath string) error {
	keyPath = strings.TrimSpace(keyPath)
	info, err := os.Stat(keyPath)
	if err != nil {
		return fmt.Errorf("SSH key %s is not readable: %w", keyPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("SSH key %s is not a regular file", keyPath)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("SSH key %s must not be group/world writable", keyPath)
	}
	return validateKnownHost(address, knownHostsPath)
}

func validateKnownHost(address, path string) error {
	out, err := exec.Command("ssh-keygen", "-F", strings.TrimSpace(address), "-f", path).CombinedOutput()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("ssh-keygen is unavailable; install the OpenSSH client tools")
		}
		return fmt.Errorf("no pinned host key for %q in %s; enroll and verify the host key before arming", address, path)
	}
	return nil
}
