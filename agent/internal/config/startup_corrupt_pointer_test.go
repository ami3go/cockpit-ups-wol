package config

import (
	"context"
	"os"
	"testing"
)

func TestRecoverForStartupRepairsCorruptActivePointer(t *testing.T) {
	m := newManager(t)
	lkg, err := m.BootstrapKnownGood("installer", validJSON())
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(m.pointerPath("active"), []byte("cfg-2026\x00\xff#corrupt\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := m.RecoverForStartup(context.Background(), nil); err != nil {
		t.Fatalf("corrupt active metadata must be repaired, not fatal: %v", err)
	}
	if got, err := m.ActiveRevision(); err != nil || got != lkg.RevisionID {
		t.Fatalf("active=%q err=%v, want last-known-good %q", got, err, lkg.RevisionID)
	}
}

func TestRecoverForStartupScansHistoryWhenKnownGoodPointerIsCorrupt(t *testing.T) {
	m := newManager(t)
	known, err := m.BootstrapKnownGood("installer", validJSON())
	if err != nil {
		t.Fatal(err)
	}

	corrupt := []byte("cfg-2026\x00\xff#corrupt\n")
	if err := os.WriteFile(m.pointerPath("active"), corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.pointerPath("last-known-good"), corrupt, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := m.RecoverForStartup(context.Background(), nil); err != nil {
		t.Fatalf("startup should recover from corrupt active and last-known-good pointers: %v", err)
	}
	if got, err := m.ActiveRevision(); err != nil || got != known.RevisionID {
		t.Fatalf("active=%q err=%v, want scanned known-good %q", got, err, known.RevisionID)
	}
	if got, err := m.LastKnownGood(); err != nil || got != known.RevisionID {
		t.Fatalf("last-known-good=%q err=%v, want repaired pointer %q", got, err, known.RevisionID)
	}
}
