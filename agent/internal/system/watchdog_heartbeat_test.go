package system

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestStartWatchdogStopsWhenHeartbeatStalls(t *testing.T) {
	oldUsec := os.Getenv("WATCHDOG_USEC")
	oldPID := os.Getenv("WATCHDOG_PID")
	oldSocket := os.Getenv("NOTIFY_SOCKET")
	defer os.Setenv("WATCHDOG_USEC", oldUsec)
	defer os.Setenv("WATCHDOG_PID", oldPID)
	defer os.Setenv("NOTIFY_SOCKET", oldSocket)

	if err := os.Setenv("WATCHDOG_USEC", "200000"); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("WATCHDOG_PID")
	_ = os.Unsetenv("NOTIFY_SOCKET")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	beat := make(chan struct{}, 1)
	ch, err := StartWatchdog(ctx, beat, 120*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	beat <- struct{}{}

	select {
	case err, ok := <-ch:
		if ok && err != nil {
			t.Fatalf("watchdog returned unexpected error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("watchdog continued running after heartbeat stalled")
	}
}
