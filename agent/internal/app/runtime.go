package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/ipc"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	systemd "github.com/ami3go/cockpit-ups-wol/agent/internal/system"
)

// NUTQuerier is the minimal UPS-status dependency used by the runtime.
type NUTQuerier interface {
	Query(context.Context, string) (nut.Status, error)
}

// NUTController covers both status queries and the destructive FSD request.
// Keeping both operations on the injected dependency prevents tests from
// silently falling back to a real upsmon process.
type NUTController interface {
	NUTQuerier
	RequestFSD(context.Context, string, string) error
}

// Options contains runtime paths and injectable platform hooks. The hooks make
// startup, logging, and watchdog behavior testable without a running systemd
// instance.
type Options struct {
	SocketPath            string
	HealthStatePath       string
	SystemHealthStatePath string
	SystemHealthMaxAge    time.Duration
	HealthInterval        time.Duration
	PowerInterval         time.Duration
	StateDir              string
	ConfigHistoryDir      string
	UPSMonConfPath        string
	NUT                   NUTController
	Log                   *slog.Logger
	Ready                 func() error
	Stopping              func() error
	StartWatchdog         func(context.Context, <-chan struct{}, time.Duration) (<-chan error, error)
}

func (o *Options) defaults() {
	if o.SocketPath == "" {
		o.SocketPath = "/run/cockpit-ups-wol/agent.sock"
	}
	if o.HealthStatePath == "" {
		o.HealthStatePath = "/var/lib/cockpit-ups-wol/health.json"
	}
	if o.SystemHealthStatePath == "" {
		o.SystemHealthStatePath = "/var/lib/cockpit-ups-wol/system-health.json"
	}
	if o.SystemHealthMaxAge <= 0 {
		o.SystemHealthMaxAge = 3 * time.Minute
	}
	if o.StateDir == "" {
		o.StateDir = "/var/lib/cockpit-ups-wol/state"
	}
	if o.ConfigHistoryDir == "" {
		o.ConfigHistoryDir = "/var/lib/cockpit-ups-wol/config-history"
	}
	if o.UPSMonConfPath == "" {
		o.UPSMonConfPath = "/etc/nut/upsmon.conf"
	}
	if o.PowerInterval <= 0 {
		o.PowerInterval = 5 * time.Second
	}
	if o.NUT == nil {
		o.NUT = nut.NewClient()
	}
	if o.Log == nil {
		o.Log = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
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

// LoadAndRun reconciles any interrupted configuration transaction before it
// parses the canonical YAML and starts the runtime. A candidate copied into the
// active path but not promoted to known-good must never execute after reboot.
func LoadAndRun(ctx context.Context, configPath string, opts Options) error {
	opts.defaults()
	manager := &config.Manager{
		HistoryDir:       opts.ConfigHistoryDir,
		ActiveConfigPath: configPath,
	}
	if err := manager.RecoverForStartup(ctx, nil); err != nil {
		return fmt.Errorf("recover configuration: %w", err)
	}

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

// Run starts the configured runtime. monitor/dry-run/maintenance stay strictly
// non-destructive. armed mode is routed through the durable orchestration path.
func Run(ctx context.Context, cfg config.Config, opts Options) error {
	opts.defaults()
	if cfg.Mode == "armed" {
		return runArmed(ctx, cfg, opts)
	}
	return runPassive(ctx, cfg, opts)
}

func runPassive(ctx context.Context, cfg config.Config, opts Options) error {
	interval := opts.HealthInterval
	if interval <= 0 {
		interval = time.Duration(cfg.Health.IntervalSeconds) * time.Second
	}
	if interval <= 0 {
		interval = 60 * time.Second
	}

	target := nut.Target(cfg.NUT.UPSName, cfg.NUT.Host, cfg.NUT.Port)
	supervisor := &health.Supervisor{
		Checks:            []health.Check{nutHealthCheck{client: opts.NUT, target: target}},
		StatePath:         opts.HealthStatePath,
		MaxRepairAttempts: cfg.Health.MaxRepairAttempts,
	}
	initial, err := supervisor.Run(ctx, false)
	if err != nil {
		return fmt.Errorf("initial health check: %w", err)
	}
	opts.Log.Info("agent runtime initialized", "mode", cfg.Mode, "health_state", initial.State)

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

	beat := make(chan struct{}, 1)
	watchdogCh, err := opts.StartWatchdog(ctx, beat, 20*time.Second)
	if err != nil {
		return fmt.Errorf("start systemd watchdog: %w", err)
	}
	beat <- struct{}{}

	healthTicker := time.NewTicker(interval)
	defer healthTicker.Stop()
	// Passive mode may perform health checks only once per minute; this ticker
	// proves that the select loop itself is still being scheduled between checks.
	livenessTicker := time.NewTicker(5 * time.Second)
	defer livenessTicker.Stop()

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
		case <-livenessTicker.C:
			nonBlockingBeat(beat)
		case <-healthTicker.C:
			snap, err := supervisor.Run(ctx, cfg.Health.Autofix)
			if err != nil {
				opts.Log.Error("health supervisor run failed", "error", err)
				return fmt.Errorf("health supervisor: %w", err)
			}
			nonBlockingBeat(beat)
			if snap.State != health.Healthy {
				opts.Log.Warn("agent health degraded", "health_state", snap.State)
			}
		}
	}
}

func nonBlockingBeat(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
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
