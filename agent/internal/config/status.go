package config

import (
	"os"
	"sort"
)

// PreviousKnownGood returns the revision immediately preceding last-known-good.
func (m *Manager) PreviousKnownGood() (string, error) { return m.readPointer("previous-known-good") }

// KnownGoodRevisions returns valid known-good manifests, newest first.
func (m *Manager) KnownGoodRevisions() ([]Manifest, error) {
	if err := m.Ensure(); err != nil { return nil, err }
	entries, err := os.ReadDir(m.revisionsDir())
	if err != nil { return nil, err }
	var out []Manifest
	for _, entry := range entries {
		if !entry.IsDir() { continue }
		manifest, err := m.readManifest(entry.Name())
		if err != nil { continue }
		if manifest.Status == RevisionKnownGood { out = append(out, manifest) }
	}
	sort.SliceStable(out, func(i,j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}
