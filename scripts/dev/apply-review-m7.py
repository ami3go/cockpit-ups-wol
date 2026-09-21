from pathlib import Path


def replace_once(text, old, new, label):
    if old not in text:
        raise SystemExit(f"{label} not found")
    return text.replace(old, new, 1)

# Keep compatibility password in YAML, but exclude it from JSON/report surfaces.
p = Path('agent/internal/config/config.go')
s = p.read_text()
s = replace_once(s,
'''type SynologyCompatibility struct {
\tEnabled  bool   `yaml:"enabled" json:"enabled"`
\tUsername string `yaml:"username" json:"username"`
\tPassword string `yaml:"password" json:"password"`
}''',
'''type SynologyCompatibility struct {
\tEnabled  bool   `yaml:"enabled" json:"enabled"`
\tUsername string `yaml:"username" json:"username"`
\tPassword string `yaml:"password" json:"-"`
}''', 'Synology password JSON tag')
p.write_text(s)

# Preserve password sensitivity in the durable runtime config revision without serializing the secret.
p = Path('agent/internal/app/armed.go')
s = p.read_text()
old = '''func configRevision(cfg config.Config) (string, error) {
\tb, err := json.Marshal(cfg)
\tif err != nil {
\t\treturn "", fmt.Errorf("encode config revision: %w", err)
\t}
\tsum := sha256.Sum256(b)
\treturn "sha256:" + hex.EncodeToString(sum[:]), nil
}'''
new = '''func configRevision(cfg config.Config) (string, error) {
\tmaterial := struct {
\t\tConfig                 config.Config `json:"config"`
\t\tSynologyPasswordSHA256 string        `json:"synology_password_sha256,omitempty"`
\t}{Config: cfg}
\tif cfg.NUT.Synology.Enabled {
\t\tdigest := sha256.Sum256([]byte(cfg.NUT.Synology.Password))
\t\tmaterial.SynologyPasswordSHA256 = hex.EncodeToString(digest[:])
\t}
\tb, err := json.Marshal(material)
\tif err != nil {
\t\treturn "", fmt.Errorf("encode config revision: %w", err)
\t}
\tsum := sha256.Sum256(b)
\treturn "sha256:" + hex.EncodeToString(sum[:]), nil
}'''
s = replace_once(s, old, new, 'configRevision')
p.write_text(s)

# Installer requires explicit acknowledgement for the compatibility credentials on trusted LAN.
p = Path('install.sh')
s = p.read_text()
s = replace_once(s, 'SYNOLOGY=0\nPROFILE=local-server\n', 'SYNOLOGY=0\nACCEPT_TRUSTED_LAN_SYNOLOGY=0\nPROFILE=local-server\n', 'installer default')
s = replace_once(s, '''Usage: sudo ./install.sh [--tui|--silent] [--synology]
                         [--profile local-server|remote-client|existing]''', '''Usage: sudo ./install.sh [--tui|--silent] [--synology]
                         [--accept-trusted-lan-synology]
                         [--profile local-server|remote-client|existing]''', 'usage')
s = replace_once(s, '    --synology) SYNOLOGY=1 ;;\n', '    --synology) SYNOLOGY=1 ;;\n    --accept-trusted-lan-synology) ACCEPT_TRUSTED_LAN_SYNOLOGY=1 ;;\n', 'option parser')
p.write_text(s)

p = Path('scripts/install/configuration.sh')
s = p.read_text()
s = replace_once(s, '''  [[ -n "$NUT_HOST" ]] || NUT_HOST=localhost
  [[ "$NUT_HOST" =~ ^[A-Za-z0-9._:-]+$ ]] || die "invalid NUT host"
''', '''  [[ -n "$NUT_HOST" ]] || NUT_HOST=localhost
  [[ "$NUT_HOST" =~ ^[A-Za-z0-9._:-]+$ ]] || die "invalid NUT host"
  if ((SYNOLOGY)) && [[ "$NETWORK_MODE" == trusted-lan ]] && (( ! ${ACCEPT_TRUSTED_LAN_SYNOLOGY:-0} )); then
    die "Synology compatibility uses fixed monitor credentials; choose --network-mode restricted or explicitly pass --accept-trusted-lan-synology"
  fi
''', 'Synology trusted-LAN guard')
p.write_text(s)

p = Path('scripts/install/tui.sh')
s = p.read_text()
s = replace_once(s, '''    else
      NUT_ALLOWED_CLIENTS=()
      NUT_LISTEN_IPV6=0
    fi
  fi
''', '''    else
      NUT_ALLOWED_CLIENTS=()
      NUT_LISTEN_IPV6=0
      if ((SYNOLOGY)); then
        if tui_yesno 'Synology trusted-LAN exposure' 'Synology DSM compatibility uses fixed monitor credentials (monuser/secret). Trusted-LAN mode exposes those monitor-only credentials to any host that can reach NUT TCP 3493.\n\nContinue only if this LAN is intentionally trusted. Restricted mode is recommended.'; then
          ACCEPT_TRUSTED_LAN_SYNOLOGY=1
        else
          tui_msg 'Restricted mode recommended' 'Restart the installer and select restricted NUT network security, then enter the Synology/client CIDRs that may access TCP 3493.'
          exit 1
        fi
      fi
    fi
  fi
''', 'TUI trusted-LAN acknowledgement')
p.write_text(s)

