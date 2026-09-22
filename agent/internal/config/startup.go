package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
		return m.repairKnownGoodLocked(ctx, r, fmt.Errorf("active pointer damaged: %w", err))
	}
	if active == "" {
		// A configuration installed outside the revision manager has not been
		// bootstrapped yet. Preserve the existing first-install behavior.
		return nil
	}

	manifest, err := m.readManifest(active)
	if err != nil {
		if restoreErr := m.repairKnownGoodLocked(ctx, r, fmt.Errorf("active revision metadata invalid: %w", err)); restoreErr != nil {
			return restoreErr
		}
		return nil
	}

	activeContent, err := os.ReadFile(m.ActiveConfigPath)
	if err == nil && contentMatches(manifest, activeContent) {
		return nil
	}

	cause := fmt.Errorf("active config does not match revision %s", active)
	if err != nil {
		cause = fmt.Errorf("read active config: %w", err)
	}
	return m.repairKnownGoodLocked(ctx, r, cause)
}

// recoverInterruptedLocked is the startup-safe equivalent of
// RecoverInterrupted for callers that already hold the history lock.
func (m *Manager) recoverInterruptedLocked(ctx context.Context, r Runtime) error {
	active, err := m.readPointer("active")
	if err != nil {
		return m.repairKnownGoodLocked(ctx, r, fmt.Errorf("active pointer damaged: %w", err))
	}
	lkg, err := m.readPointer("last-known-good")
	if err != nil {
		lkg = m.knownGoodForRepair()
		if lkg == "" {
			return fmt.Errorf("last-known-good pointer damaged and no verified known-good revision is available: %w", err)
		}
		if repairErr := m.writePointer("last-known-good", lkg); repairErr != nil {
			return fmt.Errorf("repair last-known-good pointer after %v: %w", err, repairErr)
		}
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

// repairKnownGoodLocked is used only during startup reconciliation. Normal
// pointer access remains strict; startup is deliberately more tolerant because
// damaged metadata must not turn the safety agent into a permanent crash loop.
func (m *Manager) repairKnownGoodLocked(ctx context.Context, r Runtime, cause error) error {
	lkg := m.knownGoodForRepair()
	if lkg == "" {
		return fmt.Errorf("%v; no verified known-good revision is available for startup repair", cause)
	}
	if err := m.restoreKnownGoodLocked(ctx, lkg, r); err != nil {
		return fmt.Errorf("%v; restore verified known-good %s: %w", cause, lkg, err)
	}
	if err := m.writePointer("last-known-good", lkg); err != nil {
		return fmt.Errorf("%v; repaired active revision but could not repair last-known-good pointer: %w", cause, err)
	}
	return nil
}

// knownGoodForRepair prefers the last-known-good pointer when it still resolves
// to a valid known-good manifest whose config bytes match the stored hash. If
// the pointer itself is damaged or its target is unusable, fall back to the
// newest verified known-good revision on disk.
func (m *Manager) knownGoodForRepair() string {
	if lkg, err := m.readPointer("last-known-good"); err == nil && lkg != "" {
		if manifest, manifestErr := m.readManifest(lkg); manifestErr == nil && manifest.Status == RevisionKnownGood {
			if content, contentErr := os.ReadFile(filepath.Join(m.revisionDir(lkg), "config.yaml")); contentErr == nil && contentMatches(manifest, content) {
				return lkg
			}
		}
	}

	revisions, err := m.KnownGoodRevisions()
	if err != nil {
		return ""
	}
	for _, revision := range revisions {
		content, err := os.ReadFile(filepath.Join(m.revisionDir(revision.RevisionID), "config.yaml"))
		if err == nil && contentMatches(revision, content) {
			return revision.RevisionID
		}
	}
	return ""
}
