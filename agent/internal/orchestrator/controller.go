package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/host"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/policy"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

type Prober interface {
	Check(context.Context, config.HostConfig) (host.ProbeResult, error)
	Wait(context.Context, config.HostConfig, bool) error
}
type Shutdowner interface {
	Shutdown(context.Context, config.HostConfig) (host.ShutdownResult, error)
}
type FSDRequester interface {
	RequestFSD(context.Context, string, string) error
}
type RecoveryRunner interface {
	RunNext(context.Context, state.State, []host.Config) (state.State, string, error)
}

const maxDirectShutdownAttempts = 3

var (
	errDirectShutdownRetryPending = errors.New("direct shutdown retry pending")
	errDirectShutdownFailedSafe   = errors.New("direct shutdown entered failed-safe")
)

type Controller struct {
	Config           config.Config
	Policy           *policy.Coordinator
	Probe            Prober
	Shutdown         Shutdowner
	FSD              FSDRequester
	Recovery         RecoveryRunner
	UPSMonConfPath   string
	NewTransactionID func() string

	lastRecoveryWakeAt       time.Time
	lastRecoveryWakeHostID   string
	resumeWakeDelayEvaluated bool
}

func (c *Controller) Tick(ctx context.Context, now time.Time, in policy.Inputs) (policy.Decision, error) {
	if c.Policy == nil {
		return policy.Decision{}, errors.New("policy coordinator is required")
	}
	before := c.Policy.State().PowerState
	decision, err := c.Policy.Step(now, in)
	if err != nil {
		return policy.Decision{}, err
	}
	after := c.Policy.State().PowerState
	if before == state.Normal && after == state.OnBattery {
		if err := c.beginFreshOutage(ctx); err != nil {
			return policy.Decision{}, err
		}
	} else if before == state.BootReconcile && after == state.OnBattery {
		if err := c.snapshotHosts(ctx, false); err != nil {
			return policy.Decision{}, err
		}
	}
	switch decision.Action {
	case policy.ActionCommitShutdown:
		// Persist SHUTDOWN_IN_PROGRESS before the first per-host side effect.
		if _, err := c.Policy.Step(now, in); err != nil {
			return policy.Decision{}, err
		}
		if err := c.executeShutdown(ctx); err != nil {
			return decision, err
		}
	case policy.ActionStartRestore:
		if err := c.executeRecoveryStep(ctx, now); err != nil {
			return decision, err
		}
	case policy.ActionStopRecovery:
		// A power failure during recovery is a new outage, not a continuation of
		// the old shutdown transaction. Re-snapshot actual host state so hosts
		// already restored online become shutdown targets again and stale
		// ShutdownCompleted markers cannot suppress the second shutdown.
		if c.Policy.State().PowerState == state.OnBattery {
			if err := c.beginFreshOutage(ctx); err != nil {
				return decision, err
			}
		}
		return decision, nil
	}
	if c.Policy.State().PowerState == state.ShutdownInProgress && decision.Action == policy.ActionNone {
		if err := c.executeShutdown(ctx); err != nil {
			return decision, err
		}
	}
	if c.Policy.State().PowerState == state.RestoreHosts && decision.Action == policy.ActionNone {
		if err := c.executeRecoveryStep(ctx, now); err != nil {
			return decision, err
		}
	}
	return decision, nil
}

func (c *Controller) beginFreshOutage(ctx context.Context) error {
	c.lastRecoveryWakeAt = time.Time{}
	c.lastRecoveryWakeHostID = ""
	c.resumeWakeDelayEvaluated = false
	st := cloneState(c.Policy.State())
	if c.NewTransactionID != nil {
		id := c.NewTransactionID()
		if id == "" {
			return errors.New("new transaction id is empty")
		}
		st.ParentTransactionID = st.TransactionID
		st.TransactionID = id
	}
	st.Hosts = map[string]state.HostState{}
	st.ShutdownCommitted = false
	st.RecoveryStarted = false
	st.FailedSafeReason = ""
	if _, err := c.Policy.Write(st); err != nil {
		return fmt.Errorf("persist new outage transaction: %w", err)
	}
	return c.snapshotHosts(ctx, true)
}

