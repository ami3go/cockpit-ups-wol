package config

import (
	"context"
	"fmt"
	"os"
)

// RecoverForStartup reconciles revision metadata and the active configuration
// file before the agent parses or executes that file. It closes the crash
// window where a candidate has been copied over config.yaml but power is lost
// before the active pointer/status transition is made durable.
//
// Startup must not block on the history lock. A live config activation or
// rollback intentionally holds that lock while synchronously restarting this
// agent. In that case the active file is the transaction's intended runtime
// candidate/target, and the transaction owner remains responsible for health
// probation and rollback. After a crash or power loss, flock is released and
// the next startup will acquire the lock and perform normal reconciliation.
func (m *Manager) RecoverForStartup(ctx context.Context, r Runtime) error {
	m.defaults()

	unlock, acquired, err := m.tryLock()
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer unlock()

	if err := m.recoverInterruptedLocked(ctx, r); err != nil {
		return err
	}

	active, err := m.readPointer("active")
	if err != nil {
		return err
	}
	if active == "" {
		// A configuration installed outside the revision manager has not been
		// bootstrapped yet. Preserve the existing first-install behavior.
		return nil
	}

	manifest, err := m.readManifest(active)
	if err != nil {
		lkg, lkgErr := m.readPointer("last-known-good")
		if lkgErr != nil {
			return lkgErr
		}
		if restoreErr := m.restoreKnownGoodLocked(ctx, lkg, r); restoreErr != nil {
			return fmt.Errorf("active revision metadata invalid (%v), restore known-good: %w", err, restoreErr)
		}
		return nil
	}

	activeContent, err := os.ReadFile(m.ActiveConfigPath)
	if err == nil && contentMatches(manifest, activeContent) {
		return nil
	}

	lkg, lkgErr := m.readPointer("last-known-good")
	if lkgErr != nil {
		return lkgErr
	}
	if restoreErr := m.restoreKnownGoodLocked(ctx, lkg, r); restoreErr != nil {
		if err != nil {
			return fmt.Errorf("read active config: %v; restore known-good: %w", err, restoreErr)
		}
		return fmt.Errorf("active config does not match revision %s; restore known-good: %w", active, restoreErr)
	}
	return nil
}

// recoverInterruptedLocked is the startup-safe equivalent of
// RecoverInterrupted for callers that already hold the history lock.
func (m *Manager) recoverInterruptedLocked(ctx context.Context, r Runtime) error {
	active, err := m.readPointer("active")
	if err != nil {
		return err
	}
	lkg, err := m.readPointer("last-known-good")
	if err != nil {
		return err
	}
	if active == "" {
		return nil
	}

	manifest, err := m.readManifest(active)
	if err != nil {
		return m.restoreKnownGoodLocked(ctx, lkg, r)
	}
	if active == lkg {
		if manifest.Status != RevisionKnownGood {
			return fmt.Errorf("last-known-good pointer references non-known-good revision %s (%s)", active, manifest.Status)
		}
		return nil
	}
	if manifest.Status == RevisionKnownGood {
		if lkg != "" && lkg != active {
			if err := m.writePointer("previous-known-good", lkg); err != nil {
				return err
			}
		}
		return m.writePointer("last-known-good", active)
	}
	if manifest.Status == RevisionCandidate || manifest.Status == RevisionValidating || manifest.Status == RevisionFailed {
		return m.restoreKnownGoodLocked(ctx, lkg, r)
	}
	return nil
}
