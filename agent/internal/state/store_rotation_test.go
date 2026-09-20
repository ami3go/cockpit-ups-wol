package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteWithCorruptCurrentPreservesValidPrevious(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	st := New("outage-test", "cfg-test")
	first, err := store.Write(st)
	if err != nil {
		t.Fatal(err)
	}
	first.PowerState = OnBattery
	if _, err := store.Write(first); err != nil {
		t.Fatal(err)
	}

	// previous.json is now the only generation we trust after corrupting the
	// newest current file.
	if err := os.WriteFile(filepath.Join(dir, "current.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	fallback, source, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if source != SourcePrevious || fallback.Sequence != 1 {
		t.Fatalf("source=%s seq=%d want previous seq=1", source, fallback.Sequence)
	}

	fallback.PowerState = OnBattery
	written, err := store.Write(fallback)
	if err != nil {
		t.Fatal(err)
	}
	if written.Sequence != 2 {
		t.Fatalf("new sequence=%d want 2", written.Sequence)
	}

	// The old implementation deleted the valid previous.json and moved the
	// corrupt current file into its place. A successful write would therefore
	// leave no usable fallback generation. This assertion protects the exact
	// crash-consistency invariant required by BOOT_RECONCILE.
	previous, err := readAndValidate(filepath.Join(dir, "previous.json"))
	if err != nil {
		t.Fatalf("valid fallback was destroyed: %v", err)
	}
	if previous.Sequence != 1 {
		t.Fatalf("previous sequence=%d want 1", previous.Sequence)
	}

	current, err := readAndValidate(filepath.Join(dir, "current.json"))
	if err != nil {
		t.Fatalf("new current is invalid: %v", err)
	}
	if current.Sequence != 2 {
		t.Fatalf("current sequence=%d want 2", current.Sequence)
	}
}
