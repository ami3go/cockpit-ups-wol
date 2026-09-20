package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	ConfigVersion int                `yaml:"config_version" json:"config_version"`
	Mode          string             `yaml:"mode" json:"mode"`
	NUT           NUTConfig          `yaml:"nut" json:"nut"`
	Outage        OutageConfig       `yaml:"outage" json:"outage"`
	Controller    ControllerConfig   `yaml:"controller" json:"controller"`
	Recovery      RecoveryConfig     `yaml:"recovery" json:"recovery"`
	Health        HealthConfig       `yaml:"health" json:"health"`
	Dependencies  []DependencyConfig `yaml:"network_dependencies" json:"network_dependencies"`
	Hosts         []HostConfig       `yaml:"hosts" json:"hosts"`
}

type NUTConfig struct {
	Profile              string                `yaml:"profile" json:"profile"`
	UPSName              string                `yaml:"ups_name" json:"ups_name"`
	Host                 string                `yaml:"host" json:"host"`
	Port                 int                   `yaml:"port" json:"port"`
	Driver               *string               `yaml:"driver" json:"driver"`
	DriverPort           *string               `yaml:"driver_port" json:"driver_port"`
	PowerCycleCapability string                `yaml:"power_cycle_capability" json:"power_cycle_capability"`
	Network              NUTNetworkConfig      `yaml:"network" json:"network"`
	Synology             SynologyCompatibility `yaml:"synology_compatibility" json:"synology_compatibility"`
	HostSyncSeconds      int                   `yaml:"hosts_sync_seconds" json:"hosts_sync_seconds"`
	FinalDelaySeconds    int                   `yaml:"final_delay_seconds" json:"final_delay_seconds"`
}

type NUTNetworkConfig struct {
	Mode           string   `yaml:"mode" json:"mode"`
	ListenIPv4     bool     `yaml:"listen_ipv4" json:"listen_ipv4"`
	ListenIPv6     bool     `yaml:"listen_ipv6" json:"listen_ipv6"`
	AllowedClients []string `yaml:"allowed_clients" json:"allowed_clients"`
}

type SynologyCompatibility struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

type OutageConfig struct {
	GracePeriodSeconds            int  `yaml:"grace_period_seconds" json:"grace_period_seconds"`
	MaxOnBatterySeconds           *int `yaml:"max_on_battery_seconds" json:"max_on_battery_seconds"`
	CriticalBatteryPercent        *int `yaml:"critical_battery_percent" json:"critical_battery_percent"`
	CriticalRuntimeSeconds        *int `yaml:"critical_runtime_seconds" json:"critical_runtime_seconds"`
	CommunicationLossGraceSeconds int  `yaml:"communication_loss_grace_seconds" json:"communication_loss_grace_seconds"`
}

type ControllerConfig struct {
	RequireUPSBackedPower bool `yaml:"require_ups_backed_power" json:"require_ups_backed_power"`
	RequireAutoPowerOn    bool `yaml:"require_auto_power_on" json:"require_auto_power_on"`
}

type RecoveryConfig struct {
	Enabled              bool `yaml:"enabled" json:"enabled"`
	UtilityStableSeconds int  `yaml:"utility_stable_seconds" json:"utility_stable_seconds"`
	BatteryChargeMin     *int `yaml:"battery_charge_min" json:"battery_charge_min"`
	RuntimeMinSeconds    *int `yaml:"runtime_min_seconds" json:"runtime_min_seconds"`
	RechargeTimeSeconds  *int `yaml:"recharge_time_seconds" json:"recharge_time_seconds"`
	NetworkWaitSeconds   int  `yaml:"network_wait_seconds" json:"network_wait_seconds"`
}

type HealthConfig struct {
	Enabled           bool `yaml:"enabled" json:"enabled"`
	IntervalSeconds   int  `yaml:"interval_seconds" json:"interval_seconds"`
	ProbationSeconds  int  `yaml:"probation_seconds" json:"probation_seconds"`
	Autofix           bool `yaml:"autofix" json:"autofix"`
	MaxRepairAttempts int  `yaml:"max_repair_attempts" json:"max_repair_attempts"`
}

