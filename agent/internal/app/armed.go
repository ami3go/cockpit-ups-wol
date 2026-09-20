package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/host"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/ipc"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/orchestrator"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/policy"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/wol"
)

func runArmed(ctx context.Context, cfg config.Config, opts Options) error {
	if err := host.ValidateArmedCapabilities(cfg); err != nil {
		return fmt.Errorf("armed capability validation: %w", err)
	}

	target := nut.Target(cfg.NUT.UPSName, cfg.NUT.Host, cfg.NUT.Port)
	supervisor := &health.Supervisor{
		Checks:            []health.Check{nutHealthCheck{client: opts.NUT, target: target}},
		StatePath:         opts.HealthStatePath,
		MaxRepairAttempts: cfg.Health.MaxRepairAttempts,
	}
	latestHealth, err := supervisor.Run(ctx, false)
	if err != nil {
		return fmt.Errorf("initial health check: %w", err)
	}

	store := state.NewStore(opts.StateDir)
	revision, err := configRevision(cfg)
	if err != nil {
		return err
	}
	persisted, _, err := store.Load()
	if err != nil {
		if !errors.Is(err, state.ErrNoValidState) {
			return fmt.Errorf("load durable power state: %w", err)
		}
		persisted = state.New(newTransactionID(), revision)
		persisted, err = store.Write(persisted)
		if err != nil {
			return fmt.Errorf("initialize durable power state: %w", err)
		}
	} else if persisted.ActiveConfigRevision != revision {
		if persisted.ShutdownCommitted || persisted.RecoveryStarted || (persisted.PowerState != state.Normal && persisted.PowerState != state.BootReconcile) {
			return fmt.Errorf("active power transaction belongs to config revision %q, current revision is %q", persisted.ActiveConfigRevision, revision)
		}
		persisted.ParentTransactionID = persisted.TransactionID
		persisted.TransactionID = newTransactionID()
		persisted.ActiveConfigRevision = revision
		persisted.Hosts = map[string]state.HostState{}
		persisted.PowerState = state.BootReconcile
		persisted, err = store.Write(persisted)
		if err != nil {
			return fmt.Errorf("reconcile config revision into power state: %w", err)
		}
	}

	resumePowerState := persisted.PowerState
	engine := policy.New(policyFromConfig(cfg), persisted)
	coord := policy.NewCoordinator(engine, store)
	// Once shutdown has committed, a daemon/controller restart must resume the
	// destructive transaction rather than restarting the outage grace period.
	if persisted.ShutdownCommitted && (resumePowerState == state.ShutdownCommitted || resumePowerState == state.ShutdownInProgress) {
		persisted.PowerState = state.ShutdownInProgress
		if _, err := coord.Write(persisted); err != nil {
			return fmt.Errorf("resume committed shutdown: %w", err)
		}
	}

	probe := armedProber{checker: host.StatusChecker{}}
	byID := make(map[string]config.HostConfig, len(cfg.Hosts))
	for _, h := range cfg.Hosts {
		byID[h.ID] = h
	}
	fsdClient := nut.NewClient()
	if realClient, ok := opts.NUT.(*nut.Client); ok {
		fsdClient = realClient
	}
	recovery := host.RecoveryExecutor{
		Waker:   armedWaker{hosts: byID, sender: wol.Sender{}},
		Checker: armedRecoveryChecker{hosts: byID, checker: host.StatusChecker{}},
		Store:   coord,
	}
	controller := &orchestrator.Controller{
		Config:           cfg,
		Policy:           coord,
		Probe:            probe,
		Shutdown:         host.ShutdownExecutor{},
		FSD:              fsdClient,
		Recovery:         recovery,
		UPSMonConfPath:   opts.UPSMonConfPath,
		NewTransactionID: newTransactionID,
	}
	deps := newDependencyTracker(cfg.Dependencies)

	serverCtx, cancelServer := context.WithCancel(ctx)
	defer cancelServer()
	serverErr := make(chan error, 1)
	server := &ipc.Server{
		Handler:    ipc.Handler{Health: supervisor},
		SocketPath: opts.SocketPath,
		SocketMode: 0o660,
	}
	go func() { serverErr <- server.ListenAndServe(serverCtx) }()

	// Reconcile persisted state once before announcing READY. UNKNOWN NUT state
	// is safe: policy remains in BOOT_RECONCILE and performs no destructive work.
	if err := armedPowerTick(ctx, cfg, opts, controller, deps, latestHealth, fsdClient); err != nil {
		return err
	}
	if err := opts.Ready(); err != nil {
		return fmt.Errorf("systemd READY notification: %w", err)
	}
	defer func() { _ = opts.Stopping() }()

	watchdogCh, err := opts.StartWatchdog(ctx)
	if err != nil {
		return fmt.Errorf("start systemd watchdog: %w", err)
	}
	healthInterval := opts.HealthInterval
	if healthInterval <= 0 {
		healthInterval = time.Duration(cfg.Health.IntervalSeconds) * time.Second
	}
	if healthInterval <= 0 {
		healthInterval = 60 * time.Second
	}
	healthTicker := time.NewTicker(healthInterval)
	defer healthTicker.Stop()
	powerTicker := time.NewTicker(opts.PowerInterval)
	defer powerTicker.Stop()

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
		case <-healthTicker.C:
			latestHealth, err = supervisor.Run(ctx, cfg.Health.Autofix)
			if err != nil {
				return fmt.Errorf("health supervisor: %w", err)
			}
		case <-powerTicker.C:
			if err := armedPowerTick(ctx, cfg, opts, controller, deps, latestHealth, fsdClient); err != nil {
				return err
			}
		}
	}
}

