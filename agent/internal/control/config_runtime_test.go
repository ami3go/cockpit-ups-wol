package control

import (
	"context"
	"reflect"
	"testing"
)

type recordRunner struct {
	name string
	args []string
}

func (r *recordRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return nil, nil
}
func TestServiceRuntimeApplyUsesFixedSystemctlArgs(t *testing.T) {
	rr := &recordRunner{}
	rt := &ServiceRuntime{Runner: rr, Service: "cockpit-ups-wol-agent.service"}
	if err := rt.Apply(context.Background(), "ignored"); err != nil {
		t.Fatal(err)
	}
	if rr.name != "systemctl" || !reflect.DeepEqual(rr.args, []string{"restart", "cockpit-ups-wol-agent.service"}) {
		t.Fatalf("unexpected command: %s %#v", rr.name, rr.args)
	}
}
