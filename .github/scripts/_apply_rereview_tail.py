from pathlib import Path
import json
import re


def replace(path: str, old: str, new: str, count: int = 1) -> None:
    p = Path(path)
    s = p.read_text()
    if old not in s:
        raise SystemExit(f"missing expected text in {path}: {old[:180]!r}")
    p.write_text(s.replace(old, new, count))


# H7: periodic health snapshot persistence/check errors must fail recovery closed,
# not crash-loop the whole safety daemon.
replace(
    "agent/internal/app/runtime.go",
    '''\tinitial, err := supervisor.Run(ctx, false)\n\tif err != nil {\n\t\treturn fmt.Errorf("initial health check: %w", err)\n\t}\n''',
    '''\tinitial := runAgentHealth(ctx, supervisor, false, opts.Log)\n''',
)
replace(
    "agent/internal/app/runtime.go",
    '''\t\tcase <-healthTicker.C:\n\t\t\tsnap, err := supervisor.Run(ctx, cfg.Health.Autofix)\n\t\t\tif err != nil {\n\t\t\t\topts.Log.Error("health supervisor run failed", "error", err)\n\t\t\t\treturn fmt.Errorf("health supervisor: %w", err)\n\t\t\t}\n\t\t\tnonBlockingBeat(beat)\n''',
    '''\t\tcase <-healthTicker.C:\n\t\t\tsnap := runAgentHealth(ctx, supervisor, cfg.Health.Autofix, opts.Log)\n\t\t\tnonBlockingBeat(beat)\n''',
)
replace(
    "agent/internal/app/runtime.go",
    '''func nonBlockingBeat(ch chan<- struct{}) {\n''',
    '''func runAgentHealth(ctx context.Context, supervisor *health.Supervisor, autofix bool, log *slog.Logger) health.Snapshot {\n\tsnap, err := supervisor.Run(ctx, autofix)\n\tif err == nil {\n\t\treturn snap\n\t}\n\tlog.Error("health supervisor run failed; continuing with critical degraded snapshot", "error", err)\n\tfailure := health.NewSnapshot()\n\tfailure.CheckedAt = time.Now().UTC()\n\tfailure.State = health.Degraded\n\tfailure.Results = []health.Result{{\n\t\tName:       "health.supervisor",\n\t\tOK:         false,\n\t\tCritical:   true,\n\t\tRepairable: false,\n\t\tMessage:    err.Error(),\n\t}}\n\treturn failure\n}\n\nfunc nonBlockingBeat(ch chan<- struct{}) {\n''',
)
replace(
    "agent/internal/app/armed.go",
    '''\tlatestHealth, err := supervisor.Run(ctx, false)\n\tif err != nil {\n\t\treturn fmt.Errorf("initial health check: %w", err)\n\t}\n''',
    '''\tlatestHealth := runAgentHealth(ctx, supervisor, false, opts.Log)\n''',
)
replace(
    "agent/internal/app/armed.go",
    '''\t\tcase <-healthTicker.C:\n\t\t\tlatestHealth, err = supervisor.Run(ctx, cfg.Health.Autofix)\n\t\t\tif err != nil {\n\t\t\t\topts.Log.Error("health supervisor run failed", "error", err)\n\t\t\t\treturn fmt.Errorf("health supervisor: %w", err)\n\t\t\t}\n''',
    '''\t\tcase <-healthTicker.C:\n\t\t\tlatestHealth = runAgentHealth(ctx, supervisor, cfg.Health.Autofix, opts.Log)\n''',
)

