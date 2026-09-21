package config

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestValidateRevisionIDRejectsTraversalAndMalformedInput(t *testing.T) {
	valid := "cfg-20260921T123456Z-deadbeef"
	if err := ValidateRevisionID(valid); err != nil {
		t.Fatalf("valid revision rejected: %v", err)
	}
	for _, id := range []string{"../etc", "cfg-20260921T123456Z-DEADBEEF", "cfg-20260921-deadbeef", "cfg-20260921T123456Z-deadbeef/child", ""} {
		if err := ValidateRevisionID(id); err == nil {
			t.Fatalf("malformed revision id accepted: %q", id)
		}
	}
}

func TestReadPointerRejectsTamperedRevisionID(t *testing.T) {
	m := newManager(t)
	if err := m.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(m.pointerPath("active"), []byte("../../../tmp/evil\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ActiveRevision(); err == nil || !strings.Contains(err.Error(), "invalid revision id") {
		t.Fatalf("tampered pointer not rejected: %v", err)
	}
}

func TestPruneRevisionsKeepsFloorAndProtectedPointers(t *testing.T) {
	m := newManager(t)
	m.RevisionKeep = 100
	var ids []string
	for i := 0; i < 5; i++ {
		content := append([]byte(nil), validJSON()...)
		content = append(content, []byte(strings.Repeat(" ", i))...)
		revision, err := m.Begin("retention-test", content)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := m.ActivateAndValidate(context.Background(), revision.RevisionID, &fakeRuntime{}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, revision.RevisionID)
	}
	if err := m.writePointer("previous-known-good", ids[0]); err != nil {
		t.Fatal(err)
	}
	if err := m.PruneRevisions(2); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{ids[0], ids[3], ids[4]} {
		if _, err := os.Stat(m.revisionDir(id)); err != nil {
			t.Fatalf("protected/retained revision %s missing: %v", id, err)
		}
	}
	for _, id := range []string{ids[1], ids[2]} {
		if _, err := os.Stat(m.revisionDir(id)); !os.IsNotExist(err) {
			t.Fatalf("old unprotected revision %s was not pruned: %v", id, err)
		}
	}
}
