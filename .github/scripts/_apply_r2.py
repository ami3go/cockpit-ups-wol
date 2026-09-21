from pathlib import Path


def replace(path: str, old: str, new: str, count: int = 1) -> None:
    p = Path(path)
    s = p.read_text()
    if old not in s:
        raise SystemExit(f"missing expected text in {path}: {old[:160]!r}")
    p.write_text(s.replace(old, new, count))


# The watchdog must withhold stale pings, but stay alive so a recovered loop can resume.
replace(
    "agent/internal/system/systemd_notify.go",
    '''\t\t\tif time.Since(lastProgress) > maxSilence {
\t\t\t\t// Deliberately stop feeding systemd. Do not report this as an
\t\t\t\t// ordinary process error: the external watchdog is the recovery
\t\t\t\t// authority for a stalled event loop.
\t\t\t\treturn
\t\t\t}
''',
    '''\t\t\tif time.Since(lastProgress) > maxSilence {
\t\t\t\t// Withhold this ping while the loop is stale, but keep the
\t\t\t\t// watchdog goroutine alive. If useful progress resumes before
\t\t\t\t// systemd's WatchdogSec expires, feeding resumes automatically.
\t\t\t\tcontinue
\t\t\t}
''',
)

# Any completed status probe is useful event-loop progress.
replace(
    "agent/internal/host/status.go",
    '''type StatusChecker struct {
\tRunner CommandRunner
}
''',
    '''type StatusChecker struct {
\tRunner   CommandRunner
\tProgress func()
}

func (c StatusChecker) progress() {
\tif c.Progress != nil {
\t\tc.Progress()
\t}
}
''',
)
replace(
    "agent/internal/host/status.go",
    '''func (c StatusChecker) Check(ctx context.Context, address string, cfg config.StatusConfig) (ProbeResult, error) {
\tmethod := cfg.Method
''',
    '''func (c StatusChecker) Check(ctx context.Context, address string, cfg config.StatusConfig) (ProbeResult, error) {
\tdefer c.progress()
\tmethod := cfg.Method
''',
)

# SSH may legitimately block for seconds while the remote system shuts down.
replace(
    "agent/internal/host/shutdown.go",
    '''type ShutdownExecutor struct {
\tRunner CommandRunner
\tLog    *slog.Logger
}
''',
    '''type ShutdownExecutor struct {
\tRunner           CommandRunner
\tLog              *slog.Logger
\tProgress         func()
\tProgressInterval time.Duration
}
''',
)
replace(
    "agent/internal/host/shutdown.go",
    '''\tlogger.Info("requesting SSH host shutdown", "host_id", h.ID, "timeout_seconds", int(timeout/time.Second))
\tout, err := runner.Run(runCtx, "ssh", args...)
''',
    '''\tlogger.Info("requesting SSH host shutdown", "host_id", h.ID, "timeout_seconds", int(timeout/time.Second))
\tout, err := e.runWithProgress(runCtx, runner, "ssh", args...)
''',
)
insert_before = '''// ValidateArmedCapabilities rejects configuration features that do not yet have
'''
helper = '''type commandResult struct {
\tout []byte
\terr error
}

func (e ShutdownExecutor) runWithProgress(ctx context.Context, runner CommandRunner, name string, args ...string) ([]byte, error) {
\tif e.Progress == nil {
\t\treturn runner.Run(ctx, name, args...)
\t}
\tinterval := e.ProgressInterval
\tif interval <= 0 {
\t\tinterval = 5 * time.Second
\t}
\te.Progress()
\tdone := make(chan commandResult, 1)
\tgo func() {
\t\tout, err := runner.Run(ctx, name, args...)
\t\tdone <- commandResult{out: out, err: err}
\t}()
\tticker := time.NewTicker(interval)
\tdefer ticker.Stop()
\tfor {
\t\tselect {
\t\tcase result := <-done:
\t\t\te.Progress()
\t\t\treturn result.out, result.err
\t\tcase <-ticker.C:
\t\t\te.Progress()
\t\tcase <-ctx.Done():
\t\t\treturn nil, ctx.Err()
\t\t}
\t}
}

'''
replace("agent/internal/host/shutdown.go", insert_before, helper + insert_before)

# Wire one progress source through every armed-mode prober/checker and SSH executor.
p = Path("agent/internal/app/armed.go")
s = p.read_text()
old = '''\tprobe := armedProber{checker: host.StatusChecker{}}
\tbyID := make(map[string]config.HostConfig, len(cfg.Hosts))
'''
new = '''\tbeat := make(chan struct{}, 1)
\tprogress := func() { nonBlockingBeat(beat) }
\tchecker := host.StatusChecker{Progress: progress}
\tprobe := armedProber{checker: checker}
\tbyID := make(map[string]config.HostConfig, len(cfg.Hosts))
'''
if old not in s:
    raise SystemExit("armed checker construction not found")