type DependencyConfig struct {
	ID       string       `yaml:"id" json:"id"`
	Name     string       `yaml:"name" json:"name"`
	Address  *string      `yaml:"address" json:"address"`
	Status   StatusConfig `yaml:"status" json:"status"`
	Priority int          `yaml:"priority" json:"priority"`
	Startup  string       `yaml:"startup" json:"startup"`
	Wake     WakeConfig   `yaml:"wake" json:"wake"`
}

type HostConfig struct {
	ID            string         `yaml:"id" json:"id"`
	Name          string         `yaml:"name" json:"name"`
	Address       *string        `yaml:"address" json:"address"`
	DependsOn     []string       `yaml:"depends_on" json:"depends_on"`
	Status        StatusConfig   `yaml:"status" json:"status"`
	Shutdown      ShutdownConfig `yaml:"shutdown" json:"shutdown"`
	Wake          WakeConfig     `yaml:"wake" json:"wake"`
	RestorePolicy string         `yaml:"restore_policy" json:"restore_policy"`
}

type StatusConfig struct {
	Method               string `yaml:"method" json:"method"`
	Port                 *int   `yaml:"port" json:"port"`
	TimeoutMS            int    `yaml:"timeout_ms" json:"timeout_ms"`
	SuccessConsecutive   int    `yaml:"success_consecutive" json:"success_consecutive"`
	ProbeIntervalSeconds int    `yaml:"probe_interval_seconds" json:"probe_interval_seconds"`
}

type ShutdownConfig struct {
	Method         string  `yaml:"method" json:"method"`
	Priority       int     `yaml:"priority" json:"priority"`
	TimeoutSeconds int     `yaml:"timeout_seconds" json:"timeout_seconds"`
	SSHUser        *string `yaml:"ssh_user" json:"ssh_user"`
	SSHKeyFile     *string `yaml:"ssh_key_file" json:"ssh_key_file"`
	CommandID      *string `yaml:"command_id" json:"command_id"`
}

type WakeConfig struct {
	Enabled                   bool    `yaml:"enabled" json:"enabled"`
	MAC                       *string `yaml:"mac" json:"mac"`
	Interface                 *string `yaml:"interface" json:"interface"`
	Broadcast                 *string `yaml:"broadcast" json:"broadcast"`
	Port                      int     `yaml:"port" json:"port"`
	Priority                  int     `yaml:"priority" json:"priority"`
	DelayAfterPreviousSeconds int     `yaml:"delay_after_previous_seconds" json:"delay_after_previous_seconds"`
	MaxAttempts               int     `yaml:"max_attempts" json:"max_attempts"`
}

