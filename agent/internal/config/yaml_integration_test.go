//go:build integrationyaml

package config

import (
	"os"
	"testing"
)

func TestCanonicalYAMLExampleParses(t *testing.T) {
	b, err := os.ReadFile("../../../config/config.yaml.example")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConfigVersion != 1 || cfg.Mode != "dry-run" || cfg.NUT.UPSName != "ups" {
		t.Fatalf("unexpected canonical config: version=%d mode=%q ups=%q", cfg.ConfigVersion, cfg.Mode, cfg.NUT.UPSName)
	}
}
