package config

import (
	"strings"
	"testing"
)

func TestValidateRejectsWakeWithoutIPv4Broadcast(t *testing.T) {
	cfg, err := Parse(validJSON())
	if err != nil {
		t.Fatal(err)
	}
	mac := "AA:BB:CC:DD:EE:FF"
	badBroadcast := "not-an-ip"
	cfg.Hosts = []HostConfig{{
		ID:            "server",
		Name:          "server",
		RestorePolicy: "always",
		Status:        StatusConfig{Method: "none", TimeoutMS: 1000, SuccessConsecutive: 1, ProbeIntervalSeconds: 1},
		Shutdown:      ShutdownConfig{Method: "none", TimeoutSeconds: 120},
		Wake: WakeConfig{
			Enabled:     true,
			MAC:         &mac,
			Broadcast:   &badBroadcast,
			Port:        9,
			MaxAttempts: 1,
		},
	}}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "wake.broadcast must be an IPv4 address") {
		t.Fatalf("invalid broadcast was not rejected: %v", err)
	}
}

func TestValidateRequiresWakeBroadcast(t *testing.T) {
	cfg, err := Parse(validJSON())
	if err != nil {
		t.Fatal(err)
	}
	mac := "AA:BB:CC:DD:EE:FF"
	cfg.Hosts = []HostConfig{{
		ID:            "server",
		Name:          "server",
		RestorePolicy: "always",
		Status:        StatusConfig{Method: "none", TimeoutMS: 1000, SuccessConsecutive: 1, ProbeIntervalSeconds: 1},
		Shutdown:      ShutdownConfig{Method: "none", TimeoutSeconds: 120},
		Wake:          WakeConfig{Enabled: true, MAC: &mac, Port: 9, MaxAttempts: 1},
	}}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "wake.broadcast required when wake is enabled") {
		t.Fatalf("missing broadcast was not rejected: %v", err)
	}
}

func TestValidateRequiresHostAddressWhenStatusEnabled(t *testing.T) {
	cfg, err := Parse(validJSON())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Hosts = []HostConfig{{
		ID:            "server",
		Name:          "server",
		RestorePolicy: "never",
		Status:        StatusConfig{Method: "ping", TimeoutMS: 1000, SuccessConsecutive: 1, ProbeIntervalSeconds: 1},
		Shutdown:      ShutdownConfig{Method: "none", TimeoutSeconds: 120},
	}}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "address is required when status checks are enabled") {
		t.Fatalf("missing host address was not rejected: %v", err)
	}
}

func TestValidateRejectsMalformedHostAddress(t *testing.T) {
	cfg, err := Parse(validJSON())
	if err != nil {
		t.Fatal(err)
	}
	badAddress := "bad host name"
	cfg.Hosts = []HostConfig{{
		ID:            "server",
		Name:          "server",
		Address:       &badAddress,
		RestorePolicy: "never",
		Status:        StatusConfig{Method: "ping", TimeoutMS: 1000, SuccessConsecutive: 1, ProbeIntervalSeconds: 1},
		Shutdown:      ShutdownConfig{Method: "none", TimeoutSeconds: 120},
	}}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "address must be an IP address or DNS hostname") {
		t.Fatalf("malformed host address was not rejected: %v", err)
	}
}

func TestValidateAcceptsDNSHostnameAndIPAddresses(t *testing.T) {
	for _, address := range []string{"server.lan", "192.0.2.10", "2001:db8::10"} {
		cfg, err := Parse(validJSON())
		if err != nil {
			t.Fatal(err)
		}
		addr := address
		cfg.Hosts = []HostConfig{{
			ID:            "server",
			Name:          "server",
			Address:       &addr,
			RestorePolicy: "never",
			Status:        StatusConfig{Method: "ping", TimeoutMS: 1000, SuccessConsecutive: 1, ProbeIntervalSeconds: 1},
			Shutdown:      ShutdownConfig{Method: "none", TimeoutSeconds: 120},
		}}
		if err := Validate(cfg); err != nil {
			t.Fatalf("valid address %q rejected: %v", address, err)
		}
	}
}
