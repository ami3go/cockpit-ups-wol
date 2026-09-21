package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecoverForStartupSkipsBusyTransactionLock(t *testing.T) {
	m := newManager(t)

	first, err := m.Begin("test", validJSON())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ActivateAndValidate(context.Background(), first.RevisionID, &fakeRuntime{}); err != nil {
		t.Fatal(err)
	}

	changed := strings.Replace(string(validJSON()), `"mode":"dry-run"`, `"mode":"monitor"`, 1)
	second, err := m.Begin("test", []byte(changed))
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(m.revisionDir(second.RevisionID), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(m.ActiveConfigPath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := m.writePointer("active", second.RevisionID); err != nil {
		t.Fatal(err)
	}
	manifest, err := m.readManifest(second.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Status = RevisionValidating
	if err := m.writeManifest(manifest); err != nil {
		t.Fatal(err)
	}

	// This models config-activate holding the history lock while systemctl
	// restart waits for the new agent to send READY=1.
	unlock, err := m.lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	done := make(chan error, 1)
	go func() {
		done <- m.RecoverForStartup(context.Background(), nil)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("startup recovery while activation lock held: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("startup recovery blocked on the live config transaction lock")
	}

	active, err := m.ActiveRevision()
	if err != nil {
		t.Fatal(err)
	}
	if active != second.RevisionID {
		t.Fatalf("active revision changed during live validation: got %s want %s", active, second.RevisionID)
	}
}
