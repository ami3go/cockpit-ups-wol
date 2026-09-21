from pathlib import Path


def replace_once(text, old, new, label):
    if old not in text:
        raise SystemExit(f"{label} not found")
    return text.replace(old, new, 1)


p = Path("agent/internal/app/runtime.go")
s = p.read_text()
s = replace_once(s, '''type NUTQuerier interface {
\tQuery(context.Context, string) (nut.Status, error)
}
''', '''type NUTQuerier interface {
\tQuery(context.Context, string) (nut.Status, error)
}

// NUTController covers both status queries and the destructive FSD request.
// Keeping both operations on the injected dependency prevents tests from
// silently falling back to a real upsmon process.
type NUTController interface {
\tNUTQuerier
\tRequestFSD(context.Context, string, string) error
}
''', "NUT interfaces")
s = replace_once(s, '''\tNUT                   NUTQuerier
''', '''\tNUT                   NUTController
''', "Options NUT type")
p.write_text(s)

p = Path("agent/internal/app/armed.go")
s = p.read_text()
s = replace_once(s, '''\tfsdClient := nut.NewClient()
\tif realClient, ok := opts.NUT.(*nut.Client); ok {
\t\tfsdClient = realClient
\t}
''', '', "real FSD fallback")
s = replace_once(s, '''\t\tFSD:              fsdClient,
''', '''\t\tFSD:              opts.NUT,
''', "controller FSD dependency")
s = replace_once(s, '''\tdeps := newDependencyTracker(cfg.Dependencies)
''', '''\tdeps := newDependencyTracker(cfg.Dependencies)
\tcontrollerFSD := &controllerFSDTracker{}
''', "FSD tracker construction")
s = s.replace('armedPowerTick(ctx, cfg, opts, controller, deps, latestHealth, fsdClient)', 'armedPowerTick(ctx, cfg, opts, controller, deps, latestHealth, controllerFSD)')
s = replace_once(s, '''func armedPowerTick(ctx context.Context, cfg config.Config, opts Options, controller *orchestrator.Controller, deps *dependencyTracker, latestHealth health.Snapshot, fsdClient *nut.Client) error {''', '''func armedPowerTick(ctx context.Context, cfg config.Config, opts Options, controller *orchestrator.Controller, deps *dependencyTracker, latestHealth health.Snapshot, fsdTracker *controllerFSDTracker) error {''', "armedPowerTick signature")
old = '''\tst := controller.Policy.State()
\tif st.ShutdownCommitted && st.PowerState == state.WaitingForAC && upsStatus.Utility == nut.UtilityOnBattery && cfg.NUT.Profile != "remote-client" && !hasNUTManagedHosts(cfg.Hosts) {
\t\topts.Log.Info("requesting controller FSD", "transaction_id", st.TransactionID, "ups", cfg.NUT.UPSName)
\t\tif err := fsdClient.RequestFSD(ctx, cfg.NUT.UPSName, opts.UPSMonConfPath); err != nil {
\t\t\topts.Log.Error("controller FSD request failed", "transaction_id", st.TransactionID, "error", err)
\t\t\treturn fmt.Errorf("request controller FSD: %w", err)
\t\t}
\t\topts.Log.Info("controller FSD requested", "transaction_id", st.TransactionID)
\t}
'''
new = '''\tst := controller.Policy.State()
\tif st.ShutdownCommitted && st.PowerState == state.WaitingForAC && upsStatus.Utility == nut.UtilityOnBattery && cfg.NUT.Profile != "remote-client" && !hasNUTManagedHosts(cfg.Hosts) && fsdTracker.shouldAttempt(now, st.TransactionID) {
\t\topts.Log.Info("requesting controller FSD", "transaction_id", st.TransactionID, "ups", cfg.NUT.UPSName)
\t\tif err := opts.NUT.RequestFSD(ctx, cfg.NUT.UPSName, opts.UPSMonConfPath); err != nil {
\t\t\tdelay := fsdTracker.markFailure(now, st.TransactionID)
\t\t\topts.Log.Error("controller FSD request failed; retry scheduled", "transaction_id", st.TransactionID, "retry_after", delay, "error", err)
\t\t\treturn nil
\t\t}
\t\tfsdTracker.markSuccess(st.TransactionID)
\t\topts.Log.Info("controller FSD requested", "transaction_id", st.TransactionID)
\t}
'''
s = replace_once(s, old, new, "controller FSD request block")
insert_before = '''func hasNUTManagedHosts(hosts []config.HostConfig) bool {'''
tracker = '''type controllerFSDTracker struct {
\ttransactionID string
\tcompleted     bool
\tattempts      int
\tnextAttempt   time.Time
}

func (t *controllerFSDTracker) resetFor(transactionID string) {
\tif t.transactionID == transactionID {
\t\treturn
\t}
\tt.transactionID = transactionID
\tt.completed = false
\tt.attempts = 0
\tt.nextAttempt = time.Time{}
}

func (t *controllerFSDTracker) shouldAttempt(now time.Time, transactionID string) bool {
\tt.resetFor(transactionID)
\treturn !t.completed && (t.nextAttempt.IsZero() || !now.Before(t.nextAttempt))
}

func (t *controllerFSDTracker) markSuccess(transactionID string) {
\tt.resetFor(transactionID)
\tt.completed = true
\tt.nextAttempt = time.Time{}
}

func (t *controllerFSDTracker) markFailure(now time.Time, transactionID string) time.Duration {
\tt.resetFor(transactionID)
\tt.attempts++
\tdelay := 5 * time.Second
\tfor i := 1; i < t.attempts && delay < time.Minute; i++ {
\t\tdelay *= 2
\t}
\tif delay > time.Minute {
\t\tdelay = time.Minute
\t}
\tt.nextAttempt = now.Add(delay)
\treturn delay
}

'''
if insert_before not in s:
    raise SystemExit("hasNUTManagedHosts not found")