func (c *Controller) snapshotHosts(ctx context.Context, offlineKnown bool) error {
	if c.Probe == nil {
		return errors.New("host prober is required")
	}
	st := cloneState(c.Policy.State())
	for _, h := range c.Config.Hosts {
		hs, exists := st.Hosts[h.ID]
		if exists && hs.WasOnline != nil {
			continue
		}
		hs.ShutdownState = state.ShutdownPlanned
		if h.Shutdown.Method == "none" {
			hs.ShutdownState = state.ShutdownNotRequired
		}
		hs.RecoveryState = state.RecoveryWaiting
		if !h.Wake.Enabled || h.RestorePolicy == "never" {
			hs.RecoveryState = state.RecoveryNotRequired
		}
		if h.Address == nil || h.Status.Method == "none" {
			hs.WasOnline = nil
			hs.LastVerification = "unknown"
		} else {
			result, err := c.Probe.Check(ctx, h)
			if err != nil {
				hs.WasOnline = nil
				hs.LastVerification = "unknown"
				hs.LastError = err.Error()
			} else if result.Known && result.Online {
				v := true
				hs.WasOnline = &v
				hs.LastVerification = "online"
			} else if result.Known && offlineKnown {
				v := false
				hs.WasOnline = &v
				hs.LastVerification = "offline"
			} else {
				hs.WasOnline = nil
				hs.LastVerification = "unknown"
			}
		}
		st.Hosts[h.ID] = hs
	}
	_, err := c.Policy.Write(st)
	if err != nil {
		return fmt.Errorf("persist outage snapshot: %w", err)
	}
	return nil
}

func (c *Controller) executeShutdown(ctx context.Context) error {
	if c.Probe == nil || c.Shutdown == nil {
		return errors.New("shutdown dependencies are required")
	}
	byID := make(map[string]config.HostConfig, len(c.Config.Hosts))
	for _, h := range c.Config.Hosts {
		byID[h.ID] = h
	}
	plan := host.BuildShutdownPlan(toHostConfigs(c.Config.Hosts))
	for _, id := range plan.PreFSD {
		if err := c.settleDirect(ctx, byID[id]); err != nil {
			// Retryable/terminal host states are already durable. Do not advance
			// to NUT FSD while a direct host is unresolved; the next policy tick
			// either reconciles and retries, or FAILED_SAFE holds the transaction.
			if errors.Is(err, errDirectShutdownRetryPending) || errors.Is(err, errDirectShutdownFailedSafe) {
				return nil
			}
			return err
		}
	}
	if len(plan.NUTGroup) > 0 {
		if c.FSD == nil {
			return errors.New("FSD requester is required for NUT-managed hosts")
		}
		if c.Config.NUT.Profile == "remote-client" {
			return errors.New("remote-client profile cannot request FSD")
		}
		if err := c.prepareNUTGroup(ctx, plan.NUTGroup, byID); err != nil {
			return err
		}
		if err := c.FSD.RequestFSD(ctx, c.Config.NUT.UPSName, c.upsmonPath()); err != nil {
			persistErr := c.markNUTError(plan.NUTGroup, err)
			if persistErr != nil {
				return errors.Join(fmt.Errorf("request FSD: %w", err), fmt.Errorf("persist NUT FSD failure state: %w", persistErr))
			}
			return fmt.Errorf("request FSD: %w", err)
		}
		if err := c.markNUTAcknowledged(plan.NUTGroup); err != nil {
			return fmt.Errorf("persist NUT FSD acknowledgement: %w", err)
		}
		if err := c.verifyNUTGroup(ctx, plan.NUTGroup, byID); err != nil {
			return fmt.Errorf("persist NUT shutdown verification: %w", err)
		}
	}
	_, err := c.Policy.MarkShutdownPhaseComplete()
	return err
}

