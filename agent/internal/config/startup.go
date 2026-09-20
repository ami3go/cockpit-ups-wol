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
func (m *Manager) RecoverForStartup(ctx context.Context, r Runtime) error {
	if err := m.RecoverInterrupted(ctx, r); err != nil {
		return err
	}

	m.defaults()
	unlock, err := m.lock()
	if err != nil {
		return err
	}
	defer unlock()

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
