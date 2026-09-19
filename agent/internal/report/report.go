package report

import (
	"fmt"
	"sort"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

type ConfigStatus struct {
	Active            string           `json:"active"`
	LastKnownGood     string           `json:"last_known_good"`
	PreviousKnownGood string           `json:"previous_known_good"`
	ActiveManifest    *config.Manifest `json:"active_manifest,omitempty"`
	LKGManifest       *config.Manifest `json:"last_known_good_manifest,omitempty"`
}

func ReadConfigStatus(m *config.Manager) (ConfigStatus, error) {
	active, err := m.ActiveRevision()
	if err != nil { return ConfigStatus{}, err }
	lkg, err := m.LastKnownGood()
	if err != nil { return ConfigStatus{}, err }
	prev, err := m.PreviousKnownGood()
	if err != nil { return ConfigStatus{}, err }
	out := ConfigStatus{Active: active, LastKnownGood: lkg, PreviousKnownGood: prev}
	if active != "" {
		manifest, err := m.Manifest(active)
		if err != nil { return ConfigStatus{}, fmt.Errorf("active manifest: %w", err) }
		out.ActiveManifest = &manifest
	}
	if lkg != "" {
		manifest, err := m.Manifest(lkg)
		if err != nil { return ConfigStatus{}, fmt.Errorf("last-known-good manifest: %w", err) }
		out.LKGManifest = &manifest
	}
	return out, nil
}

type Plan struct {
	Mode         string            `json:"mode"`
	UPS          UPSPlan           `json:"ups"`
	Outage       OutagePlan        `json:"outage"`
	Recovery     RecoveryPlan      `json:"recovery"`
	Dependencies []DependencyPlan  `json:"network_dependencies"`
	Shutdown     []ShutdownPlan    `json:"shutdown"`
	Restore      []RestorePlan     `json:"restore"`
}

type UPSPlan struct {
	Profile              string `json:"profile"`
	Target               string `json:"target"`
	PowerCycleCapability string `json:"power_cycle_capability"`
	SynologyCompatibility bool  `json:"synology_compatibility"`
}

type OutagePlan struct {
	GracePeriodSeconds     int  `json:"grace_period_seconds"`
	CriticalBatteryPercent *int `json:"critical_battery_percent,omitempty"`
	CriticalRuntimeSeconds *int `json:"critical_runtime_seconds,omitempty"`
	MaxOnBatterySeconds    *int `json:"max_on_battery_seconds,omitempty"`
}

type RecoveryPlan struct {
	Enabled              bool `json:"enabled"`
	UtilityStableSeconds int  `json:"utility_stable_seconds"`
	BatteryChargeMin     *int `json:"battery_charge_min,omitempty"`
	RuntimeMinSeconds    *int `json:"runtime_min_seconds,omitempty"`
	RechargeTimeSeconds  *int `json:"recharge_time_seconds,omitempty"`
	NetworkWaitSeconds   int  `json:"network_wait_seconds"`
}

type DependencyPlan struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Startup  string `json:"startup"`
	Priority int    `json:"priority"`
	Status   string `json:"status_method"`
}

type ShutdownPlan struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Method    string   `json:"method"`
	Priority  int      `json:"priority"`
	DependsOn []string `json:"depends_on,omitempty"`
}

type RestorePlan struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	RestorePolicy string   `json:"restore_policy"`
	WakeEnabled   bool     `json:"wake_enabled"`
	WakePriority  int      `json:"wake_priority"`
	DependsOn     []string `json:"depends_on,omitempty"`
}

func BuildPlan(cfg config.Config) Plan {
	p := Plan{
		Mode: cfg.Mode,
		UPS: UPSPlan{
			Profile: cfg.NUT.Profile,
			Target: fmt.Sprintf("%s@%s:%d", cfg.NUT.UPSName, cfg.NUT.Host, cfg.NUT.Port),
			PowerCycleCapability: cfg.NUT.PowerCycleCapability,
			SynologyCompatibility: cfg.NUT.Synology.Enabled,
		},
		Outage: OutagePlan{
			GracePeriodSeconds: cfg.Outage.GracePeriodSeconds,
			CriticalBatteryPercent: cfg.Outage.CriticalBatteryPercent,
			CriticalRuntimeSeconds: cfg.Outage.CriticalRuntimeSeconds,
			MaxOnBatterySeconds: cfg.Outage.MaxOnBatterySeconds,
		},
		Recovery: RecoveryPlan{
			Enabled: cfg.Recovery.Enabled,
			UtilityStableSeconds: cfg.Recovery.UtilityStableSeconds,
			BatteryChargeMin: cfg.Recovery.BatteryChargeMin,
			RuntimeMinSeconds: cfg.Recovery.RuntimeMinSeconds,
			RechargeTimeSeconds: cfg.Recovery.RechargeTimeSeconds,
			NetworkWaitSeconds: cfg.Recovery.NetworkWaitSeconds,
		},
	}
	for _, d := range cfg.Dependencies {
		p.Dependencies = append(p.Dependencies, DependencyPlan{ID:d.ID, Name:d.Name, Startup:d.Startup, Priority:d.Priority, Status:d.Status.Method})
	}
	for _, h := range cfg.Hosts {
		p.Shutdown = append(p.Shutdown, ShutdownPlan{ID:h.ID, Name:h.Name, Method:h.Shutdown.Method, Priority:h.Shutdown.Priority, DependsOn:append([]string(nil), h.DependsOn...)})
		p.Restore = append(p.Restore, RestorePlan{ID:h.ID, Name:h.Name, RestorePolicy:h.RestorePolicy, WakeEnabled:h.Wake.Enabled, WakePriority:h.Wake.Priority, DependsOn:append([]string(nil), h.DependsOn...)})
	}
	sort.SliceStable(p.Dependencies, func(i,j int) bool { return p.Dependencies[i].Priority < p.Dependencies[j].Priority })
	sort.SliceStable(p.Shutdown, func(i,j int) bool { return p.Shutdown[i].Priority < p.Shutdown[j].Priority })
	sort.SliceStable(p.Restore, func(i,j int) bool { return p.Restore[i].WakePriority < p.Restore[j].WakePriority })
	return p
}