func Parse(data []byte) (Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, errors.New("multiple YAML documents are not allowed")
		}
		return Config{}, fmt.Errorf("decode trailing config: %w", err)
	}
	applyDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Validate(cfg Config) error {
	var problems []string
	if cfg.ConfigVersion != 1 {
		problems = append(problems, "config_version must be 1")
	}
	if !oneOf(cfg.Mode, "monitor", "dry-run", "armed", "maintenance") {
		problems = append(problems, "invalid mode")
	}
	if !oneOf(cfg.NUT.Profile, "local-server", "remote-client", "existing") {
		problems = append(problems, "invalid nut.profile")
	}
	if strings.TrimSpace(cfg.NUT.UPSName) == "" {
		problems = append(problems, "nut.ups_name is required")
	}
	if strings.TrimSpace(cfg.NUT.Host) == "" {
		problems = append(problems, "nut.host is required")
	}
	if cfg.NUT.Port < 1 || cfg.NUT.Port > 65535 {
		problems = append(problems, "nut.port must be 1..65535")
	}
	if !oneOf(cfg.NUT.PowerCycleCapability, "POWER_CYCLE_VERIFIED", "POWER_CYCLE_UNVERIFIED", "MONITOR_ONLY") {
		problems = append(problems, "invalid nut.power_cycle_capability")
	}
	if !oneOf(cfg.NUT.Network.Mode, "trusted-lan", "restricted") {
		problems = append(problems, "invalid nut.network.mode")
	}
	if cfg.NUT.Network.Mode == "restricted" && len(cfg.NUT.Network.AllowedClients) == 0 {
		problems = append(problems, "restricted NUT mode requires allowed_clients")
	}
	if cfg.NUT.HostSyncSeconds < 1 {
		problems = append(problems, "nut.hosts_sync_seconds must be > 0")
	}
	if cfg.NUT.FinalDelaySeconds < 1 {
		problems = append(problems, "nut.final_delay_seconds must be > 0")
	}
	if !cfg.Controller.RequireUPSBackedPower {
		problems = append(problems, "controller.require_ups_backed_power must remain true")
	}
	if !cfg.Controller.RequireAutoPowerOn {
		problems = append(problems, "controller.require_auto_power_on must remain true")
	}
	if cfg.Outage.GracePeriodSeconds < 0 {
		problems = append(problems, "outage.grace_period_seconds must be >= 0")
	}
	if cfg.Outage.CommunicationLossGraceSeconds < 0 {
		problems = append(problems, "outage.communication_loss_grace_seconds must be >= 0")
	}
	validateOptionalPercent(&problems, "outage.critical_battery_percent", cfg.Outage.CriticalBatteryPercent)
	validateOptionalPositive(&problems, "outage.critical_runtime_seconds", cfg.Outage.CriticalRuntimeSeconds)
	validateOptionalPositive(&problems, "outage.max_on_battery_seconds", cfg.Outage.MaxOnBatterySeconds)
	if cfg.Recovery.Enabled {
		if cfg.Recovery.UtilityStableSeconds < 1 {
			problems = append(problems, "recovery.utility_stable_seconds must be > 0")
		}
		if cfg.Recovery.NetworkWaitSeconds < 0 {
			problems = append(problems, "recovery.network_wait_seconds must be >= 0")
		}
		validateOptionalPercent(&problems, "recovery.battery_charge_min", cfg.Recovery.BatteryChargeMin)
		validateOptionalPositive(&problems, "recovery.runtime_min_seconds", cfg.Recovery.RuntimeMinSeconds)
		validateOptionalPositive(&problems, "recovery.recharge_time_seconds", cfg.Recovery.RechargeTimeSeconds)
		if cfg.Recovery.BatteryChargeMin == nil && cfg.Recovery.RuntimeMinSeconds == nil && cfg.Recovery.RechargeTimeSeconds == nil {
			problems = append(problems, "recovery requires charge, runtime, recharge-time, or manual policy")
		}
	}
	if !cfg.Health.Enabled {
		problems = append(problems, "health.enabled must remain true in v0.1")
	}
	if cfg.Health.IntervalSeconds < 10 {
		problems = append(problems, "health.interval_seconds must be >= 10")
	}
	if cfg.Health.ProbationSeconds < 10 {
		problems = append(problems, "health.probation_seconds must be >= 10")
	}
	if cfg.Health.MaxRepairAttempts < 1 {
		problems = append(problems, "health.max_repair_attempts must be >= 1")
	}

	ids := map[string]string{}
	for i, d := range cfg.Dependencies {
		path := fmt.Sprintf("network_dependencies[%d]", i)
		validateID(&problems, ids, d.ID, path)
		if !oneOf(d.Startup, "auto-power", "wait-only", "wol") {
			problems = append(problems, path+".startup invalid")
		}
		validateStatus(&problems, path+".status", d.Status)
		validateWake(&problems, path+".wake", d.Wake)
		if d.Status.Method != "none" && nilOrEmpty(d.Address) {
			problems = append(problems, path+".address is required when status checks are enabled")
		}
		if d.Startup == "wol" && !d.Wake.Enabled {
			problems = append(problems, path+" startup=wol requires wake.enabled")
		}
	}
	for i, h := range cfg.Hosts {
		path := fmt.Sprintf("hosts[%d]", i)
		validateID(&problems, ids, h.ID, path)
		if strings.TrimSpace(h.Name) == "" {
			problems = append(problems, path+".name is required")
		}
		if !oneOf(h.RestorePolicy, "previous-state", "always", "never") {
			problems = append(problems, path+".restore_policy invalid")
		}
		validateStatus(&problems, path+".status", h.Status)
		validateWake(&problems, path+".wake", h.Wake)
		if !oneOf(h.Shutdown.Method, "nut", "ssh", "command", "none") {
			problems = append(problems, path+".shutdown.method invalid")
		}
		if h.Shutdown.TimeoutSeconds < 1 {
			problems = append(problems, path+".shutdown.timeout_seconds must be > 0")
		}
		if h.Shutdown.Method == "ssh" && (nilOrEmpty(h.Shutdown.SSHUser) || nilOrEmpty(h.Shutdown.SSHKeyFile)) {
			problems = append(problems, path+" ssh shutdown requires ssh_user and ssh_key_file")
		}
		if h.Shutdown.Method == "command" && nilOrEmpty(h.Shutdown.CommandID) {
			problems = append(problems, path+" command shutdown requires command_id")
		}
	}
	for i, h := range cfg.Hosts {
		for _, dep := range h.DependsOn {
			if _, ok := ids[dep]; !ok {
				problems = append(problems, fmt.Sprintf("hosts[%d].depends_on references unknown id %q", i, dep))
			}
			if dep == h.ID {
				problems = append(problems, fmt.Sprintf("hosts[%d] cannot depend on itself", i))
			}
		}
	}
	validateHostDependencyCycles(&problems, cfg.Hosts)

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.NUT.Host == "" {
		cfg.NUT.Host = "localhost"
	}
	if cfg.NUT.Port == 0 {
		cfg.NUT.Port = 3493
	}
	if cfg.NUT.PowerCycleCapability == "" {
		cfg.NUT.PowerCycleCapability = "POWER_CYCLE_UNVERIFIED"
	}
	if cfg.NUT.Network.Mode == "" {
		cfg.NUT.Network.Mode = "trusted-lan"
	}
	if cfg.NUT.HostSyncSeconds == 0 {
		cfg.NUT.HostSyncSeconds = 60
	}
	if cfg.NUT.FinalDelaySeconds == 0 {
		cfg.NUT.FinalDelaySeconds = 15
	}
	if cfg.NUT.Synology.Enabled {
		if cfg.NUT.Synology.Username == "" {
			cfg.NUT.Synology.Username = "monuser"
		}
		if cfg.NUT.Synology.Password == "" {
			cfg.NUT.Synology.Password = "secret"
		}
	}
	if cfg.Health.IntervalSeconds == 0 {
		cfg.Health.IntervalSeconds = 60
	}
	if cfg.Health.ProbationSeconds == 0 {
		cfg.Health.ProbationSeconds = 60
	}
	if cfg.Health.MaxRepairAttempts == 0 {
		cfg.Health.MaxRepairAttempts = 5
	}
	for i := range cfg.Dependencies {
		applyStatusDefaults(&cfg.Dependencies[i].Status)
		applyWakeDefaults(&cfg.Dependencies[i].Wake)
	}
	for i := range cfg.Hosts {
		applyStatusDefaults(&cfg.Hosts[i].Status)
		applyWakeDefaults(&cfg.Hosts[i].Wake)
		if cfg.Hosts[i].Shutdown.TimeoutSeconds == 0 {
			cfg.Hosts[i].Shutdown.TimeoutSeconds = 120
		}
	}
}

