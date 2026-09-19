package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/ipc"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	systemd "github.com/ami3go/cockpit-ups-wol/agent/internal/system"
)

// NUTQuerier is the minimal UPS-status dependency used by the safe runtime.
type NUTQuerier interface {
	Query(context.Context, string) (nut.Status, error)
}

// Options contains runtime paths and injectable platform hooks. The hooks make
// startup and watchdog behavior testable without a running systemd instance.
type Options struct {
	SocketPath      string
	HealthStatePath string
	HealthInterval  time.Duration
	NUT             NUTQuerier
	Ready           func() error
	Stopping        func() error
	StartWatchdog   func(context.Context) (<-chan error, error)
}

func (o *Options) defaults() {
	if o.SocketPath == "" {
		o.SocketPath = "/run/cockpit-ups-wol/agent.sock"
	}
	if o.HealthStatePath == "" {
		o.HealthStatePath = "/var/lib/cockpit-ups-wol/health.json"
	}
	if o.NUT == nil {
		o.NUT = nut.NewClient()
	}
	if o.Ready == nil {
		o.Ready = systemd.Ready
	}
	if o.Stopping == nil {
		o.Stopping = systemd.Stopping
	}
	if o.StartWatchdog == nil {
		o.StartWatchdog = systemd.StartWatchdog
	}
}

// LoadAndRun parses the canonical YAML configuration and starts the safe
// monitoring runtime. Destructive automation remains deliberately disabled
// until host action adapters and the full orchestration loop are integrated.
func LoadAndRun(ctx context.Context, configPath string, opts Options) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	return Run(ctx, cfg, opts)
}

// Run starts the non-destructive v0.1 runtime shell. It stays alive in
// monitor/dry-run/maintenance mode, feeds systemd watchdog, exposes health over
// IPC, and continuously reports NUT communication health. Armed mode fails
// closed until destructive host adapters are implemented and acceptance-tested.
func Run(ctx context.Context, cfg config.Config, opts Options) error {
	if cfg.Mode == "armed" {
		return errors.New("armed mode is not available yet: destructive host adapters are not implemented")
	}
	opts.defaults()
	interval := opts.HealthInterval
	if interval <= 0 {
		interval = time.Duration(cfg.Health.IntervalSeconds) * time.Second
	}
	if interval <= 0 {
		interval = 60 * time.Second
	}

	target := nut.Target(cfg.NUT.UPSName, cfg.NUT.Host, cfg.NUT.Port)
	supervisor := &health.Supervisor{
		Checks: []health.Check{nutHealthCheck{client: opts.NUT, target: target}},
		StatePath:         opts.HealthStatePath,
		MaxRepairAttempts: cfg.Health.MaxRepairAttempts,
	}
	if _, err := supervisor.Run(ctx, false); err != nil {
		return fmt.Errorf("initial health check: %w", err)
	}

	serverCtx, cancelServer := context.WithCancel(ctx)
	defer cancelServer()
	serverErr := make(chan error, 1)
	server := &ipc.Server{
		Handler:    ipc.Handler{Health: supervisor},
		SocketPath: opts.SocketPath,
		SocketMode: 0o660,
	}
	go func() {
		serverErr <- server.ListenAndServe(serverCtx)
	}()

	if err := opts.Ready(); err != nil {
		return fmt.Errorf("systemd READY notification: %w", err)
	}
	defer func() { _ = opts.Stopping() }()

	watchdogCh, err := opts.StartWatchdog(ctx)
	if err != nil {
		return fmt.Errorf("start systemd watchdog: %w", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-serverErr:
			if err != nil && ctx.Err() == nil {
				return fmt.Errorf("IPC server: %w", err)
			}
			serverErr = nil
		case err, ok := <-watchdogCh:
			if !ok {
				watchdogCh = nil
				continue
			}
			if err != nil {
				return fmt.Errorf("systemd watchdog: %w", err)
			}
		case <-ticker.C:
			if _, err := supervisor.Run(ctx, cfg.Health.Autofix); err != nil {
				return fmt.Errorf("health supervisor: %w", err)
			}
		}
	}
}

type nutHealthCheck struct {
	client NUTQuerier
	target string
}

func (c nutHealthCheck) Name() string { return "nut.query" }

func (c nutHealthCheck) Run(ctx context.Context) health.Result {
	st, err := c.client.Query(ctx, c.target)
	if err != nil {
		return health.Result{Name: c.Name(), OK: false, Critical: true, Repairable: false, Message: err.Error()}
	}
	if st.Utility == nut.UtilityUnknown {
		return health.Result{Name: c.Name(), OK: false, Critical: true, Repairable: false, Message: "UPS utility state is UNKNOWN"}
	}
	return health.Result{Name: c.Name(), OK: true, Critical: true, Message: "UPS status available: " + string(st.Utility)}
}
