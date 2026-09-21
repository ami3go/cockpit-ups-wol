package config

import (
	"fmt"
	"os"
	"regexp"
)

const defaultRevisionKeep = 10

var revisionIDPattern = regexp.MustCompile(`^cfg-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{8}(-[0-9]+)?$`)

func ValidateRevisionID(id string) error {
	if !revisionIDPattern.MatchString(id) {
		return fmt.Errorf("invalid revision id %q", id)
	}
	return nil
}

func (m *Manager) PruneRevisions(keep int) error {
	if keep < 1 {
		return fmt.Errorf("revision keep count must be >= 1")
	}
	unlock, err := m.lock()
	if err != nil {
		return err
	}
	defer unlock()
	return m.pruneRevisionsLocked(keep)
}

func (m *Manager) pruneRevisionsLocked(keep int) error {
	if keep < 1 {
		return fmt.Errorf("revision keep count must be >= 1")
	}
	protected := map[string]bool{}
	for _, name := range []string{"active", "last-known-good", "previous-known-good"} {
		id, err := m.readPointer(name)
		if err != nil {
			return fmt.Errorf("read protected revision pointer %s: %w", name, err)
		}
		if id != "" {
			protected[id] = true
		}
	}
	revisions, err := m.KnownGoodRevisions()
	if err != nil {
		return err
	}
	keepSet := map[string]bool{}
	for id := range protected {
		keepSet[id] = true
	}
	for i, revision := range revisions {
		if i >= keep {
			break
		}
		keepSet[revision.RevisionID] = true
	}
	removed := false
	for _, revision := range revisions {
		if keepSet[revision.RevisionID] {
			continue
		}
		if err := ValidateRevisionID(revision.RevisionID); err != nil {
			return err
		}
		if err := os.RemoveAll(m.revisionDir(revision.RevisionID)); err != nil {
			return fmt.Errorf("prune revision %s: %w", revision.RevisionID, err)
		}
		removed = true
	}
	if !removed {
		return nil
	}
	dir, err := os.Open(m.revisionsDir())
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
