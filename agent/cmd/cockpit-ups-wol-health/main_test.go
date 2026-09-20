package main

import (
	"reflect"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

func TestRequiredUnitsForLocalServerRestricted(t *testing.T) {
	cfg := config.Config{}
	cfg.NUT.Profile = "local-server"
	cfg.NUT.Network.Mode = "restricted"
	got := requiredUnitsForConfig(cfg)
	want := []string{
		"cockpit-ups-wol-agent.service",
		"cockpit.socket",
		"nut-server.service",
		"nut-monitor.service",
		"cockpit-ups-wol-firewall.service",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("units=%v want %v", got, want)
	}
}

func TestRequiredUnitsForRemoteClient(t *testing.T) {
	cfg := config.Config{}
	cfg.NUT.Profile = "remote-client"
	cfg.NUT.Network.Mode = "trusted-lan"
	got := requiredUnitsForConfig(cfg)
	want := []string{
		"cockpit-ups-wol-agent.service",
		"cockpit.socket",
		"nut-monitor.service",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("units=%v want %v", got, want)
	}
}

func TestRequiredUnitsForExistingNUTDoesNotTakeOwnershipOfNUTUnits(t *testing.T) {
	cfg := config.Config{}
	cfg.NUT.Profile = "existing"
	cfg.NUT.Network.Mode = "trusted-lan"
	got := requiredUnitsForConfig(cfg)
	want := []string{
		"cockpit-ups-wol-agent.service",
		"cockpit.socket",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("units=%v want %v", got, want)
	}
}
