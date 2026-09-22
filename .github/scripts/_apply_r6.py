from pathlib import Path


def replace(path, old, new, count=1):
    p = Path(path)
    s = p.read_text()
    if old not in s:
        raise SystemExit(f"missing anchor in {path}: {old[:120]!r}")
    p.write_text(s.replace(old, new, count))

# Strict validator delegates to the split static/runtime-prerequisite validator.
p = Path("agent/internal/host/shutdown.go")
s = p.read_text()
start = s.index("// ValidateArmedCapabilities rejects configuration features")
end = s.index("func validateSSHLocalPrerequisites", start)
replacement = '''// PrerequisiteError identifies mutable host-local state that is required for
// safe SSH shutdown. Human-controlled validation treats it as fatal; runtime
// startup can isolate the affected host without disabling the fleet.
type PrerequisiteError struct {
\tHostID string
\tErr    error
}

func (e *PrerequisiteError) Error() string {
\treturn fmt.Sprintf("host %q SSH shutdown prerequisite: %v", e.HostID, e.Err)
}
func (e *PrerequisiteError) Unwrap() error { return e.Err }

// ValidateArmedCapabilities is the strict human/preflight validator. Runtime
// startup uses RuntimeArmedExclusions so mutable SSH state cannot crash-loop the
// complete safety agent.
func ValidateArmedCapabilities(cfg config.Config) error {
\texcluded, err := RuntimeArmedExclusions(cfg)
\tif err != nil {
\t\treturn err
\t}
\tvar errs []error
\tfor _, h := range cfg.Hosts {
\t\treason, ok := excluded[h.ID]
\t\tif !ok {
\t\t\tcontinue
\t\t}
\t\terrs = append(errs, &PrerequisiteError{HostID: h.ID, Err: errors.New(reason)})
\t}
\treturn errors.Join(errs...)
}

'''
p.write_text(s[:start] + replacement + s[end:])

# Armed runtime: isolate mutable prerequisite drift and expose it via health.
old = '''func runArmed(ctx context.Context, cfg config.Config, opts Options) error {
\tif err := host.ValidateArmedCapabilities(cfg); err != nil {
\t\treturn fmt.Errorf("armed capability validation: %w", err)
\t}

\ttarget := nut.Target(cfg.NUT.UPSName, cfg.NUT.Host, cfg.NUT.Port)
\tsupervisor := &health.Supervisor{
\t\tChecks:            []health.Check{nutHealthCheck{client: opts.NUT, target: target}},
\t\tStatePath:         opts.HealthStatePath,
\t\tMaxRepairAttempts: cfg.Health.MaxRepairAttempts,
\t}
'''
new = '''func runArmed(ctx context.Context, cfg config.Config, opts Options) error {
\texcluded, err := host.RuntimeArmedExclusions(cfg)
\tif err != nil {
\t\treturn fmt.Errorf("armed capability validation: %w", err)
\t}
\truntimeCfg := host.RuntimeConfigWithExclusions(cfg, excluded)

\ttarget := nut.Target(cfg.NUT.UPSName, cfg.NUT.Host, cfg.NUT.Port)
\tchecks := []health.Check{nutHealthCheck{client: opts.NUT, target: target}}
\tfor _, h := range cfg.Hosts {
\t\treason, ok := excluded[h.ID]
\t\tif !ok {
\t\t\tcontinue
\t\t}
\t\topts.Log.Error("armed host prerequisite failed; host excluded from direct shutdown and recovery until fixed",
\t\t\t"host_id", h.ID, "error", reason)
\t\tchecks = append(checks, runtimePrerequisiteCheck{hostID: h.ID, reason: reason})
\t}
\tsupervisor := &health.Supervisor{
\t\tChecks:            checks,
\t\tStatePath:         opts.HealthStatePath,
\t\tMaxRepairAttempts: cfg.Health.MaxRepairAttempts,
\t}
'''
replace("agent/internal/app/armed.go", old, new)
replace("agent/internal/app/armed.go", "byID := make(map[string]config.HostConfig, len(cfg.Hosts))\n\tfor _, h := range cfg.Hosts {", "byID := make(map[string]config.HostConfig, len(runtimeCfg.Hosts))\n\tfor _, h := range runtimeCfg.Hosts {")
replace("agent/internal/app/armed.go", "\t\tConfig:           cfg,", "\t\tConfig:           runtimeCfg,")
replace("agent/internal/app/armed.go", "healthSafe := latestHealth.State == health.Healthy && systemSafe", "healthSafe := criticalAgentHealthSafe(latestHealth) && systemSafe")

# CLI validation/application is the human-controlled strict boundary.
p = Path("agent/cmd/cockpit-ups-wolctl/main.go")
s = p.read_text()
s = s.replace('"github.com/ami3go/cockpit-ups-wol/agent/internal/control"\n', '"github.com/ami3go/cockpit-ups-wol/agent/internal/control"\n\t"github.com/ami3go/cockpit-ups-wol/agent/internal/host"\n', 1)
old = '''\t\tcfg, err := config.Parse(content)
\t\tif err != nil {
\t\t\tfatal(err)
\t\t}
\t\tprintJSON(struct {
'''
new = '''\t\tcfg, err := config.Parse(content)
\t\tif err != nil {
\t\t\tfatal(err)
\t\t}
\t\tif cfg.Mode == "armed" {
\t\t\tif err := host.ValidateArmedCapabilities(cfg); err != nil {
\t\t\t\tfatal(err)
\t\t\t}
\t\t}
\t\tprintJSON(struct {
'''
if old not in s:
    raise SystemExit("config-validate anchor missing")
s = s.replace(old, new, 1)
old = '''\t\tcontent, err := readCandidate(os.Stdin)
\t\tif err != nil {
\t\t\tfatal(err)
\t\t}
\t\tmanifest, err := manager.Begin("cockpit", content)
'''
new = '''\t\tcontent, err := readCandidate(os.Stdin)
\t\tif err != nil {
\t\t\tfatal(err)
\t\t}
\t\tcfg, err := config.Parse(content)
\t\tif err != nil {
\t\t\tfatal(err)
\t\t}
\t\tif cfg.Mode == "armed" {
\t\t\tif err := host.ValidateArmedCapabilities(cfg); err != nil {
\t\t\t\tfatal(err)
\t\t\t}
\t\t}
\t\tmanifest, err := manager.Begin("cockpit", content)
'''
if old not in s:
    raise SystemExit("config-apply anchor missing")
s = s.replace(old, new, 1)
p.write_text(s)

# Installer is root and therefore can perform the strict local prerequisite
# check before accepting an armed installation as known-good.
p = Path("scripts/install/validate.sh")
s = p.read_text()
old = 'installation_health_gate(){ log "running post-install health gate"; wait_active cockpit-ups-wol-agent.service 30||die "agent failed startup health gate";'
new = 'installation_health_gate(){ log "running post-install health gate"; if [[ "$OPERATING_MODE" == armed ]]; then "$LIBEXEC_DIR/cockpit-ups-wolctl" config-validate <"$ETC_DIR/config.yaml" >/dev/null||die "armed host prerequisite validation failed"; fi; wait_active cockpit-ups-wol-agent.service 30||die "agent failed startup health gate";'
if old not in s:
    raise SystemExit("installer health gate anchor missing")
p.write_text(s.replace(old, new, 1))
