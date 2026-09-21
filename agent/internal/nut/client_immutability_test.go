package nut

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestClientReadPathsDoNotMutateDefaults(t *testing.T) {
	runner := &fakeRunner{out: []byte("ups.status: OL\n")}
	client := &Client{Runner: runner}
	if _, err := client.Query(context.Background(), "ups@localhost"); err != nil {
		t.Fatal(err)
	}
	if client.UPSCPath != "" || client.UPSMonPath != "" || client.Timeout != 0 {
		t.Fatalf("Query mutated client defaults: %+v", client)
	}
	if runner.name != "upsc" {
		t.Fatalf("Query default executable=%q", runner.name)
	}

	conf := filepath.Join(t.TempDir(), "upsmon.conf")
	if err := os.WriteFile(conf, []byte("MONITOR ups@localhost 1 mon secret primary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner.out = nil
	if err := client.RequestFSD(context.Background(), "ups", conf); err != nil {
		t.Fatal(err)
	}
	if client.UPSCPath != "" || client.UPSMonPath != "" || client.Timeout != 0 {
		t.Fatalf("RequestFSD mutated client defaults: %+v", client)
	}
	if runner.name != "upsmon" {
		t.Fatalf("FSD default executable=%q", runner.name)
	}
}
