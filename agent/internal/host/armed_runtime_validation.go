package host

import (
	"fmt"
	"strings"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

// RuntimeArmedExclusions validates capability rules that must remain fatal at
// runtime, while returning mutable per-host SSH prerequisite failures as
// exclusions. A missing key/known-host entry for one workstation must not take
// down UPS monitoring and shutdown protection for the rest of the fleet.
func RuntimeArmedExclusions(cfg config.Config) (map[string]string, error) {
	if err := validateStaticArmedCapabilities(cfg); err != nil {
		return nil, err
	}
	excluded := map[string]string{}
	for _, h := range cfg.Hosts {
		if h.Shutdown.Method != "ssh" {
			continue
		}
		if err := validateSSHLocalPrerequisites(*h.Address, *h.Shutdown.SSHKeyFile); err != nil {
			excluded[h.ID] = err.Error()
		}
	}
	return excluded, nil
}

// validateStaticArmedCapabilities contains only configuration-derived rules.
// These cannot be repaired by continuing in a degraded runtime, so they remain
// fatal both at human validation time and process start.
func validateStaticArmedCapabilities(cfg config.Config) error {
	for _, dep := range cfg.Dependencies {
		if dep.Status.Method == "arp" {
			return fmt.Errorf("network dependency %q uses unsupported armed status method arp", dep.ID)
		}
		if dep.Status.Method == "none" || dep.Address == nil || strings.TrimSpace(*dep.Address) == "" {
			return fmt.Errorf("network dependency %q cannot be verified in armed mode", dep.ID)
		}
		if err := validateEndpoint(*dep.Address); err != nil {
			return fmt.Errorf("network dependency %q: %w", dep.ID, err)
		}
		if dep.Startup == "wol" {
			return fmt.Errorf("network dependency %q uses startup=wol, which is disabled until dependency wake state is durable", dep.ID)
		}
	}
	for _, h := range cfg.Hosts {
		if h.Status.Method == "arp" {
			return fmt.Errorf("host %q uses unsupported armed status method arp", h.ID)
		}
		switch h.Shutdown.Method {
		case "none", "nut":
		case "ssh":
			if h.Address == nil {
				return fmt.Errorf("host %q SSH shutdown has no address", h.ID)
			}
			if err := validateEndpoint(*h.Address); err != nil {
				return fmt.Errorf("host %q: %w", h.ID, err)
			}
			if h.Shutdown.SSHUser == nil || h.Shutdown.SSHKeyFile == nil {
				return fmt.Errorf("host %q SSH shutdown is incomplete", h.ID)
			}
		case "command":
			return fmt.Errorf("host %q uses command shutdown, which is not available until the command registry is implemented", h.ID)
		default:
			return fmt.Errorf("host %q uses unsupported shutdown method %q", h.ID, h.Shutdown.Method)
		}
		if cfg.Recovery.Enabled && h.Wake.Enabled {
			if h.Address == nil || h.Status.Method == "none" {
				return fmt.Errorf("host %q wake is enabled but online state cannot be verified", h.ID)
			}
			if h.Wake.MAC == nil || strings.TrimSpace(*h.Wake.MAC) == "" {
				return fmt.Errorf("host %q wake is enabled without a MAC address", h.ID)
			}
			if h.Wake.Broadcast == nil || strings.TrimSpace(*h.Wake.Broadcast) == "" {
				return fmt.Errorf("host %q wake is enabled without an IPv4 broadcast address", h.ID)
			}
		}
	}
	return nil
}

// RuntimeConfigWithExclusions returns a deep-enough runtime copy of cfg. The
// canonical configuration remains untouched. Excluded hosts are deliberately
// removed from direct shutdown and wake planning for this process because the
// agent cannot prove it can safely shut them down.
func RuntimeConfigWithExclusions(cfg config.Config, excluded map[string]string) config.Config {
	if len(excluded) == 0 {
		return cfg
	}
	out := cfg
	out.Hosts = append([]config.HostConfig(nil), cfg.Hosts...)
	for i := range out.Hosts {
		if _, ok := excluded[out.Hosts[i].ID]; !ok {
			continue
		}
		out.Hosts[i].Shutdown.Method = "none"
		out.Hosts[i].Wake.Enabled = false
	}
	return out
}
