package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/health"
)

type stringList []string
func (s *stringList) String() string { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	fs := flag.NewFlagSet("cockpit-ups-wol-health", flag.ExitOnError)
	statePath := fs.String("state", "/var/lib/cockpit-ups-wol/health.json", "durable health/circuit state")
	repair := fs.Bool("repair", false, "attempt allowlisted safe repairs")
	maxAttempts := fs.Int("max-repair-attempts", 5, "maximum repair attempts before circuit breaker")
	var units stringList
	fs.Var(&units, "required-unit", "required systemd unit (repeatable)")
	_ = fs.Parse(os.Args[1:])
	if len(units) == 0 {
		units = append(units, "cockpit-ups-wol-agent.service")
	}

	s := &health.Supervisor{StatePath: *statePath, MaxRepairAttempts: *maxAttempts, Repairers: map[string]health.Repairer{}}
	for _, unit := range units {
		check := health.SystemdUnitCheck{Unit: unit, Critical: true}
		s.Checks = append(s.Checks, check)
		s.Repairers[check.Name()] = health.SystemdUnitRepair{Unit: unit}
	}
	snap, err := s.Run(context.Background(), *repair)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cockpit-ups-wol-health:", err)
		os.Exit(1)
	}
	fmt.Printf("%s generation=%d checks=%d repairs=%d\n", snap.State, snap.Generation, len(snap.Results), len(snap.Repairs))
	if snap.State == health.FailedSafe {
		os.Exit(2)
	}
}
