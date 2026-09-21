from pathlib import Path


def replace_once(text, old, new, label):
    if old not in text:
        raise SystemExit(f"{label} not found")
    return text.replace(old, new, 1)


rev = Path("agent/internal/config/revision.go")
text = rev.read_text()
text = replace_once(text, '''type Manager struct {
\tHistoryDir       string
\tActiveConfigPath string
\tProbation        time.Duration
\tHealthInterval   time.Duration
\tNow              func() time.Time
\tSleep            func(context.Context, time.Duration) error
}''', '''type Manager struct {
\tHistoryDir       string
\tActiveConfigPath string
\tProbation        time.Duration
\tHealthInterval   time.Duration
\tRevisionKeep     int
\tNow              func() time.Time
\tSleep            func(context.Context, time.Duration) error
}''', "Manager")
text = replace_once(text, '''\tif m.HealthInterval <= 0 {
\t\tm.HealthInterval = 5 * time.Second
\t}
\tif m.Now == nil {''', '''\tif m.HealthInterval <= 0 {
\t\tm.HealthInterval = 5 * time.Second
\t}
\tif m.RevisionKeep <= 0 {
\t\tm.RevisionKeep = defaultRevisionKeep
\t}
\tif m.Now == nil {''', "defaults")
text = replace_once(text, '''\tif err := m.writePointer("last-known-good", manifest.RevisionID); err != nil {
\t\treturn err
\t}
\treturn m.writePointer("active", manifest.RevisionID)
}''', '''\tif err := m.writePointer("last-known-good", manifest.RevisionID); err != nil {
\t\treturn err
\t}
\tif err := m.writePointer("active", manifest.RevisionID); err != nil {
\t\treturn err
\t}
\treturn m.pruneRevisionsLocked(m.RevisionKeep)
}''', "promoteLocked")
text = replace_once(text, '''func (m *Manager) writeManifest(manifest Manifest) error {
\tb, err := json.MarshalIndent(manifest, "", "  ")''', '''func (m *Manager) writeManifest(manifest Manifest) error {
\tif err := ValidateRevisionID(manifest.RevisionID); err != nil {
\t\treturn err
\t}
\tb, err := json.MarshalIndent(manifest, "", "  ")''', "writeManifest")
text = replace_once(text, '''func (m *Manager) readManifest(id string) (Manifest, error) {
\tvar x Manifest
\tb, err := os.ReadFile(filepath.Join(m.revisionDir(id), "manifest.json"))''', '''func (m *Manager) readManifest(id string) (Manifest, error) {
\tvar x Manifest
\tif err := ValidateRevisionID(id); err != nil {
\t\treturn x, err
\t}
\tb, err := os.ReadFile(filepath.Join(m.revisionDir(id), "manifest.json"))''', "readManifest")
text = replace_once(text, '''func (m *Manager) writePointer(name, value string) error {
\treturn writeAtomic(m.pointerPath(name), []byte(value+"\\n"), 0o600)
}''', '''func (m *Manager) writePointer(name, value string) error {
\tif err := ValidateRevisionID(value); err != nil {
\t\treturn fmt.Errorf("write %s pointer: %w", name, err)
\t}
\treturn writeAtomic(m.pointerPath(name), []byte(value+"\\n"), 0o600)
}''', "writePointer")
text = replace_once(text, '''\treturn strings.TrimSpace(string(b)), nil
}
func (m *Manager) lock()''', '''\tvalue := strings.TrimSpace(string(b))
\tif value == "" {
\t\treturn "", nil
\t}
\tif err := ValidateRevisionID(value); err != nil {
\t\treturn "", fmt.Errorf("read %s pointer: %w", name, err)
\t}
\treturn value, nil
}
func (m *Manager) lock()''', "readPointer")
rev.write_text(text)

ctl = Path("agent/cmd/cockpit-ups-wolctl/main.go")
text = ctl.read_text()
text = replace_once(text, '''\t\tif fs.NArg() != 2 {
\t\t\tfatal(fmt.Errorf("config-activate requires exactly one revision id"))
\t\t}
\t\tctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)''', '''\t\tif fs.NArg() != 2 {
\t\t\tfatal(fmt.Errorf("config-activate requires exactly one revision id"))
\t\t}
\t\trevisionID := fs.Arg(1)
\t\tif err := config.ValidateRevisionID(revisionID); err != nil {
\t\t\tfatal(err)
\t\t}
\t\tctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)''', "config-activate boundary")
text = replace_once(text, 'manager.ActivateAndValidate(ctx, fs.Arg(1), rt)', 'manager.ActivateAndValidate(ctx, revisionID, rt)', "config-activate call")
text = replace_once(text, '''\t\tif fs.NArg() != 2 {
\t\t\tfatal(fmt.Errorf("config-rollback requires exactly one revision id"))
\t\t}
\t\tctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)''', '''\t\tif fs.NArg() != 2 {
\t\t\tfatal(fmt.Errorf("config-rollback requires exactly one revision id"))
\t\t}
\t\trevisionID := fs.Arg(1)
\t\tif err := config.ValidateRevisionID(revisionID); err != nil {
\t\t\tfatal(err)
\t\t}
\t\tctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)''', "config-rollback boundary")
text = replace_once(text, 'manager.Rollback(ctx, fs.Arg(1), rt)', 'manager.Rollback(ctx, revisionID, rt)', "config-rollback call")
ctl.write_text(text)