func (c *Controller) settleDirect(ctx context.Context, h config.HostConfig) error {
	st := c.Policy.State()
	hs := st.Hosts[h.ID]
	if hs.ShutdownState == state.ShutdownCompleted || hs.ShutdownState == state.ShutdownNotRequired {
		return nil
	}
	if hs.ShutdownState == state.ShutdownFailed {
		if err := c.enterShutdownFailedSafe(h.ID, hs, "direct shutdown previously exhausted retries"); err != nil {
			return err
		}
		return errDirectShutdownFailedSafe
	}
	if hs.ShutdownState == state.ShutdownRequested || hs.ShutdownState == state.ShutdownAcknowledged || hs.ShutdownState == state.ShutdownUnknown {
		if h.Address == nil || h.Status.Method == "none" {
			hs.ShutdownState = state.ShutdownUnknown
			hs.LastError = "cannot reconcile prior shutdown without status probe"
			if err := c.persistHost(h.ID, hs); err != nil {
				return err
			}
			return errDirectShutdownRetryPending
		}
		result, err := c.Probe.Check(ctx, h)
		if err != nil || !result.Known {
			hs.ShutdownState = state.ShutdownUnknown
			if err != nil {
				hs.LastError = err.Error()
			} else {
				hs.LastError = "shutdown state probe returned unknown"
			}
			if err := c.persistHost(h.ID, hs); err != nil {
				return err
			}
			return errDirectShutdownRetryPending
		}
		if !result.Online {
			hs.ShutdownState = state.ShutdownCompleted
			hs.LastVerification = "offline"
			hs.LastError = ""
			return c.persistHost(h.ID, hs)
		}
	}
	if hs.ShutdownAttempts >= maxDirectShutdownAttempts {
		if err := c.enterShutdownFailedSafe(h.ID, hs, fmt.Sprintf("direct shutdown exhausted %d attempts while host is still online", maxDirectShutdownAttempts)); err != nil {
			return err
		}
		return errDirectShutdownFailedSafe
	}
	hs.ShutdownAttempts++
	hs.ShutdownState = state.ShutdownRequested
	hs.LastActionID = fmt.Sprintf("shutdown:%s:%s:%d", st.TransactionID, h.ID, hs.ShutdownAttempts)
	hs.LastError = ""
	if err := c.persistHost(h.ID, hs); err != nil {
		return err
	}
	result, err := c.Shutdown.Shutdown(ctx, h)
	if err != nil {
		hs = c.Policy.State().Hosts[h.ID]
		hs.LastError = err.Error()
		if hs.ShutdownAttempts >= maxDirectShutdownAttempts {
			if err := c.enterShutdownFailedSafe(h.ID, hs, fmt.Sprintf("direct shutdown exhausted %d attempts: %v", maxDirectShutdownAttempts, err)); err != nil {
				return err
			}
			return errDirectShutdownFailedSafe
		}
		hs.ShutdownState = state.ShutdownUnknown
		if err := c.persistHost(h.ID, hs); err != nil {
			return err
		}
		return errDirectShutdownRetryPending
	}
	hs = c.Policy.State().Hosts[h.ID]
	switch result.Disposition {
	case host.ShutdownNotRequired:
		hs.ShutdownState = state.ShutdownNotRequired
		return c.persistHost(h.ID, hs)
	case host.ShutdownManagedByNUT:
		return fmt.Errorf("pre-FSD host %s unexpectedly delegated to NUT", h.ID)
	case host.ShutdownDirectRequested:
		hs.ShutdownState = state.ShutdownAcknowledged
		if err := c.persistHost(h.ID, hs); err != nil {
			return err
		}
	}
	if h.Address == nil || h.Status.Method == "none" {
		return nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(h.Shutdown.TimeoutSeconds)*time.Second)
	defer cancel()
	err = c.Probe.Wait(waitCtx, h, false)
	hs = c.Policy.State().Hosts[h.ID]
	if err != nil {
		hs.ShutdownState = state.ShutdownUnknown
		hs.LastError = err.Error()
		hs.LastVerification = "unknown"
	} else {
		hs.ShutdownState = state.ShutdownCompleted
		hs.LastError = ""
		hs.LastVerification = "offline"
	}
	if err := c.persistHost(h.ID, hs); err != nil {
		return err
	}
	if hs.ShutdownState == state.ShutdownUnknown {
		return errDirectShutdownRetryPending
	}
	return nil
}

func (c *Controller) enterShutdownFailedSafe(id string, hs state.HostState, reason string) error {
	hs.ShutdownState = state.ShutdownFailed
	hs.LastError = reason
	st := cloneState(c.Policy.State())
	st.Hosts[id] = hs
	st.PowerState = state.FailedSafe
	st.FailedSafeReason = fmt.Sprintf("host %s: %s", id, reason)
	_, err := c.Policy.Write(st)
	return err
}

