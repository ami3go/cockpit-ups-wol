package host

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

func TestRuntimeArmedExclusionsIsolateMissingSSHKey(t *testing.T) {
	address := "127.0.0.1"
	user := "ups-shutdown"
	key := filepath.Join(t.TempDir(), "missing-key")
	cfg := config.Config{Hosts: []config.HostConfig{{
		ID:      "pc",
		Address: &address,
		Status:  config.StatusConfig{Method: "ping"},
		Shutdown: config.ShutdownConfig{
			Method:     "ssh",
			SSHUser:    &user,
			SSHKeyFile: &key,
		},
	}}}

	excluded, err := RuntimeArmedExclusions(cfg)
	if err != nil {
		t.Fatalf("mutable SSH prerequisite must not be a runtime-fatal capability error: %v", err)
	}
	if excluded["pc"] == "" {
		t.Fatal("missing SSH key did not exclude host")
	}

	runtimeCfg := RuntimeConfigWithExclusions(cfg, excluded)
	if runtimeCfg.Hosts[0].Shutdown.Method != "none" {
		t.Fatalf("runtime shutdown method=%q want none", runtimeCfg.Hosts[0].Shutdown.Method)
	}
	if runtimeCfg.Hosts[0].Wake.Enabled {
		t.Fatal("excluded host must not be automatically woken")
	}
	if cfg.Hosts[0].Shutdown.Method != "ssh" {
		t.Fatal("canonical config was mutated")
	}

	err = ValidateArmedCapabilities(cfg)
	if err == nil {
		t.Fatal("strict human-controlled validation accepted missing SSH key")
	}
	var prereq *PrerequisiteError
	if !errors.As(err, &prereq) || prereq.HostID != "pc" {
		t.Fatalf("strict error=%T %v, want PrerequisiteError for pc", err, err)
	}
}

func TestRuntimeArmedExclusionsKeepStaticCapabilityErrorsFatal(t *testing.T) {
	cfg := config.Config{Hosts: []config.HostConfig{{
		ID:       "pc",
		Status:   config.StatusConfig{Method: "ping"},
		Shutdown: config.ShutdownConfig{Method: "command"},
	}}}
	if _, err := RuntimeArmedExclusions(cfg); err == nil {
		t.Fatal("unsupported command shutdown must remain runtime-fatal")
	}
}