# Strengthen installer unit coverage for both rejected and explicitly accepted trusted-LAN use.
p = Path('scripts/test/installer-unit.sh')
s = p.read_text()
needle = '''# Test-only environment seams must never affect a production root install
'''
block = '''# Synology DSM compatibility uses fixed monitor credentials. Trusted-LAN
# exposure therefore requires an explicit acknowledgement; restricted mode does not.
if (PROFILE=local-server; MODE=default; CHECK_ONLY=0; SILENT=1; SYNOLOGY=1; UPS_NAME=ups; NUT_HOST=localhost; UPS_DRIVER=""; UPS_PORT=""; OPERATING_MODE=dry-run; OUTAGE_GRACE=120; RECOVERY_CHARGE=80; UTILITY_STABLE=120; NETWORK_WAIT=300; NETWORK_MODE=trusted-lan; NUT_LISTEN_IPV4=1; NUT_LISTEN_IPV6=0; NUT_ALLOWED_CLIENTS=(); ACCEPT_TRUSTED_LAN_SYNOLOGY=0; resolve_profile_inputs) >/dev/null 2>&1; then
  fail "Synology trusted-lan unexpectedly accepted without explicit acknowledgement"
fi
if ! (PROFILE=local-server; MODE=default; CHECK_ONLY=0; SILENT=1; SYNOLOGY=1; UPS_NAME=ups; NUT_HOST=localhost; UPS_DRIVER=""; UPS_PORT=""; OPERATING_MODE=dry-run; OUTAGE_GRACE=120; RECOVERY_CHARGE=80; UTILITY_STABLE=120; NETWORK_WAIT=300; NETWORK_MODE=trusted-lan; NUT_LISTEN_IPV4=1; NUT_LISTEN_IPV6=0; NUT_ALLOWED_CLIENTS=(); ACCEPT_TRUSTED_LAN_SYNOLOGY=1; resolve_profile_inputs) >/dev/null 2>&1; then
  fail "explicit Synology trusted-lan acknowledgement was rejected"
fi

'''
if needle not in s:
    raise SystemExit('installer test insertion point not found')
s = s.replace(needle, block + needle, 1)
p.write_text(s)

# Document that restricted mode is the default recommendation and trusted-lan requires acknowledgement.
p = Path('docs/SYNOLOGY.md')
s = p.read_text()
s = replace_once(s, '''Because `monuser/secret` is a compatibility convention rather than a strong secret, deployments rely on trusted/restricted LAN controls and no public Internet exposure of NUT TCP 3493.
''', '''Because `monuser/secret` is a compatibility convention rather than a strong secret, deployments rely on network controls and no public Internet exposure of NUT TCP 3493.

`restricted` NUT network mode is recommended for Synology deployments. If `trusted-lan` is intentionally used, the installer requires the explicit `--accept-trusted-lan-synology` acknowledgement (or the equivalent TUI confirmation) before exposing the DSM compatibility account. The acknowledgement does not make the fixed credential secret; it records that the operator intentionally trusts every host able to reach TCP 3493.
''', 'Synology security docs')
p.write_text(s)

Path('agent/internal/config/synology_secret_test.go').write_text(r'''package config

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
''')

Path('agent/internal/app/config_revision_secret_test.go').write_text(r'''package app

import "testing"

func TestConfigRevisionChangesWhenEnabledSynologyPasswordChanges(t *testing.T) {
    first := baseConfig("dry-run")
    first.NUT.Synology.Enabled = true
    first.NUT.Synology.Username = "monuser"
    first.NUT.Synology.Password = "first-secret"
    second := first
    second.NUT.Synology.Password = "second-secret"

    firstRevision, err := configRevision(first)
    if err != nil {
        t.Fatal(err)
    }
    secondRevision, err := configRevision(second)
    if err != nil {
        t.Fatal(err)
    }
    if firstRevision == secondRevision {
        t.Fatal("enabled Synology password change did not change config revision")
    }
}
''')

# Existing plan sanitization test uses the default literal; also prove a unique value cannot leak.
p = Path('agent/internal/report/report_test.go')
s = p.read_text()
s = replace_once(s, '''\tb, err := json.Marshal(BuildPlan(cfg))
''', '''\tcfg.NUT.Synology.Enabled = true
\tcfg.NUT.Synology.Password = "unique-plan-secret"
\tb, err := json.Marshal(BuildPlan(cfg))
''', 'plan secret fixture')
s = replace_once(s, '''\tfor _, forbidden := range []string{"secret", "ssh_key_file", "workstation_ed25519", "password"} {
''', '''\tfor _, forbidden := range []string{"secret", "unique-plan-secret", "ssh_key_file", "workstation_ed25519", "password"} {
''', 'plan forbidden values')
p.write_text(s)