# H1: use the same injected logger for NUT-group FSD and include SSH diagnostic output.
p = Path("agent/internal/orchestrator/controller.go")
s = p.read_text()
s = s.replace('"fmt"\n\t"time"', '"fmt"\n\t"log/slog"\n\t"time"', 1)
s = s.replace('\tNewTransactionID func() string\n', '\tNewTransactionID func() string\n\tLog              *slog.Logger\n', 1)
old = '''\t\tattempts := c.maxNUTAttempts(plan.NUTGroup)\n\t\tif attempts >= maxNUTFSDShutdownAttempts {\n\t\t\treturn c.enterNUTGroupFailedSafe(plan.NUTGroup, fmt.Sprintf("NUT FSD exhausted %d attempts; native upsmon low-battery shutdown remains the fallback", maxNUTFSDShutdownAttempts))\n\t\t}\n\t\ttx := c.Policy.State().TransactionID\n\t\tif !c.shouldAttemptNUTFSD(now, tx) {\n\t\t\treturn nil\n\t\t}\n\t\tif err := c.markNUTRequested(plan.NUTGroup); err != nil {\n\t\t\treturn fmt.Errorf("persist NUT FSD request state: %w", err)\n\t\t}\n\t\tif err := c.FSD.RequestFSD(ctx, c.Config.NUT.UPSName, c.upsmonPath()); err != nil {\n\t\t\tpersistErr := c.markNUTError(plan.NUTGroup, err)\n\t\t\tif persistErr != nil {\n\t\t\t\treturn errors.Join(fmt.Errorf("request FSD: %w", err), fmt.Errorf("persist NUT FSD failure state: %w", persistErr))\n\t\t\t}\n\t\t\tc.scheduleNUTFSDRetry(now, tx, attempts+1)\n\t\t\treturn nil\n\t\t}\n\t\tc.clearNUTFSDRetry(tx)\n'''
new = '''\t\tattempts := c.maxNUTAttempts(plan.NUTGroup)\n\t\tif attempts >= maxNUTFSDShutdownAttempts {\n\t\t\treason := fmt.Sprintf("NUT FSD exhausted %d attempts; native upsmon low-battery shutdown remains the fallback", maxNUTFSDShutdownAttempts)\n\t\t\tc.logError("NUT group FSD retries exhausted", "transaction_id", c.Policy.State().TransactionID, "attempts", attempts, "reason", reason)\n\t\t\treturn c.enterNUTGroupFailedSafe(plan.NUTGroup, reason)\n\t\t}\n\t\ttx := c.Policy.State().TransactionID\n\t\tif !c.shouldAttemptNUTFSD(now, tx) {\n\t\t\treturn nil\n\t\t}\n\t\tattempt := attempts + 1\n\t\tc.logInfo("requesting NUT group FSD", "transaction_id", tx, "attempt", attempt, "host_count", len(plan.NUTGroup), "ups", c.Config.NUT.UPSName)\n\t\tif err := c.markNUTRequested(plan.NUTGroup); err != nil {\n\t\t\treturn fmt.Errorf("persist NUT FSD request state: %w", err)\n\t\t}\n\t\tif err := c.FSD.RequestFSD(ctx, c.Config.NUT.UPSName, c.upsmonPath()); err != nil {\n\t\t\tpersistErr := c.markNUTError(plan.NUTGroup, err)\n\t\t\tif persistErr != nil {\n\t\t\t\treturn errors.Join(fmt.Errorf("request FSD: %w", err), fmt.Errorf("persist NUT FSD failure state: %w", persistErr))\n\t\t\t}\n\t\t\tc.scheduleNUTFSDRetry(now, tx, attempt)\n\t\t\tdelay := time.Until(c.nutFSDNextAttempt)\n\t\t\tif delay < 0 {\n\t\t\t\tdelay = 0\n\t\t\t}\n\t\t\tc.logError("NUT group FSD request failed; retry scheduled", "transaction_id", tx, "attempt", attempt, "retry_after", delay.Round(time.Second), "error", err)\n\t\t\treturn nil\n\t\t}\n\t\tc.clearNUTFSDRetry(tx)\n\t\tc.logInfo("NUT group FSD requested", "transaction_id", tx, "attempt", attempt, "host_count", len(plan.NUTGroup))\n'''
if old not in s:
    raise SystemExit("NUT FSD block not found")
