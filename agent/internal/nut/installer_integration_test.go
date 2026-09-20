//go:build integrationnut

package nut

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInstallerGeneratedNUTConfigMatchesFSDAndSynologyPolicy(t *testing.T) {
	repoScript, err := filepath.Abs(filepath.Join("..", "..", "..", "scripts", "install", "nut.sh"))
	if err != nil {
		t.Fatal(err)
	}
	nutDir := filepath.Join(t.TempDir(), "nut")
	cmd := exec.Command("bash", "-c", `set -euo pipefail; source "$1"; UPS_NAME=ups; nut_write_clean_local_server primary-secret 1 usbhid-ups auto 73 21`, "bash", repoScript)
	cmd.Env = append(os.Environ(), "COCKPIT_UPS_WOL_NUT_ETC_DIR="+nutDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate installer NUT config: %v: %s", err, out)
	}

	upsmonPath := filepath.Join(nutDir, "upsmon.conf")
	if err := ValidatePrimaryMonitor(upsmonPath, "ups"); err != nil {
		t.Fatalf("generated controller is not primary: %v", err)
	}
	upsmonBytes, err := os.ReadFile(upsmonPath)
	if err != nil {
		t.Fatal(err)
	}
	upsmonText := string(upsmonBytes)
	for _, required := range []string{"HOSTSYNC 73", "FINALDELAY 21"} {
		if !strings.Contains(upsmonText, required) {
			t.Fatalf("generated upsmon.conf missing %q:\n%s", required, upsmonText)
		}
	}

	users, err := os.ReadFile(filepath.Join(nutDir, "upsd.users"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(users)
	for _, required := range []string{"[ups-primary]", "upsmon primary", "[monuser]", "password = secret", "upsmon secondary"} {
		if !strings.Contains(text, required) {
			t.Fatalf("generated upsd.users missing %q:\n%s", required, text)
		}
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "actions") || strings.Contains(lower, "instcmds") {
		t.Fatalf("Synology compatibility user gained destructive privileges:\n%s", text)
	}

	// Exercise the FSD adapter against the actual installer-generated primary
	// config, but use the existing injected runner so CI can never shut itself
	// down. This verifies both ownership parsing and the exact external command.
	r := &fakeRunner{}
	client := NewClient()
	client.Runner = r
	if err := client.RequestFSD(context.Background(), "ups", upsmonPath); err != nil {
		t.Fatal(err)
	}
	if r.name != "upsmon" || !reflect.DeepEqual(r.args, []string{"-c", "fsd"}) {
		t.Fatalf("unexpected FSD command: %q %v", r.name, r.args)
	}

	// A secondary role must fail closed and must not invoke upsmon at all.
	secondary := "MONITOR ups@localhost 1 mon secret secondary\n"
	if err := os.WriteFile(upsmonPath, []byte(secondary), 0o600); err != nil {
		t.Fatal(err)
	}
	r = &fakeRunner{}
	client.Runner = r
	if err := client.RequestFSD(context.Background(), "ups", upsmonPath); !errors.Is(err, ErrNotPrimary) {
		t.Fatalf("secondary FSD err=%v want ErrNotPrimary", err)
	}
	if r.name != "" {
		t.Fatalf("secondary role invoked destructive command: %q %v", r.name, r.args)
	}

	// upsmon -c fsd is process-wide for all UPSes monitored as primary. Until
	// multi-UPS ownership is explicitly modeled, two primary entries must fail
	// closed before the destructive command is invoked.
	multiPrimary := "MONITOR ups@localhost 1 mon secret primary\nMONITOR other@localhost 1 mon secret primary\n"
	if err := os.WriteFile(upsmonPath, []byte(multiPrimary), 0o600); err != nil {
		t.Fatal(err)
	}
	r = &fakeRunner{}
	client.Runner = r
	if err := client.RequestFSD(context.Background(), "ups", upsmonPath); !errors.Is(err, ErrMultiplePrimaries) {
		t.Fatalf("multi-primary FSD err=%v want ErrMultiplePrimaries", err)
	}
	if r.name != "" {
		t.Fatalf("multi-primary config invoked destructive command: %q %v", r.name, r.args)
	}
}
