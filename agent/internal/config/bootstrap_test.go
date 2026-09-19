package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrapKnownGoodIsIdempotentAndRejectsDrift(t *testing.T) {
	content, err := os.ReadFile("../../../config/config.yaml.example")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	active := filepath.Join(dir, "etc", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(active), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(active, content, 0o600); err != nil {
		t.Fatal(err)
	}
	m := &Manager{HistoryDir: filepath.Join(dir, "history"), ActiveConfigPath: active}
	first, err := m.BootstrapKnownGood("installer", content)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != RevisionKnownGood {
		t.Fatalf("unexpected bootstrap status %s", first.Status)
	}
	second, err := m.BootstrapKnownGood("installer", content)
	if err != nil {
		t.Fatal(err)
	}
	if second.RevisionID != first.RevisionID {
		t.Fatalf("idempotent bootstrap created a new revision: %s != %s", second.RevisionID, first.RevisionID)
	}
	drifted := append([]byte(nil), content...)
	drifted = append(drifted, []byte("\n# drift\n")...)
	if _, err := m.BootstrapKnownGood("installer", drifted); err == nil {
		t.Fatal("expected drifted configuration to be rejected")
	}
}
