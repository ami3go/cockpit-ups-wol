package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	recovery := host.RecoveryExecutor{
		Waker:   armedWaker{hosts: byID, sender: wol.Sender{}, log: opts.Log},
		Checker: armedRecoveryChecker{hosts: byID, checker: host.StatusChecker{}},
		Store:   coord,
	}
	controller := &orchestrator.Controller{
		Config:           cfg,
		Policy:           coord,
		Probe:            probe,
		Shutdown:         host.ShutdownExecutor{},
		FSD:              opts.NUT,
		Recovery:         recovery,
		UPSMonConfPath:   opts.UPSMonConfPath,
		NewTransactionID: newTransactionID,
	}
	deps := newDependencyTracker(cfg.Dependencies)
	controllerFSD := &controllerFSDTracker{}

	serverCtx, cancelServer := context.WithCancel(ctx)
	defer cancelServer()
	serverErr := make(chan error, 1)
	server := &ipc.Server{
		Handler:    ipc.Handler{Health: supervisor},
		SocketPath: opts.SocketPath,
		SocketMode: 0o600,
	}
	go func() { serverErr <- server.ListenAndServe(serverCtx) }()

	// Reconcile persisted state once before announcing READY. UNKNOWN NUT state
	// is safe: policy remains in BOOT_RECONCILE and performs no destructive work.
	if err := armedPowerTick(ctx, cfg, opts, controller, deps, latestHealth, controllerFSD); err != nil {
		return err
	}
	if err := opts.Ready(); err != nil {
		return fmt.Errorf("systemd READY notification: %w", err)
	}
	defer func() { _ = opts.Stopping() }()
	opts.Log.Info("armed runtime ready",
		"transaction_id", controller.Policy.State().TransactionID,
		"power_state", controller.Policy.State().PowerState,
		"config_revision", revision)

	beat := make(chan struct{}, 1)
	watchdogCh, err := opts.StartWatchdog(ctx, beat, 20*time.Second)
	if err != nil {
		return fmt.Errorf("start systemd watchdog: %w", err)
	}
	nonBlockingBeat(beat)

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
				opts.Log.Error("health supervisor run failed", "error", err)
				return fmt.Errorf("health supervisor: %w", err)
			}
			if latestHealth.State != health.Healthy {
				opts.Log.Warn("agent health degraded", "health_state", latestHealth.State)
			}
		case <-powerTicker.C:
			if err := armedPowerTick(ctx, cfg, opts, controller, deps, latestHealth, controllerFSD); err != nil {
				return err
			}
			nonBlockingBeat(beat)
		}
	}
}

