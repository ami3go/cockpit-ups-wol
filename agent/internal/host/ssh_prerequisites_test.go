package host

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

func TestValidateSSHLocalPrerequisitesAtWithPinnedHost(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	base := filepath.Join(t.TempDir(), "id_ed25519")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", base).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v: %s", err, out)
	}
	pub, err := os.ReadFile(base + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(pub))
	if len(fields) < 2 {
		t.Fatalf("unexpected public key: %q", pub)
	}
	hosts := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(hosts, []byte("host.example "+fields[0]+" "+fields[1]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateSSHLocalPrerequisitesAt("host.example", base, hosts); err != nil {
		t.Fatalf("valid pinned host rejected: %v", err)
	}
	if err := validateKnownHost("missing.example", hosts); err == nil {
		t.Fatal("missing pinned host unexpectedly accepted")
	}
}

type diagnosticRunner struct{}

func (diagnosticRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return []byte("Host key verification failed"), errors.New("ssh exit 255")
}

func TestSSHFailureLogIncludesCommandDiagnostic(t *testing.T) {
	address, user, key := "host.example", "ups", "/tmp/key"
	var buf bytes.Buffer
	executor := ShutdownExecutor{Runner: diagnosticRunner{}, Log: slog.New(slog.NewTextHandler(&buf, nil))}
	_, _ = executor.Shutdown(context.Background(), config.HostConfig{
		ID: "pc", Address: &address,
		Shutdown: config.ShutdownConfig{Method: "ssh", SSHUser: &user, SSHKeyFile: &key, TimeoutSeconds: 1},
	})
	if !strings.Contains(buf.String(), "Host key verification failed") {
		t.Fatalf("diagnostic missing from log: %s", buf.String())
	}
}
