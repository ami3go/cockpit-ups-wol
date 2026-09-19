package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteLoadAndSequence(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	st := New("outage-test", "cfg-test")
	written, err := store.Write(st)
	if err != nil {
		t.Fatal(err)
	}
	if written.Sequence != 1 {
		t.Fatalf("sequence=%d want 1", written.Sequence)
	}
	loaded, source, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if source != SourceCurrent || loaded.Sequence != 1 || loaded.Checksum == "" {
		t.Fatalf("unexpected load: source=%s state=%+v", source, loaded)
	}

	loaded.PowerState = OnBattery
	written2, err := store.Write(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if written2.Sequence != 2 {
		t.Fatalf("sequence=%d want 2", written2.Sequence)
	}
	prev, err := readAndValidate(filepath.Join(dir, "previous.json"))
	if err != nil {
		t.Fatal(err)
	}
	if prev.Sequence != 1 {
		t.Fatalf("previous sequence=%d want 1", prev.Sequence)
	}
}

func TestCorruptCurrentFallsBackToPrevious(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(dir, "current.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, source, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if source != SourcePrevious || loaded.Sequence != 1 {
		t.Fatalf("source=%s seq=%d want previous seq=1", source, loaded.Sequence)
	}
}

func TestBothInvalidReturnsErrNoValidState(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := os.WriteFile(filepath.Join(dir, "current.json"), []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "previous.json"), []byte("also bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := store.Load()
	if !errors.Is(err, ErrNoValidState) {
		t.Fatalf("err=%v want ErrNoValidState", err)
	}
}

func TestChecksumDetectsMutation(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	st := New("outage-test", "cfg-test")
	written, err := store.Write(st)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "current.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	raw["power_state"] = string(OnBattery)
	b, _ = json.Marshal(raw)
	if err := os.WriteFile(filepath.Join(dir, "current.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAndValidate(filepath.Join(dir, "current.json")); err == nil {
		t.Fatal("expected checksum error")
	}
	_ = written
}

func TestRecoveryRequiresShutdownCommit(t *testing.T) {
	st := New("outage-test", "cfg-test")
	st.RecoveryStarted = true
	st.PowerState = RecoveryStarted
	if _, err := NewStore(t.TempDir()).Write(st); err == nil {
		t.Fatal("expected semantic validation error")
	}
}
