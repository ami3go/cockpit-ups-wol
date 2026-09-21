package ipc

import (
	"errors"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
)

type fakeHealthProvider struct {
	snap health.Snapshot
	err  error
}

func (f fakeHealthProvider) HealthSnapshot() (health.Snapshot, error) { return f.snap, f.err }
func TestGetHealth(t *testing.T) {
	want := health.NewSnapshot()
	want.State = health.Degraded
	r := Handler{Health: fakeHealthProvider{snap: want}}.Handle(Request{ID: "1", Method: "GetHealth"})
	if !r.OK {
		t.Fatalf("resp=%+v", r)
	}
	got := r.Result.(health.Snapshot)
	if got.State != health.Degraded {
		t.Fatalf("state=%s", got.State)
	}
}
func TestGetHealthError(t *testing.T) {
	r := Handler{Health: fakeHealthProvider{err: errors.New("disk")}}.Handle(Request{ID: "1", Method: "GetHealth"})
	if r.OK || r.Error == nil || r.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("resp=%+v", r)
	}
}
