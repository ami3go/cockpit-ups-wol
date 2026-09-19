package version

import "testing"

func TestCurrent(t *testing.T) {
	got := Current()
	if got.Version == "" || got.Commit == "" || got.Date == "" {
		t.Fatalf("version info must not contain empty fields: %+v", got)
	}
}
