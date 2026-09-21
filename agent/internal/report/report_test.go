package report

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/config"
)

func TestBuildPlanIsSanitized(t *testing.T) {
	data, err := os.ReadFile("../../../config/config.yaml.example")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(BuildPlan(cfg))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, forbidden := range []string{"secret", "ssh_key_file", "workstation_ed25519", "password"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("sanitized plan leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"battery_charge_min":80`) {
		t.Fatalf("recovery gate missing: %s", text)
	}
}

func TestPlanOrdering(t *testing.T) {
	data, err := os.ReadFile("../../../config/config.yaml.example")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	plan := BuildPlan(cfg)
	if len(plan.Shutdown) < 2 || plan.Shutdown[0].Priority > plan.Shutdown[1].Priority {
		t.Fatal("shutdown plan is not priority ordered")
	}
	if len(plan.Restore) < 2 || plan.Restore[0].WakePriority > plan.Restore[1].WakePriority {
		t.Fatal("restore plan is not priority ordered")
	}
}
