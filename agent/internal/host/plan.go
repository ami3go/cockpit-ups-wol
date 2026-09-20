package host

import (
	"errors"
	"sort"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/state"
)

type RestorePolicy string

const (
	RestorePrevious RestorePolicy = "previous-state"
	RestoreAlways   RestorePolicy = "always"
	RestoreNever    RestorePolicy = "never"
)

type Config struct {
	ID                       string
	ShutdownMethod           string
	ShutdownPriority         int
	WakeEnabled              bool
	WakePriority             int
	WakeMaxAttempts          int
	WakeDelayAfterPreviousSeconds int
	WakeMAC                  string
	WakeInterface            string
	WakeBroadcast            string
	WakePort                 int
	RestorePolicy            RestorePolicy
	DependsOn                []string
}

type ShutdownPlan struct {
	PreFSD   []string
	NUTGroup []string
}

func BuildShutdownPlan(hosts []Config) ShutdownPlan {
	pre := make([]Config, 0)
	nutGroup := make([]Config, 0)
	for _, h := range hosts {
		switch h.ShutdownMethod {
		case "nut":
			nutGroup = append(nutGroup, h)
		case "none", "":
			continue
		default:
			pre = append(pre, h)
		}
	}
	sort.SliceStable(pre, func(i, j int) bool {
		if pre[i].ShutdownPriority == pre[j].ShutdownPriority {
			return pre[i].ID < pre[j].ID
		}
		return pre[i].ShutdownPriority < pre[j].ShutdownPriority
	})
	sort.SliceStable(nutGroup, func(i, j int) bool { return nutGroup[i].ID < nutGroup[j].ID })
	plan := ShutdownPlan{}
	for _, h := range pre {
		plan.PreFSD = append(plan.PreFSD, h.ID)
	}
	for _, h := range nutGroup {
		plan.NUTGroup = append(plan.NUTGroup, h.ID)
	}
	return plan
}

func EligibleForRestore(cfg Config, st state.HostState) bool {
	if !cfg.WakeEnabled {
		return false
	}
	switch cfg.RestorePolicy {
	case RestoreAlways:
		return true
	case RestoreNever:
		return false
	case RestorePrevious, "":
		return st.WasOnline != nil && *st.WasOnline
	default:
		return false
	}
}

func BuildRecoveryPlan(hosts []Config, states map[string]state.HostState) ([]string, error) {
	eligible := make(map[string]Config)
	for _, h := range hosts {
		if st, ok := states[h.ID]; ok && EligibleForRestore(h, st) {
			eligible[h.ID] = h
		}
	}

	indegree := make(map[string]int, len(eligible))
	children := make(map[string][]string)
	for id := range eligible {
		indegree[id] = 0
	}
	for id, h := range eligible {
		for _, dep := range h.DependsOn {
			if _, ok := eligible[dep]; !ok {
				continue // external/non-restored dependency is checked by readiness layer
			}
			indegree[id]++
			children[dep] = append(children[dep], id)
		}
	}

	ready := make([]Config, 0)
	for id, n := range indegree {
		if n == 0 {
			ready = append(ready, eligible[id])
		}
	}
	less := func(i, j int) bool {
		if ready[i].WakePriority == ready[j].WakePriority {
			return ready[i].ID < ready[j].ID
		}
		return ready[i].WakePriority < ready[j].WakePriority
	}

	result := make([]string, 0, len(eligible))
	for len(ready) > 0 {
		sort.SliceStable(ready, less)
		next := ready[0]
		ready = ready[1:]
		result = append(result, next.ID)
		for _, child := range children[next.ID] {
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, eligible[child])
			}
		}
	}
	if len(result) != len(eligible) {
		return nil, errors.New("recovery dependency cycle detected")
	}
	return result, nil
}