func armedPowerTick(ctx context.Context, cfg config.Config, opts Options, controller *orchestrator.Controller, deps *dependencyTracker, latestHealth health.Snapshot, fsdClient *nut.Client) error {
	upsStatus, err := opts.NUT.Query(ctx, nut.Target(cfg.NUT.UPSName, cfg.NUT.Host, cfg.NUT.Port))
	if err != nil {
		upsStatus = nut.Status{Utility: nut.UtilityUnknown}
	}
	networkReady := deps.Ready(ctx)
	_, err = controller.Tick(ctx, time.Now(), policy.Inputs{
		UPS:          upsStatus,
		NetworkReady: networkReady,
		HealthSafe:   latestHealth.State == health.Healthy,
	})
	if err != nil {
		return fmt.Errorf("armed orchestration: %w", err)
	}

	// If there are no NUT-managed hosts, the controller still must be shut down
	// through its primary upsmon after all direct hosts have settled. The durable
	// shutdown commit is the intent record; retrying FSD after a daemon restart is
	// therefore safe and idempotent at the transaction level.
	st := controller.Policy.State()
	if st.ShutdownCommitted && st.PowerState == state.WaitingForAC && upsStatus.Utility == nut.UtilityOnBattery && cfg.NUT.Profile != "remote-client" && !hasNUTManagedHosts(cfg.Hosts) {
		if err := fsdClient.RequestFSD(ctx, cfg.NUT.UPSName, opts.UPSMonConfPath); err != nil {
			return fmt.Errorf("request controller FSD: %w", err)
		}
	}
	return nil
}

func hasNUTManagedHosts(hosts []config.HostConfig) bool {
	for _, h := range hosts {
		if h.Shutdown.Method == "nut" {
			return true
		}
	}
	return false
}

type armedProber struct{ checker host.StatusChecker }

func (p armedProber) Check(ctx context.Context, h config.HostConfig) (host.ProbeResult, error) {
	if h.Address == nil {
		return host.ProbeResult{Known: false}, nil
	}
	return p.checker.Check(ctx, *h.Address, h.Status)
}

func (p armedProber) Wait(ctx context.Context, h config.HostConfig, wantOnline bool) error {
	if h.Address == nil {
		return errors.New("host address is required for verified state transition")
	}
	return p.checker.WaitForConsecutive(ctx, *h.Address, h.Status, wantOnline)
}

type armedWaker struct {
	hosts  map[string]config.HostConfig
	sender wol.Sender
}