func applyStatusDefaults(s *StatusConfig) {
	if s.TimeoutMS == 0 {
		s.TimeoutMS = 1000
	}
	if s.SuccessConsecutive == 0 {
		s.SuccessConsecutive = 3
	}
	if s.ProbeIntervalSeconds == 0 {
		s.ProbeIntervalSeconds = 5
	}
}

func applyWakeDefaults(w *WakeConfig) {
	if !w.Enabled {
		return
	}
	if w.Port == 0 {
		w.Port = 9
	}
	if w.MaxAttempts == 0 {
		w.MaxAttempts = 5
	}
	if w.DelayAfterPreviousSeconds == 0 {
		w.DelayAfterPreviousSeconds = 30
	}
}

func validateID(problems *[]string, ids map[string]string, id, path string) {
	if strings.TrimSpace(id) == "" {
		*problems = append(*problems, path+".id is required")
		return
	}
	if prev, ok := ids[id]; ok {
		*problems = append(*problems, fmt.Sprintf("duplicate id %q at %s and %s", id, prev, path))
		return
	}
	ids[id] = path
}

func validateStatus(problems *[]string, path string, s StatusConfig) {
	if !oneOf(s.Method, "auto", "ping", "tcp", "arp", "none") {
		*problems = append(*problems, path+".method invalid")
	}
	if s.Method == "tcp" && (s.Port == nil || *s.Port < 1 || *s.Port > 65535) {
		*problems = append(*problems, path+" TCP method requires valid port")
	}
	if s.TimeoutMS < 100 {
		*problems = append(*problems, path+".timeout_ms must be >= 100")
	}
	if s.SuccessConsecutive < 1 {
		*problems = append(*problems, path+".success_consecutive must be >= 1")
	}
	if s.ProbeIntervalSeconds < 1 {
		*problems = append(*problems, path+".probe_interval_seconds must be >= 1")
	}
}

