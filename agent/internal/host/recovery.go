package host

import (
	"context"
	"errors"
	"fmt"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

type RecoveryWaker interface {
	Wake(ctx context.Context, hostID string) error
}
type RecoveryChecker interface {
	Online(ctx context.Context, hostID string) (bool, error)
}
type RecoveryStateWriter interface {
	Write(state.State) (state.State, error)
}

type RecoveryExecutor struct {
	Waker   RecoveryWaker
	Checker RecoveryChecker
	Store   RecoveryStateWriter
}

// Next returns the next eligible managed host whose in-plan managed
// dependencies are already confirmed online. External dependencies are gated by
// the policy/network readiness layer before RESTORE_HOSTS begins.
func NextRecoveryHost(hosts []Config, states map[string]state.HostState) (Config, bool, error) {
	plan, err := BuildRecoveryPlan(hosts, states)
	if err != nil {
		return Config{}, false, err
	}
	byID := make(map[string]Config, len(hosts))
	for _, h := range hosts {
		byID[h.ID] = h
	}
	for _, id := range plan {
		hs := states[id]
		if hs.RecoveryState == state.RecoveryOnline || hs.RecoveryState == state.RecoveryNotRequired || hs.RecoveryState == state.RecoveryFailed {
			continue
		}
		cfg := byID[id]
		ready := true
		for _, dep := range cfg.DependsOn {
			if _, managed := byID[dep]; !managed {
				continue
			}
			depState, present := states[dep]
			if present && EligibleForRestore(byID[dep], depState) && depState.RecoveryState != state.RecoveryOnline {
				ready = false
				break
			}
		}
		if ready {
			return cfg, true, nil
		}
	}
	return Config{}, false, nil
}

// RunNext reconciles and, if needed, wakes at most one host. It persists the
// attempt counter and wol_sent state before calling the external Waker.
func (r RecoveryExecutor) RunNext(ctx context.Context, st state.State, hosts []Config) (state.State, string, error) {
	if r.Store == nil || r.Checker == nil || r.Waker == nil {
		return st, "", errors.New("recovery executor dependencies are required")
	}
	cfg, ok, err := NextRecoveryHost(hosts, st.Hosts)
	if err != nil || !ok {
		return st, "", err
	}
	hs := st.Hosts[cfg.ID]
	online, err := r.Checker.Online(ctx, cfg.ID)
	if err == nil && online {
		hs.RecoveryState = state.RecoveryOnline
		hs.LastVerification = "online"
		st.Hosts[cfg.ID] = hs
		persisted, err := r.Store.Write(st)
		return persisted, cfg.ID, err
	}
	maxAttempts := cfg.WakeMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if hs.WakeAttempts >= maxAttempts {
		hs.RecoveryState = state.RecoveryFailed
		hs.LastError = "maximum wake attempts reached"
		st.Hosts[cfg.ID] = hs
		persisted, writeErr := r.Store.Write(st)
		if writeErr != nil {
			return st, cfg.ID, writeErr
		}
		return persisted, cfg.ID, fmt.Errorf("host %s: maximum wake attempts reached", cfg.ID)
	}
	hs.WakeAttempts++
	hs.RecoveryState = state.RecoveryWOLSent
	hs.LastActionID = fmt.Sprintf("wake:%s:%s:%d", st.TransactionID, cfg.ID, hs.WakeAttempts)
	hs.LastVerification = "offline"
	if err != nil {
		hs.LastError = err.Error()
	} else {
		hs.LastError = ""
	}
	st.Hosts[cfg.ID] = hs
	persisted, err := r.Store.Write(st)
	if err != nil {
		return st, cfg.ID, err
	}
	if err := r.Waker.Wake(ctx, cfg.ID); err != nil {
		hs = persisted.Hosts[cfg.ID]
		hs.LastError = err.Error()
		persisted.Hosts[cfg.ID] = hs
		updated, writeErr := r.Store.Write(persisted)
		if writeErr != nil {
			return persisted, cfg.ID, fmt.Errorf("wake failed: %v; persist error: %w", err, writeErr)
		}
		return updated, cfg.ID, err
	}
	return persisted, cfg.ID, nil
}