func (w armedWaker) Wake(ctx context.Context, hostID string) error {
	h, ok := w.hosts[hostID]
	if !ok {
		return fmt.Errorf("unknown wake host %q", hostID)
	}
	if h.Wake.MAC == nil || h.Wake.Broadcast == nil {
		return fmt.Errorf("host %q has incomplete wake configuration", hostID)
	}
	req := wol.Request{MAC: *h.Wake.MAC, Broadcast: *h.Wake.Broadcast, Port: h.Wake.Port}
	if h.Wake.Interface != nil {
		req.Interface = *h.Wake.Interface
	}
	return w.sender.Wake(ctx, req)
}

type armedRecoveryChecker struct {
	hosts   map[string]config.HostConfig
	checker host.StatusChecker
}

func (c armedRecoveryChecker) Online(ctx context.Context, hostID string) (bool, error) {
	h, ok := c.hosts[hostID]
	if !ok || h.Address == nil {
		return false, fmt.Errorf("host %q has no verifiable address", hostID)
	}
	need := h.Status.SuccessConsecutive
	if need <= 0 {
		need = 3
	}
	probeTimeout := time.Duration(h.Status.TimeoutMS) * time.Millisecond
	if probeTimeout <= 0 {
		probeTimeout = time.Second
	}
	interval := time.Duration(h.Status.ProbeIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	total := time.Duration(need)*probeTimeout + time.Duration(need)*interval
	checkCtx, cancel := context.WithTimeout(ctx, total)
	defer cancel()
	if err := c.checker.WaitForConsecutive(checkCtx, *h.Address, h.Status, true); err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

type dependencyTracker struct {
	deps    []config.DependencyConfig
	checker host.StatusChecker
	streaks map[string]int
}

func newDependencyTracker(deps []config.DependencyConfig) *dependencyTracker {
	return &dependencyTracker{deps: deps, streaks: map[string]int{}}
}

func (t *dependencyTracker) Ready(ctx context.Context) bool {
	if len(t.deps) == 0 {
		return true
	}
	allReady := true
	for _, dep := range t.deps {
		if dep.Address == nil {
			return false
		}
		result, err := t.checker.Check(ctx, *dep.Address, dep.Status)
		if err != nil || !result.Known || !result.Online {
			t.streaks[dep.ID] = 0
			allReady = false
			continue
		}
		t.streaks[dep.ID]++
		need := dep.Status.SuccessConsecutive
		if need <= 0 {
			need = 3
		}
		if t.streaks[dep.ID] < need {
			allReady = false
		}
	}
	return allReady
}

func policyFromConfig(cfg config.Config) policy.Config {
	recoveryEnabled := cfg.Recovery.Enabled
	out := policy.Config{
		GracePeriod:            time.Duration(cfg.Outage.GracePeriodSeconds) * time.Second,
		CommunicationLossGrace: time.Duration(cfg.Outage.CommunicationLossGraceSeconds) * time.Second,
		RecoveryEnabled:        &recoveryEnabled,
		UtilityStable:          time.Duration(cfg.Recovery.UtilityStableSeconds) * time.Second,
		RecoveryNetworkWait:    time.Duration(cfg.Recovery.NetworkWaitSeconds) * time.Second,
	}
	if cfg.Outage.MaxOnBatterySeconds != nil {
		out.MaxOnBattery = time.Duration(*cfg.Outage.MaxOnBatterySeconds) * time.Second
	}
	if cfg.Outage.CriticalBatteryPercent != nil {
		v := float64(*cfg.Outage.CriticalBatteryPercent)
		out.CriticalCharge = &v
	}
	if cfg.Outage.CriticalRuntimeSeconds != nil {
		out.CriticalRuntime = time.Duration(*cfg.Outage.CriticalRuntimeSeconds) * time.Second
	}
	if cfg.Recovery.BatteryChargeMin != nil {
		v := float64(*cfg.Recovery.BatteryChargeMin)
		out.RecoveryChargeMin = &v
	}
	if cfg.Recovery.RuntimeMinSeconds != nil {
		out.RecoveryRuntimeMin = time.Duration(*cfg.Recovery.RuntimeMinSeconds) * time.Second
	}
	if cfg.Recovery.RechargeTimeSeconds != nil {
		out.RecoveryRechargeTime = time.Duration(*cfg.Recovery.RechargeTimeSeconds) * time.Second
	}
	return out
}

func configRevision(cfg config.Config) (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("encode config revision: %w", err)
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func newTransactionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("tx-%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
