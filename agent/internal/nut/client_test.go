package nut

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fakeRunner struct {
	out  []byte
	err  error
	name string
	args []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.name = name
	f.args = append([]string(nil), args...)
	return f.out, f.err
}

func TestParseUPSCOnline(t *testing.T) {
	st, err := ParseUPSC([]byte("ups.status: OL CHRG\nbattery.charge: 87\nbattery.runtime: 1234\n"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Utility != UtilityOnline || st.LowBattery || st.FSD {
		t.Fatalf("unexpected status: %+v", st)
	}
	if st.ChargePercent == nil || *st.ChargePercent != 87 {
		t.Fatalf("charge=%v", st.ChargePercent)
	}
	if st.RuntimeSeconds == nil || *st.RuntimeSeconds != 1234 {
		t.Fatalf("runtime=%v", st.RuntimeSeconds)
	}
}

func TestParseUPSCOnBatteryLow(t *testing.T) {
	st, err := ParseUPSC([]byte("ups.status: OB LB\n"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Utility != UtilityOnBattery || !st.LowBattery {
		t.Fatalf("unexpected status: %+v", st)
	}
	if st.ChargePercent != nil {
		t.Fatal("missing charge must remain unavailable")
	}
}

func TestParseUPSCMissingStatusIsUnknown(t *testing.T) {
	st, err := ParseUPSC([]byte("battery.charge: 90\n"))
	if err == nil || st.Utility != UtilityUnknown {
		t.Fatalf("status=%+v err=%v", st, err)
	}
}

func TestQueryFailureIsUnknown(t *testing.T) {
	r := &fakeRunner{err: errors.New("boom")}
	c := NewClient()
	c.Runner = r
	st, err := c.Query(context.Background(), "ups@localhost")
	if err == nil || st.Utility != UtilityUnknown {
		t.Fatalf("status=%+v err=%v", st, err)
	}
}

func TestTarget(t *testing.T) {
	cases := map[string]string{
		Target("ups", "localhost", 3493): "ups@localhost",
		Target("ups", "10.0.0.2", 3494):  "ups@10.0.0.2:3494",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

func TestValidatePrimaryMonitor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upsmon.conf")
	if err := os.WriteFile(path, []byte("MONITOR ups@localhost 1 mon secret primary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePrimaryMonitor(path, "ups"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePrimaryMonitor(path, "other"); !errors.Is(err, ErrNotPrimary) {
		t.Fatalf("err=%v", err)
	}
}

func TestValidateLegacyMaster(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upsmon.conf")
	if err := os.WriteFile(path, []byte("MONITOR ups@localhost 1 mon secret master\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePrimaryMonitor(path, "ups"); err != nil {
		t.Fatal(err)
	}
}

func TestRequestFSDRequiresPrimaryAndUsesUpsmon(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upsmon.conf")
	if err := os.WriteFile(path, []byte("MONITOR ups@localhost 1 mon secret primary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}
	c := NewClient()
	c.Runner = r
	if err := c.RequestFSD(context.Background(), "ups", path); err != nil {
		t.Fatal(err)
	}
	if r.name != "upsmon" || !reflect.DeepEqual(r.args, []string{"-c", "fsd"}) {
		t.Fatalf("name=%q args=%v", r.name, r.args)
	}
}

func TestRequestFSDSecondaryRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upsmon.conf")
	if err := os.WriteFile(path, []byte("MONITOR ups@localhost 1 mon secret secondary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}
	c := NewClient()
	c.Runner = r
	if err := c.RequestFSD(context.Background(), "ups", path); !errors.Is(err, ErrNotPrimary) {
		t.Fatalf("err=%v", err)
	}
	if r.name != "" {
		t.Fatal("upsmon must not be called when not primary")
	}
}
