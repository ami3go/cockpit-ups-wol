package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type RevisionStatus string

const (
	RevisionCandidate  RevisionStatus = "candidate"
	RevisionValidating RevisionStatus = "validating"
	RevisionKnownGood  RevisionStatus = "known-good"
	RevisionFailed     RevisionStatus = "failed"
	RevisionRolledBack RevisionStatus = "rolled-back"
)

type Manifest struct {
	RevisionID     string         `json:"revision_id"`
	CreatedAt      string         `json:"created_at"`
	Source         string         `json:"source"`
	SchemaVersion  int            `json:"schema_version"`
	ParentRevision string         `json:"parent_revision,omitempty"`
	Status         RevisionStatus `json:"status"`
	ContentSHA256  string         `json:"content_sha256"`
	FailureReason  string         `json:"failure_reason,omitempty"`
}

type Runtime interface {
	Apply(context.Context, string) error
	Healthy(context.Context) error
}

type Manager struct {
	HistoryDir       string
	ActiveConfigPath string
	Probation        time.Duration
	HealthInterval   time.Duration
	RevisionKeep     int
	Now              func() time.Time
	Sleep            func(context.Context, time.Duration) error
}

func (m *Manager) defaults() {
	if m.Probation <= 0 {
		m.Probation = 60 * time.Second
	}
	if m.HealthInterval <= 0 {
		m.HealthInterval = 5 * time.Second
	}
	if m.RevisionKeep <= 0 {
		m.RevisionKeep = defaultRevisionKeep
	}
	if m.Now == nil {
		m.Now = time.Now
	}
	if m.Sleep == nil {
		m.Sleep = sleepContext
	}
}
func (m *Manager) revisionsDir() string           { return filepath.Join(m.HistoryDir, "revisions") }
func (m *Manager) lockPath() string               { return filepath.Join(m.HistoryDir, "lock") }
func (m *Manager) pointerPath(name string) string { return filepath.Join(m.HistoryDir, name) }
func (m *Manager) revisionDir(id string) string   { return filepath.Join(m.revisionsDir(), id) }
func (m *Manager) Ensure() error {
	if err := os.MkdirAll(m.revisionsDir(), 0o700); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Dir(m.ActiveConfigPath), 0o755)
}

