package app

import "testing"

func TestConfigRevisionChangesWhenEnabledSynologyPasswordChanges(t *testing.T) {
	first := baseConfig("dry-run")
	first.NUT.Synology.Enabled = true
	first.NUT.Synology.Username = "monuser"
	first.NUT.Synology.Password = "first-secret"
	second := first
	second.NUT.Synology.Password = "second-secret"

	firstRevision, err := configRevision(first)
	if err != nil {
		t.Fatal(err)
	}
	secondRevision, err := configRevision(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstRevision == secondRevision {
		t.Fatal("enabled Synology password change did not change config revision")
	}
}
