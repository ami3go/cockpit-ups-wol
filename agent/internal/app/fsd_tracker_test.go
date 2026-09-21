package app

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
