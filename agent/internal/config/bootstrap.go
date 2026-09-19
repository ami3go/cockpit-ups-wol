package config

import (
	"fmt"
)

// BootstrapKnownGood records an already-installed configuration as the initial
// known-good revision. The caller must invoke this only after the running stack
// has passed its installation probation/health gate.
//
// The operation is idempotent when the existing last-known-good content is
// identical. Configuration drift is never silently blessed on a later install.
func (m *Manager) BootstrapKnownGood(source string, content []byte) (Manifest, error) {
	m.defaults()
	unlock, err := m.lock()
	if err != nil {
		return Manifest{}, err
	}
	defer unlock()

	if _, err := Parse(content); err != nil {
		return Manifest{}, err
	}

	lkg, err := m.readPointer("last-known-good")
	if err != nil {
		return Manifest{}, err
	}
	if lkg != "" {
		manifest, err := m.readManifest(lkg)
		if err != nil {
			return Manifest{}, fmt.Errorf("read existing last-known-good: %w", err)
		}
		if manifest.Status != RevisionKnownGood {
			return Manifest{}, fmt.Errorf("last-known-good %s has invalid status %s", lkg, manifest.Status)
		}
		if !contentMatches(manifest, content) {
			return Manifest{}, fmt.Errorf("active configuration differs from existing last-known-good %s; use a transactional configuration change", lkg)
		}
		if err := m.writePointer("active", lkg); err != nil {
			return Manifest{}, err
		}
		return manifest, nil
	}

	manifest, err := m.beginLocked(source, content)
	if err != nil {
		return Manifest{}, err
	}
	if err := m.promoteLocked(&manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
