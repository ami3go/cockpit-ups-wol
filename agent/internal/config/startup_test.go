package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoverForStartupRestoresLKGWhenActiveFileDoesNotMatchPointer(t *testing.T) {
	m := newManager(t)

	first, err := m.Begin("test", validJSON())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ActivateAndValidate(context.Background(), first.RevisionID, &fakeRuntime{}); err != nil {
		t.Fatal(err)
	}

	changed := strings.Replace(string(validJSON()), `"mode":"dry-run"`, `"mode":"monitor"`, 1)
	candidate, err := m.Begin("test", []byte(changed))
	if err != nil {
		t.Fatal(err)
	}

	// Simulate loss of power in ActivateAndValidate after candidate bytes were
	// atomically copied over config.yaml but before the active pointer was
	// switched from the known-good revision to the candidate.
	candidateBytes, err := os.ReadFile(filepath.Join(m.revisionDir(candidate.RevisionID), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(m.ActiveConfigPath, candidateBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	activeBefore, err := m.ActiveRevision()
	if err != nil {
		t.Fatal(err)
	}
	if activeBefore != first.RevisionID {
		t.Fatalf("precondition failed: active=%s want %s", activeBefore, first.RevisionID)
	}

	if err := m.RecoverForStartup(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	activeAfter, err := m.ActiveRevision()
	if err != nil {
		t.Fatal(err)
	}
	if activeAfter != first.RevisionID {
		t.Fatalf("active=%s want known-good %s", activeAfter, first.RevisionID)
	}

	restored, err := os.ReadFile(m.ActiveConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(restored)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "dry-run" {
		t.Fatalf("interrupted candidate survived startup recovery: mode=%s", cfg.Mode)
	}

	candidateManifest, err := m.Manifest(candidate.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if candidateManifest.Status != RevisionCandidate {
		t.Fatalf("unexpected candidate status=%s", candidateManifest.Status)
	}
}

func TestRecoverForStartupAllowsUnbootstrappedConfig(t *testing.T) {
	m := newManager(t)
	if err := os.MkdirAll(filepath.Dir(m.ActiveConfigPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.ActiveConfigPath, validJSON(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := m.RecoverForStartup(context.Background(), nil); err != nil {
		t.Fatalf("first-install config should remain usable before bootstrap: %v", err)
	}
}
