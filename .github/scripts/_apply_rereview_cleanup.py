from pathlib import Path


def replace(path: str, old: str, new: str, count: int = 1) -> None:
    p = Path(path)
    s = p.read_text()
    if old not in s:
        raise SystemExit(f"missing expected text in {path}: {old[:160]!r}")
    p.write_text(s.replace(old, new, count))


# R7: pruning happens after the durable config commit, so report it as housekeeping only.
replace(
    "agent/internal/config/revision.go",
    '''\tif err := m.writePointer("active", manifest.RevisionID); err != nil {
\t\treturn err
\t}
\treturn m.pruneRevisionsLocked(m.RevisionKeep)
''',
    '''\tif err := m.writePointer("active", manifest.RevisionID); err != nil {
\t\treturn err
\t}
\t// Everything above is the commit point. Retention cleanup must not make an
\t// already-active known-good revision look like a failed activation.
\tif err := m.pruneRevisionsLocked(m.RevisionKeep); err != nil {
\t\tfmt.Fprintf(os.Stderr, "cockpit-ups-wol: warning: revision pruning failed after successful promotion: %v\\n", err)
\t}
\treturn nil
''',
)

replace(
    "scripts/install/transaction.sh",
    '''    log "pruning old rollback snapshot: $entry"
    rm -rf -- "$entry"
  done
  sync_transaction_dir "$BACKUP_BASE"
''',
    '''    log "pruning old rollback snapshot: $entry"
    rm -rf -- "$entry" || { warn "failed to prune $entry (installation already committed)"; continue; }
  done
  sync_transaction_dir "$BACKUP_BASE" || warn "failed to fsync rollback snapshot directory after committed housekeeping"
''',
)

# R8: finish the interrupted recovery-abort transition on boot instead of parking on battery.
replace(
    "agent/internal/policy/engine.go",
    '''\tif e.st.ShutdownCommitted {
''',
    '''\tif e.st.ShutdownCommitted && e.resumeState == state.OnBattery && in.UPS.Utility == nut.UtilityOnBattery {
\t\t// Recovery abort persists ON_BATTERY before the orchestrator opens the
\t\t// fresh outage transaction. If power is lost between those two durable
\t\t// writes, complete that transition now so restored hosts become shutdown
\t\t// targets again rather than parking forever in WAITING_FOR_AC.
\t\te.st.PowerState = state.OnBattery
\t\treturn Decision{Changed: true, Action: ActionStopRecovery, Reason: "completing interrupted recovery abort"}, nil
\t}

\tif e.st.ShutdownCommitted {
''',
    1,
)

# R11: reject numeric final labels so malformed IPv4 literals cannot fall through as hostnames.
replace(
    "agent/internal/config/address_validation.go",
    '''\tfor _, label := range strings.Split(v, ".") {
\t\tif !hostnameLabelPattern.MatchString(label) {
\t\t\treturn false
\t\t}
\t}
''',
    '''\tlabels := strings.Split(v, ".")
\tif isAllDigits(labels[len(labels)-1]) {
\t\treturn false
\t}
\tfor _, label := range labels {
\t\tif !hostnameLabelPattern.MatchString(label) {
\t\t\treturn false
\t\t}
\t}
''',
)
Path("agent/internal/config/address_validation.go").write_text(
    Path("agent/internal/config/address_validation.go").read_text()
    + '''\nfunc isAllDigits(v string) bool {
\tif v == "" {
\t\treturn false
\t}
\tfor _, r := range v {
\t\tif r < '0' || r > '9' {
\t\t\treturn false
\t\t}
\t}
\treturn true
}
'''
)

# R10: bash -n accepts one script; make xargs invoke it once per file.
replace(
    ".github/workflows/ci.yml",
    "find . -type f -name '*.sh' -not -path './.git/*' -print0 | xargs -0 bash -n",
    "find . -type f -name '*.sh' -not -path './.git/*' -print0 | xargs -0 -n1 bash -n",
)

# R5: CI and release artifacts use one supported Go line; scan the actual shipped binaries too.
for path in [".github/workflows/ci.yml", ".github/workflows/package.yml"]:
    p = Path(path)
    p.write_text(p.read_text().replace("go-version: '1.23.x'", "go-version: '1.26.x'"))
replace(
    ".github/workflows/package.yml",
    '''      - name: Verify checksums and package contents
''',
    '''      - name: Scan shipped Go binaries
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
          rm -rf /tmp/cockpit-ups-wol-release-scan
          mkdir -p /tmp/cockpit-ups-wol-release-scan
          for archive in release/*.tar.gz; do
            tar -xzf "$archive" -C /tmp/cockpit-ups-wol-release-scan
          done
          while IFS= read -r -d '' binary; do
            govulncheck -mode=binary "$binary"
          done < <(find /tmp/cockpit-ups-wol-release-scan -type f \\
            \( -name cockpit-ups-wol-agent -o -name cockpit-ups-wolctl -o -name cockpit-ups-wol-health -o -name wolctl \) -print0)
      - name: Verify checksums and package contents
''',
)

Path("agent/internal/policy/recovery_abort_restart_test.go").write_text(r'''package policy

import (
	"testing"
	"time"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/nut"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

func TestBootCompletesInterruptedRecoveryAbortOnBattery(t *testing.T) {
	st := state.New("outage-1", "cfg")
	st.PowerState = state.OnBattery
	st.ShutdownCommitted = true
	st.RecoveryStarted = false
	engine := New(Config{RecoveryEnabled: boolPtr(true)}, st)

	decision, err := engine.Step(time.Unix(100, 0), Inputs{UPS: nut.Status{Utility: nut.UtilityOnBattery}})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionStopRecovery {
		t.Fatalf("action=%s, want %s", decision.Action, ActionStopRecovery)
	}
	if engine.State().PowerState != state.OnBattery {
		t.Fatalf("power=%s, want ON_BATTERY", engine.State().PowerState)
	}
}

func boolPtr(v bool) *bool { return &v }
''')

Path("agent/internal/config/address_validation_test.go").write_text(r'''package config

import "testing"

func TestValidHostnameRejectsIPv4Typos(t *testing.T) {
	for _, value := range []string{"192.168.1.300", "192.168.1", "10.0.0.256"} {
		if validHostname(value) {
			t.Errorf("validHostname(%q)=true; malformed IPv4-like value must not pass as DNS", value)
		}
	}
}

func TestValidHostnameAllowsOrdinaryNames(t *testing.T) {
	for _, value := range []string{"nas", "nas.local", "ups-1.example.com", "node42.home.arpa"} {
		if !validHostname(value) {
			t.Errorf("validHostname(%q)=false", value)
		}
	}
}
''')