s = s.replace(old, new, 1)
s = s.replace(
    '''\t\tChecker: armedRecoveryChecker{hosts: byID, checker: host.StatusChecker{}},
''',
    '''\t\tChecker: armedRecoveryChecker{hosts: byID, checker: checker},
''',
    1,
)
s = s.replace(
    '''\t\tShutdown:         host.ShutdownExecutor{},
''',
    '''\t\tShutdown:         host.ShutdownExecutor{Log: opts.Log, Progress: progress},
''',
    1,
)
s = s.replace(
    '''\tdeps := newDependencyTracker(cfg.Dependencies)
''',
    '''\tdeps := newDependencyTracker(cfg.Dependencies, checker)
''',
    1,
)
s = s.replace(
    '''\tbeat := make(chan struct{}, 1)
\twatchdogCh, err := opts.StartWatchdog(ctx, beat, 20*time.Second)
''',
    '''\twatchdogCh, err := opts.StartWatchdog(ctx, beat, 20*time.Second)
''',
    1,
)
s = s.replace(
    '''func newDependencyTracker(deps []config.DependencyConfig) *dependencyTracker {
\treturn &dependencyTracker{deps: deps, streaks: map[string]int{}}
}
''',
    '''func newDependencyTracker(deps []config.DependencyConfig, checker host.StatusChecker) *dependencyTracker {
\treturn &dependencyTracker{deps: deps, checker: checker, streaks: map[string]int{}}
}
''',
    1,
)
p.write_text(s)

# Replace the old test whose expected behavior was the regression itself.
Path("agent/internal/system/watchdog_heartbeat_test.go").write_text(r'''package system

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartWatchdogStaysAliveAcrossRecoverableStall(t *testing.T) {
	t.Setenv("WATCHDOG_USEC", "200000")
	t.Setenv("WATCHDOG_PID", "")
	t.Setenv("NOTIFY_SOCKET", "")

	ctx, cancel := context.WithCancel(context.Background())
	beat := make(chan struct{}, 1)
	ch, err := StartWatchdog(ctx, beat, 120*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	beat <- struct{}{}
	time.Sleep(280 * time.Millisecond)
	select {
	case _, ok := <-ch:
		if !ok {
			t.Fatal("watchdog goroutine exited permanently during a recoverable stall")
		}
	default:
	}
	beat <- struct{}{}
	time.Sleep(120 * time.Millisecond)
	select {
	case _, ok := <-ch:
		if !ok {
			t.Fatal("watchdog did not survive after progress resumed")
		}
	default:
	}
	cancel()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("watchdog did not stop after context cancellation")
	}
}

func TestWatchdogPingsResumeAfterProgressReturns(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "notify.sock")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: sock, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	pings := make(chan time.Time, 32)
	readCtx, stopRead := context.WithCancel(context.Background())
	defer stopRead()
	go func() {
		buf := make([]byte, 128)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, _, err := conn.ReadFromUnix(buf)
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					select {
					case <-readCtx.Done():
						return
					default:
						continue
					}
				}
				return
			}
			if strings.Contains(string(buf[:n]), "WATCHDOG=1") {
				pings <- time.Now()
			}
		}
	}()

	t.Setenv("NOTIFY_SOCKET", sock)
	t.Setenv("WATCHDOG_USEC", "600000")
	t.Setenv("WATCHDOG_PID", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	beat := make(chan struct{}, 1)
	ch, err := StartWatchdog(ctx, beat, 350*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	beat <- struct{}{}
	select {
	case <-pings:
	case <-time.After(time.Second):
		t.Fatal("initial watchdog ping not observed")
	}

	// Cross maxSilence so at least one scheduled ping is withheld, but then
	// prove useful progress can recover the watchdog before systemd intervenes.
	time.Sleep(450 * time.Millisecond)
	resumed := time.Now()
	deadline := time.After(1200 * time.Millisecond)
	progressTicker := time.NewTicker(80 * time.Millisecond)
	defer progressTicker.Stop()
	for {
		select {
		case beat <- struct{}{}:
		default:
		}
		select {
		case p := <-pings:
			if p.After(resumed) {
				return
			}
		case <-progressTicker.C:
			continue
		case <-deadline:
			t.Fatal("watchdog never resumed pinging after useful progress returned")
		case err, ok := <-ch:
			if !ok {
				t.Fatal("watchdog channel closed during recoverable stall")
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
''')

Path("agent/internal/host/shutdown_progress_test.go").write_text(r'''package host

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

type blockingCommandRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r blockingCommandRunner) Run(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	close(r.started)
	select {
	case <-r.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestSSHShutdownReportsProgressWhileCommandIsRunning(t *testing.T) {
	address := "host.example"
	user := "ups-shutdown"
	key := "/tmp/test-key"
	started := make(chan struct{})
	release := make(chan struct{})
	var beats atomic.Int32
	exec := ShutdownExecutor{
		Runner: blockingCommandRunner{started: started, release: release},
		Progress: func() {
			beats.Add(1)
		},
		ProgressInterval: 20 * time.Millisecond,
	}
	h := config.HostConfig{
		ID:      "pc",
		Address: &address,
		Shutdown: config.ShutdownConfig{
			Method:         "ssh",
			SSHUser:        &user,
			SSHKeyFile:     &key,
			TimeoutSeconds: 2,
		},
	}
	done := make(chan error, 1)
	go func() {
		_, err := exec.Shutdown(context.Background(), h)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("SSH runner did not start")
	}
	time.Sleep(90 * time.Millisecond)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := beats.Load(); got < 4 {
		t.Fatalf("only %d progress beats while SSH command was live", got)
	}
}
''')
