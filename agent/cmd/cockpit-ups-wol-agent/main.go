package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	app "github.com/ami3go/cockpit-ups-wol/agent/internal/app"
	"github.com/ami3go/cockpit-ups-wol/agent/internal/version"
)

func main() {
	fs := flag.NewFlagSet("cockpit-ups-wol-agent", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	showVersion := fs.Bool("version", false, "print version information and exit")
	configPath := fs.String("config", "/etc/cockpit-ups-wol/config.yaml", "configuration file")
	socketPath := fs.String("socket", "/run/cockpit-ups-wol/agent.sock", "agent Unix socket")
	healthStatePath := fs.String("health-state", "/var/lib/cockpit-ups-wol/health.json", "durable health state file")
	_ = fs.Bool("foreground", false, "run in foreground (default for systemd)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: cockpit-ups-wol-agent [options]\n\n")
		fmt.Fprintf(fs.Output(), "Power-management agent for cockpit-ups-wol.\n\nOptions:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *showVersion {
		b, _ := json.Marshal(version.Current())
		fmt.Println(string(b))
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.LoadAndRun(ctx, *configPath, app.Options{SocketPath: *socketPath, HealthStatePath: *healthStatePath}); err != nil {
		fmt.Fprintf(os.Stderr, "cockpit-ups-wol-agent: %v\n", err)
		os.Exit(1)
	}
}
