package system

import (
	"context"
	"net"
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
