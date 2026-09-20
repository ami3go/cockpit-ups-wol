package config

import (
	"strings"
	"testing"
)

func validManagedHost(id string, deps ...string) HostConfig {
	return HostConfig{
		ID:        id,
		Name:      id,
		DependsOn: deps,
		Status: StatusConfig{
			Method:               "none",
			TimeoutMS:            1000,
			SuccessConsecutive:   1,
			ProbeIntervalSeconds: 1,
		},
		Shutdown: ShutdownConfig{Method: "none", TimeoutSeconds: 1},
		Wake: WakeConfig{
			Enabled:                   false,
			DelayAfterPreviousSeconds: 0,
		},
		RestorePolicy: "previous-state",
	}
}

func TestValidateRejectsMultiHostDependencyCycle(t *testing.T) {
	cfg, err := Parse(validJSON())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Hosts = []HostConfig{
		validManagedHost("host-a", "host-b"),
		validManagedHost("host-b", "host-c"),
		validManagedHost("host-c", "host-a"),
	}

	err = Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "recovery dependency cycle detected") {
		t.Fatalf("expected dependency-cycle rejection, got %v", err)
	}
}

func TestValidateAllowsHostDependencyOnExternalNetworkDependency(t *testing.T) {
	cfg, err := Parse(validJSON())
	if err != nil {
		t.Fatal(err)
	}
	addr := "192.0.2.1"
	cfg.Dependencies = []DependencyConfig{{
		ID:      "switch",
		Name:    "switch",
		Address: &addr,
		Status: StatusConfig{
			Method:               "ping",
			TimeoutMS:            1000,
			SuccessConsecutive:   1,
			ProbeIntervalSeconds: 1,
		},
		Startup: "wait-only",
	}}
	cfg.Hosts = []HostConfig{validManagedHost("nas", "switch")}
	if err := Validate(cfg); err != nil {
		t.Fatalf("external dependency must not be treated as a host cycle: %v", err)
	}
}

func TestValidateRejectsNegativeSafetyTiming(t *testing.T) {
	cfg, err := Parse(validJSON())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Outage.CommunicationLossGraceSeconds = -1
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "communication_loss_grace_seconds") {
		t.Fatalf("negative communication-loss grace accepted: %v", err)
	}

	cfg, err = Parse(validJSON())
	if err != nil {
		t.Fatal(err)
	}
	h := validManagedHost("nas")
	h.Wake.DelayAfterPreviousSeconds = -1
	cfg.Hosts = []HostConfig{h}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "delay_after_previous_seconds") {
		t.Fatalf("negative wake delay accepted: %v", err)
	}
}