s = s.replace(insert_before, tracker + insert_before, 1)
p.write_text(s)

p = Path("agent/internal/nut/client.go")
s = p.read_text()
s = replace_once(s, '''import (
\t"bufio"
''', '''import (
\t"bufio"
\t"bytes"
''', "bytes import")
s = replace_once(s, '''\tif c.UPSCPath == "" {
\t\tc.UPSCPath = "upsc"
\t}
\tif c.Timeout <= 0 {
\t\tc.Timeout = 5 * time.Second
\t}
\tctx, cancel := context.WithTimeout(ctx, c.Timeout)
\tdefer cancel()
\tout, err := c.Runner.Run(ctx, c.UPSCPath, target)
''', '''\tupscPath := c.UPSCPath
\tif upscPath == "" {
\t\tupscPath = "upsc"
\t}
\ttimeout := c.Timeout
\tif timeout <= 0 {
\t\ttimeout = 5 * time.Second
\t}
\tctx, cancel := context.WithTimeout(ctx, timeout)
\tdefer cancel()
\tout, err := c.Runner.Run(ctx, upscPath, target)
''', "Query defaults")
s = replace_once(s, '''\ts := bufio.NewScanner(strings.NewReader(string(data)))
''', '''\ts := bufio.NewScanner(bytes.NewReader(data))
''', "ParseUPSC reader")
p.write_text(s)

p = Path("agent/internal/nut/fsd.go")
s = p.read_text()
s = replace_once(s, '''\tif c.UPSMonPath == "" {
\t\tc.UPSMonPath = "upsmon"
\t}
\ttimeout := c.Timeout
''', '''\tupsmonPath := c.UPSMonPath
\tif upsmonPath == "" {
\t\tupsmonPath = "upsmon"
\t}
\ttimeout := c.Timeout
''', "FSD defaults")
s = replace_once(s, '''\tout, err := c.Runner.Run(ctx, c.UPSMonPath, "-c", "fsd")
''', '''\tout, err := c.Runner.Run(ctx, upsmonPath, "-c", "fsd")
''', "FSD runner path")
p.write_text(s)

p = Path("agent/internal/app/runtime_test.go")
s = p.read_text()
s = replace_once(s, '''func (f fakeNUT) Query(context.Context, string) (nut.Status, error) { return f.status, f.err }
''', '''func (f fakeNUT) Query(context.Context, string) (nut.Status, error) { return f.status, f.err }
func (f fakeNUT) RequestFSD(context.Context, string, string) error       { return f.err }
''', "fakeNUT RequestFSD")
p.write_text(s)

Path("agent/internal/app/fsd_tracker_test.go").write_text(r'''package app

import (
    "testing"
    "time"
)

func TestControllerFSDTrackerBacksOffAndResetsPerTransaction(t *testing.T) {
    now := time.Unix(1000, 0)
    tracker := &controllerFSDTracker{}
    if !tracker.shouldAttempt(now, "tx-1") {
        t.Fatal("first FSD attempt should be allowed")
    }
    delay := tracker.markFailure(now, "tx-1")
    if delay != 5*time.Second {
        t.Fatalf("first retry delay=%v", delay)
    }
    if tracker.shouldAttempt(now.Add(4*time.Second), "tx-1") {
        t.Fatal("FSD retry ignored backoff")
    }
    if !tracker.shouldAttempt(now.Add(5*time.Second), "tx-1") {
        t.Fatal("FSD retry did not become eligible")
    }
    tracker.markSuccess("tx-1")
    if tracker.shouldAttempt(now.Add(time.Hour), "tx-1") {
        t.Fatal("successful FSD should not repeat in same transaction")
    }
    if !tracker.shouldAttempt(now.Add(time.Hour), "tx-2") {
        t.Fatal("new outage transaction must reset FSD tracker")
    }
}
''')

Path("agent/internal/nut/client_immutability_test.go").write_text(r'''package nut

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
''')