s = s.replace(old, new, 1)
insert = '''func (c *Controller) logInfo(message string, attrs ...any) {\n\tif c.Log != nil {\n\t\tc.Log.Info(message, attrs...)\n\t}\n}\n\nfunc (c *Controller) logError(message string, attrs ...any) {\n\tif c.Log != nil {\n\t\tc.Log.Error(message, attrs...)\n\t}\n}\n\n'''
marker = 'func (c *Controller) beginFreshOutage(ctx context.Context) error {'
if marker not in s:
    raise SystemExit("controller insertion marker missing")
s = s.replace(marker, insert + marker, 1)
p.write_text(s)
replace(
    "agent/internal/app/armed.go",
    '''\t\tNewTransactionID: newTransactionID,\n''',
    '''\t\tNewTransactionID: newTransactionID,\n\t\tLog:              opts.Log,\n''',
)
replace(
    "agent/internal/host/shutdown.go",
    '''\tif err != nil {\n\t\tlogger.Error("SSH host shutdown failed", "host_id", h.ID, "error", err)\n''',
    '''\tif err != nil {\n\t\tdiagnostic := strings.TrimSpace(string(out))\n\t\tif len(diagnostic) > 2048 {\n\t\t\tdiagnostic = diagnostic[:2048] + "…"\n\t\t}\n\t\tattrs := []any{"host_id", h.ID, "error", err}\n\t\tif diagnostic != "" {\n\t\t\tattrs = append(attrs, "ssh_output", diagnostic)\n\t\t}\n\t\tlogger.Error("SSH host shutdown failed", attrs...)\n''',
)
replace(
    "agent/internal/host/shutdown.go",
    '''func validateSSHLocalPrerequisites(address, keyPath string) error {\n\tkeyPath = strings.TrimSpace(keyPath)\n''',
    '''func validateSSHLocalPrerequisites(address, keyPath string) error {\n\treturn validateSSHLocalPrerequisitesAt(address, keyPath, knownHostsPath)\n}\n\nfunc validateSSHLocalPrerequisitesAt(address, keyPath, hostsPath string) error {\n\tkeyPath = strings.TrimSpace(keyPath)\n''',
)
replace(
    "agent/internal/host/shutdown.go",
    '''\treturn validateKnownHost(address, knownHostsPath)\n}\n\nfunc validateKnownHost''',
    '''\treturn validateKnownHost(address, hostsPath)\n}\n\nfunc validateKnownHost''',
)

# Tests for H7, H1/H3 and WaitForConsecutive.
Path("agent/internal/app/health_supervisor_error_test.go").write_text(r'''package app

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
)

func TestRunAgentHealthConvertsSupervisorPersistenceErrorToCriticalDegraded(t *testing.T) {
	root := t.TempDir()
	blocking := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blocking, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	supervisor := &health.Supervisor{StatePath: filepath.Join(blocking, "health.json")}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	snap := runAgentHealth(context.Background(), supervisor, false, logger)
	if snap.State != health.Degraded || len(snap.Results) != 1 || snap.Results[0].Name != "health.supervisor" || !snap.Results[0].Critical || snap.Results[0].OK {
		t.Fatalf("unexpected degraded fallback: %+v", snap)
	}
	if logs.Len() == 0 {
		t.Fatal("supervisor persistence error was not logged")
	}
}
''')