func validateWake(problems *[]string, path string, w WakeConfig) {
	if w.DelayAfterPreviousSeconds < 0 {
		*problems = append(*problems, path+".delay_after_previous_seconds must be >= 0")
	}
	if w.Enabled {
		if w.Port < 1 || w.Port > 65535 {
			*problems = append(*problems, path+".port must be 1..65535")
		}
		if w.MaxAttempts < 1 {
			*problems = append(*problems, path+".max_attempts must be >= 1")
		}
		if nilOrEmpty(w.MAC) {
			*problems = append(*problems, path+".mac required when enabled")
		} else if mac, err := net.ParseMAC(*w.MAC); err != nil || len(mac) != 6 {
			*problems = append(*problems, path+".mac invalid")
		}
	}
}

func validateHostDependencyCycles(problems *[]string, hosts []HostConfig) {
	graph := make(map[string][]string, len(hosts))
	for _, h := range hosts {
		graph[h.ID] = append([]string(nil), h.DependsOn...)
	}

	const (
		unvisited = iota
		visiting
		visited
	)
	marks := make(map[string]int, len(graph))
	stack := make([]string, 0, len(graph))

	var visit func(string) bool
	visit = func(id string) bool {
		switch marks[id] {
		case visiting:
			start := 0
			for i, v := range stack {
				if v == id {
					start = i
					break
				}
			}
			cycle := append(append([]string(nil), stack[start:]...), id)
			*problems = append(*problems, "recovery dependency cycle detected: "+strings.Join(cycle, " -> "))
			return false
		case visited:
			return true
		}

		marks[id] = visiting
		stack = append(stack, id)
		for _, dep := range graph[id] {
			if _, managedHost := graph[dep]; !managedHost {
				continue
			}
			if !visit(dep) {
				return false
			}
		}
		stack = stack[:len(stack)-1]
		marks[id] = visited
		return true
	}

	for id := range graph {
		if marks[id] == unvisited && !visit(id) {
			return
		}
	}
}

func validateOptionalPercent(problems *[]string, path string, v *int) {
	if v != nil && (*v < 1 || *v > 100) {
		*problems = append(*problems, path+" must be 1..100")
	}
}

func validateOptionalPositive(problems *[]string, path string, v *int) {
	if v != nil && *v < 1 {
		*problems = append(*problems, path+" must be > 0")
	}
}

func nilOrEmpty(v *string) bool {
	return v == nil || strings.TrimSpace(*v) == ""
}

func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}