func (m *Manager) Begin(source string, content []byte) (Manifest, error) {
	m.defaults()
	unlock, err := m.lock()
	if err != nil {
		return Manifest{}, err
	}
	defer unlock()
	return m.beginLocked(source, content)
}
func (m *Manager) beginLocked(source string, content []byte) (Manifest, error) {
	if _, err := Parse(content); err != nil {
		return Manifest{}, err
	}
	sum := sha256.Sum256(content)
	hexsum := hex.EncodeToString(sum[:])
	id, err := m.uniqueRevisionID(hexsum)
	if err != nil {
		return Manifest{}, err
	}
	parent, _ := m.readPointer("active")
	revdir := m.revisionDir(id)
	if err := os.MkdirAll(revdir, 0o700); err != nil {
		return Manifest{}, err
	}
	if err := writeAtomic(filepath.Join(revdir, "config.yaml"), content, 0o600); err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{RevisionID: id, CreatedAt: m.Now().UTC().Format(time.RFC3339), Source: source, SchemaVersion: 1, ParentRevision: parent, Status: RevisionCandidate, ContentSHA256: hexsum}
	if err := m.writeManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
func (m *Manager) ActivateAndValidate(ctx context.Context, id string, r Runtime) (Manifest, error) {
	m.defaults()
	unlock, err := m.lock()
	if err != nil {
		return Manifest{}, err
	}
	defer unlock()
	manifest, err := m.readManifest(id)
	if err != nil {
		return Manifest{}, err
	}
	if manifest.Status != RevisionCandidate && manifest.Status != RevisionValidating {
		return Manifest{}, fmt.Errorf("revision %s not activatable from status %s", id, manifest.Status)
	}
	content, err := os.ReadFile(filepath.Join(m.revisionDir(id), "config.yaml"))
	if err != nil {
		return Manifest{}, err
	}
	if !contentMatches(manifest, content) {
		return Manifest{}, m.failLocked(manifest, errors.New("candidate content hash does not match manifest"))
	}
	if _, err := Parse(content); err != nil {
		return Manifest{}, m.failLocked(manifest, err)
	}
	if err := writeAtomic(m.ActiveConfigPath, content, 0o600); err != nil {
		return Manifest{}, m.failLocked(manifest, err)
	}
	if err := m.writePointer("active", id); err != nil {
		return Manifest{}, m.rollbackLocked(ctx, manifest, fmt.Errorf("activate pointer: %w", err), r)
	}
	manifest.Status = RevisionValidating
	manifest.FailureReason = ""
	if err := m.writeManifest(manifest); err != nil {
		return Manifest{}, err
	}
	if r == nil {
		return Manifest{}, m.rollbackLocked(ctx, manifest, errors.New("runtime validator is required"), nil)
	}
	if err := r.Apply(ctx, m.ActiveConfigPath); err != nil {
		return Manifest{}, m.rollbackLocked(ctx, manifest, fmt.Errorf("runtime apply: %w", err), r)
	}
	if err := r.Healthy(ctx); err != nil {
		return Manifest{}, m.rollbackLocked(ctx, manifest, fmt.Errorf("immediate health: %w", err), r)
	}
	deadline := m.Now().Add(m.Probation)
	for m.Now().Before(deadline) {
		remaining := deadline.Sub(m.Now())
		wait := m.HealthInterval
		if wait > remaining {
			wait = remaining
		}
		if wait > 0 {
			if err := m.Sleep(ctx, wait); err != nil {
				return Manifest{}, m.rollbackLocked(ctx, manifest, err, r)
			}
		}
		if err := r.Healthy(ctx); err != nil {
			return Manifest{}, m.rollbackLocked(ctx, manifest, fmt.Errorf("probation health: %w", err), r)
		}
	}
	if err := m.promoteLocked(&manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
func (m *Manager) promoteLocked(manifest *Manifest) error {
	manifest.Status = RevisionKnownGood
	manifest.FailureReason = ""
	if err := m.writeManifest(*manifest); err != nil {
		return err
	}
	oldLKG, _ := m.readPointer("last-known-good")
	if oldLKG != "" && oldLKG != manifest.RevisionID {
		if err := m.writePointer("previous-known-good", oldLKG); err != nil {
			return err
		}
	}
	if err := m.writePointer("last-known-good", manifest.RevisionID); err != nil {
		return err
	}
	if err := m.writePointer("active", manifest.RevisionID); err != nil {
		return err
	}
	// Everything above is the commit point. Retention cleanup must not make an
	// already-active known-good revision look like a failed activation.
	if err := m.pruneRevisionsLocked(m.RevisionKeep); err != nil {
		fmt.Fprintf(os.Stderr, "cockpit-ups-wol: warning: revision pruning failed after successful promotion: %v\n", err)
	}
	return nil
}
func (m *Manager) rollbackLocked(ctx context.Context, failed Manifest, cause error, r Runtime) error {
	failed.Status = RevisionFailed
	failed.FailureReason = cause.Error()
	_ = m.writeManifest(failed)
	lkg, err := m.readPointer("last-known-good")
	if err != nil || lkg == "" {
		return fmt.Errorf("candidate %s failed and no last-known-good is available: %w", failed.RevisionID, cause)
	}
	content, err := os.ReadFile(filepath.Join(m.revisionDir(lkg), "config.yaml"))
	if err != nil {
		return fmt.Errorf("read last-known-good %s: %w", lkg, err)
	}
	if err := writeAtomic(m.ActiveConfigPath, content, 0o600); err != nil {
		return fmt.Errorf("restore last-known-good: %w", err)
	}
	if err := m.writePointer("active", lkg); err != nil {
		return err
	}
	if r != nil {
		if err := r.Apply(ctx, m.ActiveConfigPath); err != nil {
			return fmt.Errorf("rollback apply failed: %w", err)
		}
		if err := r.Healthy(ctx); err != nil {
			return fmt.Errorf("rollback health failed: %w", err)
		}
	}
	failed.Status = RevisionRolledBack
	_ = m.writeManifest(failed)
	return fmt.Errorf("candidate %s rolled back to %s: %w", failed.RevisionID, lkg, cause)
}
func (m *Manager) RecoverInterrupted(ctx context.Context, r Runtime) error {
	m.defaults()
	unlock, err := m.lock()
	if err != nil {
		return err
	}
	defer unlock()
	active, _ := m.readPointer("active")
	lkg, _ := m.readPointer("last-known-good")
	if active == "" {
		return nil
	}
	manifest, err := m.readManifest(active)
	if err != nil {
		return m.restoreKnownGoodLocked(ctx, lkg, r)
	}
	if active == lkg {
		if manifest.Status != RevisionKnownGood {
			return fmt.Errorf("last-known-good pointer references non-known-good revision %s (%s)", active, manifest.Status)
		}
		return nil
	}
	if manifest.Status == RevisionKnownGood {
		if lkg != "" && lkg != active {
			if err := m.writePointer("previous-known-good", lkg); err != nil {
				return err
			}
		}
		return m.writePointer("last-known-good", active)
	}
	if manifest.Status == RevisionCandidate || manifest.Status == RevisionValidating || manifest.Status == RevisionFailed {
		return m.restoreKnownGoodLocked(ctx, lkg, r)
	}
	return nil
}
func (m *Manager) restoreKnownGoodLocked(ctx context.Context, lkg string, r Runtime) error {
	if lkg == "" {
		return errors.New("no last-known-good available for interrupted transaction")
	}
	content, err := os.ReadFile(filepath.Join(m.revisionDir(lkg), "config.yaml"))
	if err != nil {
		return err
	}
	if err := writeAtomic(m.ActiveConfigPath, content, 0o600); err != nil {
		return err
	}
	if err := m.writePointer("active", lkg); err != nil {
		return err
	}
	if r != nil {
		if err := r.Apply(ctx, m.ActiveConfigPath); err != nil {
			return err
		}
		if err := r.Healthy(ctx); err != nil {
			return err
		}
	}
	return nil
}
func (m *Manager) Rollback(ctx context.Context, target string, r Runtime) error {
	m.defaults()
	unlock, err := m.lock()
	if err != nil {
		return err
	}
	defer unlock()
	manifest, err := m.readManifest(target)
	if err != nil {
		return err
	}
	if manifest.Status != RevisionKnownGood {
		return fmt.Errorf("revision %s is not known-good", target)
	}
	content, err := os.ReadFile(filepath.Join(m.revisionDir(target), "config.yaml"))
	if err != nil {
		return err
	}
	if !contentMatches(manifest, content) {
		return errors.New("rollback target content hash does not match manifest")
	}
	if _, err := Parse(content); err != nil {
		return err
	}
	oldActive, _ := m.readPointer("active")
	oldLKG, _ := m.readPointer("last-known-good")
	if err := writeAtomic(m.ActiveConfigPath, content, 0o600); err != nil {
		return err
	}
	if err := m.writePointer("active", target); err != nil {
		return err
	}
	if r != nil {
		if err := r.Apply(ctx, m.ActiveConfigPath); err != nil {
			return m.restoreAfterManualRollbackFailure(ctx, oldActive, oldLKG, r, err)
		}
		if err := r.Healthy(ctx); err != nil {
			return m.restoreAfterManualRollbackFailure(ctx, oldActive, oldLKG, r, err)
		}
	}
	if oldLKG != "" && oldLKG != target {
		if err := m.writePointer("previous-known-good", oldLKG); err != nil {
			return err
		}
	}
	return m.writePointer("last-known-good", target)
}
func (m *Manager) restoreAfterManualRollbackFailure(ctx context.Context, oldActive, oldLKG string, r Runtime, cause error) error {
	if oldActive == "" {
		return fmt.Errorf("rollback target failed and no previous active revision exists: %w", cause)
	}
	content, err := os.ReadFile(filepath.Join(m.revisionDir(oldActive), "config.yaml"))
	if err != nil {
		return fmt.Errorf("rollback target failed (%v) and previous revision could not be read: %w", cause, err)
	}
	if err := writeAtomic(m.ActiveConfigPath, content, 0o600); err != nil {
		return fmt.Errorf("rollback target failed (%v) and previous config restore failed: %w", cause, err)
	}
	_ = m.writePointer("active", oldActive)
	if oldLKG != "" {
		_ = m.writePointer("last-known-good", oldLKG)
	}
	if err := r.Apply(ctx, m.ActiveConfigPath); err != nil {
		return fmt.Errorf("rollback target failed (%v); restoring previous runtime also failed: %w", cause, err)
	}
	if err := r.Healthy(ctx); err != nil {
		return fmt.Errorf("rollback target failed (%v); restored previous runtime unhealthy: %w", cause, err)
	}
	return fmt.Errorf("rollback target rejected; previous revision restored: %w", cause)
}
func (m *Manager) ActiveRevision() (string, error)      { return m.readPointer("active") }
func (m *Manager) LastKnownGood() (string, error)       { return m.readPointer("last-known-good") }
func (m *Manager) Manifest(id string) (Manifest, error) { return m.readManifest(id) }
func (m *Manager) failLocked(manifest Manifest, cause error) error {
	manifest.Status = RevisionFailed
	manifest.FailureReason = cause.Error()
	_ = m.writeManifest(manifest)
	return cause
}
func (m *Manager) writeManifest(manifest Manifest) error {
	if err := ValidateRevisionID(manifest.RevisionID); err != nil {
		return err
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeAtomic(filepath.Join(m.revisionDir(manifest.RevisionID), "manifest.json"), b, 0o600)
}
func (m *Manager) readManifest(id string) (Manifest, error) {
	var x Manifest
	if err := ValidateRevisionID(id); err != nil {
		return x, err
	}
	b, err := os.ReadFile(filepath.Join(m.revisionDir(id), "manifest.json"))
	if err != nil {
		return x, err
	}
	err = json.Unmarshal(b, &x)
	return x, err
}
func (m *Manager) writePointer(name, value string) error {
	if err := ValidateRevisionID(value); err != nil {
		return fmt.Errorf("write %s pointer: %w", name, err)
	}
	return writeAtomic(m.pointerPath(name), []byte(value+"\n"), 0o600)
}
func (m *Manager) readPointer(name string) (string, error) {
	b, err := os.ReadFile(m.pointerPath(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	value := strings.TrimSpace(string(b))
	if value == "" {
		return "", nil
	}
	if err := ValidateRevisionID(value); err != nil {
		return "", fmt.Errorf("read %s pointer: %w", name, err)
	}
	return value, nil
}
func (m *Manager) lock() (func(), error) {
	if err := m.Ensure(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(m.lockPath(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func contentMatches(manifest Manifest, content []byte) bool {
	sum := sha256.Sum256(content)
	return strings.EqualFold(manifest.ContentSHA256, hex.EncodeToString(sum[:]))
}

const maxRevisionIDAttempts = 100

func (m *Manager) uniqueRevisionID(contentHash string) (string, error) {
	if len(contentHash) < 8 {
		return "", errors.New("content hash is too short to allocate revision id")
	}
	base := fmt.Sprintf("cfg-%s-%s", m.Now().UTC().Format("20060102T150405Z"), contentHash[:8])
	for n := 0; n < maxRevisionIDAttempts; n++ {
		id := base
		if n > 0 {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		_, err := os.Stat(m.revisionDir(id))
		if errors.Is(err, os.ErrNotExist) {
			return id, nil
		}
		if err != nil {
			return "", fmt.Errorf("check revision id %q: %w", id, err)
		}
	}
	return "", fmt.Errorf("unable to allocate unique revision id after %d attempts", maxRevisionIDAttempts)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
