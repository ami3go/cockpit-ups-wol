package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSynologyPasswordIsExcludedFromJSON(t *testing.T) {
	cfg, err := Parse(validJSON())
	if err != nil {
		t.Fatal(err)
	}
	cfg.NUT.Synology.Enabled = true
	cfg.NUT.Synology.Username = "monuser"
	cfg.NUT.Synology.Password = "unique-test-password"
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if strings.Contains(text, "unique-test-password") || strings.Contains(text, `"password"`) {
		t.Fatalf("Synology password leaked through JSON: %s", text)
	}
}