Path("agent/internal/host/status_wait_test.go").write_text(r'''package host

import (
	"context"
	"errors"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

type waitRunner struct{ err error }

func (r waitRunner) Run(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, r.err
}

func TestWaitForConsecutiveReturnsAfterRequiredKnownSample(t *testing.T) {
	checker := StatusChecker{Runner: waitRunner{}}
	cfg := config.StatusConfig{Method: "ping", TimeoutMS: 10, SuccessConsecutive: 1}
	if err := checker.WaitForConsecutive(context.Background(), "127.0.0.1", cfg, true); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForConsecutivePropagatesParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	checker := StatusChecker{Runner: waitRunner{}}
	cfg := config.StatusConfig{Method: "ping", TimeoutMS: 10, SuccessConsecutive: 1}
	if err := checker.WaitForConsecutive(ctx, "127.0.0.1", cfg, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
''')

Path("agent/internal/host/ssh_prerequisites_test.go").write_text(r'''package host

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
''')

# M11: make the safety interlock directly testable; Prettier will format the file.
replace(
    "cockpit/src/App.tsx",
    "function SettingsPanel({plan,refresh}",
    "export function SettingsPanel({plan,refresh}",
)
Path("cockpit/src/App.test.tsx").write_text(r'''// @vitest-environment jsdom
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SettingsPanel } from './App';
import { applyConfig, getConfigText, validateConfig, type Plan } from './api';

vi.mock('./api', async () => {
  const actual = await vi.importActual<typeof import('./api')>('./api');
  return {
    ...actual,
    getConfigText: vi.fn(),
    validateConfig: vi.fn(),
    applyConfig: vi.fn(),
  };
});

const plan: Plan = {
  mode: 'dry-run',
  ups: { profile: 'existing', target: 'ups@localhost:3493', power_cycle_capability: 'unknown', synology_compatibility: false },
  outage: { grace_period_seconds: 0 },
  recovery: { enabled: true, utility_stable_seconds: 1, network_wait_seconds: 10 },
  shutdown: [],
  restore: [],
  network_dependencies: [],
};

const status = { active: 'cfg-a', last_known_good: 'cfg-a', previous_known_good: '', known_good_revisions: [] };

describe('SettingsPanel validation/apply guard', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(getConfigText).mockResolvedValue('mode: dry-run\n');
    vi.mocked(validateConfig).mockResolvedValue({ valid: true, plan });
    vi.mocked(applyConfig).mockResolvedValue(status);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
  });

  it('never enables Apply for text changed after validation', async () => {
    render(<SettingsPanel plan={plan} refresh={vi.fn().mockResolvedValue(undefined)} />);
    const editor = await screen.findByLabelText('Canonical YAML configuration');
    fireEvent.change(editor, { target: { value: 'mode: armed\n' } });
    fireEvent.click(screen.getByRole('button', { name: 'Validate candidate' }));
    await waitFor(() => expect(screen.getByRole('button', { name: /Activate/ })).toBeEnabled());

    fireEvent.change(editor, { target: { value: 'mode: armed\n# changed after validation\n' } });
    expect(screen.getByRole('button', { name: /Activate/ })).toBeDisabled();
    expect(applyConfig).not.toHaveBeenCalled();
  });

  it('applies exactly the text that was validated', async () => {
    render(<SettingsPanel plan={plan} refresh={vi.fn().mockResolvedValue(undefined)} />);
    const editor = await screen.findByLabelText('Canonical YAML configuration');
    const candidate = 'mode: armed\n';
    fireEvent.change(editor, { target: { value: candidate } });
    fireEvent.click(screen.getByRole('button', { name: 'Validate candidate' }));
    const activate = screen.getByRole('button', { name: /Activate/ });
    await waitFor(() => expect(activate).toBeEnabled());
    fireEvent.click(activate);
    await waitFor(() => expect(applyConfig).toHaveBeenCalledWith(candidate));
  });
});
''')

# Add formatting and component-test dependencies; npm install in the helper workflow updates the lockfile.
pkg_path = Path("cockpit/package.json")
pkg = json.loads(pkg_path.read_text())
pkg["scripts"]["format:check"] = "prettier --check 'src/**/*.{ts,tsx}'"
pkg_path.write_text(json.dumps(pkg, indent=2) + "\n")
