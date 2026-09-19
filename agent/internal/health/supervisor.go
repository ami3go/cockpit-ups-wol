package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Check interface {
	Name() string
	Run(context.Context) Result
}

type Repairer interface {
	Name() string
	Repair(context.Context) error
}

type Supervisor struct {
	Checks            []Check
	Repairers         map[string]Repairer
	StatePath         string
	MaxRepairAttempts int
	BaseBackoff       time.Duration
	MaxBackoff        time.Duration
	Now               func() time.Time
}

func (s *Supervisor) defaults() {
	if s.MaxRepairAttempts <= 0 {
		s.MaxRepairAttempts = 5
	}
	if s.BaseBackoff <= 0 {
		s.BaseBackoff = 30 * time.Second
	}
	if s.MaxBackoff <= 0 {
		s.MaxBackoff = 15 * time.Minute
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Repairers == nil {
		s.Repairers = map[string]Repairer{}
	}
}

func (s *Supervisor) Run(ctx context.Context, autofix bool) (Snapshot, error) {
	s.defaults()
	snap, _ := s.Load()
	if snap.Version == 0 {
		snap = NewSnapshot()
	}
	snap.CheckedAt = s.Now().UTC()
	snap.Generation++
	snap.Results = nil
	snap.Repairs = nil
	snap.State = Healthy
	if snap.Circuit.Attempts == nil {
		snap.Circuit.Attempts = map[string]int{}
	}
	if snap.Circuit.NextAllowedUTC == nil {
		snap.Circuit.NextAllowedUTC = map[string]time.Time{}
	}

	checks := append([]Check(nil), s.Checks...)
	sort.SliceStable(checks, func(i, j int) bool { return checks[i].Name() < checks[j].Name() })
	for _, check := range checks {
		result := check.Run(ctx)
		if result.Name == "" {
			result.Name = check.Name()
		}
		snap.Results = append(snap.Results, result)
		if result.OK {
			delete(snap.Circuit.Attempts, result.Name)
			delete(snap.Circuit.NextAllowedUTC, result.Name)
			continue
		}
		if result.Critical {
			snap.State = Degraded
		} else if snap.State == Healthy {
			snap.State = Degraded
		}

		if !autofix || !result.Repairable {
			continue
		}
		repairer, ok := s.Repairers[result.Name]
		if !ok {
			continue
		}
		attempt := snap.Circuit.Attempts[result.Name]
		if attempt >= s.MaxRepairAttempts {
			if result.Critical {
				snap.State = FailedSafe
				snap.Circuit.FailedReason = "repair attempts exhausted for " + result.Name
			}
			continue
		}
		if next := snap.Circuit.NextAllowedUTC[result.Name]; !next.IsZero() && s.Now().Before(next) {
			if snap.State != FailedSafe {
				snap.State = Recovering
			}
			continue
		}

		attempt++
		snap.Circuit.Attempts[result.Name] = attempt
		repair := RepairRecord{Check: result.Name, Attempt: attempt}
		err := repairer.Repair(ctx)
		if err != nil {
			repair.Result = "failed"
			repair.Error = err.Error()
			snap.Repairs = append(snap.Repairs, repair)
			snap.Circuit.NextAllowedUTC[result.Name] = s.Now().Add(s.backoff(attempt))
			if attempt >= s.MaxRepairAttempts && result.Critical {
				snap.State = FailedSafe
				snap.Circuit.FailedReason = "repair attempts exhausted for " + result.Name
			} else if snap.State != FailedSafe {
				snap.State = Recovering
			}
			continue
		}
		repair.Result = "attempted"
		snap.Repairs = append(snap.Repairs, repair)
		snap.Circuit.NextAllowedUTC[result.Name] = s.Now().Add(s.backoff(attempt))
		if snap.State != FailedSafe {
			snap.State = Recovering
		}
	}

	if err := s.Save(snap); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

func (s *Supervisor) backoff(attempt int) time.Duration {
	d := s.BaseBackoff
	for i := 1; i < attempt; i++ {
		if d >= s.MaxBackoff/2 {
			return s.MaxBackoff
		}
		d *= 2
	}
	if d > s.MaxBackoff {
		return s.MaxBackoff
	}
	return d
}

func (s *Supervisor) Load() (Snapshot, error) {
	if s.StatePath == "" {
		return NewSnapshot(), nil
	}
	b, err := os.ReadFile(s.StatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewSnapshot(), nil
		}
		return Snapshot{}, err
	}
	var snap Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("decode health state: %w", err)
	}
	if snap.Version != 1 {
		return Snapshot{}, fmt.Errorf("unsupported health state version %d", snap.Version)
	}
	if snap.Circuit.Attempts == nil {
		snap.Circuit.Attempts = map[string]int{}
	}
	if snap.Circuit.NextAllowedUTC == nil {
		snap.Circuit.NextAllowedUTC = map[string]time.Time{}
	}
	return snap, nil
}

func (s *Supervisor) Save(snap Snapshot) error {
	if s.StatePath == "" {
		return nil
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	dir := filepath.Dir(s.StatePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".health-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, s.StatePath); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