func armedPowerTick(ctx context.Context, cfg config.Config, opts Options, controller *orchestrator.Controller, deps *dependencyTracker, latestHealth health.Snapshot, fsdTracker *controllerFSDTracker) error {
	target := nut.Target(cfg.NUT.UPSName, cfg.NUT.Host, cfg.NUT.Port)
	upsStatus, err := opts.NUT.Query(ctx, target)
	if err != nil {
		opts.Log.Warn("UPS query failed; treating utility as UNKNOWN", "target", target, "error", err)
		upsStatus = nut.Status{Utility: nut.UtilityUnknown}
	}

	now := time.Now()
	systemSafe, systemHealthReason := systemHealthSafe(opts.SystemHealthStatePath, opts.SystemHealthMaxAge, now)
	healthSafe := latestHealth.State == health.Healthy && systemSafe
	networkReady := deps.Ready(ctx)
	before := controller.Policy.State().PowerState
	decision, err := controller.Tick(ctx, now, policy.Inputs{
		UPS:          upsStatus,
		NetworkReady: networkReady,
		HealthSafe:   healthSafe,
	})
	if err != nil {
		opts.Log.Error("armed orchestration failed",
			"power_state", before,
			"utility", upsStatus.Utility,
			"error", err)
		return fmt.Errorf("armed orchestration: %w", err)
	}
	afterState := controller.Policy.State()
	if decision.Changed || decision.Action != policy.ActionNone || before != afterState.PowerState {
		attrs := []any{
			"from", before,
			"to", afterState.PowerState,
			"action", decision.Action,
			"reason", decision.Reason,
			"transaction_id", afterState.TransactionID,
			"utility", upsStatus.Utility,
			"raw_status", upsStatus.RawStatus,
			"low_battery", upsStatus.LowBattery,
			"fsd", upsStatus.FSD,
			"network_ready", networkReady,
			"agent_health", latestHealth.State,
			"system_health_safe", systemSafe,
			"system_health_reason", systemHealthReason,
		}
		if upsStatus.ChargePercent != nil {
			attrs = append(attrs, "charge_percent", *upsStatus.ChargePercent)
		}
		if upsStatus.RuntimeSeconds != nil {
			attrs = append(attrs, "runtime_seconds", *upsStatus.RuntimeSeconds)
		}
		opts.Log.Info("power state transition", attrs...)
	}

	// If there are no NUT-managed hosts, the controller still must be shut down
	// through its primary upsmon after all direct hosts have settled. The durable
	// shutdown commit is the intent record; retrying FSD after a daemon restart is
	// therefore safe and idempotent at the transaction level.
	st := controller.Policy.State()
	if st.ShutdownCommitted && st.PowerState == state.WaitingForAC && upsStatus.Utility == nut.UtilityOnBattery && cfg.NUT.Profile != "remote-client" && !hasNUTManagedHosts(cfg.Hosts) && fsdTracker.shouldAttempt(now, st.TransactionID) {
		opts.Log.Info("requesting controller FSD", "transaction_id", st.TransactionID, "ups", cfg.NUT.UPSName)
		if err := opts.NUT.RequestFSD(ctx, cfg.NUT.UPSName, opts.UPSMonConfPath); err != nil {
			delay := fsdTracker.markFailure(now, st.TransactionID)
			opts.Log.Error("controller FSD request failed; retry scheduled", "transaction_id", st.TransactionID, "retry_after", delay, "error", err)
			return nil
		}
		fsdTracker.markSuccess(st.TransactionID)
		opts.Log.Info("controller FSD requested", "transaction_id", st.TransactionID)
	}
	return nil
}

type controllerFSDTracker struct {
	transactionID string
	completed     bool
	attempts      int
	nextAttempt   time.Time
}

func (t *controllerFSDTracker) resetFor(transactionID string) {
	if t.transactionID == transactionID {
		return
	}
	t.transactionID = transactionID
	t.completed = false
	t.attempts = 0
	t.nextAttempt = time.Time{}
}

func (t *controllerFSDTracker) shouldAttempt(now time.Time, transactionID string) bool {
	t.resetFor(transactionID)
	return !t.completed && (t.nextAttempt.IsZero() || !now.Before(t.nextAttempt))
}

func (t *controllerFSDTracker) markSuccess(transactionID string) {
	t.resetFor(transactionID)
	t.completed = true
	t.nextAttempt = time.Time{}
}

func (t *controllerFSDTracker) markFailure(now time.Time, transactionID string) time.Duration {
	t.resetFor(transactionID)
	t.attempts++
	delay := 5 * time.Second
	for i := 1; i < t.attempts && delay < time.Minute; i++ {
		delay *= 2
	}
	if delay > time.Minute {
		delay = time.Minute
	}
	t.nextAttempt = now.Add(delay)
	return delay
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
	log    *slog.Logger
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
	if w.log != nil {
		w.log.Info("sending Wake-on-LAN", "host_id", hostID, "broadcast", req.Broadcast, "port", req.Port, "interface", req.Interface)
	}
	if err := w.sender.Wake(ctx, req); err != nil {
		if w.log != nil {
			w.log.Error("Wake-on-LAN failed", "host_id", hostID, "error", err)
		}
		return err
	}
	if w.log != nil {
		w.log.Info("Wake-on-LAN sent", "host_id", hostID)
	}
	return nil
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
	material := struct {
		Config                 config.Config `json:"config"`
		SynologyPasswordSHA256 string        `json:"synology_password_sha256,omitempty"`
	}{Config: cfg}
	if cfg.NUT.Synology.Enabled {
		digest := sha256.Sum256([]byte(cfg.NUT.Synology.Password))
		material.SynologyPasswordSHA256 = hex.EncodeToString(digest[:])
	}
	b, err := json.Marshal(material)
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
