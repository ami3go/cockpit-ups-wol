package system

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func Notify(message string) error {
	socket := os.Getenv("NOTIFY_SOCKET")
	if socket == "" {
		return nil
	}
	if strings.HasPrefix(socket, "@") {
		socket = "\x00" + socket[1:]
	}
	addr := &net.UnixAddr{Name: socket, Net: "unixgram"}
	conn, err := net.DialUnix("unixgram", nil, addr)
	if err != nil {
		return fmt.Errorf("systemd notify: %w", err)
	}
	defer conn.Close()
	_, err = conn.Write([]byte(message))
	return err
}

func Ready() error    { return Notify("READY=1") }
func Watchdog() error { return Notify("WATCHDOG=1") }
func Stopping() error { return Notify("STOPPING=1") }

func WatchdogInterval() (time.Duration, bool, error) {
	usec := os.Getenv("WATCHDOG_USEC")
	if usec == "" {
		return 0, false, nil
	}
	if pidText := os.Getenv("WATCHDOG_PID"); pidText != "" {
		pid, err := strconv.Atoi(pidText)
		if err != nil {
			return 0, false, errors.New("invalid WATCHDOG_PID")
		}
		if pid != os.Getpid() {
			return 0, false, nil
		}
	}
	v, err := strconv.ParseInt(usec, 10, 64)
	if err != nil || v <= 0 {
		return 0, false, errors.New("invalid WATCHDOG_USEC")
	}
	return time.Duration(v) * time.Microsecond, true, nil
}

// StartWatchdog feeds systemd only while the caller reports event-loop
// progress through beat. If progress stops for maxSilence, watchdog pings stop
// as well and systemd is allowed to restart the stalled process.
func StartWatchdog(ctx context.Context, beat <-chan struct{}, maxSilence time.Duration) (<-chan error, error) {
	interval, enabled, err := WatchdogInterval()
	if err != nil {
		return nil, err
	}
	ch := make(chan error, 1)
	if !enabled {
		close(ch)
		return ch, nil
	}
	if maxSilence <= 0 || maxSilence >= interval {
		// Keep the liveness threshold inside the configured systemd watchdog
		// window so a stale loop cannot receive another successful ping.
		maxSilence = interval * 3 / 4
	}
	period := interval / 2
	if period <= 0 {
		period = time.Millisecond
	}

	go func() {
		defer close(ch)
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		lastProgress := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-beat:
				if !ok {
					return
				}
				lastProgress = time.Now()
			case <-ticker.C:
				if time.Since(lastProgress) > maxSilence {
					// Deliberately stop feeding systemd. Do not report this as an
					// ordinary process error: the external watchdog is the recovery
					// authority for a stalled event loop.
					return
				}
				if err := Watchdog(); err != nil {
					ch <- err
					return
				}
			}
		}
	}()
	return ch, nil
}
