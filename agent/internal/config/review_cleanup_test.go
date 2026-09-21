package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDependencyValidationReportsIndependentCycles(t *testing.T) {
	hosts := []HostConfig{{ID: "a", DependsOn: []string{"b"}}, {ID: "b", DependsOn: []string{"a"}}, {ID: "c", DependsOn: []string{"d"}}, {ID: "d", DependsOn: []string{"c"}}}
	var problems []string
	validateHostDependencyCycles(&problems, hosts)
	count := 0
	for _, p := range problems {
		if strings.Contains(p, "recovery dependency cycle detected") {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("problems=%v want 2 cycles", problems)
	}
}

func TestUniqueRevisionIDCollisionLimit(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	m := Manager{HistoryDir: t.TempDir(), Now: func() time.Time { return now }}
	if err := os.MkdirAll(m.revisionsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	base := fmt.Sprintf("cfg-%s-%s", now.Format("20060102T150405Z"), hash[:8])
	for n := 0; n < maxRevisionIDAttempts; n++ {
		id := base
		if n > 0 {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		if err := os.Mkdir(filepath.Join(m.revisionsDir(), id), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.uniqueRevisionID(hash); err == nil || !strings.Contains(err.Error(), "unable to allocate unique revision id") {
		t.Fatalf("unexpected error: %v", err)
	}
}