func (c *Controller) prepareNUTGroup(ctx context.Context, ids []string, byID map[string]config.HostConfig) error {
	for _, id := range ids {
		h := byID[id]
		hs := c.Policy.State().Hosts[id]
		if hs.ShutdownState == state.ShutdownCompleted {
			continue
		}
		if h.Address != nil && h.Status.Method != "none" {
			res, err := c.Probe.Check(ctx, h)
			if err == nil && res.Known && !res.Online {
				hs.ShutdownState = state.ShutdownCompleted
				hs.LastVerification = "offline"
				if err := c.persistHost(id, hs); err != nil {
					return err
				}
				continue
			}
		}
		hs.ShutdownAttempts++
		hs.ShutdownState = state.ShutdownRequested
		hs.LastActionID = fmt.Sprintf("fsd:%s:%s:%d", c.Policy.State().TransactionID, id, hs.ShutdownAttempts)
		if err := c.persistHost(id, hs); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) markNUTAcknowledged(ids []string) error {
	var errs []error
	for _, id := range ids {
		hs := c.Policy.State().Hosts[id]
		if hs.ShutdownState != state.ShutdownRequested {
			continue
		}
		hs.ShutdownState = state.ShutdownAcknowledged
		hs.LastError = ""
		if err := c.persistHost(id, hs); err != nil {
			errs = append(errs, fmt.Errorf("persist %s acknowledgement: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func (c *Controller) markNUTError(ids []string, cause error) error {
	var errs []error
	for _, id := range ids {
		hs := c.Policy.State().Hosts[id]
		if hs.ShutdownState == state.ShutdownCompleted {
			continue
		}
		hs.ShutdownState = state.ShutdownUnknown
		hs.LastError = cause.Error()
		if err := c.persistHost(id, hs); err != nil {
			errs = append(errs, fmt.Errorf("persist %s NUT error: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func (c *Controller) verifyNUTGroup(ctx context.Context, ids []string, byID map[string]config.HostConfig) error {
	var errs []error
	for _, id := range ids {
		h := byID[id]
		hs := c.Policy.State().Hosts[id]
		if hs.ShutdownState == state.ShutdownCompleted {
			continue
		}
		if h.Address == nil || h.Status.Method == "none" {
			continue
		}
		waitCtx, cancel := context.WithTimeout(ctx, time.Duration(h.Shutdown.TimeoutSeconds)*time.Second)
		err := c.Probe.Wait(waitCtx, h, false)
		cancel()
		hs = c.Policy.State().Hosts[id]
		if err != nil {
			hs.ShutdownState = state.ShutdownUnknown
			hs.LastError = err.Error()
			hs.LastVerification = "unknown"
		} else {
			hs.ShutdownState = state.ShutdownCompleted
			hs.LastError = ""
			hs.LastVerification = "offline"
		}
		if err := c.persistHost(id, hs); err != nil {
			errs = append(errs, fmt.Errorf("persist %s verification: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func (c *Controller) executeRecoveryStep(ctx context.Context, now time.Time) error {
	if c.Recovery == nil {
		return errors.New("recovery runner is required")
	}
	st := c.Policy.State()
	hosts := toHostConfigs(c.Config.Hosts)
	if markBlockedRecovery(&st, hosts) {
		if _, err := c.Policy.Write(st); err != nil {
			return err
		}
		st = c.Policy.State()
	}
	if recoverySettled(hosts, st.Hosts) {
		c.resetRecoveryWakeDelay()
		_, err := c.Policy.MarkRecoveryComplete()
		return err
	}

	next, hasNext, err := host.NextRecoveryHost(hosts, st.Hosts)
	if err != nil {
		return err
	}
	if hasNext {
		delay := time.Duration(next.WakeDelayAfterPreviousSeconds) * time.Second
		if !c.resumeWakeDelayEvaluated {
			c.resumeWakeDelayEvaluated = true
			if recoveryHasPriorWake(st.Hosts) && delay > 0 {
				// The exact pre-reboot monotonic timestamp cannot be reconstructed.
				// Conservatively apply a full delay once when resuming recovery.
				c.lastRecoveryWakeAt = now
				c.lastRecoveryWakeHostID = "__resume__"
				return nil
			}
		}
		if delay > 0 && !c.lastRecoveryWakeAt.IsZero() && next.ID != c.lastRecoveryWakeHostID && now.Sub(c.lastRecoveryWakeAt) < delay {
			return nil
		}
	}

	beforeAttempts := 0
	if hasNext {
		beforeAttempts = st.Hosts[next.ID].WakeAttempts
	}
	updated, id, err := c.Recovery.RunNext(ctx, cloneState(st), hosts)
	_ = updated
	if id != "" {
		after := c.Policy.State().Hosts[id]
		if after.WakeAttempts > beforeAttempts {
			c.lastRecoveryWakeAt = now
			c.lastRecoveryWakeHostID = id
		}
	}
	if err != nil { // a per-host failed wake is durable; continue independent hosts on later ticks
		if id != "" {
			return nil
		}
		return err
	}
	if id == "" && recoverySettled(hosts, c.Policy.State().Hosts) {
		c.resetRecoveryWakeDelay()
		_, err := c.Policy.MarkRecoveryComplete()
		return err
	}
	return nil
}

func (c *Controller) resetRecoveryWakeDelay() {
	c.lastRecoveryWakeAt = time.Time{}
	c.lastRecoveryWakeHostID = ""
	c.resumeWakeDelayEvaluated = false
}

func recoveryHasPriorWake(states map[string]state.HostState) bool {
	for _, hs := range states {
		if hs.WakeAttempts > 0 || hs.RecoveryState == state.RecoveryWOLSent || hs.RecoveryState == state.RecoveryOnline {
			return true
		}
	}
	return false
}

func (c *Controller) persistHost(id string, hs state.HostState) error {
	st := cloneState(c.Policy.State())
	st.Hosts[id] = hs
	_, err := c.Policy.Write(st)
	return err
}

func (c *Controller) upsmonPath() string {
	if c.UPSMonConfPath != "" {
		return c.UPSMonConfPath
	}
	return "/etc/nut/upsmon.conf"
}

func cloneState(in state.State) state.State {
	out := in
	out.Hosts = make(map[string]state.HostState, len(in.Hosts))
	for k, v := range in.Hosts {
		out.Hosts[k] = v
	}
	return out
}

func toHostConfigs(in []config.HostConfig) []host.Config {
	out := make([]host.Config, 0, len(in))
	for _, h := range in {
		hc := host.Config{
			ID:                            h.ID,
			ShutdownMethod:                h.Shutdown.Method,
			ShutdownPriority:              h.Shutdown.Priority,
			WakeEnabled:                   h.Wake.Enabled,
			WakePriority:                  h.Wake.Priority,
			WakeMaxAttempts:               h.Wake.MaxAttempts,
			WakeDelayAfterPreviousSeconds: h.Wake.DelayAfterPreviousSeconds,
			WakePort:                      h.Wake.Port,
			RestorePolicy:                 host.RestorePolicy(h.RestorePolicy),
			DependsOn:                     append([]string(nil), h.DependsOn...),
		}
		if h.Wake.MAC != nil {
			hc.WakeMAC = *h.Wake.MAC
		}
		if h.Wake.Interface != nil {
			hc.WakeInterface = *h.Wake.Interface
		}
		if h.Wake.Broadcast != nil {
			hc.WakeBroadcast = *h.Wake.Broadcast
		}
		out = append(out, hc)
	}
	return out
}

func recoverySettled(hosts []host.Config, states map[string]state.HostState) bool {
	any := false
	for _, h := range hosts {
		hs, ok := states[h.ID]
		if !ok || !host.EligibleForRestore(h, hs) {
			continue
		}
		any = true
		if hs.RecoveryState != state.RecoveryOnline && hs.RecoveryState != state.RecoveryFailed && hs.RecoveryState != state.RecoveryNotRequired {
			return false
		}
	}
	return any || true
}

func markBlockedRecovery(st *state.State, hosts []host.Config) bool {
	byID := map[string]host.Config{}
	for _, h := range hosts {
		byID[h.ID] = h
	}
	changed := false
	for _, h := range hosts {
		hs := st.Hosts[h.ID]
		if !host.EligibleForRestore(h, hs) || hs.RecoveryState == state.RecoveryOnline || hs.RecoveryState == state.RecoveryFailed {
			continue
		}
		for _, dep := range h.DependsOn {
			dcfg, ok := byID[dep]
			if !ok {
				continue
			}
			ds := st.Hosts[dep]
			if host.EligibleForRestore(dcfg, ds) && ds.RecoveryState == state.RecoveryFailed {
				hs.RecoveryState = state.RecoveryFailed
				hs.LastError = "managed recovery dependency failed: " + dep
				st.Hosts[h.ID] = hs
				changed = true
				break
			}
		}
	}
	return changed
}