Path("agent/internal/config/revision_maintenance.go").write_text(r'''package config

import (
    "fmt"
    "os"
    "regexp"
)

const defaultRevisionKeep = 10

var revisionIDPattern = regexp.MustCompile(`^cfg-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{8}(-[0-9]+)?$`)

func ValidateRevisionID(id string) error {
    if !revisionIDPattern.MatchString(id) {
        return fmt.Errorf("invalid revision id %q", id)
    }
    return nil
}

func (m *Manager) PruneRevisions(keep int) error {
    if keep < 1 {
        return fmt.Errorf("revision keep count must be >= 1")
    }
    unlock, err := m.lock()
    if err != nil {
        return err
    }
    defer unlock()
    return m.pruneRevisionsLocked(keep)
}

func (m *Manager) pruneRevisionsLocked(keep int) error {
    if keep < 1 {
        return fmt.Errorf("revision keep count must be >= 1")
    }
    protected := map[string]bool{}
    for _, name := range []string{"active", "last-known-good", "previous-known-good"} {
        id, err := m.readPointer(name)
        if err != nil {
            return fmt.Errorf("read protected revision pointer %s: %w", name, err)
        }
        if id != "" {
            protected[id] = true
        }
    }
    revisions, err := m.KnownGoodRevisions()
    if err != nil {
        return err
    }
    keepSet := map[string]bool{}
    for id := range protected {
        keepSet[id] = true
    }
    for i, revision := range revisions {
        if i >= keep {
            break
        }
        keepSet[revision.RevisionID] = true
    }
    removed := false
    for _, revision := range revisions {
        if keepSet[revision.RevisionID] {
            continue
        }
        if err := ValidateRevisionID(revision.RevisionID); err != nil {
            return err
        }
        if err := os.RemoveAll(m.revisionDir(revision.RevisionID)); err != nil {
            return fmt.Errorf("prune revision %s: %w", revision.RevisionID, err)
        }
        removed = true
    }
    if !removed {
        return nil
    }
    dir, err := os.Open(m.revisionsDir())
    if err != nil {
        return err
    }
    defer dir.Close()
    return dir.Sync()
}
''')

Path("agent/internal/config/revision_hardening_test.go").write_text(r'''package config

import (
    "context"
    "os"
    "strings"
    "testing"
)

func TestValidateRevisionIDRejectsTraversalAndMalformedInput(t *testing.T) {
    valid := "cfg-20260921T123456Z-deadbeef"
    if err := ValidateRevisionID(valid); err != nil {
        t.Fatalf("valid revision rejected: %v", err)
    }
    for _, id := range []string{"../etc", "cfg-20260921T123456Z-DEADBEEF", "cfg-20260921-deadbeef", "cfg-20260921T123456Z-deadbeef/child", ""} {
        if err := ValidateRevisionID(id); err == nil {
            t.Fatalf("malformed revision id accepted: %q", id)
        }
    }
}

func TestReadPointerRejectsTamperedRevisionID(t *testing.T) {
    m := newManager(t)
    if err := m.Ensure(); err != nil {
        t.Fatal(err)
    }
    if err := writeAtomic(m.pointerPath("active"), []byte("../../../tmp/evil\n"), 0o600); err != nil {
        t.Fatal(err)
    }
    if _, err := m.ActiveRevision(); err == nil || !strings.Contains(err.Error(), "invalid revision id") {
        t.Fatalf("tampered pointer not rejected: %v", err)
    }
}

func TestPruneRevisionsKeepsFloorAndProtectedPointers(t *testing.T) {
    m := newManager(t)
    m.RevisionKeep = 100
    var ids []string
    for i := 0; i < 5; i++ {
        content := append([]byte(nil), validJSON()...)
        content = append(content, []byte(strings.Repeat(" ", i))...)
        revision, err := m.Begin("retention-test", content)
        if err != nil {
            t.Fatal(err)
        }
        if _, err := m.ActivateAndValidate(context.Background(), revision.RevisionID, &fakeRuntime{}); err != nil {
            t.Fatal(err)
        }
        ids = append(ids, revision.RevisionID)
    }
    if err := m.writePointer("previous-known-good", ids[0]); err != nil {
        t.Fatal(err)
    }
    if err := m.PruneRevisions(2); err != nil {
        t.Fatal(err)
    }
    for _, id := range []string{ids[0], ids[3], ids[4]} {
        if _, err := os.Stat(m.revisionDir(id)); err != nil {
            t.Fatalf("protected/retained revision %s missing: %v", id, err)
        }
    }
    for _, id := range []string{ids[1], ids[2]} {
        if _, err := os.Stat(m.revisionDir(id)); !os.IsNotExist(err) {
            t.Fatalf("old unprotected revision %s was not pruned: %v", id, err)
        }
    }
}
''')
